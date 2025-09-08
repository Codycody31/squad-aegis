package discord

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/rs/zerolog/log"
	"go.codycody31.dev/squad-aegis/internal/plugin_manager"
	"go.codycody31.dev/squad-aegis/internal/shared/plug_config_schema"
)

// DiscordConnector implements the Discord connector
type DiscordConnector struct {
	session *discordgo.Session
	config  *DiscordConfig
	mu      sync.RWMutex
	status  plugin_manager.ConnectorStatus
}

// DiscordConfig represents the Discord connector configuration
type DiscordConfig struct {
	Token   string `json:"token"`
	GuildID string `json:"guild_id"`
}

// DiscordAPI provides Discord functionality to plugins
type DiscordAPI interface {
	// SendMessage sends a message to a Discord channel
	SendMessage(channelID, content string) error

	// SendEmbed sends an embed message to a Discord channel
	SendEmbed(channelID string, embed *DiscordEmbed) error

	// GetGuildID returns the configured guild ID
	GetGuildID() string

	// GetChannelMembers returns members of a specific channel (if voice channel)
	GetChannelMembers(channelID string) ([]*DiscordMember, error)

	// GetGuildMembers returns all members of the guild
	GetGuildMembers() ([]*DiscordMember, error)

	// HasRole checks if a user has a specific role
	HasRole(userID, roleID string) (bool, error)

	// AddRole adds a role to a user
	AddRole(userID, roleID string) error

	// RemoveRole removes a role from a user
	RemoveRole(userID, roleID string) error
}

// DiscordEmbed represents a Discord embed message
type DiscordEmbed struct {
	Title       string                 `json:"title,omitempty"`
	Description string                 `json:"description,omitempty"`
	Color       int                    `json:"color,omitempty"`
	Fields      []*DiscordEmbedField   `json:"fields,omitempty"`
	Footer      *DiscordEmbedFooter    `json:"footer,omitempty"`
	Thumbnail   *DiscordEmbedThumbnail `json:"thumbnail,omitempty"`
	Image       *DiscordEmbedImage     `json:"image,omitempty"`
	Timestamp   *time.Time             `json:"timestamp,omitempty"`
}

// DiscordEmbedField represents an embed field
type DiscordEmbedField struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Inline bool   `json:"inline,omitempty"`
}

// DiscordEmbedFooter represents an embed footer
type DiscordEmbedFooter struct {
	Text    string `json:"text"`
	IconURL string `json:"icon_url,omitempty"`
}

// DiscordEmbedThumbnail represents an embed thumbnail
type DiscordEmbedThumbnail struct {
	URL string `json:"url"`
}

// DiscordEmbedImage represents an embed image
type DiscordEmbedImage struct {
	URL string `json:"url"`
}

// DiscordMember represents a Discord guild member
type DiscordMember struct {
	UserID      string    `json:"user_id"`
	Username    string    `json:"username"`
	DisplayName string    `json:"display_name"`
	Roles       []string  `json:"roles"`
	JoinedAt    time.Time `json:"joined_at"`
	IsBot       bool      `json:"is_bot"`
}

// Define returns the connector definition
func Define() plugin_manager.ConnectorDefinition {
	return plugin_manager.ConnectorDefinition{
		ID:          "discord",
		Name:        "Discord",
		Description: "Discord bot connector for sending messages and managing Discord integration",
		Version:     "1.0.0",
		Author:      "Squad Aegis",

		ConfigSchema: plug_config_schema.ConfigSchema{
			Fields: []plug_config_schema.ConfigField{
				{
					Name:        "token",
					Description: "Discord bot token",
					Required:    true,
					Type:        plug_config_schema.FieldTypeString,
					Sensitive:   true,
				},
				{
					Name:        "guild_id",
					Description: "Discord guild (server) ID",
					Required:    true,
					Type:        plug_config_schema.FieldTypeString,
				},
			},
		},

		APIInterface: (*DiscordAPI)(nil),

		CreateInstance: func() plugin_manager.Connector {
			return &DiscordConnector{}
		},
	}
}

// GetDefinition returns the connector definition
func (c *DiscordConnector) GetDefinition() plugin_manager.ConnectorDefinition {
	return Define()
}

