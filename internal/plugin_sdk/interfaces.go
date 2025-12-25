package plugin_sdk

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// APIVersion is the current version of the Plugin SDK API
const APIVersion = "v1.0.0"

// FeatureID represents a specific feature that plugins can require or provide
type FeatureID string

const (
	// Core features
	FeatureEventHandling   FeatureID = "event_handling"
	FeatureRCON            FeatureID = "rcon"
	FeatureDatabaseAccess  FeatureID = "database_access"
	FeatureCommands        FeatureID = "commands"
	FeatureConnectors      FeatureID = "connectors"
	FeatureAdminAPI        FeatureID = "admin_api"
	FeatureServerAPI       FeatureID = "server_api"
	FeatureLogging         FeatureID = "logging"
	
	// Future features (examples)
	FeatureMetrics         FeatureID = "metrics"
	FeatureWebhooks        FeatureID = "webhooks"
	FeatureScheduling      FeatureID = "scheduling"
)

// PermissionID represents a specific permission that plugins can request
type PermissionID string

const (
	// RCON Permissions
	PermissionRCONAccess       PermissionID = "rcon.access"
	PermissionRCONBroadcast    PermissionID = "rcon.broadcast"
	PermissionRCONKick         PermissionID = "rcon.kick"
	PermissionRCONBan          PermissionID = "rcon.ban"
	PermissionRCONAdmin        PermissionID = "rcon.admin"
	
	// Database Permissions
	PermissionDatabaseRead     PermissionID = "database.read"
	PermissionDatabaseWrite    PermissionID = "database.write"
	
	// Admin Permissions
	PermissionAdminManagement  PermissionID = "admin.management"
	PermissionPlayerManagement PermissionID = "player.management"
	
	// System Permissions
	PermissionEventPublish     PermissionID = "event.publish"
	PermissionConnectorAccess  PermissionID = "connector.access"
)

// PluginSDK is the minimum interface that all plugins must implement
type PluginSDK interface {
	// GetManifest returns the plugin's manifest with metadata and feature declarations
	GetManifest() PluginManifest
	
	// Initialize is called when the plugin is loaded, receives the base API
	Initialize(baseAPI BaseAPI) error
	
	// Shutdown is called when the plugin is being unloaded
	Shutdown() error
}

// PluginManifest contains metadata about a plugin
type PluginManifest struct {
	// Required fields
	ID          string `json:"id"`
	Name        string `json:"name"`
	Version     string `json:"version"`
	Author      string `json:"author"`
	SDKVersion  string `json:"sdk_version"`
	
	// Optional fields
	Description string `json:"description,omitempty"`
	Website     string `json:"website,omitempty"`
	
	// Feature declarations
	RequiredFeatures []FeatureID `json:"required_features"`
	ProvidedFeatures []FeatureID `json:"provided_features"`
	
	// Permission declarations
	RequiredPermissions []PermissionID `json:"required_permissions"`
	
	// Configuration
	AllowMultipleInstances bool   `json:"allow_multiple_instances"`
	LongRunning            bool   `json:"long_running"`
	
	// Security
	Signature []byte `json:"signature,omitempty"`
}

// BaseAPI provides core functionality that all plugins receive
type BaseAPI interface {
	// GetFeatureAPI returns an API for a specific feature if the plugin has access
	GetFeatureAPI(featureID FeatureID) (interface{}, error)
	
	// GetServerID returns the current server ID this plugin instance is running on
	GetServerID() uuid.UUID
	
	// GetPluginInstanceID returns this plugin instance's unique ID
	GetPluginInstanceID() uuid.UUID
	
	// SpawnGoroutine spawns a tracked goroutine (for sandbox monitoring)
	SpawnGoroutine(fn func())
	
	// GetContext returns the plugin's context (cancelled on shutdown)
	GetContext() context.Context
}

// EventHandlingAPI provides event handling capabilities
type EventHandlingAPI interface {
	// HandleEvent is called when an event this plugin subscribed to occurs
	HandleEvent(event *PluginEvent) error
	
	// PublishEvent publishes an event to the system
	PublishEvent(eventType string, data interface{}, raw string) error
}

