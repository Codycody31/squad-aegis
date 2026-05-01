package discord

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
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

type DiscordAPI = plugin_manager.DiscordAPI
type DiscordEmbed = plugin_manager.DiscordEmbed
type DiscordEmbedField = plugin_manager.DiscordEmbedField
type DiscordEmbedFooter = plugin_manager.DiscordEmbedFooter
type DiscordEmbedThumbnail = plugin_manager.DiscordEmbedThumbnail
type DiscordEmbedImage = plugin_manager.DiscordEmbedImage

// Define returns the connector definition
func Define() plugin_manager.ConnectorDefinition {
	return plugin_manager.ConnectorDefinition{
		ID:          "com.squad-aegis.connectors.discord",
		LegacyIDs:   []string{"discord"},
		Source:      plugin_manager.PluginSourceBundled,
		Name:        "Discord",
		Description: "Discord bot connector for sending messages and managing Discord integration",
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

		APIInterface: (*plugin_manager.DiscordAPI)(nil),

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

// Invoke handles JSON connector requests (actions: send_message, send_embed).
func (c *DiscordConnector) Invoke(ctx context.Context, req *plugin_manager.ConnectorInvokeRequest) (*plugin_manager.ConnectorInvokeResponse, error) {
	_ = ctx
	out := &plugin_manager.ConnectorInvokeResponse{V: plugin_manager.ConnectorWireProtocolV1}
	if req == nil || req.Data == nil {
		out.OK = false
		out.Error = "missing data"
		return out, nil
	}
	rawAction, ok := req.Data["action"].(string)
	if !ok || rawAction == "" {
		out.OK = false
		out.Error = `data.action is required`
		return out, nil
	}
	action := strings.ToLower(strings.TrimSpace(rawAction))

	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.status != plugin_manager.ConnectorStatusRunning {
		out.OK = false
		out.Error = "Discord connector is not running"
		return out, nil
	}

	switch action {
	case "send_message":
		ch, _ := req.Data["channel_id"].(string)
		content, _ := req.Data["content"].(string)
		if ch == "" || content == "" {
			out.OK = false
			out.Error = "send_message requires channel_id and content"
			return out, nil
		}
		msgID, err := c.sendMessageLocked(ch, content)
		if err != nil {
			out.OK = false
			out.Error = err.Error()
			return out, nil
		}
		out.OK = true
		out.Data = map[string]interface{}{"message_id": msgID}
		return out, nil
	case "send_embed":
		ch, _ := req.Data["channel_id"].(string)
		if ch == "" {
			out.OK = false
			out.Error = "send_embed requires channel_id"
			return out, nil
		}
		embed, err := parseEmbedFromData(req.Data["embed"])
		if err != nil {
			out.OK = false
			out.Error = err.Error()
			return out, nil
		}
		msgID, err := c.sendEmbedLocked(ch, embed)
		if err != nil {
			out.OK = false
			out.Error = err.Error()
			return out, nil
		}
		out.OK = true
		out.Data = map[string]interface{}{"message_id": msgID}
		return out, nil
	default:
		out.OK = false
		out.Error = fmt.Sprintf("unknown action %q", rawAction)
		return out, nil
	}
}

func parseEmbedFromData(v interface{}) (*DiscordEmbed, error) {
	if v == nil {
		return nil, fmt.Errorf("send_embed requires embed")
	}
	switch t := v.(type) {
	case *DiscordEmbed:
		return t, nil
	case map[string]interface{}:
		raw, err := json.Marshal(t)
		if err != nil {
			return nil, fmt.Errorf("invalid embed: %w", err)
		}
		var e DiscordEmbed
		if err := json.Unmarshal(raw, &e); err != nil {
			return nil, fmt.Errorf("invalid embed: %w", err)
		}
		return &e, nil
	default:
		raw, err := json.Marshal(v)
		if err != nil {
			return nil, fmt.Errorf("invalid embed: %w", err)
		}
		var e DiscordEmbed
		if err := json.Unmarshal(raw, &e); err != nil {
			return nil, fmt.Errorf("invalid embed: %w", err)
		}
		return &e, nil
	}
}

func (c *DiscordConnector) sendMessageLocked(channelID, content string) (string, error) {
	msg, err := c.session.ChannelMessageSend(channelID, content)
	if err != nil {
		return "", fmt.Errorf("failed to send Discord message: %w", err)
	}
	return msg.ID, nil
}

func discordEmbedToGo(embed *DiscordEmbed) *discordgo.MessageEmbed {
	discordEmbed := &discordgo.MessageEmbed{
		Title:       embed.Title,
		Description: embed.Description,
		Color:       embed.Color,
	}
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
	if embed.Footer != nil {
		discordEmbed.Footer = &discordgo.MessageEmbedFooter{
			Text:    embed.Footer.Text,
			IconURL: embed.Footer.IconURL,
		}
	}
	if embed.Thumbnail != nil {
		discordEmbed.Thumbnail = &discordgo.MessageEmbedThumbnail{
			URL: embed.Thumbnail.URL,
		}
	}
	if embed.Image != nil {
		discordEmbed.Image = &discordgo.MessageEmbedImage{
			URL: embed.Image.URL,
		}
	}
	if embed.Timestamp != nil {
		discordEmbed.Timestamp = embed.Timestamp.Format(time.RFC3339)
	}
	return discordEmbed
}

func (c *DiscordConnector) sendEmbedLocked(channelID string, embed *DiscordEmbed) (string, error) {
	msg, err := c.session.ChannelMessageSendEmbed(channelID, discordEmbedToGo(embed))
	if err != nil {
		return "", fmt.Errorf("failed to send Discord embed: %w", err)
	}
	return msg.ID, nil
}

// discordAPI implements DiscordAPI interface
type discordAPI struct {
	connector *DiscordConnector
}

func (api *discordAPI) SendMessage(channelID, content string) (string, error) {
	api.connector.mu.RLock()
	defer api.connector.mu.RUnlock()

	if api.connector.status != plugin_manager.ConnectorStatusRunning {
		return "", fmt.Errorf("Discord connector is not running")
	}

	return api.connector.sendMessageLocked(channelID, content)
}

func (api *discordAPI) SendEmbed(channelID string, embed *DiscordEmbed) (string, error) {
	api.connector.mu.RLock()
	defer api.connector.mu.RUnlock()

	if api.connector.status != plugin_manager.ConnectorStatusRunning {
		return "", fmt.Errorf("Discord connector is not running")
	}

	return api.connector.sendEmbedLocked(channelID, embed)
}
