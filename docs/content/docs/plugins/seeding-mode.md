---
title: Seeding Mode
---

The Seeding Mode plugin automatically broadcasts seeding rules to players when the server is below a specified player threshold, and can announce when the server goes "live" with sufficient players.

## Features

- Automatic seeding message broadcasts based on player count
- "Live" announcements when server reaches full capacity
- Configurable thresholds and messages
- Event-driven execution after new games
- Adjustable broadcast intervals

## Configuration Options

| Option | Description | Default | Required |
|--------|-------------|---------|----------|
| `interval_ms` | Broadcast frequency in milliseconds | 150000 | No |
| `seeding_threshold` | Player count for seeding mode | 50 | No |
| `seeding_message` | Message during seeding | "Seeding Rules Active! Fight only over the middle flags! No FOB Hunting!" | No |
| `live_enabled` | Enable "Live" messages | true | No |
| `live_threshold` | Player count for "Live" status | 52 | No |
| `live_message` | "Live" announcement message | "Live!" | No |
| `wait_on_new_games` | Wait for new game events | true | No |
| `wait_time_on_new_game` | Delay after new game in seconds | 30 | No |

## How It Works

1. The plugin monitors player count continuously
2. When player count is below `seeding_threshold`:
   - Broadcasts seeding rules at regular intervals
   - Continues until player count reaches the threshold
3. When player count reaches `live_threshold`:
   - Broadcasts the "Live" message once
   - Stops seeding broadcasts
4. After new games, waits for the configured delay before checking player counts

## Example Configuration

```json
{
  "interval_ms": 120000,
  "seeding_threshold": 40,
  "seeding_message": "Server seeding - No FOB camping, fight over objectives only!",
  "live_enabled": true,
  "live_threshold": 45,
  "live_message": "Server is now LIVE! Normal rules apply.",
  "wait_on_new_games": true,
  "wait_time_on_new_game": 60
}
```

## Tips

- Set seeding threshold lower than live threshold to prevent rapid switching
- Use clear, concise messages that players will see and understand
- The 30-second delay after new games allows the round to stabilize
- Adjust broadcast intervals based on your server's typical seeding time
- Test the thresholds during low-population periods to ensure proper operation
