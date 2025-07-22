package discord_chat

import (
	"fmt"

	"go.codycody31.dev/squad-aegis/internal/connectors/discord"
	"go.codycody31.dev/squad-aegis/internal/extension_manager"
	"go.codycody31.dev/squad-aegis/internal/shared/plug_config_schema"
)

// DiscordChatExtension sends chat messages to Discord
type DiscordChatExtension struct {
	extension_manager.ExtensionBase
	discord *discord.DiscordConnector
}

// Define implements ExtensionRegistrar
func Define() extension_manager.ExtensionDefinition {
	return extension_manager.ExtensionDefinition{
		ID:                     "discord_chat",
		Name:                   "Discord Chat",
		Description:            "Will log chat messages to a Discord channel.",
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
					Description: "The ID of the channel to log chat messages to.",
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
				{
					Name:        "ignore_chats",
					Description: "A list of chats to ignore. (Default: ChatSquad)",
					Required:    false,
					Type:        plug_config_schema.FieldTypeArrayString,
					Default:     []interface{}{"ChatSquad"},
				},
			},
		},

		EventHandlers: []extension_manager.EventHandler{
			{
				Source:      extension_manager.EventHandlerSourceRCON,
				Name:        "CHAT_MESSAGE",
				Description: "Logs chat messages to a configured Discord channel",
				Handler: func(e extension_manager.Extension, data interface{}) error {
					return e.(*DiscordChatExtension).handleChatMessage(data)
				},
			},
		},
		CreateInstance: func() extension_manager.Extension {
			return &DiscordChatExtension{}
		},
	}
}

// Initialize initializes the extension with its configuration and dependencies
func (e *DiscordChatExtension) Initialize(config map[string]interface{}, deps *extension_manager.Dependencies) error {
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
func (e *DiscordChatExtension) Shutdown() error {
	// Nothing to clean up for this extension
	return nil
}
