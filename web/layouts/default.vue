<script setup lang="ts">
import { SidebarInset, SidebarProvider } from "@/components/ui/sidebar";
import AppSidebar from "@/components/AppSidebar.vue";
import Header from "@/components/Header.vue";
import rootNavigationItems from "@/navigation";
import serverNavigationItems from "@/navigation/server";
import type { NavigationItem } from "~/types";

const route = useRoute();
let navigationItems = ref<NavigationItem[]>(rootNavigationItems)

if (route.params.serverId) {
  navigationItems.value = [...rootNavigationItems, ...serverNavigationItems]
}

watch(route, () => {
  if (route.params.serverId) {
    navigationItems.value = [...rootNavigationItems, ...serverNavigationItems]
  } else {
    navigationItems.value = rootNavigationItems
  }
})
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
