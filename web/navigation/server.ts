import { UI_PERMISSIONS, RCON_PERMISSIONS } from "@/constants/permissions";

export default [
  { heading: "Server" },
  {
    title: "Dashboard",
    icon: "lucide:layout-dashboard",
    to: {
      name: "servers-serverId",
    },
    permissions: [UI_PERMISSIONS.DASHBOARD_VIEW],
  },
  {
    title: "Teams & Squads",
    icon: "mdi:flag-variant",
    to: {
      name: "servers-serverId-teams-and-squads",
    },
    permissions: [UI_PERMISSIONS.PLAYERS_VIEW],
  },
  {
    title: "Disconnected Players",
    icon: "mdi:account-off",
    to: {
      name: "servers-serverId-disconnected-players",
    },
    permissions: [UI_PERMISSIONS.PLAYERS_VIEW],
  },
  {
    title: "Banned Players",
    icon: "mdi:account-remove",
    to: {
      name: "servers-serverId-banned-players",
    },
    permissions: [UI_PERMISSIONS.BANS_VIEW],
  },
  {
    title: "Users & Roles",
    icon: "mdi:account-star",
    to: {
      name: "servers-serverId-users-and-roles",
    },
    permissions: ["super_admin"],
  },
  {
    title: "Console",
    icon: "mdi:console",
    to: {
      name: "servers-serverId-console",
    },
    permissions: [UI_PERMISSIONS.CONSOLE_VIEW, RCON_PERMISSIONS.MANAGE_SERVER],
  },
  {
    title: "Feeds",
    icon: "mdi:rss",
    to: {
      name: "servers-serverId-feeds",
    },
    permissions: [UI_PERMISSIONS.FEEDS_VIEW],
  },
  {
    title: "Metrics",
    icon: "mdi:chart-line",
    to: {
      name: "servers-serverId-metrics",
    },
    permissions: [UI_PERMISSIONS.METRICS_VIEW],
  },
  {
    title: "Audit Logs",
    icon: "mdi:book-open",
    to: {
      name: "servers-serverId-audit-logs",
    },
    permissions: [UI_PERMISSIONS.AUDIT_LOGS_VIEW, RCON_PERMISSIONS.MANAGE_SERVER],
  },
  {
    title: "Rules",
    icon: "mdi:book-open",
    to: {
      name: "servers-serverId-rules",
    },
    permissions: [UI_PERMISSIONS.RULES_VIEW],
  },
  {
    title: "MOTD",
    icon: "mdi:message-text",
    to: {
      name: "servers-serverId-motd",
    },
    permissions: [UI_PERMISSIONS.MOTD_VIEW],
  },
  {
    title: "Plugins",
    icon: "lucide:puzzle",
    to: {
      name: "servers-serverId-plugins",
    },
    permissions: [UI_PERMISSIONS.PLUGINS_VIEW, RCON_PERMISSIONS.MANAGE_SERVER],
  },
  {
    title: "Workflows",
    icon: "mdi:workflow",
    to: {
      name: "servers-serverId-workflows",
    },
    permissions: [UI_PERMISSIONS.WORKFLOWS_VIEW, RCON_PERMISSIONS.MANAGE_SERVER],
  },
  {
    title: "Settings",
    icon: "mdi:cog",
    to: {
      name: "servers-serverId-settings",
    },
    permissions: [UI_PERMISSIONS.SETTINGS_VIEW],
  },
];
