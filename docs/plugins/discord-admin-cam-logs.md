---
title: Discord Admin Camera Logs
icon: lucide:plug
---

# Discord Admin Camera Logs

The Discord Admin Camera Logs plugin tracks and logs admin camera usage in Squad, providing transparency and accountability for administrative actions that involve camera controls.

## Features

- Logs admin camera session start and end times
- Tracks duration of camera usage
- Includes admin identification information
- Color-coded embed messages
- Automatic session tracking

## Requirements

This plugin requires the Discord connector to be configured with a valid bot token and appropriate permissions.

## Configuration Options

| Option | Description | Default | Required |
|--------|-------------|---------|----------|
| `channel_id` | Discord channel ID for camera logs | "" | Yes |
| `color` | Color of the Discord embed | 16761867 | No |
| `enabled` | Whether the plugin is enabled | true | No |

## How It Works

1. The plugin monitors admin camera activation events
2. When an admin enters camera mode, it starts tracking:
   - Admin's name and Steam ID
   - Session start time
   - Camera usage duration
3. When the admin exits camera mode, it logs:
   - Session end time
   - Total duration
   - Complete session summary
4. All information is sent to the configured Discord channel

## Example Configuration

```json
{
  "channel_id": "123456789012345678",
  "color": 15158332
}
```

## Discord Setup

1. Create a Discord bot and get its token
2. Add the bot to your Discord server with message sending permissions
3. Create a private admin channel for camera logs
4. Copy the channel ID and use it in the plugin configuration

## Tips

- Use a private admin-only channel for camera logs
- Choose a distinctive color for camera-related events
- Monitor logs regularly to ensure appropriate admin camera usage
- Useful for maintaining transparency in administrative actions
- Helps track potential abuse of admin camera privileges
