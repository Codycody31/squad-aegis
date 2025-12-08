package kill_broadcast

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"strings"
	"sync"
	"time"

	"go.codycody31.dev/squad-aegis/internal/event_manager"
	"go.codycody31.dev/squad-aegis/internal/plugin_manager"
	"go.codycody31.dev/squad-aegis/internal/shared/plug_config_schema"
)

// intPtr returns a pointer to an int value
func intPtr(i int) *int {
	return &i
}

// KillType defines the structure of a kill type configuration
type KillType struct {
	Enabled bool     `json:"enabled"`
	Heli    bool     `json:"heli"`
	Seeding bool     `json:"seeding"`
	Layout  string   `json:"layout"`
	IDs     []string `json:"ids"`
	Verbs   []string `json:"verbs"`
}

// KillBroadcastPlugin broadcasts to the Squad server when a player gets a certain type of kill
type KillBroadcastPlugin struct {
	// Plugin configuration
	config map[string]interface{}
	apis   *plugin_manager.PluginAPIs

	// State management
	mu     sync.Mutex
	status plugin_manager.PluginStatus
	ctx    context.Context
	cancel context.CancelFunc

	// Plugin-specific state
	loadedKillTypes []KillType
	messages        []string
	messagesMu      sync.Mutex
}

// Define returns the plugin definition
func Define() plugin_manager.PluginDefinition {
	return plugin_manager.PluginDefinition{
		ID:                     "kill_broadcast",
		Name:                   "Kill Broadcast",
		Description:            "Broadcast to the Squad server when a player gets a certain type of kill.",
		Version:                "1.0.0",
		Author:                 "Squad Aegis",
		AllowMultipleInstances: false,
		RequiredConnectors:     []string{},
		LongRunning:            false,

		ConfigSchema: plug_config_schema.ConfigSchema{
			Fields: []plug_config_schema.ConfigField{
				plug_config_schema.NewBoolField(
					"use_interval",
					"Use interval-based broadcasting rather than broadcasting right away.",
					false,
					false,
				),
				plug_config_schema.NewIntField(
					"interval_ms",
					"Interval in milliseconds for broadcasting queued messages (only used when use_interval is true).",
					false,
					5000,
				),
				plug_config_schema.NewArrayObjectField(
					"broadcasts",
					"Array of kill type configurations for different types of kills.",
					false,
					[]plug_config_schema.ConfigField{
						plug_config_schema.NewBoolField(
							"enabled",
							"Whether this kill type is enabled.",
							false,
							true,
						),
						plug_config_schema.NewBoolField(
							"heli",
							"Only use to specify heli kills (self-kills in helicopters).",
							false,
							false,
						),
						plug_config_schema.NewBoolField(
							"seeding",
							"Whether this kill type executes while on a seeding map. Set to false to disable on seeding maps.",
							false,
							true,
						),
						plug_config_schema.NewStringField(
							"layout",
							"Message layout. Available placeholders: {{attacker}}, {{verb}}, {{victim}}, {{damage}}, {{weapon}}",
							false,
							"{{attacker}} {{verb}} {{victim}}",
						),
						{
							Name:        "ids",
							Description: "Array of weapon IDs to match. Use weapon blueprint names from Squad.",
							Required:    true,
							Type:        plug_config_schema.FieldTypeArrayString,
							Default:     []interface{}{},
							MinItems:    intPtr(1),
						},
						{
							Name:        "verbs",
							Description: "Array of random verbs to use (leave empty if {{verb}} is not included in layout).",
							Required:    false,
							Type:        plug_config_schema.FieldTypeArrayString,
							Default:     []interface{}{},
						},
					},
					[]interface{}{
						map[string]interface{}{
							"enabled": true,
							"heli":    true,
							"seeding": true,
							"layout":  "{{attacker}} {{verb}}",
							"ids": []interface{}{
								"BP_MI8_AFU",
								"BP_MI8_VDV",
								"BP_UH1Y",
								"BP_UH60",
								"BP_UH1H_Desert",
								"BP_UH1H",
								"BP_CH178",
								"BP_MI8",
								"BP_CH146",
								"BP_MI17_MEA",
								"BP_Z8G",
								"BP_CH146_Desert",
								"BP_SA330",
								"BP_UH60_AUS",
								"BP_MRH90_Mag58",
								"BP_Z8J",
								"BP_Loach_CAS_Small",
								"BP_Loach",
								"BP_UH60_TLF_PKM",
								"BP_CH146_Raven",
							},
							"verbs": []interface{}{
								"CRASHED LANDED",
								"MADE A FLAWLESS LANDING",
								"YOU CAN'T PARK THERE",
							},
						},
						map[string]interface{}{
							"enabled": true,
							"heli":    false,
							"seeding": true,
							"layout":  "{{attacker}} {{verb}} {{victim}}",
							"ids": []interface{}{
								"BP_AK74Bayonet",
								"BP_AKMBayonet",
								"BP_Bayonet2000",
								"BP_G3Bayonet",
								"BP_M9Bayonet",
								"BP_OKC-3S",
								"BP_QNL-95_Bayonet",
								"BP_SA80Bayonet",
								"BP_SKS_Bayonet",
								"BP_SKS_Optic_Bayonet",
								"BP_SOCP_Knife_AUS",
								"BP_SOCP_Knife_ADF",
								"BP_VibroBlade_Knife_GC",
								"BP_MeleeUbop",
								"BP_BananaClub",
								"BP_Droid_Punch",
								"BP_MagnaGuard_Punch",
								"BP_FAMAS_Bayonet",
								"BP_FAMAS_BayonetRifle",
								"BP_HK416_Bayonet",
							},
							"verbs": []interface{}{
								"KNIFED",
								"SLICED",
								"DICED",
								"ICED",
								"CUT",
								"PAPER CUT",
								"RAZORED",
								"EDWARD SCISSOR HAND'D",
								"FRUIT NINJA'D",
								"TERMINATED",
								"DELETED",
								"ASSASSINATED",
							},
						},
					},
				),
			},
		},

		Events: []event_manager.EventType{
			event_manager.EventTypeLogPlayerWounded,
		},

		CreateInstance: func() plugin_manager.Plugin {
			return &KillBroadcastPlugin{}
		},
	}
}

