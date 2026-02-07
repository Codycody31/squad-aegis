// Package permissions defines all permission constants and types for the PBAC system.
package permissions

import "strings"

// Permission represents a permission code string.
type Permission string

// Category represents the category of a permission.
type Category string

const (
	CategorySystem Category = "system"
	CategoryUI     Category = "ui"
	CategoryRCON   Category = "rcon"
)

// Wildcard permission grants all access.
const Wildcard Permission = "*"

// UI Permissions - Control access to pages/components.
const (
	UIDashboardView   Permission = "ui:dashboard:view"
	UIAuditLogsView   Permission = "ui:audit_logs:view"
	UIMetricsView     Permission = "ui:metrics:view"
	UIFeedsView       Permission = "ui:feeds:view"
	UIConsoleView     Permission = "ui:console:view"
	UIConsoleExecute  Permission = "ui:console:execute"
	UIPluginsView     Permission = "ui:plugins:view"
	UIPluginsManage   Permission = "ui:plugins:manage"
	UIWorkflowsView   Permission = "ui:workflows:view"
	UIWorkflowsManage Permission = "ui:workflows:manage"
	UISettingsView    Permission = "ui:settings:view"
	UISettingsManage  Permission = "ui:settings:manage"
	UIUsersManage     Permission = "ui:users:manage"
	UIRolesManage     Permission = "ui:roles:manage"
	UIBansView        Permission = "ui:bans:view"
	UIBansCreate      Permission = "ui:bans:create"
	UIBansEdit        Permission = "ui:bans:edit"
	UIBansDelete      Permission = "ui:bans:delete"
	UIPlayersView     Permission = "ui:players:view"
	UIPlayersKick     Permission = "ui:players:kick"
	UIPlayersWarn     Permission = "ui:players:warn"
	UIPlayersMove     Permission = "ui:players:move"
	UIRulesView       Permission = "ui:rules:view"
	UIRulesManage     Permission = "ui:rules:manage"
	UIBanListsView    Permission = "ui:ban_lists:view"
	UIBanListsManage  Permission = "ui:ban_lists:manage"
	UIMOTDView        Permission = "ui:motd:view"
	UIMOTDManage      Permission = "ui:motd:manage"
)

// RCON/Squad Permissions - Map to Squad's admin.cfg permissions.
const (
	RCONReserve         Permission = "rcon:reserve"
	RCONBalance         Permission = "rcon:balance"
	RCONCanSeeAdminChat Permission = "rcon:canseeadminchat"
	RCONManageServer    Permission = "rcon:manageserver"
	RCONTeamChange      Permission = "rcon:teamchange"
	RCONChat            Permission = "rcon:chat"
	RCONCameraman       Permission = "rcon:cameraman"
	RCONKick            Permission = "rcon:kick"
	RCONBan             Permission = "rcon:ban"
	RCONForceTeamChange Permission = "rcon:forceteamchange"
	RCONImmune          Permission = "rcon:immune"
	RCONChangeMap       Permission = "rcon:changemap"
	RCONPause           Permission = "rcon:pause"
	RCONCheat           Permission = "rcon:cheat"
	RCONPrivate         Permission = "rcon:private"
	RCONConfig          Permission = "rcon:config"
	RCONFeatureTest     Permission = "rcon:featuretest"
	RCONDemos           Permission = "rcon:demos"
	RCONDisbandSquad    Permission = "rcon:disbandsquad"
	RCONRemoveFromSquad Permission = "rcon:removefromsquad"
	RCONDemoteCommander Permission = "rcon:demotecommander"
	RCONDebug           Permission = "rcon:debug"
)

// SquadPermissionMap maps RCON permission codes to Squad's admin.cfg format.
var SquadPermissionMap = map[Permission]string{
	RCONReserve:         "reserve",
	RCONBalance:         "balance",
	RCONCanSeeAdminChat: "canseeadminchat",
	RCONManageServer:    "manageserver",
	RCONTeamChange:      "teamchange",
	RCONChat:            "chat",
	RCONCameraman:       "cameraman",
	RCONKick:            "kick",
	RCONBan:             "ban",
	RCONForceTeamChange: "forceteamchange",
	RCONImmune:          "immune",
	RCONChangeMap:       "changemap",
	RCONPause:           "pause",
	RCONCheat:           "cheat",
	RCONPrivate:         "private",
	RCONConfig:          "config",
	RCONFeatureTest:     "featuretest",
	RCONDemos:           "demos",
	RCONDisbandSquad:    "disbandSquad",
	RCONRemoveFromSquad: "removeFromSquad",
	RCONDemoteCommander: "demoteCommander",
	RCONDebug:           "debug",
}

