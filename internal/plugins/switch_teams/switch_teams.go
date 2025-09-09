package switch_teams

import (
	"context"
	"fmt"
	"math"
	"strings"
	"sync"
	"time"

	"go.codycody31.dev/squad-aegis/internal/event_manager"
	"go.codycody31.dev/squad-aegis/internal/plugin_manager"
	"go.codycody31.dev/squad-aegis/internal/shared/plug_config_schema"
)

// SwitchTeamsPlugin manages immediate team switching
type SwitchTeamsPlugin struct {
	// Plugin configuration
	config map[string]interface{}
	apis   *plugin_manager.PluginAPIs

	// State management
	mu           sync.Mutex
	status       plugin_manager.PluginStatus
	lastSwitches map[string]time.Time // Map of steamID -> last switch time for cooldown
}

// Define returns the plugin definition
func Define() plugin_manager.PluginDefinition {
	return plugin_manager.PluginDefinition{
		ID:                     "switch_teams",
		Name:                   "Switch Teams",
		Description:            "Allows players to request team switches using !switch command. Players are switched immediately if it doesn't worsen team balance. No queuing or automatic processing.",
		Version:                "1.0.0",
		Author:                 "Squad Aegis",
		AllowMultipleInstances: false,
		RequiredConnectors:     []string{},
		LongRunning:            false,

		ConfigSchema: plug_config_schema.ConfigSchema{
			Fields: []plug_config_schema.ConfigField{
				{
					Name:        "command",
					Description: "The command used to request a team switch.",
					Required:    false,
					Type:        plug_config_schema.FieldTypeString,
					Default:     "switch",
				},
				{
					Name:        "cooldown_minutes",
					Description: "Minutes a player must wait before using the command again.",
					Required:    false,
					Type:        plug_config_schema.FieldTypeInt,
					Default:     60,
				},
				{
					Name:        "team_imbalance_threshold",
					Description: "Minimum player difference between teams that allows switching from larger team to smaller team, even if it would worsen balance slightly.",
					Required:    false,
					Type:        plug_config_schema.FieldTypeInt,
					Default:     2,
				},
				{
					Name:        "admin_only",
					Description: "Whether only admins can use the switch command.",
					Required:    false,
					Type:        plug_config_schema.FieldTypeBool,
					Default:     false,
				},
			},
		},

		Events: []event_manager.EventType{
			event_manager.EventTypeRconChatMessage,
		},

		CreateInstance: func() plugin_manager.Plugin {
			return &SwitchTeamsPlugin{
				lastSwitches: make(map[string]time.Time),
			}
		},
	}
}

// GetDefinition returns the plugin definition
func (p *SwitchTeamsPlugin) GetDefinition() plugin_manager.PluginDefinition {
	return Define()
}

// Initialize initializes the plugin with its configuration and dependencies
func (p *SwitchTeamsPlugin) Initialize(config map[string]interface{}, apis *plugin_manager.PluginAPIs) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.config = config
	p.apis = apis

	// Initialize with empty switch cooldowns
	if p.lastSwitches == nil {
		p.lastSwitches = make(map[string]time.Time)
	}

	return nil
}

// GetStatus returns the current plugin status
func (p *SwitchTeamsPlugin) GetStatus() plugin_manager.PluginStatus {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.status
}

// GetConfig returns the current plugin configuration
func (p *SwitchTeamsPlugin) GetConfig() map[string]interface{} {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.config
}

// UpdateConfig updates the plugin configuration
func (p *SwitchTeamsPlugin) UpdateConfig(config map[string]interface{}) error {
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

	p.apis.LogAPI.Info("Switch Teams plugin configuration updated", map[string]interface{}{
		"command":                  p.getStringConfig("command"),
		"cooldown_minutes":         p.getIntConfig("cooldown_minutes"),
		"team_imbalance_threshold": p.getIntConfig("team_imbalance_threshold"),
		"admin_only":               p.getBoolConfig("admin_only"),
	})

	return nil
}

