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

    const { data, error: fetchError } = await useFetch(
      `${runtimeConfig.public.backendApi}/servers/${serverId}/metrics/history?period=${selectedPeriod.value}`,
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

// Lifecycle
onMounted(() => {
  fetchMetrics();
});
</script>
