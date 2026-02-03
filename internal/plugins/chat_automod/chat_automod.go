package chat_automod

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.codycody31.dev/squad-aegis/internal/event_manager"
	"go.codycody31.dev/squad-aegis/internal/plugin_manager"
	"go.codycody31.dev/squad-aegis/internal/shared/plug_config_schema"
)

// ChatAutoModPlugin handles automatic chat moderation
type ChatAutoModPlugin struct {
	config map[string]interface{}
	apis   *plugin_manager.PluginAPIs

	mu     sync.Mutex
	status plugin_manager.PluginStatus
	ctx    context.Context
	cancel context.CancelFunc

	// Filter components
	filters *LanguageFilters
	tracker *ViolationTracker

	// Cached escalation actions
	escalationActions []EscalationAction

	// Cached admin list (refreshed periodically)
	adminCache     map[string]bool
	adminCacheTime time.Time
}

// Define returns the plugin definition
func Define() plugin_manager.PluginDefinition {
	return plugin_manager.PluginDefinition{
		ID:                     "chat_automod",
		Name:                   "Chat AutoMod",
		Description:            "Automatically moderates chat for hate speech, slurs, discrimination, and other rule violations with escalating consequences.",
		Version:                "1.0.0",
		Author:                 "Squad Aegis",
		AllowMultipleInstances: false,
		RequiredConnectors:     []string{},
		LongRunning:            false,

		ConfigSchema: getConfigSchema(),

		Events: []event_manager.EventType{
			event_manager.EventTypeRconChatMessage,
		},

		CreateInstance: func() plugin_manager.Plugin {
			return &ChatAutoModPlugin{}
		},
	}
}

