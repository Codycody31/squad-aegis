package connector_manager

import (
	"github.com/google/uuid"
	"github.com/iamalone98/eventEmitter"
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

// ConnectorScope defines the scope of a connector
type ConnectorScope string

const (
	// ConnectorScopeGlobal indicates a connector that is instantiated once globally
	ConnectorScopeGlobal ConnectorScope = "global"
	// ConnectorScopeServer indicates a connector that is instantiated per server
	ConnectorScopeServer ConnectorScope = "server"
)

// ConnectorFlags represents the capabilities of a connector
type ConnectorFlags struct {
	// ImplementsEvents indicates if the connector can emit events that can be handled by the extension manager
	ImplementsEvents bool `json:"implements_events"`
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
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Version     string `json:"version"`
	Author      string `json:"author"`

	// Scope indicates if this connector is global or per-server
	Scope ConnectorScope `json:"scope"`

	// Flags indicating connector capabilities
	Flags ConnectorFlags `json:"flags"`

	// Configuration schema for this connector
	ConfigSchema plug_config_schema.ConfigSchema `json:"config_schema"`

	// Event handlers this connector provides
	EventHandlers []EventHandler `json:"event_handlers"`

	// Factory method to create new instances
	CreateInstance func() Connector `json:"-"`
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
	ID         uuid.UUID
	Definition ConnectorDefinition
	Config     map[string]interface{}

	EventEmitter eventEmitter.EventEmitter
}

// Initialize initializes the connector with its configuration
func (b *ConnectorBase) Initialize(config map[string]interface{}) error {
	b.Config = config

	if b.Definition.Flags.ImplementsEvents {
		b.EventEmitter = eventEmitter.NewEventEmitter()
	}

	return nil
}

// Shutdown gracefully shuts down the connector
func (b *ConnectorBase) Shutdown() error {
	return nil
}

// GetDefinition returns the connector's definition
func (b *ConnectorBase) GetDefinition() ConnectorDefinition {
	return b.Definition
}

// EmitEvent emits an event for the connector manager to handle
func (b *ConnectorBase) EmitEvent(eventType string, data interface{}) {
	if b.EventEmitter == nil {
		return
	}

	b.EventEmitter.Emit(eventType, data)
}

// ConnectorRegistrar is the interface that connector packages must implement
type ConnectorRegistrar interface {
	// Define returns the connector definition
	Define() ConnectorDefinition
}
