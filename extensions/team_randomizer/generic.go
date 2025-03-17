package team_randomizer

import (
	"fmt"

	"go.codycody31.dev/squad-aegis/internal/extension_manager"
	"go.codycody31.dev/squad-aegis/internal/rcon_manager"
)

// TeamRandomizerExtension randomizes team assignments
type TeamRandomizerExtension struct {
	extension_manager.ExtensionBase
	rconManager *rcon_manager.RconManager
}

// Define implements ExtensionRegistrar
func Define() extension_manager.ExtensionDefinition {
	return extension_manager.ExtensionDefinition{
		ID:                     "team_randomizer",
		Name:                   "Team Randomizer",
		Description:            "Used to randomize teams. It's great for destroying clan stacks or for social events. It can be run by typing !randomize into in-game admin chat.",
		Version:                "1.0.0",
		Author:                 "Squad Aegis",
		AllowMultipleInstances: false,

		Dependencies: extension_manager.ExtensionDependencies{
			Required: []extension_manager.DependencyType{
				extension_manager.DependencyRconManager,
			},
		},

		EventHandlers: []extension_manager.EventHandler{
			{
				Source:      extension_manager.EventHandlerSourceRCON,
				Name:        "CHAT_COMMAND",
				Description: "Handles team randomization requests from in-game chat",
				Handler: func(e extension_manager.Extension, data interface{}) error {
					return e.(*TeamRandomizerExtension).handleTeamRandomizationRequest(data)
				},
			},
		},

		CreateInstance: func() extension_manager.Extension {
			return &TeamRandomizerExtension{}
		},
	}
}

// Initialize initializes the extension with its configuration and dependencies
func (e *TeamRandomizerExtension) Initialize(config map[string]interface{}, deps *extension_manager.Dependencies) error {
	// Set the base extension properties
	e.Definition = Define()
	e.Config = config
	e.Deps = deps

	// Get RCON manager from dependencies
	if e.Deps.RconManager == nil {
		return fmt.Errorf("RCON manager dependency not provided")
	}
	e.rconManager = e.Deps.RconManager

	// Validate config
	if err := e.Definition.ConfigSchema.Validate(config); err != nil {
		return err
	}

	// Fill defaults
	e.Definition.ConfigSchema.FillDefaults(config)

	return nil
}

// Shutdown gracefully shuts down the extension
func (e *TeamRandomizerExtension) Shutdown() error {

	return nil
}
