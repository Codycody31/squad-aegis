package main

import (
	"context"
	"fmt"
	"strings"
	"sync"

	pluginapi "go.codycody31.dev/squad-aegis/pkg/pluginapi"
)

type helloPlugin struct {
	mu     sync.Mutex
	config map[string]interface{}
	apis   *pluginapi.PluginAPIs
	status pluginapi.PluginStatus
}

func definition() pluginapi.PluginDefinition {
	return pluginapi.PluginDefinition{
		ID:                     "com.squad-aegis.plugins.examples.hello",
		Name:                   "Hello Example",
		Description:            "Replies to players who type !hello in chat.",
		Version:                "0.1.0",
		Author:                 "Squad Aegis",
		Source:                 pluginapi.PluginSourceNative,
		Official:               false,
		AllowMultipleInstances: false,
		LongRunning:            false,
		ConfigSchema: pluginapi.ConfigSchema{
			Fields: []pluginapi.ConfigField{
				{
					Name:        "trigger",
					Description: "Chat message that will trigger the response.",
					Required:    false,
					Type:        pluginapi.FieldTypeString,
					Default:     "!hello",
				},
				{
					Name:        "response",
					Description: "Private message sent back to the player.",
					Required:    false,
					Type:        pluginapi.FieldTypeString,
					Default:     "Hello from a native Squad Aegis plugin.",
				},
			},
		},
		Events: []pluginapi.EventType{
			pluginapi.EventTypeRconChatMessage,
		},
		CreateInstance: func() pluginapi.Plugin {
			return &helloPlugin{}
		},
	}
}

// GetAegisPlugin is the required native plugin entrypoint.
func GetAegisPlugin() pluginapi.PluginDefinition {
	return definition()
}

func (p *helloPlugin) GetDefinition() pluginapi.PluginDefinition {
	return definition()
}

func (p *helloPlugin) Initialize(config map[string]interface{}, apis *pluginapi.PluginAPIs) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	def := p.GetDefinition()
	if err := def.ConfigSchema.Validate(config); err != nil {
		return fmt.Errorf("invalid config: %w", err)
	}

	p.config = def.ConfigSchema.FillDefaults(config)
	p.apis = apis
	p.status = pluginapi.PluginStatusStopped

	return nil
}

func (p *helloPlugin) Start(context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.status = pluginapi.PluginStatusRunning
	return nil
}

func (p *helloPlugin) Stop() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.status = pluginapi.PluginStatusStopped
	return nil
}

func (p *helloPlugin) HandleEvent(event *pluginapi.PluginEvent) error {
	if event.Type != string(pluginapi.EventTypeRconChatMessage) {
		return nil
	}

	data, ok := event.Data.(*pluginapi.RconChatMessageData)
	if !ok {
		return fmt.Errorf("unexpected event payload %T", event.Data)
	}

	p.mu.Lock()
	trigger := strings.TrimSpace(fmt.Sprint(p.config["trigger"]))
	response := strings.TrimSpace(fmt.Sprint(p.config["response"]))
	p.mu.Unlock()

	if trigger == "" || !strings.EqualFold(strings.TrimSpace(data.Message), trigger) {
		return nil
	}

	playerID := data.PreferredPlayerID()
	if playerID == "" {
		return fmt.Errorf("chat event did not include a usable player identifier")
	}

	if err := p.apis.RconAPI.SendWarningToPlayer(playerID, response); err != nil {
		return fmt.Errorf("failed to respond to player: %w", err)
	}

	p.apis.LogAPI.Info("Responded to hello command", map[string]interface{}{
		"player_name": data.PlayerName,
		"player_id":   playerID,
	})

	return nil
}

func (p *helloPlugin) GetStatus() pluginapi.PluginStatus {
	p.mu.Lock()
	defer p.mu.Unlock()

	return p.status
}

func (p *helloPlugin) GetConfig() map[string]interface{} {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.config == nil {
		return map[string]interface{}{}
	}

	cloned := make(map[string]interface{}, len(p.config))
	for key, value := range p.config {
		cloned[key] = value
	}

	return cloned
}

func (p *helloPlugin) UpdateConfig(config map[string]interface{}) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	def := p.GetDefinition()
	if err := def.ConfigSchema.Validate(config); err != nil {
		return fmt.Errorf("invalid config: %w", err)
	}

	p.config = def.ConfigSchema.FillDefaults(config)
	return nil
}

func (p *helloPlugin) GetCommands() []pluginapi.PluginCommand {
	return nil
}

func (p *helloPlugin) ExecuteCommand(string, map[string]interface{}) (*pluginapi.CommandResult, error) {
	return nil, fmt.Errorf("this plugin does not expose commands")
}

func (p *helloPlugin) GetCommandExecutionStatus(string) (*pluginapi.CommandExecutionStatus, error) {
	return nil, fmt.Errorf("this plugin does not expose commands")
}
