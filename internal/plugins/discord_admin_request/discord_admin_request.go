package discord_admin_request

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"go.codycody31.dev/squad-aegis/internal/connectors/discord"
	"go.codycody31.dev/squad-aegis/internal/plugin_manager"
	"go.codycody31.dev/squad-aegis/internal/shared/plug_config_schema"
)

// DiscordAdminRequestPlugin sends admin requests to Discord
type DiscordAdminRequestPlugin struct {
	// Plugin configuration
	config map[string]interface{}
	apis   *plugin_manager.PluginAPIs

	// Discord connector
	discordAPI discord.DiscordAPI

	// State management
	lastPingTime time.Time
	mu           sync.Mutex
	status       plugin_manager.PluginStatus
	ctx          context.Context
	cancel       context.CancelFunc
}

// Define returns the plugin definition
func Define() plugin_manager.PluginDefinition {
	return plugin_manager.PluginDefinition{
		ID:                     "discord_admin_request",
		Name:                   "Discord Admin Requests",
		Description:            "Will ping admins in a Discord channel when a player requests an admin via the !admin command in in-game chat.",
		Version:                "1.0.0",
		Author:                 "Squad Aegis",
		AllowMultipleInstances: false,
		RequiredConnectors:     []string{"discord"},
		LongRunning:            false,

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

		EventHandlers: []plugin_manager.EventHandler{
			{
				Source:      plugin_manager.EventSourceRCON,
				EventType:   "RCON_CHAT_MESSAGE",
				Description: "Handles admin requests from in-game chat",
			},
		},

		CreateInstance: func() plugin_manager.Plugin {
			return &DiscordAdminRequestPlugin{}
		},
	}
}

// GetDefinition returns the plugin definition
func (p *DiscordAdminRequestPlugin) GetDefinition() plugin_manager.PluginDefinition {
	return Define()
}

// Initialize initializes the plugin with its configuration and dependencies
func (p *DiscordAdminRequestPlugin) Initialize(config map[string]interface{}, apis *plugin_manager.PluginAPIs) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.config = config
	p.apis = apis
	p.status = plugin_manager.PluginStatusStopped

	// Validate config
	definition := p.GetDefinition()
	if err := definition.ConfigSchema.Validate(config); err != nil {
		return fmt.Errorf("invalid config: %w", err)
	}

	// Fill defaults
	definition.ConfigSchema.FillDefaults(config)

	// Get Discord connector
	discordConnector, err := apis.ConnectorAPI.GetConnector("discord")
	if err != nil {
		return fmt.Errorf("failed to get Discord connector: %w", err)
	}

	// Type assertion
	var ok bool
	p.discordAPI, ok = discordConnector.(discord.DiscordAPI)
	if !ok {
		return fmt.Errorf("invalid Discord connector type")
	}

	p.status = plugin_manager.PluginStatusStopped

	p.apis.LogAPI.Info("Discord Admin Request plugin initialized", map[string]interface{}{
		"channelID": config["channel_id"],
	})

	return nil
}

// Start begins plugin execution (for long-running plugins)
func (p *DiscordAdminRequestPlugin) Start(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.status == plugin_manager.PluginStatusRunning {
		return nil // Already running
	}

	p.ctx, p.cancel = context.WithCancel(ctx)
	p.status = plugin_manager.PluginStatusRunning

	p.apis.LogAPI.Info("Discord Admin Request plugin started", nil)

	return nil
}

// Stop gracefully stops the plugin
func (p *DiscordAdminRequestPlugin) Stop() error {
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

	p.apis.LogAPI.Info("Discord Admin Request plugin stopped", nil)

	return nil
}

// HandleEvent processes an event if the plugin is subscribed to it
func (p *DiscordAdminRequestPlugin) HandleEvent(event *plugin_manager.PluginEvent) error {
	if event.Type != "RCON_CHAT_MESSAGE" {
		return nil // Not interested in this event
	}

	return p.handleChatMessage(event)
}

