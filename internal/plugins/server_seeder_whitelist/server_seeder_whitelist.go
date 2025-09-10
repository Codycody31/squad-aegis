// Based on https://github.com/mikebjoyce/squadjs-squadlead-whitelister/tree/main/squad-leader-whitelist.js

package server_seeder_whitelist

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"sync"
	"time"

	"go.codycody31.dev/squad-aegis/internal/event_manager"
	"go.codycody31.dev/squad-aegis/internal/plugin_manager"
	"go.codycody31.dev/squad-aegis/internal/shared/plug_config_schema"
)

// ServerSeederWhitelistPlugin manages a progressive whitelist for players who help seed the server
type ServerSeederWhitelistPlugin struct {
	// Plugin configuration
	config map[string]interface{}
	apis   *plugin_manager.PluginAPIs

	// State management
	mu                sync.Mutex
	status            plugin_manager.PluginStatus
	ctx               context.Context
	cancel            context.CancelFunc
	progressTicker    *time.Ticker
	decayTicker       *time.Ticker
	adminSyncTicker   *time.Ticker
	stopProgressRound bool
	shuttingDown      bool

	// Player tracking
	playerProgress map[string]*PlayerProgressRecord
}

// PlayerProgressRecord tracks a player's seeding progress
type PlayerProgressRecord struct {
	SteamID        string    `json:"steam_id"`
	Progress       float64   `json:"progress"`
	LastProgressed time.Time `json:"last_progressed"`
	TotalSeeded    float64   `json:"total_seeded"`
	LastSeen       time.Time `json:"last_seen"`
}

// Define returns the plugin definition
func Define() plugin_manager.PluginDefinition {
	return plugin_manager.PluginDefinition{
		ID:                     "server_seeder_whitelist",
		Name:                   "Server Seeder Whitelist",
		Description:            "Tracks players who help seed the server and progressively adds them to a whitelist based on time spent seeding. Players earn progress when server is below seeding threshold and lose progress over time when inactive.",
		Version:                "1.0.0",
		Author:                 "Squad Aegis",
		AllowMultipleInstances: false,
		RequiredConnectors:     []string{},
		LongRunning:            true,

		ConfigSchema: plug_config_schema.ConfigSchema{
			Fields: []plug_config_schema.ConfigField{
				{
					Name:        "seeding_threshold",
					Description: "Player count below which server is considered in seeding mode.",
					Required:    false,
					Type:        plug_config_schema.FieldTypeInt,
					Default:     50,
				},
				{
					Name:        "hours_to_whitelist",
					Description: "Hours of seeding required to reach 100% whitelist status.",
					Required:    false,
					Type:        plug_config_schema.FieldTypeInt,
					Default:     6,
				},
				{
					Name:        "whitelist_duration_days",
					Description: "How many days whitelist status lasts before expiring.",
					Required:    false,
					Type:        plug_config_schema.FieldTypeInt,
					Default:     7,
				},
				{
					Name:        "decay_after_hours",
					Description: "Hours after last seeding before progress starts to decay.",
					Required:    false,
					Type:        plug_config_schema.FieldTypeInt,
					Default:     48,
				},
				{
					Name:        "min_players_for_decay",
					Description: "Minimum players on server for decay to be active.",
					Required:    false,
					Type:        plug_config_schema.FieldTypeInt,
					Default:     60,
				},
				{
					Name:        "min_players_for_seeding",
					Description: "Minimum players on server before seeding progress can be awarded.",
					Required:    false,
					Type:        plug_config_schema.FieldTypeInt,
					Default:     10,
				},
				{
					Name:        "progress_interval_seconds",
					Description: "How often to check for progress updates in seconds.",
					Required:    false,
					Type:        plug_config_schema.FieldTypeInt,
					Default:     60,
				},
				{
					Name:        "decay_interval_seconds",
					Description: "How often to apply decay in seconds.",
					Required:    false,
					Type:        plug_config_schema.FieldTypeInt,
					Default:     3600, // 1 hour
				},
				{
					Name:        "whitelist_group_name",
					Description: "Admin group name for whitelisted players.",
					Required:    false,
					Type:        plug_config_schema.FieldTypeString,
					Default:     "seeder_whitelist",
				},
				{
					Name:        "wait_on_new_games",
					Description: "Should progress tracking pause after new game events.",
					Required:    false,
					Type:        plug_config_schema.FieldTypeBool,
					Default:     true,
				},
				{
					Name:        "wait_time_on_new_game",
					Description: "Time to wait after new game before resuming progress tracking in seconds.",
					Required:    false,
					Type:        plug_config_schema.FieldTypeInt,
					Default:     120,
				},
				{
					Name:        "enable_chat_command",
					Description: "Enable the !wl chat command for players to check their progress.",
					Required:    false,
					Type:        plug_config_schema.FieldTypeBool,
					Default:     true,
				},
				{
					Name:        "progress_notification_thresholds",
					Description: "Progress percentage thresholds to notify players (e.g., 25, 50, 75).",
					Required:    false,
					Type:        plug_config_schema.FieldTypeArrayInt,
					Default:     []interface{}{25, 50, 75, 100},
				},
				{
					Name:        "auto_add_temporary_admins",
					Description: "Automatically add whitelisted players as temporary admins when they join the server.",
					Required:    false,
					Type:        plug_config_schema.FieldTypeBool,
					Default:     true,
				},
				{
					Name:        "admin_sync_interval_minutes",
					Description: "How often to sync temporary admin status with current whitelist in minutes.",
					Required:    false,
					Type:        plug_config_schema.FieldTypeInt,
					Default:     15,
				},
				{
					Name:        "admin_renewal_hours_before_expiry",
					Description: "How many hours before expiration to renew admin roles for qualifying players.",
					Required:    false,
					Type:        plug_config_schema.FieldTypeInt,
					Default:     24,
				},
			},
		},

		Events: []event_manager.EventType{
			event_manager.EventTypeLogNewGame,
			event_manager.EventTypeRconChatMessage,
			event_manager.EventTypeLogPlayerConnected,
		},

		CreateInstance: func() plugin_manager.Plugin {
			return &ServerSeederWhitelistPlugin{
				playerProgress: make(map[string]*PlayerProgressRecord),
			}
		},
	}
}