// Initialize initializes the Discord connector
func (c *DiscordConnector) Initialize(config map[string]interface{}) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Validate and parse config
	definition := c.GetDefinition()
	if err := definition.ConfigSchema.Validate(config); err != nil {
		return fmt.Errorf("invalid config: %w", err)
	}

	// Fill defaults
	definition.ConfigSchema.FillDefaults(config)

	// Extract config values
	c.config = &DiscordConfig{
		Token:   config["token"].(string),
		GuildID: config["guild_id"].(string),
	}

	if c.config.Token == "" {
		return fmt.Errorf("discord token is required")
	}

	if c.config.GuildID == "" {
		return fmt.Errorf("discord guild_id is required")
	}

	// Create Discord session
	session, err := discordgo.New("Bot " + c.config.Token)
	if err != nil {
		return fmt.Errorf("failed to create Discord session: %w", err)
	}

	c.session = session
	c.status = plugin_manager.ConnectorStatusStopped

	return nil
}

// Start starts the Discord connector
func (c *DiscordConnector) Start(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.status == plugin_manager.ConnectorStatusRunning {
		return nil // Already running
	}

	c.status = plugin_manager.ConnectorStatusStarting

	// Open Discord connection
	if err := c.session.Open(); err != nil {
		c.status = plugin_manager.ConnectorStatusError
		return fmt.Errorf("failed to open Discord connection: %w", err)
	}

	// Verify we can access the guild
	_, err := c.session.Guild(c.config.GuildID)
	if err != nil {
		c.session.Close()
		c.status = plugin_manager.ConnectorStatusError
		return fmt.Errorf("failed to access Discord guild %s: %w", c.config.GuildID, err)
	}

	c.status = plugin_manager.ConnectorStatusRunning

	log.Info().
		Str("guildID", c.config.GuildID).
		Msg("Discord connector started successfully")

	// Handle context cancellation
	go func() {
		<-ctx.Done()
		c.Stop()
	}()

	return nil
}

// Stop stops the Discord connector
func (c *DiscordConnector) Stop() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.status == plugin_manager.ConnectorStatusStopped {
		return nil // Already stopped
	}

	c.status = plugin_manager.ConnectorStatusStopping

	if c.session != nil {
		c.session.Close()
	}

	c.status = plugin_manager.ConnectorStatusStopped

	log.Info().Msg("Discord connector stopped")

	return nil
}

// GetStatus returns the current connector status
func (c *DiscordConnector) GetStatus() plugin_manager.ConnectorStatus {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.status
}

// GetConfig returns the current connector configuration
func (c *DiscordConnector) GetConfig() map[string]interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.config == nil {
		return make(map[string]interface{})
	}

	return map[string]interface{}{
		"token":    c.config.Token,
		"guild_id": c.config.GuildID,
	}
}

// UpdateConfig updates the connector configuration
func (c *DiscordConnector) UpdateConfig(config map[string]interface{}) error {
	// For Discord, we need to restart the connection with new config
	if err := c.Stop(); err != nil {
		return fmt.Errorf("failed to stop connector for config update: %w", err)
	}

	if err := c.Initialize(config); err != nil {
		return fmt.Errorf("failed to reinitialize connector: %w", err)
	}

	// Note: Start() should be called by the plugin manager after this
	return nil
}

// GetAPI returns the Discord API interface
func (c *DiscordConnector) GetAPI() interface{} {
	return &discordAPI{connector: c}
}

// discordAPI implements DiscordAPI interface
type discordAPI struct {
	connector *DiscordConnector
}

func (api *discordAPI) SendMessage(channelID, content string) error {
	api.connector.mu.RLock()
	defer api.connector.mu.RUnlock()

	if api.connector.status != plugin_manager.ConnectorStatusRunning {
		return fmt.Errorf("Discord connector is not running")
	}

	_, err := api.connector.session.ChannelMessageSend(channelID, content)
	if err != nil {
		return fmt.Errorf("failed to send Discord message: %w", err)
	}

	return nil
}

