---
title: Squad Leader Whitelist
icon: lucide:plug
---

## Squad Leader Whitelist

The Squad Leader Whitelist plugin tracks players who effectively lead squads and progressively grants them whitelist privileges based on their leadership time, helping to reward and retain skilled squad leaders.

### Features

- Tracks squad leadership with minimum member requirements
- Progressive whitelist system based on leadership hours
- Automatic progress decay for inactive players
- Admin group assignment for whitelisted players
- Chat command for players to check progress
- Configurable thresholds and timeframes

### Configuration Options

| Option | Description | Default | Required |
|--------|-------------|---------|----------|
| `min_squad_size` | Minimum squad members for progress | 5 | No |
| `hours_to_whitelist` | Hours needed for 100% whitelist | 8 | No |
| `whitelist_duration_days` | Days whitelist lasts | 14 | No |
| `decay_after_hours` | Hours before decay starts | 72 | No |
| `min_players_for_decay` | Server players needed for decay | 40 | No |
| `min_players_for_leadership` | Server players needed for progress | 20 | No |
| `progress_interval_seconds` | Progress check frequency | 60 | No |
| `decay_interval_seconds` | Decay application frequency | 3600 | No |
| `require_unlocked_squad` | Only unlocked squads count | true | No |
| `whitelist_group_name` | Admin group name | "squad_leader_whitelist" | No |
| `wait_on_new_games` | Pause after new games | true | No |
| `wait_time_on_new_game` | Delay after new game | 30 | No |

### How It Works

1. **Leadership Tracking**: Monitors players leading squads with minimum members
2. **Progress Accumulation**: Awards progress points based on leadership time
3. **Whitelist Achievement**: Grants admin privileges when progress reaches 100%
4. **Decay System**: Reduces progress for inactive players
5. **Automatic Management**: Handles admin group assignments and expirations

### Progress System

- Players earn progress when leading squads with 5+ members
- Progress accumulates over time toward the whitelist threshold
- 100% progress grants temporary admin privileges
- Progress decays if players become inactive
- Whitelist status expires after the configured duration

### Example Configuration

```json
{
  "min_squad_size": 5,
  "hours_to_whitelist": 10,
  "whitelist_duration_days": 21,
  "decay_after_hours": 96,
  "min_players_for_decay": 50,
  "min_players_for_leadership": 25,
  "require_unlocked_squad": true,
  "whitelist_group_name": "squad_leaders"
}
```

### Admin Group Setup

The plugin automatically manages an admin group for whitelisted players. Ensure your server configuration includes:

```ini
[Group.squad_leader_whitelist]
GroupName=squad_leader_whitelist
```

### Tips

- Adjust `hours_to_whitelist` based on your server's activity level
- Use `require_unlocked_squad` to ensure only public squads count
- Monitor decay settings to balance rewarding activity vs preventing stagnation
- The chat command `!wl` allows players to check their progress
- Consider the whitelist duration to maintain fresh leadership
