<script setup lang="ts">
import { ref, onMounted, onUnmounted, computed } from "vue";
import { Button } from "~/components/ui/button";
import { Input } from "~/components/ui/input";
import { Card, CardContent, CardHeader, CardTitle } from "~/components/ui/card";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "~/components/ui/table";
import { useToast } from "~/components/ui/toast";
import type { Player } from "~/types";
import PlayerActionMenu from "~/components/PlayerActionMenu.vue";

const authStore = useAuthStore();
const route = useRoute();
const serverId = route.params.serverId;
const { toast } = useToast();

const loading = ref(true);
const error = ref<string | null>(null);
const disconnectedPlayers = ref<Player[]>([]);
const refreshInterval = ref<NodeJS.Timeout | null>(null);
const searchQuery = ref("");
const copiedId = ref<string | null>(null);

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
  return disconnectedPlayers.value.filter(player => 
    player.name.toLowerCase().includes(query) || 
    player.steam_id.includes(query) ||
    player.eosId.toLowerCase().includes(query)
  );
});

// Function to fetch disconnected players data
async function fetchDisconnectedPlayers() {
  loading.value = true;
  error.value = null;

  try {
    const { data, error: fetchError } = await useAuthFetch<PlayersResponse>(
      `/servers/${serverId}/rcon/server-population`,
      {
        method: "GET",
      }
    );

    if (fetchError.value) {
      throw new Error(fetchError.value.message || "Failed to fetch disconnected players data");
    }

    if (data.value && data.value.data && data.value.data.players) {
      disconnectedPlayers.value = data.value.data.players.disconnectedPlayers || [];

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
    error.value = err.message || "An error occurred while fetching disconnected players data";
  } finally {
    loading.value = false;
  }
}

// Refresh data after action
function refreshAfterAction() {
  fetchDisconnectedPlayers();
}

// Format disconnect time to be more readable
function formatDisconnectTime(timeStr: string): string {
  // If already in a readable format, return as is
  if (timeStr.includes("h") || (timeStr.includes("m") && timeStr.includes("s"))) {
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

// Setup auto-refresh
onMounted(() => {
  fetchDisconnectedPlayers();
  
  // Refresh data every 30 seconds
  refreshInterval.value = setInterval(() => {
    fetchDisconnectedPlayers();
  }, 30000);
});

// Clear interval on component unmount
onUnmounted(() => {
  if (refreshInterval.value) {
    clearInterval(refreshInterval.value);
  }
});

// Manual refresh function
function refreshData() {
  fetchDisconnectedPlayers();
}

</script>

<template>
  <div class="p-4">
    <div class="flex justify-between items-center mb-4">
      <h1 class="text-2xl font-bold">Disconnected Players</h1>
      <Button @click="refreshData" :disabled="loading">
        {{ loading ? "Refreshing..." : "Refresh" }}
      </Button>
    </div>

    <div v-if="error" class="bg-red-500 text-white p-4 rounded mb-4">
      {{ error }}
    </div>

    <Card class="mb-4">
      <CardHeader class="pb-2">
        <CardTitle>Player Tracking</CardTitle>
        <p class="text-sm text-muted-foreground">
          View players who have disconnected from the server. Data refreshes automatically every 30 seconds.
        </p>
      </CardHeader>
      <CardContent>
        <div class="flex items-center space-x-2 mb-4">
          <Input 
            v-model="searchQuery" 
            placeholder="Search by name, Steam ID, or EOS ID..." 
            class="flex-grow"
          />
        </div>

        <div v-if="loading && disconnectedPlayers.length === 0" class="text-center py-8">
          <div class="animate-spin h-8 w-8 border-4 border-primary border-t-transparent rounded-full mx-auto mb-4"></div>
          <p>Loading disconnected players...</p>
        </div>

        <div v-else-if="disconnectedPlayers.length === 0" class="text-center py-8">
          <p>No disconnected players found</p>
        </div>

        <div v-else-if="filteredPlayers.length === 0" class="text-center py-8">
          <p>No players match your search</p>
        </div>

        <div v-else class="overflow-x-auto">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Name</TableHead>
                <TableHead>Steam ID</TableHead>
                <TableHead>Disconnected For</TableHead>
                <TableHead class="text-right">Actions</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              <TableRow 
                v-for="player in filteredPlayers" 
                :key="player.playerId"
                class="hover:bg-muted/50"
              >
                <TableCell class="font-medium">{{ player.name }}</TableCell>
                <TableCell>
                  <div class="flex flex-col">
                    <span class="text-xs text-muted-foreground">Steam: {{ player.steam_id }}</span>
                    <span class="text-xs text-muted-foreground">EOS: {{ player.eosId }}</span>
                  </div>
                </TableCell>
                <TableCell>
                  <span class="inline-flex items-center rounded-full bg-amber-50 px-2 py-1 text-xs font-medium text-amber-700 ring-1 ring-inset ring-amber-600/20">
                    {{ formatDisconnectTime(player.sinceDisconnect) }}
                  </span>
                </TableCell>
                <TableCell class="text-right">
                  <PlayerActionMenu :player="player" :serverId="serverId as string" @action-completed="refreshAfterAction" />
                </TableCell>
              </TableRow>
            </TableBody>
          </Table>
        </div>
      </CardContent>
    </Card>

    <Card>
      <CardHeader>
        <CardTitle>About Disconnected Players</CardTitle>
      </CardHeader>
      <CardContent>
        <p class="text-sm text-muted-foreground">
          This page shows players who have recently disconnected from the server. The server keeps track of disconnected players for a limited time, typically around 5-10 minutes depending on server settings.
        </p>
        <p class="text-sm text-muted-foreground mt-2">
          Players who have been disconnected for longer periods will no longer appear in this list. The "Disconnected For" time shows how long ago the player left the server.
        </p>
        <p class="text-sm text-muted-foreground mt-2">
          You can ban disconnected players using the actions menu. This is useful for players who disconnect to avoid punishment.
        </p>
      </CardContent>
    </Card>
  </div>
</template>

<style scoped>
/* Add any page-specific styles here */
</style>
