---
title: Discord Admin Broadcast
---

The Discord Admin Broadcast plugin forwards all admin broadcast messages from the game server to a designated Discord channel, ensuring that important administrative announcements reach your Discord community.

## Features

- Forwards all admin broadcasts to Discord
- Color-coded embed messages for easy identification
- Configurable Discord channel
- Automatic message formatting
- Real-time synchronization

## Requirements

This plugin requires the Discord connector to be configured with a valid bot token and appropriate permissions.

## Configuration Options

| Option | Description | Default | Required |
|--------|-------------|---------|----------|
| `channel_id` | Discord channel ID for broadcasting messages | "" | Yes |
| `color` | Color of the Discord embed | 16761867 | No |
| `enabled` | Whether the plugin is enabled | true | No |

## How It Works

1. The plugin monitors admin broadcast events from the game server
2. When an admin broadcast is sent in-game, it's captured by the plugin
3. The message is formatted into a Discord embed with:
   - Admin's name
   - Broadcast message content
   - Timestamp
   - Color-coded for visibility
4. The embed is sent to the configured Discord channel

## Example Configuration

```json
{
  "channel_id": "123456789012345678",
  "color": 16761867
}
```

## Discord Setup

1. Create a Discord bot and get its token
2. Add the bot to your Discord server with message sending permissions
3. Create a channel for admin broadcasts (can be the same as other admin channels)
4. Copy the channel ID and use it in the plugin configuration

## Tips

- Use a dedicated channel or integrate with your existing admin channels
- The orange color (16761867) provides good visibility for important messages
- Ensure the Discord bot has proper permissions in the target channel
- Test by sending an admin broadcast in-game and verifying it appears in Discord
- Consider using different colors for different types of administrative messages if you have multiple broadcast plugins