// GetDefinition returns the plugin definition
func (p *ServerSeederWhitelistPlugin) GetDefinition() plugin_manager.PluginDefinition {
	return Define()
}

// Initialize initializes the plugin with its configuration and dependencies
func (p *ServerSeederWhitelistPlugin) Initialize(config map[string]interface{}, apis *plugin_manager.PluginAPIs) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.config = config
	p.apis = apis
	p.status = plugin_manager.PluginStatusStopped
	p.stopProgressRound = false
	p.playerProgress = make(map[string]*PlayerProgressRecord)

	// Validate config
	definition := p.GetDefinition()
	if err := definition.ConfigSchema.Validate(config); err != nil {
		return fmt.Errorf("invalid config: %w", err)
	}

	// Fill defaults
	definition.ConfigSchema.FillDefaults(config)

	// Load existing player progress from database
	if err := p.loadPlayerProgress(); err != nil {
		p.apis.LogAPI.Error("Failed to load player progress from database", err, nil)
		// Don't fail initialization, just start with empty progress
	}

	p.status = plugin_manager.PluginStatusStopped

	return nil
}

// Start begins plugin execution
func (p *ServerSeederWhitelistPlugin) Start(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.status == plugin_manager.PluginStatusRunning {
		return nil // Already running
	}

	p.ctx, p.cancel = context.WithCancel(ctx)
	p.status = plugin_manager.PluginStatusRunning

	// Start progress tracking
	progressInterval := time.Duration(p.getIntConfig("progress_interval_seconds")) * time.Second
	p.progressTicker = time.NewTicker(progressInterval)
	go p.progressTrackingLoop()

	// Start decay
	decayInterval := time.Duration(p.getIntConfig("decay_interval_seconds")) * time.Second
	p.decayTicker = time.NewTicker(decayInterval)
	go p.decayLoop()

	// Start admin sync if enabled
	if p.getBoolConfig("auto_add_temporary_admins") {
		adminSyncInterval := time.Duration(p.getIntConfig("admin_sync_interval_minutes")) * time.Minute
		p.adminSyncTicker = time.NewTicker(adminSyncInterval)
		go p.adminSyncLoop()
	}

	return nil
}