// ReverseSquadPermissionMap maps Squad admin.cfg permission names to RCON permission codes.
var ReverseSquadPermissionMap = map[string]Permission{
	"reserve":         RCONReserve,
	"balance":         RCONBalance,
	"canseeadminchat": RCONCanSeeAdminChat,
	"manageserver":    RCONManageServer,
	"teamchange":      RCONTeamChange,
	"chat":            RCONChat,
	"cameraman":       RCONCameraman,
	"kick":            RCONKick,
	"ban":             RCONBan,
	"forceteamchange": RCONForceTeamChange,
	"immune":          RCONImmune,
	"changemap":       RCONChangeMap,
	"pause":           RCONPause,
	"cheat":           RCONCheat,
	"private":         RCONPrivate,
	"config":          RCONConfig,
	"featuretest":     RCONFeatureTest,
	"demos":           RCONDemos,
	"disbandSquad":    RCONDisbandSquad,
	"removeFromSquad": RCONRemoveFromSquad,
	"demoteCommander": RCONDemoteCommander,
	"debug":           RCONDebug,
}

// GetCategory returns the category of a permission.
func (p Permission) GetCategory() Category {
	if p == Wildcard {
		return CategorySystem
	}

	parts := strings.SplitN(string(p), ":", 2)
	if len(parts) < 2 {
		return CategorySystem
	}

	switch parts[0] {
	case "ui":
		return CategoryUI
	case "rcon":
		return CategoryRCON
	default:
		return CategorySystem
	}
}

// ToSquadPermission converts an RCON permission to its Squad admin.cfg equivalent.
// Returns empty string if not an RCON permission.
func (p Permission) ToSquadPermission() string {
	if squadPerm, ok := SquadPermissionMap[p]; ok {
		return squadPerm
	}
	return ""
}

// IsRCON returns true if this is an RCON permission.
func (p Permission) IsRCON() bool {
	return p.GetCategory() == CategoryRCON
}

// IsUI returns true if this is a UI permission.
func (p Permission) IsUI() bool {
	return p.GetCategory() == CategoryUI
}

// IsWildcard returns true if this is the wildcard permission.
func (p Permission) IsWildcard() bool {
	return p == Wildcard
}

// String returns the string representation of the permission.
func (p Permission) String() string {
	return string(p)
}

// AllPermissions returns all defined permissions.
func AllPermissions() []Permission {
	return []Permission{
		Wildcard,
		// UI
		UIDashboardView, UIAuditLogsView, UIMetricsView, UIFeedsView,
		UIConsoleView, UIConsoleExecute, UIPluginsView, UIPluginsManage,
		UIWorkflowsView, UIWorkflowsManage, UISettingsView, UISettingsManage,
		UIUsersManage, UIRolesManage, UIBansView, UIBansCreate, UIBansEdit, UIBansDelete,
		UIPlayersView, UIPlayersKick, UIPlayersWarn, UIPlayersMove,
		UIRulesView, UIRulesManage, UIBanListsView, UIBanListsManage,
		UIMOTDView, UIMOTDManage,
		// RCON
		RCONReserve, RCONBalance, RCONCanSeeAdminChat, RCONManageServer,
		RCONTeamChange, RCONChat, RCONCameraman, RCONKick, RCONBan,
		RCONForceTeamChange, RCONImmune, RCONChangeMap, RCONPause,
		RCONCheat, RCONPrivate, RCONConfig, RCONFeatureTest, RCONDemos,
		RCONDisbandSquad, RCONRemoveFromSquad, RCONDemoteCommander, RCONDebug,
	}
}

// UIPermissions returns all UI permissions.
func UIPermissions() []Permission {
	return []Permission{
		UIDashboardView, UIAuditLogsView, UIMetricsView, UIFeedsView,
		UIConsoleView, UIConsoleExecute, UIPluginsView, UIPluginsManage,
		UIWorkflowsView, UIWorkflowsManage, UISettingsView, UISettingsManage,
		UIUsersManage, UIRolesManage, UIBansView, UIBansCreate, UIBansEdit, UIBansDelete,
		UIPlayersView, UIPlayersKick, UIPlayersWarn, UIPlayersMove,
		UIRulesView, UIRulesManage, UIBanListsView, UIBanListsManage,
		UIMOTDView, UIMOTDManage,
	}
}

// RCONPermissions returns all RCON permissions.
func RCONPermissions() []Permission {
	return []Permission{
		RCONReserve, RCONBalance, RCONCanSeeAdminChat, RCONManageServer,
		RCONTeamChange, RCONChat, RCONCameraman, RCONKick, RCONBan,
		RCONForceTeamChange, RCONImmune, RCONChangeMap, RCONPause,
		RCONCheat, RCONPrivate, RCONConfig, RCONFeatureTest, RCONDemos,
		RCONDisbandSquad, RCONRemoveFromSquad, RCONDemoteCommander, RCONDebug,
	}
}
