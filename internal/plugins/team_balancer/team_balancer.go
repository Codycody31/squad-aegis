package team_balancer

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.codycody31.dev/squad-aegis/internal/event_manager"
	"go.codycody31.dev/squad-aegis/internal/plugin_manager"
	"go.codycody31.dev/squad-aegis/internal/plugins/team_balancer/scrambler"
	"go.codycody31.dev/squad-aegis/internal/plugins/team_balancer/swap_executor"
	"go.codycody31.dev/squad-aegis/internal/shared/plug_config_schema"
)

// TeamBalancerPlugin tracks win streaks and scrambles teams to maintain balance
type TeamBalancerPlugin struct {
	// Plugin configuration
	config map[string]interface{}
	apis   *plugin_manager.PluginAPIs

	// State management
	mu     sync.Mutex
	status plugin_manager.PluginStatus
	ctx    context.Context
	cancel context.CancelFunc

	// Win streak tracking
	winStreakTeam     int
	winStreakCount    int
	manuallyDisabled  bool
	lastScrambleTime  time.Time
	lastSyncTimestamp time.Time

	// Scramble execution
	scramblePending    bool
	scrambleInProgress bool
	scrambleTimeout    *time.Timer
	scrambleCountdown  *time.Timer

	// Utilities
	swapExecutor *swap_executor.SwapExecutor

	// Game mode cache
	gameModeCached    string
	cachedTeamNames   map[int]string
	teamNamePollTimer *time.Timer
}

// WinStreakState represents the persistent win streak state
type WinStreakState struct {
	WinStreakTeam     int       `json:"win_streak_team"`
	WinStreakCount    int       `json:"win_streak_count"`
	LastSyncTimestamp time.Time `json:"last_sync_timestamp"`
}

// Define returns the plugin definition
func Define() plugin_manager.PluginDefinition {
	return plugin_manager.PluginDefinition{
		ID:                     "team_balancer",
		Name:                   "Team Balancer",
		Description:            "Tracks dominant win streaks and triggers fair, squad-preserving team scrambles to maintain balanced matches.",
		Version:                "2.0.0",
		Author:                 "Squad Aegis (ported from Slacker's SquadJS plugin)",
		AllowMultipleInstances: false,
		RequiredConnectors:     []string{},
		LongRunning:            false,

		ConfigSchema: plug_config_schema.ConfigSchema{
			Fields: []plug_config_schema.ConfigField{
				// Core Settings
				{
					Name:        "enable_win_streak_tracking",
					Description: "Enable/disable automatic win streak tracking",
					Required:    false,
					Type:        plug_config_schema.FieldTypeBool,
					Default:     true,
				},
				{
					Name:        "max_win_streak",
					Description: "Number of dominant wins to trigger a scramble",
					Required:    false,
					Type:        plug_config_schema.FieldTypeInt,
					Default:     2,
				},
				{
					Name:        "enable_single_round_scramble",
					Description: "Enable scramble if a single round ticket margin is huge (mercy rule)",
					Required:    false,
					Type:        plug_config_schema.FieldTypeBool,
					Default:     false,
				},
				{
					Name:        "single_round_scramble_threshold",
					Description: "Ticket margin to trigger single-round scramble",
					Required:    false,
					Type:        plug_config_schema.FieldTypeInt,
					Default:     250,
				},
				{
					Name:        "min_tickets_dominant_win",
					Description: "Min ticket diff for a dominant win (Standard modes)",
					Required:    false,
					Type:        plug_config_schema.FieldTypeInt,
					Default:     150,
				},
				{
					Name:        "invasion_attack_threshold",
					Description: "Ticket diff for Attackers to be dominant (Invasion)",
					Required:    false,
					Type:        plug_config_schema.FieldTypeInt,
					Default:     300,
				},
				{
					Name:        "invasion_defence_threshold",
					Description: "Ticket diff for Defenders to be dominant (Invasion)",
					Required:    false,
					Type:        plug_config_schema.FieldTypeInt,
					Default:     650,
				},

				// Scramble Execution
				{
					Name:        "scramble_announcement_delay",
					Description: "Seconds before scramble executes after announcement (min: 10)",
					Required:    false,
					Type:        plug_config_schema.FieldTypeInt,
					Default:     30,
				},
				{
					Name:        "scramble_percentage",
					Description: "Percentage of players to move (0.0 - 1.0)",
					Required:    false,
					Type:        plug_config_schema.FieldTypeString,
					Default:     "0.5",
				},
				{
					Name:        "change_team_retry_interval",
					Description: "Retry interval (ms) for player swaps (min: 200)",
					Required:    false,
					Type:        plug_config_schema.FieldTypeInt,
					Default:     200,
				},
				{
					Name:        "max_scramble_completion_time",
					Description: "Max time (ms) for all swaps to complete (min: 5000)",
					Required:    false,
					Type:        plug_config_schema.FieldTypeInt,
					Default:     15000,
				},
				{
					Name:        "warn_on_swap",
					Description: "Send warning to players when swapped",
					Required:    false,
					Type:        plug_config_schema.FieldTypeBool,
					Default:     true,
				},

				// Messaging & Display
				{
					Name:        "show_win_streak_messages",
					Description: "Broadcast win streak messages",
					Required:    false,
					Type:        plug_config_schema.FieldTypeBool,
					Default:     true,
				},
				{
					Name:        "use_generic_team_names",
					Description: "Use 'Team 1'/'Team 2' instead of faction names",
					Required:    false,
					Type:        plug_config_schema.FieldTypeBool,
					Default:     false,
				},
				{
					Name:        "message_prefix",
					Description: "Prefix for all broadcast messages",
					Required:    false,
					Type:        plug_config_schema.FieldTypeString,
					Default:     ">>> ",
				},
			},
		},

		Events: []event_manager.EventType{
			event_manager.EventTypeLogGameEventUnified,
			event_manager.EventTypeRconChatMessage,
		},

		CreateInstance: func() plugin_manager.Plugin {
			return &TeamBalancerPlugin{}
		},
	}
}

// GetDefinition returns the plugin definition
func (p *TeamBalancerPlugin) GetDefinition() plugin_manager.PluginDefinition {
	return Define()
}

