package plugin_manager

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.codycody31.dev/squad-aegis/internal/event_manager"
	"go.codycody31.dev/squad-aegis/internal/shared/plug_config_schema"
	"go.codycody31.dev/squad-aegis/internal/shared/utils"
)

type PluginSource string

const (
	PluginSourceBundled PluginSource = "bundled"
	PluginSourceNative  PluginSource = "native"
)

type PluginDistribution string

const (
	PluginDistributionBundled  PluginDistribution = "bundled"
	PluginDistributionSideload PluginDistribution = "sideload"
)

type PluginInstallState string

const (
	PluginInstallStateReady          PluginInstallState = "ready"
	PluginInstallStateNotInstalled   PluginInstallState = "not_installed"
	PluginInstallStatePendingRestart PluginInstallState = "pending_restart"
	PluginInstallStateError          PluginInstallState = "error"
)

const NativePluginHostAPIVersion = 1

// NativeConnectorHostAPIVersion is the wire/API version for native connector packages (manifest min_host_api_version).
const NativeConnectorHostAPIVersion = 1

// ConnectorWireProtocolV1 is the JSON envelope version for ConnectorAPI.Invoke (field "v").
const ConnectorWireProtocolV1 = "1"

const (
	NativePluginCapabilityEntrypointGetAegisPlugin = "entrypoint.get_aegis_plugin"
	NativePluginCapabilityAPIRCON                  = "api.rcon"
	NativePluginCapabilityAPIServer                = "api.server"
	NativePluginCapabilityAPIDatabase              = "api.database"
	NativePluginCapabilityAPIRule                  = "api.rule"
	NativePluginCapabilityAPIAdmin                 = "api.admin"
	NativePluginCapabilityAPIDiscord               = "api.discord"
	NativePluginCapabilityAPIConnector             = "api.connector"
	NativePluginCapabilityAPIEvent                 = "api.event"
	NativePluginCapabilityAPILog                   = "api.log"
	NativePluginCapabilityEventsRCON               = "events.rcon"
	NativePluginCapabilityEventsLog                = "events.log"
	NativePluginCapabilityEventsSystem             = "events.system"
	NativePluginCapabilityEventsConnector          = "events.connector"
	NativePluginCapabilityEventsPlugin             = "events.plugin"
)

var nativePluginHostCapabilities = []string{
	NativePluginCapabilityEntrypointGetAegisPlugin,
	NativePluginCapabilityAPIRCON,
	NativePluginCapabilityAPIServer,
	NativePluginCapabilityAPIDatabase,
	NativePluginCapabilityAPIRule,
	NativePluginCapabilityAPIAdmin,
	NativePluginCapabilityAPIDiscord,
	NativePluginCapabilityAPIConnector,
	NativePluginCapabilityAPIEvent,
	NativePluginCapabilityAPILog,
	NativePluginCapabilityEventsRCON,
	NativePluginCapabilityEventsLog,
	NativePluginCapabilityEventsSystem,
	NativePluginCapabilityEventsConnector,
	NativePluginCapabilityEventsPlugin,
}

func NativePluginHostCapabilities() []string {
	capabilities := make([]string, len(nativePluginHostCapabilities))
	copy(capabilities, nativePluginHostCapabilities)
	return capabilities
}

type PluginPackageTarget struct {
	MinHostAPIVersion    int      `json:"min_host_api_version"`
	RequiredCapabilities []string `json:"required_capabilities,omitempty"`
	TargetOS             string   `json:"target_os"`
	TargetArch           string   `json:"target_arch"`
	SHA256               string   `json:"sha256,omitempty"`
	LibraryPath          string   `json:"library_path"`
}

// PluginPackageManifest is the signed manifest.json shipped with every
// native plugin bundle. It carries ONLY the identity and distribution
// metadata operators need to evaluate a package at upload time. Runtime
// behavior (config schema, event subscriptions, long-running flag,
// required connectors, multi-instance support) lives in the plugin binary
// and is fetched over RPC at load time via pluginrpc.PluginDefinition.
type PluginPackageManifest struct {
	PluginID    string                `json:"plugin_id"`
	Name        string                `json:"name"`
	Description string                `json:"description,omitempty"`
	Version     string                `json:"version"`
	Author      string                `json:"author,omitempty"`
	License     string                `json:"license,omitempty"`
	Official    bool                  `json:"official,omitempty"`
	Targets     []PluginPackageTarget `json:"targets"`
}

