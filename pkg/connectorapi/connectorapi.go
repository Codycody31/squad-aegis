// Package connectorapi is the public SDK for authoring Squad Aegis connectors (native .so or WASM sideloads).
package connectorapi

import (
	"go.codycody31.dev/squad-aegis/internal/plugin_manager"
	"go.codycody31.dev/squad-aegis/internal/shared/plug_config_schema"
)

const NativeConnectorEntrySymbol = "GetAegisConnector"

const NativeConnectorHostAPIVersion = plugin_manager.NativeConnectorHostAPIVersion

const ConnectorWireProtocolV1 = plugin_manager.ConnectorWireProtocolV1

// Native plugin capability used by native plugins that invoke ConnectorAPI.Call.
const NativePluginCapabilityAPIConnector = plugin_manager.NativePluginCapabilityAPIConnector

type PluginSource = plugin_manager.PluginSource
type PluginDistribution = plugin_manager.PluginDistribution
type PluginInstallState = plugin_manager.PluginInstallState

type Connector = plugin_manager.Connector
type ConnectorDefinition = plugin_manager.ConnectorDefinition
type ConnectorStatus = plugin_manager.ConnectorStatus
type ConnectorInvokeRequest = plugin_manager.ConnectorInvokeRequest
type ConnectorInvokeResponse = plugin_manager.ConnectorInvokeResponse
type InvokableConnector = plugin_manager.InvokableConnector

type ConfigSchema = plug_config_schema.ConfigSchema
type ConfigField = plug_config_schema.ConfigField
type FieldType = plug_config_schema.FieldType

const (
	PluginSourceBundled = plugin_manager.PluginSourceBundled
	PluginSourceNative  = plugin_manager.PluginSourceNative
	PluginSourceWasm    = plugin_manager.PluginSourceWasm

	PluginDistributionBundled  = plugin_manager.PluginDistributionBundled
	PluginDistributionSideload = plugin_manager.PluginDistributionSideload

	PluginInstallStateReady          = plugin_manager.PluginInstallStateReady
	PluginInstallStateNotInstalled   = plugin_manager.PluginInstallStateNotInstalled
	PluginInstallStatePendingRestart = plugin_manager.PluginInstallStatePendingRestart
	PluginInstallStateError          = plugin_manager.PluginInstallStateError

	ConnectorStatusStopped  = plugin_manager.ConnectorStatusStopped
	ConnectorStatusStarting = plugin_manager.ConnectorStatusStarting
	ConnectorStatusRunning  = plugin_manager.ConnectorStatusRunning
	ConnectorStatusStopping = plugin_manager.ConnectorStatusStopping
	ConnectorStatusError    = plugin_manager.ConnectorStatusError
	ConnectorStatusDisabled = plugin_manager.ConnectorStatusDisabled
)

type PluginPackageTarget = plugin_manager.PluginPackageTarget

func NativePluginHostCapabilities() []string {
	return plugin_manager.NativePluginHostCapabilities()
}