func (p *TeamBalancerPlugin) GetCommands() []plugin_manager.PluginCommand {
	return []plugin_manager.PluginCommand{
		{
			ID:            "status",
			Name:          "Get Status",
			Description:   "Show current team balancer status and win streak",
			Category:      "Team Management",
			ExecutionType: plugin_manager.CommandExecutionSync,
		},
		{
			ID:                  "enable",
			Name:                "Enable Tracking",
			Description:         "Enable automatic win streak tracking",
			Category:            "Team Management",
			ExecutionType:       plugin_manager.CommandExecutionSync,
			RequiredPermissions: []string{"manageserver"},
		},
		{
			ID:                  "disable",
			Name:                "Disable Tracking",
			Description:         "Disable automatic win streak tracking",
			Category:            "Team Management",
			ExecutionType:       plugin_manager.CommandExecutionSync,
			RequiredPermissions: []string{"manageserver"},
		},
		{
			ID:                  "scramble",
			Name:                "Scramble Teams",
			Description:         "Initiate a team scramble",
			Category:            "Team Management",
			ExecutionType:       plugin_manager.CommandExecutionSync,
			RequiredPermissions: []string{"manageserver"},
			Parameters: plug_config_schema.ConfigSchema{
				Fields: []plug_config_schema.ConfigField{
					{
						Name:        "immediate",
						Type:        plug_config_schema.FieldTypeBool,
						Description: "Execute scramble immediately without countdown",
						Default:     false,
					},
					{
						Name:        "dry_run",
						Type:        plug_config_schema.FieldTypeBool,
						Description: "Run a simulated scramble without moving players",
						Default:     false,
					},
				},
			},
		},
		{
			ID:                  "cancel_scramble",
			Name:                "Cancel Scramble",
			Description:         "Cancel a pending team scramble",
			Category:            "Team Management",
			ExecutionType:       plugin_manager.CommandExecutionSync,
			RequiredPermissions: []string{"manageserver"},
		},
		{
			ID:                  "diag",
			Name:                "Run Diagnostics",
			Description:         "Run team balancer diagnostics (dry-run scramble)",
			Category:            "Team Management",
			ExecutionType:       plugin_manager.CommandExecutionSync,
			RequiredPermissions: []string{"manageserver"},
		},
	}
}

func (p *TeamBalancerPlugin) ExecuteCommand(commandID string, params map[string]interface{}) (*plugin_manager.CommandResult, error) {
	switch commandID {
	case "status":
		p.mu.Lock()
		defer p.mu.Unlock()

		statusStr := "enabled"
		if p.manuallyDisabled {
			statusStr = "disabled"
		}

		msg := fmt.Sprintf("Team Balancer Status: %s | Win Streak: Team %d (%d wins) | Max: %d",
			statusStr, p.winStreakTeam, p.winStreakCount, p.getIntConfig("max_win_streak"))

		return &plugin_manager.CommandResult{
			Success: true,
			Message: msg,
			Data: map[string]interface{}{
				"enabled":          !p.manuallyDisabled,
				"win_streak_team":  p.winStreakTeam,
				"win_streak_count": p.winStreakCount,
				"max_win_streak":   p.getIntConfig("max_win_streak"),
			},
		}, nil

	case "enable":
		p.mu.Lock()
		if !p.manuallyDisabled {
			p.mu.Unlock()
			return &plugin_manager.CommandResult{
				Success: true,
				Message: "Win streak tracking is already enabled.",
			}, nil
		}
		p.manuallyDisabled = false
		p.mu.Unlock()

		p.broadcast(p.getMessage("tracking_enabled"))
		return &plugin_manager.CommandResult{
			Success: true,
			Message: "Win streak tracking enabled.",
		}, nil

	case "disable":
		p.mu.Lock()
		if p.manuallyDisabled {
			p.mu.Unlock()
			return &plugin_manager.CommandResult{
				Success: true,
				Message: "Win streak tracking is already disabled.",
			}, nil
		}
		p.manuallyDisabled = true
		p.resetStreak("Manual disable")
		p.mu.Unlock()

		p.broadcast(p.getMessage("tracking_disabled"))
		return &plugin_manager.CommandResult{
			Success: true,
			Message: "Win streak tracking disabled.",
		}, nil

	case "scramble":
		immediate := false
		if val, ok := params["immediate"].(bool); ok {
			immediate = val
		}
		dryRun := false
		if val, ok := params["dry_run"].(bool); ok {
			dryRun = val
		}

		p.mu.Lock()
		if p.scramblePending || p.scrambleInProgress {
			p.mu.Unlock()
			status := "pending"
			if p.scrambleInProgress {
				status = "executing"
			}
			return &plugin_manager.CommandResult{
				Success: false,
				Message: fmt.Sprintf("Scramble already %s. Use cancel_scramble command to cancel.", status),
			}, nil
		}
		p.mu.Unlock()

		if !dryRun {
			msg := ""
			if immediate {
				msg = p.getMessage("immediate_manual_scramble")
			} else {
				msg = p.formatMessage(p.getMessage("manual_scramble_announcement"), map[string]interface{}{
					"delay": p.getIntConfig("scramble_announcement_delay"),
				})
			}
			p.broadcast(msg)
		}

		go p.initiateScramble(dryRun, dryRun || immediate)
		return &plugin_manager.CommandResult{
			Success: true,
			Message: "Scramble initiated.",
		}, nil

	case "cancel_scramble":
		p.mu.Lock()
		if !p.scramblePending {
			p.mu.Unlock()
			if p.scrambleInProgress {
				return &plugin_manager.CommandResult{
					Success: false,
					Message: "Cannot cancel - scramble is already executing.",
				}, nil
			}
			return &plugin_manager.CommandResult{
				Success: false,
				Message: "No pending scramble to cancel.",
			}, nil
		}

		if p.scrambleCountdown != nil {
			p.scrambleCountdown.Stop()
		}
		p.scramblePending = false
		p.mu.Unlock()

		p.broadcast("Scramble cancelled by admin.")
		return &plugin_manager.CommandResult{
			Success: true,
			Message: "Scramble cancelled.",
		}, nil

	case "diag":
		go func() {
			time.Sleep(1 * time.Second)
			p.initiateScramble(true, true)
		}()
		return &plugin_manager.CommandResult{
			Success: true,
			Message: "Diagnostics started. Check logs for results.",
		}, nil

	default:
		return nil, fmt.Errorf("unknown command: %s", commandID)
	}
}

