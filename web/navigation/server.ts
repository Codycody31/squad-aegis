export default [
  { heading: "Server" },
  {
    title: "Dashboard",
    icon: "lucide:layout-dashboard",
    to: {
      name: "servers-serverId",
    },
  },
  {
    title: "Connected Players",
    icon: "mdi:account-multiple",
    to: {
      name: "servers-serverId-connected-players",
    },
  },
  {
    title: "Disconnected Players",
    icon: "mdi:account-off",
    to: {
      name: "servers-serverId-disconnected-players",
    },
  },
  {
    title: "Banned Players",
    icon: "mdi:account-remove",
    to: {
      name: "servers-serverId-banned-players",
    },
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
    title: "Teams & Squads",
    icon: "mdi:flag-variant",
    to: {
      name: "servers-serverId-teams-and-squads",
    },
  },
  {
    title: "Console",
    icon: "mdi:console",
    to: {
      name: "servers-serverId-console",
    },
    permissions: ["super_admin", "manageserver"],
  },
  {
    title: "Feeds",
    icon: "mdi:rss",
    to: {
      name: "servers-serverId-feeds",
    },
  },
  {
    title: "Metrics",
    icon: "mdi:chart-line",
    to: {
      name: "servers-serverId-metrics",
    },
  },
  {
    title: "Audit Logs",
    icon: "mdi:book-open",
    to: {
      name: "servers-serverId-audit-logs",
    },
    permissions: ["super_admin", "manageserver"],
  },
  {
    title: "Rules",
    icon: "mdi:book-open",
    to: {
      name: "servers-serverId-rules",
    },
  },
  {
    title: "Plugins",
    icon: "lucide:puzzle",
    to: {
      name: "servers-serverId-plugins",
    },
    permissions: ["super_admin", "manageserver"],
  },
  {
    title: "Settings",
    icon: "mdi:cog",
    to: {
      name: "servers-serverId-settings",
    },
  },
];
