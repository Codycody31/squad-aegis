<script setup lang="ts">
import { SidebarInset, SidebarProvider } from "@/components/ui/sidebar";
import AppSidebar from "@/components/AppSidebar.vue";
import Header from "@/components/Header.vue";
import rootNavigationItems from "@/navigation";
import serverNavigationItems from "@/navigation/server";
import type { NavigationItem } from "~/types";

const route = useRoute();
let navigationItems = ref<NavigationItem[]>(rootNavigationItems);

// Function to process server navigation items with the current serverId
const getServerNavItems = (serverId: string) => {
  return serverNavigationItems.map(item => {
    // Create a deep copy of the item to avoid modifying the original
    const newItem = { ...item };
    
    // If the item has a 'to' property, ensure the serverId param is set
    if (newItem.to && typeof newItem.to === 'object') {
      newItem.to = {
        ...newItem.to,
        params: { 
          ...newItem.to.params,
          serverId 
        }
      };
    }
    
    return newItem;
  });
};

// Initialize navigation items
if (route.params.serverId) {
  const serverId = Array.isArray(route.params.serverId) 
    ? route.params.serverId[0] 
    : route.params.serverId;
  navigationItems.value = [...rootNavigationItems, ...getServerNavItems(serverId)];
}

// Update navigation items when route changes
watch(route, () => {
  if (route.params.serverId) {
    const serverId = Array.isArray(route.params.serverId) 
      ? route.params.serverId[0] 
      : route.params.serverId;
    navigationItems.value = [...rootNavigationItems, ...getServerNavItems(serverId)];
  } else {
    navigationItems.value = rootNavigationItems;
  }
});
</script>

<template>
  <SidebarProvider>
    <AppSidebar :navigationItems="navigationItems" />
    <SidebarInset>
      <Header />
      <NuxtPage />
    </SidebarInset>
  </SidebarProvider>
</template>
