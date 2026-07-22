<script setup lang="ts">
import { ref, onMounted, onUnmounted, computed } from "vue";
import { Button } from "~/components/ui/button";
import { Input } from "~/components/ui/input";
import { Card, CardContent, CardHeader, CardTitle } from "~/components/ui/card";
import { toast } from "~/components/ui/toast";
import type { Player } from "~/types";
import PlayerActionMenu from "~/components/PlayerActionMenu.vue";
import PlayerContextPanel from "~/components/player-panel/PlayerContextPanel.vue";
import PageHeader from "~/components/layout/PageHeader.vue";
import DataTable from "~/components/data-table/DataTable.vue";
import type { DataTableColumn } from "~/components/data-table/types";

definePageMeta({ middleware: ["auth"] });

const authStore = useAuthStore();
const route = useRoute();
const serverId = route.params.serverId;

const loading = ref(true);
const error = ref<string | null>(null);
const disconnectedPlayers = ref<Player[]>([]);
const refreshInterval = ref<NodeJS.Timeout | null>(null);
const searchQuery = ref("");

const { isOpen: panelOpen, selected: panelPlayer, openPanel } = usePlayerPanel();

interface PlayersResponse {
  data: {
    players: {
      disconnectedPlayers: Player[];
    };
  };
}

// Computed property for filtered players
const filteredPlayers = computed(() => {
  if (!searchQuery.value.trim()) {
    return disconnectedPlayers.value;
  }

  const query = searchQuery.value.toLowerCase();
  return disconnectedPlayers.value.filter(
    (player) =>
      player.name.toLowerCase().includes(query) ||
      player.steam_id.includes(query) ||
      player.eosId.toLowerCase().includes(query)
  );
});

const columns: DataTableColumn<Player>[] = [
  { key: "name", label: "Name" },
  { key: "ids", label: "IDs" },
  { key: "sinceDisconnect", label: "Disconnected For" },
];

// Function to fetch disconnected players data
async function fetchDisconnectedPlayers(background = false) {
  if (!background) loading.value = true;
  error.value = null;

  const runtimeConfig = useRuntimeConfig();

  try {
    const data = await useAuthFetchImperative<PlayersResponse>(
      `${runtimeConfig.public.backendApi}/servers/${serverId}/rcon/server-population`
    );

    if (data && data.data && data.data.players) {
      disconnectedPlayers.value = data.data.players.disconnectedPlayers || [];

      // Sort by disconnect time (most recent first)
      disconnectedPlayers.value.sort((a, b) => {
        // Parse the disconnect time strings (e.g., "5m30s")
        const getSeconds = (timeStr: string) => {
          const minutesMatch = timeStr.match(/(\d+)m/);
          const secondsMatch = timeStr.match(/(\d+)s/);

          const minutes = minutesMatch ? parseInt(minutesMatch[1]) : 0;
          const seconds = secondsMatch ? parseInt(secondsMatch[1]) : 0;

          return minutes * 60 + seconds;
        };

        return getSeconds(a.sinceDisconnect) - getSeconds(b.sinceDisconnect);
      });
    }
  } catch (err: any) {
    error.value = extractApiErrorMessage(
      err,
      "An error occurred while fetching disconnected players data"
    );
  } finally {
    loading.value = false;
  }
}

// Refresh data after action
function refreshAfterAction() {
  fetchDisconnectedPlayers(true);
}

// Format disconnect time to be more readable
function formatDisconnectTime(timeStr: string): string {
  // If already in a readable format, return as is
  if (
    timeStr.includes("h") ||
    (timeStr.includes("m") && timeStr.includes("s"))
  ) {
    return timeStr;
  }

  // Try to parse the time string
  const minutesMatch = timeStr.match(/(\d+)m/);
  const secondsMatch = timeStr.match(/(\d+)s/);

  if (!minutesMatch && !secondsMatch) {
    return timeStr; // Return original if we can't parse
  }

  const minutes = minutesMatch ? parseInt(minutesMatch[1]) : 0;
  const seconds = secondsMatch ? parseInt(secondsMatch[1]) : 0;

  // Format based on time elapsed
  if (minutes > 60) {
    const hours = Math.floor(minutes / 60);
    const remainingMinutes = minutes % 60;
    return `${hours}h ${remainingMinutes}m`;
  } else if (minutes > 0) {
    return `${minutes}m ${seconds}s`;
  } else {
    return `${seconds}s`;
  }
}