// Start begins plugin execution (for long-running plugins)
func (p *SwitchTeamsPlugin) Start(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.status == plugin_manager.PluginStatusRunning {
		return nil // Already running
	}

	p.status = plugin_manager.PluginStatusRunning

	return nil
}

// Stop gracefully stops the plugin
func (p *SwitchTeamsPlugin) Stop() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.status == plugin_manager.PluginStatusStopped {
		return nil // Already stopped
	}

	p.status = plugin_manager.PluginStatusStopping
	p.status = plugin_manager.PluginStatusStopped

	return nil
}

// HandleEvent processes an event if the plugin is subscribed to it
func (p *SwitchTeamsPlugin) HandleEvent(event *plugin_manager.PluginEvent) error {
	if event.Type != "RCON_CHAT_MESSAGE" {
		return nil // Not interested in this event
	}

	return p.handleChatMessage(event)
}

// handleChatMessage processes chat message events to detect switch commands
func (p *SwitchTeamsPlugin) handleChatMessage(rawEvent *plugin_manager.PluginEvent) error {
	event, ok := rawEvent.Data.(*event_manager.RconChatMessageData)
	if !ok {
		return fmt.Errorf("invalid event data type")
	}

	// Check if this is a switch command
	message := strings.TrimSpace(event.Message)
	command := "!" + p.getStringConfig("command")

	if !strings.EqualFold(message, command) {
		return nil // Not our command
	}

	// Check if admin only and validate admin status
	if p.getBoolConfig("admin_only") {
		isAdmin, err := p.isPlayerAdmin(event.SteamID)
		if err != nil {
			p.apis.LogAPI.Error("Failed to check admin status", err, map[string]interface{}{
				"player":  event.PlayerName,
				"steamID": event.SteamID,
			})
			return err
		}
		if !isAdmin {
			return p.apis.RconAPI.SendWarningToPlayer(event.EosID, "You must be an admin to use the switch command.")
		}
	}

	// Process the switch request
	return p.processSwitchRequest(event)
}

// processSwitchRequest handles a player's switch request
func (p *SwitchTeamsPlugin) processSwitchRequest(event *event_manager.RconChatMessageData) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Check cooldown
	if lastSwitch, exists := p.lastSwitches[event.SteamID]; exists {
		cooldownMinutes := p.getIntConfig("cooldown_minutes")
		cooldown := time.Duration(cooldownMinutes) * time.Minute
		timeSinceLastSwitch := time.Since(lastSwitch)

		if timeSinceLastSwitch < cooldown {
			remainingTime := cooldown - timeSinceLastSwitch
			return p.apis.RconAPI.SendWarningToPlayer(event.EosID,
				fmt.Sprintf("You must wait %s before using !%s again.",
					p.formatDuration(remainingTime), p.getStringConfig("command")))
		}
	}

	// Get current player team info
	players, err := p.apis.ServerAPI.GetPlayers()
	if err != nil {
		return fmt.Errorf("failed to get player list: %w", err)
	}

	var currentPlayer *plugin_manager.PlayerInfo
	for _, player := range players {
		if player.SteamID == event.SteamID {
			currentPlayer = player
			break
		}
	}

	if currentPlayer == nil {
		return fmt.Errorf("player not found in server")
	}

	// Try to switch the player immediately
	if err := p.tryImmediateSwitch(event.SteamID, event.PlayerName, event.EosID, currentPlayer.TeamID, players); err != nil {
		return p.apis.RconAPI.SendWarningToPlayer(event.EosID, err.Error())
	}

	// Record successful switch time for cooldown
	p.lastSwitches[event.SteamID] = time.Now()

	return nil
}