// Stop gracefully stops the plugin
func (p *ServerSeederWhitelistPlugin) Stop() error {
	// Check status first without holding mutex for too long
	p.mu.Lock()
	if p.status == plugin_manager.PluginStatusStopped {
		p.mu.Unlock()
		return nil
	}
	p.status = plugin_manager.PluginStatusStopping
	p.shuttingDown = true // Set shutdown flag to prevent RCON calls
	p.mu.Unlock()

	// Cancel context first to signal goroutines to stop
	if p.cancel != nil {
		p.cancel()
	}

	// Stop tickers (this will stop the goroutines from getting new triggers)
	if p.progressTicker != nil {
		p.progressTicker.Stop()
	}
	if p.decayTicker != nil {
		p.decayTicker.Stop()
	}
	if p.adminSyncTicker != nil {
		p.adminSyncTicker.Stop()
	}

	// Give goroutines a moment to finish their current iteration
	// Use a timeout to prevent infinite hanging
	done := make(chan bool, 1)
	go func() {
		time.Sleep(500 * time.Millisecond) // Wait for goroutines to finish
		done <- true
	}()

	select {
	case <-done:
		// Normal completion
	case <-time.After(2 * time.Second):
		// Timeout - force shutdown
		p.apis.LogAPI.Warn("Plugin shutdown timed out, forcing stop", nil)
	}

	// Now safely acquire mutex for final cleanup
	p.mu.Lock()
	defer p.mu.Unlock()

	// Clear ticker references
	p.progressTicker = nil
	p.decayTicker = nil
	p.adminSyncTicker = nil

	// Save final state to database
	if err := p.savePlayerProgress(); err != nil {
		p.apis.LogAPI.Error("Failed to save player progress to database on shutdown", err, nil)
	}

	p.status = plugin_manager.PluginStatusStopped

	return nil
}

// HandleEvent processes events
func (p *ServerSeederWhitelistPlugin) HandleEvent(event *plugin_manager.PluginEvent) error {
	switch event.Type {
	case "LOG_NEW_GAME":
		return p.handleNewGame(event)
	case "RCON_CHAT_MESSAGE":
		return p.handleChatMessage(event)
	case "LOG_PLAYER_CONNECTED":
		return p.handlePlayerConnected(event)
	}
	return nil
}

// GetStatus returns the current plugin status
func (p *ServerSeederWhitelistPlugin) GetStatus() plugin_manager.PluginStatus {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.status
}

// GetConfig returns the current plugin configuration
func (p *ServerSeederWhitelistPlugin) GetConfig() map[string]interface{} {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.config
}

// UpdateConfig updates the plugin configuration
func (p *ServerSeederWhitelistPlugin) UpdateConfig(config map[string]interface{}) error {
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

	// If running, restart tickers with new intervals
	if p.status == plugin_manager.PluginStatusRunning {
		// Restart progress ticker
		if p.progressTicker != nil {
			p.progressTicker.Stop()
			progressInterval := time.Duration(p.getIntConfig("progress_interval_seconds")) * time.Second
			p.progressTicker = time.NewTicker(progressInterval)
		}

		// Restart decay ticker
		if p.decayTicker != nil {
			p.decayTicker.Stop()
			decayInterval := time.Duration(p.getIntConfig("decay_interval_seconds")) * time.Second
			p.decayTicker = time.NewTicker(decayInterval)
		}

		// Restart admin sync ticker if enabled
		if p.getBoolConfig("auto_add_temporary_admins") {
			if p.adminSyncTicker != nil {
				p.adminSyncTicker.Stop()
			}
			adminSyncInterval := time.Duration(p.getIntConfig("admin_sync_interval_minutes")) * time.Minute
			p.adminSyncTicker = time.NewTicker(adminSyncInterval)
			if p.adminSyncTicker == nil {
				go p.adminSyncLoop()
			}
		} else {
			// Stop admin sync if disabled
			if p.adminSyncTicker != nil {
				p.adminSyncTicker.Stop()
				p.adminSyncTicker = nil
			}
		}
	}

	p.apis.LogAPI.Info("Server Seeder Whitelist plugin configuration updated", map[string]interface{}{
		"seeding_threshold":       config["seeding_threshold"],
		"hours_to_whitelist":      config["hours_to_whitelist"],
		"whitelist_duration_days": config["whitelist_duration_days"],
	})

	return nil
}

// handleNewGame processes new game events
func (p *ServerSeederWhitelistPlugin) handleNewGame(event *plugin_manager.PluginEvent) error {
	if !p.getBoolConfig("wait_on_new_games") {
		return nil
	}

	// Temporarily pause progress tracking
	p.mu.Lock()
	p.stopProgressRound = true
	p.mu.Unlock()

	waitTime := p.getIntConfig("wait_time_on_new_game")
	if waitTime <= 0 {
		waitTime = 120
	}

	p.apis.LogAPI.Info("New game detected - temporarily pausing seeder progress tracking", map[string]interface{}{
		"wait_time_seconds": waitTime,
	})

	// Resume progress tracking after wait time
	go func() {
		timer := time.NewTimer(time.Duration(waitTime) * time.Second)
		defer timer.Stop()

		select {
		case <-timer.C:
			p.mu.Lock()
			p.stopProgressRound = false
			p.mu.Unlock()
			p.apis.LogAPI.Info("Resuming seeder progress tracking after new game wait period", nil)
		case <-p.ctx.Done():
			return
		}
	}()

	return nil
}