// GetStatus returns the current plugin status
func (p *DiscordAdminRequestPlugin) GetStatus() plugin_manager.PluginStatus {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.status
}

// GetConfig returns the current plugin configuration
func (p *DiscordAdminRequestPlugin) GetConfig() map[string]interface{} {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.config
}

// UpdateConfig updates the plugin configuration
func (p *DiscordAdminRequestPlugin) UpdateConfig(config map[string]interface{}) error {
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

	p.apis.LogAPI.Info("Discord Admin Request plugin configuration updated", map[string]interface{}{
		"channelID": config["channel_id"],
	})

	return nil
}

// handleChatMessage processes chat messages looking for admin requests
func (p *DiscordAdminRequestPlugin) handleChatMessage(event *plugin_manager.PluginEvent) error {
	// TODO: Array of roles that are considered admins

	// Extract message data
	message, ok := event.Data["message"].(string)
	if !ok {
		return nil // No message data
	}

	chatType, ok := event.Data["chat"].(string)
	if !ok {
		chatType = "Unknown"
	}

	playerName, ok := event.Data["name"].(string)
	if !ok {
		playerName = "Unknown Player"
	}

	playerID, _ := event.Data["id"].(string)
	steamID, _ := event.Data["steamId"].(string)

	// Check if this is an admin request
	if !p.isAdminRequest(message) {
		return nil
	}

	// Check if we should ignore this chat type
	if p.shouldIgnoreChat(chatType) {
		return nil
	}

	// Get server info
	serverInfo, err := p.apis.ServerAPI.GetServerInfo()
	if err != nil {
		p.apis.LogAPI.Error("Failed to get server info", err, nil)
		serverInfo = &plugin_manager.ServerInfo{
			Name: "Unknown Server",
		}
	}

	// Get admin count
	admins, err := p.apis.ServerAPI.GetAdmins()
	if err != nil {
		p.apis.LogAPI.Error("Failed to get admin list", err, nil)
		admins = []*plugin_manager.AdminInfo{}
	}

	onlineAdmins := 0
	for _, admin := range admins {
		if admin.IsOnline {
			onlineAdmins++
		}
	}

	// Send Discord notification
	if err := p.sendAdminRequestNotification(serverInfo, playerName, playerID, steamID, message, onlineAdmins); err != nil {
		p.apis.LogAPI.Error("Failed to send Discord notification", err, map[string]interface{}{
			"player":  playerName,
			"message": message,
		})
		return err
	}

	// Send in-game response
	if err := p.sendInGameResponse(steamID, onlineAdmins); err != nil {
		p.apis.LogAPI.Error("Failed to send in-game response", err, map[string]interface{}{
			"player": playerName,
		})
	}

	// Warn in-game admins if configured
	if p.getBoolConfig("warn_in_game_admins") {
		if err := p.warnInGameAdmins(playerName, message); err != nil {
			p.apis.LogAPI.Error("Failed to warn in-game admins", err, map[string]interface{}{
				"player": playerName,
			})
		}
	}

	p.apis.LogAPI.Info("Processed admin request", map[string]interface{}{
		"player":       playerName,
		"message":      message,
		"onlineAdmins": onlineAdmins,
	})

	return nil
}

// isAdminRequest checks if a message is an admin request
func (p *DiscordAdminRequestPlugin) isAdminRequest(message string) bool {
	message = strings.ToLower(strings.TrimSpace(message))
	return strings.HasPrefix(message, "!admin") || strings.Contains(message, "admin help") || strings.Contains(message, "need admin")
}

// shouldIgnoreChat checks if we should ignore this chat type
func (p *DiscordAdminRequestPlugin) shouldIgnoreChat(chatType string) bool {
	ignoreChats := p.getStringArrayConfig("ignore_chats")
	for _, ignore := range ignoreChats {
		if strings.EqualFold(chatType, ignore) {
			return true
		}
	}
	return false
}

