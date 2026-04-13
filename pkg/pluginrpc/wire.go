// Package pluginrpc is the subprocess-isolated SDK for authoring Squad Aegis
// native plugins. Plugins built against this package run as standalone
// binaries; the host spawns them via hashicorp/go-plugin and communicates
// over net/rpc, so a crashing or malicious plugin cannot corrupt the host
// process's memory.
//
// This package is intentionally self-contained: it imports nothing from
// internal/ so that external plugin authors can vendor it independently.
// The wire types mirror the host's runtime types at the JSON level and are
// converted in both directions on the host side.
package pluginrpc

import (
	"encoding/json"
	"time"
)

// WireProtocolVersion bumps whenever the wire format changes in a way that
// is not backwards-compatible. Plugins advertise a minimum version and the
// host refuses to spawn a plugin whose protocol is too new or too old.
const WireProtocolVersion = 1

// PluginStatus mirrors plugin_manager.PluginStatus on the wire.
type PluginStatus string

const (
	PluginStatusStopped  PluginStatus = "stopped"
	PluginStatusStarting PluginStatus = "starting"
	PluginStatusRunning  PluginStatus = "running"
	PluginStatusStopping PluginStatus = "stopping"
	PluginStatusError    PluginStatus = "error"
	PluginStatusDisabled PluginStatus = "disabled"
)

// Valid returns true if s is one of the recognised plugin statuses.
func (s PluginStatus) Valid() bool {
	switch s {
	case PluginStatusStopped, PluginStatusStarting, PluginStatusRunning,
		PluginStatusStopping, PluginStatusError, PluginStatusDisabled:
		return true
	}
	return false
}

// EventSource mirrors plugin_manager.EventSource on the wire.
type EventSource string

const (
	EventSourceRCON      EventSource = "rcon"
	EventSourceLog       EventSource = "log"
	EventSourceSystem    EventSource = "system"
	EventSourceConnector EventSource = "connector"
	EventSourcePlugin    EventSource = "plugin"
)

// Valid returns true if s is one of the recognised event sources.
func (s EventSource) Valid() bool {
	switch s {
	case EventSourceRCON, EventSourceLog, EventSourceSystem,
		EventSourceConnector, EventSourcePlugin:
		return true
	}
	return false
}

// CommandExecutionType mirrors plugin_manager.CommandExecutionType.
type CommandExecutionType string

const (
	CommandExecutionSync  CommandExecutionType = "sync"
	CommandExecutionAsync CommandExecutionType = "async"
)

// Valid returns true if t is one of the recognised command execution types.
func (t CommandExecutionType) Valid() bool {
	switch t {
	case CommandExecutionSync, CommandExecutionAsync:
		return true
	}
	return false
}

// FieldType mirrors plug_config_schema.FieldType as raw strings so the wire
// format survives without importing the internal schema package.
type FieldType string

const (
	FieldTypeString      FieldType = "string"
	FieldTypeInt         FieldType = "int"
	FieldTypeBool        FieldType = "bool"
	FieldTypeObject      FieldType = "object"
	FieldTypeArray       FieldType = "array"
	FieldTypeArrayString FieldType = "arraystring"
	FieldTypeArrayInt    FieldType = "arrayint"
	FieldTypeArrayBool   FieldType = "arraybool"
	FieldTypeArrayObject FieldType = "arrayobject"
)

// Valid returns true if t is one of the recognised field types.
func (t FieldType) Valid() bool {
	switch t {
	case FieldTypeString, FieldTypeInt, FieldTypeBool, FieldTypeObject,
		FieldTypeArray, FieldTypeArrayString, FieldTypeArrayInt,
		FieldTypeArrayBool, FieldTypeArrayObject:
		return true
	}
	return false
}

// ConfigField is the wire representation of a single config field.
type ConfigField struct {
	Name        string        `json:"name"`
	Description string        `json:"description,omitempty"`
	Required    bool          `json:"required,omitempty"`
	Type        FieldType     `json:"type"`
	Default     interface{}   `json:"default,omitempty"`
	Sensitive   bool          `json:"sensitive,omitempty"`
	Enum        []interface{} `json:"enum,omitempty"`
	Nested      []ConfigField `json:"nested,omitempty"`
}

// ConfigSchema is the wire representation of a config schema. It is
// structurally identical to plug_config_schema.ConfigSchema so the host can
// re-marshal it via JSON.
type ConfigSchema struct {
	Fields []ConfigField `json:"fields"`
}

