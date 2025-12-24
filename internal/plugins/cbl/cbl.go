package cbl

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
	RiskRating                    float64 `json:"riskRating"`
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

// CBLPlugin alerts admins when a harmful player is detected joining their server based on Community Ban List data
type CBLPlugin struct {
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
		ID:                     "cbl",
		Name:                   "Community Ban List",
		Description:            "The CBL plugin alerts admins when a harmful player is detected joining their server based on data from the Community Ban List.",
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
					Name:        "api_timeout_seconds",
					Description: "Timeout for Community Ban List API requests in seconds.",
					Required:    false,
					Type:        plug_config_schema.FieldTypeInt,
					Default:     10,
				},
				{
					Name:        "kick_threshold",
					Description: "Automatically kick players when their reputation points exceed this threshold. Set to 0 to disable auto-kick.",
					Required:    false,
					Type:        plug_config_schema.FieldTypeInt,
					Default:     0,
				},
				{
					Name:        "ignored_steam_ids",
					Description: "Array of Steam IDs to ignore from CBL checks. Players in this list will not be checked, alerted, or kicked.",
					Required:    false,
					Type:        plug_config_schema.FieldTypeArrayString,
					Default:     []interface{}{},
				},
			},
		},

		Events: []event_manager.EventType{
			event_manager.EventTypeLogPlayerConnected,
		},

		CreateInstance: func() plugin_manager.Plugin {
			return &CBLPlugin{}
		},
	}
}

// GetDefinition returns the plugin definition
func (p *CBLPlugin) GetDefinition() plugin_manager.PluginDefinition {
	return Define()
}

func (p *CBLPlugin) GetCommands() []plugin_manager.PluginCommand {
	return []plugin_manager.PluginCommand{}
}

func (p *CBLPlugin) ExecuteCommand(commandID string, params map[string]interface{}) (*plugin_manager.CommandResult, error) {
	return nil, fmt.Errorf("no commands available")
}

func (p *CBLPlugin) GetCommandExecutionStatus(executionID string) (*plugin_manager.CommandExecutionStatus, error) {
	return nil, fmt.Errorf("no commands available")
}

// Initialize initializes the plugin with its configuration and dependencies
func (p *CBLPlugin) Initialize(config map[string]interface{}, apis *plugin_manager.PluginAPIs) error {
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
func (p *CBLPlugin) Start(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.status == plugin_manager.PluginStatusRunning {
		return nil // Already running
	}

	// Validate channel ID
	channelID := p.getStringConfig("channel_id")
	if channelID == "" {
		return fmt.Errorf("channel_id is required but not configured")
	}

	p.ctx, p.cancel = context.WithCancel(ctx)
	p.status = plugin_manager.PluginStatusRunning

	return nil
}

// Stop gracefully stops the plugin
func (p *CBLPlugin) Stop() error {
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

	return nil
}

// HandleEvent processes an event if the plugin is subscribed to it
func (p *CBLPlugin) HandleEvent(event *plugin_manager.PluginEvent) error {
	if event.Type != "LOG_PLAYER_CONNECTED" {
		return nil // Not interested in this event
	}

	return p.handlePlayerConnected(event)
}

// GetStatus returns the current plugin status
func (p *CBLPlugin) GetStatus() plugin_manager.PluginStatus {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.status
}

// GetConfig returns the current plugin configuration
func (p *CBLPlugin) GetConfig() map[string]interface{} {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.config
}

// UpdateConfig updates the plugin configuration
func (p *CBLPlugin) UpdateConfig(config map[string]interface{}) error {
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
	})

	return nil
}

