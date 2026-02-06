<script setup lang="ts">
import { computed } from "vue";
import { useRoute } from "vue-router";
import { Avatar, AvatarFallback, AvatarImage } from "@/components/ui/avatar";
import {
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
} from "@/components/ui/collapsible";
import {
  Sidebar,
  SidebarContent,
  SidebarGroup,
  SidebarGroupLabel,
  SidebarHeader,
  SidebarMenu,
  SidebarMenuItem,
  SidebarMenuSub,
  SidebarMenuSubButton,
  SidebarMenuSubItem,
  SidebarRail,
  SidebarMenuButton,
  SidebarFooter,
  useSidebar,
} from "@/components/ui/sidebar";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuGroup,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog";
import type { NavigationItem, Server } from "~/types";
import { useAuthStore } from "@/stores/auth";

defineProps<{ navigationItems: NavigationItem[] }>();

const runtimeConfig = useRuntimeConfig();
const route = useRoute();
const pathname = computed(() => route.path);
const servers = ref<Server[]>([]);
const sidebar = useSidebar();
const authStore = useAuthStore();
const activeServer = ref<Server | null>(null);
const userServerRoles = ref<{ [serverId: string]: string[] }>({});

// About Dialog
const isAboutDialogOpen = ref(false);
const appVersion = ref<string | null>(null);
const developerName = "Codycody31"; // As per GitHub repo owner
const githubRepoUrl = "https://github.com/Codycody31/squad-aegis"; // From git remote URL

const fetchVersionInfo = async () => {
  try {
    const response = await $fetch<{ data: { version: string } }>(
      "http://localhost:3113/api/"
    );
    appVersion.value = response.data.version;
  } catch (error) {
    console.error("Failed to fetch version info:", error);
    appVersion.value = "N/A";
  }
};

const openAboutDialog = async () => {
  await fetchVersionInfo();
  isAboutDialogOpen.value = true;
  closeSidebarOnMobile();
};

// Function to close sidebar on mobile
const closeSidebarOnMobile = () => {
  if (sidebar.isMobile.value) {
    sidebar.setOpenMobile(false);
  }
};

// Check if user is super admin
const isSuperAdmin = computed(() => {
  return authStore.user?.super_admin || false;
});

// Check if a navigation item should be visible based on permissions
const shouldShowNavItem = (item: NavigationItem): boolean => {
  // If it's just a heading, always show it
  if (item.heading && (!item.items || item.items.length === 0)) {
    return true;
  }

  // If no permissions are set, show it
  if (!item.permissions || item.permissions.length === 0) {
    return true;
  }

  // If the route is admin-only and user is not super admin, hide it
  if (item.permissions?.includes("super_admin")) {
    return isSuperAdmin.value;
  }

  if (isSuperAdmin.value) {
    return true;
  }

  const serverId = activeServer.value?.id;
  if (!serverId) return false;

  return authStore.hasAnyPermission(serverId, ...(item.permissions ?? []));
};

// Fetch servers
const fetchServers = async () => {
  interface ServerResponse {
    data?: {
      servers: Server[];
    };
  }

  const response = await useFetch<ServerResponse>(
    `${runtimeConfig.public.backendApi}/servers`,
    {
      headers: {
        Authorization: `Bearer ${
          useCookie(runtimeConfig.public.sessionCookieName).value
        }`,
      },
    }
  );

  if (response.error.value) {
    console.error("Error fetching servers:", response.error.value);
  } else {
    servers.value = response.data.value?.data?.servers ?? [];

    // If we're on a server route, find the active server
    if (route.params.serverId) {
      const serverId = route.params.serverId as string;
      activeServer.value =
        servers.value.find((server) => server.id === serverId) || null;
    }
  }
};

const logout = () => {
  useAuthStore().logout();
  document.cookie = `${runtimeConfig.public.sessionCookieName}=; expires=Thu, 01 Jan 1970 00:00:00 UTC; path=/;`;
  navigateTo("/login");
};

