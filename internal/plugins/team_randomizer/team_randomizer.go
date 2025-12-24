package team_randomizer

import (
	"context"
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"time"

	"go.codycody31.dev/squad-aegis/internal/event_manager"
	"go.codycody31.dev/squad-aegis/internal/plugin_manager"
	"go.codycody31.dev/squad-aegis/internal/shared/plug_config_schema"
)

// TeamRandomizerPlugin randomizes teams to break up clan stacks
type TeamRandomizerPlugin struct {
	// Plugin configuration
	config map[string]interface{}
	apis   *plugin_manager.PluginAPIs

	// State management
	mu     sync.Mutex
	status plugin_manager.PluginStatus
	ctx    context.Context
	cancel context.CancelFunc
}

// Define returns the plugin definition
func Define() plugin_manager.PluginDefinition {
	return plugin_manager.PluginDefinition{
		ID:                     "team_randomizer",
		Name:                   "Team Randomizer",
		Description:            "The Team Randomizer can be used to randomize teams. It's great for destroying clan stacks or for social events. It can be run by typing, by default, !randomize into in-game admin chat.",
		Version:                "1.0.0",
		Author:                 "Squad Aegis",
		AllowMultipleInstances: false,
		RequiredConnectors:     []string{},
		LongRunning:            false,

		ConfigSchema: plug_config_schema.ConfigSchema{
			Fields: []plug_config_schema.ConfigField{
				{
					Name:        "command",
					Description: "The command used to randomize the teams.",
					Required:    false,
					Type:        plug_config_schema.FieldTypeString,
					Default:     "randomize",
				},
				{
					Name:        "admin_only",
					Description: "Whether only admins can use the randomize command.",
					Required:    false,
					Type:        plug_config_schema.FieldTypeBool,
					Default:     true,
				},
				{
					Name:        "require_admin_chat",
					Description: "Whether the command must be used in admin chat only.",
					Required:    false,
					Type:        plug_config_schema.FieldTypeBool,
					Default:     true,
				},
				{
					Name:        "announce_randomization",
					Description: "Whether to announce the team randomization to all players.",
					Required:    false,
					Type:        plug_config_schema.FieldTypeBool,
					Default:     true,
				},
				{
					Name:        "cooldown_seconds",
					Description: "Cooldown period in seconds between randomizations.",
					Required:    false,
					Type:        plug_config_schema.FieldTypeInt,
					Default:     30,
				},
			},
		},

		Events: []event_manager.EventType{
			event_manager.EventTypeRconChatMessage,
		},

		CreateInstance: func() plugin_manager.Plugin {
			return &TeamRandomizerPlugin{}
		},
	}
}

// GetDefinition returns the plugin definition
func (p *TeamRandomizerPlugin) GetDefinition() plugin_manager.PluginDefinition {
	return Define()
}

func (p *TeamRandomizerPlugin) GetCommands() []plugin_manager.PluginCommand {
	return []plugin_manager.PluginCommand{}
}

func (p *TeamRandomizerPlugin) ExecuteCommand(commandID string, params map[string]interface{}) (*plugin_manager.CommandResult, error) {
	return nil, fmt.Errorf("no commands available")
}

func (p *TeamRandomizerPlugin) GetCommandExecutionStatus(executionID string) (*plugin_manager.CommandExecutionStatus, error) {
	return nil, fmt.Errorf("no commands available")
}

// Initialize initializes the plugin with its configuration and dependencies
func (p *TeamRandomizerPlugin) Initialize(config map[string]interface{}, apis *plugin_manager.PluginAPIs) error {
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

	p.status = plugin_manager.PluginStatusStopped

	return nil
}

// Start begins plugin execution (for long-running plugins)
func (p *TeamRandomizerPlugin) Start(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.status == plugin_manager.PluginStatusRunning {
		return nil // Already running
	}

	p.ctx, p.cancel = context.WithCancel(ctx)
	p.status = plugin_manager.PluginStatusRunning

	return nil
}

// Stop gracefully stops the plugin
func (p *TeamRandomizerPlugin) Stop() error {
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
func (p *TeamRandomizerPlugin) HandleEvent(event *plugin_manager.PluginEvent) error {
	if event.Type != "RCON_CHAT_MESSAGE" {
		return nil // Not interested in this event
	}

	return p.handleChatMessage(event)
}

// GetStatus returns the current plugin status
func (p *TeamRandomizerPlugin) GetStatus() plugin_manager.PluginStatus {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.status
}

// GetConfig returns the current plugin configuration
func (p *TeamRandomizerPlugin) GetConfig() map[string]interface{} {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.config
}

// UpdateConfig updates the plugin configuration
func (p *TeamRandomizerPlugin) UpdateConfig(config map[string]interface{}) error {
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

	p.apis.LogAPI.Info("Team Randomizer plugin configuration updated", map[string]interface{}{
		"command": config["command"],
	})

	return nil
}

// handleChatMessage processes chat messages looking for randomize commands
func (p *TeamRandomizerPlugin) handleChatMessage(rawEvent *plugin_manager.PluginEvent) error {
	event, ok := rawEvent.Data.(*event_manager.RconChatMessageData)
	if !ok {
		return fmt.Errorf("invalid event data type")
	}

	// Check if this is a randomize command
	if !p.isRandomizeCommand(event.Message) {
		return nil
	}

	// Check if admin chat is required
	if p.getBoolConfig("require_admin_chat") && event.ChatType != "ChatAdmin" {
		return nil
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
			return p.apis.RconAPI.SendWarningToPlayer(event.SteamID, "You must be an admin to use the randomize command.")
		}
	}

	// Execute team randomization
	if err := p.randomizeTeams(event.PlayerName, event.SteamID); err != nil {
		p.apis.LogAPI.Error("Failed to randomize teams", err, map[string]interface{}{
			"initiator": event.PlayerName,
		})
		return err
	}

	return nil
}