async function handleCopy(text: string) {
  const ok = await copyToClipboard(text);
  toast(
    ok
      ? { title: "Copied to clipboard" }
      : {
          title: "Copy failed",
          description: "Unable to access the clipboard in this browser.",
          variant: "destructive",
        }
  );
}

// Poll only while the tab is visible; refresh immediately on return
function handleVisibilityChange() {
  if (document.visibilityState === "visible") {
    fetchDisconnectedPlayers(true);
    startPolling();
  } else {
    stopPolling();
  }
}

function startPolling() {
  stopPolling();
  refreshInterval.value = setInterval(() => {
    fetchDisconnectedPlayers(true);
  }, 30000);
}

function stopPolling() {
  if (refreshInterval.value) {
    clearInterval(refreshInterval.value);
    refreshInterval.value = null;
  }
}

onMounted(() => {
  fetchDisconnectedPlayers();
  startPolling();
  document.addEventListener("visibilitychange", handleVisibilityChange);
});

onUnmounted(() => {
  stopPolling();
  document.removeEventListener("visibilitychange", handleVisibilityChange);
});

// Manual refresh function
function refreshData() {
  fetchDisconnectedPlayers();
}
</script>

<template>
  <div class="p-4 md:p-6">
    <PageHeader
      title="Disconnected Players"
      description="Recently disconnected players — useful for banning leavers who dodge punishment."
    >
      <template #actions>
        <Button @click="refreshData" :disabled="loading">
          {{ loading ? "Refreshing..." : "Refresh" }}
        </Button>
      </template>
    </PageHeader>

    <div
      v-if="error"
      class="mb-4 rounded-md bg-destructive/15 p-4 text-sm text-destructive"
    >
      {{ error }}
    </div>

    <Card>
      <CardHeader class="pb-2">
        <CardTitle>Player Tracking</CardTitle>
        <p class="text-sm text-muted-foreground">
          Refreshes automatically every 30 seconds while this tab is visible.
          The server only remembers leavers for a few minutes.
        </p>
      </CardHeader>
      <CardContent>
        <div class="mb-4 flex items-center space-x-2">
          <Input
            v-model="searchQuery"
            placeholder="Search by name, Steam ID, or EOS ID..."
            class="flex-grow"
          />
        </div>

        <DataTable
          :columns="columns"
          :rows="filteredPlayers"
          :row-key="(p) => p.steam_id || p.eosId || String(p.playerId)"
          :loading="loading && disconnectedPlayers.length === 0"
          :empty-text="
            disconnectedPlayers.length === 0
              ? 'No disconnected players found'
              : 'No players match your search'
          "
        >
          <template #cell-name="{ row }">
            <button
              class="text-left font-medium hover:underline"
              @click.stop="openPanel(row)"
            >
              {{ row.name }}
            </button>
          </template>
          <template #cell-ids="{ row }">
            <div class="flex flex-col gap-0.5">
              <button
                v-if="row.steam_id"
                class="group inline-flex items-center gap-1 text-left text-xs text-muted-foreground hover:text-foreground"
                title="Copy Steam ID"
                @click.stop="handleCopy(row.steam_id)"
              >
                Steam: {{ row.steam_id }}
                <Icon
                  name="lucide:copy"
                  class="h-3 w-3 opacity-0 group-hover:opacity-100"
                />
              </button>
              <button
                v-if="row.eosId"
                class="group inline-flex items-center gap-1 text-left text-xs text-muted-foreground hover:text-foreground"
                title="Copy EOS ID"
                @click.stop="handleCopy(row.eosId)"
              >
                EOS: {{ row.eosId }}
                <Icon
                  name="lucide:copy"
                  class="h-3 w-3 opacity-0 group-hover:opacity-100"
                />
              </button>
            </div>
          </template>
          <template #cell-sinceDisconnect="{ row }">
            <span
              class="inline-flex items-center rounded-full bg-amber-500/10 px-2 py-1 text-xs font-medium text-amber-600 ring-1 ring-inset ring-amber-500/20 dark:text-amber-400"
            >
              {{ formatDisconnectTime(row.sinceDisconnect) }}
            </span>
          </template>
          <template #actions="{ row }">
            <PlayerActionMenu
              :player="row"
              :serverId="serverId as string"
              @action-completed="refreshAfterAction"
            />
          </template>
        </DataTable>
      </CardContent>
    </Card>

    <PlayerContextPanel
      v-model:open="panelOpen"
      :player="panelPlayer"
      :server-id="serverId as string"
      @action-completed="refreshAfterAction"
    />
  </div>
</template>