func (p *TeamBalancerPlugin) GetCommandExecutionStatus(executionID string) (*plugin_manager.CommandExecutionStatus, error) {
	return nil, fmt.Errorf("no commands available")
}

// Initialize initializes the plugin with its configuration and dependencies
func (p *TeamBalancerPlugin) Initialize(config map[string]interface{}, apis *plugin_manager.PluginAPIs) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.config = config
	p.apis = apis
	p.status = plugin_manager.PluginStatusStopped
	p.cachedTeamNames = make(map[int]string)

	// Validate config
	definition := p.GetDefinition()
	if err := definition.ConfigSchema.Validate(config); err != nil {
		return fmt.Errorf("invalid config: %w", err)
	}

	// Fill defaults
	definition.ConfigSchema.FillDefaults(config)

	// Validate and correct config values
	p.validateAndCorrectConfig()

	// Initialize swap executor
	executorConfig := swap_executor.ExecutorConfig{
		RetryInterval:        time.Duration(p.getIntConfig("change_team_retry_interval")) * time.Millisecond,
		MaxCompletionTime:    time.Duration(p.getIntConfig("max_scramble_completion_time")) * time.Millisecond,
		WarnOnSwap:           p.getBoolConfig("warn_on_swap"),
		MaxAttemptsPerPlayer: 5,
	}
	p.swapExecutor = swap_executor.New(executorConfig, apis.RconAPI, apis.LogAPI)

	// Load state from database
	if err := p.loadState(); err != nil {
		p.apis.LogAPI.Warn("Failed to load state from database, starting fresh", map[string]interface{}{
			"error": err.Error(),
		})
	}

	p.status = plugin_manager.PluginStatusStopped

	return nil
}

// Start begins plugin execution
func (p *TeamBalancerPlugin) Start(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.status == plugin_manager.PluginStatusRunning {
		return nil // Already running
	}

	p.ctx, p.cancel = context.WithCancel(ctx)
	p.status = plugin_manager.PluginStatusRunning

	// Start team name polling
	p.startTeamNamePolling()

	p.apis.LogAPI.Info("Team Balancer plugin started", map[string]interface{}{
		"win_streak_team":  p.winStreakTeam,
		"win_streak_count": p.winStreakCount,
	})

	return nil
}

// Stop gracefully stops the plugin
func (p *TeamBalancerPlugin) Stop() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.status == plugin_manager.PluginStatusStopped {
		return nil // Already stopped
	}

	p.status = plugin_manager.PluginStatusStopping

	// Cancel any pending scrambles
	if p.scrambleTimeout != nil {
		p.scrambleTimeout.Stop()
	}
	if p.scrambleCountdown != nil {
		p.scrambleCountdown.Stop()
	}
	if p.teamNamePollTimer != nil {
		p.teamNamePollTimer.Stop()
	}

	// Cancel context
	if p.cancel != nil {
		p.cancel()
	}

	// Cleanup swap executor
	if p.swapExecutor != nil {
		p.swapExecutor.Cleanup()
	}

	p.status = plugin_manager.PluginStatusStopped

	p.apis.LogAPI.Info("Team Balancer plugin stopped", nil)

	return nil
}

// HandleEvent processes an event if the plugin is subscribed to it
func (p *TeamBalancerPlugin) HandleEvent(event *plugin_manager.PluginEvent) error {
	switch event.Type {
	case string(event_manager.EventTypeLogGameEventUnified):
		return p.handleGameEvent(event)
	case string(event_manager.EventTypeRconChatMessage):
		return p.handleChatMessage(event)
	}
	return nil
}

// GetStatus returns the current plugin status
func (p *TeamBalancerPlugin) GetStatus() plugin_manager.PluginStatus {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.status
}

// GetConfig returns the current plugin configuration
func (p *TeamBalancerPlugin) GetConfig() map[string]interface{} {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.config
}

// UpdateConfig updates the plugin configuration
func (p *TeamBalancerPlugin) UpdateConfig(config map[string]interface{}) error {
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

	// Validate and correct
	p.validateAndCorrectConfig()

	p.apis.LogAPI.Info("Team Balancer plugin configuration updated", nil)

	return nil
}

// validateAndCorrectConfig validates and corrects configuration values
func (p *TeamBalancerPlugin) validateAndCorrectConfig() {
	// Enforce minimum scramble announcement delay
	if p.getIntConfig("scramble_announcement_delay") < 10 {
		p.apis.LogAPI.Warn("scramble_announcement_delay too low, enforcing minimum 10 seconds", map[string]interface{}{
			"configured": p.getIntConfig("scramble_announcement_delay"),
		})
		p.config["scramble_announcement_delay"] = 10
	}

	// Enforce minimum retry interval
	if p.getIntConfig("change_team_retry_interval") < 200 {
		p.apis.LogAPI.Warn("change_team_retry_interval too low, enforcing minimum 200ms", map[string]interface{}{
			"configured": p.getIntConfig("change_team_retry_interval"),
		})
		p.config["change_team_retry_interval"] = 200
	}

	// Enforce minimum completion time
	if p.getIntConfig("max_scramble_completion_time") < 5000 {
		p.apis.LogAPI.Warn("max_scramble_completion_time too low, enforcing minimum 5000ms", map[string]interface{}{
			"configured": p.getIntConfig("max_scramble_completion_time"),
		})
		p.config["max_scramble_completion_time"] = 5000
	}

	// Enforce scramble percentage range
	scramblePercentage := p.getFloatConfig("scramble_percentage")
	if scramblePercentage < 0.0 || scramblePercentage > 1.0 {
		p.apis.LogAPI.Warn("scramble_percentage out of range, enforcing 0.5", map[string]interface{}{
			"configured": scramblePercentage,
		})
		p.config["scramble_percentage"] = 0.5
	}

	// Ensure single round threshold > min tickets dominant
	singleRoundThreshold := p.getIntConfig("single_round_scramble_threshold")
	minTicketsDominant := p.getIntConfig("min_tickets_dominant_win")
	if singleRoundThreshold <= minTicketsDominant {
		newThreshold := minTicketsDominant + 50
		p.apis.LogAPI.Warn("single_round_scramble_threshold must be greater than min_tickets_dominant_win", map[string]interface{}{
			"configured":  singleRoundThreshold,
			"min_tickets": minTicketsDominant,
			"enforcing":   newThreshold,
		})
		p.config["single_round_scramble_threshold"] = newThreshold
	}
}

