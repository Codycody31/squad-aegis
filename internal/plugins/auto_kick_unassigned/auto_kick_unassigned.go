package auto_kick_unassigned

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.codycody31.dev/squad-aegis/internal/event_manager"
	"go.codycody31.dev/squad-aegis/internal/plugin_manager"
	"go.codycody31.dev/squad-aegis/internal/shared/plug_config_schema"
)

// PlayerTracker tracks information about unassigned players
type PlayerTracker struct {
	Player     *plugin_manager.PlayerInfo `json:"player"`
	Warnings   int                        `json:"warnings"`
	StartTime  time.Time                  `json:"start_time"`
	WarnTicker *time.Ticker               `json:"-"`
	KickTimer  *time.Timer                `json:"-"`
	KickCtx    context.Context            `json:"-"`
	KickCancel context.CancelFunc         `json:"-"`
}

// AutoKickUnassignedPlugin automatically kicks players that are not in a squad after a specified amount of time
type AutoKickUnassignedPlugin struct {
	// Plugin configuration
	config map[string]interface{}
	apis   *plugin_manager.PluginAPIs

	// State management
	mu     sync.Mutex
	status plugin_manager.PluginStatus
	ctx    context.Context
	cancel context.CancelFunc

	// Plugin state
	betweenRounds  bool
	trackedPlayers map[string]*PlayerTracker
	updateTicker   *time.Ticker
	cleanupTicker  *time.Ticker
}

// Define returns the plugin definition
func Define() plugin_manager.PluginDefinition {
	return plugin_manager.PluginDefinition{
		ID:                     "auto_kick_unassigned",
		Name:                   "Auto Kick Unassigned",
		Description:            "The Auto Kick Unassigned plugin will automatically kick players that are not in a squad after a specified amount of time.",
		Version:                "1.0.0",
		Author:                 "Squad Aegis",
		AllowMultipleInstances: false,
		RequiredConnectors:     []string{},
		LongRunning:            true,

		ConfigSchema: plug_config_schema.ConfigSchema{
			Fields: []plug_config_schema.ConfigField{
				{
					Name:        "warning_message",
					Description: "Message to send to players warning them they will be kicked.",
					Required:    false,
					Type:        plug_config_schema.FieldTypeString,
					Default:     "Join a squad, you are unassigned and will be kicked",
				},
				{
					Name:        "kick_message",
					Description: "Message to send to players when they are kicked.",
					Required:    false,
					Type:        plug_config_schema.FieldTypeString,
					Default:     "Unassigned - automatically removed",
				},
				{
					Name:        "frequency_of_warnings",
					Description: "How often in seconds should we warn the player about being unassigned?",
					Required:    false,
					Type:        plug_config_schema.FieldTypeInt,
					Default:     30,
				},
				{
					Name:        "unassigned_timer",
					Description: "How long in seconds to wait before an unassigned player is kicked.",
					Required:    false,
					Type:        plug_config_schema.FieldTypeInt,
					Default:     360,
				},
				{
					Name:        "player_threshold",
					Description: "Player count required for AutoKick to start kicking players, set to -1 to disable.",
					Required:    false,
					Type:        plug_config_schema.FieldTypeInt,
					Default:     93,
				},
				{
					Name:        "round_start_delay",
					Description: "Time delay in seconds from start of the round before AutoKick starts kicking again.",
					Required:    false,
					Type:        plug_config_schema.FieldTypeInt,
					Default:     900,
				},
				{
					Name:        "ignore_admins",
					Description: "Whether admins should be ignored (not kicked).",
					Required:    false,
					Type:        plug_config_schema.FieldTypeBool,
					Default:     false,
				},
				{
					Name:        "tracking_update_interval",
					Description: "How often in seconds to update the tracking list.",
					Required:    false,
					Type:        plug_config_schema.FieldTypeInt,
					Default:     60,
				},
				{
					Name:        "cleanup_interval",
					Description: "How often in seconds to clean up disconnected players from tracking.",
					Required:    false,
					Type:        plug_config_schema.FieldTypeInt,
					Default:     1200,
				},
			},
		},

		Events: []event_manager.EventType{
			event_manager.EventTypeLogGameEventUnified,
		},

		CreateInstance: func() plugin_manager.Plugin {
			return &AutoKickUnassignedPlugin{}
		},
	}
}

