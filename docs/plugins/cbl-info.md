---
title: Community Ban List Info
icon: lucide:plug
---

## Community Ban List Info

The Community Ban List (CBL) Info plugin monitors players joining your server and alerts administrators via Discord when players with poor reputation scores are detected, helping you identify potentially problematic players before they cause issues.

### Features

- Queries the Community Ban List API for player reputation data
- Alerts administrators via Discord when high-risk players join
- Configurable reputation threshold for alerts
- Includes active and expired ban information
- Automatic API timeout handling

### Requirements

This plugin requires the Discord connector to be configured with a valid bot token and appropriate permissions.

### Configuration Options

| Option | Description | Default | Required |
|--------|-------------|---------|----------|
| `channel_id` | Discord channel ID for sending alerts | "" | Yes |
| `threshold` | Reputation points threshold for alerts (6+ recommended) | 6 | No |
| `enabled` | Whether the plugin is enabled | true | No |
| `api_timeout_seconds` | Timeout for CBL API requests in seconds | 10 | No |

### How It Works

1. When a player joins the server, the plugin queries the Community Ban List API
2. The API returns reputation data including:
   - Reputation points and rank
   - Risk rating
   - Active and expired bans
   - Recent reputation changes
3. If the player's reputation points meet or exceed the threshold, an alert is sent to Discord
4. The alert includes player details and ban history for administrator review

### Understanding Reputation Points

- **0-5 points**: Generally trustworthy players
- **6-10 points**: Moderate risk - monitor closely
- **11+ points**: High risk - consider immediate action
- Points are based on community reports and ban history

### Example Configuration

```json
{
  "channel_id": "123456789012345678",
  "threshold": 8,
  "enabled": true,
  "api_timeout_seconds": 15
}
```

### Discord Setup

1. Create a Discord bot and get its token
2. Add the bot to your Discord server with message sending permissions
3. Create a private admin channel for CBL alerts
4. Copy the channel ID and use it in the plugin configuration

### Tips

- Start with a threshold of 6-8 to balance security and alert frequency
- Use a dedicated Discord channel for CBL alerts to avoid spam
- Review the Community Ban List FAQ for more information on reputation scoring
- Consider combining with other moderation plugins for comprehensive player management
- The plugin only alerts on player join - it doesn't prevent joins or take automatic action
