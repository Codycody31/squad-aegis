---
title: Chat Commands
---

The Chat Commands plugin allows you to configure custom chat commands that can broadcast messages to all players or send private warnings to the player who triggered the command.

## Features

- Configure multiple custom chat commands
- Support for broadcast (public) and warn (private) message types
- Ignore specific chat types for each command
- Easy configuration through the web interface

## Configuration Options

| Option | Description | Default | Required |
|--------|-------------|---------|----------|
| `commands` | Array of command configurations | See example below | No |

Each command in the array has the following properties:

| Property | Description | Required |
|----------|-------------|----------|
| `command` | The chat command trigger (without !) | Yes |
| `type` | Response type: 'warn' (private) or 'broadcast' (public) | Yes |
| `response` | The message to send when command is triggered | Yes |
| `ignoreChats` | Array of chat types to ignore for this command | No |

## How It Works

1. Players type commands in chat using the `!` prefix (e.g., `!help`)
2. The plugin checks if the command matches any configured commands
3. If it matches and the chat type is not ignored, the appropriate response is sent
4. Warn commands are sent privately to the player
5. Broadcast commands are sent to all players

## Example Configuration

```json
{
  "commands": [
    {
      "command": "help",
      "type": "warn",
      "response": "Welcome to our server! Visit our website for more info.",
      "ignoreChats": []
    },
    {
      "command": "rules",
      "type": "broadcast",
      "response": "Server Rules: 1. No team killing 2. Respect all players 3. Have fun!",
      "ignoreChats": ["ChatSquad"]
    },
    {
      "command": "discord",
      "type": "warn",
      "response": "Join our Discord: https://discord.gg/example",
      "ignoreChats": []
    }
  ]
}
```

## Supported Chat Types

The `ignoreChats` array can contain any of the following chat types:

- `ChatAll` - All chat
- `ChatTeam` - Team chat
- `ChatSquad` - Squad chat
- `ChatAdmin` - Admin chat

## Tips

- Use warn commands for personal information like Discord links
- Use broadcast commands for server-wide announcements
- The `ignoreChats` feature is useful to prevent commands from triggering in certain contexts
- Commands are case-insensitive
- Make sure your command names don't conflict with existing game commands
