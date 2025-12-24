// Copyright (c) 2025 Squad Aegis
// SPDX-License-Identifier: AGPL-3.0-only
//
// This file is part of the AGPL-licensed component located in internal/plugins/squad_creation_blocker.
// You may obtain a copy of the License at:
// https://www.gnu.org/licenses/agpl-3.0.html

package squad_creation_blocker

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"sync"
	"time"

	"go.codycody31.dev/squad-aegis/internal/event_manager"
	"go.codycody31.dev/squad-aegis/internal/plugin_manager"
	"go.codycody31.dev/squad-aegis/internal/shared/plug_config_schema"
)

// PlayerAttemptData tracks rate limiting data for a player
type PlayerAttemptData struct {
	AttemptCount  int
	CooldownEnd   time.Time
	WarningTicker *time.Ticker
	CancelWarning context.CancelFunc
}

// SquadCreationBlockerPlugin prevents squads with custom names from being created
// within a specified time after a new game starts and at the end of a round
type SquadCreationBlockerPlugin struct {
	// Plugin configuration
	config map[string]interface{}
	apis   *plugin_manager.PluginAPIs

	// State management
	mu     sync.Mutex
	status plugin_manager.PluginStatus
	ctx    context.Context
	cancel context.CancelFunc

	// Blocking state
	isBlocking     bool
	isRoundEnding  bool
	blockEndTime   time.Time
	broadcastTimer *time.Timer

	// Rate limiting data
	playerAttempts map[string]*PlayerAttemptData // steamID -> attempt data
	knownSquads    map[string]bool               // "teamID-squadID" -> true
	pollTicker     *time.Ticker
	pollCancel     context.CancelFunc
}

// Define returns the plugin definition
func Define() plugin_manager.PluginDefinition {
	return plugin_manager.PluginDefinition{
		ID:                     "squad_creation_blocker",
		Name:                   "Squad Creation Blocker",
		Description:            "The Squad Creation Blocker plugin prevents squads with custom names from being created within a specified time after a new game starts and at the end of a round. It includes anti-spam rate limiting with configurable warnings, cooldowns, kick functionality, and optional cooldown reset behavior to prevent players from overwhelming the system.",
		Version:                "1.0.0",
		Author:                 "Squad Aegis",
		AllowMultipleInstances: false,
		RequiredConnectors:     []string{},
		LongRunning:            true,

		ConfigSchema: plug_config_schema.ConfigSchema{
			Fields: []plug_config_schema.ConfigField{
				{
					Name:        "block_duration",
					Description: "Time period after a new game starts during which custom squad creation is blocked (in seconds).",
					Required:    false,
					Type:        plug_config_schema.FieldTypeInt,
					Default:     15,
				},
				{
					Name:        "broadcast_mode",
					Description: "If true, uses countdown broadcasts. If false, sends individual warnings to players.",
					Required:    false,
					Type:        plug_config_schema.FieldTypeBool,
					Default:     false,
				},
				{
					Name:        "allow_default_squad_names",
					Description: "If true, allows creation of squads with default names (e.g., \"Squad 1\") during the blocking period.",
					Required:    false,
					Type:        plug_config_schema.FieldTypeBool,
					Default:     true,
				},
				{
					Name:        "enable_rate_limiting",
					Description: "Enable anti-spam rate limiting for squad creation attempts.",
					Required:    false,
					Type:        plug_config_schema.FieldTypeBool,
					Default:     true,
				},
				{
					Name:        "rate_limiting_scope",
					Description: "When to apply rate limiting: \"blocking_period_only\" or \"entire_match\".",
					Required:    false,
					Type:        plug_config_schema.FieldTypeString,
					Default:     "blocking_period_only",
				},
				{
					Name:        "warning_threshold",
					Description: "Number of attempts before issuing warnings to the player.",
					Required:    false,
					Type:        plug_config_schema.FieldTypeInt,
					Default:     3,
				},
				{
					Name:        "cooldown_duration",
					Description: "Duration of cooldown period in seconds after exceeding warning threshold.",
					Required:    false,
					Type:        plug_config_schema.FieldTypeInt,
					Default:     10,
				},
				{
					Name:        "kick_threshold",
					Description: "Number of attempts before kicking the player (0 to disable).",
					Required:    false,
					Type:        plug_config_schema.FieldTypeInt,
					Default:     20,
				},
				{
					Name:        "poll_interval",
					Description: "Interval in seconds for periodic squad checking.",
					Required:    false,
					Type:        plug_config_schema.FieldTypeInt,
					Default:     1,
				},
				{
					Name:        "cooldown_warning_interval",
					Description: "Interval in seconds for warning players about remaining cooldown time.",
					Required:    false,
					Type:        plug_config_schema.FieldTypeInt,
					Default:     3,
				},
				{
					Name:        "reset_on_attempt",
					Description: "If true, cooldown timer resets on each new attempt. If false, cooldown must expire before new attempts trigger rate limiting.",
					Required:    false,
					Type:        plug_config_schema.FieldTypeBool,
					Default:     false,
				},
			},
		},

		Events: []event_manager.EventType{
			event_manager.EventTypeRconSquadCreated,
			event_manager.EventTypeLogGameEventUnified,
		},

		CreateInstance: func() plugin_manager.Plugin {
			return &SquadCreationBlockerPlugin{}
		},
	}
}

