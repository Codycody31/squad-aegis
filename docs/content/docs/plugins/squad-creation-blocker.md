---
title: Squad Creation Blocker
---

The Squad Creation Blocker plugin prevents squads with custom names from being created within a specified time after a new game starts and at the end of a round. It includes anti-spam rate limiting with configurable warnings, cooldowns, and kick functionality to prevent players from overwhelming the system.

## Features

- **Timed Blocking**: Prevents custom squad creation for a configurable period after new games
- **Round End Blocking**: Blocks custom squads during round transitions
- **Default Squad Names**: Optionally allows default squad names (e.g., "Squad 1") during blocking periods
- **Anti-Spam Rate Limiting**: Tracks and penalizes repeated squad creation attempts
- **Cooldown System**: Temporarily blocks players who spam squad creation
- **Progressive Warnings**: Escalating warnings before applying penalties
- **Auto-Kick**: Optional automatic kick for excessive spamming
- **Flexible Scope**: Apply rate limiting only during blocking periods or throughout entire matches
- **Countdown Broadcasts**: Optional public countdown announcements
- **Polling System**: Detects and disbands squads created outside the event system

## Configuration Options

### Core Settings

| Option | Description | Default | Type |
|--------|-------------|---------|------|
| `block_duration` | Time period after new game starts to block custom squads (seconds) | 15 | int |
| `broadcast_mode` | Use countdown broadcasts instead of individual warnings | false | bool |
| `allow_default_squad_names` | Allow default names like "Squad 1" during blocking period | true | bool |

### Rate Limiting

| Option | Description | Default | Type |
|--------|-------------|---------|------|
| `enable_rate_limiting` | Enable anti-spam rate limiting | true | bool |
| `rate_limiting_scope` | When to apply: "blocking_period_only" or "entire_match" | "blocking_period_only" | string |
| `warning_threshold` | Attempts before warnings start | 3 | int |
| `cooldown_duration` | Cooldown duration after threshold (seconds) | 10 | int |
| `kick_threshold` | Attempts before kicking player (0 to disable) | 20 | int |
| `reset_on_attempt` | Reset cooldown timer on each new attempt | false | bool |

### Advanced Settings

| Option | Description | Default | Type |
|--------|-------------|---------|------|
| `poll_interval` | Interval for squad checking (seconds) | 1 | int |
| `cooldown_warning_interval` | Interval for cooldown reminders (seconds) | 3 | int |

## How It Works

### Blocking Periods

1. **New Game Start**: When a new game starts, custom squad creation is blocked for `block_duration` seconds
2. **Round End**: When a round ends, all custom squad creation is blocked until the next game starts
3. **Default Names**: If `allow_default_squad_names` is true, players can still create squads with names like "Squad 1", "Squad 2", etc.

### Rate Limiting System

The plugin tracks each player's squad creation attempts and applies progressive penalties:

1. **Attempts 1-3** (default): Squad disbanded, warning sent
2. **Attempts 4+**: Player enters cooldown
3. **During Cooldown**: All squad creation blocked, periodic reminders sent
4. **After Threshold**: Player kicked if kick threshold reached

### Cooldown Behavior

Two cooldown modes are available via `reset_on_attempt`:

- **False (default)**: Cooldown must fully expire before new attempts trigger rate limiting
- **True**: Each new attempt resets the cooldown timer (more aggressive)

### Rate Limiting Scope

- **blocking_period_only** (default): Rate limiting only applies during new game blocking periods
- **entire_match**: Rate limiting active throughout the entire match

## Example Configurations

### Default Configuration (Recommended)

```json
{
  "block_duration": 15,
  "broadcast_mode": false,
  "allow_default_squad_names": true,
  "enable_rate_limiting": true,
  "rate_limiting_scope": "blocking_period_only",
  "warning_threshold": 3,
  "cooldown_duration": 10,
  "kick_threshold": 20,
  "poll_interval": 1,
  "cooldown_warning_interval": 3,
  "reset_on_attempt": false
}
```

### Strict Configuration (Aggressive Anti-Spam)

For servers with persistent spam issues:

