# Whatskel Bot

🤖 A fast, modular WhatsApp bot written in Go using the `whatsmeow` library and `gopher-lua` for dynamic plugins.

## Features
- **Core in Go**: High performance and concurrent handling of WhatsApp events.
- **Dynamic Lua Plugins**: Create, update, and modify bot commands in Lua without recompiling the main bot.
- **SQLite Database**: Persistent session storage.

## Documentation and API
To understand how to create plugins and the API structure for Whatskel, please read the [WIKI.md](./WIKI.md).

## Installation

### Requirements
- Go 1.20 or newer

### Build from source
```bash
git clone https://github.com/COXKPER/Whatskel.git
cd Whatskel
go build -o bot_bin
./bot_bin
```

When you start the bot for the first time, it will generate a QR code in the terminal. Scan this code using your WhatsApp app to link the bot to your account.

## License
This project is licensed under the GNU General Public License v3.0 (GPLv3). See the `LICENSE` file for details.
