package bot

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mdp/qrterminal/v3"
	"github.com/neoncorp/Whatskel/config"
	"github.com/neoncorp/Whatskel/plugins"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types"
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

// parseTargetJID accepts either a full JID ("628xxx@s.whatsapp.net" /
// "120363xxx@g.us") or a bare phone number ("628xxx"), defaulting bare
// numbers to the standard user server.
func parseTargetJID(raw string) (types.JID, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return types.JID{}, fmt.Errorf("empty JID/number")
	}
	if !strings.Contains(raw, "@") {
		raw = raw + "@s.whatsapp.net"
	}
	return types.ParseJID(raw)
}

// uploadAndSendImage uploads local image bytes and sends an ImageMessage
// to targetJID.
func (b *Bot) uploadAndSendImage(targetJID types.JID, path, caption string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read image file: %w", err)
	}

	uploaded, err := b.client.Upload(b.ctx, data, whatsmeow.MediaImage)
	if err != nil {
		return fmt.Errorf("failed to upload image: %w", err)
	}

	mimetype := http.DetectContentType(data)

	imgMsg := &waE2E.Message{
		ImageMessage: &waE2E.ImageMessage{
			Caption:       proto.String(caption),
			Mimetype:      proto.String(mimetype),
			URL:           proto.String(uploaded.URL),
			DirectPath:    proto.String(uploaded.DirectPath),
			MediaKey:      uploaded.MediaKey,
			FileEncSHA256: uploaded.FileEncSHA256,
			FileSHA256:    uploaded.FileSHA256,
			FileLength:    proto.Uint64(uploaded.FileLength),
		},
	}

	_, err = b.client.SendMessage(b.ctx, targetJID, imgMsg)
	return err
}

// uploadAndSendSticker uploads local sticker bytes (expects a valid .webp)
// and sends a StickerMessage to targetJID.
func (b *Bot) uploadAndSendSticker(targetJID types.JID, path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read sticker file: %w", err)
	}

	uploaded, err := b.client.Upload(b.ctx, data, whatsmeow.MediaImage)
	if err != nil {
		return fmt.Errorf("failed to upload sticker: %w", err)
	}

	stickerMsg := &waE2E.Message{
		StickerMessage: &waE2E.StickerMessage{
			Mimetype:      proto.String("image/webp"),
			URL:           proto.String(uploaded.URL),
			DirectPath:    proto.String(uploaded.DirectPath),
			MediaKey:      uploaded.MediaKey,
			FileEncSHA256: uploaded.FileEncSHA256,
			FileSHA256:    uploaded.FileSHA256,
			FileLength:    proto.Uint64(uploaded.FileLength),
		},
	}

	_, err = b.client.SendMessage(b.ctx, targetJID, stickerMsg)
	return err
}

// getMediaType returns a human-readable media type string for a message.
// Returns empty string if the message contains no downloadable media.
func getMediaType(msg *waE2E.Message) string {
	if msg == nil {
		return ""
	}
	switch {
	case msg.ImageMessage != nil:
		return "image"
	case msg.VideoMessage != nil:
		return "video"
	case msg.AudioMessage != nil:
		return "audio"
	case msg.DocumentMessage != nil:
		return "document"
	case msg.StickerMessage != nil:
		return "sticker"
	default:
		return ""
	}
}

// hasMedia checks whether the message itself contains downloadable media.
func hasMedia(msg *waE2E.Message) bool {
	return getMediaType(msg) != ""
}

// getQuotedMessage extracts the quoted message from different message types.
func getQuotedMessage(msg *waE2E.Message) *waE2E.Message {
	if msg == nil {
		return nil
	}
	// ExtendedTextMessage (text reply quoting something)
	if ext := msg.GetExtendedTextMessage(); ext != nil {
		if ci := ext.GetContextInfo(); ci != nil {
			return ci.QuotedMessage
		}
	}
	// ImageMessage with caption sent as reply
	if img := msg.GetImageMessage(); img != nil {
		if ci := img.GetContextInfo(); ci != nil {
			return ci.QuotedMessage
		}
	}
	// VideoMessage with caption sent as reply
	if vid := msg.GetVideoMessage(); vid != nil {
		if ci := vid.GetContextInfo(); ci != nil {
			return ci.QuotedMessage
		}
	}
	return nil
}

