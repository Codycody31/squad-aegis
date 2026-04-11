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

// PluginSource mirrors plugin_manager.PluginSource on the wire.
type PluginSource string

const (
	PluginSourceBundled PluginSource = "bundled"
	PluginSourceNative  PluginSource = "native"
)

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

// EventSource mirrors plugin_manager.EventSource on the wire.
type EventSource string

const (
	EventSourceRCON      EventSource = "rcon"
	EventSourceLog       EventSource = "log"
	EventSourceSystem    EventSource = "system"
	EventSourceConnector EventSource = "connector"
	EventSourcePlugin    EventSource = "plugin"
)

// CommandExecutionType mirrors plugin_manager.CommandExecutionType.
type CommandExecutionType string

const (
	CommandExecutionSync  CommandExecutionType = "sync"
	CommandExecutionAsync CommandExecutionType = "async"
)

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

// PluginDefinition is the wire-safe subset of plugin_manager.PluginDefinition.
// The host enriches the definition with runtime metadata (install state,
// runtime path, signature status, etc.) after Load.
type PluginDefinition struct {
	ID                     string       `json:"id"`
	Name                   string       `json:"name"`
	Description            string       `json:"description,omitempty"`
	Version                string       `json:"version"`
	Author                 string       `json:"author,omitempty"`
	Source                 PluginSource `json:"source,omitempty"`
	Official               bool         `json:"official,omitempty"`
	MinHostAPIVersion      int          `json:"min_host_api_version,omitempty"`
	RequiredCapabilities   []string     `json:"required_capabilities,omitempty"`
	AllowMultipleInstances bool         `json:"allow_multiple_instances,omitempty"`
	RequiredConnectors     []string     `json:"required_connectors,omitempty"`
	OptionalConnectors     []string     `json:"optional_connectors,omitempty"`
	ConfigSchema           ConfigSchema `json:"config_schema"`
	Events                 []string     `json:"events,omitempty"`
	LongRunning            bool         `json:"long_running,omitempty"`
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
