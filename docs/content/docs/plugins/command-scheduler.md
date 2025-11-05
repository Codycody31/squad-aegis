---
title: Command Scheduler
---

The Command Scheduler plugin allows you to automate the execution of RCON commands at specified intervals or in response to game events, helping you maintain server configuration and perform routine administrative tasks automatically.

## Features

- Schedule commands to run at regular intervals
- Execute commands after specific game events (like new games)
- Support for multiple scheduled commands
- Individual enable/disable control for each command
- Persistent timing across server restarts

## Configuration Options

| Option | Description | Default | Required |
|--------|-------------|---------|----------|
| `commands` | Array of command configurations | See example below | Yes |

Each command in the array has the following properties:

| Property | Description | Required |
|----------|-------------|----------|
| `name` | Unique identifier for the command | Yes |
| `command` | The RCON command to execute | Yes |
| `enabled` | Whether this command is enabled | No |
| `interval` | Interval in seconds between executions | No |
| `on_new_game` | Execute after new game events | No |

## How It Works

1. The plugin maintains a schedule for each configured command
2. Commands can be triggered by:
   - **Time intervals**: Execute every X seconds
   - **Game events**: Execute when a new game starts
   - **Both**: Execute on intervals AND after new games
3. Commands are executed through the RCON interface
4. The plugin tracks execution times and schedules next runs

## Example Configuration

```json
{
  "commands": [
    {
      "name": "reload_config",
      "command": "AdminReloadServerConfig",
      "enabled": true,
      "interval": 600,
      "on_new_game": true
    },
    {
      "name": "list_players",
      "command": "ListPlayers",
      "enabled": true,
      "interval": 300,
      "on_new_game": false
    },
    {
      "name": "force_team_balance",
      "command": "AdminForceTeamChangeBalance",
      "enabled": false,
      "interval": 60,
      "on_new_game": true
    }
  ]
}
```

## Common Use Cases

- **Server maintenance**: Reload configuration files periodically
- **Player monitoring**: List players at regular intervals
- **Game management**: Force team balance after new games
- **Administrative tasks**: Send periodic broadcasts or warnings
- **Log management**: Clear old logs or generate reports

## Tips

- Use descriptive names for commands to make management easier
- Start with longer intervals and adjust based on server needs
- Combine interval and event-based execution for comprehensive automation
- Test commands manually before scheduling them
- Use the `enabled` flag to temporarily disable commands without removing configuration
- Consider server performance when scheduling frequent commands
