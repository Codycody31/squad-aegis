package discord

import (
	"fmt"
	"sync"

	"github.com/bwmarrin/discordgo"
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

	d.session.UpdateCustomStatus("Powered by Squad Aegis")

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
		Scope:       connector_manager.ConnectorScopeGlobal,
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
		CreateInstance: func() connector_manager.Connector {
			connector := &DiscordConnector{}
			connector.Definition = Define()
			return connector
		},
	}
}