// handleChatMessage processes chat messages for the progress command
func (p *ServerSeederWhitelistPlugin) handleChatMessage(rawEvent *plugin_manager.PluginEvent) error {
	if !p.getBoolConfig("enable_chat_command") {
		return nil
	}

	event, ok := rawEvent.Data.(*event_manager.RconChatMessageData)
	if !ok {
		return fmt.Errorf("invalid event data type")
	}

	if event.Message != "!wl" && event.Message != "!progress" {
		return nil
	}

	return p.sendProgressToPlayer(event.SteamID)
}

// handlePlayerConnected tracks when players connect for statistics
func (p *ServerSeederWhitelistPlugin) handlePlayerConnected(event *plugin_manager.PluginEvent) error {
	// Extract player info from event data
	eventData, ok := event.Data.(map[string]interface{})
	if !ok {
		return nil
	}

	steamID, ok := eventData["steam_id"].(string)
	if !ok || steamID == "" {
		return nil
	}

	// Update last seen time for the player
	p.mu.Lock()
	if record, exists := p.playerProgress[steamID]; exists {
		record.LastSeen = time.Now()
	}
	p.mu.Unlock()

	return nil
}

// progressTrackingLoop handles the periodic progress tracking
func (p *ServerSeederWhitelistPlugin) progressTrackingLoop() {
	for {
		select {
		case <-p.ctx.Done():
			return
		case <-p.progressTicker.C:
			if err := p.trackProgress(); err != nil {
				p.apis.LogAPI.Error("Failed to track seeder progress", err, nil)
			}
		}
	}
}

// decayLoop handles the periodic progress decay
func (p *ServerSeederWhitelistPlugin) decayLoop() {
	for {
		select {
		case <-p.ctx.Done():
			return
		case <-p.decayTicker.C:
			if err := p.decayProgress(); err != nil {
				p.apis.LogAPI.Error("Failed to decay seeder progress", err, nil)
			}
		}
	}
}

// adminSyncLoop handles periodic admin synchronization
func (p *ServerSeederWhitelistPlugin) adminSyncLoop() {
	for {
		select {
		case <-p.ctx.Done():
			return
		case <-p.adminSyncTicker.C:
			if err := p.syncTemporaryAdmins(); err != nil {
				p.apis.LogAPI.Error("Failed to sync temporary admins", err, nil)
			}
		}
	}
}

// trackProgress awards progress to players during seeding
func (p *ServerSeederWhitelistPlugin) trackProgress() error {
	p.mu.Lock()
	stopProgress := p.stopProgressRound
	p.mu.Unlock()

	if stopProgress {
		return nil // Currently in wait period after new game
	}

	// Get current players
	players, err := p.apis.ServerAPI.GetPlayers()
	if err != nil {
		return fmt.Errorf("failed to get players: %w", err)
	}

	// Count online players
	onlinePlayerCount := 0
	onlinePlayers := make([]*plugin_manager.PlayerInfo, 0)
	for _, player := range players {
		if player.IsOnline {
			onlinePlayerCount++
			onlinePlayers = append(onlinePlayers, player)
		}
	}

	seedingThreshold := p.getIntConfig("seeding_threshold")
	if onlinePlayerCount >= seedingThreshold {
		return nil // Not in seeding mode
	}

	minPlayersForSeeding := p.getIntConfig("min_players_for_seeding")
	if onlinePlayerCount < minPlayersForSeeding {
		return nil // Not enough players to start awarding seeding progress
	}

	// Calculate progress increment based on interval
	// Progress is calculated as: (hours spent seeding / hours needed for whitelist) * 100
	hoursToWhitelist := float64(p.getIntConfig("hours_to_whitelist"))
	intervalSeconds := float64(p.getIntConfig("progress_interval_seconds"))
	progressIncrement := (intervalSeconds / 3600.0) / hoursToWhitelist * 100.0

	whitelistThreshold := 100.0 // Fixed at 100%
	notificationThresholds := p.getIntArrayConfig("progress_notification_thresholds")

	now := time.Now()
	var updatedPlayers []string

	p.mu.Lock()
	for _, player := range onlinePlayers {
		steamID := player.SteamID
		if steamID == "" {
			continue
		}

		record, exists := p.playerProgress[steamID]
		if !exists {
			record = &PlayerProgressRecord{
				SteamID:        steamID,
				Progress:       0,
				LastProgressed: now,
				TotalSeeded:    0,
				LastSeen:       now,
			}
			p.playerProgress[steamID] = record
		}

		oldProgress := record.Progress
		newProgress := record.Progress + progressIncrement
		record.Progress = newProgress
		record.LastProgressed = now
		record.TotalSeeded += progressIncrement
		record.LastSeen = now

		updatedPlayers = append(updatedPlayers, steamID)

		// Check for notification thresholds and whitelist status changes
		oldPercentage := (oldProgress / whitelistThreshold) * 100
		newPercentage := (newProgress / whitelistThreshold) * 100

		// Check if player just reached whitelist threshold
		if oldProgress < whitelistThreshold && newProgress >= whitelistThreshold {
			// Player just became whitelisted - add them as admin (skip if shutting down)
			if !p.shuttingDown {
				go p.addPlayerAsTemporaryAdmin(steamID, player.Name)
			}
		}

		for _, threshold := range notificationThresholds {
			thresholdFloat := float64(threshold)
			if oldPercentage < thresholdFloat && newPercentage >= thresholdFloat {
				// Player crossed a notification threshold
				go p.sendProgressNotification(steamID, player.Name, newPercentage, newProgress >= whitelistThreshold)
				break
			}
		}
	}
	p.mu.Unlock()

	if len(updatedPlayers) > 0 {
		// Save progress to database periodically
		if err := p.savePlayerProgress(); err != nil {
			p.apis.LogAPI.Error("Failed to save player progress", err, nil)
		}

		// Only log occasionally to reduce log spam
		if len(updatedPlayers) >= 5 || onlinePlayerCount <= 10 {
			p.apis.LogAPI.Debug("Awarded seeder progress", map[string]interface{}{
				"player_count":       onlinePlayerCount,
				"seeding_threshold":  seedingThreshold,
				"progress_increment": progressIncrement,
				"updated_players":    len(updatedPlayers),
			})
		}
	}

	return nil
}

