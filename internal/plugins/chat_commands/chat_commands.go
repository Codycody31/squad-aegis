package chat_commands

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"go.codycody31.dev/squad-aegis/internal/event_manager"
	"go.codycody31.dev/squad-aegis/internal/plugin_manager"
	"go.codycody31.dev/squad-aegis/internal/shared/plug_config_schema"
)

// CommandConfig represents a single chat command configuration
type CommandConfig struct {
	Command     string   `json:"command"`
	Type        string   `json:"type"`        // "warn" or "broadcast"
	Response    string   `json:"response"`
	IgnoreChats []string `json:"ignoreChats"`
}

// ChatCommandsPlugin handles configurable chat commands that broadcast or warn players
type ChatCommandsPlugin struct {
	// Plugin configuration
	config map[string]interface{}
	apis   *plugin_manager.PluginAPIs

	// State management
	mu     sync.Mutex
	status plugin_manager.PluginStatus
	ctx    context.Context
	cancel context.CancelFunc

	// Parsed commands for quick lookup
	commands map[string]*CommandConfig
}

// Define returns the plugin definition
func Define() plugin_manager.PluginDefinition {
	return plugin_manager.PluginDefinition{
		ID:                     "chat_commands",
		Name:                   "Chat Commands",
		Description:            "The Chat Commands plugin can be configured to make chat commands that broadcast or warn the caller with preset messages.",
		Version:                "1.0.0",
		Author:                 "Squad Aegis",
		AllowMultipleInstances: false,
		RequiredConnectors:     []string{},
		LongRunning:            false,

		ConfigSchema: plug_config_schema.ConfigSchema{
			Fields: []plug_config_schema.ConfigField{
				{
					Name:        "commands",
					Description: "An array of command configurations. Each command should have: command (string), type ('warn' or 'broadcast'), response (string), and ignoreChats (array of chat types to ignore).",
					Required:    false,
					Type:        plug_config_schema.FieldTypeArray,
					Default: []interface{}{
						map[string]interface{}{
							"command":     "squadaegis",
							"type":        "warn",
							"response":    "This server is powered by Squad Aegis.",
							"ignoreChats": []interface{}{},
						},
					},
				},
				{
					Name:        "enabled",
					Description: "Whether the plugin is enabled.",
					Required:    false,
					Type:        plug_config_schema.FieldTypeBool,
					Default:     true,
				},
			},
		},

		Events: []event_manager.EventType{
			event_manager.EventTypeRconChatMessage,
		},

		CreateInstance: func() plugin_manager.Plugin {
			return &ChatCommandsPlugin{}
		},
	}
}

// GetDefinition returns the plugin definition
func (p *ChatCommandsPlugin) GetDefinition() plugin_manager.PluginDefinition {
	return Define()
}

// Initialize initializes the plugin with its configuration and dependencies
func (p *ChatCommandsPlugin) Initialize(config map[string]interface{}, apis *plugin_manager.PluginAPIs) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.config = config
	p.apis = apis
	p.status = plugin_manager.PluginStatusStopped
	p.commands = make(map[string]*CommandConfig)

	// Validate config
	definition := p.GetDefinition()
	if err := definition.ConfigSchema.Validate(config); err != nil {
		return fmt.Errorf("invalid config: %w", err)
	}

	// Fill defaults
	definition.ConfigSchema.FillDefaults(config)

	// Parse commands configuration
	if err := p.parseCommands(); err != nil {
		return fmt.Errorf("failed to parse commands: %w", err)
	}

	p.status = plugin_manager.PluginStatusStopped

	return nil
}

// Start begins plugin execution (for long-running plugins)
func (p *ChatCommandsPlugin) Start(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.status == plugin_manager.PluginStatusRunning {
		return nil // Already running
	}

	// Check if plugin is enabled
	if !p.getBoolConfig("enabled") {
		p.apis.LogAPI.Info("Chat Commands plugin is disabled", nil)
		return nil
	}

	p.ctx, p.cancel = context.WithCancel(ctx)
	p.status = plugin_manager.PluginStatusRunning

	p.apis.LogAPI.Info("Chat Commands plugin started", map[string]interface{}{
		"command_count": len(p.commands),
	})

	return nil
}

// Stop gracefully stops the plugin
func (p *ChatCommandsPlugin) Stop() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.status == plugin_manager.PluginStatusStopped {
		return nil // Already stopped
	}

	p.status = plugin_manager.PluginStatusStopping

	if p.cancel != nil {
		p.cancel()
	}

	p.status = plugin_manager.PluginStatusStopped

	p.apis.LogAPI.Info("Chat Commands plugin stopped", nil)

	return nil
}

// HandleEvent processes an event if the plugin is subscribed to it
func (p *ChatCommandsPlugin) HandleEvent(event *plugin_manager.PluginEvent) error {
	if event.Type != "RCON_CHAT_MESSAGE" {
		return nil // Not interested in this event
	}

	return p.handleChatMessage(event)
}

// GetStatus returns the current plugin status
func (p *ChatCommandsPlugin) GetStatus() plugin_manager.PluginStatus {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.status
}

// GetConfig returns the current plugin configuration
func (p *ChatCommandsPlugin) GetConfig() map[string]interface{} {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.config
}