// handlePlayerConnected processes player connected events
func (p *CBLPlugin) handlePlayerConnected(rawEvent *plugin_manager.PluginEvent) error {
	event, ok := rawEvent.Data.(*event_manager.LogPlayerConnectedData)
	if !ok {
		return fmt.Errorf("invalid event data type")
	}

	// Query Community Ban List API in a goroutine to avoid blocking
	go func() {
		// Use p.ctx if available, otherwise use background context
		parentCtx := p.ctx
		if parentCtx == nil {
			parentCtx = context.Background()
		}

		ctx, cancel := context.WithTimeout(parentCtx, p.httpClient.Timeout)
		defer cancel()

		if err := p.checkPlayerAndAlertAutoKickIfNeeded(ctx, event); err != nil {
			p.apis.LogAPI.Error("Failed to check player against Community Ban List", err, map[string]interface{}{
				"steam_id": event.SteamID,
			})
		}
	}()

	return nil
}

// checkPlayerAndAlertAutoKickIfNeeded queries the CBL API and sends Discord alert if needed
func (p *CBLPlugin) checkPlayerAndAlertAutoKickIfNeeded(ctx context.Context, event *event_manager.LogPlayerConnectedData) error {
	// Check if Steam ID is in the ignore list
	ignoredSteamIDs := p.getArrayStringConfig("ignored_steam_ids")
	for _, ignoredID := range ignoredSteamIDs {
		if ignoredID == event.SteamID {
			p.apis.LogAPI.Debug("Player is in CBL ignore list, skipping check", map[string]interface{}{
				"steam_id": event.SteamID,
			})
			return nil
		}
	}

	user, err := p.queryCBLAPI(ctx, event.SteamID)
	if err != nil {
		return fmt.Errorf("failed to query CBL API: %w", err)
	}

	if user == nil {
		return nil
	}

	threshold := p.getIntConfig("threshold")
	if user.ReputationPoints < threshold {
		return nil
	}

	// Send Discord alert
	err = p.sendDiscordAlert(user, event)
	if err != nil {
		return fmt.Errorf("failed to send Discord alert: %w", err)
	}

	// Check for auto-kick
	kickThreshold := p.getIntConfig("kick_threshold")
	if kickThreshold > 0 && user.ReputationPoints >= kickThreshold {
		if err := p.apis.RconAPI.KickPlayer(event.SteamID, "Kicked via https://communitybanlist.com"); err != nil {
			p.apis.LogAPI.Error("Failed to kick player", err, map[string]interface{}{
				"steam_id":          event.SteamID,
				"reputation_points": user.ReputationPoints,
				"kick_threshold":    kickThreshold,
			})
		} else {
			p.apis.LogAPI.Info("Kicked player due to high reputation points", map[string]interface{}{
				"steam_id":          event.SteamID,
				"reputation_points": user.ReputationPoints,
				"kick_threshold":    kickThreshold,
			})
		}
	}

	return nil
}

// queryCBLAPI queries the Community Ban List GraphQL API
func (p *CBLPlugin) queryCBLAPI(ctx context.Context, steamID string) (*CBLUser, error) {
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
func (p *CBLPlugin) sendDiscordAlert(user *CBLUser, event *event_manager.LogPlayerConnectedData) error {
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

	if _, err := p.discordAPI.SendEmbed(channelID, embed); err != nil {
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

func (p *CBLPlugin) getStringConfig(key string) string {
	if value, ok := p.config[key].(string); ok {
		return value
	}
	return ""
}

func (p *CBLPlugin) getIntConfig(key string) int {
	if value, ok := p.config[key].(int); ok {
		return value
	}
	if value, ok := p.config[key].(float64); ok {
		return int(value)
	}
	return 0
}

func (p *CBLPlugin) getBoolConfig(key string) bool {
	if value, ok := p.config[key].(bool); ok {
		return value
	}
	return false
}

func (p *CBLPlugin) getArrayStringConfig(key string) []string {
	if value, ok := p.config[key].([]interface{}); ok {
		result := make([]string, 0, len(value))
		for _, v := range value {
			if str, ok := v.(string); ok {
				result = append(result, str)
			}
		}
		return result
	}
	return []string{}
}
