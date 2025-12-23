<script setup lang="ts">
import { SidebarInset, SidebarProvider } from "@/components/ui/sidebar";
import AppSidebar from "@/components/AppSidebar.vue";
import Header from "@/components/Header.vue";
import type { NavigationItem } from "~/types";

const sudoNavigationItems: NavigationItem[] = [
  {
    heading: "Instance Management",
  },
  {
    title: "Dashboard",
    to: {
      name: "sudo",
    },
    icon: "mdi:shield-crown",
  },
  {
    title: "Storage",
    to: {
      name: "sudo-storage",
    },
    icon: "mdi:folder",
  },
  {
    title: "Analytics",
    to: {
      name: "sudo-metrics",
    },
    icon: "mdi:chart-bar",
  },
  {
    title: "System Health",
    to: {
      name: "sudo-system",
    },
    icon: "mdi:monitor-dashboard",
  },
  {
    title: "Configuration",
    to: {
      name: "sudo-config",
    },
    icon: "mdi:cog",
  },
  {
    title: "Audit Logs",
    to: {
      name: "sudo-audit",
    },
    icon: "mdi:file-document-multiple",
  },
  {
    title: "Sessions",
    to: {
      name: "sudo-sessions",
    },
    icon: "mdi:account-key",
  },
  {
    title: "Database",
    to: {
      name: "sudo-database",
    },
    icon: "mdi:database-cog",
  },
];
</script>

<template>
  <SidebarProvider>
    <AppSidebar :navigationItems="sudoNavigationItems" />
    <SidebarInset>
      <Header />
      <NuxtPage />
    </SidebarInset>
  </SidebarProvider>
</template>

