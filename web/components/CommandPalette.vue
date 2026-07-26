<script setup lang="ts">
import { computed, ref, watch } from "vue";
import { useMagicKeys, watchDebounced } from "@vueuse/core";
import {
  CommandDialog,
  CommandEmpty,
  CommandGroup,
  CommandInput,
  CommandItem,
  CommandList,
  CommandSeparator,
} from "~/components/ui/command";
import rootNavigationItems from "~/navigation";
import serverNavigationItems from "~/navigation/server";
import type { NavigationItem } from "~/types";
import { useAuthStore } from "~/stores/auth";
import { useServersStore } from "~/stores/servers";

const { isOpen } = useCommandPalette();
const router = useRouter();
const authStore = useAuthStore();
const serversStore = useServersStore();
const runtimeConfig = useRuntimeConfig();

const keys = useMagicKeys({
  passive: false,
  onEventFired(e) {
    if (e.key === "k" && (e.metaKey || e.ctrlKey) && e.type === "keydown") {
      e.preventDefault();
    }
  },
});
watch([keys["Meta+K"], keys["Ctrl+K"]], ([meta, ctrl]) => {
  if (meta || ctrl) isOpen.value = !isOpen.value;
});

const canShow = (item: NavigationItem, serverId: string | null): boolean => {
  if (!item.permissions || item.permissions.length === 0) return true;
  if (item.permissions.includes("super_admin"))
    return authStore.user?.super_admin || false;
  if (authStore.user?.super_admin) return true;
  if (!serverId) return false;
  return authStore.hasAnyPermission(serverId, ...(item.permissions ?? []));
};

const navEntries = computed(() => {
  const serverId = serversStore.activeServerId;
  const entries: { title: string; icon?: string; to: any }[] = [];
  for (const item of rootNavigationItems) {
    if (item.title && item.to && canShow(item, serverId)) {
      entries.push({ title: item.title, icon: item.icon, to: item.to });
    }
  }
  if (serverId) {
    const serverName = serversStore.activeServer?.name ?? "Server";
    for (const item of serverNavigationItems as NavigationItem[]) {
      if (item.title && item.to && canShow(item, serverId)) {
        entries.push({
          title: `${serverName} · ${item.title}`,
          icon: item.icon,
          to: { ...item.to, params: { serverId } },
        });
      }
    }
  }
  return entries;
});

interface PlayerHit {
  player_name?: string;
  steam_id?: string;
  eos_id?: string;
  epic_id?: string;
}

const query = ref("");
const playerResults = ref<PlayerHit[]>([]);
const searchingPlayers = ref(false);

watchDebounced(
  query,
  async (q) => {
    if (!q || q.length < 3) {
      playerResults.value = [];
      return;
    }
    searchingPlayers.value = true;
    try {
      const data = await useAuthFetchImperative<{
        data?: { players?: PlayerHit[] };
      }>(
        `${runtimeConfig.public.backendApi}/players?search=${encodeURIComponent(q)}`
      );
      playerResults.value = (data.data?.players ?? []).slice(0, 8);
    } catch {
      playerResults.value = [];
    } finally {
      searchingPlayers.value = false;
    }
  },
  { debounce: 300 }
);

watch(isOpen, (open) => {
  if (!open) {
    query.value = "";
    playerResults.value = [];
  }
});

const go = (to: any) => {
  isOpen.value = false;
  router.push(to);
};

const goToPlayer = (player: PlayerHit) => {
  const id = player.steam_id || player.eos_id || player.epic_id;
  if (!id) return;
  isOpen.value = false;
  router.push(`/players/${id}`);
};
</script>

<template>
  <CommandDialog v-model:open="isOpen">
    <CommandInput
      placeholder="Search pages, servers, players…"
      @input="(e: Event) => (query = (e.target as HTMLInputElement).value)"
    />
    <CommandList>
      <CommandEmpty>
        {{ searchingPlayers ? "Searching…" : "No results found." }}
      </CommandEmpty>
      <CommandGroup heading="Navigation">
        <CommandItem
          v-for="entry in navEntries"
          :key="entry.title"
          :value="entry.title"
          @select="go(entry.to)"
        >
          <Icon v-if="entry.icon" :name="entry.icon" class="mr-2 h-4 w-4" />
          {{ entry.title }}
        </CommandItem>
      </CommandGroup>
      <CommandSeparator />
      <CommandGroup heading="Servers">
        <CommandItem
          v-for="server in serversStore.servers"
          :key="server.id"
          :value="`server ${server.name}`"
          @select="go(`/servers/${server.id}`)"
        >
          <Icon name="mdi:server" class="mr-2 h-4 w-4" />
          {{ server.name }}
        </CommandItem>
      </CommandGroup>
      <template v-if="playerResults.length > 0">
        <CommandSeparator />
        <CommandGroup heading="Players">
          <CommandItem
            v-for="player in playerResults"
            :key="player.steam_id || player.eos_id || player.epic_id || ''"
            :value="`player ${player.player_name} ${player.steam_id ?? ''}`"
            @select="goToPlayer(player)"
          >
            <Icon name="mdi:account" class="mr-2 h-4 w-4" />
            <span class="truncate">{{ player.player_name || "Unknown" }}</span>
            <span class="ml-2 truncate text-xs text-muted-foreground">
              {{ player.steam_id || player.eos_id || player.epic_id }}
            </span>
          </CommandItem>
        </CommandGroup>
      </template>
    </CommandList>
  </CommandDialog>
</template>
