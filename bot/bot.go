package bot

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/mdp/qrterminal/v3"
	"github.com/neoncorp/Whatskel/config"
	"github.com/neoncorp/Whatskel/plugins"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types/events"
	waLog "go.mau.fi/whatsmeow/util/log"
	"google.golang.org/protobuf/proto"

	_ "modernc.org/sqlite"
)

type Bot struct {
	client  *whatsmeow.Client
	cfg     *config.Config
	plugins *plugins.Loader
	ctx     context.Context
	cancel  context.CancelFunc
}

func New(cfg *config.Config) (*Bot, error) {
	ctx, cancel := context.WithCancel(context.Background())

	dbLog := waLog.Noop
	storeContainer, err := sqlstore.New(ctx, "sqlite", "file:"+cfg.Bot.DbPath+"?_pragma=foreign_keys(1)", dbLog)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to create store: %w", err)
	}

	device, err := storeContainer.GetFirstDevice(ctx)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to get device: %w", err)
	}

	client := whatsmeow.NewClient(device, nil)

	loader := plugins.NewLoader(cfg.Plugins.Directory)
	if err := loader.LoadAll(); err != nil {
		cancel()
		return nil, fmt.Errorf("failed to load plugins: %w", err)
	}

	return &Bot{
		client:  client,
		cfg:     cfg,
		plugins: loader,
		ctx:     ctx,
		cancel:  cancel,
	}, nil
}

func (b *Bot) Start() error {
	b.client.AddEventHandler(func(evt interface{}) {
		switch v := evt.(type) {
		case *events.Message:
			b.handleMessage(v)
		case *events.Disconnected:
			log.Println("Disconnected from WhatsApp. Attempting to auto-reconnect...")
			go func() {
				backoff := 2 * time.Second
				for i := 0; i < 5; i++ {
					time.Sleep(backoff)
					log.Printf("Reconnect attempt %d/5...", i+1)
					if err := b.client.Connect(); err != nil {
						log.Printf("Reconnect attempt %d failed: %v", i+1, err)
						backoff *= 2
						continue
					}
					log.Println("Reconnected to WhatsApp successfully!")
					return
				}
				log.Println("Failed to reconnect after 5 attempts. Manual restart required.")
			}()
		case *events.CallOffer:
			log.Printf("Incoming call received from %s. Rejecting...", v.CallCreator)
			if err := b.client.RejectCall(b.ctx, v.CallCreator, v.CallID); err != nil {
				log.Printf("Failed to reject call: %v", err)
			}
		}
	})

	if b.client.Store.ID == nil {
		qrChan, err := b.client.GetQRChannel(b.ctx)
		if err != nil {
			return fmt.Errorf("failed to get QR channel: %w", err)
		}
		go func() {
			fmt.Println("Scan the QR code below with WhatsApp (Linked Devices):")
			for item := range qrChan {
				switch item.Event {
				case "code":
					fmt.Println()
					qrterminal.GenerateHalfBlock(item.Code, qrterminal.L, os.Stdout)
					fmt.Println()
				case "timeout":
					fmt.Println("QR code timed out, restart needed")
				case "success":
					fmt.Println("QR code scanned successfully!")
				case "error":
					fmt.Printf("QR error: %v\n", item.Error)
				}
			}
		}()
	}

	if err := b.client.Connect(); err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}

	log.Println("Connected to WhatsApp!")
	return nil
}

func (b *Bot) Stop() {
	b.cancel()
	b.plugins.Close()
	b.client.Disconnect()
}

func (b *Bot) handleMessage(v *events.Message) {
	// Skip messages sent by the bot itself
	if v.Info.IsFromMe {
		return
	}

	msg := v.Message.GetConversation()
	if msg == "" {
		if v.Message.GetExtendedTextMessage() != nil {
			msg = v.Message.GetExtendedTextMessage().GetText()
		}
	}
	if msg == "" {
		return
	}

	prefix := b.cfg.Bot.Prefix
	if !strings.HasPrefix(msg, prefix) {
		return
	}

	rest := strings.TrimSpace(strings.TrimPrefix(msg, prefix))
	parts := strings.SplitN(rest, " ", 2)
	cmdName := strings.ToLower(parts[0])
	args := ""
	if len(parts) > 1 {
		args = parts[1]
	}

	chatJID := v.Info.Chat
	if chatJID.IsEmpty() {
		chatJID = v.Info.Sender
	}

	replyFn := func(text string) error {
		replyMsg := &waE2E.Message{
			Conversation: proto.String(text),
		}
		_, err := b.client.SendMessage(b.ctx, chatJID, replyMsg)
		return err
	}

	replyQuoteFn := func(text string) error {
		replyMsg := &waE2E.Message{
			ExtendedTextMessage: &waE2E.ExtendedTextMessage{
				Text: proto.String(text),
				ContextInfo: &waE2E.ContextInfo{
					StanzaID:      proto.String(v.Info.ID),
					Participant:   proto.String(v.Info.Sender.String()),
					QuotedMessage: v.Message,
				},
			},
		}
		_, err := b.client.SendMessage(b.ctx, chatJID, replyMsg)
		return err
	}

	reactFn := func(emoji string) error {
		reactMsg := b.client.BuildReaction(chatJID, v.Info.Sender, v.Info.ID, emoji)
		_, err := b.client.SendMessage(b.ctx, chatJID, reactMsg)
		return err
	}

	deleteFn := func() error {
		_, err := b.client.RevokeMessage(b.ctx, chatJID, v.Info.ID)
		return err
	}

	senderName := v.Info.PushName
	if senderName == "" {
		senderName = v.Info.Sender.User
	}

	isGroup := v.Info.Chat.Server == "g.us"

	ctx := &plugins.Context{
		Message:    msg,
		Sender:     v.Info.Sender.String(),
		SenderName: senderName,
		IsGroup:    isGroup,
		Chat:       chatJID.String(),
		Args:       args,
		Prefix:     prefix,
		Reply:      replyFn,
		ReplyQuote: replyQuoteFn,
		React:      reactFn,
		Delete:     deleteFn,
	}

	if !b.plugins.Dispatch(cmdName, ctx) {
		replyFn("Unknown command. Use " + prefix + "menu to see available commands.")
	}
}
