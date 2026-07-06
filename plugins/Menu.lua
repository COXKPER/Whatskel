export("menu", function(ctx)
    ctx:React("📋")
    local msg = "╔══════════════════════╗\n"
    msg = msg .. "║  🤖 Whatskel Bot    ║\n"
    msg = msg .. "╚══════════════════════╝\n\n"
    msg = msg .. "📋 *Available Commands:*\n\n"
    msg = msg .. "▸ menu - Show this menu\n"
    msg = msg .. "▸ ping - Check bot status\n"
    msg = msg .. "▸ echo <text> - Echo message with quote\n\n"
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