// GetDefinition returns the plugin definition
func (p *AutoKickUnassignedPlugin) GetDefinition() plugin_manager.PluginDefinition {
	return Define()
}

// Initialize initializes the plugin with its configuration and dependencies
func (p *AutoKickUnassignedPlugin) Initialize(config map[string]interface{}, apis *plugin_manager.PluginAPIs) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.config = config
	p.apis = apis
	p.status = plugin_manager.PluginStatusStopped
	p.betweenRounds = false
	p.trackedPlayers = make(map[string]*PlayerTracker)

	// Validate config
	definition := p.GetDefinition()
	if err := definition.ConfigSchema.Validate(config); err != nil {
		return fmt.Errorf("invalid config: %w", err)
	}

	// Fill defaults
	definition.ConfigSchema.FillDefaults(config)

	p.status = plugin_manager.PluginStatusStopped

	return nil
}

// Start begins plugin execution (for long-running plugins)
func (p *AutoKickUnassignedPlugin) Start(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.status == plugin_manager.PluginStatusRunning {
		return nil // Already running
	}

	p.ctx, p.cancel = context.WithCancel(ctx)
	p.status = plugin_manager.PluginStatusRunning

	// Start periodic update ticker
	updateInterval := p.getIntConfig("tracking_update_interval")
	if updateInterval <= 0 {
		updateInterval = 60
	}
	p.updateTicker = time.NewTicker(time.Duration(updateInterval) * time.Second)

	// Start cleanup ticker
	cleanupInterval := p.getIntConfig("cleanup_interval")
	if cleanupInterval <= 0 {
		cleanupInterval = 1200
	}
	p.cleanupTicker = time.NewTicker(time.Duration(cleanupInterval) * time.Second)

	// Start background goroutines
	go p.updateTrackingLoop()
	go p.cleanupLoop()

	return nil
}

// Stop gracefully stops the plugin
func (p *AutoKickUnassignedPlugin) Stop() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.status == plugin_manager.PluginStatusStopped {
		return nil // Already stopped
	}

	p.status = plugin_manager.PluginStatusStopping

	// Stop all tickers
	if p.updateTicker != nil {
		p.updateTicker.Stop()
		p.updateTicker = nil
	}
	if p.cleanupTicker != nil {
		p.cleanupTicker.Stop()
		p.cleanupTicker = nil
	}

	// Stop tracking all players
	for steamID := range p.trackedPlayers {
		p.untrackPlayerUnsafe(steamID)
	}

	if p.cancel != nil {
		p.cancel()
	}

	p.status = plugin_manager.PluginStatusStopped

	return nil
}

// HandleEvent processes an event if the plugin is subscribed to it
func (p *AutoKickUnassignedPlugin) HandleEvent(event *plugin_manager.PluginEvent) error {
	if event.Type != string(event_manager.EventTypeLogGameEventUnified) {
		return nil // Not interested in this event
	}

	return p.handleNewGame(event)
}

// GetStatus returns the current plugin status
func (p *AutoKickUnassignedPlugin) GetStatus() plugin_manager.PluginStatus {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.status
}

// GetConfig returns the current plugin configuration
func (p *AutoKickUnassignedPlugin) GetConfig() map[string]interface{} {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.config
}

// UpdateConfig updates the plugin configuration
func (p *AutoKickUnassignedPlugin) UpdateConfig(config map[string]interface{}) error {
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

	p.apis.LogAPI.Info("Auto Kick Unassigned plugin configuration updated", map[string]interface{}{
		"player_threshold":  config["player_threshold"],
		"unassigned_timer":  config["unassigned_timer"],
		"round_start_delay": config["round_start_delay"],
	})

	return nil
}

