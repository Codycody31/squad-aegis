package discord_chat

import (
	"fmt"
	"slices"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"go.codycody31.dev/squad-aegis/connectors/discord"
	"go.codycody31.dev/squad-aegis/internal/connector_manager"
	"go.codycody31.dev/squad-aegis/internal/extension_manager"
	"go.codycody31.dev/squad-aegis/internal/models"
	"go.codycody31.dev/squad-aegis/internal/rcon"
	"go.codycody31.dev/squad-aegis/internal/rcon_manager"
	"go.codycody31.dev/squad-aegis/shared/plug_config_schema"
)

// ConfigSchema defines the configuration schema for this extension
func ConfigSchema() plug_config_schema.ConfigSchema {
	return plug_config_schema.ConfigSchema{
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
				Description: "A list of chats to ignore. (Default: none)",
				Required:    false,
				Type:        plug_config_schema.FieldTypeArray,
				Default:     []string{},
			},
		},
	}
}

// DiscordChatExtension sends chat messages to Discord
type DiscordChatExtension struct {
	id            uuid.UUID
	server        *models.Server
	config        map[string]interface{}
	discord       *discord.DiscordConnector
	rconManager   *rcon_manager.RconManager
	eventHandlers map[string]extension_manager.ExtensionHandler
	lastPingTime  time.Time
	mu            sync.Mutex
}

// DiscordChatFactory creates instances of the extension
type DiscordChatFactory struct{}

// Create creates a new instance of the extension
func (f *DiscordChatFactory) Create() extension_manager.Extension {
	ext := &DiscordChatExtension{
		id:            uuid.New(),
		eventHandlers: make(map[string]extension_manager.ExtensionHandler),
	}

	// Register event handlers
	ext.eventHandlers["CHAT_MESSAGE"] = ext.handleChatMessage

	return ext
}

// GetConfigSchema returns the configuration schema for this extension
func (f *DiscordChatFactory) GetConfigSchema() map[string]interface{} {
	schema := ConfigSchema()
	result := make(map[string]interface{})

	for _, field := range schema.Fields {
		fieldInfo := map[string]interface{}{
			"description": field.Description,
			"required":    field.Required,
			"type":        string(field.Type),
		}

		if field.Default != nil {
			fieldInfo["default"] = field.Default
		}

		result[field.Name] = fieldInfo
	}

	return result
}

// GetID returns the unique identifier for this extension
func (e *DiscordChatExtension) GetID() uuid.UUID {
	return e.id
}

// GetName returns the name of the extension
func (e *DiscordChatExtension) GetName() string {
	return "Discord Chat"
}

// GetDescription returns the description of the extension
func (e *DiscordChatExtension) GetDescription() string {
	return "Will log chat messages to a Discord channel."
}

// GetVersion returns the version of the extension
func (e *DiscordChatExtension) GetVersion() string {
	return "1.0.0"
}

// GetAuthor returns the author of the extension
func (e *DiscordChatExtension) GetAuthor() string {
	return "Squad Aegis"
}

// GetEventHandlers returns a map of event types to handlers
func (e *DiscordChatExtension) GetEventHandlers() map[string]extension_manager.ExtensionHandler {
	return e.eventHandlers
}

// GetRequiredConnectors returns a list of connector types required by this extension
func (e *DiscordChatExtension) GetRequiredConnectors() []string {
	return []string{"discord"}
}

// Initialize initializes the extension with its configuration and connectors
func (e *DiscordChatExtension) Initialize(server *models.Server, config map[string]interface{}, connectors map[string]connector_manager.ConnectorInstance, rconManager *rcon_manager.RconManager) error {
	e.server = server
	e.config = config
	e.rconManager = rconManager

	// Get discord connector
	discordConnector, ok := connectors["discord"]
	if !ok {
		return fmt.Errorf("discord connector not found")
	}

	// Type assertion
	e.discord, ok = discordConnector.(*discord.DiscordConnector)
	if !ok {
		return fmt.Errorf("invalid connector type for Discord")
	}

	// Validate config
	schema := ConfigSchema()
	if err := schema.Validate(config); err != nil {
		return err
	}

	log.Info().
		Str("extension", e.GetName()).
		Str("serverID", server.Id.String()).
		Msg("Discord Admin Request extension initialized")

	return nil
}

// Shutdown gracefully shuts down the extension
func (e *DiscordChatExtension) Shutdown() error {
	// Nothing to clean up for this extension
	return nil
}

// handleChatMessage handles chat messages and looks for admin requests
func (e *DiscordChatExtension) handleChatMessage(serverID uuid.UUID, eventType string, data interface{}) error {
	// Only process events for our server
	if serverID != e.server.Id {
		return nil
	}

	// Type assertion
	message, ok := data.(rcon.Message)
	if !ok {
		return fmt.Errorf("invalid data type for chat message")
	}

	if e.config["ignore_chats"] != nil {
		ignoreChats := e.config["ignore_chats"].([]string)
		if slices.Contains(ignoreChats, message.Message) {
			return nil
		}
	}

	// Send message to Discord
	return e.sendDiscordMessage(message)
}

// sendDiscordMessage sends the chat message to Discord
func (e *DiscordChatExtension) sendDiscordMessage(message rcon.Message) error {
	// Get channel ID
	channelID, ok := e.config["channel_id"].(string)
	if !ok || channelID == "" {
		return fmt.Errorf("channel_id not configured properly")
	}

	// Set embed color
	color := 16761867 // default: orange
	if colorVal, ok := e.config["color"].(float64); ok {
		color = int(colorVal)
	}

	// Create embed
	embed := &discordgo.MessageEmbed{
		Title: "Chat Message",
		Color: color,
		Fields: []*discordgo.MessageEmbedField{
			{
				Name:   "Player",
				Value:  message.PlayerName,
				Inline: false,
			},
			{
				Name:   "Message",
				Value:  message.Message,
				Inline: false,
			},
			{
				Name:   "Server",
				Value:  e.server.Name,
				Inline: false,
			},
		},
		Timestamp: time.Now().Format(time.RFC3339),
	}

	// Send message to Discord
	session := e.discord.GetSession()
	_, err := session.ChannelMessageSendComplex(channelID, &discordgo.MessageSend{
		Embeds: []*discordgo.MessageEmbed{embed},
	})

	return err
}

// Factory instance for registration
var Factory = &DiscordChatFactory{}
