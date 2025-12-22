---
title: Team Balancer
---

The Team Balancer plugin tracks dominant win streaks and automatically triggers squad-preserving team scrambles to maintain balanced matches and prevent steamrolling.

## Features

- **Automatic Win Streak Tracking**: Monitors dominant victories and triggers scrambles when thresholds are reached
- **Squad Preservation**: Intelligent scrambling algorithm that keeps squads together when possible
- **Single Round Scramble**: Optional "mercy rule" for extremely unbalanced rounds
- **Game Mode Aware**: Different dominance thresholds for Standard (RAAS/AAS) and Invasion modes
- **Manual Scramble Commands**: Admins can trigger scrambles on demand with dry-run support
- **Persistent State**: Win streaks survive server restarts
- **Countdown System**: Configurable delay before scrambles execute with cancellation support
- **Comprehensive Chat Commands**: Full admin control via in-game commands
- **Detailed Messaging**: Contextual win messages and scramble announcements

## Configuration Options

### Core Settings

| Option | Description | Default | Type |
|--------|-------------|---------|------|
| `enable_win_streak_tracking` | Enable/disable automatic win streak tracking | true | bool |
| `max_win_streak` | Number of dominant wins to trigger a scramble | 2 | int |
| `enable_single_round_scramble` | Enable mercy rule for extremely unbalanced rounds | false | bool |
| `single_round_scramble_threshold` | Ticket margin to trigger single-round scramble | 250 | int |
| `min_tickets_dominant_win` | Minimum ticket difference for dominant win (Standard modes) | 150 | int |
| `invasion_attack_threshold` | Ticket difference for Attackers to be dominant (Invasion) | 300 | int |
| `invasion_defence_threshold` | Ticket difference for Defenders to be dominant (Invasion) | 650 | int |

### Scramble Execution

| Option | Description | Default | Type |
|--------|-------------|---------|------|
| `scramble_announcement_delay` | Seconds before scramble executes (min: 10) | 12 | int |
| `scramble_percentage` | Percentage of players to move (0.0 - 1.0) | "0.5" | string |
| `change_team_retry_interval` | Retry interval (ms) for player swaps (min: 200) | 200 | int |
| `max_scramble_completion_time` | Max time (ms) for all swaps to complete (min: 5000) | 15000 | int |
| `warn_on_swap` | Send warning message to swapped players | true | bool |

### Messaging & Display

| Option | Description | Default | Type |
|--------|-------------|---------|------|
| `show_win_streak_messages` | Broadcast win streak messages to all players | true | bool |
| `use_generic_team_names` | Use "Team 1"/"Team 2" instead of faction names | false | bool |
| `message_prefix` | Prefix for all broadcast messages | ">>> " | string |

## Chat Commands

### Public Commands

- `!teambalancer` - View current win streak and status

### Admin Commands

**Plugin Control:**

- `!teambalancer status` - View win streak and plugin status
- `!teambalancer on` - Enable win streak tracking
- `!teambalancer off` - Disable win streak tracking
- `!teambalancer diag` - Run diagnostics (performs dry-run scramble)

**Scramble Commands:**

- `!scramble` - Manually trigger scramble with countdown
- `!scramble now` - Immediate scramble (no countdown)
- `!scramble dry` - Dry-run scramble (simulation only, no players moved)
- `!scramble cancel` - Cancel pending scramble countdown

## How It Works

### Win Streak Tracking

1. **Round Ends**: Plugin analyzes final ticket counts
2. **Dominance Check**: Determines if win was dominant based on game mode:
   - **Standard Modes** (RAAS/AAS/TC): Margin ≥ 150 tickets (configurable)
   - **Invasion - Attackers**: Margin ≥ 300 tickets (configurable)
   - **Invasion - Defenders**: Margin ≥ 650 tickets (configurable)
3. **Streak Update**: Increments counter if same team wins dominantly, resets otherwise
4. **Threshold Check**: Triggers scramble if streak reaches max (default: 2 wins)

### Scrambling Algorithm

The plugin uses a 5-stage squad-preserving algorithm:

1. **Data Preparation**: Converts squads and unassigned players into groups
2. **Target Calculation**: Determines how many players to move based on scramble percentage
3. **Squad Selection**: Prioritizes moving:
   - Unassigned players (highest priority)
   - Small squads (less disruption)
   - Unlocked squads (avoids breaking locked squads)
   - Players from winning team
4. **Execution**: Moves entire squads together when possible
5. **Retry Logic**: Retries failed moves with exponential backoff

### Team Flipping

After each new game, team IDs are flipped (1↔2) since teams swap sides on the map. The win streak automatically tracks the correct team through map changes.

## Example Configuration

```json
{
  "plugin": "team_balancer",
  "enabled": true,
  "enable_win_streak_tracking": true,
  "max_win_streak": 2,
  "enable_single_round_scramble": false,
  "single_round_scramble_threshold": 250,
  "min_tickets_dominant_win": 150,
  "invasion_attack_threshold": 300,
  "invasion_defence_threshold": 650,
  "scramble_announcement_delay": 12,
  "scramble_percentage": "0.5",
  "change_team_retry_interval": 200,
  "max_scramble_completion_time": 15000,
  "show_win_streak_messages": true,
  "warn_on_swap": true,
  "use_generic_team_names": false,
  "message_prefix": ">>> "
}
```

## Scramble Scenarios

### Automatic Scramble (Win Streak)

