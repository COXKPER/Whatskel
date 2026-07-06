# Whatskel Wiki — API Reference & Architecture Guide

> Panduan lengkap untuk memahami arsitektur internal, sistem plugin, dan seluruh API yang tersedia pada Whatskel Bot.

---

## Table of Contents

- [Arsitektur](#arsitektur)
  - [Diagram Alur](#diagram-alur)
  - [Go Core — `bot` package](#go-core--bot-package)
  - [Lua Loader — `plugins` package](#lua-loader--plugins-package)
  - [Konfigurasi — `config` package](#konfigurasi--config-package)
- [Lua API Reference](#lua-api-reference)
  - [Context Object (`ctx`)](#context-object-ctx)
  - [Properties](#properties)
  - [Methods](#methods)
- [Fitur Built-in](#fitur-built-in)
  - [Auto-Reconnect](#auto-reconnect)
  - [Auto-Reject Panggilan](#auto-reject-panggilan)
  - [Ignore Self Message](#ignore-self-message)
- [Panduan Membuat Plugin](#panduan-membuat-plugin)
  - [Struktur Dasar Plugin](#struktur-dasar-plugin)
  - [Contoh: Command Sederhana](#contoh-command-sederhana)
  - [Contoh: Command dengan Argumen](#contoh-command-dengan-argumen)
  - [Contoh: Greeting Personal](#contoh-greeting-personal)
  - [Contoh: Group-Only Command](#contoh-group-only-command)
  - [Contoh: Self-Destructing Command](#contoh-self-destructing-command)
  - [Contoh: Multi-Command dalam Satu File](#contoh-multi-command-dalam-satu-file)
- [Konfigurasi Bot](#konfigurasi-bot)
- [FAQ & Troubleshooting](#faq--troubleshooting)

---

## Arsitektur

Whatskel dibangun dengan arsitektur modular yang memisahkan antara **Go core** (penanganan koneksi WhatsApp) dan **Lua plugins** (logika command). Hal ini memungkinkan pengembang menambahkan fitur tanpa perlu mengompilasi ulang binary.

### Diagram Alur

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
│  │ Prefix, IsGroup, Reply, ReplyQuote,      │           │
│  │ React, DeleteMessage                     │           │
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

File utama: [`bot/bot.go`](./bot/bot.go)

Modul ini bertanggung jawab untuk:

| Fungsi | Deskripsi |
|---|---|
| `New(cfg)` | Inisialisasi client WhatsApp, database SQLite, dan plugin loader |
| `Start()` | Mendaftarkan event handler, menampilkan QR code, dan menghubungkan ke WhatsApp |
| `Stop()` | Menutup koneksi, membersihkan Lua state, dan membatalkan context |
| `handleMessage(v)` | Menerima pesan masuk, mem-parsing command, membangun `Context`, dan men-dispatch ke Lua |

Event handler menangani tiga jenis event:
- **`events.Message`** → Diteruskan ke `handleMessage()` untuk parsing command.
- **`events.Disconnected`** → Memicu auto-reconnect dengan exponential backoff.
- **`events.CallOffer`** → Otomatis menolak panggilan masuk.

### Lua Loader — `plugins` package

File utama: [`plugins/loader.go`](./plugins/loader.go)

Modul ini bertanggung jawab untuk:

| Fungsi | Deskripsi |
|---|---|
| `NewLoader(dir)` | Membuat Lua VM baru, mendaftarkan metatable Context, dan menyiapkan fungsi `export()` |
| `LoadAll()` | Memuat semua file `.lua` di dalam direktori plugin |
| `Dispatch(cmd, ctx)` | Mencari command yang sesuai dan mengeksekusi handler Lua-nya |
| `GetCommands()` | Mengembalikan daftar nama semua command yang terdaftar |
| `Close()` | Menutup Lua state dengan aman |

**Bagaimana Context Bekerja di Lua:**

Context diimplementasikan menggunakan `UserData` dan `Metatable` dari `gopher-lua`. Ini berbeda dari pendekatan naif (menyuntikkan ke tabel biasa) karena:
- Tidak ada konflik field (sebelumnya `Args` di-overwrite oleh fungsi).
- Metode dipanggil secara proper menggunakan sintaks `:` (colon syntax).
- Type-safe — Lua akan error jika argument pertama bukan Context yang valid.

### Konfigurasi — `config` package

File utama: [`config/config.go`](./config/config.go)

Membaca konfigurasi dari file TOML (`config.toml`). Lihat bagian [Konfigurasi Bot](#konfigurasi-bot) untuk detail format.

---

## Lua API Reference

### Context Object (`ctx`)

Setiap handler command Lua menerima satu argumen `ctx` yang berisi semua informasi tentang pesan masuk beserta metode-metode untuk berinteraksi dengan WhatsApp.

```lua
export("mycommand", function(ctx)
    -- ctx adalah Context object
    -- akses property: ctx.PropertyName
    -- panggil method:  ctx:MethodName(args)
end)
```

### Properties

| Property | Tipe | Deskripsi | Contoh Nilai |
|---|---|---|---|
| `ctx.Message` | `string` | Teks lengkap pesan yang diterima (termasuk prefix dan command) | `".ping"` |
| `ctx.Sender` | `string` | JID (WhatsApp ID) pengirim pesan | `"6281234567890@s.whatsapp.net"` |
| `ctx.SenderName` | `string` | Nama profil (PushName) pengirim. Jika tidak ada, fallback ke nomor telepon | `"John Doe"` |
| `ctx.Chat` | `string` | JID chat tempat pesan diterima (private chat atau grup) | `"120363xxx@g.us"` (grup) atau `"628xxx@s.whatsapp.net"` (private) |
| `ctx.Args` | `string` | Argumen setelah command. Kosong jika tidak ada argumen | `"hello world"` (dari pesan `.echo hello world`) |
| `ctx.Prefix` | `string` | Prefix command yang dikonfigurasi di `config.toml` | `"."` |
| `ctx.IsGroup` | `boolean` | `true` jika pesan berasal dari grup, `false` jika private chat | `true` |

### Methods

#### `ctx:Reply(text)`

Mengirim pesan teks biasa ke chat.

```lua
ctx:Reply("Hello, World!")
```

| Parameter | Tipe | Wajib | Deskripsi |
|---|---|---|---|
| `text` | `string` | ✅ | Teks yang akan dikirim |

---

#### `ctx:ReplyQuote(text)`

Mengirim pesan teks sambil mengutip (quote) pesan asli pengirim. Pesan yang dikutip akan muncul sebagai bubble reply di WhatsApp.

```lua
ctx:ReplyQuote("Ini balasan dengan quote!")
```

| Parameter | Tipe | Wajib | Deskripsi |
|---|---|---|---|
| `text` | `string` | ✅ | Teks yang akan dikirim |

**Kapan menggunakan Reply vs ReplyQuote?**
- Gunakan `Reply()` untuk pesan informatif umum (misal: menu, bantuan).
- Gunakan `ReplyQuote()` ketika penting bagi pengguna untuk tahu pesan mana yang dibalas (misal: echo, jawaban pertanyaan spesifik).

---

#### `ctx:React(emoji)`

Menambahkan reaksi emoji pada pesan command pengirim. Emoji muncul di bawah bubble chat pengirim.

```lua
ctx:React("👍")
ctx:React("✅")
ctx:React("❌")
```

| Parameter | Tipe | Wajib | Deskripsi |
|---|---|---|---|
| `emoji` | `string` | ✅ | Karakter emoji tunggal |

> **Tips:** Gunakan reaksi sebagai feedback visual cepat sebelum mengirim balasan. Misalnya, `ctx:React("⏳")` saat memproses, lalu kirim hasil.

---

#### `ctx:DeleteMessage()`

Menghapus/menarik kembali (*revoke*) pesan command yang dikirim oleh pengguna. Berguna untuk command yang mengandung informasi sensitif.

```lua
ctx:DeleteMessage()
```

> **Catatan:** Bot hanya dapat menghapus pesan di grup jika bot adalah admin grup. Di private chat, bot hanya dapat menghapus pesan miliknya sendiri.

---

## Fitur Built-in

### Auto-Reconnect

Ketika koneksi WebSocket ke WhatsApp terputus (misalnya karena masalah jaringan), bot secara otomatis mencoba menyambung ulang menggunakan **exponential backoff**:

| Percobaan | Delay |
|---|---|
| 1 | 2 detik |
| 2 | 4 detik |
| 3 | 8 detik |
| 4 | 16 detik |
| 5 | 32 detik |

Jika setelah 5 percobaan masih gagal, bot akan berhenti mencoba dan menampilkan log bahwa restart manual diperlukan.

### Auto-Reject Panggilan

Semua panggilan masuk (suara dan video) secara otomatis ditolak. Ini mencegah bot dari *hang* atau terinterupsi oleh spam panggilan.

### Ignore Self Message

Bot mengabaikan pesan yang dikirim oleh dirinya sendiri, sehingga tidak terjadi *infinite loop* saat bot mengirim balasan.

---

## Panduan Membuat Plugin

### Struktur Dasar Plugin

Setiap plugin adalah file `.lua` yang disimpan di dalam direktori `plugins/`. Semua file `.lua` dimuat otomatis saat bot pertama kali dijalankan.

```
plugins/
├── Menu.lua          # Menu dan command bawaan
├── Greetings.lua     # Plugin sapaan
├── Utils.lua         # Utility commands
└── YourPlugin.lua    # Plugin custom Anda
```

Gunakan fungsi global `export(name, handler)` untuk mendaftarkan command:

```lua
export("namacommand", function(ctx)
    -- logika command di sini
end)
```

- `name` (string): Nama command (tanpa prefix). Pengguna akan mengetik `{prefix}{name}` untuk memanggil command ini.
- `handler` (function): Fungsi yang menerima satu argumen `ctx` (Context).

### Contoh: Command Sederhana

```lua
-- plugins/Hello.lua
export("hello", function(ctx)
    ctx:React("👋")
    ctx:Reply("Hello, World! 🌍")
end)
```

**Penggunaan:** `.hello`
**Output:** Bot mereaksi dengan 👋 dan membalas "Hello, World! 🌍"

### Contoh: Command dengan Argumen

```lua
-- plugins/Repeat.lua
export("repeat", function(ctx)
    local text = ctx.Args
    if text == "" then
        ctx:React("❓")
        ctx:ReplyQuote("Usage: " .. ctx.Prefix .. "repeat <teks yang ingin diulang>")
        return
    end

    ctx:React("🔁")
    ctx:ReplyQuote(text .. "\n" .. text .. "\n" .. text)
end)
```

**Penggunaan:** `.repeat halo`
**Output:** Bot mereaksi dengan 🔁 dan mengutip pesan sambil membalas teks diulang 3 kali.

### Contoh: Greeting Personal

```lua
-- plugins/Greet.lua
export("greet", function(ctx)
    local name = ctx.SenderName
    ctx:React("🤝")

    local msg = "Halo, *" .. name .. "*! 👋\n\n"
    msg = msg .. "Selamat datang di Whatskel Bot.\n"
    msg = msg .. "Ketik " .. ctx.Prefix .. "menu untuk melihat daftar perintah."

    ctx:ReplyQuote(msg)
end)
```

**Penggunaan:** `.greet`
**Output:** "Halo, *John Doe*! 👋" — menggunakan `ctx.SenderName` untuk personalisasi.

### Contoh: Group-Only Command

```lua
-- plugins/GroupInfo.lua
export("groupinfo", function(ctx)
    if not ctx.IsGroup then
        ctx:React("🚫")
        ctx:ReplyQuote("Command ini hanya bisa digunakan di dalam grup!")
        return
    end

    ctx:React("📊")
    local msg = "📊 *Info Grup*\n\n"
    msg = msg .. "Chat ID: " .. ctx.Chat .. "\n"
    msg = msg .. "Pengirim: " .. ctx.SenderName

    ctx:Reply(msg)
end)
```

**Penggunaan:** `.groupinfo`
**Output:** Menampilkan info grup, atau menolak jika digunakan di private chat.

### Contoh: Self-Destructing Command

```lua
-- plugins/Secret.lua
export("secret", function(ctx)
    -- Hapus command pengguna agar tidak terlihat orang lain
    ctx:DeleteMessage()

    -- Kirim pesan rahasia
    ctx:Reply("🤫 Pesan rahasia diterima! Command kamu sudah dihapus.")
end)
```

**Penggunaan:** `.secret`
**Output:** Pesan `.secret` dihapus dari chat, lalu bot membalas konfirmasi.

### Contoh: Multi-Command dalam Satu File

Anda bisa mendaftarkan beberapa command dalam satu file `.lua`:

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
    ctx:ReplyQuote("🎲 Kamu mendapat angka: *" .. result .. "*")
end)
```

---

## Konfigurasi Bot

Konfigurasi bot dibaca dari file `config.toml`:

```toml
[bot]
prefix = "."                        # Prefix yang digunakan untuk trigger command
session_path = "whatsapp-session.db" # Path file session WhatsApp
db_path = "whatsapp-store.db"        # Path file database SQLite

[plugins]
directory = "plugins"                # Direktori tempat file .lua disimpan
```

| Key | Default | Deskripsi |
|---|---|---|
| `bot.prefix` | `"."` | Karakter atau string yang harus diketik sebelum nama command |
| `bot.session_path` | `"whatsapp-session.db"` | Lokasi penyimpanan session WhatsApp |
| `bot.db_path` | `"whatsapp-store.db"` | Lokasi database utama (device store) |
| `plugins.directory` | `"plugins"` | Direktori yang berisi file-file plugin `.lua` |

---

## FAQ & Troubleshooting

### Bot tidak merespon command saya
- Pastikan Anda menggunakan prefix yang benar (lihat `config.toml`).
- Pastikan nama command sesuai dengan yang di-`export()` di file `.lua`.
- Cek log terminal untuk error "Error loading plugin" atau "Error executing command".

### QR Code tidak muncul
- Pastikan `db_path` dapat ditulis oleh bot.
- Hapus file database lama jika Anda ingin melakukan login ulang: `make clean`.

### Plugin tidak dimuat
- Pastikan file plugin memiliki ekstensi `.lua`.
- Pastikan file berada di dalam direktori yang dikonfigurasi pada `plugins.directory`.
- Periksa syntax Lua — error syntax akan ditampilkan di log terminal.

### Bot disconnect terus-menerus
- Bot sudah memiliki fitur auto-reconnect dengan exponential backoff (sampai 5 percobaan).
- Jika tetap gagal, cek koneksi internet Anda.
- Pastikan hanya satu instance bot yang berjalan (dua instance dengan session yang sama akan saling kick).

### Bagaimana cara menambahkan command baru tanpa restart?
- Saat ini, plugin dimuat sekali saat startup. Anda perlu me-restart bot setelah menambahkan atau mengubah file `.lua`.
