<script setup lang="ts">
import { ref, onMounted } from "vue";
import { useRouter } from "vue-router";
import { Card, CardContent } from "~/components/ui/card";
import { Badge } from "~/components/ui/badge";
import { Button } from "~/components/ui/button";
import {
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
} from "~/components/ui/collapsible";
import type { AltAccountGroup, AltAccountGroupsResponse } from "~/types/player";

const runtimeConfig = useRuntimeConfig();
const router = useRouter();

const loading = ref(true);
const error = ref<string | null>(null);
const altGroups = ref<AltAccountGroup[]>([]);
const totalGroups = ref(0);
const page = ref(1);
const limit = 10;
const expandedGroups = ref<Set<string>>(new Set());

async function fetchAltGroups() {
  loading.value = true;
  error.value = null;

  const cookieToken = useCookie(
    runtimeConfig.public.sessionCookieName as string
  );
  const token = cookieToken.value;

  if (!token) {
    error.value = "Authentication required";
    loading.value = false;
    return;
  }

  try {
    const response = await fetch(
      `${runtimeConfig.public.backendApi}/players/alt-groups?page=${page.value}&limit=${limit}`,
      {
        method: "GET",
        headers: {
          "Content-Type": "application/json",
          Authorization: `Bearer ${token}`,
        },
        credentials: "include",
      }
    );

    if (response.status === 403) {
      error.value = "permission_denied";
      loading.value = false;
      return;
    }

    if (!response.ok) {
      throw new Error("Failed to fetch alt account groups");
    }

    const data: { data: AltAccountGroupsResponse } = await response.json();
    altGroups.value = data.data.alt_groups || [];
    totalGroups.value = data.data.total_groups || 0;
  } catch (err: any) {
    error.value = err.message || "Failed to load alt account groups";
  } finally {
    loading.value = false;
  }
}

function toggleGroup(groupId: string) {
  if (expandedGroups.value.has(groupId)) {
    expandedGroups.value.delete(groupId);
  } else {
    expandedGroups.value.add(groupId);
  }
  expandedGroups.value = new Set(expandedGroups.value);
}

function viewPlayer(player: { steam_id: string; eos_id: string }) {
  const playerId = player.steam_id || player.eos_id;
  if (playerId) {
    router.push(`/players/${playerId}`);
  }
}

function getTimeAgo(dateString: string | null): string {
  if (!dateString) return "Unknown";
  const date = new Date(dateString);
  const now = new Date();
  const diff = now.getTime() - date.getTime();

  const minutes = Math.floor(diff / 60000);
  const hours = Math.floor(minutes / 60);
  const days = Math.floor(hours / 24);

  if (days > 0) return `${days}d ago`;
  if (hours > 0) return `${hours}h ago`;
  if (minutes > 0) return `${minutes}m ago`;
  return "Just now";
}

function loadMore() {
  page.value++;
  fetchAltGroups();
}

onMounted(() => {
  fetchAltGroups();
});
</script>

