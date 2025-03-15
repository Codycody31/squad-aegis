package discord_admin_request

import (
	"fmt"
	"strings"
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
	squadRcon "go.codycody31.dev/squad-aegis/internal/squad-rcon"
	"go.codycody31.dev/squad-aegis/shared/plug_config_schema"
)

// ConfigSchema defines the configuration schema for this extension
func ConfigSchema() plug_config_schema.ConfigSchema {
	return plug_config_schema.ConfigSchema{
		Fields: []plug_config_schema.ConfigField{
			{
				Name:        "channel_id",
				Description: "The ID of the channel to log admin requests to.",
				Required:    true,
				Type:        plug_config_schema.FieldTypeString,
			},
			{
				Name:        "command",
				Description: "The command that calls an admin.",
				Required:    false,
				Type:        plug_config_schema.FieldTypeString,
				Default:     "admin",
			},
			{
				Name:        "ping_groups",
				Description: "A list of Discord role IDs to ping.",
				Required:    false,
				Type:        plug_config_schema.FieldTypeArray,
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
	}
}

// DiscordAdminRequestExtension sends admin requests to Discord
type DiscordAdminRequestExtension struct {
	id            uuid.UUID
	server        *models.Server
	config        map[string]interface{}
	discord       *discord.DiscordConnector
	rconManager   *rcon_manager.RconManager
	eventHandlers map[string]extension_manager.ExtensionHandler
	lastPingTime  time.Time
	mu            sync.Mutex
}

// DiscordAdminRequestFactory creates instances of the extension
type DiscordAdminRequestFactory struct{}

// Create creates a new instance of the extension
func (f *DiscordAdminRequestFactory) Create() extension_manager.Extension {
	ext := &DiscordAdminRequestExtension{
		id:            uuid.New(),
		eventHandlers: make(map[string]extension_manager.ExtensionHandler),
	}

	// Register event handlers
	ext.eventHandlers["CHAT_MESSAGE"] = ext.handleChatMessage

	return ext
}

// GetConfigSchema returns the configuration schema for this extension
func (f *DiscordAdminRequestFactory) GetConfigSchema() map[string]interface{} {
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
func (e *DiscordAdminRequestExtension) GetID() uuid.UUID {
	return e.id
}

// GetName returns the name of the extension
func (e *DiscordAdminRequestExtension) GetName() string {
	return "Discord Admin Requests"
}

// GetDescription returns the description of the extension
func (e *DiscordAdminRequestExtension) GetDescription() string {
	return "Will ping admins in a Discord channel when a player requests an admin via the !admin command in in-game chat."
}

// GetVersion returns the version of the extension
func (e *DiscordAdminRequestExtension) GetVersion() string {
	return "1.0.0"
}

// GetAuthor returns the author of the extension
func (e *DiscordAdminRequestExtension) GetAuthor() string {
	return "Squad Aegis"
}

// GetEventHandlers returns a map of event types to handlers
func (e *DiscordAdminRequestExtension) GetEventHandlers() map[string]extension_manager.ExtensionHandler {
	return e.eventHandlers
}

// GetRequiredConnectors returns a list of connector types required by this extension
func (e *DiscordAdminRequestExtension) GetRequiredConnectors() []string {
	return []string{"discord"}
}

// Initialize initializes the extension with its configuration and connectors
func (e *DiscordAdminRequestExtension) Initialize(server *models.Server, config map[string]interface{}, connectors map[string]connector_manager.ConnectorInstance, rconManager *rcon_manager.RconManager) error {
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
func (e *DiscordAdminRequestExtension) Shutdown() error {
	// Nothing to clean up for this extension
	return nil
}

// handleChatMessage handles chat messages and looks for admin requests
func (e *DiscordAdminRequestExtension) handleChatMessage(serverID uuid.UUID, eventType string, data interface{}) error {
	// Only process events for our server
	if serverID != e.server.Id {
		return nil
	}

	// Type assertion
	message, ok := data.(rcon.Message)
	if !ok {
		return fmt.Errorf("invalid data type for chat message")
	}

	// Get the command prefix from config
	command, ok := e.config["command"].(string)
	if !ok {
		command = "admin" // default to "admin" if not configured
	}

	// Check if this is an admin request
	commandPrefix := "!" + command
	if !strings.HasPrefix(message.Message, commandPrefix) {
		return nil
	}

	// Extract reason (everything after the command)
	reason := strings.TrimSpace(strings.TrimPrefix(message.Message, commandPrefix))

	// Check if we can ping (cooldown)
	e.mu.Lock()
	pingDelay := 60000 // default cooldown: 60 seconds
	if delay, ok := e.config["ping_delay"].(float64); ok {
		pingDelay = int(delay)
	}
	canPing := time.Since(e.lastPingTime).Milliseconds() > int64(pingDelay)
	if canPing {
		e.lastPingTime = time.Now()
	}
	e.mu.Unlock()

	r := squadRcon.NewSquadRcon(e.rconManager, e.server.Id)
	_, err := r.ExecuteRaw(fmt.Sprintf("AdminWarn %s An admin has been notified of your request.", message.SteamID))
	if err != nil {
		log.Error().
			Err(err).
			Str("serverID", e.server.Id.String()).
			Msg("Failed to notify player")
	}

	// Format the Discord message
	return e.sendDiscordMessage(message.PlayerName, message.SteamID, reason, canPing)
}

// sendDiscordMessage sends the admin request to Discord
func (e *DiscordAdminRequestExtension) sendDiscordMessage(playerName, steamID, reason string, canPing bool) error {
	channelID, ok := e.config["channel_id"].(string)
	if !ok || channelID == "" {
		return fmt.Errorf("channel_id not configured properly")
	}

	// Create the message content with pings if allowed
	content := ""
	if canPing {
		// Add @here if configured
		pingHere, _ := e.config["ping_here"].(bool)
		if pingHere {
			content += "@here "
		}

		// Add role pings if configured
		pingGroups, ok := e.config["ping_groups"].([]interface{})
		if ok && len(pingGroups) > 0 {
			for _, group := range pingGroups {
				if groupID, ok := group.(string); ok {
					content += fmt.Sprintf("<@&%s> ", groupID)
				}
			}
		}
	}

	// Set embed color
	color := 16761867 // default: orange
	if colorVal, ok := e.config["color"].(float64); ok {
		color = int(colorVal)
	}

	// Create embed
	embed := &discordgo.MessageEmbed{
		Title:       "Admin Request",
		Description: fmt.Sprintf("Player **%s** is requesting admin assistance", playerName),
		Color:       color,
		Fields: []*discordgo.MessageEmbedField{
			{
				Name:   "Steam ID",
				Value:  steamID,
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

	// Add reason if provided
	if reason != "" {
		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
			Name:   "Reason",
			Value:  reason,
			Inline: false,
		})
	}

	// Send message to Discord
	session := e.discord.GetSession()
	_, err := session.ChannelMessageSendComplex(channelID, &discordgo.MessageSend{
		Content: content,
		Embeds:  []*discordgo.MessageEmbed{embed},
	})

	return err
}

// Factory instance for registration
var Factory = &DiscordAdminRequestFactory{}
