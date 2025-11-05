---
title: Team Randomizer
---

The Team Randomizer plugin allows administrators to randomize team assignments to break up clan stacks or for social events, promoting fair and mixed gameplay.

## Features

- Random team assignment for all players
- Chat command activation
- Admin-only access control
- Configurable command trigger
- Cooldown system to prevent abuse
- Optional announcement to all players

## Configuration Options

| Option | Description | Default | Required |
|--------|-------------|---------|----------|
| `command` | Chat command trigger (without !) | "randomize" | No |
| `admin_only` | Restrict to admins only | true | No |
| `require_admin_chat` | Require admin chat usage | true | No |
| `announce_randomization` | Announce to all players | true | No |
| `cooldown_seconds` | Cooldown between uses | 30 | No |

## How It Works

1. Administrator types the command in the appropriate chat
2. The plugin validates permissions and cooldown
3. All players are randomly reassigned to teams
4. Squad assignments are cleared (players become unassigned)
5. Optional announcement is broadcast to all players

## Example Configuration

```json
{
  "command": "randomteams",
  "admin_only": true,
  "require_admin_chat": true,
  "announce_randomization": true,
  "cooldown_seconds": 60
}
```

## Usage

- **Admin chat**: `!randomize` (if `require_admin_chat` is true)
- **All chat**: `!randomize` (if `require_admin_chat` is false, but admin_only should be true)

## Important Notes

- **Squad clearing**: All players become unassigned from squads after randomization
- **Immediate effect**: Randomization happens instantly
- **No undo**: Once executed, randomization cannot be reversed
- **All players**: Affects every player currently on the server

## Tips

- Use during clan matches to break up stacked teams
- Announce randomization in advance to prepare players
- Set appropriate cooldowns to prevent accidental spam
- Consider using admin-only mode for controlled execution
- Test the command in a private setting first
- Players will need to reform squads after randomization