<template>
  <Card>
    <div class="px-4 py-3 border-b">
      <span class="text-sm font-medium flex items-center gap-2">
        <svg
          xmlns="http://www.w3.org/2000/svg"
          class="h-4 w-4 text-orange-500"
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          stroke-width="2"
        >
          <path d="M16 21v-2a4 4 0 0 0-4-4H6a4 4 0 0 0-4 4v2" />
          <circle cx="9" cy="7" r="4" />
          <path d="M22 21v-2a4 4 0 0 0-3-3.87" />
          <path d="M16 3.13a4 4 0 0 1 0 7.75" />
        </svg>
        Suspected Alts
      </span>
      <p class="text-xs text-muted-foreground mt-1">
        Players sharing IP addresses
      </p>
    </div>
    <CardContent class="pt-3">

      <!-- Loading State -->
      <div v-if="loading && altGroups.length === 0" class="py-4 text-center">
        <div class="animate-spin rounded-full h-6 w-6 border-b-2 border-primary mx-auto"></div>
        <p class="text-xs text-muted-foreground mt-2">Loading...</p>
      </div>

      <!-- Permission Denied -->
      <div v-else-if="error === 'permission_denied'" class="py-4 text-center text-muted-foreground">
        <p class="text-xs">Super admin access required</p>
      </div>

      <!-- Error State -->
      <div v-else-if="error && error !== 'permission_denied'" class="py-4 text-center text-destructive">
        <p class="text-sm">{{ error }}</p>
        <Button
          variant="outline"
          size="sm"
          class="mt-2"
          @click="fetchAltGroups"
        >
          Retry
        </Button>
      </div>

      <!-- Empty State -->
      <div v-else-if="altGroups.length === 0" class="py-4 text-center text-muted-foreground">
        <p class="text-xs">No suspected alt accounts found</p>
      </div>

      <!-- Alt Groups List -->
      <div v-else class="space-y-2">
        <Collapsible
          v-for="group in altGroups"
          :key="group.group_id"
          :open="expandedGroups.has(group.group_id)"
          @update:open="toggleGroup(group.group_id)"
        >
          <CollapsibleTrigger
            class="w-full flex items-center justify-between p-2 rounded-lg border hover:bg-muted/50 transition-colors"
          >
            <div class="flex items-center gap-2">
              <div class="flex -space-x-1.5">
                <div
                  v-for="(player, idx) in group.players.slice(0, 3)"
                  :key="idx"
                  class="w-6 h-6 rounded-full bg-muted border border-background flex items-center justify-center text-xs font-medium"
                  :class="{ 'bg-destructive/20 text-destructive': player.is_banned }"
                >
                  {{ player.player_name?.charAt(0)?.toUpperCase() || "?" }}
                </div>
                <div
                  v-if="group.players.length > 3"
                  class="w-6 h-6 rounded-full bg-muted border border-background flex items-center justify-center text-xs"
                >
                  +{{ group.players.length - 3 }}
                </div>
              </div>
              <div class="text-left">
                <p class="text-xs font-medium">{{ group.players.length }} Players</p>
                <p class="text-xs text-muted-foreground">{{ getTimeAgo(group.last_activity) }}</p>
              </div>
            </div>
            <div class="flex items-center gap-2">
              <Badge
                v-if="group.players.some((p) => p.is_banned)"
                variant="destructive"
                class="text-xs"
              >
                Has Bans
              </Badge>
              <svg
                xmlns="http://www.w3.org/2000/svg"
                class="h-4 w-4 transition-transform"
                :class="{ 'rotate-180': expandedGroups.has(group.group_id) }"
                viewBox="0 0 24 24"
                fill="none"
                stroke="currentColor"
                stroke-width="2"
              >
                <path d="m6 9 6 6 6-6" />
              </svg>
            </div>
          </CollapsibleTrigger>
          <CollapsibleContent>
            <div class="mt-2 ml-4 space-y-1">
              <div
                v-for="player in group.players"
                :key="player.steam_id || player.eos_id"
                class="flex items-center justify-between p-2 rounded hover:bg-muted/30 cursor-pointer"
                @click="viewPlayer(player)"
              >
                <div class="flex items-center gap-2">
                  <span class="text-sm">{{ player.player_name || "Unknown" }}</span>
                  <Badge
                    v-if="player.is_banned"
                    variant="destructive"
                    class="text-xs"
                  >
                    Banned
                  </Badge>
                </div>
                <div class="flex items-center gap-3 text-xs text-muted-foreground">
                  <span>{{ player.shared_sessions }} sessions</span>
                  <span>{{ getTimeAgo(player.last_seen) }}</span>
                </div>
              </div>
            </div>
          </CollapsibleContent>
        </Collapsible>

        <!-- Load More -->
        <div v-if="altGroups.length < totalGroups" class="pt-3">
          <Button
            variant="outline"
            size="sm"
            class="w-full"
            :disabled="loading"
            @click="loadMore"
          >
            <span v-if="loading">Loading...</span>
            <span v-else>Load More ({{ totalGroups - altGroups.length }} remaining)</span>
          </Button>
        </div>
      </div>
    </CardContent>
  </Card>
</template>
