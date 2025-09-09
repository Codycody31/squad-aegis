package cbl_info

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"go.codycody31.dev/squad-aegis/internal/connectors/discord"
	"go.codycody31.dev/squad-aegis/internal/event_manager"
	"go.codycody31.dev/squad-aegis/internal/plugin_manager"
	"go.codycody31.dev/squad-aegis/internal/shared/plug_config_schema"
)

// CBLUser represents a user from the Community Ban List API
type CBLUser struct {
	ID                            string  `json:"id"`
	Name                          string  `json:"name"`
	AvatarFull                    string  `json:"avatarFull"`
	ReputationPoints              int     `json:"reputationPoints"`
	RiskRating                    int     `json:"riskRating"`
	ReputationRank                int     `json:"reputationRank"`
	LastRefreshedInfo             string  `json:"lastRefreshedInfo"`
	LastRefreshedReputationPoints string  `json:"lastRefreshedReputationPoints"`
	LastRefreshedReputationRank   string  `json:"lastRefreshedReputationRank"`
	ReputationPointsMonthChange   int     `json:"reputationPointsMonthChange"`
	ActiveBans                    BanList `json:"activeBans"`
	ExpiredBans                   BanList `json:"expiredBans"`
}

// BanList represents a list of bans
type BanList struct {
	Edges []BanEdge `json:"edges"`
}

// BanEdge represents a ban edge
type BanEdge struct {
	Cursor string `json:"cursor"`
	Node   Ban    `json:"node"`
}

// Ban represents a ban
type Ban struct {
	ID string `json:"id"`
}

// GraphQLRequest represents a GraphQL request
type GraphQLRequest struct {
	Query     string                 `json:"query"`
	Variables map[string]interface{} `json:"variables"`
}

// GraphQLResponse represents a GraphQL response
type GraphQLResponse struct {
	Data struct {
		SteamUser *CBLUser `json:"steamUser"`
	} `json:"data"`
	Errors []struct {
		Message string `json:"message"`
	} `json:"errors"`
}

// CBLInfoPlugin alerts admins when a harmful player is detected joining their server based on Community Ban List data
type CBLInfoPlugin struct {
	// Plugin configuration
	config map[string]interface{}
	apis   *plugin_manager.PluginAPIs

	// Discord connector
	discordAPI discord.DiscordAPI

	// State management
	mu     sync.Mutex
	status plugin_manager.PluginStatus
	ctx    context.Context
	cancel context.CancelFunc

	// HTTP client for API requests
	httpClient *http.Client
}

// Define returns the plugin definition
func Define() plugin_manager.PluginDefinition {
	return plugin_manager.PluginDefinition{
		ID:                     "cbl_info",
		Name:                   "Community Ban List Info",
		Description:            "The CBL Info plugin alerts admins when a harmful player is detected joining their server based on data from the Community Ban List.",
		Version:                "1.0.0",
		Author:                 "Squad Aegis",
		AllowMultipleInstances: false,
		RequiredConnectors:     []string{"discord"},
		LongRunning:            false,

		ConfigSchema: plug_config_schema.ConfigSchema{
			Fields: []plug_config_schema.ConfigField{
				{
					Name:        "channel_id",
					Description: "The ID of the channel to alert admins through.",
					Required:    true,
					Type:        plug_config_schema.FieldTypeString,
					Default:     "",
				},
				{
					Name:        "threshold",
					Description: "Admins will be alerted when a player has this or more reputation points. For more information on reputation points, see the Community Ban List's FAQ.",
					Required:    false,
					Type:        plug_config_schema.FieldTypeInt,
					Default:     6,
				},
				{
					Name:        "enabled",
					Description: "Whether the plugin is enabled.",
					Required:    false,
					Type:        plug_config_schema.FieldTypeBool,
					Default:     true,
				},
				{
					Name:        "api_timeout_seconds",
					Description: "Timeout for Community Ban List API requests in seconds.",
					Required:    false,
					Type:        plug_config_schema.FieldTypeInt,
					Default:     10,
				},
			},
		},

		Events: []event_manager.EventType{
			event_manager.EventTypeLogPlayerConnected,
		},

		CreateInstance: func() plugin_manager.Plugin {
			return &CBLInfoPlugin{}
		},
	}
}

// GetDefinition returns the plugin definition
func (p *CBLInfoPlugin) GetDefinition() plugin_manager.PluginDefinition {
	return Define()
}

