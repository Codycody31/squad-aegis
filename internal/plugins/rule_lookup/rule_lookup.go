package rule_lookup

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"go.codycody31.dev/squad-aegis/internal/event_manager"
	"go.codycody31.dev/squad-aegis/internal/plugin_manager"
	"go.codycody31.dev/squad-aegis/internal/shared/plug_config_schema"
)

// RuleLookupPlugin handles rule lookup commands in chat
type RuleLookupPlugin struct {
	// Plugin configuration
	config map[string]interface{}
	apis   *plugin_manager.PluginAPIs

	// State management
	mu     sync.Mutex
	status plugin_manager.PluginStatus
	ctx    context.Context
	cancel context.CancelFunc

	// Rule number pattern regex
	rulePattern *regexp.Regexp
}

// Define returns the plugin definition
func Define() plugin_manager.PluginDefinition {
	return plugin_manager.PluginDefinition{
		ID:                     "rule_lookup",
		Name:                   "Rule Lookup",
		Description:            "Allows players to look up server rules by typing !rule followed by a rule number (e.g., !rule 1.1)",
		Version:                "1.0.0",
		Author:                 "Squad Aegis",
		AllowMultipleInstances: false,
		RequiredConnectors:     []string{},
		LongRunning:            false,

		ConfigSchema: plug_config_schema.ConfigSchema{
			Fields: []plug_config_schema.ConfigField{
				{
					Name:        "command",
					Description: "The command prefix to trigger rule lookup (without !)",
					Required:    false,
					Type:        plug_config_schema.FieldTypeString,
					Default:     "rule",
				},
				{
					Name:        "admin_roles",
					Description: "Array of role names that are considered admins for public broadcast (empty array allows all admin roles)",
					Required:    false,
					Type:        plug_config_schema.FieldTypeArrayString,
					Default:     []interface{}{},
				},
			},
		},

		Events: []event_manager.EventType{
			event_manager.EventTypeRconChatMessage,
		},

		CreateInstance: func() plugin_manager.Plugin {
			return &RuleLookupPlugin{}
		},
	}
}

// GetDefinition returns the plugin definition
func (p *RuleLookupPlugin) GetDefinition() plugin_manager.PluginDefinition {
	return Define()
}

// Initialize initializes the plugin with its configuration and dependencies
func (p *RuleLookupPlugin) Initialize(config map[string]interface{}, apis *plugin_manager.PluginAPIs) error {
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

	// Compile regex pattern for rule numbers (supports formats like 1, 1.1, 1.1.1, etc.)
	p.rulePattern = regexp.MustCompile(`^\d+(?:\.\d+)*$`)

	p.status = plugin_manager.PluginStatusStopped

	return nil
}

