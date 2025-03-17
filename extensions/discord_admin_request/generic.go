package discord_admin_request

import (
	"fmt"
	"sync"
	"time"

	"go.codycody31.dev/squad-aegis/connectors/discord"
	"go.codycody31.dev/squad-aegis/internal/extension_manager"
	"go.codycody31.dev/squad-aegis/shared/plug_config_schema"
)

// DiscordAdminRequestExtension sends admin requests to Discord
type DiscordAdminRequestExtension struct {
	extension_manager.ExtensionBase
	discord      *discord.DiscordConnector
	lastPingTime time.Time
	mu           sync.Mutex
}

// Define implements ExtensionRegistrar
func Define() extension_manager.ExtensionDefinition {
	return extension_manager.ExtensionDefinition{
		ID:          "discord_admin_request",
		Name:        "Discord Admin Requests",
		Description: "Will ping admins in a Discord channel when a player requests an admin via the !admin command in in-game chat.",
		Version:     "1.0.0",
		Author:      "Squad Aegis",

		Dependencies: extension_manager.ExtensionDependencies{
			Required: []extension_manager.DependencyType{
				extension_manager.DependencyServer,
				extension_manager.DependencyConnectors,
				extension_manager.DependencyRconManager,
				extension_manager.DependencyDatabase,
			},
		},

		// Specify required connector types
		RequiredConnectors: []string{"discord"},

		ConfigSchema: plug_config_schema.ConfigSchema{
			Fields: []plug_config_schema.ConfigField{
				{
					Name:        "channel_id",
					Description: "The ID of the channel to log admin requests to.",
					Required:    true,
					Type:        plug_config_schema.FieldTypeString,
				},
				{
					Name:        "ignore_chats",
					Description: "A list of chat names to ignore.",
					Required:    false,
					Type:        plug_config_schema.FieldTypeArrayString,
					Default:     []interface{}{"ChatSquad"},
				},
				{
					Name:        "ping_groups",
					Description: "A list of Discord role IDs to ping.",
					Required:    false,
					Type:        plug_config_schema.FieldTypeArrayString,
					Default:     []interface{}{},
				},
				{
					Name:        "ping_here",
					Description: "Ping @here. Great if Admin Requests are posted to a Squad Admin ONLY channel, allows pinging only Online Admins",
					Required:    false,
					Type:        plug_config_schema.FieldTypeBool,
					Default:     false,
				},
				{
					Name:        "ping_delay",
					Description: "Cooldown for pings in milliseconds.",
					Required:    false,
					Type:        plug_config_schema.FieldTypeInt,
					Default:     60000,
				},
				{
					Name:        "color",
					Description: "Color of the embed.",
					Required:    false,
					Type:        plug_config_schema.FieldTypeInt,
					Default:     16761867,
				},
				{
					Name:        "warn_in_game_admins",
					Description: "Should in-game admins be warned after a players uses the command and should we tell how much admins are active in-game right now.",
					Required:    false,
					Type:        plug_config_schema.FieldTypeBool,
					Default:     false,
				},
				{
					Name:        "show_in_game_admins",
					Description: "Should players know how much in-game admins there are active/online?",
					Required:    false,
					Type:        plug_config_schema.FieldTypeBool,
					Default:     true,
				},
			},
		},

		EventHandlers: []extension_manager.EventHandler{
			{
				Source:      extension_manager.EventHandlerSourceRCON,
				Name:        "CHAT_MESSAGE",
				Description: "Handles admin requests from in-game chat",
				Handler: func(e extension_manager.Extension, data interface{}) error {
					return e.(*DiscordAdminRequestExtension).handleChatMessage(data)
				},
			},
		},

		CreateInstance: func() extension_manager.Extension {
			return &DiscordAdminRequestExtension{}
		},
	}
}

// Initialize initializes the extension with its configuration and dependencies
func (e *DiscordAdminRequestExtension) Initialize(config map[string]interface{}, deps *extension_manager.Dependencies) error {
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
func (e *DiscordAdminRequestExtension) Shutdown() error {
	// Nothing to clean up for this extension
	return nil
}