// Config helper methods
func (p *TeamBalancerPlugin) getStringConfig(key string) string {
	if value, ok := p.config[key].(string); ok {
		return value
	}
	return ""
}

func (p *TeamBalancerPlugin) getBoolConfig(key string) bool {
	if value, ok := p.config[key].(bool); ok {
		return value
	}
	return false
}

func (p *TeamBalancerPlugin) getIntConfig(key string) int {
	if value, ok := p.config[key].(int); ok {
		return value
	}
	if value, ok := p.config[key].(float64); ok {
		return int(value)
	}
	return 0
}

func (p *TeamBalancerPlugin) getFloatConfig(key string) float64 {
	if value, ok := p.config[key].(float64); ok {
		return value
	}
	if value, ok := p.config[key].(int); ok {
		return float64(value)
	}
	if value, ok := p.config[key].(string); ok {
		if f, err := strconv.ParseFloat(value, 64); err == nil {
			return f
		}
	}
	return 0.0
}

// State persistence methods
func (p *TeamBalancerPlugin) loadState() error {
	// Load win streak state
	stateJSON, err := p.apis.DatabaseAPI.GetPluginData("win_streak_state")
	if err != nil {
		// No state exists yet, start fresh
		return nil
	}

	var state WinStreakState
	if err := json.Unmarshal([]byte(stateJSON), &state); err != nil {
		return fmt.Errorf("failed to unmarshal state: %w", err)
	}

	// Check if state is stale (> 2 hours old)
	if time.Since(state.LastSyncTimestamp) > 2*time.Hour {
		p.apis.LogAPI.Info("Win streak state is stale, resetting", map[string]interface{}{
			"age": time.Since(state.LastSyncTimestamp).String(),
		})
		return nil
	}

	p.winStreakTeam = state.WinStreakTeam
	p.winStreakCount = state.WinStreakCount
	p.lastSyncTimestamp = state.LastSyncTimestamp

	// Load last scramble time
	scrambleTimeStr, err := p.apis.DatabaseAPI.GetPluginData("last_scramble_time")
	if err == nil {
		if scrambleTime, err := time.Parse(time.RFC3339, scrambleTimeStr); err == nil {
			p.lastScrambleTime = scrambleTime
		}
	}

	return nil
}

func (p *TeamBalancerPlugin) saveState() error {
	state := WinStreakState{
		WinStreakTeam:     p.winStreakTeam,
		WinStreakCount:    p.winStreakCount,
		LastSyncTimestamp: time.Now(),
	}

	stateJSON, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("failed to marshal state: %w", err)
	}

	if err := p.apis.DatabaseAPI.SetPluginData("win_streak_state", string(stateJSON)); err != nil {
		return fmt.Errorf("failed to save state: %w", err)
	}

	p.lastSyncTimestamp = state.LastSyncTimestamp

	return nil
}

func (p *TeamBalancerPlugin) saveScrambleTime() error {
	p.lastScrambleTime = time.Now()
	return p.apis.DatabaseAPI.SetPluginData("last_scramble_time", p.lastScrambleTime.Format(time.RFC3339))
}

func (p *TeamBalancerPlugin) resetStreak(reason string) error {
	p.apis.LogAPI.Debug("Resetting win streak", map[string]interface{}{
		"reason": reason,
	})

	p.winStreakTeam = 0
	p.winStreakCount = 0
	p.scramblePending = false

	return p.saveState()
}

// Team name extraction and caching
func (p *TeamBalancerPlugin) startTeamNamePolling() {
	// Poll every 5 seconds until we have both team names
	p.pollTeamNames()
}

func (p *TeamBalancerPlugin) pollTeamNames() {
	// Get players to extract team names from roles
	players, err := p.apis.ServerAPI.GetPlayers()
	if err != nil {
		// Retry in 5 seconds
		p.teamNamePollTimer = time.AfterFunc(5*time.Second, p.pollTeamNames)
		return
	}

	newNames := p.extractTeamNamesFromPlayers(players)
	for teamID, name := range newNames {
		p.cachedTeamNames[teamID] = name
	}

	// Stop polling if we have both teams
	if len(p.cachedTeamNames) >= 2 {
		p.apis.LogAPI.Debug("Team names cached", map[string]interface{}{
			"team_names": p.cachedTeamNames,
		})
		return
	}

	// Continue polling
	p.teamNamePollTimer = time.AfterFunc(5*time.Second, p.pollTeamNames)
}

func (p *TeamBalancerPlugin) extractTeamNamesFromPlayers(players []*plugin_manager.PlayerInfo) map[int]string {
	names := make(map[int]string)

	// Extract faction abbreviation from role (e.g., "USA_SquadLeader" -> "USA")
	rolePattern := regexp.MustCompile(`^([A-Z]{2,6})_`)

	for _, player := range players {
		if player.TeamID == 0 {
			continue
		}

		// Skip if we already have this team
		if _, exists := names[player.TeamID]; exists {
			continue
		}

		if player.Role != "" {
			matches := rolePattern.FindStringSubmatch(player.Role)
			if len(matches) > 1 {
				names[player.TeamID] = matches[1]
			}
		}
	}

	return names
}

func (p *TeamBalancerPlugin) getTeamName(teamID int) string {
	if p.getBoolConfig("use_generic_team_names") {
		return fmt.Sprintf("Team %d", teamID)
	}

	if name, exists := p.cachedTeamNames[teamID]; exists {
		return name
	}

	return fmt.Sprintf("Team %d", teamID)
}