// Start begins plugin execution (for long-running plugins)
func (p *RuleLookupPlugin) Start(ctx context.Context) error {
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
func (p *RuleLookupPlugin) Stop() error {
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
func (p *RuleLookupPlugin) HandleEvent(event *plugin_manager.PluginEvent) error {
	if event.Type != string(event_manager.EventTypeRconChatMessage) {
		return nil // Not interested in this event
	}

	return p.handleChatMessage(event)
}

// GetStatus returns the current plugin status
func (p *RuleLookupPlugin) GetStatus() plugin_manager.PluginStatus {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.status
}

// GetConfig returns the current plugin configuration
func (p *RuleLookupPlugin) GetConfig() map[string]interface{} {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.config
}

// UpdateConfig updates the plugin configuration
func (p *RuleLookupPlugin) UpdateConfig(config map[string]interface{}) error {
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

	p.apis.LogAPI.Info("Rule Lookup plugin configuration updated", nil)

	return nil
}

// handleChatMessage processes chat message events to detect rule lookup commands
func (p *RuleLookupPlugin) handleChatMessage(rawEvent *plugin_manager.PluginEvent) error {
	event, ok := rawEvent.Data.(*event_manager.RconChatMessageData)
	if !ok {
		return fmt.Errorf("invalid event data type")
	}

	// Check if this is a rule lookup command
	message := strings.TrimSpace(event.Message)
	command := "!" + p.getStringConfig("command")

	// Check if message starts with our command
	if !strings.HasPrefix(strings.ToLower(message), strings.ToLower(command)) {
		return nil // Not our command
	}

	// Parse the rule number from the message
	// Expected format: !rule 1.1 or !rule 7.7.1
	parts := strings.Fields(message)
	if len(parts) < 2 {
		// No rule number provided
		return p.sendHelpMessage(event.SteamID)
	}

	ruleNumber := parts[1]

	// Validate rule number format
	if !p.rulePattern.MatchString(ruleNumber) {
		return p.sendInvalidFormatMessage(event.SteamID, ruleNumber)
	}

	// Look up the rule
	return p.lookupAndSendRule(event, ruleNumber)
}

// lookupAndSendRule finds a rule by its display order pattern and sends it to players
func (p *RuleLookupPlugin) lookupAndSendRule(event *event_manager.RconChatMessageData, ruleNumber string) error {
	serverID := p.apis.ServerAPI.GetServerID()

	// Parse the rule number into components (e.g., "1.1.2" -> [1, 1, 2])
	numberParts := strings.Split(ruleNumber, ".")

	// Find the rule by traversing the hierarchy
	rule, err := p.findRuleByHierarchy(serverID.String(), numberParts)
	if err != nil {
		p.apis.LogAPI.Error("Failed to lookup rule", err, map[string]interface{}{
			"rule_number": ruleNumber,
			"player":      event.PlayerName,
		})
		return nil
	}

	if rule == nil {
		return nil
	}

	// Format the response message
	response := p.formatRuleResponse(ruleNumber, rule)

	admins, err := p.apis.ServerAPI.GetAdmins()
	if err != nil {
		p.apis.LogAPI.Error("Failed to get server admins", err, nil)
		return err
	}

	isAdmin := false
	for _, admin := range admins {
		if admin.SteamID == event.SteamID {
			permittedRoles := plug_config_schema.GetArrayStringValue(p.config, "admin_roles")
			for _, role := range permittedRoles {
				for _, adminRole := range admin.Roles {
					if role == adminRole.RoleName {
						isAdmin = true
						break
					}
				}
			}
			break
		}
	}

	// Send the response
	if isAdmin || event.ChatType == "ChatAdmin" {
		err = p.apis.RconAPI.Broadcast(response)
	} else {
		err = p.apis.RconAPI.SendWarningToPlayer(event.SteamID, response)
	}

	if err != nil {
		p.apis.LogAPI.Error("Failed to send rule response", err, map[string]interface{}{
			"rule_number": ruleNumber,
			"player":      event.PlayerName,
		})
		return err
	}

	return nil
}

// findRuleByHierarchy finds a rule by traversing the display order hierarchy
func (p *RuleLookupPlugin) findRuleByHierarchy(serverID string, numberParts []string) (*RuleData, error) {
	// Start by finding top-level rules (no parent)
	query := `
		SELECT id, title, description, display_order, parent_id
		FROM server_rules
		WHERE server_id = $1 AND parent_id IS NULL
		ORDER BY display_order ASC
	`

	rows, err := p.apis.DatabaseAPI.ExecuteQuery(query, serverID)
	if err != nil {
		return nil, fmt.Errorf("failed to query top-level rules: %w", err)
	}
	defer rows.Close()

	// Parse first number part
	targetOrder, err := strconv.Atoi(numberParts[0])
	if err != nil {
		return nil, fmt.Errorf("invalid rule number format: %s", numberParts[0])
	}

	// Find the rule with the matching display order at the top level
	var currentRule *RuleData
	currentOrder := 1
	for rows.Next() {
		var rule RuleData
		var parentID sql.NullString
		err := rows.Scan(&rule.ID, &rule.Title, &rule.Description, &rule.DisplayOrder, &parentID)
		if err != nil {
			return nil, fmt.Errorf("failed to scan rule: %w", err)
		}

		if currentOrder == targetOrder {
			currentRule = &rule
			break
		}
		currentOrder++
	}

	if currentRule == nil {
		return nil, nil // Rule not found
	}

	// If we only have one number part, we found our rule
	if len(numberParts) == 1 {
		return currentRule, nil
	}

	// Otherwise, we need to traverse down the hierarchy
	return p.findSubRule(currentRule.ID, numberParts[1:])
}

// findSubRule finds a sub-rule by traversing down the hierarchy
func (p *RuleLookupPlugin) findSubRule(parentRuleID string, numberParts []string) (*RuleData, error) {
	query := `
		SELECT id, title, description, display_order, parent_id
		FROM server_rules
		WHERE parent_id = $1
		ORDER BY display_order ASC
	`

	rows, err := p.apis.DatabaseAPI.ExecuteQuery(query, parentRuleID)
	if err != nil {
		return nil, fmt.Errorf("failed to query sub-rules: %w", err)
	}
	defer rows.Close()

	// Parse the next number part
	targetOrder, err := strconv.Atoi(numberParts[0])
	if err != nil {
		return nil, fmt.Errorf("invalid rule number format: %s", numberParts[0])
	}

	// Find the rule with the matching display order
	var currentRule *RuleData
	currentOrder := 1
	for rows.Next() {
		var rule RuleData
		var parentID sql.NullString
		err := rows.Scan(&rule.ID, &rule.Title, &rule.Description, &rule.DisplayOrder, &parentID)
		if err != nil {
			return nil, fmt.Errorf("failed to scan rule: %w", err)
		}

		if currentOrder == targetOrder {
			currentRule = &rule
			break
		}
		currentOrder++
	}

	if currentRule == nil {
		return nil, nil // Rule not found
	}

	// If we have more number parts, continue traversing
	if len(numberParts) > 1 {
		return p.findSubRule(currentRule.ID, numberParts[1:])
	}

	return currentRule, nil
}

// formatRuleResponse formats the rule for display
func (p *RuleLookupPlugin) formatRuleResponse(ruleNumber string, rule *RuleData) string {
	return fmt.Sprintf("Rule %s: %s", ruleNumber, rule.Title)
}

// sendHelpMessage sends a help message when no rule number is provided
func (p *RuleLookupPlugin) sendHelpMessage(steamID string) error {
	message := fmt.Sprintf("Usage: !%s <rule_number> (e.g., !%s 1.1)",
		p.getStringConfig("command"), p.getStringConfig("command"))
	return p.apis.RconAPI.SendWarningToPlayer(steamID, message)
}

// sendInvalidFormatMessage sends an error message for invalid rule number format
func (p *RuleLookupPlugin) sendInvalidFormatMessage(steamID, ruleNumber string) error {
	message := fmt.Sprintf("Invalid rule number format: %s. Use format like 1, 1.1, or 1.1.2", ruleNumber)
	return p.apis.RconAPI.SendWarningToPlayer(steamID, message)
}

// Helper methods for config access
func (p *RuleLookupPlugin) getStringConfig(key string) string {
	if val, ok := p.config[key].(string); ok {
		return val
	}
	return ""
}

func (p *RuleLookupPlugin) getBoolConfig(key string) bool {
	if val, ok := p.config[key].(bool); ok {
		return val
	}
	return false
}

// RuleData represents the essential rule data we need
type RuleData struct {
	ID           string
	Title        string
	Description  string
	DisplayOrder int
}