// GetDefinition returns the plugin definition
func (p *SquadCreationBlockerPlugin) GetDefinition() plugin_manager.PluginDefinition {
	return Define()
}

func (p *SquadCreationBlockerPlugin) GetCommands() []plugin_manager.PluginCommand {
	return []plugin_manager.PluginCommand{}
}

func (p *SquadCreationBlockerPlugin) ExecuteCommand(commandID string, params map[string]interface{}) (*plugin_manager.CommandResult, error) {
	return nil, fmt.Errorf("no commands available")
}

func (p *SquadCreationBlockerPlugin) GetCommandExecutionStatus(executionID string) (*plugin_manager.CommandExecutionStatus, error) {
	return nil, fmt.Errorf("no commands available")
}

// Initialize initializes the plugin with its configuration and dependencies
func (p *SquadCreationBlockerPlugin) Initialize(config map[string]interface{}, apis *plugin_manager.PluginAPIs) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.config = config
	p.apis = apis
	p.status = plugin_manager.PluginStatusStopped

	// Initialize data structures
	p.playerAttempts = make(map[string]*PlayerAttemptData)
	p.knownSquads = make(map[string]bool)
	p.isBlocking = false
	p.isRoundEnding = false

	// Validate config
	definition := p.GetDefinition()
	if err := definition.ConfigSchema.Validate(config); err != nil {
		return fmt.Errorf("invalid config: %w", err)
	}

	// Fill defaults
	definition.ConfigSchema.FillDefaults(config)

	return nil
}

// Start begins plugin execution
func (p *SquadCreationBlockerPlugin) Start(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.status == plugin_manager.PluginStatusRunning {
		return nil // Already running
	}

	p.ctx, p.cancel = context.WithCancel(ctx)
	p.status = plugin_manager.PluginStatusRunning

	// Start polling if rate limiting is enabled for entire match
	if p.getBoolConfig("enable_rate_limiting") && p.getStringConfig("rate_limiting_scope") == "entire_match" {
		p.startPollingLocked()
	}

	p.apis.LogAPI.Info("Squad Creation Blocker plugin started", nil)

	return nil
}

// Stop gracefully stops the plugin
func (p *SquadCreationBlockerPlugin) Stop() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.status == plugin_manager.PluginStatusStopped {
		return nil // Already stopped
	}

	p.status = plugin_manager.PluginStatusStopping

	// Stop polling
	p.stopPollingLocked()

	// Clear all cooldown warnings
	p.clearAllCooldownWarningsLocked()

	// Stop broadcast timer
	if p.broadcastTimer != nil {
		p.broadcastTimer.Stop()
		p.broadcastTimer = nil
	}

	if p.cancel != nil {
		p.cancel()
	}

	p.status = plugin_manager.PluginStatusStopped

	p.apis.LogAPI.Info("Squad Creation Blocker plugin stopped", nil)

	return nil
}

