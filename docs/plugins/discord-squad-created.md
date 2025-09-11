---
title: Discord Squad Created
icon: lucide:plug
---

## Discord Squad Created

The Discord Squad Created plugin logs squad creation events to Discord, allowing administrators to track when new squads are formed during matches.

### Features

- Logs all squad creation events to Discord
- Includes squad name, creator, and team information
- Color-coded embed messages
- Configurable Discord channel
- Embed or plain text message format

### Requirements

This plugin requires the Discord connector to be configured with a valid bot token and appropriate permissions.

### Configuration Options

| Option | Description | Default | Required |
|--------|-------------|---------|----------|
| `channel_id` | Discord channel ID for squad logs | "" | Yes |
| `color` | Color of the Discord embed | 16761867 | No |
| `use_embed` | Send as embed or plain text | true | No |
| `enabled` | Whether the plugin is enabled | false | No |

### How It Works

1. The plugin monitors squad creation events from the game server
2. When a player creates a squad, it captures:
   - Squad name
   - Creator's name
   - Team information
   - Creation timestamp
3. Formats the information into a Discord message or embed
4. Sends the notification to the configured Discord channel

### Example Configuration

```json
{
  "channel_id": "123456789012345678",
  "color": 3447003,
  "use_embed": true,
  "enabled": true
}
```

### Discord Setup

1. Create a Discord bot and get its token
2. Add the bot to your Discord server with message sending permissions
3. Create a channel for squad activity logs
4. Copy the channel ID and use it in the plugin configuration

### Tips

- Use embeds for better formatting and visual appeal
- Choose a color that stands out for squad creation events
- Consider creating a dedicated channel for squad activity
- The plugin is disabled by default - enable it when you want to monitor squad formation
- Useful for tracking team composition and player activity
