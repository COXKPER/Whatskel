# Whatskel Wiki — API Reference & Architecture Guide

> Complete guide to understanding Whatskel Bot's internal architecture, plugin system, and the full API available to plugins.

---

## Table of Contents

- [Architecture](#architecture)
  - [Flow Diagram](#flow-diagram)
  - [Go Core — `bot` package](#go-core--bot-package)
  - [Lua Loader — `plugins` package](#lua-loader--plugins-package)
  - [Configuration — `config` package](#configuration--config-package)
- [Lua API Reference](#lua-api-reference)
  - [Context Object (`ctx`)](#context-object-ctx)
  - [Properties](#properties)
  - [Methods](#methods)
- [Built-in Features](#built-in-features)
  - [Auto-Reconnect](#auto-reconnect)
  - [Auto-Reject Calls](#auto-reject-calls)
  - [Ignore Self Messages](#ignore-self-messages)
- [Writing a Plugin](#writing-a-plugin)
  - [Basic Plugin Structure](#basic-plugin-structure)
  - [Example: Simple Command](#example-simple-command)
  - [Example: Command with Arguments](#example-command-with-arguments)
  - [Example: Personal Greeting](#example-personal-greeting)
  - [Example: Group-Only Command](#example-group-only-command)
  - [Example: Self-Destructing Command](#example-self-destructing-command)
  - [Example: Multiple Commands in One File](#example-multiple-commands-in-one-file)
  - [Example: Sending Media (Image/Sticker)](#example-sending-media-imagesticker)
  - [Example: Owner-Gated Private Messaging](#example-owner-gated-private-messaging)
- [Bot Configuration](#bot-configuration)
- [FAQ & Troubleshooting](#faq--troubleshooting)

---

## Architecture

Whatskel is built on a modular architecture separating the **Go core** (WhatsApp connection handling) from **Lua plugins** (command logic). This lets developers add features without recompiling the binary.

### Flow Diagram

```
┌─────────────────────────────────────────────────────────┐
│                     WhatsApp Server                     │
└────────────────────────┬────────────────────────────────┘
                         │  WebSocket (whatsmeow)
                         ▼
┌─────────────────────────────────────────────────────────┐
│                   bot.go (Go Core)                      │
│                                                         │
│  ┌──────────────┐  ┌──────────────┐  ┌───────────────┐ │
│  │ Event Handler │  │ Auto-Reconn. │  │ Call Rejector  │ │
│  │ (Message)     │  │ (Disconnect) │  │ (CallOffer)   │ │
│  └──────┬───────┘  └──────────────┘  └───────────────┘ │
│         │                                               │
│         ▼                                               │
│  ┌──────────────────────────────────────────┐           │
│  │ Command Parser                           │           │
│  │ prefix + command + args                  │           │
│  └──────┬───────────────────────────────────┘           │
│         │                                               │
│         ▼                                               │
│  ┌──────────────────────────────────────────┐           │
│  │ Context Builder                          │           │
│  │ Message, Sender, SenderName, Chat, Args, │           │
│  │ Prefix, IsGroup, HasMedia, MediaType,     │           │
│  │ HasQuotedMedia, QuotedMediaType,          │           │
│  │ Reply, ReplyQuote, React, DeleteMessage,  │           │
│  │ ReplyImage, ReplySticker,                │           │
│  │ SendPrivateMessage, DownloadMedia,        │           │
│  │ DownloadQuotedMedia                       │           │
│  └──────┬───────────────────────────────────┘           │
└─────────┼───────────────────────────────────────────────┘
          │
          ▼
┌─────────────────────────────────────────────────────────┐
│               loader.go (Plugin Loader)                 │
│                                                         │
│  ┌───────────────────────────────────────────────────┐  │
│  │ Lua VM (gopher-lua)                               │  │
│  │                                                   │  │
│  │  ┌──────────┐ ┌──────────┐ ┌──────────────────┐  │  │
│  │  │ Menu.lua │ │ Utils.lua│ │ YourPlugin.lua   │  │  │
│  │  └──────────┘ └──────────┘ └──────────────────┘  │  │
│  │                                                   │  │
│  │  export("cmd", function(ctx) ... end)             │  │
│  └───────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────┘
```

### Go Core — `bot` package

Main file: [`bot/bot.go`](./bot/bot.go)

This module is responsible for:

| Function | Description |
|---|---|
| `New(cfg)` | Initializes the WhatsApp client, SQLite database, and plugin loader |
| `Start()` | Registers event handlers, shows the QR code, and connects to WhatsApp |
| `Stop()` | Closes the connection, cleans up Lua state, and cancels the context |
| `handleMessage(v)` | Receives incoming messages, parses commands, builds the `Context`, and dispatches to Lua |

The event handler covers three event types:
- **`events.Message`** → forwarded to `handleMessage()` for command parsing.
- **`events.Disconnected`** → triggers auto-reconnect with exponential backoff.
- **`events.CallOffer`** → automatically rejects incoming calls.

Two upload/send helpers back the media API exposed to Lua:
- **`uploadAndSendImage(targetJID, path, caption)`** — reads a local file, uploads it via `client.Upload(ctx, data, whatsmeow.MediaImage)`, detects its MIME type, and sends a `waE2E.ImageMessage`.
- **`uploadAndSendSticker(targetJID, path)`** — same upload flow, sends a `waE2E.StickerMessage` (expects a valid `.webp`).
- **`parseTargetJID(raw)`** — normalizes a bare phone number (e.g. `"628123456789"`) into a full JID by appending `@s.whatsapp.net` when no `@` is present, otherwise parses the string as-is via `types.ParseJID`.

Media detection and download helpers:
- **`getMediaType(msg)`** — returns the media type string (`"image"`, `"video"`, `"audio"`, `"document"`, `"sticker"`, or `""`).
- **`hasMedia(msg)`** — returns `true` if the message contains any downloadable media.
- **`getQuotedMessage(msg)`** — extracts the `QuotedMessage` from `ContextInfo` across `ExtendedTextMessage`, `ImageMessage`, and `VideoMessage`.
- **`getMediaExtension(mediaType)`** — maps a media type to a file extension (`.jpg`, `.mp4`, `.ogg`, `.webp`, `.bin`).

> **Caption-based command detection:** Commands can be triggered via image/video/document captions. If a user sends an image with the caption `.sticker`, the bot extracts the caption, parses the command, and the image is accessible via `ctx:DownloadMedia()`.

### Lua Loader — `plugins` package

Main file: [`plugins/loader.go`](./plugins/loader.go)

This module is responsible for:

| Function | Description |
|---|---|
| `NewLoader(dir)` | Creates a new Lua VM, registers the Context metatable, and sets up `export()` |
| `LoadAll()` | Loads every `.lua` file inside the plugin directory |
| `Dispatch(cmd, ctx)` | Finds the matching command and executes its Lua handler |
| `GetCommands()` | Returns the list of all registered command names |
| `Close()` | Safely closes the Lua state |

**How Context works in Lua:**

Context is implemented using `gopher-lua`'s `UserData` and `Metatable`. This differs from a naive approach (injecting into a plain table) because:
- There's no field collision (previously, `Args` could be overwritten by a same-named function).
- Methods are called properly using `:` (colon) syntax.
- It's type-safe — Lua errors out if the first argument isn't a valid Context.

> **Note:** `ReplyImage`, `ReplySticker`, and `SendPrivateMessage` are registered in `plugins/loader.go`'s `contextIndex` alongside `Reply`/`React`/`Delete`, following the same `UserData`/Metatable pattern. Unlike those four (which swallow errors — only logged Go-side), these three **return an error string on failure, or `nil` on success**, so plugins can surface upload/send failures to the user directly.

### Configuration — `config` package

Main file: [`config/config.go`](./config/config.go)

Reads configuration from a TOML file (`config.toml`). See [Bot Configuration](#bot-configuration) for the format.

---

## Lua API Reference

### Context Object (`ctx`)

Every Lua command handler receives one `ctx` argument, containing all information about the incoming message plus methods to interact with WhatsApp.

```lua
export("mycommand", function(ctx)
    -- ctx is a Context object
    -- property access: ctx.PropertyName
    -- method call:      ctx:MethodName(args)
end)
```

### Properties

| Property | Type | Description | Example Value |
|---|---|---|---|
| `ctx.Message` | `string` | Full text of the incoming message (including prefix and command) | `".ping"` |
| `ctx.Sender` | `string` | Sender's WhatsApp JID | `"6281234567890@s.whatsapp.net"` |
| `ctx.SenderName` | `string` | Sender's profile name (PushName); falls back to phone number if unset | `"John Doe"` |
| `ctx.Chat` | `string` | JID of the chat the message was received in (private or group) | `"120363xxx@g.us"` (group) or `"628xxx@s.whatsapp.net"` (private) |
| `ctx.Args` | `string` | Everything after the command name; empty if no arguments | `"hello world"` (from `.echo hello world`) |
| `ctx.Prefix` | `string` | Command prefix configured in `config.toml` | `"."` |
| `ctx.IsGroup` | `boolean` | `true` if the message came from a group, `false` for private chat | `true` |
| `ctx.HasMedia` | `boolean` | `true` if the current message contains downloadable media (image/video/audio/document/sticker) | `true` |
| `ctx.HasQuotedMedia` | `boolean` | `true` if the quoted (replied-to) message contains downloadable media | `false` |
| `ctx.MediaType` | `string` | Type of media on the message: `"image"`, `"video"`, `"audio"`, `"document"`, `"sticker"`, or `""` | `"image"` |
| `ctx.QuotedMediaType` | `string` | Type of media on the quoted message (same possible values as `MediaType`) | `"sticker"` |

### Methods

#### `ctx:Reply(text)`

Sends a plain text message to the chat.

```lua
ctx:Reply("Hello, World!")
```

| Parameter | Type | Required | Description |
|---|---|---|---|
| `text` | `string` | ✅ | Text to send |

---

#### `ctx:ReplyQuote(text)`

Sends a text message quoting the sender's original message. The quoted message appears as a reply bubble in WhatsApp.

```lua
ctx:ReplyQuote("This is a quoted reply!")
```

| Parameter | Type | Required | Description |
|---|---|---|---|
| `text` | `string` | ✅ | Text to send |

**When to use Reply vs ReplyQuote?**
- Use `Reply()` for general informative output (e.g. menus, help text).
- Use `ReplyQuote()` when it matters which message is being answered (e.g. echo, direct answers to a specific question).

---

#### `ctx:React(emoji)`

Adds an emoji reaction to the sender's command message. The emoji appears under the sender's chat bubble.

```lua
ctx:React("👍")
ctx:React("✅")
ctx:React("❌")
```

| Parameter | Type | Required | Description |
|---|---|---|---|
| `emoji` | `string` | ✅ | A single emoji character |

> **Tip:** use a reaction as quick visual feedback before sending a reply — e.g. `ctx:React("⏳")` while processing, then send the result.

---

#### `ctx:DeleteMessage()`

Deletes/revokes the sender's command message. Useful for commands carrying sensitive information.

```lua
ctx:DeleteMessage()
```

> **Note:** the bot can only delete other people's messages in a group if it's a group admin. In private chats it can only delete its own messages.

---

#### `ctx:ReplyImage(path, caption)`

Uploads a local image file and sends it to the **current chat**, with an optional caption.

```lua
ctx:ReplyImage("/path/to/image.jpg", "Look at this!")
```

| Parameter | Type | Required | Description |
|---|---|---|---|
| `path` | `string` | ✅ | Filesystem path to the image, readable by the bot process |
| `caption` | `string` | ➖ | Optional caption; pass `""` for none |

> Under the hood this reads the file, uploads it via `whatsmeow`'s `client.Upload(..., whatsmeow.MediaImage)`, detects the MIME type, and sends a `waE2E.ImageMessage`.

---

#### `ctx:ReplySticker(path)`

Uploads a local `.webp` file and sends it as a sticker to the **current chat**.

```lua
ctx:ReplySticker("/path/to/sticker.webp")
```

| Parameter | Type | Required | Description |
|---|---|---|---|
| `path` | `string` | ✅ | Filesystem path to a valid `.webp` sticker file |

---

#### `ctx:SendPrivateMessage(target, text)`

Sends a plain text message to an **arbitrary user**, independent of the chat the command was invoked in.

```lua
ctx:SendPrivateMessage("628123456789", "Hello from the bot!")
```

| Parameter | Type | Required | Description |
|---|---|---|---|
| `target` | `string` | ✅ | Bare phone number (auto-suffixed with `@s.whatsapp.net`) or a full JID |
| `text` | `string` | ✅ | Message text to send |

> **Security note:** any user able to message the bot could otherwise abuse this to spam or contact arbitrary third parties. Gate plugin commands that wrap this behind an owner/allowlist check on `ctx.Sender` — see [Owner-Gated Private Messaging](#example-owner-gated-private-messaging) below.

---

#### `ctx:DownloadMedia()`

Downloads the media attached to the **current message** (e.g. when a user sends an image with a command as its caption). Saves the decrypted file to a temp directory and returns the absolute file path.

```lua
local path, err = ctx:DownloadMedia()
if err then
    ctx:ReplyQuote("Failed to download: " .. err)
    return
end
-- path is now something like "/tmp/whatskel-media-123456.jpg"
ctx:ReplyQuote("Media downloaded to: " .. path)
```

| Returns | Type | Description |
|---|---|---|
| `path` | `string` or `nil` | Absolute path to the downloaded temp file, or `nil` on error |
| `err` | `string` or `nil` | Error message, or `nil` on success |

> **How it works:** WhatsApp media is end-to-end encrypted. The Go core uses `whatsmeow`'s `client.DownloadAny()` to decrypt and download the media bytes, then writes them to a temp file with the appropriate extension (`.jpg`, `.mp4`, `.ogg`, `.webp`, `.bin`).

> **Tip:** Always check `ctx.HasMedia` before calling `ctx:DownloadMedia()` to avoid unnecessary error handling.

---

#### `ctx:DownloadQuotedMedia()`

Downloads the media from the **quoted (replied-to) message**. This allows a user to reply to an image/video/sticker with a command, and the bot downloads that quoted media.

```lua
local path, err = ctx:DownloadQuotedMedia()
if err then
    ctx:ReplyQuote("Failed to download quoted media: " .. err)
    return
end
ctx:ReplyQuote("Quoted media saved to: " .. path)
```

| Returns | Type | Description |
|---|---|---|
| `path` | `string` or `nil` | Absolute path to the downloaded temp file, or `nil` on error |
| `err` | `string` or `nil` | Error message, or `nil` on success |

> **Tip:** Always check `ctx.HasQuotedMedia` before calling. You can also use `ctx.QuotedMediaType` to know what kind of media it is (`"image"`, `"sticker"`, etc.) before deciding what to do with it.

---

## Built-in Features

### Auto-Reconnect

When the WebSocket connection to WhatsApp drops (e.g. due to network issues), the bot automatically retries using **exponential backoff**:

| Attempt | Delay |
|---|---|
| 1 | 2 seconds |
| 2 | 4 seconds |
| 3 | 8 seconds |
| 4 | 16 seconds |
| 5 | 32 seconds |

If all 5 attempts fail, the bot stops retrying and logs that a manual restart is required.

### Auto-Reject Calls

All incoming calls (voice and video) are automatically rejected, preventing the bot from hanging or being interrupted by call spam.

### Ignore Self Messages

The bot ignores messages sent by itself, avoiding infinite loops when it sends replies.

---

## Writing a Plugin

### Basic Plugin Structure

Every plugin is a `.lua` file stored inside the `plugins/` directory. All `.lua` files are loaded automatically when the bot starts.

```
plugins/
├── Menu.lua          # Built-in menu and commands
├── Greetings.lua     # Greeting plugin
├── Utils.lua         # Utility commands
└── YourPlugin.lua    # Your custom plugin
```

Use the global `export(name, handler)` function to register a command:

```lua
export("commandname", function(ctx)
    -- command logic here
end)
```

- `name` (string): command name (without prefix). Users type `{prefix}{name}` to invoke it.
- `handler` (function): receives one `ctx` (Context) argument.

### Example: Simple Command

```lua
-- plugins/Hello.lua
export("hello", function(ctx)
    ctx:React("👋")
    ctx:Reply("Hello, World! 🌍")
end)
```

**Usage:** `.hello`
**Output:** the bot reacts with 👋 and replies "Hello, World! 🌍"

### Example: Command with Arguments

```lua
-- plugins/Repeat.lua
export("repeat", function(ctx)
    local text = ctx.Args
    if text == "" then
        ctx:React("❓")
        ctx:ReplyQuote("Usage: " .. ctx.Prefix .. "repeat <text to repeat>")
        return
    end

    ctx:React("🔁")
    ctx:ReplyQuote(text .. "\n" .. text .. "\n" .. text)
end)
```

**Usage:** `.repeat hello`
**Output:** the bot reacts with 🔁 and quotes the message while replying with the text repeated 3 times.

### Example: Personal Greeting

```lua
-- plugins/Greet.lua
export("greet", function(ctx)
    local name = ctx.SenderName
    ctx:React("🤝")

    local msg = "Hello, *" .. name .. "*! 👋\n\n"
    msg = msg .. "Welcome to Whatskel Bot.\n"
    msg = msg .. "Type " .. ctx.Prefix .. "menu to see the list of commands."

    ctx:ReplyQuote(msg)
end)
```

**Usage:** `.greet`
**Output:** "Hello, *John Doe*! 👋" — using `ctx.SenderName` for personalization.

### Example: Group-Only Command

```lua
-- plugins/GroupInfo.lua
export("groupinfo", function(ctx)
    if not ctx.IsGroup then
        ctx:React("🚫")
        ctx:ReplyQuote("This command can only be used inside a group!")
        return
    end

    ctx:React("📊")
    local msg = "📊 *Group Info*\n\n"
    msg = msg .. "Chat ID: " .. ctx.Chat .. "\n"
    msg = msg .. "Sender: " .. ctx.SenderName

    ctx:Reply(msg)
end)
```

**Usage:** `.groupinfo`
**Output:** shows group info, or refuses if used in a private chat.

### Example: Self-Destructing Command

```lua
-- plugins/Secret.lua
export("secret", function(ctx)
    -- Delete the user's command so no one else sees it
    ctx:DeleteMessage()

    -- Send the secret reply
    ctx:Reply("🤫 Secret message received! Your command has been deleted.")
end)
```

**Usage:** `.secret`
**Output:** the `.secret` message is deleted from the chat, then the bot replies with a confirmation.

### Example: Multiple Commands in One File

You can register several commands in a single `.lua` file:

```lua
-- plugins/Fun.lua

export("coinflip", function(ctx)
    ctx:React("🪙")
    math.randomseed(os.time())
    local result = math.random(2) == 1 and "🪙 *Heads!*" or "🪙 *Tails!*"
    ctx:ReplyQuote(result)
end)

export("dice", function(ctx)
    ctx:React("🎲")
    math.randomseed(os.time())
    local result = math.random(1, 6)
    ctx:ReplyQuote("🎲 You rolled: *" .. result .. "*")
end)
```

### Example: Sending Media (Image/Sticker)

```lua
-- plugins/Media.lua
export("gambar", function(ctx)
    -- Usage: .gambar <path> | <caption>
    local path, caption = ctx.Args:match("^(.-)%s*|%s*(.*)$")
    if not path then path, caption = ctx.Args, "" end

    if path == "" then
        ctx:React("❓")
        ctx:ReplyQuote("Usage: " .. ctx.Prefix .. "gambar <path> | <optional caption>")
        return
    end

    ctx:React("🖼️")
    local err = ctx:ReplyImage(path, caption)
    if err then
        ctx:ReplyQuote("Failed to send image: " .. err)
    end
end)

export("stiker", function(ctx)
    -- Usage: .stiker <path to a .webp file>
    local path = ctx.Args
    if path == "" then
        ctx:React("❓")
        ctx:ReplyQuote("Usage: " .. ctx.Prefix .. "stiker <path to .webp>")
        return
    end

    ctx:React("🏷️")
    local err = ctx:ReplySticker(path)
    if err then
        ctx:ReplyQuote("Failed to send sticker: " .. err)
    end
end)
```

**Usage:** `.gambar /home/bot/photo.jpg | Nice view` or `.stiker /home/bot/stickers/wave.webp`

### Example: Owner-Gated Private Messaging

Because `ctx:SendPrivateMessage` can message *any* number, restrict it to the bot owner to prevent abuse:

```lua
-- plugins/Admin.lua
export("pm", function(ctx)
    local OWNER_JID = "REPLACE_WITH_YOUR_JID@s.whatsapp.net"
    if ctx.Sender ~= OWNER_JID then
        ctx:React("🚫")
        ctx:ReplyQuote("This command is owner-only.")
        return
    end

    -- Usage: .pm <number> <message>
    local nomor, pesan = ctx.Args:match("^(%S+)%s+(.+)$")
    if not nomor or not pesan then
        ctx:React("❓")
        ctx:ReplyQuote("Usage: " .. ctx.Prefix .. "pm <number> <message>\nExample: " .. ctx.Prefix .. "pm 628123456789 Hi!")
        return
    end

    ctx:React("📩")
    local err = ctx:SendPrivateMessage(nomor, pesan)
    if err then
        ctx:ReplyQuote("Failed to send: " .. err)
        return
    end
    ctx:ReplyQuote("✅ Message sent to " .. nomor .. ".")
end)
```

### Example: Downloading and Re-sending Media

Download an image (or quoted media) and re-send it as a sticker, or vice versa:

```lua
-- plugins/MediaTools.lua

-- Save media: reply to an image/video/sticker with .save
export("save", function(ctx)
    local path, err

    if ctx.HasMedia then
        path, err = ctx:DownloadMedia()
    elseif ctx.HasQuotedMedia then
        path, err = ctx:DownloadQuotedMedia()
    else
        ctx:React("❌")
        ctx:ReplyQuote("No media found! Send or reply to a media file.")
        return
    end

    if err then
        ctx:React("❌")
        ctx:ReplyQuote("Download failed: " .. err)
        return
    end

    local mediaType = ctx.HasMedia and ctx.MediaType or ctx.QuotedMediaType
    ctx:React("✅")
    ctx:ReplyQuote("✅ Media (" .. mediaType .. ") saved to:\n" .. path)
end)

-- Convert image to sticker: send/reply to an image with .tosticker
export("tosticker", function(ctx)
    local path, err

    if ctx.HasMedia and ctx.MediaType == "image" then
        path, err = ctx:DownloadMedia()
    elseif ctx.HasQuotedMedia and ctx.QuotedMediaType == "image" then
        path, err = ctx:DownloadQuotedMedia()
    else
        ctx:React("❌")
        ctx:ReplyQuote("Reply to or send an image with this command.")
        return
    end

    if err then
        ctx:React("❌")
        ctx:ReplyQuote("Download failed: " .. err)
        return
    end

    ctx:React("🏷️")
    local sendErr = ctx:ReplySticker(path)
    if sendErr then
        ctx:ReplyQuote("Failed to send sticker: " .. sendErr)
    end
end)
```

**Usage:** `.save` (reply to any media) or `.tosticker` (reply to/send an image with caption)

---

## Bot Configuration

Bot configuration is read from `config.toml`:

```toml
[bot]
prefix = "."                        # Command trigger prefix
session_path = "whatsapp-session.db" # WhatsApp session file path
db_path = "whatsapp-store.db"        # SQLite database path

[plugins]
directory = "plugins"                # Directory holding .lua files
```

| Key | Default | Description |
|---|---|---|
| `bot.prefix` | `"."` | Character or string required before a command name |
| `bot.session_path` | `"whatsapp-session.db"` | WhatsApp session storage location |
| `bot.db_path` | `"whatsapp-store.db"` | Main database (device store) location |
| `plugins.directory` | `"plugins"` | Directory containing `.lua` plugin files |

---

## FAQ & Troubleshooting

### The bot doesn't respond to my command
- Make sure you're using the correct prefix (check `config.toml`).
- Make sure the command name matches what was passed to `export()` in the `.lua` file.
- Check the terminal log for "Error loading plugin" or "Error executing command".

### The QR code doesn't show up
- Make sure `db_path` is writable by the bot.
- Delete old database files if you want to log in again: `make clean`.

### The plugin doesn't load
- Make sure the plugin file has a `.lua` extension.
- Make sure the file is inside the directory configured in `plugins.directory`.
- Check for Lua syntax errors — they'll show up in the terminal log.

### The bot keeps disconnecting
- The bot already has auto-reconnect with exponential backoff (up to 5 attempts).
- If it still fails, check your internet connection.
- Make sure only one bot instance is running per session (two instances sharing the same session will kick each other).

### Media (image/sticker) commands fail with an error
- Make sure the file path passed to `ctx:ReplyImage`/`ctx:ReplySticker` is readable by the process the bot runs as (not just readable by you).
- Stickers must be valid `.webp` files — other formats will upload but may not render as stickers on WhatsApp.
- If the returned error string mentions upload failures, double check network access to WhatsApp's media servers and that the file isn't corrupted/empty.

### How do I add a new command without restarting?
- Currently not supported — plugins are loaded once at startup. Restart the bot after adding or editing `.lua` files.

### How do I trigger a command with an image caption?
- Send an image with a caption like `.sticker` or `.save`. The bot extracts the caption as the command text, and the image is accessible via `ctx:DownloadMedia()` inside the Lua handler.

### Media download returns an error
- Make sure the media hasn't expired — WhatsApp media URLs expire after some time.
- Check `ctx.HasMedia` or `ctx.HasQuotedMedia` before calling the download method.
- Network issues can also cause download failures. The error message will contain details.