// GetDefinition returns the plugin definition
func (p *KillBroadcastPlugin) GetDefinition() plugin_manager.PluginDefinition {
	return Define()
}

// Initialize initializes the plugin with its configuration and dependencies
func (p *KillBroadcastPlugin) Initialize(config map[string]interface{}, apis *plugin_manager.PluginAPIs) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.config = config
	p.apis = apis
	p.status = plugin_manager.PluginStatusStopped
	p.messages = make([]string, 0)
	p.loadedKillTypes = make([]KillType, 0)

	// Validate config
	definition := p.GetDefinition()
	if err := definition.ConfigSchema.Validate(config); err != nil {
		return fmt.Errorf("invalid config: %w", err)
	}

	// Fill defaults
	definition.ConfigSchema.FillDefaults(config)

	// Parse broadcasts configuration using schema helper
	broadcasts := plug_config_schema.GetArrayObjectValue(config, "broadcasts")
	for _, broadcastMap := range broadcasts {
		killType := p.parseKillType(broadcastMap)
		if killType.Enabled && len(killType.IDs) > 0 {
			p.loadedKillTypes = append(p.loadedKillTypes, killType)
		}
	}

	p.status = plugin_manager.PluginStatusStopped

	return nil
}

// parseKillType converts a map to a KillType struct with defaults
func (p *KillBroadcastPlugin) parseKillType(m map[string]interface{}) KillType {
	// Get enabled with default true
	enabled := true
	if val := plug_config_schema.GetBoolValue(m, "enabled"); m["enabled"] != nil {
		enabled = val
	}

	// Get heli with default false
	heli := false
	if m["heli"] != nil {
		heli = plug_config_schema.GetBoolValue(m, "heli")
	}

	// Get seeding with default true
	seeding := true
	if val := plug_config_schema.GetBoolValue(m, "seeding"); m["seeding"] != nil {
		seeding = val
	}

	// Get layout with default
	layout := plug_config_schema.GetStringValue(m, "layout")
	if layout == "" {
		layout = "{{attacker}} {{verb}} {{victim}}"
	}

	kt := KillType{
		Enabled: enabled,
		Heli:    heli,
		Seeding: seeding,
		Layout:  layout,
		IDs:     plug_config_schema.GetArrayStringValue(m, "ids"),
		Verbs:   plug_config_schema.GetArrayStringValue(m, "verbs"),
	}

	return kt
}

// Start starts the plugin
func (p *KillBroadcastPlugin) Start(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.status == plugin_manager.PluginStatusRunning {
		return nil // Already running
	}

	p.ctx, p.cancel = context.WithCancel(ctx)

	// Start interval-based broadcasting if enabled
	if p.getBoolConfig("use_interval") {
		intervalMs := p.getIntConfig("interval_ms")
		if intervalMs <= 0 {
			intervalMs = 5000 // Default to 5 seconds
		}
		go p.intervalBroadcastLoop(time.Duration(intervalMs) * time.Millisecond)
	}

	p.status = plugin_manager.PluginStatusRunning
	return nil
}