func (p *TeamBalancerPlugin) formatTeamNameForBroadcast(teamID int) string {
	name := p.getTeamName(teamID)

	// Add "The" prefix if not using generic names and not already present
	if !p.getBoolConfig("use_generic_team_names") && !strings.HasPrefix(name, "The ") && !strings.HasPrefix(name, "Team ") {
		return "The " + name
	}

	return name
}

// Event Handlers

// handleGameEvent processes unified game events (ROUND_ENDED, NEW_GAME)
func (p *TeamBalancerPlugin) handleGameEvent(rawEvent *plugin_manager.PluginEvent) error {
	event, ok := rawEvent.Data.(*event_manager.LogGameEventUnifiedData)
	if !ok {
		return fmt.Errorf("invalid event data type")
	}

	switch event.EventType {
	case "ROUND_ENDED":
		return p.handleRoundEnded(event)
	case "NEW_GAME":
		return p.handleNewGame(event)
	}

	return nil
}

// handleRoundEnded processes round ended events
func (p *TeamBalancerPlugin) handleRoundEnded(event *event_manager.LogGameEventUnifiedData) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.getBoolConfig("enable_win_streak_tracking") || p.manuallyDisabled {
		p.apis.LogAPI.Debug("Win streak tracking disabled, skipping round evaluation", nil)
		return nil
	}

	// Parse winner data
	var winnerData, loserData map[string]interface{}
	if event.WinnerData != "" {
		json.Unmarshal([]byte(event.WinnerData), &winnerData)
	}
	if event.LoserData != "" {
		json.Unmarshal([]byte(event.LoserData), &loserData)
	}

	// Extract tickets
	winnerTickets := 0
	loserTickets := 0

	if winnerData != nil {
		if tickets, ok := winnerData["tickets"].(float64); ok {
			winnerTickets = int(tickets)
		} else if ticketsStr, ok := winnerData["tickets"].(string); ok {
			winnerTickets, _ = strconv.Atoi(ticketsStr)
		}
	}

	if loserData != nil {
		if tickets, ok := loserData["tickets"].(float64); ok {
			loserTickets = int(tickets)
		} else if ticketsStr, ok := loserData["tickets"].(string); ok {
			loserTickets, _ = strconv.Atoi(ticketsStr)
		}
	}

	// Determine winner team ID
	winnerID := 0
	if event.Winner != "" {
		// Winner might be "1" or "2" or faction name
		if id, err := strconv.Atoi(event.Winner); err == nil {
			winnerID = id
		} else {
			// Try to match faction name to team
			for teamID, name := range p.cachedTeamNames {
				if strings.Contains(strings.ToLower(event.Winner), strings.ToLower(name)) {
					winnerID = teamID
					break
				}
			}
		}
	}

	if winnerID == 0 {
		p.apis.LogAPI.Warn("Could not determine winner team ID", map[string]interface{}{
			"winner": event.Winner,
		})
		return nil
	}

	margin := winnerTickets - loserTickets

	p.apis.LogAPI.Debug("Round ended", map[string]interface{}{
		"winner":         winnerID,
		"winner_tickets": winnerTickets,
		"loser_tickets":  loserTickets,
		"margin":         margin,
	})

	// Check for single round scramble (mercy rule)
	isInvasion := strings.Contains(strings.ToLower(p.gameModeCached), "invasion")
	if p.getBoolConfig("enable_single_round_scramble") && !isInvasion {
		threshold := p.getIntConfig("single_round_scramble_threshold")
		if margin >= threshold {
			p.apis.LogAPI.Info("Single round scramble triggered", map[string]interface{}{
				"margin":    margin,
				"threshold": threshold,
			})

			msg := p.formatMessage(p.getMessage("single_round_scramble"), map[string]interface{}{
				"margin": margin,
				"delay":  p.getIntConfig("scramble_announcement_delay"),
			})
			p.broadcast(msg)

			go p.initiateScramble(false, false)
			return nil
		}
	}

	// Determine if this was a dominant win
	isDominant := p.isDominantWin(winnerID, margin, isInvasion)

	if !isDominant {
		// Non-dominant win - broadcast message and reset streak
		if p.getBoolConfig("show_win_streak_messages") {
			msg := p.getNonDominantWinMessage(winnerID, margin, isInvasion)
			p.broadcast(msg)
		}
		p.resetStreak("Non-dominant win")
		return nil
	}

	// Dominant win - update streak
	if p.winStreakTeam == winnerID {
		p.winStreakCount++
	} else {
		p.winStreakTeam = winnerID
		p.winStreakCount = 1
	}

	p.saveState()

	p.apis.LogAPI.Debug("Win streak updated", map[string]interface{}{
		"team":  p.winStreakTeam,
		"count": p.winStreakCount,
	})

	// Check if scramble threshold reached
	maxStreak := p.getIntConfig("max_win_streak")
	if p.winStreakCount >= maxStreak {
		p.apis.LogAPI.Info("Win streak threshold reached, initiating scramble", map[string]interface{}{
			"team":       p.winStreakTeam,
			"count":      p.winStreakCount,
			"max_streak": maxStreak,
		})

		msg := p.formatMessage(p.getMessage("scramble_announcement"), map[string]interface{}{
			"team":   p.formatTeamNameForBroadcast(p.winStreakTeam),
			"count":  p.winStreakCount,
			"margin": margin,
			"delay":  p.getIntConfig("scramble_announcement_delay"),
		})
		p.broadcast(msg)

		go p.initiateScramble(false, false)
	} else if p.getBoolConfig("show_win_streak_messages") {
		// Broadcast dominant win message
		msg := p.getDominantWinMessage(winnerID, margin, isInvasion)
		p.broadcast(msg)
	}

	return nil
}

// handleNewGame processes new game events
func (p *TeamBalancerPlugin) handleNewGame(event *event_manager.LogGameEventUnifiedData) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.apis.LogAPI.Debug("New game started", map[string]interface{}{
		"layer": event.Layer,
	})

	// Reset game mode cache
	p.gameModeCached = ""

	// Clear team name cache
	p.cachedTeamNames = make(map[int]string)

	// Restart team name polling
	go p.startTeamNamePolling()

	// Reset scramble state
	p.scramblePending = false
	p.scrambleInProgress = false

	// Flip team IDs (teams swap sides on new game)
	if p.winStreakTeam == 1 {
		p.winStreakTeam = 2
	} else if p.winStreakTeam == 2 {
		p.winStreakTeam = 1
	}

	p.saveState()

	return nil
}

