package heartbeat

import (
	"context"
	"time"

	"go.codycody31.dev/squad-aegis/internal/connector_manager"
)

// HeartbeatConnector implements a simple connector that publishes events every 15 seconds
type HeartbeatConnector struct {
	connector_manager.ConnectorBase
	ctx    context.Context
	cancel context.CancelFunc
}

// Initialize sets up the heartbeat connector
func (c *HeartbeatConnector) Initialize(config map[string]interface{}) error {
	// Call the base Initialize to set up common fields
	if err := c.ConnectorBase.Initialize(config); err != nil {
		return err
	}

	c.ctx, c.cancel = context.WithCancel(context.Background())

	go func() {
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				c.EmitEvent("heartbeat", nil)
			case <-c.ctx.Done():
				return
			}
		}
	}()

	return nil
}

// Shutdown stops the heartbeat connector
func (c *HeartbeatConnector) Shutdown() error {
	if c.cancel != nil {
		c.cancel()
	}
	return nil
}

func Define() connector_manager.ConnectorDefinition {
	return connector_manager.ConnectorDefinition{
		ID:          "heartbeat",
		Name:        "Heartbeat",
		Description: "A simple connector that publishes heartbeat events every 15 seconds",
		Version:     "1.0.0",
		Author:      "Squad Aegis",
		Scope:       connector_manager.ConnectorScopeServer,
		Flags: connector_manager.ConnectorFlags{
			ImplementsEvents: true,
		},
		CreateInstance: func() connector_manager.Connector {
			connector := &HeartbeatConnector{}
			connector.Definition = Define()
			return connector
		},
	}
}
