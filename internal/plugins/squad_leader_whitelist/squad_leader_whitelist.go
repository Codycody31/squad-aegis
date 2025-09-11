// Based on the server seeder whitelist plugin, adapted for squad leadership

package squad_leader_whitelist

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

// SquadLeaderWhitelistPlugin manages a progressive whitelist for players who lead squads effectively
type SquadLeaderWhitelistPlugin struct {
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
	playerProgress     map[string]*PlayerProgressRecord
	squadLeaderSession map[string]*SquadLeaderSession
}

// PlayerProgressRecord tracks a player's squad leadership progress
type PlayerProgressRecord struct {
	SteamID         string    `json:"steam_id"`
	Progress        float64   `json:"progress"`
	LastProgressed  time.Time `json:"last_progressed"`
	TotalLeadership float64   `json:"total_leadership"`
	LastSeen        time.Time `json:"last_seen"`
}

// SquadLeaderSession tracks an active squad leadership session
type SquadLeaderSession struct {
	SteamID   string    `json:"steam_id"`
	StartTime time.Time `json:"start_time"`
	LastCheck time.Time `json:"last_check"`
	SquadSize int       `json:"squad_size"`
	SquadName string    `json:"squad_name"`
	Unlocked  bool      `json:"unlocked"`
}

// Define returns the plugin definition
func Define() plugin_manager.PluginDefinition {
	return plugin_manager.PluginDefinition{
		ID:                     "squad_leader_whitelist",
		Name:                   "Squad Leader Whitelist",
		Description:            "Tracks players who serve as squad leaders with 5+ members and progressively adds them to a whitelist based on time spent leading. Players earn progress when leading unlocked squads with minimum members and lose progress over time when inactive.",
		Version:                "1.0.0",
		Author:                 "Squad Aegis",
		AllowMultipleInstances: false,
		RequiredConnectors:     []string{},
		LongRunning:            true,

		ConfigSchema: plug_config_schema.ConfigSchema{
			Fields: []plug_config_schema.ConfigField{
				{
					Name:        "min_squad_size",
					Description: "Minimum squad size required for squad leadership progress.",
					Required:    false,
					Type:        plug_config_schema.FieldTypeInt,
					Default:     5,
				},
				{
					Name:        "hours_to_whitelist",
					Description: "Hours of squad leadership required to reach 100% whitelist status.",
					Required:    false,
					Type:        plug_config_schema.FieldTypeInt,
					Default:     8,
				},
				{
					Name:        "whitelist_duration_days",
					Description: "How many days whitelist status lasts before expiring.",
					Required:    false,
					Type:        plug_config_schema.FieldTypeInt,
					Default:     14,
				},
				{
					Name:        "decay_after_hours",
					Description: "Hours after last leadership before progress starts to decay.",
					Required:    false,
					Type:        plug_config_schema.FieldTypeInt,
					Default:     72,
				},
				{
					Name:        "min_players_for_decay",
					Description: "Minimum players on server for decay to be active.",
					Required:    false,
					Type:        plug_config_schema.FieldTypeInt,
					Default:     40,
				},
				{
					Name:        "min_players_for_leadership",
					Description: "Minimum players on server before leadership progress can be awarded.",
					Required:    false,
					Type:        plug_config_schema.FieldTypeInt,
					Default:     20,
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
					Name:        "require_unlocked_squad",
					Description: "Only award progress for leading unlocked squads.",
					Required:    false,
					Type:        plug_config_schema.FieldTypeBool,
					Default:     true,
				},
				{
					Name:        "whitelist_group_name",
					Description: "Admin group name for whitelisted squad leaders.",
					Required:    false,
					Type:        plug_config_schema.FieldTypeString,
					Default:     "squad_leader_whitelist",
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
					Default:     300, // 5 minutes
				},
				{
					Name:        "enable_chat_command",
					Description: "Enable the !slwl chat command for players to check their progress.",
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
					Default:     30,
				},
				{
					Name:        "admin_renewal_hours_before_expiry",
					Description: "How many hours before expiration to renew admin roles for qualifying players.",
					Required:    false,
					Type:        plug_config_schema.FieldTypeInt,
					Default:     48,
				},
			},
		},

		Events: []event_manager.EventType{
			event_manager.EventTypeLogNewGame,
			event_manager.EventTypeRconChatMessage,
			event_manager.EventTypeLogPlayerConnected,
			// event_manager.EventTypeLogPlayerSquadChange,
		},

		CreateInstance: func() plugin_manager.Plugin {
			return &SquadLeaderWhitelistPlugin{
				playerProgress:     make(map[string]*PlayerProgressRecord),
				squadLeaderSession: make(map[string]*SquadLeaderSession),
			}
		},
	}
}