// handleNewGame processes new game events
func (p *AutoKickUnassignedPlugin) handleNewGame(rawEvent *plugin_manager.PluginEvent) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.betweenRounds = true

	// Stop tracking all players during round transition
	for steamID := range p.trackedPlayers {
		p.untrackPlayerUnsafe(steamID)
	}

	// Schedule end of grace period
	roundStartDelay := p.getIntConfig("round_start_delay")
	if roundStartDelay <= 0 {
		roundStartDelay = 900
	}

	p.apis.LogAPI.Info("New game detected - starting grace period", map[string]interface{}{
		"grace_period_seconds": roundStartDelay,
	})

	go func() {
		timer := time.NewTimer(time.Duration(roundStartDelay) * time.Second)
		defer timer.Stop()

		select {
		case <-timer.C:
			p.mu.Lock()
			p.betweenRounds = false
			p.mu.Unlock()
			p.apis.LogAPI.Info("Grace period ended - resuming auto-kick monitoring", nil)
		case <-p.ctx.Done():
			return // Plugin is stopping
		}
	}()

	return nil
}

// updateTrackingLoop handles the periodic tracking updates
func (p *AutoKickUnassignedPlugin) updateTrackingLoop() {
	for {
		select {
		case <-p.ctx.Done():
			return // Plugin is stopping
		case <-p.updateTicker.C:
			if err := p.updateTrackingList(); err != nil {
				p.apis.LogAPI.Error("Failed to update tracking list", err, nil)
			}
		}
	}
}

// cleanupLoop handles the periodic cleanup of disconnected players
func (p *AutoKickUnassignedPlugin) cleanupLoop() {
	for {
		select {
		case <-p.ctx.Done():
			return // Plugin is stopping
		case <-p.cleanupTicker.C:
			p.clearDisconnectedPlayers()
		}
	}
}

