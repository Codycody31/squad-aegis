package pluginapi

import (
	"go.codycody31.dev/squad-aegis/internal/event_manager"
	"go.codycody31.dev/squad-aegis/internal/plugin_manager"
	"go.codycody31.dev/squad-aegis/internal/shared/plug_config_schema"
)

const NativePluginEntrySymbol = "GetAegisPlugin"
const NativePluginHostAPIVersion = plugin_manager.NativePluginHostAPIVersion
const WasmPluginHostABIVersion = plugin_manager.WasmPluginHostABIVersion

// ConnectorWireProtocolV1 is the JSON "v" field for ConnectorAPI.Call requests.
const ConnectorWireProtocolV1 = plugin_manager.ConnectorWireProtocolV1

const (
	NativePluginCapabilityEntrypointGetAegisPlugin = plugin_manager.NativePluginCapabilityEntrypointGetAegisPlugin
	NativePluginCapabilityAPIRCON                  = plugin_manager.NativePluginCapabilityAPIRCON
	NativePluginCapabilityAPIServer                = plugin_manager.NativePluginCapabilityAPIServer
	NativePluginCapabilityAPIDatabase              = plugin_manager.NativePluginCapabilityAPIDatabase
	NativePluginCapabilityAPIRule                  = plugin_manager.NativePluginCapabilityAPIRule
	NativePluginCapabilityAPIAdmin                 = plugin_manager.NativePluginCapabilityAPIAdmin
	NativePluginCapabilityAPIDiscord               = plugin_manager.NativePluginCapabilityAPIDiscord
	NativePluginCapabilityAPIConnector             = plugin_manager.NativePluginCapabilityAPIConnector
	NativePluginCapabilityAPIEvent                 = plugin_manager.NativePluginCapabilityAPIEvent
	NativePluginCapabilityAPILog                   = plugin_manager.NativePluginCapabilityAPILog
	NativePluginCapabilityEventsRCON               = plugin_manager.NativePluginCapabilityEventsRCON
	NativePluginCapabilityEventsLog                = plugin_manager.NativePluginCapabilityEventsLog
	NativePluginCapabilityEventsSystem             = plugin_manager.NativePluginCapabilityEventsSystem
	NativePluginCapabilityEventsConnector          = plugin_manager.NativePluginCapabilityEventsConnector
	NativePluginCapabilityEventsPlugin             = plugin_manager.NativePluginCapabilityEventsPlugin
)

type Plugin = plugin_manager.Plugin
type PluginDefinition = plugin_manager.PluginDefinition
type PluginAPIs = plugin_manager.PluginAPIs
type PluginEvent = plugin_manager.PluginEvent
type PluginStatus = plugin_manager.PluginStatus
type PluginCommand = plugin_manager.PluginCommand
type CommandResult = plugin_manager.CommandResult
type CommandExecutionStatus = plugin_manager.CommandExecutionStatus
type PluginSource = plugin_manager.PluginSource
type PluginDistribution = plugin_manager.PluginDistribution
type PluginInstallState = plugin_manager.PluginInstallState
type PluginPackageTarget = plugin_manager.PluginPackageTarget
type PluginPackageManifest = plugin_manager.PluginPackageManifest

type ConfigSchema = plug_config_schema.ConfigSchema
type ConfigField = plug_config_schema.ConfigField
type FieldType = plug_config_schema.FieldType

type EventType = event_manager.EventType
type EventData = event_manager.EventData

type ServerAPI = plugin_manager.ServerAPI
type DatabaseAPI = plugin_manager.DatabaseAPI
type RuleAPI = plugin_manager.RuleAPI
type RconAPI = plugin_manager.RconAPI
type AdminAPI = plugin_manager.AdminAPI
type EventAPI = plugin_manager.EventAPI
type DiscordAPI = plugin_manager.DiscordAPI
type ConnectorAPI = plugin_manager.ConnectorAPI
type ConnectorInvokeRequest = plugin_manager.ConnectorInvokeRequest
type ConnectorInvokeResponse = plugin_manager.ConnectorInvokeResponse
type LogAPI = plugin_manager.LogAPI

type ServerInfo = plugin_manager.ServerInfo
type PlayerInfo = plugin_manager.PlayerInfo
type AdminInfo = plugin_manager.AdminInfo
type SquadInfo = plugin_manager.SquadInfo
type RuleInfo = plugin_manager.RuleInfo
type RuleActionInfo = plugin_manager.RuleActionInfo
type PlayerAdminStatus = plugin_manager.PlayerAdminStatus
type PlayerAdminRole = plugin_manager.PlayerAdminRole
type TemporaryAdminInfo = plugin_manager.TemporaryAdminInfo
type DiscordEmbed = plugin_manager.DiscordEmbed
type DiscordEmbedField = plugin_manager.DiscordEmbedField
type DiscordEmbedFooter = plugin_manager.DiscordEmbedFooter
type DiscordEmbedThumbnail = plugin_manager.DiscordEmbedThumbnail
type DiscordEmbedImage = plugin_manager.DiscordEmbedImage

