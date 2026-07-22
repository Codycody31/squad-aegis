<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from "vue";
import { Button } from "~/components/ui/button";
import { Badge } from "~/components/ui/badge";
import { Card, CardContent, CardHeader, CardTitle } from "~/components/ui/card";
import PageHeader from "~/components/layout/PageHeader.vue";
import StatCard from "~/components/StatCard.vue";
import DataTable from "~/components/data-table/DataTable.vue";
import type { DataTableColumn } from "~/components/data-table/types";

useHead({
  title: "Dashboard",
});

definePageMeta({
  middleware: "auth",
});

interface SummaryPlayers {
  current: number;
  max: number;
  queue: number;
}

interface ServerSummary {
  id: string;
  name: string;
  ip_address: string;
  game_port: number;
  rcon_port: number;
  game_online: boolean;
  rcon_online: boolean;
  players?: SummaryPlayers;
  current_layer?: string;
  ban_count: number;
}

const runtimeConfig = useRuntimeConfig();
const loading = ref(true);
const servers = ref<ServerSummary[]>([]);
const refreshInterval = ref<NodeJS.Timeout | null>(null);

async function fetchSummary(background = false) {
  if (!background) loading.value = true;
  try {
    const data = await useAuthFetchImperative<{
      data?: { servers: ServerSummary[] };
    }>(`${runtimeConfig.public.backendApi}/servers/summary`);
    servers.value = data.data?.servers ?? [];
  } catch (err) {
    console.error("Failed to fetch server summary:", err);
  } finally {
    loading.value = false;
  }
}

const onlineServers = computed(
  () => servers.value.filter((s) => s.game_online && s.rcon_online).length
);
const totalPlayers = computed(() =>
  servers.value.reduce((sum, s) => sum + (s.players?.current ?? 0), 0)
);
const totalBans = computed(() =>
  servers.value.reduce((sum, s) => sum + s.ban_count, 0)
);
const systemHealthy = computed(
  () =>
    servers.value.length > 0 &&
    servers.value.every((s) => s.game_online && s.rcon_online)
);

const columns: DataTableColumn<ServerSummary>[] = [
  { key: "name", label: "Server", sortable: true },
  { key: "status", label: "Status" },
  {
    key: "players",
    label: "Players",
    sortable: true,
    accessor: (s) => s.players?.current ?? -1,
  },
  { key: "current_layer", label: "Current Layer" },
];

function handleVisibilityChange() {
  if (document.visibilityState === "visible") {
    fetchSummary(true);
    startPolling();
  } else {
    stopPolling();
  }
}

function startPolling() {
  stopPolling();
  refreshInterval.value = setInterval(() => fetchSummary(true), 30000);
}

function stopPolling() {
  if (refreshInterval.value) {
    clearInterval(refreshInterval.value);
    refreshInterval.value = null;
  }
}

onMounted(() => {
  fetchSummary();
  startPolling();
  document.addEventListener("visibilitychange", handleVisibilityChange);
});

onUnmounted(() => {
  stopPolling();
  document.removeEventListener("visibilitychange", handleVisibilityChange);
});
</script>

<template>
  <div class="p-4 md:p-6">
    <PageHeader
      title="Dashboard"
      description="Fleet health across all your Squad servers."
    >
      <template #actions>
        <Button variant="outline" :disabled="loading" @click="fetchSummary()">
          {{ loading ? "Refreshing..." : "Refresh" }}
        </Button>
      </template>
    </PageHeader>

    <div class="mb-4 grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
      <StatCard
        label="Servers"
        :value="`${onlineServers}/${servers.length}`"
        subline="online"
        icon="mdi:server"
        :loading="loading"
      />
      <StatCard
        label="Players Online"
        :value="totalPlayers"
        subline="across all servers"
        icon="mdi:account-group"
        :loading="loading"
      />
      <StatCard
        label="Active Bans"
        :value="totalBans"
        subline="across all servers"
        icon="mdi:ban"
        icon-class="bg-destructive/10 text-destructive"
        :loading="loading"
      />
      <StatCard
        label="System Status"
        :value="systemHealthy ? 'Healthy' : 'Degraded'"
        :subline="
          systemHealthy
            ? 'All systems operational'
            : 'One or more servers unreachable'
        "
        icon="mdi:heart-pulse"
        :icon-class="
          systemHealthy
            ? 'bg-green-500/10 text-green-600 dark:text-green-400'
            : 'bg-destructive/10 text-destructive'
        "
        :loading="loading"
      />
    </div>

    <Card>
      <CardHeader>
        <CardTitle class="text-lg">Servers</CardTitle>
      </CardHeader>
      <CardContent>
        <DataTable
          :columns="columns"
          :rows="servers"
          row-key="id"
          :loading="loading && servers.length === 0"
          empty-text="No servers yet. Add one from the Servers page."
          :row-to="(s) => `/servers/${s.id}`"
        >
          <template #cell-name="{ row }">
            <div>
              <span class="font-medium">{{ row.name }}</span>
              <p class="text-xs text-muted-foreground">
                {{ row.ip_address }}:{{ row.game_port }}
              </p>
            </div>
          </template>
          <template #cell-status="{ row }">
            <div class="flex flex-wrap gap-1">
              <Badge :variant="row.game_online ? 'default' : 'destructive'">
                Game {{ row.game_online ? "online" : "offline" }}
              </Badge>
              <Badge :variant="row.rcon_online ? 'default' : 'destructive'">
                RCON {{ row.rcon_online ? "online" : "offline" }}
              </Badge>
            </div>
          </template>
          <template #cell-players="{ row }">
            <template v-if="row.players">
              {{ row.players.current }}/{{ row.players.max }}
              <span
                v-if="row.players.queue > 0"
                class="text-xs text-muted-foreground"
              >
                +{{ row.players.queue }} queued
              </span>
            </template>
            <span v-else class="text-muted-foreground">—</span>
          </template>
          <template #cell-current_layer="{ row }">
            {{ row.current_layer || "—" }}
          </template>
          <template #actions="{ row }">
            <Button variant="outline" size="sm" as-child>
              <NuxtLink :to="`/servers/${row.id}`">Manage</NuxtLink>
            </Button>
          </template>
        </DataTable>
      </CardContent>
    </Card>
  </div>
</template>
