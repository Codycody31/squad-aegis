package discord_cbl_info

import (
	"fmt"

	"go.codycody31.dev/squad-aegis/internal/connectors/discord"
	"go.codycody31.dev/squad-aegis/internal/extension_manager"
	"go.codycody31.dev/squad-aegis/internal/shared/plug_config_schema"
)

// DiscordCBLInfoExtension sends CBL alerts to Discord when a harmful player joins
type DiscordCBLInfoExtension struct {
	extension_manager.ExtensionBase
	discord *discord.DiscordConnector
}

// Define implements ExtensionRegistrar
func Define() extension_manager.ExtensionDefinition {
	return extension_manager.ExtensionDefinition{
		ID:                     "discord_cbl_info",
		Name:                   "Discord CBL Info",
		Description:            "Will alert admins in a Discord channel when a harmful player joins based on Community Ban List data.",
		Version:                "1.0.0",
		Author:                 "Squad Aegis",
		AllowMultipleInstances: true,

		Dependencies: extension_manager.ExtensionDependencies{
			Required: []extension_manager.DependencyType{
				extension_manager.DependencyServer,
				extension_manager.DependencyConnectors,
				extension_manager.DependencyRconManager,
			},
		},

		// Specify required connector types
		RequiredConnectors: []string{"discord"},

		ConfigSchema: plug_config_schema.ConfigSchema{
			Fields: []plug_config_schema.ConfigField{
				{
					Name:        "channel_id",
					Description: "The ID of the channel to send CBL alerts to.",
					Required:    true,
					Type:        plug_config_schema.FieldTypeString,
				},
				{
					Name:        "threshold",
					Description: "Players with this or more reputation points will trigger an alert. Default is 6.",
					Required:    false,
					Type:        plug_config_schema.FieldTypeInt,
					Default:     6,
				},
				{
					Name:        "color",
					Description: "The color of the embed.",
					Required:    false,
					Type:        plug_config_schema.FieldTypeInt,
					Default:     16761867, // Orange
				},
			},
		},

		EventHandlers: []extension_manager.EventHandler{
			{
				Source:      extension_manager.EventHandlerSourceRCON,
				Name:        "PLAYER_CONNECTED",
				Description: "Checks a player's CBL status when they connect",
				Handler: func(e extension_manager.Extension, data interface{}) error {
					return e.(*DiscordCBLInfoExtension).handlePlayerConnected(data)
				},
			},
		},
		CreateInstance: func() extension_manager.Extension {
			return &DiscordCBLInfoExtension{}
		},
	}
}

// Initialize initializes the extension with its configuration and dependencies
func (e *DiscordCBLInfoExtension) Initialize(config map[string]interface{}, deps *extension_manager.Dependencies) error {
	// Set the base extension properties
	e.Definition = Define()
	e.Config = config
	e.Deps = deps

	// Get discord connector from dependencies
	if e.Deps.Connectors == nil {
		return fmt.Errorf("connectors dependency not provided")
	}

	discordConnector, ok := e.Deps.Connectors["discord"]
	if !ok {
		return fmt.Errorf("discord connector not found")
	}

	// Type assertion
	e.discord, ok = discordConnector.(*discord.DiscordConnector)
	if !ok {
		return fmt.Errorf("invalid connector type for Discord")
	}

	// Validate config
	if err := e.Definition.ConfigSchema.Validate(config); err != nil {
		return err
	}

	// Fill defaults
	e.Definition.ConfigSchema.FillDefaults(config)

	return nil
}

// Shutdown gracefully shuts down the extension
func (e *DiscordCBLInfoExtension) Shutdown() error {
	// Nothing to clean up for this extension
	return nil
}
