export("menu", function(ctx)
    ctx:React("📋")
    local name = ctx.SenderName

    local msg = "╔══════════════════════╗\n"
    msg = msg .. "║  🤖 Whatskel Bot    ║\n"
    msg = msg .. "╚══════════════════════╝\n\n"
    msg = msg .. "Halo, *" .. name .. "*! 👋\n\n"
    msg = msg .. "📋 *Available Commands:*\n\n"
    msg = msg .. "▸ menu - Show this menu\n"
    msg = msg .. "▸ ping - Check bot status\n"
    msg = msg .. "▸ echo <text> - Echo message with quote\n"
    msg = msg .. "▸ greet - Personal greeting\n"
    msg = msg .. "▸ info - Show chat info\n\n"
    msg = msg .. "Prefix: \"" .. ctx.Prefix .. "\""

    ctx:Reply(msg)
end)

export("ping", function(ctx)
    ctx:React("🏓")
    ctx:ReplyQuote("Pong! 🏓")
end)

export("echo", function(ctx)
    local text = ctx.Args
    if text == "" then
        ctx:React("❓")
        ctx:ReplyQuote("Usage: " .. ctx.Prefix .. "echo <text>")
        return
    end
    ctx:React("✅")
    ctx:ReplyQuote(text)
end)

export("greet", function(ctx)
    ctx:React("🤝")
    local name = ctx.SenderName
    local msg = "Halo, *" .. name .. "*! 👋\n\n"
    msg = msg .. "Selamat datang di Whatskel Bot.\n"
    msg = msg .. "Ketik " .. ctx.Prefix .. "menu untuk melihat daftar perintah."
    ctx:ReplyQuote(msg)
end)

export("info", function(ctx)
    ctx:React("📊")
    local msg = "📊 *Chat Info*\n\n"
    msg = msg .. "👤 Sender: " .. ctx.SenderName .. "\n"
    msg = msg .. "🆔 JID: " .. ctx.Sender .. "\n"
    msg = msg .. "💬 Chat: " .. ctx.Chat .. "\n"

    if ctx.IsGroup then
        msg = msg .. "🏘️ Type: Group"
    else
        msg = msg .. "🔒 Type: Private Chat"
    end

    ctx:Reply(msg)
end)