// HandleEvent processes an event if the plugin is subscribed to it
func (p *SquadCreationBlockerPlugin) HandleEvent(event *plugin_manager.PluginEvent) error {
	switch event.Type {
	case string(event_manager.EventTypeRconSquadCreated):
		return p.handleSquadCreated(event)
	case string(event_manager.EventTypeLogGameEventUnified):
		return p.handleGameEvent(event)
	}
	return nil
}

// GetStatus returns the current plugin status
func (p *SquadCreationBlockerPlugin) GetStatus() plugin_manager.PluginStatus {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.status
}

// GetConfig returns the current plugin configuration
func (p *SquadCreationBlockerPlugin) GetConfig() map[string]interface{} {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.config
}

// UpdateConfig updates the plugin configuration
func (p *SquadCreationBlockerPlugin) UpdateConfig(config map[string]interface{}) error {
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

	p.apis.LogAPI.Info("Squad Creation Blocker plugin configuration updated", nil)

	return nil
}

// handleGameEvent processes unified game events (NEW_GAME, ROUND_ENDED)
func (p *SquadCreationBlockerPlugin) handleGameEvent(rawEvent *plugin_manager.PluginEvent) error {
	event, ok := rawEvent.Data.(*event_manager.LogGameEventUnifiedData)
	if !ok {
		return fmt.Errorf("invalid event data type")
	}

	switch event.EventType {
	case "NEW_GAME":
		return p.handleNewGame()
	case "ROUND_ENDED":
		return p.handleRoundEnd()
	}

	return nil
}

// handleNewGame processes NEW_GAME events
func (p *SquadCreationBlockerPlugin) handleNewGame() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	blockDuration := p.getIntConfig("block_duration")
	p.isBlocking = true
	p.isRoundEnding = false
	p.blockEndTime = time.Now().Add(time.Duration(blockDuration) * time.Second)

	// Reset rate limiting data if scope is blocking period only
	rateLimitingScope := p.getStringConfig("rate_limiting_scope")
	if rateLimitingScope == "blocking_period_only" {
		p.resetRateLimitingDataLocked()
	}

	// Initialize known squads
	go p.initializeKnownSquads()

	// Start broadcasting if enabled
	if p.getBoolConfig("broadcast_mode") {
		p.scheduleBroadcastsLocked()
	}

	// Start polling if rate limiting is enabled and scope is blocking period only
	if p.getBoolConfig("enable_rate_limiting") && rateLimitingScope == "blocking_period_only" {
		p.startPollingLocked()
	}

	// Schedule the unblock
	p.broadcastTimer = time.AfterFunc(time.Duration(blockDuration)*time.Second, func() {
		p.mu.Lock()
		p.isBlocking = false
		p.mu.Unlock()

		if err := p.apis.RconAPI.Broadcast("Custom squad creation is now unlocked!"); err != nil {
			p.apis.LogAPI.Error("Failed to send unlock broadcast", err, nil)
		}

		// Stop polling if scope is blocking period only
		if p.getBoolConfig("enable_rate_limiting") && p.getStringConfig("rate_limiting_scope") == "blocking_period_only" {
			p.mu.Lock()
			p.stopPollingLocked()
			p.mu.Unlock()
		}
	})

	p.apis.LogAPI.Info("New game started - blocking custom squad creation", map[string]interface{}{
		"block_duration": blockDuration,
	})

	return nil
}

// handleRoundEnd processes ROUND_ENDED events
func (p *SquadCreationBlockerPlugin) handleRoundEnd() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.isBlocking = true
	p.isRoundEnding = true

	// Stop broadcast timer if running
	if p.broadcastTimer != nil {
		p.broadcastTimer.Stop()
		p.broadcastTimer = nil
	}

	// Reset rate limiting data if scope is blocking period only
	if p.getStringConfig("rate_limiting_scope") == "blocking_period_only" {
		p.resetRateLimitingDataLocked()
	}

	p.apis.LogAPI.Info("Round ended - blocking custom squad creation", nil)

	return nil
}

