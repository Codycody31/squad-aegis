package plugin_manager

import (
	"context"
	"database/sql"
	"time"

	"github.com/google/uuid"
	"go.codycody31.dev/squad-aegis/internal/event_manager"
	"go.codycody31.dev/squad-aegis/internal/shared/plug_config_schema"
)

// Plugin represents a server-specific plugin instance
type Plugin interface {
	// GetDefinition returns the plugin definition
	GetDefinition() PluginDefinition

	// Initialize sets up the plugin with configuration and dependencies
	Initialize(config map[string]interface{}, apis *PluginAPIs) error

	// Start begins plugin execution (for long-running plugins)
	Start(ctx context.Context) error

	// Stop gracefully stops the plugin
	Stop() error

	// HandleEvent processes an event if the plugin is subscribed to it
	HandleEvent(event *PluginEvent) error

	// GetStatus returns the current plugin status
	GetStatus() PluginStatus

	// GetConfig returns the current plugin configuration
	GetConfig() map[string]interface{}

	// UpdateConfig updates the plugin configuration
	UpdateConfig(config map[string]interface{}) error

	// GetCommands returns the list of commands exposed by this plugin
	GetCommands() []PluginCommand

	// ExecuteCommand executes a command with the given parameters
	// For sync commands: returns result immediately
	// For async commands: returns execution ID and starts background execution
	ExecuteCommand(commandID string, params map[string]interface{}) (*CommandResult, error)

	// GetCommandExecutionStatus returns the status of an async command execution
	GetCommandExecutionStatus(executionID string) (*CommandExecutionStatus, error)
}

// PluginDefinition defines the metadata and capabilities of a plugin
type PluginDefinition struct {
	ID                     string                          `json:"id"`
	Name                   string                          `json:"name"`
	Description            string                          `json:"description"`
	Version                string                          `json:"version"`
	Author                 string                          `json:"author"`
	AllowMultipleInstances bool                            `json:"allow_multiple_instances"`
	RequiredConnectors     []string                        `json:"required_connectors"`
	ConfigSchema           plug_config_schema.ConfigSchema `json:"config_schema"`
	Events                 []event_manager.EventType       `json:"event_handlers"`
	LongRunning            bool                            `json:"long_running"`
	CreateInstance         func() Plugin                   `json:"-"`
}

// EventHandler defines an event handler for a plugin
type EventHandler struct {
	Source      EventSource `json:"source"`
	EventType   string      `json:"event_type"`
	Description string      `json:"description"`
}

// EventSource represents the source of an event
type EventSource string

const (
	EventSourceRCON      EventSource = "rcon"
	EventSourceLog       EventSource = "log"
	EventSourceSystem    EventSource = "system"
	EventSourceConnector EventSource = "connector"
	EventSourcePlugin    EventSource = "plugin"
)

// PluginEvent represents an event passed to plugins
type PluginEvent struct {
	ID        uuid.UUID   `json:"id"`
	ServerID  uuid.UUID   `json:"server_id"`
	Source    EventSource `json:"source"`
	Type      string      `json:"type"`
	Data      interface{} `json:"data"`
	Raw       string      `json:"raw,omitempty"`
	Timestamp time.Time   `json:"timestamp"`
}

// PluginStatus represents the current status of a plugin
type PluginStatus string

const (
	PluginStatusStopped  PluginStatus = "stopped"
	PluginStatusStarting PluginStatus = "starting"
	PluginStatusRunning  PluginStatus = "running"
	PluginStatusStopping PluginStatus = "stopping"
	PluginStatusError    PluginStatus = "error"
	PluginStatusDisabled PluginStatus = "disabled"
)

// PluginCommand defines a user-executable command exposed by a plugin
type PluginCommand struct {
	ID                  string                          `json:"id"`
	Name                string                          `json:"name"`
	Description         string                          `json:"description"`
	Category            string                          `json:"category,omitempty"`
	Parameters          plug_config_schema.ConfigSchema `json:"parameters,omitempty"`
	ExecutionType       CommandExecutionType            `json:"execution_type"`
	RequiredPermissions []string                        `json:"required_permissions,omitempty"`
	ConfirmMessage      string                          `json:"confirm_message,omitempty"`
}

// CommandExecutionType defines how a command executes
type CommandExecutionType string

const (
	CommandExecutionSync  CommandExecutionType = "sync"
	CommandExecutionAsync CommandExecutionType = "async"
)

// CommandResult represents the result of a command execution
type CommandResult struct {
	Success     bool                   `json:"success"`
	Message     string                 `json:"message,omitempty"`
	Data        map[string]interface{} `json:"data,omitempty"`
	ExecutionID string                 `json:"execution_id,omitempty"` // For async commands
	Error       string                 `json:"error,omitempty"`
}