// isRandomizeCommand checks if a message is a randomize command
func (p *TeamRandomizerPlugin) isRandomizeCommand(message string) bool {
	command := p.getStringConfig("command")
	message = strings.ToLower(strings.TrimSpace(message))
	return strings.HasPrefix(message, "!"+strings.ToLower(command))
}

// isPlayerAdmin checks if a player is an admin
func (p *TeamRandomizerPlugin) isPlayerAdmin(steamID string) (bool, error) {
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

// randomizeTeams performs the team randomization
func (p *TeamRandomizerPlugin) randomizeTeams(initiatorName, steamID string) error {
	// Get current players
	players, err := p.apis.ServerAPI.GetPlayers()
	if err != nil {
		return fmt.Errorf("failed to get players: %w", err)
	}

	// Filter out players who are not online or don't have valid IDs
	var validPlayers []*plugin_manager.PlayerInfo
	for _, player := range players {
		if player.IsOnline && player.SteamID != "" {
			validPlayers = append(validPlayers, player)
		}
	}

	if len(validPlayers) == 0 {
		return fmt.Errorf("no valid players found for randomization")
	}

	// Announce randomization if configured
	if p.getBoolConfig("announce_randomization") {
		announcement := fmt.Sprintf("Team randomization initiated by %s! Players will be moved to balance teams.", initiatorName)
		if err := p.apis.RconAPI.Broadcast(announcement); err != nil {
			p.apis.LogAPI.Warn("Failed to announce randomization", map[string]interface{}{
				"error": err.Error(),
			})
		}
	}

	// Shuffle players using Fisher-Yates algorithm
	shuffledPlayers := make([]*plugin_manager.PlayerInfo, len(validPlayers))
	copy(shuffledPlayers, validPlayers)

	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	for i := len(shuffledPlayers) - 1; i > 0; i-- {
		j := rng.Intn(i + 1)
		shuffledPlayers[i], shuffledPlayers[j] = shuffledPlayers[j], shuffledPlayers[i]
	}

	// Assign players to teams alternately
	// Team 1 first, then Team 2, then Team 1, etc.
	targetTeam := 1
	moveCount := 0

	for _, player := range shuffledPlayers {
		// Only move player if they're not already on the target team
		if player.TeamID != targetTeam {
			// Use SteamID for AdminForceTeamChange command
			command := fmt.Sprintf("AdminForceTeamChange %s", player.SteamID)

			if _, err := p.apis.RconAPI.SendCommand(command); err != nil {
				p.apis.LogAPI.Error("Failed to move player to team", err, map[string]interface{}{
					"player":     player.Name,
					"steamID":    player.SteamID,
					"targetTeam": targetTeam,
				})
				// Continue with other players instead of failing completely
			} else {
				moveCount++
				p.apis.LogAPI.Debug("Moved player to team", map[string]interface{}{
					"player":   player.Name,
					"steamID":  player.SteamID,
					"fromTeam": player.TeamID,
					"toTeam":   targetTeam,
				})
			}
		}

		// Alternate between team 1 and team 2
		if targetTeam == 1 {
			targetTeam = 2
		} else {
			targetTeam = 1
		}
	}

	// Final announcement
	if p.getBoolConfig("announce_randomization") {
		finalAnnouncement := fmt.Sprintf("Team randomization complete! Moved %d players.", moveCount)
		if err := p.apis.RconAPI.Broadcast(finalAnnouncement); err != nil {
			p.apis.LogAPI.Warn("Failed to announce randomization completion", map[string]interface{}{
				"error": err.Error(),
			})
		}
	}

	p.apis.LogAPI.Info("Team randomization completed", map[string]interface{}{
		"totalPlayers": len(validPlayers),
		"playersMoved": moveCount,
		"initiator":    initiatorName,
		"steamID":      steamID,
	})

	return nil
}

// Helper methods for config access

func (p *TeamRandomizerPlugin) getStringConfig(key string) string {
	if value, ok := p.config[key].(string); ok {
		return value
	}
	return ""
}

func (p *TeamRandomizerPlugin) getBoolConfig(key string) bool {
	if value, ok := p.config[key].(bool); ok {
		return value
	}
	return false
}