// handleChatMessage processes chat messages for commands
func (p *TeamBalancerPlugin) handleChatMessage(rawEvent *plugin_manager.PluginEvent) error {
	event, ok := rawEvent.Data.(*event_manager.RconChatMessageData)
	if !ok {
		return fmt.Errorf("invalid event data type")
	}

	message := strings.ToLower(strings.TrimSpace(event.Message))

	// Check for teambalancer command
	if strings.HasPrefix(message, "!teambalancer") {
		return p.handleTeamBalancerCommand(event)
	}

	// Check for scramble command
	if strings.HasPrefix(message, "!scramble") {
		return p.handleScrambleCommand(event)
	}

	return nil
}

// handleTeamBalancerCommand processes !teambalancer commands
func (p *TeamBalancerPlugin) handleTeamBalancerCommand(event *event_manager.RconChatMessageData) error {
	parts := strings.Fields(strings.ToLower(event.Message))
	subcommand := ""
	if len(parts) > 1 {
		subcommand = parts[1]
	}

	// Check admin status for admin-only commands
	isAdmin := false
	if subcommand != "" && subcommand != "status" {
		var err error
		isAdmin, err = p.isPlayerAdmin(event.SteamID)
		if err != nil {
			p.apis.LogAPI.Error("Failed to check admin status", err, map[string]interface{}{
				"player": event.PlayerName,
			})
			return err
		}

		if !isAdmin {
			p.apis.RconAPI.SendWarningToPlayer(event.SteamID, "You must be an admin to use this command.")
			return nil
		}
	}

	switch subcommand {
	case "", "status":
		return p.handleStatusCommand(event.SteamID)
	case "on":
		return p.handleToggleCommand(event.SteamID, true)
	case "off":
		return p.handleToggleCommand(event.SteamID, false)
	case "diag":
		return p.handleDiagCommand(event.SteamID)
	default:
		p.apis.RconAPI.SendWarningToPlayer(event.SteamID, "Unknown command. Use: status, on, off, diag")
	}

	return nil
}

// handleScrambleCommand processes !scramble commands
func (p *TeamBalancerPlugin) handleScrambleCommand(event *event_manager.RconChatMessageData) error {
	// Check admin status
	isAdmin, err := p.isPlayerAdmin(event.SteamID)
	if err != nil {
		p.apis.LogAPI.Error("Failed to check admin status", err, map[string]interface{}{
			"player": event.PlayerName,
		})
		return err
	}

	if !isAdmin {
		p.apis.RconAPI.SendWarningToPlayer(event.SteamID, "You must be an admin to use this command.")
		return nil
	}

	parts := strings.Fields(strings.ToLower(event.Message))
	hasNow := false
	hasDry := false
	isCancel := false

	for _, part := range parts[1:] {
		if part == "now" {
			hasNow = true
		} else if part == "dry" {
			hasDry = true
		} else if part == "cancel" {
			isCancel = true
		}
	}

	if isCancel {
		return p.handleCancelScramble(event.SteamID)
	}

	p.mu.Lock()
	if p.scramblePending || p.scrambleInProgress {
		p.mu.Unlock()
		status := "pending"
		if p.scrambleInProgress {
			status = "executing"
		}
		p.apis.RconAPI.SendWarningToPlayer(event.SteamID, fmt.Sprintf("Scramble already %s. Use !scramble cancel to cancel.", status))
		return nil
	}
	p.mu.Unlock()

	// Announce scramble
	if !hasDry {
		msg := ""
		if hasNow {
			msg = p.getMessage("immediate_manual_scramble")
		} else {
			msg = p.formatMessage(p.getMessage("manual_scramble_announcement"), map[string]interface{}{
				"delay": p.getIntConfig("scramble_announcement_delay"),
			})
		}
		p.broadcast(msg)
	}

	p.apis.RconAPI.SendWarningToPlayer(event.SteamID, "Scramble initiated.")

	go p.initiateScramble(hasDry, hasDry || hasNow)

	return nil
}

// Chat command handlers
func (p *TeamBalancerPlugin) handleStatusCommand(steamID string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	status := "enabled"
	if p.manuallyDisabled {
		status = "disabled"
	}

	msg := fmt.Sprintf("Team Balancer Status: %s | Win Streak: Team %d (%d wins) | Max: %d",
		status, p.winStreakTeam, p.winStreakCount, p.getIntConfig("max_win_streak"))

	return p.apis.RconAPI.SendWarningToPlayer(steamID, msg)
}

func (p *TeamBalancerPlugin) handleToggleCommand(steamID string, enable bool) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if enable {
		if !p.manuallyDisabled {
			p.apis.RconAPI.SendWarningToPlayer(steamID, "Win streak tracking is already enabled.")
			return nil
		}
		p.manuallyDisabled = false
		p.broadcast(p.getMessage("tracking_enabled"))
		p.apis.RconAPI.SendWarningToPlayer(steamID, "Win streak tracking enabled.")
	} else {
		if p.manuallyDisabled {
			p.apis.RconAPI.SendWarningToPlayer(steamID, "Win streak tracking is already disabled.")
			return nil
		}
		p.manuallyDisabled = true
		p.resetStreak("Manual disable")
		p.broadcast(p.getMessage("tracking_disabled"))
		p.apis.RconAPI.SendWarningToPlayer(steamID, "Win streak tracking disabled.")
	}

	return nil
}

func (p *TeamBalancerPlugin) handleDiagCommand(steamID string) error {
	p.apis.RconAPI.SendWarningToPlayer(steamID, "Running diagnostics...")

	// Run a dry-run scramble as diagnostic
	go func() {
		time.Sleep(1 * time.Second)
		p.initiateScramble(true, true)
		p.apis.RconAPI.SendWarningToPlayer(steamID, "Diagnostics complete. Check logs for details.")
	}()

	return nil
}

