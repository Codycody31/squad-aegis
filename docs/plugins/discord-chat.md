---
title: Discord Chat
icon: lucide:plug
---

## Discord Chat

The Discord Chat plugin logs in-game chat messages to a specified Discord channel, allowing administrators to monitor player communication from Discord.

### Features

- Logs all in-game chat to Discord
- Color-coded embeds for different chat types
- Configurable channel ID
- Option to ignore specific chat types
- Customizable embed colors

### Requirements

This plugin requires the Discord connector to be configured with a valid bot token and appropriate permissions.

### Configuration Options

| Option | Description | Default | Required |
|--------|-------------|---------|----------|
| `channel_id` | The ID of the Discord channel to log chat messages to | "" | Yes |
| `chat_colors` | Color mapping for different chat types | See below | No |
| `color` | Default embed color if no specific color is set | 16761867 | No |
| `ignore_chats` | Array of chat types to ignore | ["ChatSquad"] | No |
| `enabled` | Whether the plugin is enabled | true | No |

### Chat Colors Configuration

The `chat_colors` object maps chat types to color values:

```json
{
  "ChatAll": 16761867,    // Orange
  "ChatTeam": 65280,      // Green
  "ChatAdmin": 16711680   // Red
}
```

### Supported Chat Types

- `ChatAll` - All chat messages
- `ChatTeam` - Team chat messages
- `ChatSquad` - Squad chat messages
- `ChatAdmin` - Admin chat messages

### How It Works

1. The plugin listens for all in-game chat messages
2. Messages are filtered based on the `ignore_chats` configuration
3. Each message is formatted into a Discord embed
4. The embed color is determined by the chat type or uses the default color
5. Messages are sent to the configured Discord channel

### Example Configuration

```json
{
  "channel_id": "123456789012345678",
  "chat_colors": {
    "ChatAll": 16761867,
    "ChatTeam": 65280,
    "ChatAdmin": 16711680
  },
  "color": 16761867,
  "ignore_chats": ["ChatSquad"],
  "enabled": true
}
```

### Discord Setup

1. Create a Discord bot and get its token
2. Add the bot to your Discord server with appropriate permissions
3. Create a channel for chat logs
4. Copy the channel ID and use it in the plugin configuration
5. Ensure the bot has permission to send messages in the channel

### Tips

- Use different colors for different chat types to make logs easier to read
- Consider ignoring squad chat if it creates too much noise
- Make sure the Discord bot has the necessary permissions in the target channel
- Test the configuration by sending messages in-game and checking the Discord channel