// getConfigSchema returns the configuration schema for the plugin
func getConfigSchema() plug_config_schema.ConfigSchema {
	return plug_config_schema.ConfigSchema{
		Fields: []plug_config_schema.ConfigField{
			// Rule Integration
			{
				Name:        "rule_id",
				Description: "The UUID of the server rule to link violations to (e.g., the discrimination rule). Leave empty to not link to a rule.",
				Required:    false,
				Type:        plug_config_schema.FieldTypeString,
				Default:     "",
			},
			{
				Name:        "rule_display_id",
				Description: "The display ID of the rule (e.g., '2.1') to show in messages. Used when {rule_id} placeholder is in messages.",
				Required:    false,
				Type:        plug_config_schema.FieldTypeString,
				Default:     "",
			},

			// Filter category toggles
			{
				Name:        "enable_racial_slurs",
				Description: "Enable detection of racial slurs",
				Required:    false,
				Type:        plug_config_schema.FieldTypeBool,
				Default:     true,
			},
			{
				Name:        "enable_homophobic_slurs",
				Description: "Enable detection of homophobic slurs",
				Required:    false,
				Type:        plug_config_schema.FieldTypeBool,
				Default:     true,
			},
			{
				Name:        "enable_ableist_language",
				Description: "Enable detection of ableist language",
				Required:    false,
				Type:        plug_config_schema.FieldTypeBool,
				Default:     true,
			},
			// Regional settings
			{
				Name:        "region",
				Description: "Region setting for regional word tolerance. Affects which words are flagged. Options: us, uk, au, eu",
				Required:    false,
				Type:        plug_config_schema.FieldTypeString,
				Default:     "us",
			},

			// Custom word lists
			{
				Name:        "custom_blacklist",
				Description: "Additional words or regex patterns to flag (e.g., server-specific prohibited terms)",
				Required:    false,
				Type:        plug_config_schema.FieldTypeArrayString,
				Default:     []interface{}{},
			},
			{
				Name:        "whitelist",
				Description: "Words to exempt from detection (e.g., player names, clan tags that might trigger false positives)",
				Required:    false,
				Type:        plug_config_schema.FieldTypeArrayString,
				Default:     []interface{}{},
			},

			// Escalation actions
			plug_config_schema.NewArrayObjectField(
				"escalation_actions",
				"Actions to take based on violation count. Configure warn, kick, and ban escalation.",
				false,
				[]plug_config_schema.ConfigField{
					{
						Name:        "violation_count",
						Description: "Violation number this action applies to (1 = first offense, 2 = second, etc.)",
						Required:    true,
						Type:        plug_config_schema.FieldTypeInt,
						Default:     1,
					},
					{
						Name:        "action",
						Description: "Action type: WARN, KICK, or BAN",
						Required:    true,
						Type:        plug_config_schema.FieldTypeString,
						Default:     "WARN",
					},
					{
						Name:        "ban_duration_days",
						Description: "Ban duration in days (0 for permanent). Only applies to BAN action.",
						Required:    false,
						Type:        plug_config_schema.FieldTypeInt,
						Default:     0,
					},
					{
						Name:        "message",
						Description: "Message to show the player. Use {rule_id} for rule reference, {category} for violation type.",
						Required:    true,
						Type:        plug_config_schema.FieldTypeString,
						Default:     "Your message violated server rules.",
					},
				},
				[]interface{}{
					map[string]interface{}{
						"violation_count":   1,
						"action":            "WARN",
						"ban_duration_days": 0,
						"message":           "Warning: Your message contained prohibited language ({category}). Please review rule {rule_id}.",
					},
					map[string]interface{}{
						"violation_count":   2,
						"action":            "KICK",
						"ban_duration_days": 0,
						"message":           "You have been kicked for repeated language violations. Next offense will result in a ban.",
					},
					map[string]interface{}{
						"violation_count":   3,
						"action":            "BAN",
						"ban_duration_days": 7,
						"message":           "Banned for 7 days for repeated hate speech/discrimination violations (Rule {rule_id}).",
					},
				},
			),

			// Use server rule actions
			{
				Name:        "use_server_rule_actions",
				Description: "Use server rule actions instead of plugin escalation_actions when rule_id is set. Queries the server_rule_actions table for escalation.",
				Required:    false,
				Type:        plug_config_schema.FieldTypeBool,
				Default:     false,
			},

			// Violation expiry
			{
				Name:        "violation_expiry_days",
				Description: "Days after which old violations expire and don't count toward escalation (0 = violations never expire)",
				Required:    false,
				Type:        plug_config_schema.FieldTypeInt,
				Default:     30,
			},

			// Chat type filtering
			{
				Name:        "ignore_chat_types",
				Description: "Chat types to ignore (e.g., ChatAdmin for admin chat). Options: ChatAll, ChatTeam, ChatSquad, ChatAdmin",
				Required:    false,
				Type:        plug_config_schema.FieldTypeArrayString,
				Default:     []interface{}{"ChatAdmin"},
			},

			// Admin exemption
			{
				Name:        "exempt_admins",
				Description: "Exempt server admins from automod detection and actions",
				Required:    false,
				Type:        plug_config_schema.FieldTypeBool,
				Default:     false,
			},

			// Logging
			{
				Name:        "log_detections",
				Description: "Log all detections to plugin logs (useful for monitoring false positives)",
				Required:    false,
				Type:        plug_config_schema.FieldTypeBool,
				Default:     true,
			},
		},
	}
}

// GetDefinition returns the plugin definition
func (p *ChatAutoModPlugin) GetDefinition() plugin_manager.PluginDefinition {
	return Define()
}

func (p *ChatAutoModPlugin) GetCommands() []plugin_manager.PluginCommand {
	return []plugin_manager.PluginCommand{}
}

func (p *ChatAutoModPlugin) ExecuteCommand(commandID string, params map[string]interface{}) (*plugin_manager.CommandResult, error) {
	return nil, fmt.Errorf("no commands available")
}

func (p *ChatAutoModPlugin) GetCommandExecutionStatus(executionID string) (*plugin_manager.CommandExecutionStatus, error) {
	return nil, fmt.Errorf("no commands available")
}

// Initialize initializes the plugin with its configuration and dependencies
func (p *ChatAutoModPlugin) Initialize(config map[string]interface{}, apis *plugin_manager.PluginAPIs) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.config = config
	p.apis = apis
	p.status = plugin_manager.PluginStatusStopped
	p.adminCache = make(map[string]bool)

	// Validate config
	definition := p.GetDefinition()
	if err := definition.ConfigSchema.Validate(config); err != nil {
		return fmt.Errorf("invalid config: %w", err)
	}

	// Fill defaults
	definition.ConfigSchema.FillDefaults(config)

	// Initialize filters
	if err := p.initializeFilters(); err != nil {
		return fmt.Errorf("failed to initialize filters: %w", err)
	}

	// Initialize violation tracker
	expiryDays := p.getIntConfig("violation_expiry_days")
	p.tracker = NewViolationTracker(p.apis.DatabaseAPI, expiryDays)

	// Parse escalation actions
	if err := p.parseEscalationActions(); err != nil {
		return fmt.Errorf("failed to parse escalation actions: %w", err)
	}

	p.apis.LogAPI.Info("Chat AutoMod plugin initialized", map[string]interface{}{
		"region":                  p.getStringConfig("region"),
		"enable_racial_slurs":     p.getBoolConfig("enable_racial_slurs"),
		"enable_homophobic_slurs": p.getBoolConfig("enable_homophobic_slurs"),
		"enable_ableist_language": p.getBoolConfig("enable_ableist_language"),
		"violation_expiry_days":   expiryDays,
		"escalation_action_count": len(p.escalationActions),
	})

	return nil
}

