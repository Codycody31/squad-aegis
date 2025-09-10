<template>
  <div class="p-6 space-y-6">
    <div class="flex items-center justify-between">
      <div>
        <h1 class="text-3xl font-bold tracking-tight">Server Metrics</h1>
        <p class="text-muted-foreground">
          Performance metrics, player statistics, and server analytics
        </p>
      </div>
      <div class="flex items-center space-x-2">
        <Select
          :model-value="selectedPeriod"
          @update:model-value="(value: AcceptableValue) => { if (typeof value === 'string') { selectedPeriod = value; fetchMetrics(); } }"
          class="px-3 py-2 border rounded-md"
        >
          <SelectTrigger>
            <SelectValue placeholder="Select a period" />
          </SelectTrigger>
          <SelectContent>
            <SelectGroup>
              <SelectItem value="1h">Last 1 Hour</SelectItem>
              <SelectItem value="6h">Last 6 Hours</SelectItem>
              <SelectItem value="24h">Last 24 Hours</SelectItem>
              <SelectItem value="7d">Last 7 Days</SelectItem>
              <SelectItem value="30d">Last 30 Days</SelectItem>
            </SelectGroup>
          </SelectContent>
        </Select>
        <Button variant="outline" size="sm" @click="fetchMetrics">
          <Icon name="mdi:refresh" class="h-4 w-4 mr-2" />
          Refresh
        </Button>
      </div>
    </div>

    <!-- Loading State -->
    <div v-if="loading" class="flex items-center justify-center py-12">
      <div class="text-center">
        <Icon name="mdi:loading" class="h-8 w-8 animate-spin mx-auto mb-4" />
        <p class="text-muted-foreground">Loading metrics...</p>
      </div>
    </div>

    <!-- Error State -->
    <Card v-if="error" class="border-red-200 bg-red-50">
      <CardContent class="p-4">
        <div class="flex items-center space-x-2">
          <Icon name="mdi:alert-circle" class="h-4 w-4 text-red-600" />
          <div>
            <p class="font-medium text-red-900">Error Loading Metrics</p>
            <p class="text-sm text-red-700">{{ error }}</p>
          </div>
        </div>
      </CardContent>
    </Card>

    <!-- Metrics Content -->
    <div v-if="!loading && !error && metrics" class="space-y-6">
      <!-- Summary Cards -->
      <div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 xl:grid-cols-6 gap-4">
        <Card>
          <CardContent class="p-6">
            <div class="flex items-center space-x-2">
              <Icon name="mdi:account-group" class="h-8 w-8 text-blue-500" />
              <div>
                <p class="text-sm font-medium text-muted-foreground">
                  Peak Players
                </p>
                <p class="text-2xl font-bold">
                  {{ metrics.summary.peak_player_count }}
                </p>
                <p class="text-xs text-muted-foreground">
                  {{ metrics.summary.unique_players_count }} unique players
                </p>
              </div>
            </div>
          </CardContent>
        </Card>

        <Card>
          <CardContent class="p-6">
            <div class="flex items-center space-x-2">
              <Icon name="mdi:speedometer" class="h-8 w-8 text-green-500" />
              <div>
                <p class="text-sm font-medium text-muted-foreground">Avg TPS</p>
                <p class="text-2xl font-bold">
                                  <p class="text-xl font-bold text-foreground">
                  {{ metrics.summary.avg_tick_rate.toFixed(2) }}
                </p>
                </p>
                <p class="text-xs text-muted-foreground">
                  {{ metrics.summary.uptime_percentage.toFixed(1) }}% uptime
                </p>
              </div>
            </div>
          </CardContent>
        </Card>

        <Card>
          <CardContent class="p-6">
            <div class="flex items-center space-x-2">
              <Icon name="mdi:map" class="h-8 w-8 text-purple-500" />
              <div>
                <p class="text-sm font-medium text-muted-foreground">
                  Rounds Played
                </p>
                <p class="text-2xl font-bold">
                  {{ metrics.summary.total_rounds }}
                </p>
                <p class="text-xs text-muted-foreground">
                  Most played: {{ metrics.summary.most_played_map }}
                </p>
              </div>
            </div>
          </CardContent>
        </Card>

        <Card>
          <CardContent class="p-6">
            <div class="flex items-center space-x-2">
              <Icon name="mdi:message-text" class="h-8 w-8 text-orange-500" />
              <div>
                <p class="text-sm font-medium text-muted-foreground">
                  Chat Messages
                </p>
                <p class="text-2xl font-bold">
                  {{ metrics.summary.total_chat_messages }}
                </p>
                <p class="text-xs text-muted-foreground">
                  {{ metrics.summary.total_teamkills }} teamkills
                </p>
              </div>
            </div>
          </CardContent>
        </Card>

        <Card>
          <CardContent class="p-6">
            <div class="flex items-center space-x-2">
              <Icon name="mdi:heart-broken" class="h-8 w-8 text-red-500" />
              <div>
                <p class="text-sm font-medium text-muted-foreground">
                  Player Deaths
                </p>
                <p class="text-2xl font-bold">
                  {{ metrics.summary.total_player_died }}
                </p>
                <p class="text-xs text-muted-foreground">
                  {{ metrics.summary.total_player_wounded }} wounded
                </p>
              </div>
            </div>
          </CardContent>
        </Card>

        <Card>
          <CardContent class="p-6">
            <div class="flex items-center space-x-2">
              <Icon name="mdi:medical-bag" class="h-8 w-8 text-green-600" />
              <div>
                <p class="text-sm font-medium text-muted-foreground">
                  Player Revives
                </p>
                <p class="text-2xl font-bold">
                  {{ metrics.summary.total_player_revived }}
                </p>
                <p class="text-xs text-muted-foreground">
                  {{ metrics.summary.total_player_damaged }} damage events
                </p>
              </div>
            </div>
          </CardContent>
        </Card>
      </div>

      <!-- Charts -->
      <div class="grid grid-cols-1 lg:grid-cols-2 xl:grid-cols-3 gap-6">
        <!-- Player Count Chart -->
        <Card>
          <CardContent class="p-6">
            <h3 class="text-lg font-semibold mb-4">Player Count Over Time</h3>
            <div class="h-64 flex items-center justify-center">
              <PlayerCountChart
                :data="metrics.player_count"
                :period="selectedPeriod"
              />
            </div>
          </CardContent>
        </Card>

        <!-- TPS Chart -->
        <Card>
          <CardContent class="p-6">
            <h3 class="text-lg font-semibold mb-4">Server Performance (TPS)</h3>
            <div class="h-64 flex items-center justify-center">
              <TickRateChart
                :data="metrics.tick_rate"
                :period="selectedPeriod"
              />
            </div>
          </CardContent>
        </Card>

        <!-- Chat Activity Chart -->
        <Card>
          <CardContent class="p-6">
            <h3 class="text-lg font-semibold mb-4">Chat Activity</h3>
            <div class="h-64 flex items-center justify-center">
              <ChatActivityChart
                :data="metrics.chat_activity"
                :period="selectedPeriod"
              />
            </div>
          </CardContent>
        </Card>

        <!-- Connection Stats Chart -->
        <Card>
          <CardContent class="p-6">
            <h3 class="text-lg font-semibold mb-4">Player Connections</h3>
            <div class="h-64 flex items-center justify-center">
              <ConnectionChart
                :data="metrics.connection_stats"
                :period="selectedPeriod"
              />
            </div>
          </CardContent>
        </Card>

        <!-- Player Wounded Chart -->
        <Card>
          <CardContent class="p-6">
            <h3 class="text-lg font-semibold mb-4">Player Wounded Events</h3>
            <div class="h-64 flex items-center justify-center">
              <PlayerWoundedChart
                :data="metrics.player_wounded_stats"
                :period="selectedPeriod"
              />
            </div>
          </CardContent>
        </Card>

        <!-- Player Revived Chart -->
        <Card>
          <CardContent class="p-6">
            <h3 class="text-lg font-semibold mb-4">Player Revived Events</h3>
            <div class="h-64 flex items-center justify-center">
              <PlayerRevivedChart
                :data="metrics.player_revived_stats"
                :period="selectedPeriod"
              />
            </div>
          </CardContent>
        </Card>

        <!-- Player Died Chart -->
        <Card>
          <CardContent class="p-6">
            <h3 class="text-lg font-semibold mb-4">Player Death Events</h3>
            <div class="h-64 flex items-center justify-center">
              <PlayerDiedChart
                :data="metrics.player_died_stats"
                :period="selectedPeriod"
              />
            </div>
          </CardContent>
        </Card>

        <!-- Player Damaged Chart -->
        <Card>
          <CardContent class="p-6">
            <h3 class="text-lg font-semibold mb-4">Player Damage Events</h3>
            <div class="h-64 flex items-center justify-center">
              <PlayerDamagedChart
                :data="metrics.player_damaged_stats"
                :period="selectedPeriod"
              />
            </div>
          </CardContent>
        </Card>

        <!-- Player Possess Chart -->
        <Card>
          <CardContent class="p-6">
            <h3 class="text-lg font-semibold mb-4">Player Possession Events</h3>
            <div class="h-64 flex items-center justify-center">
              <PlayerPossessChart
                :data="metrics.player_possess_stats"
                :period="selectedPeriod"
              />
            </div>
          </CardContent>
        </Card>

        <!-- Deployable Damaged Chart -->
        <Card>
          <CardContent class="p-6">
            <h3 class="text-lg font-semibold mb-4">Deployable Damage Events</h3>
            <div class="h-64 flex items-center justify-center">
              <DeployableDamagedChart
                :data="metrics.deployable_damaged_stats"
                :period="selectedPeriod"
              />
            </div>
          </CardContent>
        </Card>

        <!-- Admin Broadcast Chart -->
        <Card>
          <CardContent class="p-6">
            <h3 class="text-lg font-semibold mb-4">Admin Broadcasts</h3>
            <div class="h-64 flex items-center justify-center">
              <AdminBroadcastChart
                :data="metrics.admin_broadcast_stats"
                :period="selectedPeriod"
              />
            </div>
          </CardContent>
        </Card>
      </div>

      <!-- Maps and Rounds -->
      <div class="grid grid-cols-1 lg:grid-cols-2 gap-6">
        <!-- Recent Maps -->
        <Card>
          <CardContent class="p-6">
            <h3 class="text-lg font-semibold mb-4">Recent Maps</h3>
            <div class="space-y-3">
              <div
                v-for="(map, index) in recentMaps"
                :key="index"
                class="flex items-center justify-between p-3 rounded-lg bg-muted/50"
              >
                <div class="flex items-center space-x-3">
                  <Icon name="mdi:map" class="h-5 w-5 text-purple-500" />
                  <div>
                    <p class="font-medium">{{ map.name }}</p>
                    <p class="text-sm text-muted-foreground">
                      {{ formatTimestamp(map.timestamp) }}
                    </p>
                  </div>
                </div>
                <Badge variant="secondary">{{ map.duration }}</Badge>
              </div>
            </div>
          </CardContent>
        </Card>

        <!-- Server Statistics -->
        <Card>
          <CardContent class="p-6">
            <h3 class="text-lg font-semibold mb-4">Server Statistics</h3>
            <div class="space-y-4">
              <div class="flex items-center justify-between">
                <span class="text-sm font-medium">Total Connections</span>
                <span class="text-sm">{{
                  metrics.summary.total_connections
                }}</span>
              </div>
              <div class="flex items-center justify-between">
                <span class="text-sm font-medium">Unique Players</span>
                <span class="text-sm">{{
                  metrics.summary.unique_players_count
                }}</span>
              </div>
              <div class="flex items-center justify-between">
                <span class="text-sm font-medium">Chat Messages</span>
                <span class="text-sm">{{
                  metrics.summary.total_chat_messages
                }}</span>
              </div>
              <div class="flex items-center justify-between">
                <span class="text-sm font-medium">Teamkills</span>
                <span class="text-sm text-red-600">{{
                  metrics.summary.total_teamkills
                }}</span>
              </div>
              <div class="flex items-center justify-between">
                <span class="text-sm font-medium">Server Uptime</span>
                <span class="text-sm"
                  >{{ metrics.summary.uptime_percentage.toFixed(1) }}%</span
                >
              </div>
              <div class="flex items-center justify-between">
                <span class="text-sm font-medium">Average TPS</span>
                <span class="text-sm">{{
                  metrics.summary.avg_tick_rate.toFixed(2)
                }}</span>
              </div>
              <div class="flex items-center justify-between">
                <span class="text-sm font-medium">Player Deaths</span>
                <span class="text-sm text-red-600">{{
                  metrics.summary.total_player_died
                }}</span>
              </div>
              <div class="flex items-center justify-between">
                <span class="text-sm font-medium">Player Wounded</span>
                <span class="text-sm text-orange-600">{{
                  metrics.summary.total_player_wounded
                }}</span>
              </div>
              <div class="flex items-center justify-between">
                <span class="text-sm font-medium">Player Revived</span>
                <span class="text-sm text-green-600">{{
                  metrics.summary.total_player_revived
                }}</span>
              </div>
              <div class="flex items-center justify-between">
                <span class="text-sm font-medium">Admin Broadcasts</span>
                <span class="text-sm">{{
                  metrics.summary.total_admin_broadcasts
                }}</span>
              </div>
              <div class="flex items-center justify-between">
                <span class="text-sm font-medium">Plugin Logs</span>
                <span class="text-sm">{{
                  metrics.summary.total_plugin_logs
                }}</span>
              </div>
            </div>
          </CardContent>
        </Card>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, computed } from "vue";
