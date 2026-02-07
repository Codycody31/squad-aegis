import { computed, type ComputedRef } from "vue";
import { useAuthStore } from "@/stores/auth";
import { type Permission, UI_PERMISSIONS } from "@/constants/permissions";

/**
 * Composable for checking permissions in Vue components.
 * Provides reactive permission checking with pre-computed common permission checks.
 *
 * @param serverId - Optional server ID. If not provided, only super admin check is available.
 */
export function usePermissions(serverId?: string | ComputedRef<string>) {
  const authStore = useAuthStore();

  // Resolve serverId if it's a computed ref
  const resolvedServerId = computed(() => {
    if (!serverId) return undefined;
    return typeof serverId === "string" ? serverId : serverId.value;
  });

  // Check if user is a super admin
  const isSuperAdmin = computed(() => authStore.isSuperAdmin);

  // Check if user has any permissions for the server
  const hasAnyServerAccess = computed(() => {
    const sid = resolvedServerId.value;
    if (!sid) return authStore.isSuperAdmin;
    const perms = authStore.getServerPermissions(sid);
    return authStore.isSuperAdmin || (perms && perms.length > 0);
  });

  // Permission check functions
  const hasPermission = (permission: Permission | string): boolean => {
    const sid = resolvedServerId.value;
    if (!sid) return authStore.isSuperAdmin;
    return authStore.hasPermission(sid, permission);
  };

  const hasAnyPermission = (...permissions: (Permission | string)[]): boolean => {
    const sid = resolvedServerId.value;
    if (!sid) return authStore.isSuperAdmin;
    return authStore.hasAnyPermission(sid, ...permissions);
  };

  const hasAllPermissions = (...permissions: (Permission | string)[]): boolean => {
    const sid = resolvedServerId.value;
    if (!sid) return authStore.isSuperAdmin;
    return authStore.hasAllPermissions(sid, ...permissions);
  };

  // Pre-computed common permission checks (reactive)

  // Dashboard & Navigation
  const canViewDashboard = computed(() =>
    hasPermission(UI_PERMISSIONS.DASHBOARD_VIEW)
  );
  const canViewAuditLogs = computed(() =>
    hasPermission(UI_PERMISSIONS.AUDIT_LOGS_VIEW)
  );
  const canViewMetrics = computed(() =>
    hasPermission(UI_PERMISSIONS.METRICS_VIEW)
  );
  const canViewFeeds = computed(() =>
    hasPermission(UI_PERMISSIONS.FEEDS_VIEW)
  );

  // Console
  const canViewConsole = computed(() =>
    hasPermission(UI_PERMISSIONS.CONSOLE_VIEW)
  );
  const canExecuteConsole = computed(() =>
    hasPermission(UI_PERMISSIONS.CONSOLE_EXECUTE)
  );

  // Plugins
  const canViewPlugins = computed(() =>
    hasPermission(UI_PERMISSIONS.PLUGINS_VIEW)
  );
  const canManagePlugins = computed(() =>
    hasPermission(UI_PERMISSIONS.PLUGINS_MANAGE)
  );

  // Workflows
  const canViewWorkflows = computed(() =>
    hasPermission(UI_PERMISSIONS.WORKFLOWS_VIEW)
  );
  const canManageWorkflows = computed(() =>
    hasPermission(UI_PERMISSIONS.WORKFLOWS_MANAGE)
  );

  // Settings
  const canViewSettings = computed(() =>
    hasPermission(UI_PERMISSIONS.SETTINGS_VIEW)
  );
  const canManageSettings = computed(() =>
    hasPermission(UI_PERMISSIONS.SETTINGS_MANAGE)
  );

  // Users & Roles
  const canManageUsers = computed(() =>
    hasPermission(UI_PERMISSIONS.USERS_MANAGE)
  );
  const canManageRoles = computed(() =>
    hasPermission(UI_PERMISSIONS.ROLES_MANAGE)
  );

  // Bans
  const canViewBans = computed(() =>
    hasPermission(UI_PERMISSIONS.BANS_VIEW)
  );
  const canCreateBans = computed(() =>
    hasPermission(UI_PERMISSIONS.BANS_CREATE)
  );
  const canEditBans = computed(() =>
    hasPermission(UI_PERMISSIONS.BANS_EDIT)
  );
  const canDeleteBans = computed(() =>
    hasPermission(UI_PERMISSIONS.BANS_DELETE)
  );
  const canManageBans = computed(() =>
    hasAnyPermission(
      UI_PERMISSIONS.BANS_CREATE,
      UI_PERMISSIONS.BANS_EDIT,
      UI_PERMISSIONS.BANS_DELETE
    )
  );

  // Players
  const canViewPlayers = computed(() =>
    hasPermission(UI_PERMISSIONS.PLAYERS_VIEW)
  );
  const canKickPlayers = computed(() =>
    hasPermission(UI_PERMISSIONS.PLAYERS_KICK)
  );
  const canWarnPlayers = computed(() =>
    hasPermission(UI_PERMISSIONS.PLAYERS_WARN)
  );
  const canMovePlayers = computed(() =>
    hasPermission(UI_PERMISSIONS.PLAYERS_MOVE)
  );

  // Rules
  const canViewRules = computed(() =>
    hasPermission(UI_PERMISSIONS.RULES_VIEW)
  );
  const canManageRules = computed(() =>
    hasPermission(UI_PERMISSIONS.RULES_MANAGE)
  );

  // Ban Lists
  const canViewBanLists = computed(() =>
    hasPermission(UI_PERMISSIONS.BAN_LISTS_VIEW)
  );
  const canManageBanLists = computed(() =>
    hasPermission(UI_PERMISSIONS.BAN_LISTS_MANAGE)
  );

  return {
    // Core checks
    isSuperAdmin,
    hasAnyServerAccess,
    hasPermission,
    hasAnyPermission,
    hasAllPermissions,

    // Dashboard & Navigation
    canViewDashboard,
    canViewAuditLogs,
    canViewMetrics,
    canViewFeeds,

    // Console
    canViewConsole,
    canExecuteConsole,

    // Plugins
    canViewPlugins,
    canManagePlugins,

    // Workflows
    canViewWorkflows,
    canManageWorkflows,

    // Settings
    canViewSettings,
    canManageSettings,

    // Users & Roles
    canManageUsers,
    canManageRoles,

    // Bans
    canViewBans,
    canCreateBans,
    canEditBans,
    canDeleteBans,
    canManageBans,

    // Players
    canViewPlayers,
    canKickPlayers,
    canWarnPlayers,
    canMovePlayers,

    // Rules
    canViewRules,
    canManageRules,

    // Ban Lists
    canViewBanLists,
    canManageBanLists,
  };
}
