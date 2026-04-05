// Native connector example: build with:
//
//	go build -buildmode=plugin -o hello-connector.so .
//
// Bundle manifest.json (connector_id, targets, etc.) like native plugin packages; entry symbol GetAegisConnector.
package main

import (
	"context"
	"fmt"
	"sync"

	connectorapi "go.codycody31.dev/squad-aegis/pkg/connectorapi"
)

type helloConnector struct {
	mu     sync.RWMutex
	config map[string]interface{}
	status connectorapi.ConnectorStatus
}

func definition() connectorapi.ConnectorDefinition {
	return connectorapi.ConnectorDefinition{
		ID:          "com.squad-aegis.connectors.examples.hello",
		Source:      connectorapi.PluginSourceNative,
		Name:        "Hello connector example",
		Description: "Responds to JSON invoke action ping.",
		Version:     "0.1.0",
		Author:      "Squad Aegis",
		ConfigSchema: connectorapi.ConfigSchema{
			Fields: []connectorapi.ConfigField{},
		},
		CreateInstance: func() connectorapi.Connector {
			return &helloConnector{}
		},
	}
}

// GetAegisConnector is the required native connector entrypoint.
func GetAegisConnector() connectorapi.ConnectorDefinition {
	return definition()
}

func (c *helloConnector) GetDefinition() connectorapi.ConnectorDefinition {
	return definition()
}

func (c *helloConnector) Initialize(config map[string]interface{}) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.config = config
	c.status = connectorapi.ConnectorStatusStopped
	return nil
}

func (c *helloConnector) Start(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.status = connectorapi.ConnectorStatusRunning
	go func() {
		<-ctx.Done()
		_ = c.Stop()
	}()
	return nil
}

func (c *helloConnector) Stop() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.status = connectorapi.ConnectorStatusStopped
	return nil
}

func (c *helloConnector) GetStatus() connectorapi.ConnectorStatus {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.status
}

func (c *helloConnector) GetConfig() map[string]interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.config == nil {
		return map[string]interface{}{}
	}
	return c.config
}

func (c *helloConnector) UpdateConfig(config map[string]interface{}) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.config = config
	return nil
}

func (c *helloConnector) GetAPI() interface{} {
	return nil
}

func (c *helloConnector) Invoke(ctx context.Context, req *connectorapi.ConnectorInvokeRequest) (*connectorapi.ConnectorInvokeResponse, error) {
	_ = ctx
	out := &connectorapi.ConnectorInvokeResponse{V: connectorapi.ConnectorWireProtocolV1}
	if req == nil || req.Data == nil {
		out.Error = "missing data"
		return out, nil
	}
	if req.Data["action"] == "ping" {
		out.OK = true
		out.Data = map[string]interface{}{"message": "pong"}
		return out, nil
	}
	out.Error = fmt.Sprintf("unknown action %v", req.Data["action"])
	return out, nil
}