func (api *discordAPI) SendEmbed(channelID string, embed *DiscordEmbed) error {
	api.connector.mu.RLock()
	defer api.connector.mu.RUnlock()

	if api.connector.status != plugin_manager.ConnectorStatusRunning {
		return fmt.Errorf("Discord connector is not running")
	}

	// Convert our embed to discordgo embed
	discordEmbed := &discordgo.MessageEmbed{
		Title:       embed.Title,
		Description: embed.Description,
		Color:       embed.Color,
	}

	// Convert fields
	if embed.Fields != nil {
		discordEmbed.Fields = make([]*discordgo.MessageEmbedField, len(embed.Fields))
		for i, field := range embed.Fields {
			discordEmbed.Fields[i] = &discordgo.MessageEmbedField{
				Name:   field.Name,
				Value:  field.Value,
				Inline: field.Inline,
			}
		}
	}

	// Convert footer
	if embed.Footer != nil {
		discordEmbed.Footer = &discordgo.MessageEmbedFooter{
			Text:    embed.Footer.Text,
			IconURL: embed.Footer.IconURL,
		}
	}

	// Convert thumbnail
	if embed.Thumbnail != nil {
		discordEmbed.Thumbnail = &discordgo.MessageEmbedThumbnail{
			URL: embed.Thumbnail.URL,
		}
	}

	// Convert image
	if embed.Image != nil {
		discordEmbed.Image = &discordgo.MessageEmbedImage{
			URL: embed.Image.URL,
		}
	}

	// Convert timestamp
	if embed.Timestamp != nil {
		discordEmbed.Timestamp = embed.Timestamp.Format(time.RFC3339)
	}

	_, err := api.connector.session.ChannelMessageSendEmbed(channelID, discordEmbed)
	if err != nil {
		return fmt.Errorf("failed to send Discord embed: %w", err)
	}

	return nil
}

func (api *discordAPI) GetGuildID() string {
	api.connector.mu.RLock()
	defer api.connector.mu.RUnlock()

	if api.connector.config == nil {
		return ""
	}

	return api.connector.config.GuildID
}

func (api *discordAPI) GetChannelMembers(channelID string) ([]*DiscordMember, error) {
	api.connector.mu.RLock()
	defer api.connector.mu.RUnlock()

	if api.connector.status != plugin_manager.ConnectorStatusRunning {
		return nil, fmt.Errorf("Discord connector is not running")
	}

	// This is primarily for voice channels
	// For text channels, this doesn't make much sense as all guild members can see them
	// We'll return an empty list for now
	return []*DiscordMember{}, nil
}

func (api *discordAPI) GetGuildMembers() ([]*DiscordMember, error) {
	api.connector.mu.RLock()
	defer api.connector.mu.RUnlock()

	if api.connector.status != plugin_manager.ConnectorStatusRunning {
		return nil, fmt.Errorf("Discord connector is not running")
	}

	members, err := api.connector.session.GuildMembers(api.connector.config.GuildID, "", 1000)
	if err != nil {
		return nil, fmt.Errorf("failed to get guild members: %w", err)
	}

	result := make([]*DiscordMember, len(members))
	for i, member := range members {
		// Use the JoinedAt time directly since it's already a time.Time
		joinedAt := member.JoinedAt

		displayName := member.Nick
		if displayName == "" {
			displayName = member.User.Username
		}

		result[i] = &DiscordMember{
			UserID:      member.User.ID,
			Username:    member.User.Username,
			DisplayName: displayName,
			Roles:       member.Roles,
			JoinedAt:    joinedAt,
			IsBot:       member.User.Bot,
		}
	}

	return result, nil
}

func (api *discordAPI) HasRole(userID, roleID string) (bool, error) {
	api.connector.mu.RLock()
	defer api.connector.mu.RUnlock()

	if api.connector.status != plugin_manager.ConnectorStatusRunning {
		return false, fmt.Errorf("Discord connector is not running")
	}

	member, err := api.connector.session.GuildMember(api.connector.config.GuildID, userID)
	if err != nil {
		return false, fmt.Errorf("failed to get guild member: %w", err)
	}

	for _, role := range member.Roles {
		if role == roleID {
			return true, nil
		}
	}

	return false, nil
}

func (api *discordAPI) AddRole(userID, roleID string) error {
	api.connector.mu.RLock()
	defer api.connector.mu.RUnlock()

	if api.connector.status != plugin_manager.ConnectorStatusRunning {
		return fmt.Errorf("Discord connector is not running")
	}

	err := api.connector.session.GuildMemberRoleAdd(api.connector.config.GuildID, userID, roleID)
	if err != nil {
		return fmt.Errorf("failed to add role: %w", err)
	}

	return nil
}

func (api *discordAPI) RemoveRole(userID, roleID string) error {
	api.connector.mu.RLock()
	defer api.connector.mu.RUnlock()

	if api.connector.status != plugin_manager.ConnectorStatusRunning {
		return fmt.Errorf("Discord connector is not running")
	}

	err := api.connector.session.GuildMemberRoleRemove(api.connector.config.GuildID, userID, roleID)
	if err != nil {
		return fmt.Errorf("failed to remove role: %w", err)
	}

	return nil
}