import { Card, CardContent } from "~/components/ui/card";
import { Button } from "~/components/ui/button";
import { Badge } from "~/components/ui/badge";
import { Icon } from "#components";
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
} from "~/components/ui/select";

// Simple chart components (placeholder implementations)
import PlayerCountChart from "~/components/charts/PlayerCountChart.vue";
import TickRateChart from "~/components/charts/TickRateChart.vue";
import ChatActivityChart from "~/components/charts/ChatActivityChart.vue";
import ConnectionChart from "~/components/charts/ConnectionChart.vue";
import PlayerWoundedChart from "~/components/charts/PlayerWoundedChart.vue";
import PlayerRevivedChart from "~/components/charts/PlayerRevivedChart.vue";
import PlayerPossessChart from "~/components/charts/PlayerPossessChart.vue";
import PlayerDiedChart from "~/components/charts/PlayerDiedChart.vue";
import PlayerDamagedChart from "~/components/charts/PlayerDamagedChart.vue";
import DeployableDamagedChart from "~/components/charts/DeployableDamagedChart.vue";
import AdminBroadcastChart from "~/components/charts/AdminBroadcastChart.vue";
import PluginLogRateChart from "~/components/charts/PluginLogRateChart.vue";
import type { AcceptableValue } from "reka-ui";