```sh
Round 1: Team 1 wins by 200 tickets → Dominant win (streak: 1)
Round 2: Team 1 wins by 180 tickets → Dominant win (streak: 2)
>>> Team 1 has won 2 dominant rounds in a row! Teams will be scrambled in 12 seconds...
[12 seconds later]
>>> Executing team scramble now...
>>> Team scramble complete! Good luck and have fun!
```

### Single Round Scramble (Mercy Rule)

```sh
Round 1: Team 2 wins by 300 tickets → Mercy rule triggered
>>> Extremely unbalanced round detected (margin: 300 tickets)! Teams will be scrambled in 12 seconds.
```

### Manual Scramble

```sh
Admin: !scramble
>>> Admin has initiated a team scramble. Scrambling in 12 seconds...
>>> Executing team scramble now...
>>> Team scramble complete! Good luck and have fun!
```

## Win Message Examples

### Non-Dominant Wins

- "The USA secured a narrow victory over The RUS by 45 tickets. Well fought!"
- "The INS achieved a marginal victory over The MIL (78 ticket margin)."
- "The CAF gained a tactical advantage over The MEA (120 tickets)."
- "The USA demonstrated operational superiority over The RUS (145 tickets)."

### Dominant Wins

- "The USA steamrolled The RUS with a 200 ticket margin!"
- "The MEA completely stomped The CAF (280 tickets)!"

### Invasion-Specific

- "Attackers (The USA) captured objectives with 350 tickets remaining."
- "Defenders (The RUS) held the line with 720 tickets remaining."
- "Attackers (The INS) dominated with 450 tickets remaining!"
- "Defenders (The MIL) crushed the attack with 800 tickets remaining!"

## Important Notes

### Squad Preservation

- **Entire Squads Moved**: When possible, squads are moved together to preserve team cohesion
- **Locked Squads**: Locked squads are avoided unless necessary for balance
- **Unassigned Priority**: Unassigned players are moved first to minimize squad disruption
- **Squad Leaders**: Algorithm tries to preserve squad leader positions when possible

### State Persistence

- **Win Streaks Saved**: Streak data persists through server restarts
- **Staleness Check**: Streaks older than 2 hours are automatically reset
- **Database Storage**: Uses plugin database API for reliable persistence

### Safety Features

- **One Scramble at a Time**: Cannot start new scramble while one is pending or executing
- **Countdown Cancellation**: Admins can cancel scrambles during countdown
- **Graceful Failures**: Failed player moves are retried, remaining players still moved
- **Config Validation**: Invalid configuration values are auto-corrected with warnings

### Performance

- **Retry Logic**: Failed team changes retried up to 5 times per player
- **Timeout Protection**: Scramble execution has maximum completion time (15s default)
- **Concurrent Execution**: Player moves executed concurrently for faster completion

## Tips

### For Server Owners

- **Test First**: Use `!scramble dry` to test scramble logic without moving players
- **Adjust Thresholds**: Fine-tune dominance thresholds based on your server's skill distribution
- **Invasion Tuning**: Invasion modes need different thresholds due to asymmetric gameplay
- **Mercy Rule**: Enable single-round scramble if you see frequent blowouts
- **Message Customization**: Adjust `message_prefix` to match your server's style

### For Administrators

- **Status Check**: Use `!teambalancer status` to see current streak before scrambling
- **Dry Runs**: Always test with `!scramble dry` before manual scrambles
- **Countdown**: Give players time to finish objectives before scramble executes
- **Cancel Option**: Use `!scramble cancel` if scramble triggered at wrong time
- **Diagnostics**: Run `!teambalancer diag` if scrambling isn't working as expected

### Recommended Settings

**Competitive Server (High Skill):**

```json
{
  "max_win_streak": 2,
  "min_tickets_dominant_win": 175,
  "scramble_percentage": "0.6",
  "enable_single_round_scramble": true,
  "single_round_scramble_threshold": 300
}
```

**Casual Server (Mixed Skill):**

```json
{
  "max_win_streak": 3,
  "min_tickets_dominant_win": 150,
  "scramble_percentage": "0.5",
  "enable_single_round_scramble": false
}
```

**New Player Friendly:**

```json
{
  "max_win_streak": 2,
  "min_tickets_dominant_win": 125,
  "scramble_percentage": "0.4",
  "enable_single_round_scramble": true,
  "single_round_scramble_threshold": 250
}
```

## Troubleshooting

### Win Streak Not Tracking

1. Check if tracking is enabled: `!teambalancer status`
2. Verify game mode is supported (not Training mode)
3. Check if admin disabled tracking: `!teambalancer on`

### Scramble Not Executing

1. Verify players are online (need players to scramble)
2. Check for pending/in-progress scrambles
3. Run diagnostics: `!teambalancer diag`
4. Check server logs for errors

### Incorrect Team Names

- Plugin extracts team names from player roles
- May take 10-15 seconds after round start to populate
- Use `use_generic_team_names: true` for consistent "Team 1"/"Team 2"

### Players Not Moving

- Check RCON connection is stable
- Increase `max_scramble_completion_time` if many players
- Increase `change_team_retry_interval` if server is laggy

## Credits

Ported from the original SquadJS Team Balancer plugin by **Slacker** (Discord: `real_slacker`).

Original JavaScript version: [github.com/mikebjoyce/squadjs-team-balancer](https://github.com/mikebjoyce/squadjs-team-balancer)