// getMediaExtension returns a file extension based on the media type.
func getMediaExtension(mediaType string) string {
	switch mediaType {
	case "image":
		return ".jpg"
	case "video":
		return ".mp4"
	case "audio":
		return ".ogg"
	case "document":
		return ".bin"
	case "sticker":
		return ".webp"
	default:
		return ".bin"
	}
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
	// Also extract text from image/video/document captions
	if msg == "" {
		if v.Message.GetImageMessage() != nil {
			msg = v.Message.GetImageMessage().GetCaption()
		} else if v.Message.GetVideoMessage() != nil {
			msg = v.Message.GetVideoMessage().GetCaption()
		} else if v.Message.GetDocumentMessage() != nil {
			msg = v.Message.GetDocumentMessage().GetCaption()
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

	// Sends an image to the CURRENT chat (same chat the command was invoked in).
	replyImageFn := func(path, caption string) error {
		return b.uploadAndSendImage(chatJID, path, caption)
	}

	// Sends a sticker to the CURRENT chat.
	replyStickerFn := func(path string) error {
		return b.uploadAndSendSticker(chatJID, path)
	}

	// Sends a plain text message to an ARBITRARY user, independent of the
	// chat the command was invoked in. Accepts a bare number ("628xxx")
	// or a full JID ("628xxx@s.whatsapp.net").
	sendPrivateMessageFn := func(targetRaw, text string) error {
		targetJID, err := parseTargetJID(targetRaw)
		if err != nil {
			return fmt.Errorf("invalid JID/number %q: %w", targetRaw, err)
		}
		privMsg := &waE2E.Message{
			Conversation: proto.String(text),
		}
		_, err = b.client.SendMessage(b.ctx, targetJID, privMsg)
		return err
	}

	senderName := v.Info.PushName
	if senderName == "" {
		senderName = v.Info.Sender.User
	}

	isGroup := v.Info.Chat.Server == "g.us"

	// --- Media bridge ---
	// Detect media on the message itself
	mediaAvailable := hasMedia(v.Message)
	mediaType := getMediaType(v.Message)

	// Detect media on quoted message
	quotedMsg := getQuotedMessage(v.Message)
	quotedMediaAvailable := hasMedia(quotedMsg)
	quotedMediaType := getMediaType(quotedMsg)

	// DownloadMedia: downloads the media attached to this message,
	// saves it to a temp file, and returns the file path.
	downloadMediaFn := func() (string, error) {
		if !mediaAvailable {
			return "", fmt.Errorf("no media attached to this message")
		}
		data, err := b.client.DownloadAny(b.ctx, v.Message)
		if err != nil {
			return "", fmt.Errorf("failed to download media: %w", err)
		}
		ext := getMediaExtension(mediaType)
		tmpFile, err := os.CreateTemp("", "whatskel-media-*"+ext)
		if err != nil {
			return "", fmt.Errorf("failed to create temp file: %w", err)
		}
		defer tmpFile.Close()
		if _, err := tmpFile.Write(data); err != nil {
			return "", fmt.Errorf("failed to write media: %w", err)
		}
		absPath, _ := filepath.Abs(tmpFile.Name())
		return absPath, nil
	}

	// DownloadQuotedMedia: downloads the media from the quoted message.
	downloadQuotedMediaFn := func() (string, error) {
		if !quotedMediaAvailable || quotedMsg == nil {
			return "", fmt.Errorf("no media in quoted message")
		}
		data, err := b.client.DownloadAny(b.ctx, quotedMsg)
		if err != nil {
			return "", fmt.Errorf("failed to download quoted media: %w", err)
		}
		ext := getMediaExtension(quotedMediaType)
		tmpFile, err := os.CreateTemp("", "whatskel-quoted-*"+ext)
		if err != nil {
			return "", fmt.Errorf("failed to create temp file: %w", err)
		}
		defer tmpFile.Close()
		if _, err := tmpFile.Write(data); err != nil {
			return "", fmt.Errorf("failed to write quoted media: %w", err)
		}
		absPath, _ := filepath.Abs(tmpFile.Name())
		return absPath, nil
	}

	ctx := &plugins.Context{
		Message:             msg,
		Sender:              v.Info.Sender.String(),
		SenderName:          senderName,
		IsGroup:             isGroup,
		Chat:                chatJID.String(),
		Args:                args,
		Prefix:              prefix,
		Reply:               replyFn,
		ReplyQuote:          replyQuoteFn,
		React:               reactFn,
		Delete:              deleteFn,
		ReplyImage:          replyImageFn,
		ReplySticker:        replyStickerFn,
		SendPrivateMessage:  sendPrivateMessageFn,
		HasMedia:            mediaAvailable,
		HasQuotedMedia:      quotedMediaAvailable,
		MediaType:           mediaType,
		QuotedMediaType:     quotedMediaType,
		DownloadMedia:       downloadMediaFn,
		DownloadQuotedMedia: downloadQuotedMediaFn,
	}

	if !b.plugins.Dispatch(cmdName, ctx) {
		replyFn("Unknown command. Use " + prefix + "menu to see available commands.")
	}
}