// UpdateConfig updates the plugin configuration
func (p *ChatCommandsPlugin) UpdateConfig(config map[string]interface{}) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Validate new config
	definition := p.GetDefinition()
	if err := definition.ConfigSchema.Validate(config); err != nil {
		return fmt.Errorf("invalid config: %w", err)
	}

	// Fill defaults
	definition.ConfigSchema.FillDefaults(config)

	p.config = config

	// Re-parse commands
	if err := p.parseCommands(); err != nil {
		return fmt.Errorf("failed to parse commands: %w", err)
	}

	p.apis.LogAPI.Info("Chat Commands plugin configuration updated", map[string]interface{}{
		"command_count": len(p.commands),
	})

	return nil
}

// parseCommands parses the commands configuration into a lookup map
func (p *ChatCommandsPlugin) parseCommands() error {
	p.commands = make(map[string]*CommandConfig)

	commandsConfig, ok := p.config["commands"].([]interface{})
	if !ok {
		return fmt.Errorf("commands config is not an array")
	}

	for i, cmdInterface := range commandsConfig {
		cmdMap, ok := cmdInterface.(map[string]interface{})
		if !ok {
			return fmt.Errorf("command %d is not an object", i)
		}

		command := &CommandConfig{}

		// Parse command name
		if cmdName, ok := cmdMap["command"].(string); ok {
			command.Command = strings.ToLower(strings.TrimSpace(cmdName))
		} else {
			return fmt.Errorf("command %d missing 'command' field", i)
		}

		// Parse type
		if cmdType, ok := cmdMap["type"].(string); ok {
			cmdType = strings.ToLower(strings.TrimSpace(cmdType))
			if cmdType != "warn" && cmdType != "broadcast" {
				return fmt.Errorf("command %d has invalid type '%s', must be 'warn' or 'broadcast'", i, cmdType)
			}
			command.Type = cmdType
		} else {
			return fmt.Errorf("command %d missing 'type' field", i)
		}

		// Parse response
		if response, ok := cmdMap["response"].(string); ok {
			command.Response = response
		} else {
			return fmt.Errorf("command %d missing 'response' field", i)
		}

		// Parse ignoreChats
		if ignoreChatsInterface, ok := cmdMap["ignoreChats"].([]interface{}); ok {
			command.IgnoreChats = make([]string, len(ignoreChatsInterface))
			for j, chatInterface := range ignoreChatsInterface {
				if chat, ok := chatInterface.(string); ok {
					command.IgnoreChats[j] = strings.ToUpper(strings.TrimSpace(chat))
				} else {
					return fmt.Errorf("command %d ignoreChats[%d] is not a string", i, j)
				}
			}
		}

		// Check for duplicate commands
		if _, exists := p.commands[command.Command]; exists {
			return fmt.Errorf("duplicate command '%s'", command.Command)
		}

		p.commands[command.Command] = command
	}

	return nil
}

// handleChatMessage processes chat message events to detect commands
func (p *ChatCommandsPlugin) handleChatMessage(rawEvent *plugin_manager.PluginEvent) error {
	if !p.getBoolConfig("enabled") {
		return nil // Plugin is disabled
	}

	event, ok := rawEvent.Data.(*event_manager.RconChatMessageData)
	if !ok {
		return fmt.Errorf("invalid event data type")
	}

	// Check if this is a command (starts with !)
	message := strings.TrimSpace(event.Message)
	if !strings.HasPrefix(message, "!") {
		return nil // Not a command
	}

	// Extract command name (remove ! and get first word)
	commandText := strings.ToLower(strings.TrimPrefix(message, "!"))
	commandParts := strings.Fields(commandText)
	if len(commandParts) == 0 {
		return nil // Empty command
	}

	commandName := commandParts[0]

	// Look up the command
	p.mu.Lock()
	command, exists := p.commands[commandName]
	p.mu.Unlock()

	if !exists {
		return nil // Command not configured
	}

	// Check if we should ignore this chat type
	chatType := strings.ToUpper(event.ChatType)
	for _, ignoredChat := range command.IgnoreChats {
		if chatType == ignoredChat {
			p.apis.LogAPI.Debug("Ignoring command in chat type", map[string]interface{}{
				"command":   commandName,
				"chat_type": chatType,
				"player":    event.PlayerName,
			})
			return nil
		}
	}

	// Execute the command
	return p.executeCommand(command, event)
}

// executeCommand executes a chat command
func (p *ChatCommandsPlugin) executeCommand(command *CommandConfig, event *event_manager.RconChatMessageData) error {
	switch command.Type {
	case "broadcast":
		if err := p.apis.RconAPI.Broadcast(command.Response); err != nil {
			return fmt.Errorf("failed to broadcast message: %w", err)
		}

		p.apis.LogAPI.Info("Executed broadcast command", map[string]interface{}{
			"command":  command.Command,
			"player":   event.PlayerName,
			"steam_id": event.SteamID,
			"response": command.Response,
		})

	case "warn":
		if err := p.apis.RconAPI.SendWarningToPlayer(event.EosID, command.Response); err != nil {
			return fmt.Errorf("failed to warn player: %w", err)
		}

		p.apis.LogAPI.Info("Executed warn command", map[string]interface{}{
			"command":  command.Command,
			"player":   event.PlayerName,
			"steam_id": event.SteamID,
			"eos_id":   event.EosID,
			"response": command.Response,
		})

	default:
		return fmt.Errorf("unknown command type: %s", command.Type)
	}

	return nil
}

// Helper methods for config access

func (p *ChatCommandsPlugin) getBoolConfig(key string) bool {
	if value, ok := p.config[key].(bool); ok {
		return value
	}
	return false
}