// handleSquadCreated processes SQUAD_CREATED events
func (p *SquadCreationBlockerPlugin) handleSquadCreated(rawEvent *plugin_manager.PluginEvent) error {
	event, ok := rawEvent.Data.(*event_manager.RconSquadCreatedData)
	if !ok {
		return fmt.Errorf("invalid event data type")
	}

	p.mu.Lock()
	steamID := event.SteamID
	squadName := event.SquadName
	shouldBlock := p.isBlocking || (p.shouldApplyRateLimitLocked() && p.isPlayerInCooldownLocked(steamID))
	allowDefault := p.getBoolConfig("allow_default_squad_names")
	isRoundEnding := p.isRoundEnding
	blockEndTime := p.blockEndTime
	broadcastMode := p.getBoolConfig("broadcast_mode")
	p.mu.Unlock()

	if !shouldBlock {
		return nil
	}

	// Allow default squad names if the option is enabled
	if allowDefault && p.isDefaultSquadName(squadName) {
		return nil
	}

	// Disband the squad
	// Parse squad ID from event
	squadIDStr := event.SquadID
	squadID, err := strconv.Atoi(squadIDStr)
	if err != nil {
		p.apis.LogAPI.Error("Failed to parse squad ID", err, map[string]interface{}{
			"squad_id_str": squadIDStr,
		})
		return err
	}

	teamName := event.TeamName
	teamID := p.getTeamIDFromName(teamName)

	if err := p.disbandSquad(teamID, squadID); err != nil {
		p.apis.LogAPI.Error("Failed to disband squad", err, map[string]interface{}{
			"squad_name": squadName,
			"squad_id":   squadID,
			"team_id":    teamID,
			"steam_id":   steamID,
		})
		return err
	}

	// Apply rate limiting or send standard message
	p.mu.Lock()
	shouldApplyRateLimit := p.shouldApplyRateLimitLocked()
	p.mu.Unlock()

	if shouldApplyRateLimit {
		return p.processRateLimit(steamID, squadName)
	}

	// Standard blocking period message
	if isRoundEnding {
		if err := p.apis.RconAPI.SendWarningToPlayer(steamID, "You are not allowed to create a custom squad at the end of a round."); err != nil {
			return fmt.Errorf("failed to send warning: %w", err)
		}
	} else if !broadcastMode {
		timeLeft := int(time.Until(blockEndTime).Seconds())
		if timeLeft < 0 {
			timeLeft = 0
		}
		message := fmt.Sprintf("Please wait for %d second%s before creating a custom squad. Default names (e.g. \"Squad 1\") are allowed.", timeLeft, pluralize(timeLeft))
		if err := p.apis.RconAPI.SendWarningToPlayer(steamID, message); err != nil {
			return fmt.Errorf("failed to send warning: %w", err)
		}
	}

	p.apis.LogAPI.Debug("Disbanded custom squad during blocking period", map[string]interface{}{
		"player_name": event.PlayerName,
		"steam_id":    steamID,
		"squad_name":  squadName,
	})

	return nil
}

