<div align="center">

# 🤖 Whatskel

**A fast, modular WhatsApp bot built with Go and Lua plugins.**

[![License: GPL v3](https://img.shields.io/badge/License-GPLv3-blue.svg)](https://www.gnu.org/licenses/gpl-3.0)
[![Go Version](https://img.shields.io/badge/Go-1.26.4-00ADD8?logo=go)](https://go.dev/)
[![Release](https://img.shields.io/github/v/release/COXKPER/Whatskel?include_prereleases)](https://github.com/COXKPER/Whatskel/releases)

Whatskel is a WhatsApp bot framework that separates its **core** (Go) from its **command logic** (Lua), enabling you to create, modify, and deploy new commands without recompiling the binary.

</div>

---

## ✨ Features

| Feature | Description |
|---|---|
| 🔌 **Dynamic Lua Plugins** | Write commands in Lua — no recompilation needed |
| 🔄 **Auto-Reconnect** | Automatically reconnects on disconnect with exponential backoff |
| 📵 **Auto-Reject Calls** | Incoming voice/video calls are instantly declined |
| 💬 **Reply with Quote** | Reply to messages while quoting the original message |
| 😀 **Message Reactions** | React to messages with emoji |
| 🗑️ **Delete Messages** | Revoke/delete command messages programmatically |
| 👤 **Sender Name Access** | Access sender's display name for personalized responses |
| 🏘️ **Group Detection** | Distinguish between group and private messages |
| 💾 **SQLite Persistence** | Session and device data stored locally |

## 📖 Documentation

> **📚 [Read the full Wiki →](./WIKI.md)**
>
> The Wiki contains the complete **API reference**, **architecture guide**, **plugin creation tutorial** with examples, **configuration details**, and **troubleshooting FAQ**.

## 🚀 Quick Start

### Requirements

- **Go** 1.26.4 or newer

### Build from Source

```bash
git clone https://github.com/COXKPER/Whatskel.git
cd Whatskel
go build -o whatskel .
```

### Run

```bash
./whatskel -config config.toml
```

On first run, a QR code will be displayed in the terminal. Scan it using your WhatsApp app (**Linked Devices**) to authenticate the bot.

### Using Make

```bash
make build   # Build the binary
make run     # Build and run
make clean   # Remove binary and database files
```

## ⚙️ Configuration

Edit `config.toml` to customize the bot:

```toml
[bot]
prefix = "."                         # Command prefix
session_path = "whatsapp-session.db"  # Session storage
db_path = "whatsapp-store.db"        # Device store

[plugins]
directory = "plugins"                # Lua plugins directory
```

## 🔌 Creating Plugins

Create a `.lua` file inside the `plugins/` directory:

```lua
-- plugins/Hello.lua
export("hello", function(ctx)
    ctx:React("👋")
    ctx:ReplyQuote("Hello, " .. ctx.SenderName .. "! 🌍")
end)
```

That's it! The bot loads all `.lua` files automatically on startup.

**Available Context API:**

| Properties | Methods |
|---|---|
| `ctx.Message` | `ctx:Reply(text)` |
| `ctx.Sender` | `ctx:ReplyQuote(text)` |
| `ctx.SenderName` | `ctx:React(emoji)` |
| `ctx.Chat` | `ctx:DeleteMessage()` |
| `ctx.Args` | |
| `ctx.Prefix` | |
| `ctx.IsGroup` | |

> See the **[Wiki](./WIKI.md)** for the full API reference and more plugin examples.

## 📁 Project Structure

```
Whatskel/
├── main.go              # Entry point
├── bot/
│   └── bot.go           # WhatsApp client, event handling, context building
├── plugins/
│   ├── loader.go        # Lua VM, UserData metatables, command dispatch
│   └── Menu.lua         # Default commands (menu, ping, echo)
├── config/
│   └── config.go        # TOML config parser
├── config.toml          # Bot configuration
├── WIKI.md              # Full API documentation
├── LICENSE              # GPLv3
└── .github/
    └── workflows/
        └── release.yml  # Automated cross-platform release builds
```

## 📦 Releases

Pre-built binaries for **Linux** and **Windows** are available on the [Releases](https://github.com/COXKPER/Whatskel/releases) page. Each tagged version automatically triggers a GitHub Actions workflow that compiles and publishes the binaries.

## 📄 License

This project is licensed under the **GNU General Public License v3.0**. See the [`LICENSE`](./LICENSE) file for details.
