package extension_manager

import (
	"database/sql"

	"go.codycody31.dev/squad-aegis/internal/connector_manager"
	"go.codycody31.dev/squad-aegis/internal/models"
	"go.codycody31.dev/squad-aegis/internal/rcon_manager"
)

// DependencyType represents a type of dependency that an extension can request
type DependencyType string

const (
	// Core dependencies
	DependencyDatabase    DependencyType = "database"
	DependencyServer      DependencyType = "server"
	DependencyRconManager DependencyType = "rcon_manager"

	// Optional dependencies
	DependencyConnectors DependencyType = "connectors"
)

// Dependencies holds all possible dependencies that can be injected into an extension
type Dependencies struct {
	Database    *sql.DB
	Server      *models.Server
	RconManager *rcon_manager.RconManager
	Connectors  map[string]connector_manager.Connector
}

// DependencyProvider provides dependencies to extensions
type DependencyProvider interface {
	// GetDependency returns a specific dependency
	GetDependency(depType DependencyType) (interface{}, error)
}

// ExtensionDependencies represents the dependencies required by an extension
type ExtensionDependencies struct {
	// Required dependencies that must be provided
	Required []DependencyType

	// Optional dependencies that will be used if available
	Optional []DependencyType
}
