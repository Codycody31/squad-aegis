// Based on the server seeder whitelist plugin, adapted for squad leadership

package squad_leader_whitelist

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"sync"
	"time"

	"go.codycody31.dev/squad-aegis/internal/event_manager"
	"go.codycody31.dev/squad-aegis/internal/plugin_manager"
	"go.codycody31.dev/squad-aegis/internal/shared/plug_config_schema"
	"go.codycody31.dev/squad-aegis/internal/shared/utils"
	"go.codycody31.dev/squad-aegis/internal/shared/whitelistprogress"
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
type PlayerProgressRecord = whitelistprogress.PlayerRecord

type legacyPlayerProgressRecord struct {
	SteamID         string    `json:"steam_id"`
	Progress        float64   `json:"progress"`
	LastProgressed  time.Time `json:"last_progressed"`
	TotalLeadership float64   `json:"total_leadership"`
	LastSeen        time.Time `json:"last_seen"`
}

// SquadLeaderSession tracks an active squad leadership session
type SquadLeaderSession struct {
	PlayerID  string    `json:"player_id"`
	SteamID   string    `json:"steam_id,omitempty"`
	EOSID     string    `json:"eos_id,omitempty"`
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
			event_manager.EventTypeLogGameEventUnified,
			event_manager.EventTypeRconChatMessage,
			event_manager.EventTypeLogPlayerConnected,
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

func (p *SquadLeaderWhitelistPlugin) GetCommands() []plugin_manager.PluginCommand {
	return []plugin_manager.PluginCommand{}
}

func (p *SquadLeaderWhitelistPlugin) ExecuteCommand(commandID string, params map[string]interface{}) (*plugin_manager.CommandResult, error) {
	return nil, fmt.Errorf("no commands available")
}

func (p *SquadLeaderWhitelistPlugin) GetCommandExecutionStatus(executionID string) (*plugin_manager.CommandExecutionStatus, error) {
	return nil, fmt.Errorf("no commands available")
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
	case string(event_manager.EventTypeLogGameEventUnified):
		if unifiedEvent, ok := event.Data.(*event_manager.LogGameEventUnifiedData); ok {
			if unifiedEvent.EventType == "NEW_GAME" {
				return p.handleNewGame(event)
			}
		}
	case "RCON_CHAT_MESSAGE":
		return p.handleChatMessage(event)
	case "LOG_PLAYER_CONNECTED":
		return p.handlePlayerConnected(event)
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

	// Validate new config
	definition := p.GetDefinition()
	if err := definition.ConfigSchema.Validate(config); err != nil {
		p.mu.Unlock()
		return fmt.Errorf("invalid config: %w", err)
	}

	// Fill defaults
	definition.ConfigSchema.FillDefaults(config)

	p.config = config
	shouldSyncAdmins := false

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
			shouldSyncAdmins = true
			startAdminSyncLoop := p.adminSyncTicker == nil
			if p.adminSyncTicker != nil {
				p.adminSyncTicker.Stop()
			}
			adminSyncInterval := time.Duration(p.getIntConfig("admin_sync_interval_minutes")) * time.Minute
			p.adminSyncTicker = time.NewTicker(adminSyncInterval)
			if startAdminSyncLoop {
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

	p.mu.Unlock()

	p.apis.LogAPI.Info("Squad Leader Whitelist plugin configuration updated", map[string]interface{}{
		"min_squad_size":          config["min_squad_size"],
		"hours_to_whitelist":      config["hours_to_whitelist"],
		"whitelist_duration_days": config["whitelist_duration_days"],
	})

	if shouldSyncAdmins {
		if err := p.syncTemporaryAdmins(); err != nil {
			p.apis.LogAPI.Error("Failed to reconcile squad leader whitelist admins after config update", err, nil)
		}
	}

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

	playerID := event.PreferredPlayerID()
	return p.sendProgressToPlayer(playerID, event.SteamID, event.EosID)
}

// handlePlayerConnected tracks when players connect for statistics
func (p *SquadLeaderWhitelistPlugin) handlePlayerConnected(event *plugin_manager.PluginEvent) error {
	var steamID string
	var eosID string
	switch data := event.Data.(type) {
	case *event_manager.LogPlayerConnectedData:
		steamID = data.SteamID
		eosID = data.EOSID
	case event_manager.LogPlayerConnectedData:
		steamID = data.SteamID
		eosID = data.EOSID
	case map[string]interface{}:
		steamID, _ = data["steam_id"].(string)
		eosID, _ = data["eos_id"].(string)
	}

	playerID := utils.NormalizePlayerID(steamID)
	if playerID == "" {
		playerID = utils.NormalizePlayerID(eosID)
	}
	playerID = utils.NormalizePlayerID(playerID)
	if playerID == "" {
		return nil
	}

	// Update last seen time for the player
	p.mu.Lock()
	if record, exists := whitelistprogress.FindRecordByIdentifiers(p.playerProgress, steamID, eosID); exists {
		record.LastSeenAt = time.Now()
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

	var playerID string
	if steamID, ok := eventData["steam_id"].(string); ok && steamID != "" {
		playerID = steamID
	} else if eosID, ok := eventData["eos_id"].(string); ok {
		playerID = eosID
	}
	playerID = utils.NormalizePlayerID(playerID)
	if playerID == "" {
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
		sessionKey, _, exists := p.findSquadLeaderSessionLocked(playerID, eventData["steam_id"], eventData["eos_id"])
		if exists {
			delete(p.squadLeaderSession, sessionKey)
		} else {
			delete(p.squadLeaderSession, playerID)
		}
		p.mu.Unlock()
		return nil
	}

	// Player is a squad leader, update or create their session
	p.mu.Lock()
	defer p.mu.Unlock()

	if sessionKey, session, exists := p.findSquadLeaderSessionLocked(playerID, eventData["steam_id"], eventData["eos_id"]); exists {
		if sessionKey != playerID {
			delete(p.squadLeaderSession, sessionKey)
			p.squadLeaderSession[playerID] = session
		}
		session.PlayerID = playerID
		if steamID, ok := eventData["steam_id"].(string); ok && steamID != "" {
			session.SteamID = utils.NormalizePlayerID(steamID)
		}
		if eosID, ok := eventData["eos_id"].(string); ok && eosID != "" {
			session.EOSID = utils.NormalizePlayerID(eosID)
		}
		session.LastCheck = time.Now()
		session.SquadSize = int(memberCount)
		session.SquadName = squadName
		session.Unlocked = unlocked
	} else {
		steamID, _ := eventData["steam_id"].(string)
		eosID, _ := eventData["eos_id"].(string)
		p.squadLeaderSession[playerID] = &SquadLeaderSession{
			PlayerID:  playerID,
			SteamID:   utils.NormalizePlayerID(steamID),
			EOSID:     utils.NormalizePlayerID(eosID),
			StartTime: time.Now(),
			LastCheck: time.Now(),
			SquadSize: int(memberCount),
			SquadName: squadName,
			Unlocked:  unlocked,
		}
	}

	return nil
}

func (p *SquadLeaderWhitelistPlugin) findSquadLeaderSessionLocked(playerID string, steamIDValue interface{}, eosIDValue interface{}) (string, *SquadLeaderSession, bool) {
	steamID, _ := steamIDValue.(string)
	eosID, _ := eosIDValue.(string)

	candidates := []string{
		utils.NormalizePlayerID(playerID),
		utils.NormalizePlayerID(steamID),
		utils.NormalizePlayerID(eosID),
	}

	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}
		if session, exists := p.squadLeaderSession[candidate]; exists && session != nil {
			return candidate, session, true
		}
	}

	for key, session := range p.squadLeaderSession {
		if session == nil {
			continue
		}
		for _, candidate := range candidates {
			if candidate == "" {
				continue
			}
			if utils.MatchPlayerID(candidate, session.SteamID, session.EOSID) || candidate == utils.NormalizePlayerID(session.PlayerID) {
				return key, session, true
			}
		}
	}

	return "", nil, false
}

// updateSquadLeaderSessions updates the squad leader sessions based on current player state
func (p *SquadLeaderWhitelistPlugin) updateSquadLeaderSessions(players []*plugin_manager.PlayerInfo) {
	p.mu.Lock()
	defer p.mu.Unlock()

	now := time.Now()

	// First, mark all existing sessions as potentially inactive
	activeLeaders := make(map[string]bool)

	// Check each online player to see if they're currently a squad leader
	for _, player := range players {
		if !player.IsOnline || !player.IsSquadLeader {
			continue
		}

		playerID := player.PreferredID()
		if playerID == "" {
			continue
		}

		activeLeaders[playerID] = true

		// Update or create squad leader session
		if session, exists := p.squadLeaderSession[playerID]; exists {
			// Update existing session
			session.SteamID = utils.NormalizePlayerID(player.SteamID)
			session.EOSID = utils.NormalizePlayerID(player.EOSID)
			session.LastCheck = now
			session.SquadSize = p.getSquadSize(players, player.TeamID, player.SquadID)
			session.SquadName = fmt.Sprintf("Squad %d", player.SquadID)
			session.Unlocked = true // We don't have unlock info from the player API, assume unlocked for now
		} else {
			// Create new session for this squad leader
			p.squadLeaderSession[playerID] = &SquadLeaderSession{
				PlayerID:  playerID,
				SteamID:   utils.NormalizePlayerID(player.SteamID),
				EOSID:     utils.NormalizePlayerID(player.EOSID),
				StartTime: now,
				LastCheck: now,
				SquadSize: p.getSquadSize(players, player.TeamID, player.SquadID),
				SquadName: fmt.Sprintf("Squad %d", player.SquadID),
				Unlocked:  true, // We don't have unlock info from the player API, assume unlocked for now
			}
		}
	}

	// Remove sessions for players who are no longer squad leaders
	for playerID := range p.squadLeaderSession {
		if !activeLeaders[playerID] {
			delete(p.squadLeaderSession, playerID)
		}
	}
}

// getSquadSize counts the number of players in a specific squad
func (p *SquadLeaderWhitelistPlugin) getSquadSize(players []*plugin_manager.PlayerInfo, teamID, squadID int) int {
	count := 0
	for _, player := range players {
		if player.IsOnline && player.TeamID == teamID && player.SquadID == squadID {
			count++
		}
	}
	return count
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

	// Update squad leader sessions based on current player state
	p.updateSquadLeaderSessions(players)

	// Get configuration values
	minSquadSize := p.getIntConfig("min_squad_size")
	requireUnlocked := p.getBoolConfig("require_unlocked_squad")
	requiredSeconds := p.requiredWhitelistSeconds()
	if requiredSeconds <= 0 {
		return nil
	}

	intervalSeconds := int64(p.getIntConfig("progress_interval_seconds"))
	if intervalSeconds <= 0 {
		return nil
	}

	notificationThresholds := p.getIntArrayConfig("progress_notification_thresholds")
	manageTemporaryAdmins := p.getBoolConfig("auto_add_temporary_admins")

	now := time.Now()
	var updatedPlayers []string

	p.mu.Lock()
	// Process active squad leader sessions
	for playerID, session := range p.squadLeaderSession {
		// Check if squad meets size requirement
		if session.SquadSize < minSquadSize {
			continue
		}

		// Check if unlocked squad is required
		if requireUnlocked && !session.Unlocked {
			continue
		}

		// Find player name and preferred ID for RCON calls
		var playerName string
		for _, player := range players {
			if player.MatchesPlayerID(playerID) {
				playerName = player.Name
				break
			}
		}

		// Update session check time
		session.LastCheck = now

		// Get or create player progress record
		record := whitelistprogress.EnsureRecord(p.playerProgress, session.SteamID, session.EOSID, now)
		if record == nil {
			continue
		}

		oldQualifiedSeconds := record.QualifiedSeconds
		record.QualifiedSeconds += intervalSeconds
		record.LastEarnedAt = now
		record.LifetimeSeconds += intervalSeconds
		record.LastSeenAt = now
		newQualifiedSeconds := record.QualifiedSeconds

		updatedPlayers = append(updatedPlayers, record.PlayerID)

		// Check for notification thresholds and whitelist status changes
		oldPercentage := whitelistprogress.Percent(oldQualifiedSeconds, requiredSeconds)
		newPercentage := whitelistprogress.Percent(newQualifiedSeconds, requiredSeconds)

		// Check if player just reached whitelist threshold
		if manageTemporaryAdmins &&
			oldQualifiedSeconds < requiredSeconds &&
			newQualifiedSeconds >= requiredSeconds {
			// Player just became whitelisted - add them as admin (skip if shutting down)
			if !p.shuttingDown {
				go p.addPlayerAsTemporaryAdmin(record.PlayerID, playerName)
			}
		}

		for _, threshold := range notificationThresholds {
			thresholdFloat := float64(threshold)
			if oldPercentage < thresholdFloat && newPercentage >= thresholdFloat {
				go p.sendProgressNotification(playerID, playerName, newPercentage, newQualifiedSeconds >= requiredSeconds)
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
				"player_count":    onlinePlayerCount,
				"min_squad_size":  minSquadSize,
				"seconds_awarded": intervalSeconds,
				"updated_players": len(updatedPlayers),
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

	requiredSeconds := p.requiredWhitelistSeconds()
	if requiredSeconds <= 0 {
		return nil
	}

	decayAfterHours := float64(p.getIntConfig("decay_after_hours"))
	decaySeconds := int64(p.getIntConfig("decay_interval_seconds"))
	if decaySeconds <= 0 {
		return nil
	}
	manageTemporaryAdmins := p.getBoolConfig("auto_add_temporary_admins")

	now := time.Now()
	decayThreshold := time.Duration(decayAfterHours * float64(time.Hour))
	var decayedPlayers int

	p.mu.Lock()
	for _, record := range p.playerProgress {
		timeSinceProgress := now.Sub(record.LastEarnedAt)
		if timeSinceProgress > decayThreshold {
			if record.QualifiedSeconds > 0 {
				oldQualifiedSeconds := record.QualifiedSeconds
				record.QualifiedSeconds = whitelistprogress.DecayQualifiedSeconds(record.QualifiedSeconds, decaySeconds)
				decayedPlayers++

				// Check if player lost whitelist status due to decay
				if manageTemporaryAdmins &&
					oldQualifiedSeconds >= requiredSeconds &&
					record.QualifiedSeconds < requiredSeconds {
					// Player lost whitelist status - remove admin privileges (skip if shutting down)
					if !p.shuttingDown {
						go p.removePlayerAsTemporaryAdmin(record.PlayerID, "")
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
				"decay_seconds":     decaySeconds,
				"server_population": onlinePlayerCount,
			})
		}
	}

	return nil
}

// sendProgressToPlayer sends progress information to a specific player
func (p *SquadLeaderWhitelistPlugin) sendProgressToPlayer(playerID string, steamID string, eosID string) error {
	p.mu.Lock()
	record, exists := whitelistprogress.FindRecordByIdentifiers(p.playerProgress, steamID, eosID)
	if !exists {
		record, exists = whitelistprogress.FindRecord(p.playerProgress, playerID)
	}
	_, session, hasActiveSession := p.findSquadLeaderSessionLocked(playerID, steamID, eosID)
	p.mu.Unlock()

	requiredSeconds := p.requiredWhitelistSeconds()

	var message string
	if !exists || record.QualifiedSeconds == 0 || requiredSeconds <= 0 {
		message = "No squad leadership progress found.\n" +
			"Lead a squad with 5+ members to earn progress!"
	} else {
		percentage := whitelistprogress.Percent(record.QualifiedSeconds, requiredSeconds)
		if percentage > 100 {
			percentage = 100
		}

		var status string
		if whitelistprogress.IsQualified(record.QualifiedSeconds, requiredSeconds) {
			// Get rank among whitelisted players
			rank := p.getPlayerRank(record.PlayerID)
			status = fmt.Sprintf("WHITELISTED (Rank #%d)", rank)
		} else {
			status = fmt.Sprintf("Progress: %.1f%%", percentage)
		}

		totalHours := whitelistprogress.SecondsToHours(record.LifetimeSeconds)

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

	if err := p.apis.RconAPI.SendWarningToPlayer(playerID, message); err != nil {
		return fmt.Errorf("failed to send progress message: %w", err)
	}

	return nil
}

// sendProgressNotification sends a notification when a player crosses a threshold
func (p *SquadLeaderWhitelistPlugin) sendProgressNotification(playerID, playerName string, percentage float64, isWhitelisted bool) {
	var message string
	if isWhitelisted {
		message = "CONGRATULATIONS!\n" +
			"You are now SQUAD LEADER WHITELISTED!\n" +
			"Thank you for your leadership!"
	} else {
		message = "SQUAD LEADER PROGRESS\n" +
			fmt.Sprintf("Leadership Update: %.0f%%", percentage) + "\n" +
			"Keep leading to earn whitelist!"
	}

	if err := p.apis.RconAPI.SendWarningToPlayer(playerID, message); err != nil {
		p.apis.LogAPI.Error("Failed to send progress notification", err, map[string]interface{}{
			"player_id":   playerID,
			"player_name": playerName,
		})
	}
}

// getPlayerRank returns the rank of a player among whitelisted players
func (p *SquadLeaderWhitelistPlugin) getPlayerRank(playerID string) int {
	requiredSeconds := p.requiredWhitelistSeconds()
	if requiredSeconds <= 0 {
		return 1
	}

	type playerRank struct {
		playerID string
		progress int64
	}

	var whitelistedPlayers []playerRank
	p.mu.Lock()
	record, exists := whitelistprogress.FindRecord(p.playerProgress, playerID)
	if exists {
		playerID = record.PlayerID
	}
	for _, record := range p.playerProgress {
		if whitelistprogress.IsQualified(record.QualifiedSeconds, requiredSeconds) {
			whitelistedPlayers = append(whitelistedPlayers, playerRank{
				playerID: record.PlayerID,
				progress: record.QualifiedSeconds,
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
		if player.playerID == playerID {
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

	state, err := whitelistprogress.ParseState(data)
	if err == nil {
		p.playerProgress = state.Players
		return nil
	}

	if !errors.Is(err, whitelistprogress.ErrUnknownFormat) {
		return fmt.Errorf("failed to parse player progress: %w", err)
	}

	var legacyProgress map[string]*legacyPlayerProgressRecord
	if err := json.Unmarshal([]byte(data), &legacyProgress); err != nil {
		return fmt.Errorf("failed to unmarshal legacy player progress: %w", err)
	}

	requiredHours := p.getIntConfig("hours_to_whitelist")
	migratedProgress := make(map[string]*PlayerProgressRecord, len(legacyProgress))
	for legacyPlayerID, record := range legacyProgress {
		if record == nil {
			continue
		}

		recordPlayerID := utils.NormalizePlayerID(record.SteamID)
		if recordPlayerID == "" {
			recordPlayerID = utils.NormalizePlayerID(legacyPlayerID)
		}
		if recordPlayerID == "" {
			continue
		}

		migratedProgress[recordPlayerID] = &PlayerProgressRecord{
			PlayerID: recordPlayerID,
			SteamID: func() string {
				if utils.IsSteamID(recordPlayerID) {
					return recordPlayerID
				}
				return ""
			}(),
			EOSID: func() string {
				if utils.IsEOSID(recordPlayerID) {
					return recordPlayerID
				}
				return ""
			}(),
			QualifiedSeconds: whitelistprogress.LegacyPercentToSeconds(record.Progress, requiredHours),
			LifetimeSeconds:  whitelistprogress.LegacyPercentToSeconds(record.TotalLeadership, requiredHours),
			LastEarnedAt:     record.LastProgressed,
			LastSeenAt:       record.LastSeen,
		}
	}

	p.playerProgress = migratedProgress

	migratedData, err := p.marshalPlayerProgressLocked()
	if err != nil {
		p.apis.LogAPI.Warn("Failed to marshal migrated squad leader player progress", map[string]interface{}{
			"error": err.Error(),
		})
		return nil
	}

	if err := p.apis.DatabaseAPI.SetPluginData("player_progress", string(migratedData)); err != nil {
		p.apis.LogAPI.Warn("Failed to persist migrated squad leader player progress", map[string]interface{}{
			"error": err.Error(),
		})
	}

	return nil
}

// savePlayerProgress saves player progress to database
func (p *SquadLeaderWhitelistPlugin) savePlayerProgress() error {
	p.mu.Lock()
	data, err := p.marshalPlayerProgressLocked()
	p.mu.Unlock()

	if err != nil {
		return fmt.Errorf("failed to marshal player progress: %w", err)
	}

	return p.apis.DatabaseAPI.SetPluginData("player_progress", string(data))
}

// marshalPlayerProgressLocked serializes player progress while p.mu is held.
func (p *SquadLeaderWhitelistPlugin) marshalPlayerProgressLocked() ([]byte, error) {
	return whitelistprogress.MarshalPlayers(p.playerProgress)
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

func (p *SquadLeaderWhitelistPlugin) requiredWhitelistSeconds() int64 {
	return whitelistprogress.RequiredSeconds(p.getIntConfig("hours_to_whitelist"))
}

func (p *SquadLeaderWhitelistPlugin) whitelistGroupName() string {
	groupName := p.getStringConfig("whitelist_group_name")
	if groupName == "" {
		return "squad_leader_whitelist"
	}
	return groupName
}

// addPlayerAsTemporaryAdmin adds a player as a temporary admin via direct database operations
func (p *SquadLeaderWhitelistPlugin) addPlayerAsTemporaryAdmin(playerID, playerName string) {
	groupName := p.whitelistGroupName()

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
	if err := p.apis.AdminAPI.AddTemporaryAdmin(playerID, groupName, notes, expiresAt); err != nil {
		p.apis.LogAPI.Error("Failed to add player as temporary admin via AdminAPI", err, map[string]interface{}{
			"player_id":   playerID,
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
				"player_id": playerID,
			})
		}
	}

	p.apis.LogAPI.Info("Added player as temporary admin", map[string]interface{}{
		"player_id":   playerID,
		"player_name": playerName,
		"group_name":  groupName,
		"notes":       notes,
	})
}

// removePlayerAsTemporaryAdmin removes a player as a temporary admin via direct database operations
func (p *SquadLeaderWhitelistPlugin) removePlayerAsTemporaryAdmin(playerID, playerName string) {
	groupName := p.whitelistGroupName()

	// Create admin notes indicating this is from the Squad Leader Whitelist plugin
	notes := fmt.Sprintf("Plugin: Squad Leader Whitelist - Automatically removed (no longer qualifies). Player: %s", playerName)

	// Remove player as temporary admin
	if err := p.apis.AdminAPI.RemoveTemporaryAdminRole(playerID, groupName, notes); err != nil {
		p.apis.LogAPI.Error("Failed to remove player as temporary admin via AdminAPI", err, map[string]interface{}{
			"player_id":   playerID,
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
			p.apis.LogAPI.Error("Failed to reload server admin config after removing admin", err, map[string]interface{}{
				"player_id": playerID,
			})
		}
	}

	p.apis.LogAPI.Info("Removed player as temporary admin", map[string]interface{}{
		"player_id":   playerID,
		"player_name": playerName,
		"group_name":  groupName,
		"notes":       notes,
	})
}

// renewPlayerAdminRole refreshes a player's managed admin role to extend expiration
func (p *SquadLeaderWhitelistPlugin) renewPlayerAdminRole(playerID, playerName string) {
	groupName := p.whitelistGroupName()

	// Create admin notes indicating this is a renewal from the Squad Leader Whitelist plugin
	notes := fmt.Sprintf("Plugin: Squad Leader Whitelist - Role renewed to extend expiration. Player: %s", playerName)

	// Calculate new expiration time
	var expiresAt *time.Time
	whitelistDurationDays := p.getIntConfig("whitelist_duration_days")
	if whitelistDurationDays > 0 {
		expiration := time.Now().AddDate(0, 0, whitelistDurationDays)
		expiresAt = &expiration
	}

	// Add new admin role with fresh expiration
	if err := p.apis.AdminAPI.AddTemporaryAdmin(playerID, groupName, notes, expiresAt); err != nil {
		p.apis.LogAPI.Error("Failed to re-add admin role after renewal", err, map[string]interface{}{
			"player_id":   playerID,
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
				"player_id": playerID,
			})
		}
	}

	p.apis.LogAPI.Info("Renewed player admin role with extended expiration", map[string]interface{}{
		"player_id":   playerID,
		"player_name": playerName,
		"group_name":  groupName,
		"notes":       notes,
	})
}

// syncTemporaryAdmins synchronizes temporary admin status with current whitelist
func (p *SquadLeaderWhitelistPlugin) syncTemporaryAdmins() error {
	requiredSeconds := p.requiredWhitelistSeconds()
	if requiredSeconds <= 0 {
		return nil
	}

	groupName := p.whitelistGroupName()
	whitelistDurationDays := p.getIntConfig("whitelist_duration_days")
	renewalHours := p.getIntConfig("admin_renewal_hours_before_expiry")
	if renewalHours <= 0 {
		renewalHours = 48
	}
	renewalWindow := time.Duration(renewalHours) * time.Hour

	p.mu.Lock()
	shuttingDown := p.shuttingDown
	qualifiedPlayers := make(map[string]bool, len(p.playerProgress))
	for playerID, record := range p.playerProgress {
		if record != nil && whitelistprogress.IsQualified(record.QualifiedSeconds, requiredSeconds) {
			qualifiedPlayers[playerID] = true
		}
	}
	p.mu.Unlock()

	admins, err := p.apis.AdminAPI.ListTemporaryAdmins()
	if err != nil {
		return fmt.Errorf("failed to list temporary admins: %w", err)
	}

	managedAdmins := make(map[string]*plugin_manager.TemporaryAdminInfo)
	for _, admin := range admins {
		if admin == nil || admin.RoleName != groupName {
			continue
		}
		if adminID := admin.PreferredID(); adminID != "" {
			managedAdmins[adminID] = admin
		}
	}

	if shuttingDown {
		return nil
	}

	for playerID := range qualifiedPlayers {
		admin, exists := managedAdmins[playerID]
		if !exists {
			p.addPlayerAsTemporaryAdmin(playerID, "")
			continue
		}

		if whitelistDurationDays <= 0 {
			delete(managedAdmins, playerID)
			continue
		}

		if admin.ExpiresAt == nil {
			p.renewPlayerAdminRole(playerID, "")
			delete(managedAdmins, playerID)
			continue
		}

		timeUntilExpiry := time.Until(*admin.ExpiresAt)
		if timeUntilExpiry <= renewalWindow && timeUntilExpiry > 0 {
			p.renewPlayerAdminRole(playerID, "")
		}
		delete(managedAdmins, playerID)
	}

	for playerID := range managedAdmins {
		if qualifiedPlayers[playerID] {
			continue
		}
		p.removePlayerAsTemporaryAdmin(playerID, "")
	}

	return nil
}

// max returns the maximum of two float64 values
