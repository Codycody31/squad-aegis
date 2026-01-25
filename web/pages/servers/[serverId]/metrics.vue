<template>
  <div class="p-6 space-y-6">
    <div class="flex items-center justify-between">
      <div>
        <h1 class="text-3xl font-bold tracking-tight">Server Metrics</h1>
        <p class="text-muted-foreground">
          Performance metrics, player statistics, and server analytics
        </p>
      </div>
      <div class="flex flex-col sm:flex-row items-start sm:items-center gap-2 sm:space-x-2">
        <div class="flex items-center space-x-2">
          <Select
            v-if="!useCustomRange"
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
          <DateRangePicker
            v-if="useCustomRange"
            v-model="customDateRange"
            @update:model-value="(value) => { customDateRange = value; if (value?.start && value?.end) { fetchMetrics(); } }"
          />
          <Button
            variant="ghost"
            size="sm"
            @click="useCustomRange = !useCustomRange"
            :title="useCustomRange ? 'Switch to period selector' : 'Switch to custom date range'"
          >
            <Icon
              :name="useCustomRange ? 'mdi:calendar-clock' : 'mdi:calendar-range'"
              class="h-4 w-4"
            />
          </Button>
        </div>
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
                  {{ metrics.summary.peak_player_count || 0 }}
                </p>
                <p class="text-xs text-muted-foreground">
                  Current: {{ metrics.summary.total_players || 0 }}
                </p>
              </div>
            </div>
          </CardContent>
        </Card>        <Card>
          <CardContent class="p-6">
            <div class="flex items-center space-x-2">
              <Icon name="mdi:speedometer" class="h-8 w-8 text-green-500" />
              <div>
                <p class="text-sm font-medium text-muted-foreground">Avg TPS</p>
                <p class="text-2xl font-bold">
                  {{ metrics.summary.avg_tick_rate?.toFixed(1) || '0.0' }}
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
                  Total Rounds
                </p>
                <p class="text-2xl font-bold">
                  {{ metrics.summary.total_rounds || 0 }}
                </p>
                <p class="text-xs text-muted-foreground">
                  Most Played: {{ metrics.summary.most_played_map || 'N/A' }}
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
                  {{ metrics.summary.total_chat_messages || 0 }}
                </p>
                <p class="text-xs text-muted-foreground">
                  Total communications
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
                  Teamkills
                </p>
                <p class="text-2xl font-bold">
                  {{ metrics.summary.total_teamkills || 0 }}
                </p>
                <p class="text-xs text-muted-foreground">
                  Friendly fire incidents
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
      <div class="grid grid-cols-1 lg:grid-cols-2 gap-6">
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
        <!-- Recent Maps -->
        <Card>
          <CardContent class="p-6">
            <h3 class="text-lg font-semibold mb-4">Recent Games (Last 10)</h3>
            <div class="space-y-3">
              <div
                v-for="(map, index) in recentMaps"
                :key="index"
                class="flex items-center justify-between p-3 rounded-lg bg-muted/50"
              >
                <div class="flex items-center space-x-3">
                  <Icon name="mdi:map" class="h-4 w-4 text-muted-foreground" />
                  <div class="flex-1">
                    <span class="font-medium">{{ map.name }}</span>
                    <p class="text-xs text-muted-foreground">
                      {{ formatTimestamp(map.timestamp) }}
                    </p>
                  </div>
                  <div class="text-right">
                    <Badge 
                      :variant="map.winner !== 'Unknown' ? 'default' : 'secondary'"
                      class="text-xs"
                    >
                      {{ map.winner }}
                    </Badge>
                  </div>
                </div>
              </div>
              <div v-if="recentMaps.length === 0" class="text-center py-4">
                <p class="text-muted-foreground">No recent games found</p>
              </div>
            </div>
          </CardContent>
        </Card>        <!-- Server Statistics -->
        <Card>
          <CardContent class="p-6">
            <h3 class="text-lg font-semibold mb-4">Server Statistics</h3>
            <div class="space-y-4">
              <div class="flex items-center justify-between">
                <span class="text-sm font-medium">Total Connections</span>
                <span class="text-sm">{{
                  metrics.summary.total_connections || 0
                }}</span>
              </div>
              <div class="flex items-center justify-between">
                <span class="text-sm font-medium">Unique Players</span>
                <span class="text-sm">{{
                  metrics.summary.unique_players_count || 0
                }}</span>
              </div>
              <div class="flex items-center justify-between">
                <span class="text-sm font-medium">Chat Messages</span>
                <span class="text-sm">{{
                  metrics.summary.total_chat_messages || 0
                }}</span>
              </div>
              <div class="flex items-center justify-between">
                <span class="text-sm font-medium">Teamkills</span>
                <span class="text-sm text-red-600">{{
                  metrics.summary.total_teamkills || 0
                }}</span>
              </div>
              <div class="flex items-center justify-between">
                <span class="text-sm font-medium">Average TPS</span>
                <span class="text-sm">{{
                  metrics.summary.avg_tick_rate?.toFixed(2) || '0.00'
                }}</span>
              </div>
              <div class="flex items-center justify-between">
                <span class="text-sm font-medium">Player Deaths</span>
                <span class="text-sm text-red-600">{{
                  metrics.summary.total_player_died || 0
                }}</span>
              </div>
              <div class="flex items-center justify-between">
                <span class="text-sm font-medium">Player Wounded</span>
                <span class="text-sm text-orange-600">{{
                  metrics.summary.total_player_wounded || 0
                }}</span>
              </div>
              <div class="flex items-center justify-between">
                <span class="text-sm font-medium">Player Revived</span>
                <span class="text-sm text-green-600">{{
                  metrics.summary.total_player_revived || 0
                }}</span>
              </div>
              <div class="flex items-center justify-between">
                <span class="text-sm font-medium">Admin Broadcasts</span>
                <span class="text-sm">{{
                  metrics.summary.total_admin_broadcasts || 0
                }}</span>
              </div>
              <div class="flex items-center justify-between">
                <span class="text-sm font-medium">Total Rounds</span>
                <span class="text-sm text-blue-600">{{
                  metrics.summary.total_rounds || 0
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
import { DateRangePicker } from "~/components/ui/date-range-picker";

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
const useCustomRange = ref(false);
const customDateRange = ref<{ start: Date | null; end: Date | null } | null>(null);

// Computed data
const recentMaps = computed(() => {
  if (!metrics.value?.maps) return [];

  return metrics.value.maps
    .slice(-10) // Get last 10 games instead of 5
    .reverse()
    .map((map: any) => {
      // Handle new format with layer and winner data
      if (typeof map.value === 'object' && map.value !== null) {
        return {
          name: map.value.layer || 'Unknown',
          winner: map.value.winner || 'Unknown',
          timestamp: map.timestamp,
        };
      }
      // Handle legacy format
      return {
        name: map.value || 'Unknown',
        winner: 'Unknown',
        timestamp: map.timestamp,
      };
    });
});

// Fetch metrics data
const fetchMetrics = async () => {
  loading.value = true;
  error.value = null;

  try {
    const runtimeConfig = useRuntimeConfig();

    // Build query parameters
    let url = `${runtimeConfig.public.backendApi}/servers/${serverId}/metrics/history`;
    const params = new URLSearchParams();

    if (useCustomRange.value && customDateRange.value?.start && customDateRange.value?.end) {
      // Use custom date range
      const startTime = customDateRange.value.start.toISOString();
      const endTime = customDateRange.value.end.toISOString();
      params.append("startTime", startTime);
      params.append("endTime", endTime);

      // Calculate interval based on date range duration
      const daysDiff = (customDateRange.value.end.getTime() - customDateRange.value.start.getTime()) / (1000 * 60 * 60 * 24);
      let interval = 15; // default
      if (daysDiff <= 1) {
        interval = 1; // 1 minute intervals for <= 1 day
      } else if (daysDiff <= 7) {
        interval = 5; // 5 minute intervals for <= 7 days
      } else if (daysDiff <= 30) {
        interval = 15; // 15 minute intervals for <= 30 days
      } else if (daysDiff <= 60) {
        interval = 120; // 2 hour intervals for <= 60 days
      } else {
        interval = 720; // 12 hour intervals for > 60 days
      }
      params.append("interval", interval.toString());
    } else {
      // Use period selector
      params.append("period", selectedPeriod.value);

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
      params.append("interval", interval.toString());
    }

    url += `?${params.toString()}`;

    const { data, error: fetchError } = await useAuthFetch(
      url,
      {
        method: "GET",
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
