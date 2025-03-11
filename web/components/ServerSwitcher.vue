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
import { ChevronsUpDown, Plus } from "lucide-vue-next";

import { ref } from "vue";
import type { Server } from "~/types";

const props = defineProps<{
  servers: Server[];
}>();

const emit = defineEmits<{
  (e: "click"): void;
}>();

const uiStore = useUiStore();
const { isMobile } = useSidebar();
const router = useRouter();

const onServerSelect = (server: Server) => {
  uiStore.setActiveServer(server);
  useRouter().push(`/servers/${server.id}`);
  emit("click");
};

const handleServerChange = (serverId: string | undefined) => {
  if (!serverId) {
    uiStore.setActiveServer(null);
    return;
  }

  const server = props.servers.find((server) => server.id === serverId);
  if (server) {
    uiStore.setActiveServer(server);
  } else {
    uiStore.setActiveServer(null);
  }
};

watch(
  () => router.currentRoute.value.params.serverId,
  (newServerId) => {
    handleServerChange(newServerId as string);
  }
);

handleServerChange(router.currentRoute.value.params.serverId as string);
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
            <div
              class="grid flex-1 text-left text-sm leading-tight"
              v-if="uiStore.activeServer"
            >
              <span class="truncate font-semibold">
                {{ uiStore.activeServer?.name }}
              </span>
              <span class="truncate text-xs">{{
                uiStore.activeServer?.ip_address +
                ":" +
                uiStore.activeServer?.game_port
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
            v-for="(server, index) in props.servers"
            :key="server.name"
            class="gap-2 p-2"
            @click="onServerSelect(server)"
          >
            {{ server.name }}
          </DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>
    </SidebarMenuItem>
  </SidebarMenu>
</template>
