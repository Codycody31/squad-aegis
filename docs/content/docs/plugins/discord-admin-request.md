---
title: Discord Admin Requests
---

The Discord Admin Requests plugin allows players to request administrator assistance through in-game chat, automatically notifying admins via Discord with configurable ping options.

## Features

- Player-initiated admin requests via `!admin` command
- Discord notifications with player information
- Configurable role pings or @here pings
- Cooldown system to prevent spam
- Optional in-game admin notifications
- Chat type filtering

## Requirements

This plugin requires the Discord connector to be configured with a valid bot token and appropriate permissions.

## Configuration Options

| Option | Description | Default | Required |
|--------|-------------|---------|----------|
| `channel_id` | Discord channel ID for admin requests | "" | Yes |
| `ignore_chats` | Chat types to ignore | ["ChatSquad"] | No |
| `ping_groups` | Discord role IDs to ping | [] | No |
| `ping_here` | Ping @here instead of roles | false | No |
| `ping_delay` | Cooldown between pings (ms) | 60000 | No |
| `color` | Color of the Discord embed | 16761867 | No |
| `warn_in_game_admins` | Notify in-game admins | false | No |
| `show_in_game_admins` | Show admin count to players | true | No |

## How It Works

1. Players type `!admin` in game chat to request assistance
2. The plugin checks cooldown and chat type filters
3. Creates a Discord notification with:
   - Player's name and information
   - Request timestamp
   - Server context
4. Pings configured roles or @here
5. Optionally notifies in-game admins
6. Shows active admin count to requesting player

## Example Configuration

```json
{
  "channel_id": "123456789012345678",
  "ignore_chats": ["ChatSquad"],
  "ping_groups": ["987654321098765432", "876543210987654321"],
  "ping_here": false,
  "ping_delay": 30000,
  "color": 15158332,
  "warn_in_game_admins": true,
  "show_in_game_admins": true
}
```

## Discord Setup

1. Create a Discord bot and get its token
2. Add the bot to your Discord server with message sending permissions
3. Create an admin requests channel
4. Get role IDs for admin roles you want to ping
5. Copy channel and role IDs to the plugin configuration

## Tips

- Use role pings for specific admin groups or @here for all online members
- Set appropriate cooldowns to prevent spam
- Consider using a dedicated admin channel for requests
- The red color (15158332) is good for urgent admin requests
- Test the ping functionality to ensure proper notifications