// GetDefinition returns the plugin definition
func (p *SquadLeaderWhitelistPlugin) GetDefinition() plugin_manager.PluginDefinition {
	return Define()
}

// Initialize initializes the plugin with its configuration and dependencies
func (p *SquadLeaderWhitelistPlugin) Initialize(config map[string]interface{}, apis *plugin_manager.PluginAPIs) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.config = config
	p.apis = apis
	p.status = plugin_manager.PluginStatusStopped
	p.stopProgressRound = false
	p.playerProgress = make(map[string]*PlayerProgressRecord)
	p.squadLeaderSession = make(map[string]*SquadLeaderSession)

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

	return nil
}

// Start begins plugin execution
func (p *SquadLeaderWhitelistPlugin) Start(ctx context.Context) error {
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
func (p *SquadLeaderWhitelistPlugin) Stop() error {
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

	// Clear ticker references
	p.mu.Lock()
	p.progressTicker = nil
	p.decayTicker = nil
	p.adminSyncTicker = nil
	p.mu.Unlock()

	// Save final state to database
	if err := p.savePlayerProgress(); err != nil {
		p.apis.LogAPI.Error("Failed to save player progress to database on shutdown", err, nil)
	}

	// Now safely acquire mutex for final cleanup
	p.mu.Lock()
	defer p.mu.Unlock()

	p.status = plugin_manager.PluginStatusStopped

	return nil
}

// HandleEvent processes events
func (p *SquadLeaderWhitelistPlugin) HandleEvent(event *plugin_manager.PluginEvent) error {
	switch event.Type {
	case "LOG_NEW_GAME":
		return p.handleNewGame(event)
	case "RCON_CHAT_MESSAGE":
		return p.handleChatMessage(event)
	case "LOG_PLAYER_CONNECTED":
		return p.handlePlayerConnected(event)
	case "LOG_PLAYER_SQUAD_CHANGE":
		return p.handleSquadChange(event)
	}
	return nil
}

// GetStatus returns the current plugin status
func (p *SquadLeaderWhitelistPlugin) GetStatus() plugin_manager.PluginStatus {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.status
}

// GetConfig returns the current plugin configuration
func (p *SquadLeaderWhitelistPlugin) GetConfig() map[string]interface{} {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.config
}

// UpdateConfig updates the plugin configuration
func (p *SquadLeaderWhitelistPlugin) UpdateConfig(config map[string]interface{}) error {
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

	p.apis.LogAPI.Info("Squad Leader Whitelist plugin configuration updated", map[string]interface{}{
		"min_squad_size":          config["min_squad_size"],
		"hours_to_whitelist":      config["hours_to_whitelist"],
		"whitelist_duration_days": config["whitelist_duration_days"],
	})

	return nil
}

// handleNewGame processes new game events
func (p *SquadLeaderWhitelistPlugin) handleNewGame(event *plugin_manager.PluginEvent) error {
	if !p.getBoolConfig("wait_on_new_games") {
		return nil
	}

	// Temporarily pause progress tracking and clear squad sessions
	p.mu.Lock()
	p.stopProgressRound = true
	p.squadLeaderSession = make(map[string]*SquadLeaderSession) // Clear sessions on new game
	p.mu.Unlock()

	waitTime := p.getIntConfig("wait_time_on_new_game")
	if waitTime <= 0 {
		waitTime = 300
	}

	p.apis.LogAPI.Info("New game detected - temporarily pausing squad leader progress tracking", map[string]interface{}{
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
			p.apis.LogAPI.Info("Resuming squad leader progress tracking after new game wait period", nil)
		case <-p.ctx.Done():
			return
		}
	}()

	return nil
}

// handleChatMessage processes chat messages for the progress command
func (p *SquadLeaderWhitelistPlugin) handleChatMessage(rawEvent *plugin_manager.PluginEvent) error {
	if !p.getBoolConfig("enable_chat_command") {
		return nil
	}

	event, ok := rawEvent.Data.(*event_manager.RconChatMessageData)
	if !ok {
		return fmt.Errorf("invalid event data type")
	}

	if event.Message != "!slwl" && event.Message != "!squadleader" {
		return nil
	}

	return p.sendProgressToPlayer(event.SteamID)
}

// handlePlayerConnected tracks when players connect for statistics
func (p *SquadLeaderWhitelistPlugin) handlePlayerConnected(event *plugin_manager.PluginEvent) error {
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

// handleSquadChange tracks squad membership changes to identify squad leaders
func (p *SquadLeaderWhitelistPlugin) handleSquadChange(event *plugin_manager.PluginEvent) error {
	// Extract squad change info from event data
	eventData, ok := event.Data.(map[string]interface{})
	if !ok {
		return nil
	}

	steamID, ok := eventData["steam_id"].(string)
	if !ok || steamID == "" {
		return nil
	}

	// Check if player is a squad leader
	squadData, ok := eventData["squad"].(map[string]interface{})
	if !ok {
		return nil
	}

	squadName, _ := squadData["name"].(string)
	isLeader, _ := squadData["is_leader"].(bool)
	unlocked, _ := squadData["unlocked"].(bool)
	memberCount, _ := squadData["member_count"].(float64) // Comes as float64 from JSON

	if !isLeader {
		// Player is no longer a squad leader, remove their session
		p.mu.Lock()
		delete(p.squadLeaderSession, steamID)
		p.mu.Unlock()
		return nil
	}

	// Player is a squad leader, update or create their session
	p.mu.Lock()
	defer p.mu.Unlock()

	if session, exists := p.squadLeaderSession[steamID]; exists {
		session.LastCheck = time.Now()
		session.SquadSize = int(memberCount)
		session.SquadName = squadName
		session.Unlocked = unlocked
	} else {
		p.squadLeaderSession[steamID] = &SquadLeaderSession{
			SteamID:   steamID,
			StartTime: time.Now(),
			LastCheck: time.Now(),
			SquadSize: int(memberCount),
			SquadName: squadName,
			Unlocked:  unlocked,
		}
	}

	return nil
}

// progressTrackingLoop handles the periodic progress tracking
func (p *SquadLeaderWhitelistPlugin) progressTrackingLoop() {
	for {
		select {
		case <-p.ctx.Done():
			return
		case <-p.progressTicker.C:
			if err := p.trackProgress(); err != nil {
				p.apis.LogAPI.Error("Failed to track squad leader progress", err, nil)
			}
		}
	}
}

// decayLoop handles the periodic progress decay
func (p *SquadLeaderWhitelistPlugin) decayLoop() {
	for {
		select {
		case <-p.ctx.Done():
			return
		case <-p.decayTicker.C:
			if err := p.decayProgress(); err != nil {
				p.apis.LogAPI.Error("Failed to decay squad leader progress", err, nil)
			}
		}
	}
}

// adminSyncLoop handles periodic admin synchronization
func (p *SquadLeaderWhitelistPlugin) adminSyncLoop() {
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

// trackProgress awards progress to players leading qualifying squads
func (p *SquadLeaderWhitelistPlugin) trackProgress() error {
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
	for _, player := range players {
		if player.IsOnline {
			onlinePlayerCount++
		}
	}

	minPlayersForLeadership := p.getIntConfig("min_players_for_leadership")
	if onlinePlayerCount < minPlayersForLeadership {
		return nil // Not enough players to start awarding leadership progress
	}

	// Get configuration values
	minSquadSize := p.getIntConfig("min_squad_size")
	requireUnlocked := p.getBoolConfig("require_unlocked_squad")
	hoursToWhitelist := float64(p.getIntConfig("hours_to_whitelist"))
	intervalSeconds := float64(p.getIntConfig("progress_interval_seconds"))
	progressIncrement := (intervalSeconds / 3600.0) / hoursToWhitelist * 100.0

	whitelistThreshold := 100.0
	notificationThresholds := p.getIntArrayConfig("progress_notification_thresholds")

	now := time.Now()
	var updatedPlayers []string

	p.mu.Lock()
	// Process active squad leader sessions
	for steamID, session := range p.squadLeaderSession {
		// Check if squad meets size requirement
		if session.SquadSize < minSquadSize {
			continue
		}

		// Check if unlocked squad is required
		if requireUnlocked && !session.Unlocked {
			continue
		}

		// Find player name
		var playerName string
		for _, player := range players {
			if player.SteamID == steamID {
				playerName = player.Name
				break
			}
		}

		// Update session check time
		session.LastCheck = now

		// Get or create player progress record
		record, exists := p.playerProgress[steamID]
		if !exists {
			record = &PlayerProgressRecord{
				SteamID:         steamID,
				Progress:        0,
				LastProgressed:  now,
				TotalLeadership: 0,
				LastSeen:        now,
			}
			p.playerProgress[steamID] = record
		}

		oldProgress := record.Progress
		newProgress := record.Progress + progressIncrement
		record.Progress = newProgress
		record.LastProgressed = now
		record.TotalLeadership += progressIncrement
		record.LastSeen = now

		updatedPlayers = append(updatedPlayers, steamID)

		// Check for notification thresholds and whitelist status changes
		oldPercentage := oldProgress
		newPercentage := newProgress

		// Check if player just reached whitelist threshold
		if oldProgress < whitelistThreshold && newProgress >= whitelistThreshold {
			// Player just became whitelisted - add them as admin (skip if shutting down)
			if !p.shuttingDown {
				go p.addPlayerAsTemporaryAdmin(steamID, playerName)
			}
		}

		for _, threshold := range notificationThresholds {
			thresholdFloat := float64(threshold)
			if oldPercentage < thresholdFloat && newPercentage >= thresholdFloat {
				// Player crossed a notification threshold
				go p.sendProgressNotification(steamID, playerName, newPercentage, newProgress >= whitelistThreshold)
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
		if len(updatedPlayers) >= 3 {
			p.apis.LogAPI.Debug("Awarded squad leader progress", map[string]interface{}{
				"player_count":       onlinePlayerCount,
				"min_squad_size":     minSquadSize,
				"progress_increment": progressIncrement,
				"updated_players":    len(updatedPlayers),
			})
		}
	}

	return nil
}

// decayProgress applies progress decay to inactive squad leaders
func (p *SquadLeaderWhitelistPlugin) decayProgress() error {
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

	// Calculate decay increment
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
				whitelistThreshold := 100.0
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
			p.apis.LogAPI.Debug("Applied squad leader progress decay", map[string]interface{}{
				"decayed_players":   decayedPlayers,
				"decay_increment":   decayIncrement,
				"server_population": onlinePlayerCount,
			})
		}
	}

	return nil
}

// sendProgressToPlayer sends progress information to a specific player
func (p *SquadLeaderWhitelistPlugin) sendProgressToPlayer(steamID string) error {
	p.mu.Lock()
	record, exists := p.playerProgress[steamID]
	session, hasActiveSession := p.squadLeaderSession[steamID]
	p.mu.Unlock()

	whitelistThreshold := 100.0

	var message string
	if !exists || record.Progress == 0 {
		message = "No squad leadership progress found.\n" +
			"Lead a squad with 5+ members to earn progress!"
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
		totalHours := record.TotalLeadership / 100.0 * hoursToWhitelist

		message = status + "\n" +
			fmt.Sprintf("Total Leadership: %.1f hours", totalHours)

		if hasActiveSession {
			message += "\n" +
				fmt.Sprintf("Currently leading squad: %s (%d members)", session.SquadName, session.SquadSize)
			if !session.Unlocked {
				message += " [LOCKED]"
			}
		}
	}

	if err := p.apis.RconAPI.SendWarningToPlayer(steamID, message); err != nil {
		return fmt.Errorf("failed to send progress message: %w", err)
	}

	return nil
}

// sendProgressNotification sends a notification when a player crosses a threshold
func (p *SquadLeaderWhitelistPlugin) sendProgressNotification(steamID, playerName string, percentage float64, isWhitelisted bool) {
	var message string
	if isWhitelisted {
		message = "🎉 CONGRATULATIONS! 🎉\n" +
			"You are now SQUAD LEADER WHITELISTED!\n" +
			"Thank you for your leadership!"
	} else {
		message = "🎉 SQUAD LEADER PROGRESS 🎉\n" +
			fmt.Sprintf("Leadership Update: %.0f%%", percentage) + "\n" +
			"Keep leading to earn whitelist!"
	}

	if err := p.apis.RconAPI.SendWarningToPlayer(steamID, message); err != nil {
		p.apis.LogAPI.Error("Failed to send progress notification", err, map[string]interface{}{
			"steam_id":    steamID,
			"player_name": playerName,
		})
	}
}

// getPlayerRank returns the rank of a player among whitelisted players
func (p *SquadLeaderWhitelistPlugin) getPlayerRank(steamID string) int {
	whitelistThreshold := 100.0

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
func (p *SquadLeaderWhitelistPlugin) loadPlayerProgress() error {
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
func (p *SquadLeaderWhitelistPlugin) savePlayerProgress() error {
	p.mu.Lock()
	data, err := json.Marshal(p.playerProgress)
	p.mu.Unlock()

	if err != nil {
		return fmt.Errorf("failed to marshal player progress: %w", err)
	}

	return p.apis.DatabaseAPI.SetPluginData("player_progress", string(data))
}

// Helper methods for config access

func (p *SquadLeaderWhitelistPlugin) getStringConfig(key string) string {
	if value, ok := p.config[key].(string); ok {
		return value
	}
	return ""
}

func (p *SquadLeaderWhitelistPlugin) getIntConfig(key string) int {
	if value, ok := p.config[key].(int); ok {
		return value
	}
	if value, ok := p.config[key].(float64); ok {
		return int(value)
	}
	return 0
}

func (p *SquadLeaderWhitelistPlugin) getBoolConfig(key string) bool {
	if value, ok := p.config[key].(bool); ok {
		return value
	}
	return false
}

func (p *SquadLeaderWhitelistPlugin) getIntArrayConfig(key string) []int {
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
func (p *SquadLeaderWhitelistPlugin) addPlayerAsTemporaryAdmin(steamID, playerName string) {
	groupName := p.getStringConfig("whitelist_group_name")
	if groupName == "" {
		groupName = "squad_leader_whitelist"
	}

	// Create admin notes indicating this is from the Squad Leader Whitelist plugin
	notes := fmt.Sprintf("Plugin: Squad Leader Whitelist - Automatically added for squad leadership contributions. Player: %s", playerName)

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

	// Also broadcast a server-wide message about the new squad leader whitelist member
	broadcastMessage := fmt.Sprintf("⭐ %s has earned squad leader whitelist status! Thank you for your leadership!", playerName)
	if err := p.apis.RconAPI.Broadcast(broadcastMessage); err != nil {
		p.apis.LogAPI.Error("Failed to broadcast squad leader whitelist achievement", err, map[string]interface{}{
			"steam_id":    steamID,
			"player_name": playerName,
		})
	}
}

// removePlayerAsTemporaryAdmin removes a player as a temporary admin via direct database operations
func (p *SquadLeaderWhitelistPlugin) removePlayerAsTemporaryAdmin(steamID, playerName string) {
	// Create admin notes indicating this is from the Squad Leader Whitelist plugin
	notes := fmt.Sprintf("Plugin: Squad Leader Whitelist - Automatically removed (no longer qualifies). Player: %s", playerName)

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
func (p *SquadLeaderWhitelistPlugin) renewPlayerAdminRole(steamID, playerName string) {
	groupName := p.getStringConfig("whitelist_group_name")
	if groupName == "" {
		groupName = "squad_leader_whitelist"
	}

	// Create admin notes indicating this is a renewal from the Squad Leader Whitelist plugin
	notes := fmt.Sprintf("Plugin: Squad Leader Whitelist - Role renewed to extend expiration. Player: %s", playerName)

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
func (p *SquadLeaderWhitelistPlugin) syncTemporaryAdmins() error {
	// This function ensures database consistency for all whitelisted players
	// and refreshes admin roles that are about to expire
	whitelistThreshold := 100.0
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
				groupName = "squad_leader_whitelist"
			}

			for _, role := range adminStatus.Roles {
				if role.RoleName == groupName && role.ExpiresAt != nil {
					renewalHours := p.getIntConfig("admin_renewal_hours_before_expiry")
					if renewalHours <= 0 {
						renewalHours = 48 // Default to 48 hours
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