// CommandExecutionStatus represents async command execution status
type CommandExecutionStatus struct {
	ExecutionID string         `json:"execution_id"`
	CommandID   string         `json:"command_id"`
	Status      string         `json:"status"`             // "running", "completed", "failed"
	Progress    int            `json:"progress,omitempty"` // 0-100
	Message     string         `json:"message,omitempty"`
	Result      *CommandResult `json:"result,omitempty"`
	StartedAt   time.Time      `json:"started_at"`
	CompletedAt *time.Time     `json:"completed_at,omitempty"`
}

// PluginInstance represents an active plugin instance
type PluginInstance struct {
	ID         uuid.UUID              `json:"id"`
	ServerID   uuid.UUID              `json:"server_id"`
	PluginID   string                 `json:"plugin_id"`
	PluginName string                 `json:"plugin_name"`
	Notes      string                 `json:"notes"`
	Config     map[string]interface{} `json:"config"`
	Status     PluginStatus           `json:"status"`
	Enabled    bool                   `json:"enabled"`
	LogLevel   string                 `json:"log_level"` // debug, info, warn, error
	Plugin     Plugin                 `json:"-"`
	Context    context.Context        `json:"-"`
	Cancel     context.CancelFunc     `json:"-"`
	LastError  string                 `json:"last_error,omitempty"`
	CreatedAt  time.Time              `json:"created_at"`
	UpdatedAt  time.Time              `json:"updated_at"`
}

// Connector represents a global service connector (Discord, Slack, etc.)
type Connector interface {
	// GetDefinition returns the connector definition
	GetDefinition() ConnectorDefinition

	// Initialize sets up the connector with configuration
	Initialize(config map[string]interface{}) error

	// Start begins connector execution
	Start(ctx context.Context) error

	// Stop gracefully stops the connector
	Stop() error

	// GetStatus returns the current connector status
	GetStatus() ConnectorStatus

	// GetConfig returns the current connector configuration
	GetConfig() map[string]interface{}

	// UpdateConfig updates the connector configuration
	UpdateConfig(config map[string]interface{}) error

	// GetAPI returns the connector's API interface for plugins
	GetAPI() interface{}
}

// ConnectorDefinition defines the metadata and capabilities of a connector
type ConnectorDefinition struct {
	ID             string                          `json:"id"`
	Name           string                          `json:"name"`
	Description    string                          `json:"description"`
	Version        string                          `json:"version"`
	Author         string                          `json:"author"`
	ConfigSchema   plug_config_schema.ConfigSchema `json:"config_schema"`
	APIInterface   interface{}                     `json:"-"`
	CreateInstance func() Connector                `json:"-"`
}

// ConnectorStatus represents the current status of a connector
type ConnectorStatus string

const (
	ConnectorStatusStopped  ConnectorStatus = "stopped"
	ConnectorStatusStarting ConnectorStatus = "starting"
	ConnectorStatusRunning  ConnectorStatus = "running"
	ConnectorStatusStopping ConnectorStatus = "stopping"
	ConnectorStatusError    ConnectorStatus = "error"
	ConnectorStatusDisabled ConnectorStatus = "disabled"
)

// ConnectorInstance represents an active connector instance
type ConnectorInstance struct {
	ID        string                 `json:"id"`
	Config    map[string]interface{} `json:"config"`
	Status    ConnectorStatus        `json:"status"`
	Enabled   bool                   `json:"enabled"`
	Connector Connector              `json:"-"`
	Context   context.Context        `json:"-"`
	Cancel    context.CancelFunc     `json:"-"`
	LastError string                 `json:"last_error,omitempty"`
	CreatedAt time.Time              `json:"created_at"`
	UpdatedAt time.Time              `json:"updated_at"`
}

// PluginAPIs provides secure access to server functionality for plugins
type PluginAPIs struct {
	// Server information
	ServerAPI ServerAPI

	// Database access (limited)
	DatabaseAPI DatabaseAPI

	// RCON access (limited)
	RconAPI RconAPI

	// Admin management
	AdminAPI AdminAPI

	// Event system access
	EventAPI EventAPI

	// Connector access
	ConnectorAPI ConnectorAPI

	// Logging
	LogAPI LogAPI
}

// ServerAPI provides server-related functionality to plugins
type ServerAPI interface {
	// GetServerID returns the current server ID
	GetServerID() uuid.UUID

	// GetServerInfo returns basic server information
	GetServerInfo() (*ServerInfo, error)

	// GetPlayers returns current player list
	GetPlayers() ([]*PlayerInfo, error)

	// GetAdmins returns current admin list
	GetAdmins() ([]*AdminInfo, error)

	// GetSquads returns current squad list with enriched player data
	GetSquads() ([]*SquadInfo, error)
}

