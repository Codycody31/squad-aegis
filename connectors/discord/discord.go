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
	connector_manager.ConnectorBase
	token   string
	session *discordgo.Session
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

// Initialize initializes the bot with config data
func (d *DiscordConnector) Initialize(config map[string]interface{}) error {
	// Call the base Initialize to set up common fields
	if err := d.ConnectorBase.Initialize(config); err != nil {
		return err
	}

	token, ok := config["token"].(string)
	if !ok || token == "" {
		return ErrInvalidToken
	}

	d.token = token

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

	// Call the base Shutdown method
	return d.ConnectorBase.Shutdown()
}

// GetSession returns the Discord session
func (d *DiscordConnector) GetSession() *discordgo.Session {
	return d.session
}

// Define returns the connector definition
func Define() connector_manager.ConnectorDefinition {
	return connector_manager.ConnectorDefinition{
		ID:          "discord",
		Name:        "Discord",
		Description: "Discord bot connector for Squad Aegis",
		Version:     "1.0.0",
		Author:      "Squad Aegis Team",
		ConfigSchema: plug_config_schema.ConfigSchema{
			Fields: []plug_config_schema.ConfigField{
				{
					Name:        "token",
					Description: "Bot token from Discord Developer Portal",
					Required:    true,
					Type:        plug_config_schema.FieldTypeString,
				},
			},
		},
		CreateInstance: CreateInstance,
	}
}

// CreateInstance creates a new Discord connector instance
func CreateInstance(id uuid.UUID, config map[string]interface{}) (connector_manager.Connector, error) {
	// Create the connector instance
	connector := &DiscordConnector{
		ConnectorBase: connector_manager.ConnectorBase{
			ID:     id,
			Config: config,
		},
	}

	// Set the definition
	connector.Definition = Define()

	// Extract token from config (optional here as Initialize will validate)
	if token, ok := config["token"].(string); ok && token != "" {
		connector.token = token
	}

	// Initialize the Discord session
	if err := connector.Initialize(config); err != nil {
		return nil, err
	}

	return connector, nil
}