// sendAdminRequestNotification sends the Discord notification
func (p *DiscordAdminRequestPlugin) sendAdminRequestNotification(serverInfo *plugin_manager.ServerInfo, playerName, playerID, steamID, message string, onlineAdmins int) error {
	channelID := p.getStringConfig("channel_id")
	if channelID == "" {
		return fmt.Errorf("channel_id not configured")
	}

	// Check ping cooldown
	if !p.canPing() {
		return p.sendWithoutPing(channelID, serverInfo, playerName, playerID, steamID, message, onlineAdmins)
	}

	// Build ping string
	pingString := p.buildPingString()

	players, err := p.apis.ServerAPI.GetPlayers()
	if err != nil {
		p.apis.LogAPI.Error("Failed to get players", err, nil)
		players = []*plugin_manager.PlayerInfo{}
	}

	player := &plugin_manager.PlayerInfo{}
	for _, p := range players {
		if p.SteamID == steamID {
			player = p
			break
		}
	}

	// Create embed
	embed := &discord.DiscordEmbed{
		Title: fmt.Sprintf("Player **%s** has requested admin support!", playerName),
		Color: p.getIntConfig("color"),
		Fields: []*discord.DiscordEmbedField{
			{
				Name:   "Player",
				Value:  playerName,
				Inline: true,
			},
			{
				Name:   "Steam ID",
				Value:  fmt.Sprintf("[%s](https://steamcommunity.com/profiles/%s)", steamID, steamID),
				Inline: true,
			},
			{
				Name:   "Player's EosID",
				Value:  player.EOSID,
				Inline: true,
			},
			{
				Name:   "Team & Squad",
				Value:  fmt.Sprintf("Team: %d, Squad: %d", player.TeamID, player.SquadID),
				Inline: false,
			},
			{
				Name:   "Reason",
				Value:  strings.ReplaceAll(message, "!admin", ""),
				Inline: false,
			},
			{
				Name:   "Online Admins",
				Value:  fmt.Sprintf("%d", onlineAdmins),
				Inline: false,
			},
		},
		Footer: &discord.DiscordEmbedFooter{
			Text: "Powered by Squad Aegis",
		},
		Timestamp: func() *time.Time { t := time.Now(); return &t }(),
	}

	// Add player details if available
	if playerID != "" {
		embed.Fields = append(embed.Fields, &discord.DiscordEmbedField{
			Name:   "Player ID",
			Value:  playerID,
			Inline: true,
		})
	}

	if steamID != "" {
		embed.Fields = append(embed.Fields, &discord.DiscordEmbedField{
			Name:   "Steam ID",
			Value:  steamID,
			Inline: true,
		})
	}

	// Send with ping
	content := pingString
	if content != "" {
		if err := p.discordAPI.SendMessage(channelID, content); err != nil {
			return fmt.Errorf("failed to send ping message: %w", err)
		}
	}

	// Send embed
	if err := p.discordAPI.SendEmbed(channelID, embed); err != nil {
		return fmt.Errorf("failed to send embed: %w", err)
	}

	// Update last ping time
	p.mu.Lock()
	p.lastPingTime = time.Now()
	p.mu.Unlock()

	return nil
}

// sendWithoutPing sends notification without ping due to cooldown
func (p *DiscordAdminRequestPlugin) sendWithoutPing(channelID string, serverInfo *plugin_manager.ServerInfo, playerName, playerID, steamID, message string, onlineAdmins int) error {
	embed := &discord.DiscordEmbed{
		Title:       "🔕 Admin Request (Ping on Cooldown)",
		Description: fmt.Sprintf("**Player:** %s\n**Message:** %s", playerName, message),
		Color:       p.getIntConfig("color"),
		Fields: []*discord.DiscordEmbedField{
			{
				Name:   "Server",
				Value:  serverInfo.Name,
				Inline: true,
			},
			{
				Name:   "Online Admins",
				Value:  fmt.Sprintf("%d", onlineAdmins),
				Inline: true,
			},
		},
		Footer: &discord.DiscordEmbedFooter{
			Text: "Squad Aegis Admin Request System (Ping Cooldown Active)",
		},
		Timestamp: func() *time.Time { t := time.Now(); return &t }(),
	}

	// Add player details if available
	if playerID != "" {
		embed.Fields = append(embed.Fields, &discord.DiscordEmbedField{
			Name:   "Player ID",
			Value:  playerID,
			Inline: true,
		})
	}

	if steamID != "" {
		embed.Fields = append(embed.Fields, &discord.DiscordEmbedField{
			Name:   "Steam ID",
			Value:  steamID,
			Inline: true,
		})
	}

	return p.discordAPI.SendEmbed(channelID, embed)
}

