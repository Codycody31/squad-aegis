---
title: Fog of War
---

The Fog of War plugin automates the configuration of Squad's fog of war settings, allowing you to set the desired fog of war mode automatically after each new game or with a configurable delay.

## Features

- Automatic fog of war mode configuration
- Configurable delay after game start
- Support for all three fog of war modes
- Customizable RCON command template
- Event-driven execution

## Configuration Options

| Option | Description | Default | Required |
|--------|-------------|---------|----------|
| `mode` | Fog of war mode (0=Disabled, 1=Enabled, 2=Enemies only) | 1 | No |
| `delay_ms` | Delay before setting mode in milliseconds | 10000 | No |
| `enabled` | Whether the plugin is enabled | true | No |
| `command_template` | RCON command template (use {mode} placeholder) | "AdminSetFogOfWar {mode}" | No |

## Fog of War Modes

- **Mode 0**: Disabled - No fog of war, full map visibility
- **Mode 1**: Enabled - Standard fog of war (recommended)
- **Mode 2**: Enemies only - Fog of war affects only enemy positions

## How It Works

1. The plugin listens for new game events
2. After the configured delay, it executes the fog of war command
3. The command sets the desired fog of war mode for the match
4. The delay allows the game to fully initialize before applying settings

## Example Configuration

```json
{
  "mode": 1,
  "delay_ms": 15000,
  "command_template": "AdminSetFogOfWar {mode}"
}
```

## Tips

- Use a delay of 10-15 seconds to ensure the game has fully loaded
- Mode 1 (standard fog of war) is recommended for competitive play
- Mode 0 can be useful for training or casual servers
- The command template allows for custom RCON command formats if needed
- Test the configuration during a match to ensure proper timing
