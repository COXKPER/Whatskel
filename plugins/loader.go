package plugins

import (
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/yuin/gopher-lua"
)

type Context struct {
	Message             string
	Sender              string
	SenderName          string
	IsGroup             bool
	Chat                string
	Args                string
	Prefix              string
	Reply               func(text string) error
	ReplyQuote          func(text string) error
	React               func(emoji string) error
	Delete              func() error
	ReplyImage          func(path, caption string) error
	ReplySticker        func(path string) error
	SendPrivateMessage  func(target, text string) error
	HasMedia            bool
	HasQuotedMedia      bool
	MediaType           string
	QuotedMediaType     string
	DownloadMedia       func() (string, error)
	DownloadQuotedMedia func() (string, error)
}

type Loader struct {
	L        *lua.LState
	commands map[string]*lua.LFunction
	dir      string
}

const contextLuaTypeName = "context"

func registerContextType(L *lua.LState) {
	mt := L.NewTypeMetatable(contextLuaTypeName)
	L.SetField(mt, "__index", L.NewFunction(contextIndex))
}

func checkContext(L *lua.LState) *Context {
	ud := L.CheckUserData(1)
	if v, ok := ud.Value.(*Context); ok {
		return v
	}
	L.ArgError(1, "context expected")
	return nil
}

func contextIndex(L *lua.LState) int {
	ctx := checkContext(L)
	field := L.CheckString(2)

	switch field {
	case "Message":
		L.Push(lua.LString(ctx.Message))
	case "Sender":
		L.Push(lua.LString(ctx.Sender))
	case "SenderName":
		L.Push(lua.LString(ctx.SenderName))
	case "IsGroup":
		L.Push(lua.LBool(ctx.IsGroup))
	case "Chat":
		L.Push(lua.LString(ctx.Chat))
	case "Args":
		L.Push(lua.LString(ctx.Args))
	case "Prefix":
		L.Push(lua.LString(ctx.Prefix))
	case "Reply":
		L.Push(L.NewFunction(contextReply))
	case "ReplyQuote":
		L.Push(L.NewFunction(contextReplyQuote))
	case "React":
		L.Push(L.NewFunction(contextReact))
	case "DeleteMessage":
		L.Push(L.NewFunction(contextDelete))
	case "ReplyImage":
		L.Push(L.NewFunction(contextReplyImage))
	case "ReplySticker":
		L.Push(L.NewFunction(contextReplySticker))
	case "SendPrivateMessage":
		L.Push(L.NewFunction(contextSendPrivateMessage))
	case "HasMedia":
		L.Push(lua.LBool(ctx.HasMedia))
	case "HasQuotedMedia":
		L.Push(lua.LBool(ctx.HasQuotedMedia))
	case "MediaType":
		L.Push(lua.LString(ctx.MediaType))
	case "QuotedMediaType":
		L.Push(lua.LString(ctx.QuotedMediaType))
	case "DownloadMedia":
		L.Push(L.NewFunction(contextDownloadMedia))
	case "DownloadQuotedMedia":
		L.Push(L.NewFunction(contextDownloadQuotedMedia))
	default:
		L.Push(lua.LNil)
	}
	return 1
}

func contextReply(L *lua.LState) int {
	ctx := checkContext(L)
	text := L.CheckString(2)
	if ctx.Reply != nil {
		if err := ctx.Reply(text); err != nil {
			log.Printf("Reply error: %v", err)
		}
	}
	return 0
}

func contextReplyQuote(L *lua.LState) int {
	ctx := checkContext(L)
	text := L.CheckString(2)
	if ctx.ReplyQuote != nil {
		if err := ctx.ReplyQuote(text); err != nil {
			log.Printf("ReplyQuote error: %v", err)
		}
	}
	return 0
}

func contextReact(L *lua.LState) int {
	ctx := checkContext(L)
	emoji := L.CheckString(2)
	if ctx.React != nil {
		if err := ctx.React(emoji); err != nil {
			log.Printf("React error: %v", err)
		}
	}
	return 0
}

func contextDelete(L *lua.LState) int {
	ctx := checkContext(L)
	if ctx.Delete != nil {
		if err := ctx.Delete(); err != nil {
			log.Printf("DeleteMessage error: %v", err)
		}
	}
	return 0
}

// contextReplyImage implements ctx:ReplyImage(path, caption).
// caption is optional — omitting it (or passing "") sends no caption.
func contextReplyImage(L *lua.LState) int {
	ctx := checkContext(L)
	path := L.CheckString(2)
	caption := L.OptString(3, "")

	if ctx.ReplyImage == nil {
		L.Push(lua.LString("ReplyImage not available"))
		return 1
	}

	if err := ctx.ReplyImage(path, caption); err != nil {
		log.Printf("ReplyImage error: %v", err)
		L.Push(lua.LString(err.Error()))
		return 1
	}
	return 0
}