// Initialize initializes the plugin with its configuration and dependencies
func (p *CBLInfoPlugin) Initialize(config map[string]interface{}, apis *plugin_manager.PluginAPIs) error {
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

	// Initialize HTTP client
	timeout := p.getIntConfig("api_timeout_seconds")
	if timeout <= 0 {
		timeout = 10
	}
	p.httpClient = &http.Client{
		Timeout: time.Duration(timeout) * time.Second,
	}

	p.status = plugin_manager.PluginStatusStopped

	return nil
}

// Start begins plugin execution (for long-running plugins)
func (p *CBLInfoPlugin) Start(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.status == plugin_manager.PluginStatusRunning {
		return nil // Already running
	}

	// Check if plugin is enabled
	if !p.getBoolConfig("enabled") {
		p.apis.LogAPI.Info("CBL Info plugin is disabled", nil)
		return nil
	}

	// Validate channel ID
	channelID := p.getStringConfig("channel_id")
	if channelID == "" {
		return fmt.Errorf("channel_id is required but not configured")
	}

	p.ctx, p.cancel = context.WithCancel(ctx)
	p.status = plugin_manager.PluginStatusRunning

	p.apis.LogAPI.Info("CBL Info plugin started", map[string]interface{}{
		"channel_id": channelID,
		"threshold":  p.getIntConfig("threshold"),
	})

	return nil
}

// Stop gracefully stops the plugin
func (p *CBLInfoPlugin) Stop() error {
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

	p.apis.LogAPI.Info("CBL Info plugin stopped", nil)

	return nil
}

// HandleEvent processes an event if the plugin is subscribed to it
func (p *CBLInfoPlugin) HandleEvent(event *plugin_manager.PluginEvent) error {
	if event.Type != "LOG_PLAYER_CONNECTED" {
		return nil // Not interested in this event
	}

	return p.handlePlayerConnected(event)
}

// GetStatus returns the current plugin status
func (p *CBLInfoPlugin) GetStatus() plugin_manager.PluginStatus {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.status
}

// GetConfig returns the current plugin configuration
func (p *CBLInfoPlugin) GetConfig() map[string]interface{} {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.config
}

// UpdateConfig updates the plugin configuration
func (p *CBLInfoPlugin) UpdateConfig(config map[string]interface{}) error {
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

	// Update HTTP client timeout if needed
	timeout := p.getIntConfig("api_timeout_seconds")
	if timeout <= 0 {
		timeout = 10
	}
	p.httpClient.Timeout = time.Duration(timeout) * time.Second

	p.apis.LogAPI.Info("CBL Info plugin configuration updated", map[string]interface{}{
		"channel_id": config["channel_id"],
		"threshold":  config["threshold"],
		"enabled":    config["enabled"],
	})

	return nil
}

// handlePlayerConnected processes player connected events
func (p *CBLInfoPlugin) handlePlayerConnected(rawEvent *plugin_manager.PluginEvent) error {
	if !p.getBoolConfig("enabled") {
		return nil // Plugin is disabled
	}

	event, ok := rawEvent.Data.(*event_manager.LogPlayerConnectedData)
	if !ok {
		return fmt.Errorf("invalid event data type")
	}

	// Query Community Ban List API in a goroutine to avoid blocking
	go func() {
		ctx, cancel := context.WithTimeout(p.ctx, p.httpClient.Timeout)
		defer cancel()

		if err := p.checkPlayerAndAlert(ctx, event); err != nil {
			p.apis.LogAPI.Error("Failed to check player against Community Ban List", err, map[string]interface{}{
				"steam_id": event.SteamID,
			})
		}
	}()

	return nil
}

// checkPlayerAndAlert queries the CBL API and sends Discord alert if needed
func (p *CBLInfoPlugin) checkPlayerAndAlert(ctx context.Context, event *event_manager.LogPlayerConnectedData) error {
	user, err := p.queryCBLAPI(ctx, event.SteamID)
	if err != nil {
		return fmt.Errorf("failed to query CBL API: %w", err)
	}

	if user == nil {
		p.apis.LogAPI.Debug("Player not found in Community Ban List", map[string]interface{}{
			"steam_id": event.SteamID,
		})
		return nil
	}

	threshold := p.getIntConfig("threshold")
	if user.ReputationPoints < threshold {
		p.apis.LogAPI.Debug("Player reputation below threshold", map[string]interface{}{
			"steam_id":          event.SteamID,
			"reputation_points": user.ReputationPoints,
			"threshold":         threshold,
		})
		return nil
	}

	// Send Discord alert
	return p.sendDiscordAlert(user, event)
}