// processRateLimit handles rate limiting logic for a player
func (p *SquadCreationBlockerPlugin) processRateLimit(steamID string, squadName string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Get or create player attempt data
	attemptData, exists := p.playerAttempts[steamID]
	if !exists {
		attemptData = &PlayerAttemptData{
			AttemptCount: 0,
		}
		p.playerAttempts[steamID] = attemptData
	}

	// Increment attempt counter
	attemptData.AttemptCount++
	currentAttempts := attemptData.AttemptCount

	kickThreshold := p.getIntConfig("kick_threshold")
	warningThreshold := p.getIntConfig("warning_threshold")
	cooldownDuration := p.getIntConfig("cooldown_duration")
	resetOnAttempt := p.getBoolConfig("reset_on_attempt")

	// Check for kick threshold
	if kickThreshold > 0 && currentAttempts >= kickThreshold {
		if err := p.apis.RconAPI.KickPlayer(steamID, "Excessive squad creation spam"); err != nil {
			p.apis.LogAPI.Error("Failed to kick player for spam", err, map[string]interface{}{
				"steam_id": steamID,
			})
		} else {
			p.apis.LogAPI.Info("Kicked player for excessive squad creation spam", map[string]interface{}{
				"steam_id": steamID,
				"attempts": currentAttempts,
			})
		}
		p.resetPlayerDataLocked(steamID)
		return nil
	}

	// Check if player should be put in cooldown
	if currentAttempts > warningThreshold {
		cooldownEndTime := time.Now().Add(time.Duration(cooldownDuration) * time.Second)

		// Only set/reset cooldown if resetOnAttempt is true, or if player is not currently in cooldown
		if resetOnAttempt || !p.isPlayerInCooldownLocked(steamID) {
			attemptData.CooldownEnd = cooldownEndTime

			message := fmt.Sprintf("You are on cooldown for %ds due to squad creation spam. Stop spamming or you will be kicked!", cooldownDuration)
			if err := p.apis.RconAPI.SendWarningToPlayer(steamID, message); err != nil {
				return fmt.Errorf("failed to send cooldown warning: %w", err)
			}

			p.startCooldownWarningLocked(steamID)
		}
	} else {
		// Send warning about approaching cooldown
		remaining := warningThreshold - currentAttempts + 1
		message := fmt.Sprintf("Warning: Stop spamming squad creation! %d more attempt%s before cooldown.", remaining, pluralize(remaining))
		if err := p.apis.RconAPI.SendWarningToPlayer(steamID, message); err != nil {
			return fmt.Errorf("failed to send warning: %w", err)
		}
	}

	return nil
}

// startCooldownWarningLocked starts a periodic warning for a player in cooldown
func (p *SquadCreationBlockerPlugin) startCooldownWarningLocked(steamID string) {
	// Clear existing warning if any
	p.clearCooldownWarningLocked(steamID)

	attemptData := p.playerAttempts[steamID]
	if attemptData == nil {
		return
	}

	cooldownWarningInterval := p.getIntConfig("cooldown_warning_interval")
	warningCtx, warningCancel := context.WithCancel(p.ctx)
	attemptData.CancelWarning = warningCancel

	// Start warning goroutine
	go func() {
		ticker := time.NewTicker(time.Duration(cooldownWarningInterval) * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-warningCtx.Done():
				return
			case <-ticker.C:
				p.mu.Lock()
				data, exists := p.playerAttempts[steamID]
				if !exists {
					p.mu.Unlock()
					return
				}

				timeLeft := int(time.Until(data.CooldownEnd).Seconds())
				if timeLeft <= 0 {
					// Cooldown expired
					data.CooldownEnd = time.Time{}
					p.clearCooldownWarningLocked(steamID)
					p.mu.Unlock()

					if err := p.apis.RconAPI.SendWarningToPlayer(steamID, "Squad creation cooldown has expired."); err != nil {
						p.apis.LogAPI.Error("Failed to send cooldown expiry warning", err, map[string]interface{}{
							"steam_id": steamID,
						})
					}
					return
				}

				p.mu.Unlock()

				message := fmt.Sprintf("Squad creation cooldown: %d second%s remaining.", timeLeft, pluralize(timeLeft))
				if err := p.apis.RconAPI.SendWarningToPlayer(steamID, message); err != nil {
					p.apis.LogAPI.Error("Failed to send cooldown reminder", err, map[string]interface{}{
						"steam_id": steamID,
					})
				}
			}
		}
	}()
}

// clearCooldownWarningLocked stops the cooldown warning for a player (must be called with lock held)
func (p *SquadCreationBlockerPlugin) clearCooldownWarningLocked(steamID string) {
	attemptData := p.playerAttempts[steamID]
	if attemptData != nil && attemptData.CancelWarning != nil {
		attemptData.CancelWarning()
		attemptData.CancelWarning = nil
	}
}

// clearAllCooldownWarningsLocked clears all cooldown warnings (must be called with lock held)
func (p *SquadCreationBlockerPlugin) clearAllCooldownWarningsLocked() {
	for steamID := range p.playerAttempts {
		p.clearCooldownWarningLocked(steamID)
	}
}

// resetPlayerDataLocked resets rate limiting data for a player (must be called with lock held)
func (p *SquadCreationBlockerPlugin) resetPlayerDataLocked(steamID string) {
	p.clearCooldownWarningLocked(steamID)
	delete(p.playerAttempts, steamID)
}