// initializeFilters creates and configures the language filters
func (p *ChatAutoModPlugin) initializeFilters() error {
	region := p.getStringConfig("region")
	enableRacial := p.getBoolConfig("enable_racial_slurs")
	enableHomophobic := p.getBoolConfig("enable_homophobic_slurs")
	enableAbleist := p.getBoolConfig("enable_ableist_language")

	p.filters = NewLanguageFilters(region, enableRacial, enableHomophobic, enableAbleist)

	// Set whitelist
	whitelist := p.getArrayStringConfig("whitelist")
	p.filters.SetWhitelist(whitelist)

	// Set custom blacklist
	customBlacklist := p.getArrayStringConfig("custom_blacklist")
	p.filters.SetCustomPatterns(customBlacklist)

	return nil
}

// parseEscalationActions parses the escalation actions from config
func (p *ChatAutoModPlugin) parseEscalationActions() error {
	p.escalationActions = []EscalationAction{}

	actionsConfig := plug_config_schema.GetArrayObjectValue(p.config, "escalation_actions")

	for _, actionObj := range actionsConfig {
		action := EscalationAction{
			ViolationCount:  plug_config_schema.GetIntValue(actionObj, "violation_count"),
			Action:          strings.ToUpper(plug_config_schema.GetStringValue(actionObj, "action")),
			BanDurationDays: plug_config_schema.GetIntValue(actionObj, "ban_duration_days"),
			Message:         plug_config_schema.GetStringValue(actionObj, "message"),
		}

		// Validate action type
		if action.Action != "WARN" && action.Action != "KICK" && action.Action != "BAN" {
			return fmt.Errorf("invalid action type: %s (must be WARN, KICK, or BAN)", action.Action)
		}

		p.escalationActions = append(p.escalationActions, action)
	}

	return nil
}

// Start begins plugin execution
func (p *ChatAutoModPlugin) Start(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.status == plugin_manager.PluginStatusRunning {
		return nil
	}

	p.ctx, p.cancel = context.WithCancel(ctx)
	p.status = plugin_manager.PluginStatusRunning

	return nil
}

// Stop gracefully stops the plugin
func (p *ChatAutoModPlugin) Stop() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.status == plugin_manager.PluginStatusStopped {
		return nil
	}

	p.status = plugin_manager.PluginStatusStopping

	if p.cancel != nil {
		p.cancel()
	}

	p.status = plugin_manager.PluginStatusStopped
	p.apis.LogAPI.Info("Chat AutoMod plugin stopped", nil)

	return nil
}

// HandleEvent processes chat message events
func (p *ChatAutoModPlugin) HandleEvent(event *plugin_manager.PluginEvent) error {
	if event.Type != string(event_manager.EventTypeRconChatMessage) {
		return nil
	}

	chatEvent, ok := event.Data.(*event_manager.RconChatMessageData)
	if !ok {
		return fmt.Errorf("invalid event data type")
	}

	// Check ignored chat types
	ignoredTypes := p.getArrayStringConfig("ignore_chat_types")
	for _, ignored := range ignoredTypes {
		if strings.EqualFold(chatEvent.ChatType, ignored) {
			return nil
		}
	}

	// Check admin exemption
	if p.getBoolConfig("exempt_admins") {
		if p.isPlayerAdmin(chatEvent.SteamID) {
			return nil
		}
	}

	// Run through filters
	result := p.filters.CheckMessage(chatEvent.Message)

	if !result.Detected {
		return nil
	}

	// Log detection
	if p.getBoolConfig("log_detections") {
		p.apis.LogAPI.Info("Chat violation detected", map[string]interface{}{
			"player_name":   chatEvent.PlayerName,
			"steam_id":      chatEvent.SteamID,
			"message":       chatEvent.Message,
			"category":      string(result.Category),
			"matched_terms": result.MatchedTerms,
			"severity":      result.Severity,
		})
	}

	// Handle violation
	return p.handleViolation(event.ID, chatEvent, result)
}