func (p *TeamBalancerPlugin) handleCancelScramble(steamID string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.scramblePending {
		if p.scrambleInProgress {
			p.apis.RconAPI.SendWarningToPlayer(steamID, "Cannot cancel - scramble is already executing.")
		} else {
			p.apis.RconAPI.SendWarningToPlayer(steamID, "No pending scramble to cancel.")
		}
		return nil
	}

	// Cancel countdown
	if p.scrambleCountdown != nil {
		p.scrambleCountdown.Stop()
	}
	p.scramblePending = false

	p.broadcast("Scramble cancelled by admin.")
	p.apis.RconAPI.SendWarningToPlayer(steamID, "Scramble cancelled.")

	return nil
}

// Scramble execution

// initiateScramble starts the scramble process
func (p *TeamBalancerPlugin) initiateScramble(isSimulated, immediate bool) error {
	p.mu.Lock()

	if p.scramblePending || p.scrambleInProgress {
		p.mu.Unlock()
		p.apis.LogAPI.Debug("Scramble already pending or in progress", nil)
		return nil
	}

	if isSimulated {
		p.mu.Unlock()
		p.apis.LogAPI.Info("Running simulated scramble (dry run)", nil)
		return p.executeScramble(true)
	}

	if immediate {
		p.mu.Unlock()
		p.apis.LogAPI.Info("Executing immediate scramble", nil)
		return p.executeScramble(false)
	}

	// Start countdown
	p.scramblePending = true
	delay := time.Duration(p.getIntConfig("scramble_announcement_delay")) * time.Second
	p.mu.Unlock()

	p.apis.LogAPI.Info("Scramble countdown started", map[string]interface{}{
		"delay_seconds": delay.Seconds(),
	})

	p.scrambleCountdown = time.AfterFunc(delay, func() {
		p.executeScramble(false)
	})

	return nil
}

// executeScramble performs the actual team scrambling
func (p *TeamBalancerPlugin) executeScramble(isSimulated bool) error {
	p.mu.Lock()

	if p.scrambleInProgress {
		p.mu.Unlock()
		return fmt.Errorf("scramble already in progress")
	}

	p.scrambleInProgress = true
	p.scramblePending = false
	p.mu.Unlock()

	defer func() {
		p.mu.Lock()
		p.scrambleInProgress = false
		p.mu.Unlock()
	}()

	// Broadcast execution message
	if !isSimulated {
		p.broadcast(p.getMessage("execute_scramble"))
	}

	// Get current squads and players
	squads, err := p.apis.ServerAPI.GetSquads()
	if err != nil {
		p.apis.LogAPI.Error("Failed to get squads for scramble", err, nil)
		return err
	}

	players, err := p.apis.ServerAPI.GetPlayers()
	if err != nil {
		p.apis.LogAPI.Error("Failed to get players for scramble", err, nil)
		return err
	}

	p.apis.LogAPI.Debug("Scramble data retrieved", map[string]interface{}{
		"squads":  len(squads),
		"players": len(players),
	})

	// Create scrambler
	scramblerConfig := scrambler.Config{
		ScramblePercentage: p.getFloatConfig("scramble_percentage"),
		WinStreakTeam:      p.winStreakTeam,
		LogAPI:             p.apis.LogAPI,
	}
	s := scrambler.New(scramblerConfig)

	// Generate swap plan
	swapPlan, err := s.GenerateSwapPlan(squads, players)
	if err != nil {
		p.apis.LogAPI.Error("Failed to generate swap plan", err, nil)
		return err
	}

	p.apis.LogAPI.Info("Swap plan generated", map[string]interface{}{
		"total_moves":      swapPlan.Summary.PlayersToMove,
		"squads_preserved": swapPlan.Summary.SquadsPreserved,
		"squads_split":     swapPlan.Summary.SquadsSplit,
	})

	if isSimulated {
		p.apis.LogAPI.Info("Dry run complete - no players moved", map[string]interface{}{
			"would_move": len(swapPlan.Moves),
		})
		return nil
	}

	// Execute swaps
	if len(swapPlan.Moves) == 0 {
		p.apis.LogAPI.Warn("No players to move in swap plan", nil)
		p.broadcast("Scramble complete (no moves required).")
		return nil
	}

	// Queue all moves
	for _, move := range swapPlan.Moves {
		p.swapExecutor.QueueMove(move.SteamID, move.Name, move.TargetTeam)
	}

	// Execute all moves
	if err := p.swapExecutor.ExecuteAll(); err != nil {
		p.apis.LogAPI.Error("Swap execution encountered errors", err, nil)
	}

	// Get execution summary
	total, completed, failed, pending := p.swapExecutor.GetSummary()

	p.apis.LogAPI.Info("Scramble execution complete", map[string]interface{}{
		"total":     total,
		"completed": completed,
		"failed":    failed,
		"pending":   pending,
	})

	// Broadcast completion
	p.broadcast(p.getMessage("scramble_complete"))

	// Save scramble time and reset streak
	p.mu.Lock()
	p.saveScrambleTime()
	p.resetStreak("Post-scramble")
	p.mu.Unlock()

	// Cleanup executor
	p.swapExecutor.Cleanup()

	return nil
}

// Win streak logic

// isDominantWin determines if a win was dominant based on ticket margin
func (p *TeamBalancerPlugin) isDominantWin(winnerID, margin int, isInvasion bool) bool {
	if isInvasion {
		if winnerID == 1 {
			// Attackers
			return margin >= p.getIntConfig("invasion_attack_threshold")
		}
		// Defenders
		return margin >= p.getIntConfig("invasion_defence_threshold")
	}

	// Standard modes
	return margin >= p.getIntConfig("min_tickets_dominant_win")
}

// Message templates and formatting

