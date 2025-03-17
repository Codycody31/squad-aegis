package intervalled_broadcasts

import (
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"go.codycody31.dev/squad-aegis/internal/extension_manager"
	"go.codycody31.dev/squad-aegis/internal/rcon_manager"
	"go.codycody31.dev/squad-aegis/shared/plug_config_schema"
)

// IntervalledBroadcastsExtension sends periodic RCON messages to server
type IntervalledBroadcastsExtension struct {
	extension_manager.ExtensionBase
	rconManager *rcon_manager.RconManager
	serverID    uuid.UUID
	ticker      *time.Ticker
	stopChan    chan struct{}
	mu          sync.Mutex
}

// Define implements ExtensionRegistrar
func Define() extension_manager.ExtensionDefinition {
	return extension_manager.ExtensionDefinition{
		ID:          "intervalled_broadcasts",
		Name:        "Intervalled Broadcasts",
		Description: "Allows you to set broadcasts, which will be broadcasted at a set interval.",
		Version:     "1.0.0",
		Author:      "Squad Aegis",

		Dependencies: extension_manager.ExtensionDependencies{
			Required: []extension_manager.DependencyType{
				extension_manager.DependencyServer,
				extension_manager.DependencyRconManager,
			},
		},

		ConfigSchema: plug_config_schema.ConfigSchema{
			Fields: []plug_config_schema.ConfigField{
				{
					Name:        "interval_seconds",
					Description: "The interval between broadcasts in seconds",
					Required:    true,
					Type:        plug_config_schema.FieldTypeInt,
					Default:     300,
				},
				{
					Name:        "messages",
					Description: "List of messages to broadcast",
					Required:    true,
					Type:        plug_config_schema.FieldTypeArray,
					Default:     []string{"This server is powered by Squad Aegis."},
				},
				{
					Name:        "prefix",
					Description: "Prefix to add before each message",
					Required:    false,
					Type:        plug_config_schema.FieldTypeString,
					Default:     "",
				},
			},
		},

		CreateInstance: func() extension_manager.Extension {
			return &IntervalledBroadcastsExtension{}
		},
	}
}

// Initialize initializes the extension with its configuration and dependencies
func (e *IntervalledBroadcastsExtension) Initialize(config map[string]interface{}, deps *extension_manager.Dependencies) error {
	// Set the base extension properties
	e.Definition = Define()
	e.Config = config
	e.Deps = deps

	// Get RCON manager from dependencies
	if e.Deps.RconManager == nil {
		return fmt.Errorf("RCON manager dependency not provided")
	}
	e.rconManager = e.Deps.RconManager

	// Get server ID
	if e.Deps.Server == nil {
		return fmt.Errorf("server dependency not provided")
	}
	e.serverID = e.Deps.Server.Id

	// Validate config
	if err := e.Definition.ConfigSchema.Validate(config); err != nil {
		return err
	}

	// Fill defaults
	e.Definition.ConfigSchema.FillDefaults(config)

	// Start the broadcast ticker
	return e.startBroadcasts()
}

// startBroadcasts starts the periodic broadcast messages
func (e *IntervalledBroadcastsExtension) startBroadcasts() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	// Get interval from config
	intervalSeconds, ok := e.Config["interval_seconds"].(float64)
	if !ok {
		intervalSeconds = 300 // Default to 5 minutes if not specified
	}

	// Get messages from config
	messagesRaw, ok := e.Config["messages"].([]interface{})
	if !ok || len(messagesRaw) == 0 {
		return fmt.Errorf("no messages configured for broadcasts")
	}

	// Get prefix from config
	prefix, _ := e.Config["prefix"].(string)

	// Convert messages to strings
	messages := make([]string, len(messagesRaw))
	for i, msg := range messagesRaw {
		if strMsg, ok := msg.(string); ok {
			messages[i] = strMsg
		} else {
			messages[i] = fmt.Sprintf("%v", msg)
		}
	}

	// Create channels for stopping the ticker
	e.stopChan = make(chan struct{})
	e.ticker = time.NewTicker(time.Duration(intervalSeconds) * time.Second)

	log.Info().
		Str("extension", "intervalled_broadcasts").
		Str("serverID", e.serverID.String()).
		Int("message_count", len(messages)).
		Float64("interval_seconds", intervalSeconds).
		Msg("Starting broadcast messages")

	// Start the broadcast loop
	currentMessageIndex := 0
	go func() {
		for {
			select {
			case <-e.ticker.C:
				// Get the next message
				message := messages[currentMessageIndex]
				currentMessageIndex = (currentMessageIndex + 1) % len(messages)

				// Add prefix if configured
				if prefix != "" {
					message = fmt.Sprintf("%s %s", prefix, message)
				}

				// Send the broadcast
				broadcastCmd := fmt.Sprintf("AdminBroadcast %s", message)
				_, err := e.rconManager.ExecuteCommand(e.serverID, broadcastCmd)
				if err != nil {
					log.Error().
						Str("extension", "intervalled_broadcasts").
						Str("serverID", e.serverID.String()).
						Err(err).
						Msg("Failed to send broadcast message")
				} else {
					log.Debug().
						Str("extension", "intervalled_broadcasts").
						Str("serverID", e.serverID.String()).
						Str("message", message).
						Msg("Sent broadcast message")
				}
			case <-e.stopChan:
				e.ticker.Stop()
				return
			}
		}
	}()

	return nil
}

// Shutdown gracefully shuts down the extension
func (e *IntervalledBroadcastsExtension) Shutdown() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	// Stop the ticker if it's running
	if e.ticker != nil {
		// Stop the ticker first
		e.ticker.Stop()

		// Then signal the goroutine to exit
		if e.stopChan != nil {
			close(e.stopChan)
		}

		e.ticker = nil
		e.stopChan = nil
	}

	log.Info().
		Str("extension", "intervalled_broadcasts").
		Str("serverID", e.serverID.String()).
		Msg("Shutting down broadcast messages")

	return nil
}