// handleViolation processes a detected violation
func (p *ChatAutoModPlugin) handleViolation(eventID uuid.UUID, chatEvent *event_manager.RconChatMessageData, result *FilterResult) error {
	// Get current violation count (before adding this one)
	currentCount, err := p.tracker.GetActiveViolationCount(chatEvent.SteamID)
	if err != nil {
		p.apis.LogAPI.Error("Failed to get violation count", err, map[string]interface{}{
			"steam_id": chatEvent.SteamID,
		})
		currentCount = 0
	}

	// New violation count
	newCount := currentCount + 1

	// Determine action
	var action *EscalationAction

	if p.getBoolConfig("use_server_rule_actions") && p.getStringConfig("rule_id") != "" {
		// Try to get actions from server rules
		serverActions, err := p.getServerRuleActions()
		if err != nil {
			p.apis.LogAPI.Warn("Failed to get server rule actions, using plugin config", map[string]interface{}{
				"error": err.Error(),
			})
			action = DetermineAction(newCount, p.escalationActions)
		} else {
			action = DetermineAction(newCount, serverActions)
		}
	} else {
		action = DetermineAction(newCount, p.escalationActions)
	}

	if action == nil {
		p.apis.LogAPI.Warn("No escalation action found for violation count", map[string]interface{}{
			"violation_count": newCount,
			"steam_id":        chatEvent.SteamID,
		})
		return nil
	}

	// Format message with placeholders
	message := p.formatMessage(action.Message, result.Category)

	// Execute action
	var actionErr error
	switch action.Action {
	case "WARN":
		actionErr = p.apis.RconAPI.SendWarningToPlayer(chatEvent.SteamID, message)
	case "KICK":
		actionErr = p.apis.RconAPI.KickPlayer(chatEvent.SteamID, message)
	case "BAN":
		actionErr = p.executeBan(chatEvent, eventID, message, action.BanDurationDays)
	}

	if actionErr != nil {
		p.apis.LogAPI.Error("Failed to execute moderation action", actionErr, map[string]interface{}{
			"action":    action.Action,
			"steam_id":  chatEvent.SteamID,
			"player":    chatEvent.PlayerName,
		})
		return actionErr
	}

	// Record violation
	if err := p.tracker.RecordViolation(
		chatEvent.SteamID,
		eventID.String(),
		result.Category,
		action.Action,
		chatEvent.Message,
	); err != nil {
		p.apis.LogAPI.Error("Failed to record violation", err, map[string]interface{}{
			"steam_id": chatEvent.SteamID,
		})
	}

	p.apis.LogAPI.Info("Moderation action executed", map[string]interface{}{
		"action":          action.Action,
		"player_name":     chatEvent.PlayerName,
		"steam_id":        chatEvent.SteamID,
		"violation_count": newCount,
		"category":        string(result.Category),
		"message":         message,
	})

	return nil
}

// executeBan performs a ban with evidence linking
func (p *ChatAutoModPlugin) executeBan(chatEvent *event_manager.RconChatMessageData, eventID uuid.UUID, reason string, durationDays int) error {
	duration := time.Duration(durationDays*24) * time.Hour

	// Use BanWithEvidence to link the chat message as evidence
	banID, err := p.apis.RconAPI.BanWithEvidence(
		chatEvent.SteamID,
		reason,
		duration,
		eventID.String(),
		"RCON_CHAT_MESSAGE",
	)

	if err != nil {
		return fmt.Errorf("failed to ban player: %w", err)
	}

	p.apis.LogAPI.Info("Player banned with evidence", map[string]interface{}{
		"ban_id":        banID,
		"steam_id":      chatEvent.SteamID,
		"player_name":   chatEvent.PlayerName,
		"duration_days": durationDays,
		"event_id":      eventID.String(),
	})

	return nil
}