```json
{
  "block_duration": 20,
  "broadcast_mode": true,
  "allow_default_squad_names": true,
  "enable_rate_limiting": true,
  "rate_limiting_scope": "entire_match",
  "warning_threshold": 2,
  "cooldown_duration": 15,
  "kick_threshold": 10,
  "poll_interval": 1,
  "cooldown_warning_interval": 5,
  "reset_on_attempt": true
}
```

### Lenient Configuration (Minimal Blocking)

For servers that want minimal interference:

```json
{
  "block_duration": 10,
  "broadcast_mode": false,
  "allow_default_squad_names": true,
  "enable_rate_limiting": false,
  "rate_limiting_scope": "blocking_period_only",
  "warning_threshold": 5,
  "cooldown_duration": 5,
  "kick_threshold": 0,
  "poll_interval": 2,
  "cooldown_warning_interval": 3,
  "reset_on_attempt": false
}
```

### Broadcast Mode Configuration

For servers that want public announcements:

```json
{
  "block_duration": 30,
  "broadcast_mode": true,
  "allow_default_squad_names": true,
  "enable_rate_limiting": true,
  "rate_limiting_scope": "blocking_period_only",
  "warning_threshold": 3,
  "cooldown_duration": 10,
  "kick_threshold": 15,
  "poll_interval": 1,
  "cooldown_warning_interval": 3,
  "reset_on_attempt": false
}
```

## Player Experience

### Normal Player

1. New game starts
2. Player tries to create "Alpha Squad"
3. Squad is disbanded immediately
4. Player receives warning: "Please wait for 12 seconds before creating a custom squad. Default names (e.g. 'Squad 1') are allowed."
5. Player creates "Squad 1" - allowed
6. After 15 seconds, blocking period ends
7. Player can now create custom named squads

### Spamming Player

1. Player repeatedly tries to create custom squads during blocking period
2. **Attempt 1-3**: Warnings about approaching cooldown
   - "Warning: Stop spamming squad creation! 2 more attempts before cooldown."
3. **Attempt 4**: Cooldown applied
   - "You are on cooldown for 10s due to squad creation spam. Stop spamming or you will be kicked!"
4. **During cooldown**: Periodic reminders
   - "Squad creation cooldown: 7 seconds remaining."
5. **After cooldown expires**: Can create squads again
6. **If spamming continues**: Player kicked at attempt 20 (if configured)

### Broadcast Mode Experience

When `broadcast_mode` is enabled, all players see countdown messages:

```
[At 30s] Custom squad names unlock in 30s. Default names (e.g. "Squad 1") are allowed. Spammers get 10s cooldown.
[At 20s] Custom squad names unlock in 20s. Default names (e.g. "Squad 1") are allowed. Spammers get 10s cooldown.
[At 10s] Custom squad names unlock in 10s. Default names (e.g. "Squad 1") are allowed. Spammers get 10s cooldown.
[At 0s] Custom squad creation is now unlocked!
```

## Use Cases

### Prevent Early Game Chaos

Long block duration at game start gives players time to assess the map and plan properly:

```json
{
  "block_duration": 30,
  "broadcast_mode": true
}
```

### Stop Troll Names

Prevent inappropriate squad names during vulnerable periods:

```json
{
  "block_duration": 15,
  "allow_default_squad_names": true,
  "enable_rate_limiting": true
}
```

### Enforce Naming Standards

Combined with other plugins, encourage proper squad naming:

```json
{
  "rate_limiting_scope": "entire_match",
  "warning_threshold": 2,
  "cooldown_duration": 30
}
```

## Important Notes

### Squad Name Detection

- **Default Names**: Matched using regex pattern `^[Ss]quad \d+$`
- **Case Insensitive**: "Squad 1", "squad 1", "SQUAD 1" all count as default
- **Number Required**: "Squad Alpha" is NOT a default name
- **Spacing Matters**: "Squad1" (no space) is NOT a default name

### Event System + Polling

The plugin uses a dual detection system:

1. **RCON Events**: Catches most squad creations immediately
2. **Polling System**: Backup detection for squads that slip through
3. **Known Squads**: Tracks existing squads to avoid re-processing

