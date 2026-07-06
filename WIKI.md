# Whatskel Bot Specifications

Whatskel is a WhatsApp bot written in Go using the `whatsmeow` library. It features a dynamically loaded plugin system utilizing Lua (`gopher-lua`), making it easy to create and extend bot commands without recompiling the main binary.

## Architecture

1. **Go Core (`bot` package)**:
   - Connects to WhatsApp using `whatsmeow`.
   - Handles session persistence with a SQLite database.
   - Listens for incoming messages and checks for the configured prefix.
   - If a command is detected, it wraps the event into a Context and dispatches it to the Lua Plugin Loader.

2. **Lua Loader (`plugins` package)**:
   - Evaluates and runs all `.lua` scripts found in the `plugins/` directory.
   - Registers commands exported by scripts via the `export(name, function)` mechanism.
   - Binds the Go context variables and methods into a robust Lua UserData (metatables).

## Lua API Reference

When a command is triggered, the mapped Lua function is called with a `Context` (`ctx`) argument.

### Properties
- `ctx.Message` (string): The full message text (excluding the command prefix and name).
- `ctx.Sender` (string): The JID of the message sender.
- `ctx.Chat` (string): The JID of the chat (Group or Private Chat).
- `ctx.Args` (string): The arguments passed after the command.
- `ctx.Prefix` (string): The command prefix used to trigger this bot.

### Methods
- `ctx:Reply(text)`
  Sends a standard text reply to the chat.
- `ctx:ReplyQuote(text)`
  **[Mandatory Feature]** Sends a text reply while quoting the original command message.
- `ctx:React(emoji)`
  **[Mandatory Feature]** Reacts to the user's message with a specific emoji (e.g., "👍", "✅", "❓").

## Creating Plugins

To create a new command, create a `.lua` file inside the `plugins/` directory. Use the `export` function to register the command.

```lua
export("mycommand", function(ctx)
    -- Your logic here
    local args = ctx.Args
    
    if args == "" then
        ctx:React("❌")
        ctx:ReplyQuote("Please provide arguments!")
        return
    end

    ctx:React("✅")
    ctx:Reply("You said: " .. args)
end)
```