// resetRateLimitingDataLocked resets all rate limiting data (must be called with lock held)
func (p *SquadCreationBlockerPlugin) resetRateLimitingDataLocked() {
	p.clearAllCooldownWarningsLocked()
	p.playerAttempts = make(map[string]*PlayerAttemptData)
}

// isPlayerInCooldownLocked checks if a player is in cooldown (must be called with lock held)
func (p *SquadCreationBlockerPlugin) isPlayerInCooldownLocked(steamID string) bool {
	attemptData, exists := p.playerAttempts[steamID]
	if !exists {
		return false
	}

	if !attemptData.CooldownEnd.IsZero() && time.Now().Before(attemptData.CooldownEnd) {
		return true
	}

	// Cooldown expired, clean up
	if !attemptData.CooldownEnd.IsZero() {
		attemptData.CooldownEnd = time.Time{}
		p.clearCooldownWarningLocked(steamID)
	}

	return false
}

// shouldApplyRateLimitLocked checks if rate limiting should be applied (must be called with lock held)
func (p *SquadCreationBlockerPlugin) shouldApplyRateLimitLocked() bool {
	if !p.getBoolConfig("enable_rate_limiting") {
		return false
	}

	scope := p.getStringConfig("rate_limiting_scope")
	if scope == "entire_match" {
		return true
	}
	if scope == "blocking_period_only" {
		return p.isBlocking
	}

	return false
}