// PluginDefinition is what the subprocess returns from GetDefinition(). It
// covers ONLY the runtime/behavioral surface that the plugin uniquely knows
// about — identity (name, version, author, license, etc.) and compatibility
// (min host API version, required capabilities, target OS/arch) live in the
// signed manifest.json shipped alongside the plugin binary. The host
// cross-checks PluginID against manifest.plugin_id during load and then
// merges the two halves into its in-process PluginDefinition.
type PluginDefinition struct {
	// PluginID is echoed here so the host can verify the subprocess agrees
	// with the manifest it was loaded from. A mismatch aborts the load.
	PluginID string `json:"plugin_id"`

	// AllowMultipleInstances governs whether more than one instance of this
	// plugin can be enabled on a single server.
	AllowMultipleInstances bool `json:"allow_multiple_instances,omitempty"`

	// LongRunning is true for plugins that need a dedicated Start() call.
	// Event-driven plugins can leave it false.
	LongRunning bool `json:"long_running,omitempty"`

	// RequiredConnectors are connector IDs the plugin cannot run without.
	// OptionalConnectors are connectors the plugin can use if available.
	RequiredConnectors []string `json:"required_connectors,omitempty"`
	OptionalConnectors []string `json:"optional_connectors,omitempty"`

	// ConfigSchema describes the config fields the plugin accepts, used by
	// the host to validate operator-provided config before Initialize.
	ConfigSchema ConfigSchema `json:"config_schema"`

	// Events is the list of event types the plugin wants to receive via
	// HandleEvent RPC calls.
	Events []string `json:"events,omitempty"`
}

// PluginEvent is the wire shape of plugin_manager.PluginEvent. Data is a raw
// JSON blob so plugins can unmarshal it into concrete types.
type PluginEvent struct {
	ID        string          `json:"id"`
	ServerID  string          `json:"server_id"`
	Source    EventSource     `json:"source"`
	Type      string          `json:"type"`
	Data      json.RawMessage `json:"data,omitempty"`
	Raw       string          `json:"raw,omitempty"`
	Timestamp time.Time       `json:"timestamp"`
}

// PluginCommand mirrors plugin_manager.PluginCommand on the wire.
type PluginCommand struct {
	ID                  string               `json:"id"`
	Name                string               `json:"name"`
	Description         string               `json:"description,omitempty"`
	Category            string               `json:"category,omitempty"`
	Parameters          ConfigSchema         `json:"parameters,omitempty"`
	ExecutionType       CommandExecutionType `json:"execution_type,omitempty"`
	RequiredPermissions []string             `json:"required_permissions,omitempty"`
	ConfirmMessage      string               `json:"confirm_message,omitempty"`
}

// CommandResult mirrors plugin_manager.CommandResult on the wire.
type CommandResult struct {
	Success     bool                   `json:"success"`
	Message     string                 `json:"message,omitempty"`
	Data        map[string]interface{} `json:"data,omitempty"`
	ExecutionID string                 `json:"execution_id,omitempty"`
	Error       string                 `json:"error,omitempty"`
}

// CommandExecutionStatus mirrors plugin_manager.CommandExecutionStatus.
type CommandExecutionStatus struct {
	ExecutionID string         `json:"execution_id"`
	CommandID   string         `json:"command_id"`
	Status      string         `json:"status"`
	Progress    int            `json:"progress,omitempty"`
	Message     string         `json:"message,omitempty"`
	Result      *CommandResult `json:"result,omitempty"`
	StartedAt   time.Time      `json:"started_at"`
	CompletedAt *time.Time     `json:"completed_at,omitempty"`
}

// InitializeArgs is the RPC payload passed from host → plugin at Initialize
// time. It carries the plugin config plus the broker ID the plugin should
// dial back into for HostAPI calls, plus an instance identifier for logging.
type InitializeArgs struct {
	Config          map[string]interface{} `json:"config"`
	HostAPIBrokerID uint32                 `json:"host_api_broker_id"`
	InstanceID      string                 `json:"instance_id,omitempty"`
	ServerID        string                 `json:"server_id,omitempty"`
	LogLevel        string                 `json:"log_level,omitempty"`
}

// HostAPIRequest is the generic envelope the plugin sends back to the host
// when invoking a host API method. Target is a dotted "api.Method" string
// (for example "log.Info" or "rcon.SendCommand"), Payload is the JSON-encoded
// arguments specific to that method.
type HostAPIRequest struct {
	Target  string          `json:"target"`
	Payload json.RawMessage `json:"payload"`
}

// HostAPIResponse is the reply envelope. Payload is the JSON-encoded result;
// Error is the stringified error if the call failed.
type HostAPIResponse struct {
	Payload json.RawMessage `json:"payload,omitempty"`
	Error   string          `json:"error,omitempty"`
}

// ExecuteCommandArgs carries the command invocation payload.
type ExecuteCommandArgs struct {
	CommandID string                 `json:"command_id"`
	Params    map[string]interface{} `json:"params,omitempty"`
}

// HandleEventArgs wraps a PluginEvent for the RPC call.
type HandleEventArgs struct {
	Event PluginEvent `json:"event"`
}
