---
title: Discord Kill Feed
icon: lucide:plug
---

# Discord Kill Feed

The Discord Kill Feed plugin logs all player wounds and kill-related information to a Discord channel, providing administrators with detailed combat logs for review and moderation.

## Features

- Logs all player wounds and deaths
- Includes weapon information and damage details
- Optional Community Ban List (CBL) information
- Color-coded embeds for easy identification
- Configurable Discord channel

## Requirements

This plugin requires the Discord connector to be configured with a valid bot token and appropriate permissions.

## Configuration Options

| Option | Description | Default | Required |
|--------|-------------|---------|----------|
| `channel_id` | The ID of the Discord channel to log kill feed to | "" | Yes |
| `color` | The color of the Discord embeds | 16761867 | No |
| `disable_cbl` | Disable Community Ban List information in logs | false | No |
| `enabled` | Whether the plugin is enabled | false | No |

## How It Works

1. The plugin monitors all player wound events from the game logs
2. Each wound/kill is formatted into a detailed Discord embed
3. Information includes:
   - Victim and attacker names
   - Weapon used
   - Damage amount and type
   - Team information
   - CBL status (if enabled)
4. Embeds are sent to the configured Discord channel

## Example Configuration

```json
{
  "channel_id": "123456789012345678",
  "color": 16761867,
  "disable_cbl": false
}
```

## Discord Setup

1. Create a Discord bot and get its token
2. Add the bot to your Discord server
3. Create a dedicated channel for kill feed logs
4. Copy the channel ID for the plugin configuration
5. Ensure the bot has message sending permissions

## CBL Integration

When `disable_cbl` is set to `false`, the plugin will include Community Ban List information in the logs:

- Player ban status
- Ban reasons
- Ban expiration dates
- Links to ban details

## Tips

- Use a dedicated Discord channel for kill feed to avoid cluttering other channels
- The orange color (16761867) provides good visibility for kill events
- Consider enabling CBL integration for enhanced moderation capabilities
- Monitor the Discord channel regularly to catch potential rule violations
- The plugin is disabled by default - enable it only when you have a dedicated moderation team
