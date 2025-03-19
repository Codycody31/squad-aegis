package connector_manager

import (
	"github.com/google/uuid"
	"go.codycody31.dev/squad-aegis/shared/plug_config_schema"
)

// Connector represents a loaded connector instance
type Connector interface {
	// Initialize initializes the connector with its configuration
	Initialize(config map[string]interface{}) error
	// Shutdown gracefully shuts down the connector
	Shutdown() error
	// GetDefinition returns the connector's definition
	GetDefinition() ConnectorDefinition
}

// ConnectorDefinition contains all metadata and configuration for a connector
//
// To implement a new connector:
// 1. Create a new package in the connectors directory
// 2. Define a struct that embeds ConnectorBase
// 3. Implement the Initialize and Shutdown methods
// 4. Create a Define() function that returns a ConnectorDefinition
// 5. Create a CreateInstance function
// 6. Create a registrar.go file that implements ConnectorRegistrar
//
// See connectors/template for an example implementation.
type ConnectorDefinition struct {
	// Basic metadata
	ID          string
	Name        string
	Description string
	Version     string
	Author      string

	// Configuration schema for this connector
	ConfigSchema plug_config_schema.ConfigSchema

	// Event handlers this connector provides
	EventHandlers []EventHandler

	// Factory method to create new instances
	CreateInstance func() Connector
}

// EventHandlerSource defines the source of an event
type EventHandlerSource string

const (
	EventHandlerSourceRCON EventHandlerSource = "RCON"
	EventHandlerSourceLOGS EventHandlerSource = "LOGS"
)

// EventHandler defines a specific event that can be handled by the connector
type EventHandler struct {
	// Source of the event (e.g., "RCON", "LOGS", etc.)
	Source EventHandlerSource

	// Name of the event (e.g., "CHAT", "PLAYER_CONNECTED", etc.)
	Name string

	// Description of what this handler does
	Description string

	// The actual handler function that processes the event data
	Handler func(c Connector, data interface{}) error
}

// ConnectorBase provides a base implementation for connectors
//
// This struct can be embedded in your connector implementation to provide
// default implementations of the Connector interface methods.
//
// At minimum, you should implement the Initialize and Shutdown methods
// in your connector, calling the base methods as appropriate.
type ConnectorBase struct {
	Definition  ConnectorDefinition
	ID          uuid.UUID
	Config      map[string]interface{}
	Initialized bool
}

// Initialize initializes the base connector - can be extended by implementing connectors
func (b *ConnectorBase) Initialize(config map[string]interface{}) error {
	b.Config = config
	b.Initialized = true
	return nil
}

// Shutdown provides a basic shutdown implementation
func (b *ConnectorBase) Shutdown() error {
	b.Initialized = false
	return nil
}

func (b *ConnectorBase) GetDefinition() ConnectorDefinition {
	return b.Definition
}

// ConnectorRegistrar is the interface that connector packages must implement
type ConnectorRegistrar interface {
	// Define returns the connector definition
	Define() ConnectorDefinition
}
