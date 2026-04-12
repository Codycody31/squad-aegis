// Subprocess-isolated native connector example. Build as a standalone binary:
//
//	go build -o hello-connector .
//
// Package the resulting binary into a connector bundle alongside a
// manifest.json whose target.library_path points at the binary. The Aegis
// host will launch it via hashicorp/go-plugin and communicate over net/rpc.
package main

import (
	"context"
	"fmt"
	"sync"

	connectorrpc "go.codycody31.dev/squad-aegis/pkg/connectorrpc"
)

type helloConnector struct {
	mu     sync.RWMutex
	config map[string]interface{}
	status connectorrpc.ConnectorStatus
}

// definition returns the connector's runtime behavior. Identity (name,
// version, author, legacy_ids, instance_key, license, target OS/arch,
// sha256) lives in the signed manifest.json shipped alongside the binary.
func definition() connectorrpc.ConnectorDefinition {
	return connectorrpc.ConnectorDefinition{
		ConnectorID: "com.squad-aegis.connectors.examples.hello",
		ConfigSchema: connectorrpc.ConfigSchema{
			Fields: []connectorrpc.ConfigField{},
		},
	}
}

func main() {
	connectorrpc.Serve(&helloConnector{})
}

func (c *helloConnector) GetDefinition() connectorrpc.ConnectorDefinition {
	return definition()
}

func (c *helloConnector) Initialize(config map[string]interface{}) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.config = config
	c.status = connectorrpc.ConnectorStatusStopped
	return nil
}

func (c *helloConnector) Start(ctx context.Context) error {
	c.mu.Lock()
	c.status = connectorrpc.ConnectorStatusRunning
	c.mu.Unlock()
	go func() {
		<-ctx.Done()
		_ = c.Stop()
	}()
	return nil
}

func (c *helloConnector) Stop() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.status = connectorrpc.ConnectorStatusStopped
	return nil
}

func (c *helloConnector) GetStatus() connectorrpc.ConnectorStatus {
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

func (c *helloConnector) Invoke(ctx context.Context, req *connectorrpc.ConnectorInvokeRequest) (*connectorrpc.ConnectorInvokeResponse, error) {
	_ = ctx
	out := &connectorrpc.ConnectorInvokeResponse{V: "1"}
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