// queryCBLAPI queries the Community Ban List GraphQL API
func (p *CBLInfoPlugin) queryCBLAPI(ctx context.Context, steamID string) (*CBLUser, error) {
	query := `
		query Search($id: String!) {
			steamUser(id: $id) {
				id
				name
				avatarFull
				reputationPoints
				riskRating
				reputationRank
				lastRefreshedInfo
				lastRefreshedReputationPoints
				lastRefreshedReputationRank
				reputationPointsMonthChange
				activeBans: bans(orderBy: "created", orderDirection: DESC, expired: false) {
					edges {
						cursor
						node {
							id
						}
					}
				}
				expiredBans: bans(orderBy: "created", orderDirection: DESC, expired: true) {
					edges {
						cursor
						node {
							id
						}
					}
				}
			}
		}
	`

	request := GraphQLRequest{
		Query: query,
		Variables: map[string]interface{}{
			"id": steamID,
		},
	}

	jsonData, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal GraphQL request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", "https://communitybanlist.com/graphql", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Squad Aegis CBL Plugin")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	var response GraphQLResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(response.Errors) > 0 {
		return nil, fmt.Errorf("GraphQL errors: %v", response.Errors)
	}

	return response.Data.SteamUser, nil
}

// sendDiscordAlert sends a Discord embed alert about the harmful player
func (p *CBLInfoPlugin) sendDiscordAlert(user *CBLUser, event *event_manager.LogPlayerConnectedData) error {
	channelID := p.getStringConfig("channel_id")
	if channelID == "" {
		return fmt.Errorf("channel_id not configured")
	}

	// Try to get the player name from the server
	playerName := "Unknown Player"
	if players, err := p.apis.ServerAPI.GetPlayers(); err == nil {
		for _, player := range players {
			if player.SteamID == event.SteamID {
				playerName = player.Name
				break
			}
		}
	}

	// If we couldn't find the player name, use the CBL name
	if playerName == "Unknown Player" && user.Name != "" {
		playerName = user.Name
	}

	embed := &discord.DiscordEmbed{
		Title: fmt.Sprintf("%s is a potentially harmful player!", playerName),
		Thumbnail: &discord.DiscordEmbedThumbnail{
			URL: user.AvatarFull,
		},
		Description: fmt.Sprintf("[%s](https://communitybanlist.com/search/%s) has %d reputation points on the Community Ban List and is therefore a potentially harmful player.",
			playerName, event.SteamID, user.ReputationPoints),
		Fields: []*discord.DiscordEmbedField{
			{
				Name:   "Reputation Points",
				Value:  fmt.Sprintf("%d (%d from this month)", user.ReputationPoints, user.ReputationPointsMonthChange),
				Inline: true,
			},
			{
				Name:   "Risk Rating",
				Value:  fmt.Sprintf("%d / 10", user.RiskRating),
				Inline: true,
			},
			{
				Name:   "Reputation Rank",
				Value:  fmt.Sprintf("#%d", user.ReputationRank),
				Inline: true,
			},
			{
				Name:   "Active Bans",
				Value:  fmt.Sprintf("%d", len(user.ActiveBans.Edges)),
				Inline: true,
			},
			{
				Name:   "Expired Bans",
				Value:  fmt.Sprintf("%d", len(user.ExpiredBans.Edges)),
				Inline: true,
			},
		},
		Color: 0xffc40b, // #ffc40b
		Footer: &discord.DiscordEmbedFooter{
			Text: "Powered by Squad Aegis and the Community Ban List",
		},
		Timestamp: func() *time.Time { t := time.Now(); return &t }(),
	}

	if err := p.discordAPI.SendEmbed(channelID, embed); err != nil {
		return fmt.Errorf("failed to send Discord embed: %w", err)
	}

	p.apis.LogAPI.Info("Sent CBL alert for potentially harmful player", map[string]interface{}{
		"player_name":       playerName,
		"steam_id":          event.SteamID,
		"reputation_points": user.ReputationPoints,
		"risk_rating":       user.RiskRating,
		"active_bans":       len(user.ActiveBans.Edges),
		"expired_bans":      len(user.ExpiredBans.Edges),
	})

	return nil
}

// Helper methods for config access

func (p *CBLInfoPlugin) getStringConfig(key string) string {
	if value, ok := p.config[key].(string); ok {
		return value
	}
	return ""
}

func (p *CBLInfoPlugin) getIntConfig(key string) int {
	if value, ok := p.config[key].(int); ok {
		return value
	}
	if value, ok := p.config[key].(float64); ok {
		return int(value)
	}
	return 0
}

func (p *CBLInfoPlugin) getBoolConfig(key string) bool {
	if value, ok := p.config[key].(bool); ok {
		return value
	}
	return false
}
