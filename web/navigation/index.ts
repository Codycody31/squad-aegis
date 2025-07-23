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
  }
];

export default navigationItems;