// Stop stops the plugin
func (p *KillBroadcastPlugin) Stop() error {
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

// GetStatus returns the current status of the plugin
func (p *KillBroadcastPlugin) GetStatus() plugin_manager.PluginStatus {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.status
}

// GetConfig returns the current plugin configuration
func (p *KillBroadcastPlugin) GetConfig() map[string]interface{} {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.config
}

// UpdateConfig updates the plugin configuration
func (p *KillBroadcastPlugin) UpdateConfig(config map[string]interface{}) error {
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

	// Re-parse kill types using schema helper
	p.loadedKillTypes = make([]KillType, 0)
	broadcasts := plug_config_schema.GetArrayObjectValue(config, "broadcasts")
	for _, broadcastMap := range broadcasts {
		killType := p.parseKillType(broadcastMap)
		if killType.Enabled && len(killType.IDs) > 0 {
			p.loadedKillTypes = append(p.loadedKillTypes, killType)
		}
	}

	return nil
}

// HandleEvent handles events for the plugin
func (p *KillBroadcastPlugin) HandleEvent(event *plugin_manager.PluginEvent) error {
	data, ok := event.Data.(*event_manager.LogPlayerWoundedData)
	if !ok {
		return fmt.Errorf("invalid event data type")
	}

	return p.onKill(data)
}

// onKill handles player wounded events
func (p *KillBroadcastPlugin) onKill(data *event_manager.LogPlayerWoundedData) error {
	// Find matching kill type
	var matchedKillType *KillType
	for i := range p.loadedKillTypes {
		for _, id := range p.loadedKillTypes[i].IDs {
			if strings.Contains(data.Weapon, id) {
				matchedKillType = &p.loadedKillTypes[i]
				break
			}
		}
		if matchedKillType != nil {
			break
		}
	}

	if matchedKillType == nil {
		return nil
	}

	// Check seeding mode
	if !matchedKillType.Seeding {
		// Get current map info from server
		serverInfo, err := p.apis.ServerAPI.GetServerInfo()
		if err == nil && serverInfo != nil && serverInfo.CurrentMap != "" {
			if strings.Contains(strings.ToLower(serverInfo.CurrentMap), "seed") {
				return nil
			}
		}
	}

	// Determine if we should broadcast
	shouldBroadcast := false
	if !data.Teamkill {
		shouldBroadcast = true
	} else if matchedKillType.Heli && data.AttackerEOS == data.VictimEOS {
		// Self-kill in a heli
		shouldBroadcast = true
	}

	if !shouldBroadcast {
		return nil
	}

	// Build the message
	message := p.buildMessage(matchedKillType, data)

	// Broadcast or queue the message
	if p.getBoolConfig("use_interval") {
		p.queueMessage(message)
	} else {
		if err := p.apis.RconAPI.Broadcast(message); err != nil {
			return fmt.Errorf("failed to broadcast message: %w", err)
		}
	}

	return nil
}

// buildMessage constructs the message from the layout template
func (p *KillBroadcastPlugin) buildMessage(kt *KillType, data *event_manager.LogPlayerWoundedData) string {
	message := kt.Layout

	// Replace template variables
	replacements := map[string]string{
		"{{attacker}}": data.AttackerName,
		"{{victim}}":   data.VictimName,
		"{{weapon}}":   data.Weapon,
		"{{damage}}":   data.Damage,
	}

	// Pick a random verb if verbs are available
	if len(kt.Verbs) > 0 {
		verb := p.pickRandom(kt.Verbs)
		replacements["{{verb}}"] = verb
	}

	for placeholder, value := range replacements {
		message = strings.ReplaceAll(message, placeholder, value)
	}

	return message
}

// pickRandom randomly selects an item from a slice
func (p *KillBroadcastPlugin) pickRandom(items []string) string {
	if len(items) == 0 {
		return ""
	}

	n, err := rand.Int(rand.Reader, big.NewInt(int64(len(items))))
	if err != nil {
		return items[0]
	}

	return items[n.Int64()]
}

// queueMessage adds a message to the broadcast queue
func (p *KillBroadcastPlugin) queueMessage(message string) {
	p.messagesMu.Lock()
	defer p.messagesMu.Unlock()
	p.messages = append(p.messages, message)
}

// intervalBroadcastLoop broadcasts queued messages at intervals
func (p *KillBroadcastPlugin) intervalBroadcastLoop(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-p.ctx.Done():
			return
		case <-ticker.C:
			p.messagesMu.Lock()
			if len(p.messages) > 0 {
				// Pop the last message (LIFO like the JS version)
				message := p.messages[len(p.messages)-1]
				p.messages = p.messages[:len(p.messages)-1]
				p.messagesMu.Unlock()

				// Broadcast the message
				if err := p.apis.RconAPI.Broadcast(message); err != nil {
					// Log error but continue
					p.apis.LogAPI.Error("Failed to broadcast queued message", err, nil)
				}
			} else {
				p.messagesMu.Unlock()
			}
		}
	}
}

// Helper methods for config access

func (p *KillBroadcastPlugin) getBoolConfig(key string) bool {
	if value, ok := p.config[key].(bool); ok {
		return value
	}
	return false
}

func (p *KillBroadcastPlugin) getIntConfig(key string) int {
	if value, ok := p.config[key].(int); ok {
		return value
	}
	// Handle float64 from JSON
	if value, ok := p.config[key].(float64); ok {
		return int(value)
	}
	return 0
}

func (p *KillBroadcastPlugin) getStringConfig(key string) string {
	if value, ok := p.config[key].(string); ok {
		return value
	}
	return ""
}
