---
title: Auto Kick SL Wrong Kit
---

The Auto Kick SL Wrong Kit plugin automatically warns and kicks squad leaders that have the wrong kit for longer than a specified amount of time. This helps maintain proper squad leadership by ensuring squad leaders use appropriate kits.

## Features

- Automatically detects squad leaders with incorrect kits
- Sends periodic warning messages before taking action
- Configurable kick timer and warning frequency
- Option to kick players or remove them from squads
- Player count threshold to prevent kicking during low population
- Round start delay to allow kit changes after round transitions
- Automatic cleanup of disconnected players
- Grace period after round starts before monitoring resumes

## Configuration Options

| Option | Description | Default | Required |
|--------|-------------|---------|----------|
| `warning_message` | Message to send to players warning them they will be kicked | "Squad Leaders are required to have an SL kit. Change your kit or you will be kicked" | No |
| `kick_message` | Message to send to players when they are kicked | "Squad Leader with wrong kit - automatically removed" | No |
| `should_kick` | If true, kick the player. If false, remove them from the squad | false | No |
| `frequency_of_warnings` | How often in seconds should we warn the Squad Leader about having the wrong kit? | 30 | No |
| `wrong_kit_timer` | How long in seconds to wait before a Squad Leader with wrong kit is kicked | 300 | No |
| `player_threshold` | Player count required for AutoKick to start kicking Squad Leaders, set to -1 to disable | 93 | No |
| `round_start_delay` | Time delay in seconds from start of the round before AutoKick starts kicking Squad Leaders again | 900 | No |
| `tracking_update_interval` | How often in seconds to update the tracking list of Squad Leaders with wrong kits | 60 | No |
| `cleanup_interval` | How often in seconds to clean up disconnected Squad Leaders from tracking | 1200 | No |

## How It Works

1. The plugin continuously monitors all players on the server at the configured `tracking_update_interval`
2. When a new game/round starts, all tracking is paused and a grace period begins
3. After the `round_start_delay` expires, monitoring resumes
4. The plugin checks if the player threshold is met (or disabled with -1)
5. For each online player, it checks if they are a squad leader
6. If a squad leader's role doesn't contain SL indicators, they are added to the tracking list
7. Warning messages are sent at the configured `frequency_of_warnings` interval
8. Each warning includes the time remaining before action is taken
9. If the squad leader doesn't change their kit within `wrong_kit_timer` seconds, action is taken:
   - If `should_kick` is `true`, the player is kicked from the server
   - If `should_kick` is `false`, the player is removed from their squad
10. If a squad leader changes their kit or is no longer a squad leader, tracking stops
11. Disconnected players are automatically cleaned up at the `cleanup_interval`

## Example Configuration

```json
{
  "warning_message": "Squad Leaders are required to have an SL kit. Change your kit or you will be kicked in {time}",
  "kick_message": "Squad Leader with wrong kit - automatically removed",
  "should_kick": true,
  "frequency_of_warnings": 30,
  "wrong_kit_timer": 300,
  "player_threshold": 80,
  "round_start_delay": 900,
  "tracking_update_interval": 60,
  "cleanup_interval": 1200
}
```

## Warning Messages

Warning messages are sent privately to the squad leader and include:

- The configured `warning_message`
- The time remaining before action is taken (formatted as MM:SS)

Example: "Squad Leaders are required to have an SL kit. Change your kit or you will be kicked - 04:30"

## Player Threshold

The `player_threshold` setting controls when the plugin becomes active:

- If set to a positive number (e.g., 93), the plugin only tracks and kicks when there are at least that many players online
- If set to `-1`, the threshold is disabled and the plugin works regardless of player count
- This prevents kicking during low population periods when squad formation may be more flexible

## Round Start Delay

When a new game/round starts:

- All current tracking is stopped
- A grace period begins (configured by `round_start_delay`)
- During this grace period, no squad leaders are tracked or warned
- After the grace period expires, normal monitoring resumes
- This allows players time to change kits after round transitions
