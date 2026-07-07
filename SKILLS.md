# Whatskel Skills — API Quick Reference

> Cheat-sheet ringkas seluruh property dan method yang tersedia pada objek `ctx` di plugin Lua Whatskel.
>
> Untuk penjelasan lengkap, contoh kode, dan panduan arsitektur, lihat **[WIKI.md](./WIKI.md)**.

---

## Properties

| Property | Type | Deskripsi |
|---|---|---|
| `ctx.Message` | `string` | Teks penuh pesan masuk (termasuk prefix dan command) |
| `ctx.Sender` | `string` | JID pengirim pesan (misal `628xxx@s.whatsapp.net`) |
| `ctx.SenderName` | `string` | Nama profil (PushName) pengirim. Fallback ke nomor telepon jika kosong |
| `ctx.Chat` | `string` | JID chat tempat pesan diterima (grup atau private) |
| `ctx.Args` | `string` | Argumen setelah nama command. Kosong jika tidak ada |
| `ctx.Prefix` | `string` | Prefix command yang dikonfigurasi di `config.toml` |
| `ctx.IsGroup` | `boolean` | `true` jika pesan berasal dari grup |
| `ctx.HasMedia` | `boolean` | `true` jika pesan ini mengandung media yang bisa didownload |
| `ctx.HasQuotedMedia` | `boolean` | `true` jika pesan yang di-quote mengandung media |
| `ctx.MediaType` | `string` | Tipe media: `"image"`, `"video"`, `"audio"`, `"document"`, `"sticker"`, atau `""` |
| `ctx.QuotedMediaType` | `string` | Tipe media pada pesan yang di-quote (sama dengan `MediaType`) |

---

## Methods — Messaging

| Method | Parameter | Return | Deskripsi |
|---|---|---|---|
| `ctx:Reply(text)` | `text: string` | — | Kirim pesan teks biasa ke chat |
| `ctx:ReplyQuote(text)` | `text: string` | — | Kirim pesan teks sambil mengutip pesan asli |
| `ctx:React(emoji)` | `emoji: string` | — | Tambahkan reaksi emoji pada pesan pengirim |
| `ctx:DeleteMessage()` | — | — | Hapus/revoke pesan command pengirim |

## Methods — Media Sending

| Method | Parameter | Return | Deskripsi |
|---|---|---|---|
| `ctx:ReplyImage(path, caption)` | `path: string`, `caption: string` (opsional) | `err` atau `nil` | Upload dan kirim gambar dari file lokal |
| `ctx:ReplySticker(path)` | `path: string` | `err` atau `nil` | Upload dan kirim sticker `.webp` dari file lokal |

## Methods — Media Downloading

| Method | Parameter | Return | Deskripsi |
|---|---|---|---|
| `ctx:DownloadMedia()` | — | `path, err` | Download media dari pesan ini ke file temp, return path absolut |
| `ctx:DownloadQuotedMedia()` | — | `path, err` | Download media dari pesan yang di-quote ke file temp, return path absolut |

> **Catatan:** `DownloadMedia` dan `DownloadQuotedMedia` mengembalikan **dua nilai**: `path` (string path file) dan `err` (string error atau nil). Selalu cek `err` sebelum menggunakan `path`.

## Methods — Utility

| Method | Parameter | Return | Deskripsi |
|---|---|---|---|
| `ctx:SendPrivateMessage(target, text)` | `target: string`, `text: string` | `err` atau `nil` | Kirim pesan ke user lain berdasarkan nomor/JID |

---

## Pola Penggunaan Umum

### 1. Command Dasar

```lua
export("hello", function(ctx)
    ctx:React("👋")
    ctx:Reply("Hello, " .. ctx.SenderName .. "!")
end)
```

### 2. Command dengan Validasi Argumen

```lua
export("echo", function(ctx)
    if ctx.Args == "" then
        ctx:React("❓")
        ctx:ReplyQuote("Usage: " .. ctx.Prefix .. "echo <text>")
        return
    end
    ctx:React("✅")
    ctx:ReplyQuote(ctx.Args)
end)
```

### 3. Group-Only Guard

```lua
export("groupcmd", function(ctx)
    if not ctx.IsGroup then
        ctx:React("🚫")
        ctx:ReplyQuote("Only available in groups!")
        return
    end
    -- command logic here
end)
```

### 4. Owner-Only Guard

```lua
local OWNER = "628xxxxxxxxxx@s.whatsapp.net"

export("admin", function(ctx)
    if ctx.Sender ~= OWNER then
        ctx:React("🚫")
        ctx:ReplyQuote("Owner only!")
        return
    end
    -- admin command logic here
end)
```

### 5. Download Media (Direct atau Quoted)

```lua
export("dl", function(ctx)
    local path, err

    if ctx.HasMedia then
        path, err = ctx:DownloadMedia()
    elseif ctx.HasQuotedMedia then
        path, err = ctx:DownloadQuotedMedia()
    else
        ctx:ReplyQuote("No media found!")
        return
    end

    if err then
        ctx:ReplyQuote("Download error: " .. err)
        return
    end

    ctx:React("✅")
    ctx:ReplyQuote("Saved to: " .. path)
end)
```

### 6. Image ke Sticker

```lua
export("sticker", function(ctx)
    local path, err
    if ctx.HasMedia and ctx.MediaType == "image" then
        path, err = ctx:DownloadMedia()
    elseif ctx.HasQuotedMedia and ctx.QuotedMediaType == "image" then
        path, err = ctx:DownloadQuotedMedia()
    else
        ctx:ReplyQuote("Send or reply to an image!")
        return
    end

    if err then ctx:ReplyQuote("Error: " .. err); return end

    ctx:React("🏷️")
    ctx:ReplySticker(path)
end)
```

### 7. Kirim Gambar dengan Caption

```lua
export("photo", function(ctx)
    local err = ctx:ReplyImage("/path/to/photo.jpg", "Caption text here")
    if err then
        ctx:ReplyQuote("Failed: " .. err)
    end
end)
```

---

## Media Type Reference

| `MediaType` / `QuotedMediaType` | Ekstensi file temp | Deskripsi |
|---|---|---|
| `"image"` | `.jpg` | Foto/gambar |
| `"video"` | `.mp4` | Video |
| `"audio"` | `.ogg` | Voice note / audio |
| `"document"` | `.bin` | Dokumen (PDF, ZIP, dll) |
| `"sticker"` | `.webp` | Sticker WhatsApp |
| `""` | — | Tidak ada media |

---

## Error Handling

Method yang mengembalikan error:

| Method | Jika sukses | Jika gagal |
|---|---|---|
| `ctx:ReplyImage(...)` | return `nil` | return `string` (error message) |
| `ctx:ReplySticker(...)` | return `nil` | return `string` (error message) |
| `ctx:SendPrivateMessage(...)` | return `nil` | return `string` (error message) |
| `ctx:DownloadMedia()` | return `path, nil` | return `nil, string` |
| `ctx:DownloadQuotedMedia()` | return `path, nil` | return `nil, string` |

> Method tanpa return (`Reply`, `ReplyQuote`, `React`, `DeleteMessage`) akan mem-log error di sisi Go jika gagal, tetapi tidak mengembalikan error ke Lua.