// DatabaseAPI provides limited database access to plugins
type DatabaseAPI interface {
	// ExecuteQuery executes a read-only query (SELECT only)
	ExecuteQuery(query string, args ...interface{}) (*sql.Rows, error)

	// GetPluginData retrieves plugin-specific data
	GetPluginData(key string) (string, error)

	// SetPluginData stores plugin-specific data
	SetPluginData(key string, value string) error

	// DeletePluginData removes plugin-specific data
	DeletePluginData(key string) error
}

// RconAPI provides limited RCON access to plugins
type RconAPI interface {
	// SendCommand sends an RCON command (restricted list)
	SendCommand(command string) (string, error)

	// Broadcast sends a message to all players
	Broadcast(message string) error

	// SendWarningToPlayer sends a warning message to a specific player
	SendWarningToPlayer(playerID string, message string) error

	// KickPlayer kicks a player (admin only)
	KickPlayer(playerID string, reason string) error

	// BanPlayer bans a player (admin only)
	BanPlayer(playerID string, reason string, duration time.Duration) error

	// RemovePlayerFromSquad removes a player from their squad without kicking them
	RemovePlayerFromSquad(playerID string) error

	// RemovePlayerFromSquadById removes a player from their squad by player ID without kicking them
	RemovePlayerFromSquadById(playerID string) error
}

// AdminAPI provides admin management functionality to plugins
type AdminAPI interface {
	// AddTemporaryAdmin adds a player as a temporary admin with specified role and notes
	AddTemporaryAdmin(steamID string, roleName string, notes string, expiresAt *time.Time) error

	// RemoveTemporaryAdmin removes a player's temporary admin status
	RemoveTemporaryAdmin(steamID string, notes string) error

	// GetPlayerAdminStatus checks if a player has admin status and returns their roles
	GetPlayerAdminStatus(steamID string) (*PlayerAdminStatus, error)

	// ListTemporaryAdmins lists all temporary admins managed by plugins
	ListTemporaryAdmins() ([]*TemporaryAdminInfo, error)
}

// PlayerAdminStatus contains admin status information for a player
type PlayerAdminStatus struct {
	SteamID     string             `json:"steam_id"`
	IsAdmin     bool               `json:"is_admin"`
	Roles       []*PlayerAdminRole `json:"roles"`
	HasExpiring bool               `json:"has_expiring"`
}

// PlayerAdminRole contains role information for a player's admin status
type PlayerAdminRole struct {
	ID        string     `json:"id"`
	RoleName  string     `json:"role_name"`
	Notes     string     `json:"notes"`
	ExpiresAt *time.Time `json:"expires_at"`
	IsExpired bool       `json:"is_expired"`
}

// TemporaryAdminInfo contains information about temporary admins
type TemporaryAdminInfo struct {
	ID        string     `json:"id"`
	SteamID   string     `json:"steam_id"`
	RoleName  string     `json:"role_name"`
	Notes     string     `json:"notes"`
	ExpiresAt *time.Time `json:"expires_at"`
	IsExpired bool       `json:"is_expired"`
	CreatedAt time.Time  `json:"created_at"`
}

// EventAPI provides event system access to plugins
type EventAPI interface {
	// PublishEvent publishes an event to the system
	PublishEvent(eventType string, data map[string]interface{}, raw string) error

	// SubscribeToEvents subscribes to specific event types
	SubscribeToEvents(eventTypes []string, handler func(*PluginEvent)) error
}

// ConnectorAPI provides access to global connectors
type ConnectorAPI interface {
	// GetConnector returns a connector API by ID
	GetConnector(connectorID string) (interface{}, error)

	// ListConnectors returns available connector IDs
	ListConnectors() []string
}

// LogAPI provides logging functionality to plugins
type LogAPI interface {
	// Info logs an info message
	Info(message string, fields map[string]interface{})

	// Warn logs a warning message
	Warn(message string, fields map[string]interface{})

	// Error logs an error message
	Error(message string, err error, fields map[string]interface{})

	// Debug logs a debug message
	Debug(message string, fields map[string]interface{})
}

// Data structures for API responses

// ServerInfo contains basic server information
type ServerInfo struct {
	ID          uuid.UUID `json:"id"`
	Name        string    `json:"name"`
	Host        string    `json:"host"`
	Port        int       `json:"port"`
	MaxPlayers  int       `json:"max_players"`
	CurrentMap  string    `json:"current_map"`
	GameMode    string    `json:"game_mode"`
	PlayerCount int       `json:"player_count"`
	Status      string    `json:"status"`
}

