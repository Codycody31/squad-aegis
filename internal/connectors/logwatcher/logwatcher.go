package logwatcher

import (
	"context"

	"github.com/rs/zerolog/log"
	"go.codycody31.dev/squad-aegis/clients/logwatcher"
	"go.codycody31.dev/squad-aegis/internal/connector_manager"
	plug_config_schema "go.codycody31.dev/squad-aegis/internal/shared/plug_config_schema"
)

type LogWatcherConnector struct {
	connector_manager.ConnectorBase
	ctx    context.Context
	cancel context.CancelFunc
}

func (c *LogWatcherConnector) Initialize(config map[string]any) error {
	if err := c.ConnectorBase.Initialize(config); err != nil {
		return err
	}

	c.ctx, c.cancel = context.WithCancel(context.Background())

	go func() {
		eventStreamer := logwatcher.NewEventStreamer(config["address"].(string), config["auth-token"].(string))
		if err := eventStreamer.Start(c.ctx); err != nil {
			log.Error().Err(err).Msg("Failed to start event streamer")
			return
		}

		for parsedEvent := range eventStreamer.GetEvents() {
			c.EmitEvent(parsedEvent.Original.Event, parsedEvent.Data)
		}
	}()

	return nil
}

func (c *LogWatcherConnector) Shutdown() error {
	if c.cancel != nil {
		c.cancel()
	}
	return nil
}

func Define() connector_manager.ConnectorDefinition {
	return connector_manager.ConnectorDefinition{
		ID:          "logwatcher",
		Name:        "Log Watcher",
		Description: "A simple connector that broadcasts events from your squad log watcher server.",
		Version:     "1.0.0",
		Author:      "Squad Aegis",
		Scope:       connector_manager.ConnectorScopeServer,
		Flags: connector_manager.ConnectorFlags{
			ImplementsEvents: true,
		},
		ConfigSchema: plug_config_schema.ConfigSchema{
			Fields: []plug_config_schema.ConfigField{
				{
					Name:        "address",
					Type:        plug_config_schema.FieldTypeString,
					Description: "The address of the log watcher server",
					Required:    true,
				},
				{
					Name:        "auth-token",
					Type:        plug_config_schema.FieldTypeString,
					Description: "The authentication token to use when connecting to the log watcher server",
					Required:    true,
				},
			},
		},
		CreateInstance: func() connector_manager.Connector {
			connector := &LogWatcherConnector{}
			connector.Definition = Define()
			return connector
		},
	}
}