type InstalledPluginPackage struct {
	PluginID             string                `json:"plugin_id"`
	Name                 string                `json:"name"`
	Description          string                `json:"description"`
	Version              string                `json:"version"`
	Source               PluginSource          `json:"source"`
	Distribution         PluginDistribution    `json:"distribution"`
	Official             bool                  `json:"official"`
	InstallState         PluginInstallState    `json:"install_state"`
	RuntimePath          string                `json:"runtime_path,omitempty"`
	Manifest             PluginPackageManifest `json:"manifest"`
	ManifestJSON         json.RawMessage       `json:"-"`
	ManifestSignature    []byte                `json:"-"`
	ManifestPublicKey    []byte                `json:"-"`
	SignatureVerified    bool                  `json:"signature_verified"`
	Unsafe               bool                  `json:"unsafe"`
	Checksum             string                `json:"checksum"`
	MinHostAPIVersion    int                   `json:"min_host_api_version"`
	RequiredCapabilities []string              `json:"required_capabilities,omitempty"`
	TargetOS             string                `json:"target_os"`
	TargetArch           string                `json:"target_arch"`
	LastError            string                `json:"last_error,omitempty"`
	CreatedAt            time.Time             `json:"created_at"`
	UpdatedAt            time.Time             `json:"updated_at"`
}

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
	Source                 PluginSource                    `json:"source"`
	Official               bool                            `json:"official"`
	InstallState           PluginInstallState              `json:"install_state"`
	Distribution           PluginDistribution              `json:"distribution"`
	MinHostAPIVersion      int                             `json:"min_host_api_version,omitempty"`
	RequiredCapabilities   []string                        `json:"required_capabilities,omitempty"`
	TargetOS               string                          `json:"target_os,omitempty"`
	TargetArch             string                          `json:"target_arch,omitempty"`
	RuntimePath            string                          `json:"runtime_path,omitempty"`
	SignatureVerified      bool                            `json:"signature_verified"`
	Unsafe                 bool                            `json:"unsafe"`
	AllowMultipleInstances bool                            `json:"allow_multiple_instances"`
	RequiredConnectors     []string                        `json:"required_connectors"`
	OptionalConnectors     []string                        `json:"optional_connectors,omitempty"`
	ConfigSchema           plug_config_schema.ConfigSchema `json:"config_schema"`
	Events                 []event_manager.EventType       `json:"event_handlers"`
	LongRunning            bool                            `json:"long_running"`
	CreateInstance         func() Plugin                   `json:"-"`
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
	ID                uuid.UUID              `json:"id"`
	ServerID          uuid.UUID              `json:"server_id"`
	PluginID          string                 `json:"plugin_id"`
	PluginName        string                 `json:"plugin_name"`
	Source            PluginSource           `json:"source,omitempty"`
	Official          bool                   `json:"official,omitempty"`
	Distribution      PluginDistribution     `json:"distribution,omitempty"`
	InstallState      PluginInstallState     `json:"install_state,omitempty"`
	MinHostAPIVersion int                    `json:"min_host_api_version,omitempty"`
	Notes             string                 `json:"notes"`
	Config            map[string]interface{} `json:"config"`
	Status            PluginStatus           `json:"status"`
	Enabled           bool                   `json:"enabled"`
	LogLevel          string                 `json:"log_level"` // debug, info, warn, error
	Plugin            Plugin                 `json:"-"`
	Context           context.Context        `json:"-"`
	Cancel            context.CancelFunc     `json:"-"`
	LastError         string                 `json:"last_error,omitempty"`
	CreatedAt         time.Time              `json:"created_at"`
	UpdatedAt         time.Time              `json:"updated_at"`

	// mu protects mutable state (Status, LastError) that may be written
	// from concurrent event-handler goroutines.
	mu sync.Mutex `json:"-"`
}

