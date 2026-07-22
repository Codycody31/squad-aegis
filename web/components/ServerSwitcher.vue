<script setup lang="ts">
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuShortcut,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";

import {
  SidebarMenu,
  SidebarMenuButton,
  SidebarMenuItem,
  useSidebar,
} from "@/components/ui/sidebar";
import { ChevronsUpDown } from "lucide-vue-next";

import type { Server } from "~/types";
import { useServersStore } from "@/stores/servers";

const emit = defineEmits<{
  (e: "click"): void;
}>();

const serversStore = useServersStore();
const { isMobile } = useSidebar();
const router = useRouter();

const onServerSelect = (server: Server) => {
  serversStore.setActiveServerId(server.id);
  router.push(`/servers/${server.id}`);
  emit("click");
};
</script>

<template>
  <SidebarMenu>
    <SidebarMenuItem>
      <DropdownMenu>
        <DropdownMenuTrigger as-child>
          <SidebarMenuButton
            size="lg"
            class="data-[state=open]:bg-sidebar-accent data-[state=open]:text-sidebar-accent-foreground"
          >
            <Avatar class="h-8 w-8 rounded-lg" v-if="serversStore.activeServer">
              <AvatarImage
                :src="`/api/images/avatar?username=${serversStore.activeServer?.name}&width=256&height=256&gridsize=8`"
                :alt="serversStore.activeServer?.name"
              />
              <AvatarFallback class="rounded-lg">
                {{ serversStore.activeServer?.name?.slice(0, 2)?.toUpperCase() || "CN" }}
              </AvatarFallback>
            </Avatar>
            <div
              class="grid flex-1 text-left text-sm leading-tight"
              v-if="serversStore.activeServer"
            >
              <span class="truncate font-semibold">
                {{ serversStore.activeServer?.name }}
              </span>
              <span class="truncate text-xs">{{
                serversStore.activeServer?.ip_address +
                ":" +
                serversStore.activeServer?.game_port
              }}</span>
            </div>
            <div class="grid flex-1 text-left text-sm leading-tight" v-else>
              Select server
            </div>
            <ChevronsUpDown class="ml-auto" />
          </SidebarMenuButton>
        </DropdownMenuTrigger>
        <DropdownMenuContent
          class="w-[--reka-dropdown-menu-trigger-width] min-w-56 rounded-lg"
          align="start"
          :side="isMobile ? 'bottom' : 'right'"
          :side-offset="4"
        >
          <DropdownMenuLabel class="text-xs text-muted-foreground">
            Servers
          </DropdownMenuLabel>
          <DropdownMenuItem
            v-for="server in serversStore.servers"
            :key="server.name"
            class="gap-2 p-2"
            @click="onServerSelect(server)"
          >
            <Avatar class="h-8 w-8 rounded-lg">
              <AvatarImage
                :src="`/api/images/avatar?username=${server.name}&width=256&height=256&gridsize=8`"
                :alt="server.name"
              />
            </Avatar>
            {{ server.name }}
          </DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>
    </SidebarMenuItem>
  </SidebarMenu>
</template>