### Rate Limiting Persistence

- **Per-Match**: Rate limiting data resets at new games (if scope is `blocking_period_only`)
- **Per-Player**: Each player tracked independently
- **Cooldown Warnings**: Automatic cleanup when cooldowns expire
- **Memory Efficient**: Data cleared when players are kicked or cooldowns expire

### Performance Considerations

- **Polling Interval**: Lower values (1s) provide better detection but use more resources
- **Cooldown Warnings**: Sent periodically to remind players without spamming
- **Squad Tracking**: Efficient map-based tracking minimizes overhead

## Troubleshooting

### Players Creating Custom Squads During Block Period

1. Check if `block_duration` has expired
2. Verify plugin is enabled and running
3. Check if squad name is actually a default name
4. Review server logs for RCON errors
5. Ensure `poll_interval` is not too high

### Players Not Receiving Warnings

1. Verify `broadcast_mode` setting matches expectation
2. Check RCON connection is stable
3. Ensure player Steam ID is being captured correctly
4. Review rate limiting configuration

### Squads Not Being Disbanded

1. Check RCON permissions for AdminDisbandSquad command
2. Verify team ID and squad ID parsing is correct
3. Enable debug logging to see disband commands
4. Test RCON connection manually

### Rate Limiting Not Working

1. Verify `enable_rate_limiting` is true
2. Check `rate_limiting_scope` matches your desired behavior
3. Ensure `warning_threshold` is set appropriately
4. Review polling system functionality

### False Positives

If default squads are being blocked:

1. Check `allow_default_squad_names` is true
2. Verify squad name format matches regex pattern
3. Test with exact names: "Squad 1", "Squad 2", etc.
4. Check for extra spaces or special characters

## Tips

### For Server Owners

- **Start Conservative**: Use default settings first, then adjust based on player behavior
- **Monitor Logs**: Watch for spam patterns to tune thresholds
- **Test Thoroughly**: Create test squads to verify blocking works as expected
- **Communicate**: Tell players about the blocking period in server rules
- **Adjust Duration**: Match `block_duration` to your server's typical setup time

### For Administrators

- **Watch New Games**: Monitor first 30 seconds of new games for issues
- **Track Spammers**: Note Steam IDs of repeat offenders for potential bans
- **Balance Settings**: Too strict = frustrated players, too lenient = chaos
- **Consider Broadcast**: Public countdowns help players understand the system
- **Test Rate Limits**: Verify cooldowns work before enabling entire-match scope

### Recommended Settings by Server Type

**Competitive/Clan Server:**

```json
{
  "block_duration": 20,
  "enable_rate_limiting": true,
  "rate_limiting_scope": "blocking_period_only",
  "warning_threshold": 3,
  "kick_threshold": 15
}
```

**Public Server:**

```json
{
  "block_duration": 15,
  "broadcast_mode": true,
  "enable_rate_limiting": true,
  "warning_threshold": 3,
  "kick_threshold": 20
}
```

**Casual Server:**

```json
{
  "block_duration": 10,
  "enable_rate_limiting": false,
  "allow_default_squad_names": true
}
```

## Technical Details

### Event Subscriptions

- `RCON_SQUAD_CREATED`: Primary detection for squad creation
- `LOG_GAME_EVENT_UNIFIED`: Detection for NEW_GAME and ROUND_ENDED events

### RCON Commands Used

- `AdminDisbandSquad <teamID> <squadID>`: Disband squads
- `AdminWarn "<steamID>" <message>`: Send warnings to players
- `AdminKick "<steamID>" <reason>`: Kick players for excessive spam
- `AdminBroadcast <message>`: Send server-wide announcements

### Timing Precision

- Block duration: Accurate to the second
- Cooldown timers: Updated in real-time
- Polling: Configurable precision (1-second minimum recommended)
- Warning intervals: Configurable, prevents spam

## Credits

Ported from the [squadjs-squad-creation-blocker](https://github.com/lbzepoqo/squadjs-squad-creation-blocker) plugin.
