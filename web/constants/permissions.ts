// Permission constants for the PBAC system
// These must stay in sync with internal/permissions/permissions.go

export type PermissionCategory = "system" | "ui" | "rcon";

// Wildcard permission grants all access
export const PERMISSION_WILDCARD = "*";

// UI Permissions - Control access to pages/components
export const UI_PERMISSIONS = {
  DASHBOARD_VIEW: "ui:dashboard:view",
  AUDIT_LOGS_VIEW: "ui:audit_logs:view",
  METRICS_VIEW: "ui:metrics:view",
  FEEDS_VIEW: "ui:feeds:view",
  CONSOLE_VIEW: "ui:console:view",
  CONSOLE_EXECUTE: "ui:console:execute",
  PLUGINS_VIEW: "ui:plugins:view",
  PLUGINS_MANAGE: "ui:plugins:manage",
  WORKFLOWS_VIEW: "ui:workflows:view",
  WORKFLOWS_MANAGE: "ui:workflows:manage",
  SETTINGS_VIEW: "ui:settings:view",
  SETTINGS_MANAGE: "ui:settings:manage",
  USERS_MANAGE: "ui:users:manage",
  ROLES_MANAGE: "ui:roles:manage",
  BANS_VIEW: "ui:bans:view",
  BANS_CREATE: "ui:bans:create",
  BANS_EDIT: "ui:bans:edit",
  BANS_DELETE: "ui:bans:delete",
  PLAYERS_VIEW: "ui:players:view",
  PLAYERS_KICK: "ui:players:kick",
  PLAYERS_WARN: "ui:players:warn",
  PLAYERS_MOVE: "ui:players:move",
  RULES_VIEW: "ui:rules:view",
  RULES_MANAGE: "ui:rules:manage",
  BAN_LISTS_VIEW: "ui:ban_lists:view",
  BAN_LISTS_MANAGE: "ui:ban_lists:manage",
  MOTD_VIEW: "ui:motd:view",
  MOTD_MANAGE: "ui:motd:manage",
} as const;

// RCON/Squad Permissions - Map to Squad's admin.cfg permissions
export const RCON_PERMISSIONS = {
  RESERVE: "rcon:reserve",
  BALANCE: "rcon:balance",
  CAN_SEE_ADMIN_CHAT: "rcon:canseeadminchat",
  MANAGE_SERVER: "rcon:manageserver",
  TEAM_CHANGE: "rcon:teamchange",
  CHAT: "rcon:chat",
  CAMERAMAN: "rcon:cameraman",
  KICK: "rcon:kick",
  BAN: "rcon:ban",
  FORCE_TEAM_CHANGE: "rcon:forceteamchange",
  IMMUNE: "rcon:immune",
  CHANGE_MAP: "rcon:changemap",
  PAUSE: "rcon:pause",
  CHEAT: "rcon:cheat",
  PRIVATE: "rcon:private",
  CONFIG: "rcon:config",
  FEATURE_TEST: "rcon:featuretest",
  DEMOS: "rcon:demos",
  DISBAND_SQUAD: "rcon:disbandsquad",
  REMOVE_FROM_SQUAD: "rcon:removefromsquad",
  DEMOTE_COMMANDER: "rcon:demotecommander",
  DEBUG: "rcon:debug",
} as const;

// Combined permissions object for convenience
export const Permissions = {
  WILDCARD: PERMISSION_WILDCARD,
  UI: UI_PERMISSIONS,
  RCON: RCON_PERMISSIONS,
} as const;

// Type for any permission string
export type Permission =
  | typeof PERMISSION_WILDCARD
  | (typeof UI_PERMISSIONS)[keyof typeof UI_PERMISSIONS]
  | (typeof RCON_PERMISSIONS)[keyof typeof RCON_PERMISSIONS];

// Maps RCON permission codes to Squad's admin.cfg format
export const SQUAD_PERMISSION_MAP: Record<string, string> = {
  "rcon:reserve": "reserve",
  "rcon:balance": "balance",
  "rcon:canseeadminchat": "canseeadminchat",
  "rcon:manageserver": "manageserver",
  "rcon:teamchange": "teamchange",
  "rcon:chat": "chat",
  "rcon:cameraman": "cameraman",
  "rcon:kick": "kick",
  "rcon:ban": "ban",
  "rcon:forceteamchange": "forceteamchange",
  "rcon:immune": "immune",
  "rcon:changemap": "changemap",
  "rcon:pause": "pause",
  "rcon:cheat": "cheat",
  "rcon:private": "private",
  "rcon:config": "config",
  "rcon:featuretest": "featuretest",
  "rcon:demos": "demos",
  "rcon:disbandsquad": "disbandSquad",
  "rcon:removefromsquad": "removeFromSquad",
  "rcon:demotecommander": "demoteCommander",
  "rcon:debug": "debug",
};

// Helper to get permission category
export function getPermissionCategory(permission: string): PermissionCategory {
  if (permission === PERMISSION_WILDCARD) return "system";
  const prefix = permission.split(":")[0];
  switch (prefix) {
    case "ui":
      return "ui";
    case "rcon":
      return "rcon";
    default:
      return "system";
  }
}

// Helper to check if permission is RCON type
export function isRconPermission(permission: string): boolean {
  return permission.startsWith("rcon:");
}

// Helper to check if permission is UI type
export function isUiPermission(permission: string): boolean {
  return permission.startsWith("ui:");
}

// Helper to convert RCON permission to Squad format
export function toSquadPermission(permission: string): string | null {
  return SQUAD_PERMISSION_MAP[permission] || null;
}

// Permission category display names
export const PERMISSION_CATEGORY_NAMES: Record<PermissionCategory, string> = {
  system: "System",
  ui: "User Interface",
  rcon: "RCON / Squad Server",
};

// All permissions as a flat array
export const ALL_PERMISSIONS: Permission[] = [
  PERMISSION_WILDCARD,
  ...Object.values(UI_PERMISSIONS),
  ...Object.values(RCON_PERMISSIONS),
];
