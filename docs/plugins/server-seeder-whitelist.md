---
title: Server Seeder Whitelist
icon: lucide:plug
---

# Server Seeder Whitelist

The Server Seeder Whitelist plugin rewards players who help populate the server during low-population periods by progressively granting them whitelist privileges based on their seeding time.

## Features

- Tracks players who help seed the server during low population
- Progressive whitelist system based on seeding hours
- Automatic progress decay for inactive players
- Admin group assignment for whitelisted players
- Chat command for players to check progress
- Configurable seeding thresholds and timeframes

## Configuration Options

| Option | Description | Default | Required |
|--------|-------------|---------|----------|
| `seeding_threshold` | Player count for seeding mode | 50 | No |
| `hours_to_whitelist` | Hours needed for 100% whitelist | 6 | No |
| `whitelist_duration_days` | Days whitelist lasts | 7 | No |
| `decay_after_hours` | Hours before decay starts | 48 | No |
| `min_players_for_decay` | Server players needed for decay | 60 | No |
| `min_players_for_seeding` | Server players needed for progress | 10 | No |
| `progress_interval_seconds` | Progress check frequency | 60 | No |
| `decay_interval_seconds` | Decay application frequency | 3600 | No |
| `whitelist_group_name` | Admin group name | "seeder_whitelist" | No |
| `wait_on_new_games` | Pause after new games | true | No |
| `wait_time_on_new_game` | Delay after new game | 120 | No |
| `enable_chat_command` | Enable !wl command | true | No |

## How It Works

1. **Seeding Detection**: Monitors when server is below seeding threshold
2. **Progress Accumulation**: Awards progress to players present during seeding
3. **Whitelist Achievement**: Grants admin privileges when progress reaches 100%
4. **Decay System**: Reduces progress for inactive players
5. **Automatic Management**: Handles admin group assignments and expirations

## Progress System

- Players earn progress when server population is below threshold
- Progress accumulates over time toward the whitelist threshold
- 100% progress grants temporary admin privileges
- Progress decays if players become inactive
- Whitelist status expires after the configured duration

## Example Configuration

```json
{
  "seeding_threshold": 40,
  "hours_to_whitelist": 8,
  "whitelist_duration_days": 10,
  "decay_after_hours": 72,
  "min_players_for_decay": 70,
  "min_players_for_seeding": 15,
  "whitelist_group_name": "server_seeders"
}
```

## Admin Group Setup

The plugin automatically manages an admin group for whitelisted players. Ensure your server configuration includes:

```ini
[Group.seeder_whitelist]
GroupName=seeder_whitelist
```

## Tips

- Set seeding threshold based on your server's typical population
- Adjust hours requirement based on how often your server needs seeding
- Use decay settings to encourage regular participation
- The chat command `!wl` allows players to check their progress
- Consider whitelist duration to maintain active seeders
- Monitor progress to ensure the system rewards helpful players