// PlayerInfo contains player information
type PlayerInfo struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	SteamID       string `json:"steam_id"`
	EOSID         string `json:"eos_id"`
	TeamID        int    `json:"team_id"`
	SquadID       int    `json:"squad_id"`
	Role          string `json:"role"`
	IsSquadLeader bool   `json:"is_squad_leader"`
	IsAdmin       bool   `json:"is_admin"`
	IsOnline      bool   `json:"is_online"`
}

// AdminInfo contains admin information
type AdminInfo struct {
	ID       string             `json:"id"`
	Name     string             `json:"name"`
	SteamID  string             `json:"steam_id"`
	IsOnline bool               `json:"is_online"`
	Roles    []*PlayerAdminRole `json:"roles"`
}

// SquadInfo contains squad information with enriched player data
type SquadInfo struct {
	ID      int           `json:"id"`
	TeamID  int           `json:"team_id"`
	Name    string        `json:"name"`
	Size    int           `json:"size"`
	Locked  bool          `json:"locked"`
	Leader  *PlayerInfo   `json:"leader"`
	Players []*PlayerInfo `json:"players"`
}

// PluginRegistry manages available plugin definitions
type PluginRegistry interface {
	// RegisterPlugin registers a new plugin definition
	RegisterPlugin(definition PluginDefinition) error

	// GetPlugin returns a plugin definition by ID
	GetPlugin(pluginID string) (*PluginDefinition, error)

	// ListPlugins returns all available plugin definitions
	ListPlugins() []PluginDefinition

	// CreatePluginInstance creates a new plugin instance
	CreatePluginInstance(pluginID string) (Plugin, error)
}

// ConnectorRegistry manages available connector definitions
type ConnectorRegistry interface {
	// RegisterConnector registers a new connector definition
	RegisterConnector(definition ConnectorDefinition) error

	// GetConnector returns a connector definition by ID
	GetConnector(connectorID string) (*ConnectorDefinition, error)

	// ListConnectors returns all available connector definitions
	ListConnectors() []ConnectorDefinition

	// CreateConnectorInstance creates a new connector instance
	CreateConnectorInstance(connectorID string) (Connector, error)
}

/*
Example Plugin Command Implementation:

	func (p *MyPlugin) GetCommands() []PluginCommand {
		return []PluginCommand{
			{
				ID:            "balance_teams",
				Name:          "Balance Teams",
				Description:   "Automatically balance teams based on player skill",
				Category:      "Team Management",
				ExecutionType: CommandExecutionSync,
				RequiredPermissions: []string{"manageserver"},
				ConfirmMessage: "This will move players between teams. Continue?",
				Parameters: plug_config_schema.ConfigSchema{
					Fields: []plug_config_schema.ConfigField{
						{
							Name:        "method",
							Type:        "string",
							Description: "Balancing method",
							Required:    true,
							Options:     []string{"kd_ratio", "playtime", "random"},
							Default:     "kd_ratio",
						},
						{
							Name:        "preserve_squads",
							Type:        "bool",
							Description: "Keep squad members together",
							Default:     true,
						},
					},
				},
			},
		}
	}

	func (p *MyPlugin) ExecuteCommand(commandID string, params map[string]interface{}) (*CommandResult, error) {
		switch commandID {
		case "balance_teams":
			method := params["method"].(string)
			preserveSquads := params["preserve_squads"].(bool)

			// Execute balancing logic...
			movedPlayers := p.balanceTeams(method, preserveSquads)

			return &CommandResult{
				Success: true,
				Message: fmt.Sprintf("Teams balanced! Moved %d players", movedPlayers),
				Data: map[string]interface{}{
					"players_moved": movedPlayers,
					"method_used":   method,
				},
			}, nil

		default:
			return nil, fmt.Errorf("unknown command: %s", commandID)
		}
	}

	// For async commands, return an execution ID and track status
	func (p *MyPlugin) ExecuteCommand(commandID string, params map[string]interface{}) (*CommandResult, error) {
		switch commandID {
		case "long_running_task":
			executionID := uuid.New().String()

			// Start task in background
			go p.runLongTask(executionID, params)

			return &CommandResult{
				Success:     true,
				ExecutionID: executionID,
				Message:     "Task started",
			}, nil

		default:
			return nil, fmt.Errorf("unknown command: %s", commandID)
		}
	}

	func (p *MyPlugin) GetCommandExecutionStatus(executionID string) (*CommandExecutionStatus, error) {
		// Look up execution status from plugin's internal tracking
		status, exists := p.executions[executionID]
		if !exists {
			return nil, fmt.Errorf("execution not found")
		}
		return status, nil
	}
*/