// initializeKnownSquads initializes the known squads set
func (p *SquadCreationBlockerPlugin) initializeKnownSquads() {
	squads, err := p.apis.ServerAPI.GetSquads()
	if err != nil {
		p.apis.LogAPI.Error("Failed to initialize known squads", err, nil)
		return
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	p.knownSquads = make(map[string]bool)
	for _, squad := range squads {
		key := fmt.Sprintf("%d-%d", squad.TeamID, squad.ID)
		p.knownSquads[key] = true
	}

	p.apis.LogAPI.Debug("Initialized known squads", map[string]interface{}{
		"count": len(p.knownSquads),
	})
}

// pollSquads periodically checks for new squads
func (p *SquadCreationBlockerPlugin) pollSquads() {
	p.mu.Lock()
	shouldApply := p.shouldApplyRateLimitLocked()
	p.mu.Unlock()

	if !shouldApply {
		return
	}

	squads, err := p.apis.ServerAPI.GetSquads()
	if err != nil {
		p.apis.LogAPI.Error("Failed to poll squads", err, nil)
		return
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	allowDefault := p.getBoolConfig("allow_default_squad_names")

	for _, squad := range squads {
		squadKey := fmt.Sprintf("%d-%d", squad.TeamID, squad.ID)

		// Skip if this squad was already known
		if p.knownSquads[squadKey] {
			continue
		}

		// New squad detected
		p.knownSquads[squadKey] = true

		// Skip if it's a default squad name and default names are allowed
		if allowDefault && p.isDefaultSquadName(squad.Name) {
			continue
		}

		// Get creator's Steam ID from squad leader
		if squad.Leader == nil {
			continue
		}

		creatorSteamID := squad.Leader.SteamID

		// Check if creator is in cooldown or if we're in blocking period
		inCooldown := p.isPlayerInCooldownLocked(creatorSteamID)
		if inCooldown || p.isBlocking {
			// Don't disband if it's a default squad name and default names are allowed
			if allowDefault && p.isDefaultSquadName(squad.Name) {
				continue
			}

			// Disband the squad
			p.mu.Unlock()
			if err := p.disbandSquad(squad.TeamID, squad.ID); err != nil {
				p.apis.LogAPI.Error("Failed to disband squad during poll", err, map[string]interface{}{
					"squad_name": squad.Name,
					"squad_id":   squad.ID,
					"team_id":    squad.TeamID,
				})
				p.mu.Lock()
				continue
			}
			p.mu.Lock()

			delete(p.knownSquads, squadKey) // Remove since we disbanded it

			// Process rate limiting
			p.mu.Unlock()
			_ = p.processRateLimit(creatorSteamID, squad.Name)
			p.mu.Lock()
		}
	}
}

// startPollingLocked starts the squad polling (must be called with lock held)
func (p *SquadCreationBlockerPlugin) startPollingLocked() {
	if p.pollTicker != nil {
		return // Already polling
	}

	pollInterval := p.getIntConfig("poll_interval")
	p.pollTicker = time.NewTicker(time.Duration(pollInterval) * time.Second)

	pollCtx, pollCancel := context.WithCancel(p.ctx)
	p.pollCancel = pollCancel

	go func() {
		for {
			select {
			case <-pollCtx.Done():
				return
			case <-p.pollTicker.C:
				p.pollSquads()
			}
		}
	}()

	p.apis.LogAPI.Debug("Started squad polling", map[string]interface{}{
		"poll_interval": pollInterval,
	})
}

// stopPollingLocked stops the squad polling (must be called with lock held)
func (p *SquadCreationBlockerPlugin) stopPollingLocked() {
	if p.pollTicker != nil {
		p.pollTicker.Stop()
		p.pollTicker = nil
	}
	if p.pollCancel != nil {
		p.pollCancel()
		p.pollCancel = nil
	}
}

// scheduleBroadcastsLocked schedules countdown broadcasts (must be called with lock held)
func (p *SquadCreationBlockerPlugin) scheduleBroadcastsLocked() {
	blockDuration := p.getIntConfig("block_duration")
	cooldownDuration := p.getIntConfig("cooldown_duration")

	// Schedule broadcasts at 10-second intervals
	for i := (blockDuration / 10) * 10; i > 0; i -= 10 {
		delay := time.Duration(blockDuration-i) * time.Second
		countdown := i

		time.AfterFunc(delay, func() {
			message := fmt.Sprintf("Custom squad names unlock in %ds. Default names (e.g. \"Squad 1\") are allowed. Spammers get %ds cooldown.", countdown, cooldownDuration)
			if err := p.apis.RconAPI.Broadcast(message); err != nil {
				p.apis.LogAPI.Error("Failed to send countdown broadcast", err, map[string]interface{}{
					"countdown": countdown,
				})
			}
		})
	}
}

// disbandSquad sends the command to disband a squad
func (p *SquadCreationBlockerPlugin) disbandSquad(teamID int, squadID int) error {
	command := fmt.Sprintf("AdminDisbandSquad %d %d", teamID, squadID)
	_, err := p.apis.RconAPI.SendCommand(command)
	return err
}

// isDefaultSquadName checks if a squad name is a default name like "Squad 1"
func (p *SquadCreationBlockerPlugin) isDefaultSquadName(squadName string) bool {
	// Check if the squad name matches the pattern "Squad X" or "squad X" where X is a number
	matched, _ := regexp.MatchString(`(?i)^squad \d+$`, squadName)
	return matched
}

// getTeamIDFromName converts team name to team ID (1 or 2)
func (p *SquadCreationBlockerPlugin) getTeamIDFromName(teamName string) int {
	// Team names are typically "Team 1" or "Team 2"
	// Try to extract the number
	re := regexp.MustCompile(`\d+`)
	match := re.FindString(teamName)
	if match != "" {
		if id, err := strconv.Atoi(match); err == nil {
			return id
		}
	}
	// Default to team 1 if we can't parse
	return 1
}

// Helper methods for config access

func (p *SquadCreationBlockerPlugin) getStringConfig(key string) string {
	if value, ok := p.config[key].(string); ok {
		return value
	}
	return ""
}

func (p *SquadCreationBlockerPlugin) getIntConfig(key string) int {
	if value, ok := p.config[key].(int); ok {
		return value
	}
	if value, ok := p.config[key].(float64); ok {
		return int(value)
	}
	return 0
}

func (p *SquadCreationBlockerPlugin) getBoolConfig(key string) bool {
	if value, ok := p.config[key].(bool); ok {
		return value
	}
	return false
}

// pluralize returns "s" if count is not 1, otherwise empty string
func pluralize(count int) string {
	if count == 1 {
		return ""
	}
	return "s"
}
