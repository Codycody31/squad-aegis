import type { NavigationItem } from "~/types";

const navigationItems: NavigationItem[] = [
  {
    heading: "Overview",
  },
  {
    title: "Dashboard",
    to: {
      name: "dashboard",
    },
    icon: "mdi:home",
  },
  {
    title: "Users",
    to: {
      name: "users",
    },
    icon: "mdi:account-group",
    permissions: ["super_admin"],
  },
  {
    title: "Servers",
    to: {
      name: "servers",
    },
    icon: "mdi:server",
  },
  {
    title: "Admin Chat",
    to: {
      name: "admin-chat",
    },
    icon: "mdi:message",
  },
  {
    title: "Connectors",
    to: {
      name: "connectors",
    },
    icon: "mdi:server-network",
  },
];

export default navigationItems;
