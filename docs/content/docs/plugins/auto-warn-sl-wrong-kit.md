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
6. A squad leader has the "wrong kit" if their role does NOT contain any of these SL indicators:
   - `_SL_` (e.g., "USA_SL_Rifleman")
   - `_SL` (e.g., "USA_SL")
   - `SL_` (e.g., "SL_Rifleman")
   - `SL` (e.g., "SL")
7. If a squad leader has the wrong kit, they are added to the tracking list
8. Warning messages are sent at the configured `frequency_of_warnings` interval
9. Each warning includes the time remaining before action is taken (formatted as MM:SS)
10. If the squad leader doesn't change their kit within `wrong_kit_timer` seconds, action is taken:
    - If `should_kick` is `true`, the player is kicked from the server
    - If `should_kick` is `false`, the player is removed from their squad using `AdminRemovePlayerFromSquadById`
11. If a squad leader changes their kit (now contains SL indicators) or is no longer a squad leader, tracking stops immediately
12. Disconnected players are automatically cleaned up at the `cleanup_interval`

## Example Configuration

**Kick Mode (Recommended for Active Servers):**
```json
{
  "warning_message": "Squad Leaders are required to have an SL kit. Change your kit or you will be kicked",
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

**Remove from Squad Mode (Less Aggressive):**
```json
{
  "warning_message": "Squad Leaders are required to have an SL kit. Change your kit or you will be removed from your squad",
  "kick_message": "Squad Leader with wrong kit - automatically removed",
  "should_kick": false,
  "frequency_of_warnings": 45,
  "wrong_kit_timer": 600,
  "player_threshold": 93,
  "round_start_delay": 900,
  "tracking_update_interval": 60,
  "cleanup_interval": 1200
}
```

**Always Active (No Threshold):**
```json
{
  "warning_message": "Squad Leaders are required to have an SL kit. Change your kit or you will be kicked",
  "kick_message": "Squad Leader with wrong kit - automatically removed",
  "should_kick": true,
  "frequency_of_warnings": 30,
  "wrong_kit_timer": 300,
  "player_threshold": -1,
  "round_start_delay": 600,
  "tracking_update_interval": 30,
  "cleanup_interval": 1200
}
```

## Kit Detection

The plugin determines if a squad leader has the "wrong kit" by checking their role string. A squad leader has the **correct kit** if their role contains any of these patterns:

- `_SL_` (e.g., "USA_SL_Rifleman", "RUS_SL_Medic")
- `_SL` (e.g., "USA_SL", "RUS_SL")
- `SL_` (e.g., "SL_Rifleman", "SL_Medic")
- `SL` (e.g., "SL", "SLRifleman")

If the role does **not** contain any of these patterns, the squad leader is considered to have the wrong kit and will be tracked.

## Warning Messages

Warning messages are sent privately to the squad leader via RCON and include:

- The configured `warning_message`
- A dash separator (`-`)
- The time remaining before action is taken (formatted as MM:SS)

**Example:** "Squad Leaders are required to have an SL kit. Change your kit or you will be kicked - 04:30"

The time remaining is automatically calculated and formatted. Warnings stop being sent when less than one warning interval remains before the kick timer expires.

## Player Threshold

The `player_threshold` setting controls when the plugin becomes active:

- If set to a positive number (e.g., 93), the plugin only tracks and kicks when there are at least that many players online
- If set to `-1`, the threshold is disabled and the plugin works regardless of player count
- This prevents kicking during low population periods when squad formation may be more flexible
- When the threshold is not met, all currently tracked players are immediately untracked
- The threshold check happens at each `tracking_update_interval`

## Action Types

The plugin supports two action types when a squad leader exceeds the timer:

**Kick Mode (`should_kick: true`):**
- Player is kicked from the server using `AdminKick`
- Kick reason uses the configured `kick_message`
- Player must reconnect to rejoin the server

**Remove from Squad Mode (`should_kick: false`):**
- Player is removed from their squad using `AdminRemovePlayerFromSquadById`
- Player remains on the server and can rejoin a squad
- Less disruptive but may be less effective at enforcing kit requirements

## Tracking Behavior

- Players are tracked individually with their own warning tickers and kick timers
- If a player changes their kit to include SL indicators, tracking stops immediately
- If a player is no longer a squad leader, tracking stops immediately
- Disconnected players are cleaned up periodically (every `cleanup_interval` seconds)
- All tracking is reset when a new game/round starts

## Round Start Delay

When a new game/round starts (detected via `NEW_GAME` event):

- All current tracking is immediately stopped
- All active warning tickers and kick timers are cancelled
- A grace period begins (configured by `round_start_delay` in seconds)
- During this grace period, no squad leaders are tracked or warned
- The plugin sets `betweenRounds` flag to `true` during the grace period
- After the grace period expires, normal monitoring resumes
- This allows players time to change kits after round transitions without being penalized

**Note:** The grace period timer runs independently and will complete even if the plugin is restarted (as long as the server remains running).