definePageMeta({
  middleware: "auth",
});

const route = useRoute();
const serverId = route.params.serverId as string;

// Reactive state
const loading = ref(false);
const error = ref<string | null>(null);
const metrics = ref<any>(null);
const selectedPeriod = ref("24h");

// Computed data
const recentMaps = computed(() => {
  if (!metrics.value?.maps) return [];

  return metrics.value.maps
    .slice(-5)
    .reverse()
    .map((map: any, index: number) => ({
      name: map.value,
      timestamp: map.timestamp,
      duration: `${Math.floor(Math.random() * 60 + 30)}min`, // Sample duration
    }));
});

// Fetch metrics data
const fetchMetrics = async () => {
  loading.value = true;
  error.value = null;

  try {
    const runtimeConfig = useRuntimeConfig();
    const cookieToken = useCookie(
      runtimeConfig.public.sessionCookieName as string
    );
    const token = cookieToken.value;

    if (!token) {
      throw new Error("Authentication required");
    }

    // Calculate appropriate interval based on period for higher fidelity
    let interval = 15; // default
    switch (selectedPeriod.value) {
      case "1h":
        interval = 1; // 1 minute intervals for 1 hour
        break;
      case "6h":
        interval = 5; // 5 minute intervals for 6 hours
        break;
      case "24h":
        interval = 15; // 15 minute intervals for 24 hours
        break;
      case "7d":
        interval = 120; // 2 hour intervals for 7 days
        break;
      case "30d":
        interval = 720; // 12 hour intervals for 30 days
        break;
    }

    const { data, error: fetchError } = await useFetch(
      `${runtimeConfig.public.backendApi}/servers/${serverId}/metrics/history?period=${selectedPeriod.value}&interval=${interval}`,
      {
        method: "GET",
        headers: {
          Authorization: `Bearer ${token}`,
        },
      }
    );

    if (fetchError.value) {
      throw new Error(fetchError.value.message || "Failed to fetch metrics");
    }

    if (data.value) {
      metrics.value = (data.value as any).data?.metrics || data.value;
    }
  } catch (err: any) {
    error.value = err.message || "An error occurred while fetching metrics";
    console.error(err);
  } finally {
    loading.value = false;
  }
};

// Helper functions
const formatTimestamp = (timestamp: string) => {
  return new Date(timestamp).toLocaleString();
};

// Lifecycle
onMounted(() => {
  fetchMetrics();
});
</script>
