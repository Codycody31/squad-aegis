// Package connectorrpc is the subprocess-isolated SDK for authoring Squad
// Aegis native connectors. Connectors built against this package run as
// standalone binaries; the host spawns them via hashicorp/go-plugin and
// communicates over net/rpc, so a crashing or malicious connector cannot
// corrupt the host process memory.
//
// This package mirrors pkg/pluginrpc for the connector surface. See the
// pluginrpc package doc for the overall design.
package connectorrpc

import "time"

// WireProtocolVersion bumps when the connector wire format changes.
const WireProtocolVersion = 1

// PluginSource mirrors the plugin_manager.PluginSource constants.
type PluginSource string

const (
	PluginSourceBundled PluginSource = "bundled"
	PluginSourceNative  PluginSource = "native"
)

// ConnectorStatus mirrors plugin_manager.ConnectorStatus on the wire.
type ConnectorStatus string

const (
	ConnectorStatusStopped  ConnectorStatus = "stopped"
	ConnectorStatusStarting ConnectorStatus = "starting"
	ConnectorStatusRunning  ConnectorStatus = "running"
	ConnectorStatusStopping ConnectorStatus = "stopping"
	ConnectorStatusError    ConnectorStatus = "error"
	ConnectorStatusDisabled ConnectorStatus = "disabled"
)

// FieldType mirrors plug_config_schema.FieldType.
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

// ConfigField mirrors plug_config_schema.ConfigField.
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

// ConfigSchema mirrors plug_config_schema.ConfigSchema.
type ConfigSchema struct {
	Fields []ConfigField `json:"fields"`
}

// ConnectorDefinition is the wire-safe subset of plugin_manager.ConnectorDefinition.
type ConnectorDefinition struct {
	ID                   string       `json:"id"`
	LegacyIDs            []string     `json:"legacy_ids,omitempty"`
	InstanceKey          string       `json:"instance_key,omitempty"`
	Source               PluginSource `json:"source,omitempty"`
	Name                 string       `json:"name"`
	Description          string       `json:"description,omitempty"`
	Version              string       `json:"version"`
	Author               string       `json:"author,omitempty"`
	ConfigSchema         ConfigSchema `json:"config_schema"`
	MinHostAPIVersion    int          `json:"min_host_api_version,omitempty"`
	RequiredCapabilities []string     `json:"required_capabilities,omitempty"`
}

// ConnectorInvokeRequest mirrors plugin_manager.ConnectorInvokeRequest.
type ConnectorInvokeRequest struct {
	V    string                 `json:"v"`
	Data map[string]interface{} `json:"data"`
}

// ConnectorInvokeResponse mirrors plugin_manager.ConnectorInvokeResponse.
type ConnectorInvokeResponse struct {
	V     string                 `json:"v"`
	OK    bool                   `json:"ok"`
	Data  map[string]interface{} `json:"data,omitempty"`
	Error string                 `json:"error,omitempty"`
}

// InitializeArgs is the RPC payload passed from host → connector.
type InitializeArgs struct {
	Config     map[string]interface{} `json:"config"`
	InstanceID string                 `json:"instance_id,omitempty"`
}

// InvokeArgs is the RPC payload for Invoke calls.
type InvokeArgs struct {
	Request   ConnectorInvokeRequest `json:"request"`
	TimeoutMs int64                  `json:"timeout_ms,omitempty"`
}

// WaitTimeout is a sentinel used to clamp per-call invoke waits.
var WaitTimeout = 30 * time.Second