// decayProgress applies progress decay to inactive players
func (p *ServerSeederWhitelistPlugin) decayProgress() error {
	// Get current players to check server population
	players, err := p.apis.ServerAPI.GetPlayers()
	if err != nil {
		return fmt.Errorf("failed to get players: %w", err)
	}

	onlinePlayerCount := 0
	for _, player := range players {
		if player.IsOnline {
			onlinePlayerCount++
		}
	}

	minPlayersForDecay := p.getIntConfig("min_players_for_decay")
	if onlinePlayerCount < minPlayersForDecay {
		return nil // Server population too low for decay
	}

	// Calculate decay increment based on interval
	// Decay is calculated to lose whitelist after the same amount of time it took to earn it
	hoursToWhitelist := float64(p.getIntConfig("hours_to_whitelist"))
	decayAfterHours := float64(p.getIntConfig("decay_after_hours"))
	intervalSeconds := float64(p.getIntConfig("decay_interval_seconds"))

	// Decay rate: lose 100% progress over the same time it took to earn it
	decayIncrement := (intervalSeconds / 3600.0) / hoursToWhitelist * 100.0

	now := time.Now()
	decayThreshold := time.Duration(decayAfterHours * float64(time.Hour))
	var decayedPlayers int

	p.mu.Lock()
	for _, record := range p.playerProgress {
		timeSinceProgress := now.Sub(record.LastProgressed)
		if timeSinceProgress > decayThreshold {
			if record.Progress > 0 {
				oldProgress := record.Progress
				record.Progress = max(0, record.Progress-decayIncrement)
				decayedPlayers++

				// Check if player lost whitelist status due to decay
				whitelistThreshold := 100.0 // Fixed at 100%
				if oldProgress >= whitelistThreshold && record.Progress < whitelistThreshold {
					// Player lost whitelist status - remove admin privileges (skip if shutting down)
					if !p.shuttingDown {
						go p.removePlayerAsTemporaryAdmin(record.SteamID, "")
					}
				}
			}
		}
	}
	p.mu.Unlock()

	if decayedPlayers > 0 {
		// Save progress after decay
		if err := p.savePlayerProgress(); err != nil {
			p.apis.LogAPI.Error("Failed to save player progress after decay", err, nil)
		}

		// Only log decay when significant numbers are affected
		if decayedPlayers >= 3 {
			p.apis.LogAPI.Debug("Applied seeder progress decay", map[string]interface{}{
				"decayed_players":   decayedPlayers,
				"decay_increment":   decayIncrement,
				"server_population": onlinePlayerCount,
			})
		}
	}

	return nil
}