// tryImmediateSwitch attempts to immediately switch a player if conditions are met
func (p *SwitchTeamsPlugin) tryImmediateSwitch(steamID, playerName, eosID string, fromTeam int, players []*plugin_manager.PlayerInfo) error {
	team1Count, team2Count := p.getTeamCounts(players)

	// Find which team the player wants to switch to
	var toTeam int
	var currentImbalance int
	var newImbalance int

	if fromTeam == 1 {
		toTeam = 2
		currentImbalance = int(math.Abs(float64(team1Count - team2Count)))
		newImbalance = int(math.Abs(float64((team1Count - 1) - (team2Count + 1))))
	} else {
		toTeam = 1
		currentImbalance = int(math.Abs(float64(team1Count - team2Count)))
		newImbalance = int(math.Abs(float64((team1Count + 1) - (team2Count - 1))))
	}

	threshold := p.getIntConfig("team_imbalance_threshold")

	// Allow switch if:
	// 1. It doesn't make teams more imbalanced, OR
	// 2. It helps balance teams (reduces imbalance), OR
	// 3. The new imbalance after switch is still within threshold
	allowSwitch := false
	reason := ""

	if newImbalance <= currentImbalance {
		allowSwitch = true
		reason = "switch maintains or improves team balance"
	} else if newImbalance <= threshold {
		allowSwitch = true
		reason = fmt.Sprintf("switch is allowed as new imbalance (%d) is within threshold (%d)", newImbalance, threshold)
	} else {
		reason = fmt.Sprintf("Switch would exceed threshold (current: %d, after: %d, threshold: %d)", currentImbalance, newImbalance, threshold)
	}

	if !allowSwitch {
		return fmt.Errorf("%s", reason)
	}

	// Switch the player
	if err := p.switchPlayerToTeam(steamID, toTeam); err != nil {
		return fmt.Errorf("failed to switch player: %w", err)
	}

	// Notify player
	p.apis.RconAPI.SendWarningToPlayer(eosID, "You have switched teams.")

	p.apis.LogAPI.Info("Player switched immediately", map[string]interface{}{
		"player":            playerName,
		"from_team":         fromTeam,
		"to_team":           toTeam,
		"team1_count":       team1Count,
		"team2_count":       team2Count,
		"current_imbalance": currentImbalance,
		"new_imbalance":     newImbalance,
		"reason":            reason,
	})

	return nil
}

// switchPlayerToTeam moves a player to the specified team using RCON
func (p *SwitchTeamsPlugin) switchPlayerToTeam(steamID string, teamID int) error {
	command := fmt.Sprintf("AdminForceTeamChange %s", steamID)
	_, err := p.apis.RconAPI.SendCommand(command)
	return err
}

// getTeamCounts returns the number of players on each team
func (p *SwitchTeamsPlugin) getTeamCounts(players []*plugin_manager.PlayerInfo) (int, int) {
	team1Count := 0
	team2Count := 0

	for _, player := range players {
		switch player.TeamID {
		case 1:
			team1Count++
		case 2:
			team2Count++
		}
	}

	return team1Count, team2Count
}

// isPlayerAdmin checks if a player is an admin
func (p *SwitchTeamsPlugin) isPlayerAdmin(steamID string) (bool, error) {
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

// formatDuration formats a duration into a human-readable string
func (p *SwitchTeamsPlugin) formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%.0f seconds", d.Seconds())
	} else if d < time.Hour {
		return fmt.Sprintf("%.0f minutes", d.Minutes())
	} else {
		hours := int(d.Hours())
		minutes := int(d.Minutes()) % 60
		if minutes == 0 {
			return fmt.Sprintf("%d hours", hours)
		}
		return fmt.Sprintf("%d hours %d minutes", hours, minutes)
	}
}

// Helper methods for config access

func (p *SwitchTeamsPlugin) getStringConfig(key string) string {
	if val, ok := p.config[key].(string); ok {
		return val
	}
	return ""
}

func (p *SwitchTeamsPlugin) getIntConfig(key string) int {
	if val, ok := p.config[key].(int); ok {
		return val
	}
	if val, ok := p.config[key].(float64); ok {
		return int(val)
	}
	return 0
}

func (p *SwitchTeamsPlugin) getBoolConfig(key string) bool {
	if val, ok := p.config[key].(bool); ok {
		return val
	}
	return false
}