func (p *TeamBalancerPlugin) getMessage(key string) string {
	messages := map[string]string{
		// Scramble messages
		"scramble_announcement":        "{team} has won {count} dominant rounds in a row! Teams will be scrambled in {delay} seconds to maintain balance.",
		"single_round_scramble":        "Extremely unbalanced round detected (margin: {margin} tickets)! Teams will be scrambled in {delay} seconds.",
		"manual_scramble_announcement": "Admin has initiated a team scramble. Scrambling in {delay} seconds...",
		"immediate_manual_scramble":    "Admin has initiated an immediate team scramble!",
		"execute_scramble":             "Executing team scramble now...",
		"scramble_complete":            "Team scramble complete! Good luck and have fun!",

		// System messages
		"tracking_enabled":  "Win streak tracking has been enabled.",
		"tracking_disabled": "Win streak tracking has been disabled.",

		// Non-dominant win messages
		"narrow_victory":          "{team} secured a narrow victory over {loser} by {margin} tickets. Well fought!",
		"marginal_victory":        "{team} achieved a marginal victory over {loser} ({margin} ticket margin).",
		"tactical_advantage":      "{team} gained a tactical advantage over {loser} ({margin} tickets).",
		"operational_superiority": "{team} demonstrated operational superiority over {loser} ({margin} tickets).",
		"streak_broken":           "{team} broke the win streak! Previous streak reset.",

		// Dominant win messages
		"steamrolled": "{team} steamrolled {loser} with a {margin} ticket margin!",
		"stomped":     "{team} completely stomped {loser} ({margin} tickets)!",

		// Invasion messages
		"invasion_attack_win":   "Attackers ({team}) captured objectives with {margin} tickets remaining.",
		"invasion_defend_win":   "Defenders ({team}) held the line with {margin} tickets remaining.",
		"invasion_attack_stomp": "Attackers ({team}) dominated with {margin} tickets remaining!",
		"invasion_defend_stomp": "Defenders ({team}) crushed the attack with {margin} tickets remaining!",
	}

	if msg, exists := messages[key]; exists {
		return msg
	}
	return ""
}

func (p *TeamBalancerPlugin) formatMessage(template string, data map[string]interface{}) string {
	msg := template
	for key, value := range data {
		placeholder := fmt.Sprintf("{%s}", key)
		msg = strings.ReplaceAll(msg, placeholder, fmt.Sprintf("%v", value))
	}
	return msg
}

func (p *TeamBalancerPlugin) broadcast(message string) {
	prefix := p.getStringConfig("message_prefix")
	fullMessage := prefix + message

	if err := p.apis.RconAPI.Broadcast(fullMessage); err != nil {
		p.apis.LogAPI.Error("Failed to broadcast message", err, map[string]interface{}{
			"message": message,
		})
	}
}

func (p *TeamBalancerPlugin) getDominantWinMessage(winnerID, margin int, isInvasion bool) string {
	winnerName := p.formatTeamNameForBroadcast(winnerID)
	loserName := p.formatTeamNameForBroadcast(3 - winnerID)

	if isInvasion {
		if winnerID == 1 {
			// Attackers won
			if margin >= p.getIntConfig("invasion_attack_threshold")*2 {
				return p.formatMessage(p.getMessage("invasion_attack_stomp"), map[string]interface{}{
					"team":   winnerName,
					"margin": margin,
				})
			}
			return p.formatMessage(p.getMessage("invasion_attack_win"), map[string]interface{}{
				"team":   winnerName,
				"margin": margin,
			})
		}
		// Defenders won
		if margin >= int(float64(p.getIntConfig("invasion_defence_threshold"))*1.5) {
			return p.formatMessage(p.getMessage("invasion_defend_stomp"), map[string]interface{}{
				"team":   winnerName,
				"margin": margin,
			})
		}
		return p.formatMessage(p.getMessage("invasion_defend_win"), map[string]interface{}{
			"team":   winnerName,
			"margin": margin,
		})
	}

	// Standard modes
	dominantThreshold := p.getIntConfig("min_tickets_dominant_win")
	stompThreshold := int(float64(dominantThreshold) * 1.5)

	if margin >= stompThreshold {
		return p.formatMessage(p.getMessage("stomped"), map[string]interface{}{
			"team":   winnerName,
			"loser":  loserName,
			"margin": margin,
		})
	}

	return p.formatMessage(p.getMessage("steamrolled"), map[string]interface{}{
		"team":   winnerName,
		"loser":  loserName,
		"margin": margin,
	})
}

func (p *TeamBalancerPlugin) getNonDominantWinMessage(winnerID, margin int, isInvasion bool) string {
	winnerName := p.formatTeamNameForBroadcast(winnerID)
	loserName := p.formatTeamNameForBroadcast(3 - winnerID)

	// Check if streak was broken
	if p.winStreakTeam != 0 && p.winStreakTeam != winnerID {
		return p.formatMessage(p.getMessage("streak_broken"), map[string]interface{}{
			"team": winnerName,
		})
	}

	if isInvasion {
		if winnerID == 1 {
			return p.formatMessage(p.getMessage("invasion_attack_win"), map[string]interface{}{
				"team":   winnerName,
				"margin": margin,
			})
		}
		return p.formatMessage(p.getMessage("invasion_defend_win"), map[string]interface{}{
			"team":   winnerName,
			"margin": margin,
		})
	}

	// Standard modes - categorize by margin
	dominantThreshold := p.getIntConfig("min_tickets_dominant_win")
	veryCloseCutoff := int(float64(dominantThreshold) * 0.11)
	closeCutoff := int(float64(dominantThreshold) * 0.45)
	tacticalCutoff := int(float64(dominantThreshold) * 0.68)

	var template string
	if margin < veryCloseCutoff {
		template = p.getMessage("narrow_victory")
	} else if margin < closeCutoff {
		template = p.getMessage("marginal_victory")
	} else if margin < tacticalCutoff {
		template = p.getMessage("tactical_advantage")
	} else {
		template = p.getMessage("operational_superiority")
	}

	return p.formatMessage(template, map[string]interface{}{
		"team":   winnerName,
		"loser":  loserName,
		"margin": margin,
	})
}

// Helper methods

func (p *TeamBalancerPlugin) isPlayerAdmin(steamID string) (bool, error) {
	admins, err := p.apis.ServerAPI.GetAdmins()
	if err != nil {
		return false, fmt.Errorf("failed to get admin list: %w", err)
	}

	for _, admin := range admins {
		if admin.SteamID == steamID {
			return true, nil
		}
	}

	return false, nil
}