// sendProgressToPlayer sends progress information to a specific player
func (p *ServerSeederWhitelistPlugin) sendProgressToPlayer(steamID string) error {
	p.mu.Lock()
	record, exists := p.playerProgress[steamID]
	p.mu.Unlock()

	whitelistThreshold := 100.0 // Fixed at 100%

	var message string
	if !exists || record.Progress == 0 {
		message = "No seeding progress found.\n" +
			"Join during low population to earn progress!"
	} else {
		percentage := record.Progress
		if percentage > 100 {
			percentage = 100
		}

		var status string
		if record.Progress >= whitelistThreshold {
			// Get rank among whitelisted players
			rank := p.getPlayerRank(steamID)
			status = fmt.Sprintf("✓ WHITELISTED (Rank #%d)", rank)
		} else {
			status = fmt.Sprintf("Progress: %.1f%%", percentage)
		}

		hoursToWhitelist := float64(p.getIntConfig("hours_to_whitelist"))
		totalHours := record.TotalSeeded / 100.0 * hoursToWhitelist
		message = status + "\n" +
			fmt.Sprintf("Total Seeded: %.1f hours", totalHours)
	}

	if err := p.apis.RconAPI.SendWarningToPlayer(steamID, message); err != nil {
		return fmt.Errorf("failed to send progress message: %w", err)
	}

	// Removed debug log to reduce log spam - only log on error

	return nil
}

// sendProgressNotification sends a notification when a player crosses a threshold
func (p *ServerSeederWhitelistPlugin) sendProgressNotification(steamID, playerName string, percentage float64, isWhitelisted bool) {
	var message string
	if isWhitelisted {
		message = "🎉 CONGRATULATIONS! 🎉\n" +
			"You are now WHITELISTED!\n" +
			"Thank you for helping seed the server!"
	} else {
		message = "🎉 SEEDER PROGRESS 🎉\n" +
			fmt.Sprintf("Progress Update: %.0f%%", percentage) + "\n" +
			"Keep seeding to earn whitelist!"
	}

	if err := p.apis.RconAPI.SendWarningToPlayer(steamID, message); err != nil {
		p.apis.LogAPI.Error("Failed to send progress notification", err, map[string]interface{}{
			"steam_id":    steamID,
			"player_name": playerName,
		})
	}
}

// getPlayerRank returns the rank of a player among whitelisted players
func (p *ServerSeederWhitelistPlugin) getPlayerRank(steamID string) int {
	whitelistThreshold := 100.0 // Fixed at 100%

	type playerRank struct {
		steamID  string
		progress float64
	}

	var whitelistedPlayers []playerRank
	p.mu.Lock()
	for _, record := range p.playerProgress {
		if record.Progress >= whitelistThreshold {
			whitelistedPlayers = append(whitelistedPlayers, playerRank{
				steamID:  record.SteamID,
				progress: record.Progress,
			})
		}
	}
	p.mu.Unlock()

	// Sort by progress descending
	sort.Slice(whitelistedPlayers, func(i, j int) bool {
		return whitelistedPlayers[i].progress > whitelistedPlayers[j].progress
	})

	// Find rank
	for i, player := range whitelistedPlayers {
		if player.steamID == steamID {
			return i + 1
		}
	}

	return 1 // Default rank if not found
}

// loadPlayerProgress loads player progress from database
func (p *ServerSeederWhitelistPlugin) loadPlayerProgress() error {
	data, err := p.apis.DatabaseAPI.GetPluginData("player_progress")
	if err != nil {
		// No data found is okay, start fresh
		return nil
	}

	var progress map[string]*PlayerProgressRecord
	if err := json.Unmarshal([]byte(data), &progress); err != nil {
		return fmt.Errorf("failed to unmarshal player progress: %w", err)
	}

	p.playerProgress = progress
	return nil
}

// savePlayerProgress saves player progress to database
func (p *ServerSeederWhitelistPlugin) savePlayerProgress() error {
	p.mu.Lock()
	data, err := json.Marshal(p.playerProgress)
	p.mu.Unlock()

	if err != nil {
		return fmt.Errorf("failed to marshal player progress: %w", err)
	}

	return p.apis.DatabaseAPI.SetPluginData("player_progress", string(data))
}

// Helper methods for config access

func (p *ServerSeederWhitelistPlugin) getStringConfig(key string) string {
	if value, ok := p.config[key].(string); ok {
		return value
	}
	return ""
}

func (p *ServerSeederWhitelistPlugin) getIntConfig(key string) int {
	if value, ok := p.config[key].(int); ok {
		return value
	}
	if value, ok := p.config[key].(float64); ok {
		return int(value)
	}
	return 0
}

func (p *ServerSeederWhitelistPlugin) getBoolConfig(key string) bool {
	if value, ok := p.config[key].(bool); ok {
		return value
	}
	return false
}