// sendInGameResponse sends a response to the player in-game
func (p *DiscordAdminRequestPlugin) sendInGameResponse(playerSteamID string, onlineAdmins int) error {
	if !p.getBoolConfig("show_in_game_admins") {
		return p.apis.RconAPI.SendMessageToPlayer(playerSteamID, "An admin has been notified. Please wait for us to get back to you.")
	}

	if onlineAdmins == 0 {
		return p.apis.RconAPI.SendMessageToPlayer(playerSteamID, "There are no in-game admins, however, an admin has been notified via Discord. Please wait for us to get back to you.")
	}

	return p.apis.RconAPI.SendMessageToPlayer(playerSteamID, fmt.Sprintf("There are %d in-game admin(s). Please wait for us to get back to you.", onlineAdmins))
}

// warnInGameAdmins sends a warning to in-game admins
func (p *DiscordAdminRequestPlugin) warnInGameAdmins(playerName, message string) error {
	adminMessage := fmt.Sprintf("AdminWarn [%s] %s", playerName, message)

	// Get online admins and send them individual messages
	admins, err := p.apis.ServerAPI.GetAdmins()
	if err != nil {
		p.apis.LogAPI.Error("Failed to get admins", err, nil)
		return nil
	}

	// Send to each online admin individually
	for _, admin := range admins {
		if admin.IsOnline {
			if err := p.apis.RconAPI.SendMessageToPlayer(admin.SteamID, adminMessage); err != nil {
				p.apis.LogAPI.Error("Failed to send message to admin", err, map[string]interface{}{
					"adminID":   admin.SteamID,
					"adminName": admin.Name,
				})
			}
		}
	}

	return nil
}

// canPing checks if we can send a ping (not on cooldown)
func (p *DiscordAdminRequestPlugin) canPing() bool {
	p.mu.Lock()
	defer p.mu.Unlock()

	pingDelay := time.Duration(p.getIntConfig("ping_delay")) * time.Millisecond
	return time.Since(p.lastPingTime) >= pingDelay
}

// buildPingString builds the ping string based on configuration
func (p *DiscordAdminRequestPlugin) buildPingString() string {
	var pings []string

	// Add @here if configured
	if p.getBoolConfig("ping_here") {
		pings = append(pings, "@here")
	}

	// Add role pings
	pingGroups := p.getStringArrayConfig("ping_groups")
	for _, roleID := range pingGroups {
		pings = append(pings, fmt.Sprintf("<@&%s>", roleID))
	}

	if len(pings) == 0 {
		return ""
	}

	return strings.Join(pings, " ")
}

// Helper methods for config access

func (p *DiscordAdminRequestPlugin) getStringConfig(key string) string {
	if value, ok := p.config[key].(string); ok {
		return value
	}
	return ""
}

func (p *DiscordAdminRequestPlugin) getIntConfig(key string) int {
	if value, ok := p.config[key].(int); ok {
		return value
	}
	if value, ok := p.config[key].(float64); ok {
		return int(value)
	}
	return 0
}

func (p *DiscordAdminRequestPlugin) getBoolConfig(key string) bool {
	if value, ok := p.config[key].(bool); ok {
		return value
	}
	return false
}

func (p *DiscordAdminRequestPlugin) getStringArrayConfig(key string) []string {
	if value, ok := p.config[key].([]interface{}); ok {
		result := make([]string, len(value))
		for i, v := range value {
			if str, ok := v.(string); ok {
				result[i] = str
			}
		}
		return result
	}
	return []string{}
}
