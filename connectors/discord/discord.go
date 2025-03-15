package discord

import (
	"fmt"
	"sync"

	"github.com/bwmarrin/discordgo"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"go.codycody31.dev/squad-aegis/internal/connector_manager"
	plug_config_schema "go.codycody31.dev/squad-aegis/shared/plug_config_schema"
)

// Singleton instance
var once sync.Once
var discordInstance *DiscordConnector

// DiscordConnector manages the bot connection
type DiscordConnector struct {
	id      uuid.UUID
	token   string
	session *discordgo.Session
	config  map[string]interface{}
}

// ConfigSchema defines required fields for Discord
func ConfigSchema() plug_config_schema.ConfigSchema {
	return plug_config_schema.ConfigSchema{
		Fields: []plug_config_schema.ConfigField{
			{
				Name:        "token",
				Description: "Bot token from Discord Developer Portal",
				Required:    true,
				Type:        plug_config_schema.FieldTypeString,
			},
		},
	}
}

// DiscordConnectorFactory creates Discord connector instances
type DiscordConnectorFactory struct{}

// Create creates a new Discord connector instance
func (f *DiscordConnectorFactory) Create(id uuid.UUID, config map[string]interface{}) (connector_manager.ConnectorInstance, error) {
	token, ok := config["token"].(string)
	if !ok || token == "" {
		return nil, fmt.Errorf("discord connector requires a valid token")
	}

	connector := &DiscordConnector{
		id:     id,
		token:  token,
		config: config,
	}

	// Initialize the Discord session
	if err := connector.Initialize(config); err != nil {
		return nil, err
	}

	return connector, nil
}

// GetType returns the type of connector this factory creates
func (f *DiscordConnectorFactory) GetType() string {
	return "discord"
}

// ConfigSchema returns the configuration schema for this connector type
func (f *DiscordConnectorFactory) ConfigSchema() map[string]interface{} {
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

// GetID returns the unique identifier for this connector instance
func (d *DiscordConnector) GetID() uuid.UUID {
	return d.id
}

// GetType returns the type of connector
func (d *DiscordConnector) GetType() string {
	return "discord"
}

// GetConfig returns the configuration for this connector
func (d *DiscordConnector) GetConfig() map[string]interface{} {
	return d.config
}

// Initialize initializes the bot with config data
func (d *DiscordConnector) Initialize(config map[string]interface{}) error {
	token, ok := config["token"].(string)
	if !ok || token == "" {
		return fmt.Errorf("discord connector requires a valid token")
	}

	d.token = token
	d.config = config

	session, err := discordgo.New("Bot " + token)
	if err != nil {
		return fmt.Errorf("failed to initialize Discord bot: %v", err)
	}

	d.session = session
	err = d.session.Open()
	if err != nil {
		return fmt.Errorf("failed to start Discord session: %v", err)
	}

	log.Info().Msg("Discord bot is running")
	return nil
}

// Shutdown gracefully shuts down the connector
func (d *DiscordConnector) Shutdown() error {
	if d.session != nil {
		err := d.session.Close()
		if err != nil {
			return fmt.Errorf("failed to close Discord session: %v", err)
		}
	}
	return nil
}

// GetSession returns the Discord session
func (d *DiscordConnector) GetSession() *discordgo.Session {
	return d.session
}

// Factory instance for registration
var Factory = &DiscordConnectorFactory{}
