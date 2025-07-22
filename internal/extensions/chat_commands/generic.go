package chat_commands

import (
	"fmt"
	"slices"

	"github.com/SquadGO/squad-rcon-go/v2/rconTypes"
	"github.com/rs/zerolog/log"
	"go.codycody31.dev/squad-aegis/internal/connectors/discord"
	"go.codycody31.dev/squad-aegis/internal/extension_manager"
	"go.codycody31.dev/squad-aegis/internal/shared/plug_config_schema"
	"go.codycody31.dev/squad-aegis/internal/shared/utils"
	squadRcon "go.codycody31.dev/squad-aegis/internal/squad-rcon"
)

// Command represents a chat command configuration
type Command struct {
	Command     string
	Type        string
	Response    string
	IgnoreChats []string
}

// ChatCommandsExtension handles chat commands
type ChatCommandsExtension struct {
	extension_manager.ExtensionBase
	discord *discord.DiscordConnector
}

// Define implements ExtensionRegistrar
func Define() extension_manager.ExtensionDefinition {
	return extension_manager.ExtensionDefinition{
		ID:                     "chat_commands",
		Name:                   "Chat Commands",
		Description:            "Make chat commands that broadcast or warn the caller with present messages.",
		Version:                "1.0.0",
		Author:                 "Squad Aegis",
		AllowMultipleInstances: false,

		Dependencies: extension_manager.ExtensionDependencies{
			Required: []extension_manager.DependencyType{
				extension_manager.DependencyRconManager,
				extension_manager.DependencyServer,
			},
		},

		ConfigSchema: plug_config_schema.ConfigSchema{
			Fields: []plug_config_schema.ConfigField{
				{
					Name:        "commands",
					Type:        plug_config_schema.FieldTypeArrayObject,
					Description: "List of command configurations",
					Nested: []plug_config_schema.ConfigField{
						{
							Name:     "command",
							Type:     plug_config_schema.FieldTypeString,
							Required: true,
						},
						{
							Name:     "type",
							Type:     plug_config_schema.FieldTypeString,
							Required: true,
							Options:  []interface{}{"broadcast", "warn"},
						},
						{
							Name:     "response",
							Type:     plug_config_schema.FieldTypeString,
							Required: true,
						},
						{
							Name:     "ignoreChats",
							Type:     plug_config_schema.FieldTypeArrayString,
							Default:  []string{},
							Required: false,
						},
					},
				},
			},
		},

		EventHandlers: []extension_manager.EventHandler{
			{
				Source:      extension_manager.EventHandlerSourceRCON,
				Name:        "CHAT_COMMAND",
				Description: "Handles chat commands from in-game chat",
				Handler: func(e extension_manager.Extension, data interface{}) error {
					return e.(*ChatCommandsExtension).handleChatMessage(data)
				},
			},
		},

		CreateInstance: func() extension_manager.Extension {
			return &ChatCommandsExtension{}
		},
	}
}

// Initialize initializes the extension with its configuration and dependencies
func (e *ChatCommandsExtension) Initialize(config map[string]interface{}, deps *extension_manager.Dependencies) error {
	// Set the base extension properties
	e.Definition = Define()
	e.Config = config
	e.Deps = deps

	// Validate config
	if err := e.Definition.ConfigSchema.Validate(config); err != nil {
		return err
	}

	// Fill defaults
	e.Definition.ConfigSchema.FillDefaults(config)

	return nil
}

// handleChatMessage processes chat command messages
func (e *ChatCommandsExtension) handleChatMessage(data interface{}) error {
	rconMessage, ok := data.(rconTypes.Message)
	if !ok {
		return fmt.Errorf("invalid data type for chat message")
	}

	message, err := utils.ParseRconCommandMessage(rconMessage)
	if err != nil {
		return fmt.Errorf("failed to parse RCON command message: %w", err)
	}

	// Process each configured command
	commands, ok := e.Config["commands"].([]interface{})
	if !ok {
		return fmt.Errorf("commands configuration not found or invalid")
	}

	for _, cmdConfig := range commands {
		cmdMap, ok := cmdConfig.(map[string]interface{})
		if !ok {
			continue
		}

		cmdTrigger, ok := cmdMap["command"].(string)
		// TODO: Handle this
		if !ok || cmdTrigger != fmt.Sprintf("!%s", message.Message) {
			continue
		}

		// Check if this chat should be ignored
		ignoreChats := plug_config_schema.GetArrayStringValue(cmdMap, "ignoreChats")
		if slices.Contains(ignoreChats, message.ChatType) {
			log.Debug().
				Str("extension", "chat_commands").
				Str("command", message.Command).
				Str("chatType", message.ChatType).
				Msg("Command ignored due to chat type")
			return nil
		}

		// Execute the command based on type
		cmdType, ok := cmdMap["type"].(string)
		if !ok {
			continue
		}

		response, ok := cmdMap["response"].(string)
		if !ok {
			continue
		}

		// Create a Squad RCON instance
		r := squadRcon.NewSquadRcon(e.Deps.RconManager, e.Deps.Server.Id)

		if cmdType == "broadcast" {
			// Use Squad RCON to broadcast
			_, err := r.ExecuteRaw(fmt.Sprintf("AdminBroadcast %s", response))
			if err != nil {
				log.Error().
					Str("extension", "chat_commands").
					Str("command", message.Command).
					Str("action", "broadcast").
					Err(err).
					Msg("Failed to broadcast message")
				return err
			}
			log.Debug().
				Str("extension", "chat_commands").
				Str("command", message.Command).
				Str("action", "broadcast").
				Str("message", response).
				Msg("Broadcast message sent")
		} else if cmdType == "warn" {
			// Use Squad RCON to warn the player
			_, err := r.ExecuteRaw(fmt.Sprintf("AdminWarn %s %s", message.EosID, response))
			if err != nil {
				log.Error().
					Str("extension", "chat_commands").
					Str("command", message.Command).
					Str("action", "warn").
					Err(err).
					Msg("Failed to warn player")
				return err
			}
			log.Debug().
				Str("extension", "chat_commands").
				Str("command", message.Command).
				Str("action", "warn").
				Str("player", message.PlayerName).
				Str("message", response).
				Msg("Warning sent to player")
		}
	}

	return nil
}

// Shutdown gracefully shuts down the extension
func (e *ChatCommandsExtension) Shutdown() error {
	// Nothing to clean up for this extension
	return nil
}