type RconChatMessageData = event_manager.RconChatMessageData
type RconPlayerWarnedData = event_manager.RconPlayerWarnedData
type RconPlayerKickedData = event_manager.RconPlayerKickedData
type RconPlayerBannedData = event_manager.RconPlayerBannedData
type RconAdminCameraData = event_manager.RconAdminCameraData
type RconSquadCreatedData = event_manager.RconSquadCreatedData
type RconServerInfoData = event_manager.RconServerInfoData
type LogAdminBroadcastData = event_manager.LogAdminBroadcastData
type LogDeployableDamagedData = event_manager.LogDeployableDamagedData
type LogPlayerConnectedData = event_manager.LogPlayerConnectedData
type LogPlayerDamagedData = event_manager.LogPlayerDamagedData
type LogPlayerDiedData = event_manager.LogPlayerDiedData
type LogPlayerWoundedData = event_manager.LogPlayerWoundedData
type LogPlayerRevivedData = event_manager.LogPlayerRevivedData
type LogPlayerPossessData = event_manager.LogPlayerPossessData
type LogJoinSucceededData = event_manager.LogJoinSucceededData
type LogTickRateData = event_manager.LogTickRateData
type LogGameEventUnifiedData = event_manager.LogGameEventUnifiedData
type LogPlayerDisconnectedData = event_manager.LogPlayerDisconnectedData
type PluginCustomEventData = event_manager.PluginCustomEventData
type PluginLogEventData = event_manager.PluginLogEventData
type PlayerListUpdatedData = event_manager.PlayerListUpdatedData
type PlayerTeamChangedData = event_manager.PlayerTeamChangedData
type PlayerSquadChangedData = event_manager.PlayerSquadChangedData
type SquadCreatedData = event_manager.SquadCreatedData
type SquadDisbandedData = event_manager.SquadDisbandedData
type PlayerConnectedData = event_manager.PlayerConnectedData
type PlayerDisconnectedData = event_manager.PlayerDisconnectedData
type PlayerStatsUpdatedData = event_manager.PlayerStatsUpdatedData

func NativePluginHostCapabilities() []string {
	return plugin_manager.NativePluginHostCapabilities()
}

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

	PluginStatusStopped  = plugin_manager.PluginStatusStopped
	PluginStatusStarting = plugin_manager.PluginStatusStarting
	PluginStatusRunning  = plugin_manager.PluginStatusRunning
	PluginStatusStopping = plugin_manager.PluginStatusStopping
	PluginStatusError    = plugin_manager.PluginStatusError
	PluginStatusDisabled = plugin_manager.PluginStatusDisabled

	EventTypeAll                        = event_manager.EventTypeAll
	EventTypeRconChatMessage            = event_manager.EventTypeRconChatMessage
	EventTypeRconPlayerWarned           = event_manager.EventTypeRconPlayerWarned
	EventTypeRconPlayerKicked           = event_manager.EventTypeRconPlayerKicked
	EventTypeRconPlayerBanned           = event_manager.EventTypeRconPlayerBanned
	EventTypeRconPossessedAdminCamera   = event_manager.EventTypeRconPossessedAdminCamera
	EventTypeRconUnpossessedAdminCamera = event_manager.EventTypeRconUnpossessedAdminCamera
	EventTypeRconSquadCreated           = event_manager.EventTypeRconSquadCreated
	EventTypeRconServerInfo             = event_manager.EventTypeRconServerInfo
	EventTypeLogAdminBroadcast          = event_manager.EventTypeLogAdminBroadcast
	EventTypeLogDeployableDamaged       = event_manager.EventTypeLogDeployableDamaged
	EventTypeLogPlayerConnected         = event_manager.EventTypeLogPlayerConnected
	EventTypeLogPlayerDamaged           = event_manager.EventTypeLogPlayerDamaged
	EventTypeLogPlayerDied              = event_manager.EventTypeLogPlayerDied
	EventTypeLogPlayerWounded           = event_manager.EventTypeLogPlayerWounded
	EventTypeLogPlayerRevived           = event_manager.EventTypeLogPlayerRevived
	EventTypeLogPlayerPossess           = event_manager.EventTypeLogPlayerPossess
	EventTypeLogPlayerDisconnected      = event_manager.EventTypeLogPlayerDisconnected
	EventTypeLogJoinSucceeded           = event_manager.EventTypeLogJoinSucceeded
	EventTypeLogTickRate                = event_manager.EventTypeLogTickRate
	EventTypeLogGameEventUnified        = event_manager.EventTypeLogGameEventUnified
	EventTypePlayerListUpdated          = event_manager.EventTypePlayerListUpdated
	EventTypePlayerTeamChanged          = event_manager.EventTypePlayerTeamChanged
	EventTypePlayerSquadChanged         = event_manager.EventTypePlayerSquadChanged
	EventTypeSquadCreated               = event_manager.EventTypeSquadCreated
	EventTypeSquadDisbanded             = event_manager.EventTypeSquadDisbanded
	EventTypePlayerConnected            = event_manager.EventTypePlayerConnected
	EventTypePlayerDisconnected         = event_manager.EventTypePlayerDisconnected
	EventTypeEnhancedTeamkill           = event_manager.EventTypeEnhancedTeamkill
	EventTypePlayerStatsUpdated         = event_manager.EventTypePlayerStatsUpdated
	EventTypePluginCustom               = event_manager.EventTypePluginCustom
	EventTypePluginLog                  = event_manager.EventTypePluginLog

	FieldTypeString      = plug_config_schema.FieldTypeString
	FieldTypeInt         = plug_config_schema.FieldTypeInt
	FieldTypeBool        = plug_config_schema.FieldTypeBool
	FieldTypeObject      = plug_config_schema.FieldTypeObject
	FieldTypeArray       = plug_config_schema.FieldTypeArray
	FieldTypeArrayString = plug_config_schema.FieldTypeArrayString
	FieldTypeArrayInt    = plug_config_schema.FieldTypeArrayInt
	FieldTypeArrayBool   = plug_config_schema.FieldTypeArrayBool
	FieldTypeArrayObject = plug_config_schema.FieldTypeArrayObject
)