// RconAPI provides RCON access
type RconAPI interface {
	// SendCommand sends an RCON command (restricted list)
	SendCommand(command string) (string, error)
	
	// Broadcast sends a message to all players
	Broadcast(message string) error
	
	// SendWarningToPlayer sends a warning message to a specific player
	SendWarningToPlayer(playerID string, message string) error
	
	// KickPlayer kicks a player (requires permission)
	KickPlayer(playerID string, reason string) error
	
	// BanPlayer bans a player (requires permission)
	BanPlayer(playerID string, reason string, duration time.Duration) error
	
	// RemovePlayerFromSquad removes a player from their squad
	RemovePlayerFromSquad(playerID string) error
}

// DatabaseAPI provides limited database access
type DatabaseAPI interface {
	// GetPluginData retrieves plugin-specific data
	GetPluginData(key string) (string, error)
	
	// SetPluginData stores plugin-specific data
	SetPluginData(key string, value string) error
	
	// DeletePluginData removes plugin-specific data
	DeletePluginData(key string) error
	
	// ExecuteQuery executes a read-only query (SELECT only)
	ExecuteQuery(query string, args ...interface{}) (interface{}, error)
}

// ServerAPI provides server information
type ServerAPI interface {
	// GetServerInfo returns basic server information
	GetServerInfo() (*ServerInfo, error)
	
	// GetPlayers returns current player list
	GetPlayers() ([]*PlayerInfo, error)
	
	// GetAdmins returns current admin list
	GetAdmins() ([]*AdminInfo, error)
	
	// GetSquads returns current squad list with enriched player data
	GetSquads() ([]*SquadInfo, error)
}

// AdminAPI provides admin management functionality
type AdminAPI interface {
	// AddTemporaryAdmin adds a player as a temporary admin
	AddTemporaryAdmin(steamID string, roleName string, notes string, expiresAt *time.Time) error
	
	// RemoveTemporaryAdmin removes a player's temporary admin status
	RemoveTemporaryAdmin(steamID string, notes string) error
	
	// GetPlayerAdminStatus checks if a player has admin status
	GetPlayerAdminStatus(steamID string) (*PlayerAdminStatus, error)
	
	// ListTemporaryAdmins lists all temporary admins
	ListTemporaryAdmins() ([]*TemporaryAdminInfo, error)
}

// ConnectorAPI provides access to global connectors
type ConnectorAPI interface {
	// GetConnector returns a connector API by ID
	GetConnector(connectorID string) (interface{}, error)
	
	// ListConnectors returns available connector IDs
	ListConnectors() []string
}

// LogAPI provides logging functionality
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

// CommandsAPI provides command execution capabilities
type CommandsAPI interface {
	// GetCommands returns the list of commands exposed by this plugin
	GetCommands() []PluginCommand
	
	// ExecuteCommand executes a command with the given parameters
	ExecuteCommand(commandID string, params map[string]interface{}) (*CommandResult, error)
	
	// GetCommandExecutionStatus returns the status of an async command execution
	GetCommandExecutionStatus(executionID string) (*CommandExecutionStatus, error)
}

// Data structures

// PluginEvent represents an event passed to plugins
type PluginEvent struct {
	ID        uuid.UUID   `json:"id"`
	ServerID  uuid.UUID   `json:"server_id"`
	Source    string      `json:"source"`
	Type      string      `json:"type"`
	Data      interface{} `json:"data"`
	Raw       string      `json:"raw,omitempty"`
	Timestamp time.Time   `json:"timestamp"`
}

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

// PluginCommand defines a user-executable command exposed by a plugin
type PluginCommand struct {
	ID                  string                 `json:"id"`
	Name                string                 `json:"name"`
	Description         string                 `json:"description"`
	Category            string                 `json:"category,omitempty"`
	Parameters          map[string]interface{} `json:"parameters,omitempty"`
	ExecutionType       string                 `json:"execution_type"` // "sync" or "async"
	RequiredPermissions []string               `json:"required_permissions,omitempty"`
	ConfirmMessage      string                 `json:"confirm_message,omitempty"`
}

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
	Status      string         `json:"status"` // "running", "completed", "failed"
	Progress    int            `json:"progress,omitempty"` // 0-100
	Message     string         `json:"message,omitempty"`
	Result      *CommandResult `json:"result,omitempty"`
	StartedAt   time.Time      `json:"started_at"`
	CompletedAt *time.Time     `json:"completed_at,omitempty"`
}

