package extension_manager

import (
	"go.codycody31.dev/squad-aegis/internal/shared/plug_config_schema"
)

// Extension represents a loaded extension instance
type Extension interface {
	// Initialize initializes the extension with its configuration and dependencies
	Initialize(config map[string]interface{}, deps *Dependencies) error

	// Shutdown gracefully shuts down the extension
	Shutdown() error

	// GetDefinition returns the extension's definition
	GetDefinition() ExtensionDefinition
}

// ExtensionDefinition contains all metadata and configuration for an extension
type ExtensionDefinition struct {
	// Basic metadata
	ID          string
	Name        string
	Description string
	Version     string
	Author      string

	// Required dependencies and connectors
	Dependencies ExtensionDependencies

	// Required connector types (e.g., "discord", etc.)
	RequiredConnectors []string

	// Optional connector types (e.g., "discord", etc.)
	OptionalConnectors []string

	// Configuration schema for this extension
	ConfigSchema plug_config_schema.ConfigSchema

	// Controls whether multiple instances of this extension can be used on a single server
	AllowMultipleInstances bool

	// Event handlers this extension provides
	EventHandlers []EventHandler

	// Factory method to create new instances
	CreateInstance func() Extension
}

// EventHandlerSource defines the source of an event
type EventHandlerSource string

const (
	EventHandlerSourceRCON      EventHandlerSource = "RCON"
	EventHandlerSourceCONNECTOR EventHandlerSource = "CONNECTOR"
)

// EventHandler defines a specific event that can be handled by the extension
type EventHandler struct {
	// Source of the event (e.g., "RCON", "LOGS", etc.)
	Source EventHandlerSource

	// Name of the event (e.g., "CHAT", "PLAYER_CONNECTED", etc.)
	Name string

	// Description of what this handler does
	Description string

	// The actual handler function that processes the event data
	// The handler is bound to the extension instance and server context
	Handler func(e Extension, data interface{}) error
}

// ExtensionBase provides a base implementation for extensions
type ExtensionBase struct {
	Definition ExtensionDefinition
	Config     map[string]interface{}
	Deps       *Dependencies
}

func (b *ExtensionBase) GetDefinition() ExtensionDefinition {
	return b.Definition
}

// ExtensionRegistrar is the interface that extension packages must implement
type ExtensionRegistrar interface {
	// Define returns the extension definition
	Define() ExtensionDefinition
}