// Watch for route changes to update active server
watch(route, async () => {
  if (route.params.serverId) {
    const serverId = route.params.serverId as string;
    activeServer.value =
      servers.value.find((server) => server.id === serverId) || null;
  } else {
    activeServer.value = null;
  }
});

await fetchServers();
</script>

<template>
  <Sidebar collapsible="icon">
    <SidebarHeader>
      <ServerSwitcher :servers="servers" @click="closeSidebarOnMobile" />
    </SidebarHeader>
    <SidebarContent class="overflow-x-hidden">
      <SidebarGroup>
        <SidebarMenu>
          <template
            v-for="item in navigationItems"
            :key="item.title || item.heading"
          >
            <SidebarGroupLabel
              v-if="
                item.heading &&
                (!item.items || item.items.length === 0) &&
                shouldShowNavItem(item)
              "
            >
              {{ item.heading }}
            </SidebarGroupLabel>

            <Collapsible
              v-else-if="
                item.items && item.items.length > 0 && shouldShowNavItem(item)
              "
              asChild
              :defaultOpen="item.isActive"
              class="group/collapsible"
            >
              <SidebarMenuItem>
                <CollapsibleTrigger asChild>
                  <SidebarMenuButton
                    :tooltip="item.title"
                    :isActive="pathname === item.to?.name"
                  >
                    <Icon :name="item.icon" v-if="item.icon" class="size-4 shrink-0 text-sidebar-foreground [&>svg]:size-4 [&>svg]:shrink-0 [&>svg]:text-sidebar-foreground" />
                    <span class="group-data-[collapsible=icon]:hidden">{{ item.title }}</span>
                    <Icon
                      name="lucide:chevron-right"
                      class="ml-auto size-4 transition-transform duration-200 group-data-[state=open]/collapsible:rotate-90 group-data-[collapsible=icon]:hidden shrink-0 text-sidebar-foreground"
                    />
                  </SidebarMenuButton>
                </CollapsibleTrigger>
                <CollapsibleContent>
                  <SidebarMenuSub>
                    <template
                      v-for="subItem in item.items"
                      :key="subItem.title"
                    >
                      <SidebarMenuSubItem v-if="shouldShowNavItem(subItem)">
                        <SidebarMenuSubButton
                          asChild
                          :isActive="pathname === subItem.to?.name"
                        >
                          <RouterLink
                            :to="subItem.to?.name || ''"
                            @click="closeSidebarOnMobile"
                          >
                            <Icon :name="subItem.icon" v-if="subItem.icon" />
                            <span>{{ subItem.title }}</span>
                          </RouterLink>
                        </SidebarMenuSubButton>
                      </SidebarMenuSubItem>
                    </template>
                  </SidebarMenuSub>
                </CollapsibleContent>
              </SidebarMenuItem>
            </Collapsible>

            <SidebarMenuItem v-else-if="shouldShowNavItem(item)">
              <SidebarMenuButton
                asChild
                :tooltip="item.title"
                :isActive="pathname === item.to?.name"
              >
                <RouterLink
                  :to="item.to ? item.to : '/'"
                  @click="closeSidebarOnMobile"
                >
                  <Icon :name="item.icon" v-if="item.icon" class="size-4 shrink-0 text-sidebar-foreground [&>svg]:size-4 [&>svg]:shrink-0 [&>svg]:text-sidebar-foreground" />
                  <span class="group-data-[collapsible=icon]:hidden">{{ item.title }}</span>
                </RouterLink>
              </SidebarMenuButton>
            </SidebarMenuItem>
          </template>
        </SidebarMenu>
      </SidebarGroup>
    </SidebarContent>
    <SidebarFooter>
      <SidebarMenu>
        <SidebarMenuItem>
          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <SidebarMenuButton
                size="lg"
                class="data-[state=open]:bg-sidebar-accent data-[state=open]:text-sidebar-accent-foreground"
              >
                <Avatar class="h-8 w-8 rounded-lg">
                  <AvatarImage
                    :src="`/api/images/avatar?username=${useAuthStore().user?.name}&width=256&height=256&gridsize=8`"
                    :alt="useAuthStore().user?.name ?? ''"
                  />
                  <AvatarFallback class="rounded-lg">
                    {{
                      useAuthStore().user?.name?.slice(0, 2)?.toUpperCase() ||
                      "CN"
                    }}
                  </AvatarFallback>
                </Avatar>
                <div class="grid flex-1 text-left text-sm leading-tight">
                  <span class="truncate font-semibold">
                    {{ useAuthStore().user?.name }}
                  </span>
                  <span class="truncate text-xs">
                    {{ useAuthStore().user?.username }}
                  </span>
                </div>
                <Icon name="lucide:chevrons-up-down" class="ml-auto size-4" />
              </SidebarMenuButton>
            </DropdownMenuTrigger>
            <DropdownMenuContent
              class="w-[--radix-dropdown-menu-trigger-width] min-w-56 rounded-lg"
              side="bottom"
              align="end"
            >
              <DropdownMenuLabel class="p-0 font-normal">
                <div
                  class="flex items-center gap-2 px-1 py-1.5 text-left text-sm"
                >
                  <Avatar class="h-8 w-8 rounded-lg">
                    <AvatarImage
                      :src="`/api/images/avatar?username=${useAuthStore().user?.name}&width=256&height=256&gridsize=8`"
                      :alt="useAuthStore().user?.name ?? ''"
                    />
                    <AvatarFallback class="rounded-lg">
                      {{
                        useAuthStore().user?.name?.slice(0, 2)?.toUpperCase() ||
                        "CN"
                      }}
                    </AvatarFallback>
                  </Avatar>
                  <div class="grid flex-1 text-left text-sm leading-tight">
                    <span class="truncate font-semibold">
                      {{ useAuthStore().user?.name }}
                    </span>
                    <span class="truncate text-xs">
                      {{ useAuthStore().user?.username }}
                    </span>
                  </div>
                </div>
              </DropdownMenuLabel>
              <DropdownMenuSeparator />

              <DropdownMenuGroup>
                <DropdownMenuItem v-if="authStore.user?.super_admin" as-child>
                  <RouterLink to="/sudo" @click="closeSidebarOnMobile">
                    <Icon name="mdi:shield-crown" />
                    Super
                  </RouterLink>
                </DropdownMenuItem>
                <DropdownMenuItem as-child>
                  <RouterLink to="/settings" @click="closeSidebarOnMobile">
                    <Icon name="lucide:settings" />
                    Settings
                  </RouterLink>
                </DropdownMenuItem>
                <DropdownMenuItem @click="openAboutDialog">
                  <Icon name="lucide:info" />
                  About
                </DropdownMenuItem>
              </DropdownMenuGroup>
              <DropdownMenuSeparator />
              <DropdownMenuItem
                @click="
                  () => {
                    closeSidebarOnMobile();
                    logout();
                  }
                "
              >
                <Icon name="lucide:log-out" />
                Log out
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
        </SidebarMenuItem>
      </SidebarMenu>
    </SidebarFooter>
    <SidebarRail />
  </Sidebar>

  <Dialog v-model:open="isAboutDialogOpen">
    <DialogContent class="sm:max-w-[425px]">
      <DialogHeader>
        <DialogTitle>About Squad Aegis</DialogTitle>
        <DialogDescription>
          Application information and details.
        </DialogDescription>
      </DialogHeader>
      <div class="grid gap-4 py-4">
        <div class="grid grid-cols-4 items-center gap-4">
          <span class="text-sm font-medium">Version:</span>
          <span class="col-span-3">{{ appVersion ?? 'Loading...' }}</span>
        </div>
        <div class="grid grid-cols-4 items-center gap-4">
          <span class="text-sm font-medium">Developer:</span>
          <span class="col-span-3">{{ developerName }}</span>
        </div>
        <div class="grid grid-cols-4 items-center gap-4">
          <span class="text-sm font-medium">Source:</span>
          <a
            :href="githubRepoUrl"
            target="_blank"
            rel="noopener noreferrer"
            class="col-span-3 text-primary hover:underline"
          >
            GitHub Repository
          </a>
        </div>
      </div>
    </DialogContent>
  </Dialog>
</template>