// contextReplySticker implements ctx:ReplySticker(path).
func contextReplySticker(L *lua.LState) int {
	ctx := checkContext(L)
	path := L.CheckString(2)

	if ctx.ReplySticker == nil {
		L.Push(lua.LString("ReplySticker not available"))
		return 1
	}

	if err := ctx.ReplySticker(path); err != nil {
		log.Printf("ReplySticker error: %v", err)
		L.Push(lua.LString(err.Error()))
		return 1
	}
	return 0
}

// contextSendPrivateMessage implements ctx:SendPrivateMessage(target, text).
func contextSendPrivateMessage(L *lua.LState) int {
	ctx := checkContext(L)
	target := L.CheckString(2)
	text := L.CheckString(3)

	if ctx.SendPrivateMessage == nil {
		L.Push(lua.LString("SendPrivateMessage not available"))
		return 1
	}

	if err := ctx.SendPrivateMessage(target, text); err != nil {
		log.Printf("SendPrivateMessage error: %v", err)
		L.Push(lua.LString(err.Error()))
		return 1
	}
	return 0
}

// contextDownloadMedia implements ctx:DownloadMedia().
// Returns (filepath, nil) on success, or (nil, error_string) on failure.
func contextDownloadMedia(L *lua.LState) int {
	ctx := checkContext(L)
	if ctx.DownloadMedia == nil {
		L.Push(lua.LNil)
		L.Push(lua.LString("DownloadMedia not available"))
		return 2
	}
	path, err := ctx.DownloadMedia()
	if err != nil {
		log.Printf("DownloadMedia error: %v", err)
		L.Push(lua.LNil)
		L.Push(lua.LString(err.Error()))
		return 2
	}
	L.Push(lua.LString(path))
	L.Push(lua.LNil)
	return 2
}

// contextDownloadQuotedMedia implements ctx:DownloadQuotedMedia().
// Returns (filepath, nil) on success, or (nil, error_string) on failure.
func contextDownloadQuotedMedia(L *lua.LState) int {
	ctx := checkContext(L)
	if ctx.DownloadQuotedMedia == nil {
		L.Push(lua.LNil)
		L.Push(lua.LString("DownloadQuotedMedia not available"))
		return 2
	}
	path, err := ctx.DownloadQuotedMedia()
	if err != nil {
		log.Printf("DownloadQuotedMedia error: %v", err)
		L.Push(lua.LNil)
		L.Push(lua.LString(err.Error()))
		return 2
	}
	L.Push(lua.LString(path))
	L.Push(lua.LNil)
	return 2
}

func NewLoader(dir string) *Loader {
	L := lua.NewState()
	ld := &Loader{
		L:        L,
		commands: make(map[string]*lua.LFunction),
		dir:      dir,
	}

	registerContextType(L)

	L.SetGlobal("export", L.NewFunction(func(L *lua.LState) int {
		name := L.CheckString(1)
		fn := L.CheckFunction(2)
		ld.commands[name] = fn
		return 0
	}))

	return ld
}

func (l *Loader) LoadAll() error {
	entries, err := os.ReadDir(l.dir)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".lua") {
			path := filepath.Join(l.dir, entry.Name())
			if err := l.L.DoFile(path); err != nil {
				log.Printf("Error loading plugin %s: %v", entry.Name(), err)
			} else {
				log.Printf("Loaded plugin: %s", entry.Name())
			}
		}
	}

	return nil
}

func (l *Loader) Dispatch(cmd string, ctx *Context) bool {
	fn, ok := l.commands[cmd]
	if !ok {
		return false
	}

	ud := l.L.NewUserData()
	ud.Value = ctx
	l.L.SetMetatable(ud, l.L.GetTypeMetatable(contextLuaTypeName))

	l.L.Push(fn)
	l.L.Push(ud)
	if err := l.L.PCall(1, 0, nil); err != nil {
		log.Printf("Error executing command %s: %v", cmd, err)
		return false
	}
	return true
}

// GetCommands returns a list of all registered command names.
func (l *Loader) GetCommands() []string {
	cmds := make([]string, 0, len(l.commands))
	for name := range l.commands {
		cmds = append(cmds, name)
	}
	return cmds
}

// Close shuts down the Lua state gracefully.
func (l *Loader) Close() {
	l.L.Close()
}