---
title: Discord Round Ended
icon: lucide:plug
---

## Discord Round Ended

The Discord Round Ended plugin sends notifications to Discord whenever a round ends in Squad, including information about the winning team and round statistics.

### Features

- Announces round winners to Discord
- Color-coded embed messages
- Automatic round end detection
- Configurable Discord channel
- Real-time notifications

### Requirements

This plugin requires the Discord connector to be configured with a valid bot token and appropriate permissions.

### Configuration Options

| Option | Description | Default | Required |
|--------|-------------|---------|----------|
| `channel_id` | Discord channel ID for notifications | "" | Yes |
| `color` | Color of the Discord embed | 16761867 | No |
| `enabled` | Whether the plugin is enabled | true | No |

### How It Works

1. The plugin monitors game events for round endings
2. When a round ends, it captures the winning team information
3. A formatted embed is created with round details
4. The embed is sent to the configured Discord channel
5. Includes winner information and match statistics

### Example Configuration

```json
{
  "channel_id": "123456789012345678",
  "color": 65280
}
```

### Discord Setup

1. Create a Discord bot and get its token
2. Add the bot to your Discord server with message sending permissions
3. Create a channel for match results and announcements
4. Copy the channel ID and use it in the plugin configuration

### Tips

- Use different colors for different game modes or servers
- Consider creating a dedicated channel for match results
- The plugin works with both standard and custom game modes
- Ensure the Discord bot has proper permissions in the target channel
- Test the configuration during a match to verify proper operation