// setStatus safely updates the instance status.
func (pi *PluginInstance) setStatus(s PluginStatus) {
	pi.mu.Lock()
	pi.Status = s
	pi.mu.Unlock()
}

// setError safely updates the instance status and last error.
func (pi *PluginInstance) setError(s PluginStatus, msg string) {
	pi.mu.Lock()
	pi.Status = s
	pi.LastError = msg
	pi.mu.Unlock()
}

// clearError safely updates status and clears the last error.
func (pi *PluginInstance) clearError(s PluginStatus) {
	pi.mu.Lock()
	pi.Status = s
	pi.LastError = ""
	pi.mu.Unlock()
}

// ConnectorInvokeRequest is the versioned JSON envelope plugins send to connectors via ConnectorAPI.
type ConnectorInvokeRequest struct {
	V    string                 `json:"v"`
	Data map[string]interface{} `json:"data"`
}

// ConnectorInvokeResponse is returned from ConnectorAPI.Call and InvokableConnector.Invoke.
type ConnectorInvokeResponse struct {
	V     string                 `json:"v"`
	OK    bool                   `json:"ok"`
	Data  map[string]interface{} `json:"data,omitempty"`
	Error string                 `json:"error,omitempty"`
}

// InvokableConnector is implemented by connectors that support JSON invoke (ConnectorAPI).
type InvokableConnector interface {
	Invoke(ctx context.Context, req *ConnectorInvokeRequest) (*ConnectorInvokeResponse, error)
}

// ConnectorAPI lets plugins call connectors by ID with a versioned JSON envelope.
type ConnectorAPI interface {
	Call(ctx context.Context, connectorID string, req *ConnectorInvokeRequest) (*ConnectorInvokeResponse, error)
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
	ID           string                          `json:"id"`
	LegacyIDs    []string                        `json:"legacy_ids,omitempty"`
	InstanceKey  string                          `json:"instance_key,omitempty"`
	Source       PluginSource                    `json:"source"`
	Name         string                          `json:"name"`
	Description  string                          `json:"description"`
	Version      string                          `json:"version"`
	Author       string                          `json:"author"`
	ConfigSchema plug_config_schema.ConfigSchema `json:"config_schema"`
	APIInterface interface{}                     `json:"-"`
	// MinHostAPIVersion is required for native connector packages (see NativeConnectorHostAPIVersion).
	MinHostAPIVersion    int                `json:"min_host_api_version,omitempty"`
	RequiredCapabilities []string           `json:"required_capabilities,omitempty"`
	TargetOS             string             `json:"target_os,omitempty"`
	TargetArch           string             `json:"target_arch,omitempty"`
	RuntimePath          string             `json:"runtime_path,omitempty"`
	SignatureVerified    bool               `json:"signature_verified,omitempty"`
	Unsafe               bool               `json:"unsafe,omitempty"`
	InstallState         PluginInstallState `json:"install_state,omitempty"`
	Distribution         PluginDistribution `json:"distribution,omitempty"`
	Official             bool               `json:"official,omitempty"`
	CreateInstance       func() Connector   `json:"-"`
}

