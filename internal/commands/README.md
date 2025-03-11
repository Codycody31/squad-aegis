# Squad Server Commands Package

This package provides functionality for managing and executing Squad game server commands.

## Features

- Defines both admin and public commands for Squad game servers
- Provides command categorization (kick, ban, chat, etc.)
- Includes command metadata (syntax, description, RCON support)
- Supports filtering commands by type (admin/public) and category
- Provides access control for command execution

## Usage

### Importing the Package

```go
import "go.codycody31.dev/squad-aegis/internal/commands"
```

### Command Types

Commands are categorized as either admin commands or public commands:

```go
// Admin command
commands.AdminCommand  // Value: 1

// Public command
commands.PublicCommand // Value: 0
```

### Accessing Commands

```go
// Get all commands
allCommands := commands.CommandMatrix

// Get commands by type
adminCommands := commands.GetCommandsByType(commands.AdminCommand)
publicCommands := commands.GetCommandsByType(commands.PublicCommand)

// Get commands by category
kickCommands := commands.GetCommandsByCategory("kick")
chatCommands := commands.GetCommandsByCategory("chat")

// Get a specific command
if cmd, found := commands.GetCommandByName("AdminKick"); found {
    // Use the command
}

// Check if a user can execute specific commands
executableCommands := commands.CommandsCanExecute(userPermissions, supportsRCON)
```

## Command Structure

Each command includes the following information:

- `SupportsRCON`: Whether the command can be executed via RCON
- `Name`: The name of the command
- `Category`: The category the command belongs to
- `Syntax`: The syntax for using the command
- `Description`: A description of what the command does
- `CommandType`: Whether it's an admin or public command

## Adding New Commands

To add new commands, simply append them to the `CommandMatrix` slice in `commands.go`:

```go
// Admin command example
{true, "AdminNewCommand", "category", "AdminNewCommand <param>", "Description", commands.AdminCommand}

// Public command example
{true, "PublicCommand", "category", "PublicCommand", "Description", commands.PublicCommand}
``` 