// formatMessage replaces placeholders in a message
func (p *ChatAutoModPlugin) formatMessage(message string, category FilterCategory) string {
	// Replace {rule_id} with display ID
	ruleDisplayID := p.getStringConfig("rule_display_id")
	if ruleDisplayID == "" {
		ruleDisplayID = "server rules"
	}
	message = strings.ReplaceAll(message, "{rule_id}", ruleDisplayID)

	// Replace {category} with human-readable category
	categoryName := GetCategoryDisplayName(category)
	message = strings.ReplaceAll(message, "{category}", categoryName)

	return message
}

// getServerRuleActions queries the server_rule_actions table for escalation
func (p *ChatAutoModPlugin) getServerRuleActions() ([]EscalationAction, error) {
	ruleID := p.getStringConfig("rule_id")
	if ruleID == "" {
		return nil, fmt.Errorf("rule_id not configured")
	}

	query := `
		SELECT violation_count, action_type, duration, message
		FROM server_rule_actions
		WHERE rule_id = $1
		ORDER BY violation_count ASC
	`

	rows, err := p.apis.DatabaseAPI.ExecuteQuery(query, ruleID)
	if err != nil {
		return nil, fmt.Errorf("failed to query server rule actions: %w", err)
	}
	defer rows.Close()

	var actions []EscalationAction
	for rows.Next() {
		var action EscalationAction
		var duration *int
		var message *string

		if err := rows.Scan(&action.ViolationCount, &action.Action, &duration, &message); err != nil {
			return nil, fmt.Errorf("failed to scan rule action: %w", err)
		}

		if duration != nil {
			action.BanDurationDays = *duration
		}
		if message != nil {
			action.Message = *message
		} else {
			action.Message = "Server rule violation"
		}

		actions = append(actions, action)
	}

	return actions, nil
}

// isPlayerAdmin checks if a player is a server admin (cached)
func (p *ChatAutoModPlugin) isPlayerAdmin(steamID string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Refresh cache if older than 5 minutes
	if time.Since(p.adminCacheTime) > 5*time.Minute {
		p.refreshAdminCache()
	}

	return p.adminCache[steamID]
}

// refreshAdminCache updates the admin cache
func (p *ChatAutoModPlugin) refreshAdminCache() {
	admins, err := p.apis.ServerAPI.GetAdmins()
	if err != nil {
		p.apis.LogAPI.Warn("Failed to refresh admin cache", map[string]interface{}{
			"error": err.Error(),
		})
		return
	}

	p.adminCache = make(map[string]bool)
	for _, admin := range admins {
		p.adminCache[admin.SteamID] = true
	}
	p.adminCacheTime = time.Now()
}

// GetStatus returns the current plugin status
func (p *ChatAutoModPlugin) GetStatus() plugin_manager.PluginStatus {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.status
}

// GetConfig returns the current plugin configuration
func (p *ChatAutoModPlugin) GetConfig() map[string]interface{} {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.config
}

// UpdateConfig updates the plugin configuration
func (p *ChatAutoModPlugin) UpdateConfig(config map[string]interface{}) error {
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

	// Reinitialize filters with new config
	if err := p.initializeFilters(); err != nil {
		return fmt.Errorf("failed to reinitialize filters: %w", err)
	}

	// Reparse escalation actions
	if err := p.parseEscalationActions(); err != nil {
		return fmt.Errorf("failed to reparse escalation actions: %w", err)
	}

	// Update tracker expiry
	expiryDays := p.getIntConfig("violation_expiry_days")
	p.tracker = NewViolationTracker(p.apis.DatabaseAPI, expiryDays)

	p.apis.LogAPI.Info("Chat AutoMod plugin configuration updated", map[string]interface{}{
		"region":                  p.getStringConfig("region"),
		"violation_expiry_days":   expiryDays,
		"escalation_action_count": len(p.escalationActions),
	})

	return nil
}

// Config helper methods
func (p *ChatAutoModPlugin) getStringConfig(key string) string {
	return plug_config_schema.GetStringValue(p.config, key)
}

func (p *ChatAutoModPlugin) getBoolConfig(key string) bool {
	return plug_config_schema.GetBoolValue(p.config, key)
}

func (p *ChatAutoModPlugin) getIntConfig(key string) int {
	return plug_config_schema.GetIntValue(p.config, key)
}

func (p *ChatAutoModPlugin) getArrayStringConfig(key string) []string {
	return plug_config_schema.GetArrayStringValue(p.config, key)
}
