package discord_admin_broadcast

import (
	"fmt"

	"go.codycody31.dev/squad-aegis/connectors/discord"
	"go.codycody31.dev/squad-aegis/internal/extension_manager"
	"go.codycody31.dev/squad-aegis/shared/plug_config_schema"
)

// DiscordAdminBroadcastExtension sends chat messages to Discord
type DiscordAdminBroadcastExtension struct {
	extension_manager.ExtensionBase
	discord *discord.DiscordConnector
}

// Define implements ExtensionRegistrar
func Define() extension_manager.ExtensionDefinition {
	return extension_manager.ExtensionDefinition{
		ID:                     "discord_admin_broadcast",
		Name:                   "Discord Admin Broadcast",
		Description:            "will send a copy of admin broadcasts made in game to a Discord channel.",
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
		RequiredConnectors: []string{"discord", "logwatcher"},

		ConfigSchema: plug_config_schema.ConfigSchema{
			Fields: []plug_config_schema.ConfigField{
				{
					Name:        "channel_id",
					Description: "The ID of the channel to log admin cam logs to.",
					Required:    true,
					Type:        plug_config_schema.FieldTypeString,
				},
				{
					Name:        "color",
					Description: "The color of the embed.",
					Required:    false,
					Type:        plug_config_schema.FieldTypeInt,
					Default:     16761867,
				},
			},
		},

		EventHandlers: []extension_manager.EventHandler{
			{
				Source:      extension_manager.EventHandlerSourceLOGS,
				Name:        "ADMIN_BROADCAST",
				Description: "Logs admin broadcasts to Discord.",
				Handler: func(e extension_manager.Extension, data interface{}) error {
					return e.(*DiscordAdminBroadcastExtension).handleAdminBroadcast(data)
				},
			},
		},
		CreateInstance: func() extension_manager.Extension {
			return &DiscordAdminBroadcastExtension{}
		},
	}
}

// Initialize initializes the extension with its configuration and dependencies
func (e *DiscordAdminBroadcastExtension) Initialize(config map[string]interface{}, deps *extension_manager.Dependencies) error {
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
func (e *DiscordAdminBroadcastExtension) Shutdown() error {
	// Nothing to clean up for this extension
	return nil
}
