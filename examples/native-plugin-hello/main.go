// Subprocess-isolated native plugin example. Build as a standalone binary:
//
//	go build -o hello-plugin .
//
// Package the resulting binary into a plugin bundle alongside a manifest.json
// whose target.library_path points at the binary (e.g. "bin/hello-plugin").
// The Aegis host will launch it via hashicorp/go-plugin and communicate over
// net/rpc.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	pluginrpc "go.codycody31.dev/squad-aegis/pkg/pluginrpc"
)

const helloExampleConnectorID = "com.squad-aegis.connectors.examples.hello"

// rconChatMessage is the minimal shape of an RCON chat message event the
// plugin needs. Declared locally so the subprocess doesn't have to import
// the host event package.
type rconChatMessage struct {
	SteamID    string `json:"steam_id,omitempty"`
	EOSID      string `json:"eos_id,omitempty"`
	PlayerName string `json:"player_name,omitempty"`
	Message    string `json:"message,omitempty"`
}

func (r rconChatMessage) preferredPlayerID() string {
	if strings.TrimSpace(r.EOSID) != "" {
		return r.EOSID
	}
	return r.SteamID
}

type helloPlugin struct {
	mu     sync.Mutex
	config map[string]interface{}
	apis   *pluginrpc.HostAPIs
	status pluginrpc.PluginStatus
}

// definition returns the plugin's runtime behavior. Identity (name,
// version, authors, license, official flag, min host API version, required
// capabilities, target OS/arch, sha256) lives in the signed manifest.json
// that ships alongside the binary — this struct is only the behavioral
// contract the host needs to validate config and route events.
func definition() pluginrpc.PluginDefinition {
	return pluginrpc.PluginDefinition{
		PluginID:               "com.squad-aegis.plugins.examples.hello",
		AllowMultipleInstances: false,
		LongRunning:            false,
		OptionalConnectors:     []string{helloExampleConnectorID},
		ConfigSchema: pluginrpc.ConfigSchema{
			Fields: []pluginrpc.ConfigField{
				{
					Name:        "trigger",
					Description: "Chat message that will trigger the response.",
					Type:        pluginrpc.FieldTypeString,
					Default:     "!hello",
				},
				{
					Name:        "response",
					Description: "Private message sent back to the player.",
					Type:        pluginrpc.FieldTypeString,
					Default:     "Hello from a native Squad Aegis plugin.",
				},
			},
		},
		Events: []string{"RCON_CHAT_MESSAGE"},
	}
}

func main() {
	pluginrpc.Serve(&helloPlugin{})
}

func (p *helloPlugin) GetDefinition() pluginrpc.PluginDefinition {
	return definition()
}

func (p *helloPlugin) Initialize(config map[string]interface{}, apis *pluginrpc.HostAPIs) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if config == nil {
		config = map[string]interface{}{}
	}
	p.config = applyHelloDefaults(config)
	p.apis = apis
	p.status = pluginrpc.PluginStatusStopped
	return nil
}

func applyHelloDefaults(config map[string]interface{}) map[string]interface{} {
	if _, ok := config["trigger"]; !ok {
		config["trigger"] = "!hello"
	}
	if _, ok := config["response"]; !ok {
		config["response"] = "Hello from a native Squad Aegis plugin."
	}
	return config
}

func (p *helloPlugin) Start(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.status = pluginrpc.PluginStatusRunning
	return nil
}

func (p *helloPlugin) Stop() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.status = pluginrpc.PluginStatusStopped
	return nil
}

func (p *helloPlugin) HandleEvent(event *pluginrpc.PluginEvent) error {
	if event == nil || event.Type != "RCON_CHAT_MESSAGE" {
		return nil
	}

	var data rconChatMessage
	if len(event.Data) > 0 {
		if err := json.Unmarshal(event.Data, &data); err != nil {
			return fmt.Errorf("decode chat event: %w", err)
		}
	}

	p.mu.Lock()
	trigger := strings.TrimSpace(fmt.Sprint(p.config["trigger"]))
	response := strings.TrimSpace(fmt.Sprint(p.config["response"]))
	apis := p.apis
	p.mu.Unlock()

	if trigger == "" || !strings.EqualFold(strings.TrimSpace(data.Message), trigger) {
		return nil
	}

	playerID := data.preferredPlayerID()
	if playerID == "" {
		return fmt.Errorf("chat event did not include a usable player identifier")
	}

	out := response
	if apis != nil && apis.ConnectorAPI != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		connResp, err := apis.ConnectorAPI.Call(ctx, helloExampleConnectorID, &pluginrpc.ConnectorInvokeRequest{
			V:    "1",
			Data: map[string]interface{}{"action": "ping"},
		})
		cancel()
		if err == nil && connResp != nil && connResp.OK && connResp.Data != nil {
			if s, ok := connResp.Data["message"].(string); ok && strings.TrimSpace(s) != "" {
				out = response + " (" + s + ")"
			}
		}
	}

	if apis != nil && apis.RconAPI != nil {
		if err := apis.RconAPI.SendWarningToPlayer(playerID, out); err != nil {
			return fmt.Errorf("failed to respond to player: %w", err)
		}
	}

	if apis != nil && apis.LogAPI != nil {
		apis.LogAPI.Info("Responded to hello command", map[string]interface{}{
			"player_name": data.PlayerName,
			"player_id":   playerID,
		})
	}

	return nil
}

func (p *helloPlugin) GetStatus() pluginrpc.PluginStatus {
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
	if config == nil {
		config = map[string]interface{}{}
	}
	p.config = applyHelloDefaults(config)
	return nil
}

func (p *helloPlugin) GetCommands() []pluginrpc.PluginCommand {
	return nil
}

func (p *helloPlugin) ExecuteCommand(string, map[string]interface{}) (*pluginrpc.CommandResult, error) {
	return nil, fmt.Errorf("this plugin does not expose commands")
}

func (p *helloPlugin) GetCommandExecutionStatus(string) (*pluginrpc.CommandExecutionStatus, error) {
	return nil, fmt.Errorf("this plugin does not expose commands")
}
