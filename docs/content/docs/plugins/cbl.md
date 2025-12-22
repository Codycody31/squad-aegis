---
title: Community Ban List
---

The Community Ban List (CBL) plugin monitors players joining your server and alerts administrators via Discord when players with poor reputation scores are detected, helping you identify potentially problematic players before they cause issues.

## Features

- Queries the Community Ban List API for player reputation data
- Alerts administrators via Discord when high-risk players join
- Configurable reputation threshold for alerts
- Optional automatic kicking of players exceeding kick threshold
- Steam ID ignore list to exclude specific players from checks
- Includes active and expired ban information
- Automatic API timeout handling

## Requirements

This plugin requires the Discord connector to be configured with a valid bot token and appropriate permissions.

## Configuration Options

| Option | Description | Default | Required |
|--------|-------------|---------|----------|
| `channel_id` | Discord channel ID for sending alerts | "" | Yes |
| `threshold` | Reputation points threshold for alerts (6+ recommended) | 6 | No |
| `kick_threshold` | Automatically kick players when reputation points exceed this threshold. Set to 0 to disable auto-kick | 0 | No |
| `ignored_steam_ids` | Array of Steam IDs to ignore from CBL checks. Players in this list will not be checked, alerted, or kicked | [] | No |
| `api_timeout_seconds` | Timeout for CBL API requests in seconds | 10 | No |

## How It Works

1. When a player joins the server, the plugin checks if their Steam ID is in the ignore list
2. If not ignored, the plugin queries the Community Ban List API
3. The API returns reputation data including:
   - Reputation points and rank
   - Risk rating
   - Active and expired bans
   - Recent reputation changes
4. If the player's reputation points meet or exceed the `threshold`, an alert is sent to Discord
5. The alert includes player details and ban history for administrator review
6. If `kick_threshold` is set and the player's reputation exceeds it, the player is automatically kicked with the message "Kicked via https://communitybanlist.com"

## Understanding Reputation Points

- **0-5 points**: Generally trustworthy players
- **6-10 points**: Moderate risk - monitor closely
- **11+ points**: High risk - consider immediate action
- Points are based on community reports and ban history

## Example Configuration

**Basic Setup (Alerts Only):**
```json
{
  "channel_id": "123456789012345678",
  "threshold": 8,
  "api_timeout_seconds": 15
}
```

**With Auto-Kick:**
```json
{
  "channel_id": "123456789012345678",
  "threshold": 6,
  "kick_threshold": 12,
  "api_timeout_seconds": 15
}
```

**With Ignore List:**
```json
{
  "channel_id": "123456789012345678",
  "threshold": 8,
  "kick_threshold": 0,
  "ignored_steam_ids": ["76561198000000000", "76561198011111111"],
  "api_timeout_seconds": 15
}
```

## Discord Setup

1. Create a Discord bot and get its token
2. Add the bot to your Discord server with message sending permissions
3. Create a private admin channel for CBL alerts
4. Copy the channel ID and use it in the plugin configuration

## Auto-Kick Feature

The `kick_threshold` option allows automatic kicking of players with very high reputation scores:

- Set `kick_threshold` to 0 to disable auto-kick (default)
- When enabled, players exceeding the threshold are immediately kicked
- Kick message: "Kicked via https://communitybanlist.com"
- Recommended: Set `kick_threshold` higher than `threshold` to allow manual review before auto-kick
- Example: `threshold: 6` (alert) and `kick_threshold: 12` (auto-kick) gives admins time to review

## Ignore List

The `ignored_steam_ids` array allows you to exclude specific players from CBL checks:

- Players in the ignore list are completely skipped (no API query, no alerts, no kicks)
- Useful for trusted players with false positives or known good players
- Format: Array of Steam ID strings (e.g., `["76561198000000000"]`)
- Players are checked against the ignore list before any API calls are made

## Tips

- Start with a threshold of 6-8 to balance security and alert frequency
- Use a dedicated Discord channel for CBL alerts to avoid spam
- Review the Community Ban List FAQ for more information on reputation scoring
- Consider combining with other moderation plugins for comprehensive player management
- Set `kick_threshold` higher than `threshold` to allow manual review before auto-kick
- Use the ignore list for trusted players who may have false positives
- The plugin checks players on join - it doesn't prevent joins but can auto-kick if configured