func (p *ServerSeederWhitelistPlugin) getIntArrayConfig(key string) []int {
	if value, ok := p.config[key].([]interface{}); ok {
		result := make([]int, 0, len(value))
		for _, item := range value {
			if intVal, ok := item.(int); ok {
				result = append(result, intVal)
			} else if floatVal, ok := item.(float64); ok {
				result = append(result, int(floatVal))
			} else if strVal, ok := item.(string); ok {
				if intVal, err := strconv.Atoi(strVal); err == nil {
					result = append(result, intVal)
				}
			}
		}
		return result
	}
	return []int{}
}

// addPlayerAsTemporaryAdmin adds a player as a temporary admin via direct database operations
func (p *ServerSeederWhitelistPlugin) addPlayerAsTemporaryAdmin(steamID, playerName string) {
	groupName := p.getStringConfig("whitelist_group_name")
	if groupName == "" {
		groupName = "seeder_whitelist"
	}

	// Create admin notes indicating this is from the Seeder Whitelist plugin
	notes := fmt.Sprintf("Plugin: Seeder Whitelist - Automatically added for server seeding contributions. Player: %s", playerName)

	// Calculate expiration time based on configuration
	var expiresAt *time.Time
	whitelistDurationDays := p.getIntConfig("whitelist_duration_days")
	if whitelistDurationDays > 0 {
		expiration := time.Now().AddDate(0, 0, whitelistDurationDays)
		expiresAt = &expiration
	}

	// Add player as temporary admin with configured expiration
	if err := p.apis.AdminAPI.AddTemporaryAdmin(steamID, groupName, notes, expiresAt); err != nil {
		p.apis.LogAPI.Error("Failed to add player as temporary admin via AdminAPI", err, map[string]interface{}{
			"steam_id":    steamID,
			"player_name": playerName,
			"group_name":  groupName,
			"notes":       notes,
		})
		return
	}

	// Reload server admin configuration to apply changes immediately
	// Skip RCON call if shutting down to prevent hanging
	p.mu.Lock()
	isShuttingDown := p.shuttingDown
	p.mu.Unlock()

	if !isShuttingDown {
		if _, err := p.apis.RconAPI.SendCommand("AdminReloadServerConfig"); err != nil {
			p.apis.LogAPI.Error("Failed to reload server admin config after adding admin", err, map[string]interface{}{
				"steam_id": steamID,
			})
		}
	}

	p.apis.LogAPI.Info("Added player as temporary admin", map[string]interface{}{
		"steam_id":    steamID,
		"player_name": playerName,
		"group_name":  groupName,
		"notes":       notes,
	})

	// Also broadcast a server-wide message about the new seeder whitelist member
	broadcastMessage := fmt.Sprintf("🌱 %s has earned seeder whitelist status! Thank you for helping populate the server!", playerName)
	if err := p.apis.RconAPI.Broadcast(broadcastMessage); err != nil {
		p.apis.LogAPI.Error("Failed to broadcast seeder whitelist achievement", err, map[string]interface{}{
			"steam_id":    steamID,
			"player_name": playerName,
		})
	}
}

// removePlayerAsTemporaryAdmin removes a player as a temporary admin via direct database operations
func (p *ServerSeederWhitelistPlugin) removePlayerAsTemporaryAdmin(steamID, playerName string) {
	// Create admin notes indicating this is from the Seeder Whitelist plugin
	notes := fmt.Sprintf("Plugin: Seeder Whitelist - Automatically removed (no longer qualifies). Player: %s", playerName)

	// Remove player as temporary admin
	if err := p.apis.AdminAPI.RemoveTemporaryAdmin(steamID, notes); err != nil {
		p.apis.LogAPI.Error("Failed to remove player as temporary admin via AdminAPI", err, map[string]interface{}{
			"steam_id":    steamID,
			"player_name": playerName,
			"notes":       notes,
		})
		return
	}

	// Reload server admin configuration to apply changes immediately
	// Skip RCON call if shutting down to prevent hanging
	p.mu.Lock()
	isShuttingDown := p.shuttingDown
	p.mu.Unlock()

	if !isShuttingDown {
		if _, err := p.apis.RconAPI.SendCommand("AdminReloadServerConfig"); err != nil {
			p.apis.LogAPI.Error("Failed to reload server admin config after removing admin", err, map[string]interface{}{
				"steam_id": steamID,
			})
		}
	}

	p.apis.LogAPI.Info("Removed player as temporary admin", map[string]interface{}{
		"steam_id":    steamID,
		"player_name": playerName,
		"notes":       notes,
	})
}

