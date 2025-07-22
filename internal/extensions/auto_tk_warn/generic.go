package auto_tk_warn

import (
	"fmt"

	"go.codycody31.dev/squad-aegis/internal/extension_manager"
	"go.codycody31.dev/squad-aegis/internal/shared/plug_config_schema"
)

// AutoTKWarnExtension automatically warns players when they teamkill
type AutoTKWarnExtension struct {
	extension_manager.ExtensionBase
}

// Define implements ExtensionRegistrar
func Define() extension_manager.ExtensionDefinition {
	return extension_manager.ExtensionDefinition{
		ID:                     "auto_tk_warn",
		Name:                   "Auto TK Warn",
		Description:            "Automatically warns players with a message when they teamkill.",
		Version:                "1.0.0",
		Author:                 "Squad Aegis",
		AllowMultipleInstances: false,

		Dependencies: extension_manager.ExtensionDependencies{
			Required: []extension_manager.DependencyType{
				extension_manager.DependencyServer,
				extension_manager.DependencyConnectors,
				extension_manager.DependencyRconManager,
			},
		},

		// Required connectors
		RequiredConnectors: []string{"logwatcher"},

		ConfigSchema: plug_config_schema.ConfigSchema{
			Fields: []plug_config_schema.ConfigField{
				{
					Name:        "attacker_message",
					Description: "The message to warn attacking players with.",
					Required:    false,
					Type:        plug_config_schema.FieldTypeString,
					Default:     "Please apologise for ALL TKs in ALL chat!",
				},
				{
					Name:        "victim_message",
					Description: "The message that will be sent to the victim.",
					Required:    false,
					Type:        plug_config_schema.FieldTypeString,
					Default:     nil, // Default is null
				},
			},
		},

		EventHandlers: []extension_manager.EventHandler{
			{
				Source:      extension_manager.EventHandlerSourceCONNECTOR,
				Name:        "TEAMKILL",
				Description: "Warns players when they teamkill",
				Handler: func(e extension_manager.Extension, data interface{}) error {
					return e.(*AutoTKWarnExtension).handleTeamkill(data)
				},
			},
		},
		CreateInstance: func() extension_manager.Extension {
			return &AutoTKWarnExtension{}
		},
	}
}

// Initialize initializes the extension with its configuration and dependencies
func (e *AutoTKWarnExtension) Initialize(config map[string]interface{}, deps *extension_manager.Dependencies) error {
	// Set the base extension properties
	e.Definition = Define()
	e.Config = config
	e.Deps = deps

	// Validate config
	if err := e.Definition.ConfigSchema.Validate(config); err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}

	// Fill defaults
	e.Definition.ConfigSchema.FillDefaults(config)

	return nil
}

// Shutdown gracefully shuts down the extension
func (e *AutoTKWarnExtension) Shutdown() error {
	// Nothing to clean up for this extension
	return nil
}
