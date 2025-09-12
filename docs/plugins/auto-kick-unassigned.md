---
title: Auto-Kick Unassigned Players
icon: lucide:plug
---

## Auto-Kick Unassigned Players

The Auto-Kick Unassigned plugin automatically kicks players that are not in a squad after a specified amount of time. This helps maintain server performance and encourages players to join organized gameplay.

### Features

- Automatically tracks unassigned players
- Sends warning messages before kicking
- Configurable kick timer and warning frequency
- Player count threshold to prevent kicking during low population
- Round start delay to allow squad formation
- Option to ignore admins
- Periodic cleanup of disconnected players

### Configuration Options

| Option | Description | Default | Required |
|--------|-------------|---------|----------|
| `warning_message` | Message sent to players warning them they will be kicked | "Join a squad, you are unassigned and will be kicked" | No |
| `kick_message` | Message sent to players when they are kicked | "Unassigned - automatically removed" | No |
| `frequency_of_warnings` | How often in seconds to warn the player | 30 | No |
| `unassigned_timer` | How long in seconds to wait before kicking an unassigned player | 360 | No |
| `player_threshold` | Player count required for AutoKick to start kicking (set to -1 to disable) | 93 | No |
| `round_start_delay` | Time delay in seconds from round start before AutoKick starts | 900 | No |
| `ignore_admins` | Whether admins should be ignored and not kicked | false | No |
| `tracking_update_interval` | How often in seconds to update the tracking list | 60 | No |
| `cleanup_interval` | How often in seconds to clean up disconnected players | 1200 | No |

### How It Works

1. The plugin monitors all players on the server
2. When a player is detected as unassigned (not in a squad), they are added to the tracking list
3. Warning messages are sent at regular intervals
4. If the player remains unassigned after the specified timer, they are automatically kicked
5. The plugin respects the player threshold - kicking only occurs when there are enough players online
6. After a round starts, there's a delay before kicking resumes to allow squad formation

### Example Configuration

```json
{
  "warning_message": "Join a squad or you will be kicked in 5 minutes!",
  "kick_message": "Kicked for being unassigned too long",
  "frequency_of_warnings": 60,
  "unassigned_timer": 300,
  "player_threshold": 80,
  "round_start_delay": 600,
  "ignore_admins": true
}
```

### Tips

- Set the `player_threshold` to prevent kicking during low population periods
- Use a reasonable `round_start_delay` to give players time to form squads at the beginning of rounds
- Customize the warning and kick messages to match your server's language and rules
- Consider enabling `ignore_admins` if you want administrators to be exempt from auto-kicking