// renewPlayerAdminRole removes and re-adds a player's admin role to extend expiration
func (p *ServerSeederWhitelistPlugin) renewPlayerAdminRole(steamID, playerName string) {
	groupName := p.getStringConfig("whitelist_group_name")
	if groupName == "" {
		groupName = "seeder_whitelist"
	}

	// Create admin notes indicating this is a renewal from the Seeder Whitelist plugin
	notes := fmt.Sprintf("Plugin: Seeder Whitelist - Role renewed to extend expiration. Player: %s", playerName)

	// First remove the existing admin role
	if err := p.apis.AdminAPI.RemoveTemporaryAdmin(steamID, "Role renewal - removing old assignment"); err != nil {
		p.apis.LogAPI.Error("Failed to remove admin role for renewal", err, map[string]interface{}{
			"steam_id":    steamID,
			"player_name": playerName,
		})
		return
	}

	// Calculate new expiration time
	var expiresAt *time.Time
	whitelistDurationDays := p.getIntConfig("whitelist_duration_days")
	if whitelistDurationDays > 0 {
		expiration := time.Now().AddDate(0, 0, whitelistDurationDays)
		expiresAt = &expiration
	}

	// Add new admin role with fresh expiration
	if err := p.apis.AdminAPI.AddTemporaryAdmin(steamID, groupName, notes, expiresAt); err != nil {
		p.apis.LogAPI.Error("Failed to re-add admin role after renewal", err, map[string]interface{}{
			"steam_id":    steamID,
			"player_name": playerName,
			"group_name":  groupName,
			"notes":       notes,
		})
		return
	}

	// Reload server admin configuration to apply changes immediately
	// Skip RCON call if shutting down to prevent hanging
	p.mu.Lock()
	isShuttingDown := p.shuttingDown
	p.mu.Unlock()

	if !isShuttingDown {
		if _, err := p.apis.RconAPI.SendCommand("AdminReloadServerConfig"); err != nil {
			p.apis.LogAPI.Error("Failed to reload server admin config after renewing admin role", err, map[string]interface{}{
				"steam_id": steamID,
			})
		}
	}

	p.apis.LogAPI.Info("Renewed player admin role with extended expiration", map[string]interface{}{
		"steam_id":    steamID,
		"player_name": playerName,
		"group_name":  groupName,
		"notes":       notes,
	})
}

// syncTemporaryAdmins synchronizes temporary admin status with current whitelist
func (p *ServerSeederWhitelistPlugin) syncTemporaryAdmins() error {
	// This function ensures database consistency for all whitelisted players
	// and refreshes admin roles that are about to expire
	whitelistThreshold := 100.0 // Fixed at 100%
	whitelistDurationDays := p.getIntConfig("whitelist_duration_days")

	if whitelistDurationDays <= 0 {
		return nil // No expiration handling needed
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	// Check all players and manage their admin status
	for steamID, record := range p.playerProgress {
		if record.Progress >= whitelistThreshold {
			// Player should be whitelisted

			// Check if they have an existing admin role that's about to expire (within 1 day)
			adminStatus, err := p.apis.AdminAPI.GetPlayerAdminStatus(steamID)
			if err != nil {
				// No existing admin status or error checking - add new admin (skip if shutting down)
				if !p.shuttingDown {
					go p.addPlayerAsTemporaryAdmin(steamID, "")
				}
				continue
			}

			// Check if any of their admin roles are about to expire
			needsRenewal := false
			groupName := p.getStringConfig("whitelist_group_name")
			if groupName == "" {
				groupName = "seeder_whitelist"
			}

			for _, role := range adminStatus.Roles {
				if role.RoleName == groupName && role.ExpiresAt != nil {
					renewalHours := p.getIntConfig("admin_renewal_hours_before_expiry")
					if renewalHours <= 0 {
						renewalHours = 24 // Default to 24 hours
					}

					timeUntilExpiry := time.Until(*role.ExpiresAt)
					renewalWindow := time.Duration(renewalHours) * time.Hour

					// If expires within the renewal window, renew it
					if timeUntilExpiry <= renewalWindow && timeUntilExpiry > 0 {
						needsRenewal = true
						break
					}
				}
			}

			if needsRenewal {
				// Remove old admin role and add new one to extend expiration (skip if shutting down)
				if !p.shuttingDown {
					go p.renewPlayerAdminRole(steamID, "")
				}
			} else if len(adminStatus.Roles) == 0 {
				// No admin roles found, add new one (skip if shutting down)
				if !p.shuttingDown {
					go p.addPlayerAsTemporaryAdmin(steamID, "")
				}
			}
		}
	}

	return nil
}

// max returns the maximum of two float64 values
func max(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}