// updateTrackingList updates the list of tracked unassigned players
func (p *AutoKickUnassignedPlugin) updateTrackingList() error {
	p.mu.Lock()
	betweenRounds := p.betweenRounds
	playerThreshold := p.getIntConfig("player_threshold")
	p.mu.Unlock()

	// Get current players
	players, err := p.apis.ServerAPI.GetPlayers()
	if err != nil {
		return fmt.Errorf("failed to get players: %w", err)
	}

	// Count online players
	onlinePlayerCount := 0
	for _, player := range players {
		if player.IsOnline {
			onlinePlayerCount++
		}
	}

	shouldRun := !betweenRounds && (playerThreshold < 0 || onlinePlayerCount >= playerThreshold)

	p.apis.LogAPI.Debug("Update tracking list check", map[string]interface{}{
		"should_run":       shouldRun,
		"between_rounds":   betweenRounds,
		"online_players":   onlinePlayerCount,
		"player_threshold": playerThreshold,
	})

	if !shouldRun {
		// Stop tracking all players if conditions aren't met
		p.mu.Lock()
		for steamID := range p.trackedPlayers {
			p.untrackPlayerUnsafe(steamID)
		}
		p.mu.Unlock()
		return nil
	}

	// Get admin list for checking permissions
	admins, err := p.apis.ServerAPI.GetAdmins()
	if err != nil {
		p.apis.LogAPI.Warn("Failed to get admin list", map[string]interface{}{
			"error": err.Error(),
		})
		admins = []*plugin_manager.AdminInfo{}
	}

	// Create admin lookup map
	adminMap := make(map[string]bool)
	for _, admin := range admins {
		adminMap[admin.SteamID] = true
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	ignoreAdmins := p.getBoolConfig("ignore_admins")

	// Process each online player
	for _, player := range players {
		if !player.IsOnline {
			continue
		}

		isTracked := p.trackedPlayers[player.SteamID] != nil
		isUnassigned := player.SquadID <= 0 // Squad ID 0 or negative means unassigned
		isAdmin := adminMap[player.SteamID]

		// If player joined a squad, stop tracking them
		if !isUnassigned && isTracked {
			p.untrackPlayerUnsafe(player.SteamID)
			continue
		}

		// Skip if player is in a squad
		if !isUnassigned {
			continue
		}

		// Check admin exemption
		if isAdmin && ignoreAdmins {
			p.apis.LogAPI.Debug("Skipping admin player", map[string]interface{}{
				"player": player.Name,
			})
			continue
		}

		// Start tracking unassigned player
		if !isTracked {
			p.trackPlayerUnsafe(player)
		}
	}

	return nil
}

// clearDisconnectedPlayers removes disconnected players from tracking
func (p *AutoKickUnassignedPlugin) clearDisconnectedPlayers() {
	players, err := p.apis.ServerAPI.GetPlayers()
	if err != nil {
		p.apis.LogAPI.Error("Failed to get players for cleanup", err, nil)
		return
	}

	// Create lookup map of online players
	onlineMap := make(map[string]bool)
	for _, player := range players {
		if player.IsOnline {
			onlineMap[player.SteamID] = true
		}
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	// Remove tracking for players who are no longer online
	for steamID := range p.trackedPlayers {
		if !onlineMap[steamID] {
			p.untrackPlayerUnsafe(steamID)
		}
	}
}

// trackPlayerUnsafe starts tracking a player (must be called with mutex held)
func (p *AutoKickUnassignedPlugin) trackPlayerUnsafe(player *plugin_manager.PlayerInfo) {
	if p.trackedPlayers[player.SteamID] != nil {
		return // Already tracking
	}

	p.apis.LogAPI.Debug("Starting to track unassigned player", map[string]interface{}{
		"player":   player.Name,
		"steam_id": player.SteamID,
		"squad_id": player.SquadID,
	})

	kickCtx, kickCancel := context.WithCancel(p.ctx)

	tracker := &PlayerTracker{
		Player:     player,
		Warnings:   0,
		StartTime:  time.Now(),
		KickCtx:    kickCtx,
		KickCancel: kickCancel,
	}

	// Start warning ticker
	warningInterval := p.getIntConfig("frequency_of_warnings")
	if warningInterval <= 0 {
		warningInterval = 30
	}
	tracker.WarnTicker = time.NewTicker(time.Duration(warningInterval) * time.Second)

	// Start kick timer
	kickTimeout := p.getIntConfig("unassigned_timer")
	if kickTimeout <= 0 {
		kickTimeout = 360
	}
	tracker.KickTimer = time.NewTimer(time.Duration(kickTimeout) * time.Second)

	p.trackedPlayers[player.SteamID] = tracker

	// Start warning and kick goroutines
	go p.warningLoop(tracker)
	go p.kickLoop(tracker)
}

// untrackPlayerUnsafe stops tracking a player (must be called with mutex held)
func (p *AutoKickUnassignedPlugin) untrackPlayerUnsafe(steamID string) {
	tracker := p.trackedPlayers[steamID]
	if tracker == nil {
		return
	}

	p.apis.LogAPI.Debug("Stopping tracking of player", map[string]interface{}{
		"player":   tracker.Player.Name,
		"steam_id": steamID,
	})

	// Stop timers and cancel context
	if tracker.WarnTicker != nil {
		tracker.WarnTicker.Stop()
	}
	if tracker.KickTimer != nil {
		tracker.KickTimer.Stop()
	}
	if tracker.KickCancel != nil {
		tracker.KickCancel()
	}

	delete(p.trackedPlayers, steamID)
}

// warningLoop handles sending warnings to a tracked player
func (p *AutoKickUnassignedPlugin) warningLoop(tracker *PlayerTracker) {
	warningInterval := p.getIntConfig("frequency_of_warnings")
	kickTimeout := p.getIntConfig("unassigned_timer")
	warningMessage := p.getStringConfig("warning_message")

	for {
		select {
		case <-tracker.KickCtx.Done():
			return
		case <-tracker.WarnTicker.C:
			p.mu.Lock()
			tracker.Warnings++
			timeElapsed := time.Since(tracker.StartTime)
			timeLeft := time.Duration(kickTimeout)*time.Second - timeElapsed

			// Stop warning if kick is imminent
			if timeLeft <= time.Duration(warningInterval)*time.Second {
				tracker.WarnTicker.Stop()
			}
			p.mu.Unlock()

			// Format time left
			timeLeftFormatted := p.formatDuration(timeLeft)
			message := fmt.Sprintf("%s - %s", warningMessage, timeLeftFormatted)

			if err := p.apis.RconAPI.SendWarningToPlayer(tracker.Player.SteamID, message); err != nil {
				p.apis.LogAPI.Error("Failed to send warning to player", err, map[string]interface{}{
					"player":   tracker.Player.Name,
					"steam_id": tracker.Player.SteamID,
				})
			} else {
				p.apis.LogAPI.Debug("Warned unassigned player", map[string]interface{}{
					"player":    tracker.Player.Name,
					"warnings":  tracker.Warnings,
					"time_left": timeLeftFormatted,
				})
			}
		}
	}
}

// kickLoop handles kicking a tracked player after timeout
func (p *AutoKickUnassignedPlugin) kickLoop(tracker *PlayerTracker) {
	select {
	case <-tracker.KickCtx.Done():
		return
	case <-tracker.KickTimer.C:
		// Double-check player is still unassigned before kicking
		if err := p.updateTrackingList(); err != nil {
			p.apis.LogAPI.Error("Failed to update tracking list before kick", err, nil)
		}

		p.mu.Lock()
		stillTracked := p.trackedPlayers[tracker.Player.SteamID] != nil
		kickMessage := p.getStringConfig("kick_message")
		p.mu.Unlock()

		if !stillTracked {
			return // Player was removed from tracking (joined squad or left)
		}

		// Kick the player
		if err := p.apis.RconAPI.KickPlayer(tracker.Player.SteamID, kickMessage); err != nil {
			p.apis.LogAPI.Error("Failed to kick unassigned player", err, map[string]interface{}{
				"player":   tracker.Player.Name,
				"steam_id": tracker.Player.SteamID,
			})
		} else {
			p.apis.LogAPI.Info("Kicked unassigned player", map[string]interface{}{
				"player":   tracker.Player.Name,
				"steam_id": tracker.Player.SteamID,
				"warnings": tracker.Warnings,
				"duration": time.Since(tracker.StartTime),
			})
		}

		// Remove from tracking
		p.mu.Lock()
		p.untrackPlayerUnsafe(tracker.Player.SteamID)
		p.mu.Unlock()
	}
}

// formatDuration formats a duration into MM:SS format
func (p *AutoKickUnassignedPlugin) formatDuration(d time.Duration) string {
	if d < 0 {
		d = 0
	}

	minutes := int(d.Minutes())
	seconds := int(d.Seconds()) % 60

	return fmt.Sprintf("%02d:%02d", minutes, seconds)
}

// Helper methods for config access

func (p *AutoKickUnassignedPlugin) getStringConfig(key string) string {
	if value, ok := p.config[key].(string); ok {
		return value
	}
	return ""
}

func (p *AutoKickUnassignedPlugin) getIntConfig(key string) int {
	if value, ok := p.config[key].(int); ok {
		return value
	}
	if value, ok := p.config[key].(float64); ok {
		return int(value)
	}
	return 0
}

func (p *AutoKickUnassignedPlugin) getBoolConfig(key string) bool {
	if value, ok := p.config[key].(bool); ok {
		return value
	}
	return false
}