// ConnectorInstanceStorageKey returns the primary key used in the connectors table and pm.connectors.
func (d ConnectorDefinition) ConnectorInstanceStorageKey() string {
	if strings.TrimSpace(d.InstanceKey) != "" {
		return strings.TrimSpace(d.InstanceKey)
	}
	if len(d.LegacyIDs) > 0 && strings.TrimSpace(d.LegacyIDs[0]) != "" {
		return strings.TrimSpace(d.LegacyIDs[0])
	}
	return d.ID
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

	// Plugin-scoped key/value storage
	DatabaseAPI DatabaseAPI

	// Server rule access
	RuleAPI RuleAPI

	// RCON access (limited)
	RconAPI RconAPI

	// Admin management
	AdminAPI AdminAPI

	// Event system access
	EventAPI EventAPI

	// Discord messaging access when the Discord connector is available
	DiscordAPI DiscordAPI

	// ConnectorAPI provides JSON invoke into connectors when exposed for this plugin instance.
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

// DatabaseAPI provides plugin-scoped key/value storage
type DatabaseAPI interface {
	// GetPluginData retrieves plugin-specific data
	GetPluginData(key string) (string, error)

	// SetPluginData stores plugin-specific data
	SetPluginData(key string, value string) error

	// DeletePluginData removes plugin-specific data
	DeletePluginData(key string) error
}

// RuleAPI provides read-only access to server rules and their configured actions.
type RuleAPI interface {
	// ListServerRules returns rules for the current server scoped to the provided parent rule.
	// Pass nil to fetch top-level rules.
	ListServerRules(parentRuleID *string) ([]*RuleInfo, error)

	// ListServerRuleActions returns escalation actions for a rule on the current server.
	ListServerRuleActions(ruleID string) ([]*RuleActionInfo, error)
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

	// BanWithEvidence bans a player and links evidence from an event (admin only)
	// eventID should be the UUID from Event.ID or ClickHouse message_id/event_id
	// eventType should match EventType constants (e.g., "RCON_CHAT_MESSAGE")
	// Returns the ban UUID and any error
	BanWithEvidence(playerID string, reason string, duration time.Duration, eventID string, eventType string) (string, error)

	// WarnPlayerWithRule sends a warning and logs the violation to player history if ruleID is provided
	WarnPlayerWithRule(playerID string, message string, ruleID *string) error

	// KickPlayerWithRule kicks a player and logs the violation to player history if ruleID is provided
	KickPlayerWithRule(playerID string, reason string, ruleID *string) error

	// BanPlayerWithRule bans a player and logs the violation to player history if ruleID is provided
	BanPlayerWithRule(playerID string, reason string, duration time.Duration, ruleID *string) error

	// BanWithEvidenceAndRule bans a player with evidence linking and logs the violation to player history if ruleID is provided
	// Returns the ban UUID and any error
	BanWithEvidenceAndRule(playerID string, reason string, duration time.Duration, eventID string, eventType string, ruleID *string) (string, error)

	// BanWithEvidenceAndRuleAndMetadata bans a player with evidence linking, logs the violation,
	// and persists additional evidence metadata when provided.
	BanWithEvidenceAndRuleAndMetadata(playerID string, reason string, duration time.Duration, eventID string, eventType string, ruleID *string, metadata map[string]interface{}) (string, error)

	// RemovePlayerFromSquad removes a player from their squad without kicking them
	RemovePlayerFromSquad(playerID string) error

	// RemovePlayerFromSquadById removes a player from their squad by player ID without kicking them
	RemovePlayerFromSquadById(playerID string) error
}

// AdminAPI provides admin management functionality to plugins
type AdminAPI interface {
	// AddTemporaryAdmin adds a player as a temporary admin with specified role and notes
	AddTemporaryAdmin(playerID string, roleName string, notes string, expiresAt *time.Time) error

	// RemoveTemporaryAdmin removes a player's temporary admin status
	RemoveTemporaryAdmin(playerID string, notes string) error

	// RemoveTemporaryAdminRole removes a player's temporary admin status for a specific role
	RemoveTemporaryAdminRole(playerID string, roleName string, notes string) error

	// GetPlayerAdminStatus checks if a player has admin status and returns their roles
	GetPlayerAdminStatus(playerID string) (*PlayerAdminStatus, error)

	// ListTemporaryAdmins lists all temporary admins managed by plugins
	ListTemporaryAdmins() ([]*TemporaryAdminInfo, error)
}

// PlayerAdminStatus contains admin status information for a player
type PlayerAdminStatus struct {
	SteamID     string             `json:"steam_id,omitempty"`
	EOSID       string             `json:"eos_id,omitempty"`
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
	SteamID   string     `json:"steam_id,omitempty"`
	EOSID     string     `json:"eos_id,omitempty"`
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

// DiscordAPI provides limited Discord messaging functionality to plugins.
type DiscordAPI interface {
	// SendMessage sends a plain text message to a Discord channel.
	SendMessage(channelID, content string) (string, error)

	// SendEmbed sends an embed message to a Discord channel.
	SendEmbed(channelID string, embed *DiscordEmbed) (string, error)
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

// RuleInfo contains the rule fields plugins are allowed to inspect.
type RuleInfo struct {
	ID           string `json:"id"`
	ParentID     string `json:"parent_id,omitempty"`
	DisplayOrder int    `json:"display_order"`
	Title        string `json:"title"`
	Description  string `json:"description"`
}

// RuleActionInfo contains server-configured escalation actions for a rule.
type RuleActionInfo struct {
	ID             string `json:"id"`
	RuleID         string `json:"rule_id"`
	ViolationCount int    `json:"violation_count"`
	ActionType     string `json:"action_type"`
	Duration       *int   `json:"duration,omitempty"`
	Message        string `json:"message,omitempty"`
}

// DiscordEmbed represents a Discord embed message.
type DiscordEmbed struct {
	Title       string                 `json:"title,omitempty"`
	Description string                 `json:"description,omitempty"`
	Color       int                    `json:"color,omitempty"`
	Fields      []*DiscordEmbedField   `json:"fields,omitempty"`
	Footer      *DiscordEmbedFooter    `json:"footer,omitempty"`
	Thumbnail   *DiscordEmbedThumbnail `json:"thumbnail,omitempty"`
	Image       *DiscordEmbedImage     `json:"image,omitempty"`
	Timestamp   *time.Time             `json:"timestamp,omitempty"`
}

// DiscordEmbedField represents an embed field.
type DiscordEmbedField struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Inline bool   `json:"inline,omitempty"`
}

// DiscordEmbedFooter represents an embed footer.
type DiscordEmbedFooter struct {
	Text    string `json:"text"`
	IconURL string `json:"icon_url,omitempty"`
}

// DiscordEmbedThumbnail represents an embed thumbnail.
type DiscordEmbedThumbnail struct {
	URL string `json:"url"`
}

// DiscordEmbedImage represents an embed image.
type DiscordEmbedImage struct {
	URL string `json:"url"`
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

// PreferredID returns the best available player identifier for RCON commands.
// Prefers Steam ID, falls back to EOS ID.
func (p *PlayerInfo) PreferredID() string {
	if p.SteamID != "" {
		return utils.NormalizePlayerID(p.SteamID)
	}
	return utils.NormalizePlayerID(p.EOSID)
}

// MatchesPlayerID returns true when the provided ID matches this player.
func (p *PlayerInfo) MatchesPlayerID(playerID string) bool {
	return utils.MatchPlayerID(playerID, p.SteamID, p.EOSID)
}

// AdminInfo contains admin information
type AdminInfo struct {
	ID       string             `json:"id"`
	Name     string             `json:"name"`
	SteamID  string             `json:"steam_id,omitempty"`
	EOSID    string             `json:"eos_id,omitempty"`
	IsOnline bool               `json:"is_online"`
	Roles    []*PlayerAdminRole `json:"roles"`
}

// PreferredID returns the best available admin identifier.
func (a *AdminInfo) PreferredID() string {
	if a.SteamID != "" {
		return utils.NormalizePlayerID(a.SteamID)
	}
	return utils.NormalizePlayerID(a.EOSID)
}

// MatchesPlayerID returns true when the provided ID matches this admin.
func (a *AdminInfo) MatchesPlayerID(playerID string) bool {
	return utils.MatchPlayerID(playerID, a.SteamID, a.EOSID)
}

// PreferredID returns the best available identifier for the admin status.
func (s *PlayerAdminStatus) PreferredID() string {
	if s.SteamID != "" {
		return utils.NormalizePlayerID(s.SteamID)
	}
	return utils.NormalizePlayerID(s.EOSID)
}

// PreferredID returns the best available identifier for the temporary admin.
func (t *TemporaryAdminInfo) PreferredID() string {
	if t.SteamID != "" {
		return utils.NormalizePlayerID(t.SteamID)
	}
	return utils.NormalizePlayerID(t.EOSID)
}

// MatchesPlayerID returns true when the provided ID matches this temporary admin.
func (t *TemporaryAdminInfo) MatchesPlayerID(playerID string) bool {
	return utils.MatchPlayerID(playerID, t.SteamID, t.EOSID)
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

	// UnregisterPlugin removes a plugin definition by ID
	UnregisterPlugin(pluginID string)

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

	// UnregisterConnector removes a connector definition by canonical ID and its aliases
	UnregisterConnector(canonicalID string)

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
