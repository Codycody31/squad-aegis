<script setup lang="ts">
import { ref, onMounted } from "vue";
import { Button } from "~/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "~/components/ui/card";
import { Badge } from "~/components/ui/badge";
import { toast } from "~/components/ui/toast";
import { useAuthStore } from "~/stores/auth";
import { ArrowLeft, BarChart3, TrendingUp, Activity, Clock } from "lucide-vue-next";

definePageMeta({
  middleware: ["auth"],
});

const route = useRoute();
const router = useRouter();
const serverId = route.params.serverId;
const pluginId = route.params.pluginId;
const authStore = useAuthStore();

// State variables
const loading = ref(true);
const plugin = ref<any>(null);
const metrics = ref<any>({});

// Load plugin details
const loadPlugin = async () => {
  try {
    const response = await $fetch(`/api/servers/${serverId}/plugins/${pluginId}`, {
      headers: {
        Authorization: `Bearer ${authStore.token}`,
      },
    });
    plugin.value = response.data.plugin;
  } catch (error: any) {
    console.error("Failed to load plugin:", error);
    toast({
      title: "Error",
      description: "Failed to load plugin details",
      variant: "destructive",
    });
  }
};

// Load plugin metrics
const loadMetrics = async () => {
  try {
    const response = await $fetch(`/api/servers/${serverId}/plugins/${pluginId}/metrics`, {
      headers: {
        Authorization: `Bearer ${authStore.token}`,
      },
    });
    metrics.value = response.data.metrics || {};
  } catch (error: any) {
    console.error("Failed to load metrics:", error);
    toast({
      title: "Error",
      description: "Failed to load plugin metrics",
      variant: "destructive",
    });
  }
};

// Go back to plugins list
const goBack = () => {
  router.push(`/servers/${serverId}/plugins`);
};

onMounted(async () => {
  loading.value = true;
  try {
    await Promise.all([loadPlugin(), loadMetrics()]);
  } finally {
    loading.value = false;
  }
});
</script>

<template>
  <div class="container mx-auto py-6">
    <div class="flex items-center space-x-4 mb-6">
      <Button variant="outline" @click="goBack">
        <ArrowLeft class="w-4 h-4 mr-2" />
        Back to Plugins
      </Button>
      <div>
        <h1 class="text-3xl font-bold">Plugin Metrics</h1>
        <p class="text-muted-foreground">
          Performance and usage metrics for {{ plugin?.name || 'Plugin' }}
        </p>
      </div>
    </div>

    <div v-if="loading" class="flex items-center justify-center py-12">
      <div class="animate-spin rounded-full h-8 w-8 border-b-2 border-primary"></div>
    </div>

    <div v-else class="space-y-6">
      <!-- Plugin Info Card -->
      <Card v-if="plugin">
        <CardHeader>
          <CardTitle class="flex items-center space-x-2">
            <BarChart3 class="w-5 h-5" />
            <span>{{ plugin.name }}</span>
          </CardTitle>
          <CardDescription>
            {{ plugin.plugin_id }} • Status: 
            <Badge :class="plugin.status === 'running' ? 'bg-green-100 text-green-800' : 'bg-gray-100 text-gray-800'">
              {{ plugin.status }}
            </Badge>
          </CardDescription>
        </CardHeader>
      </Card>

      <!-- Metrics Overview -->
      <div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-6">
        <Card>
          <CardHeader class="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle class="text-sm font-medium">Events Processed</CardTitle>
            <Activity class="h-4 w-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            <div class="text-2xl font-bold">{{ metrics.events_processed || 0 }}</div>
            <p class="text-xs text-muted-foreground">
              Total events handled by this plugin
            </p>
          </CardContent>
        </Card>

        <Card>
          <CardHeader class="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle class="text-sm font-medium">Uptime</CardTitle>
            <Clock class="h-4 w-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            <div class="text-2xl font-bold">{{ metrics.uptime || '0h' }}</div>
            <p class="text-xs text-muted-foreground">
              Time since last restart
            </p>
          </CardContent>
        </Card>

        <Card>
          <CardHeader class="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle class="text-sm font-medium">Error Rate</CardTitle>
            <TrendingUp class="h-4 w-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            <div class="text-2xl font-bold">{{ metrics.error_rate || '0%' }}</div>
            <p class="text-xs text-muted-foreground">
              Errors in the last 24h
            </p>
          </CardContent>
        </Card>

        <Card>
          <CardHeader class="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle class="text-sm font-medium">Memory Usage</CardTitle>
            <BarChart3 class="h-4 w-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            <div class="text-2xl font-bold">{{ metrics.memory_usage || '0 MB' }}</div>
            <p class="text-xs text-muted-foreground">
              Current memory consumption
            </p>
          </CardContent>
        </Card>
      </div>

      <!-- Plugin-Specific Metrics -->
      <Card v-if="plugin?.plugin_id === 'discord_admin_request'">
        <CardHeader>
          <CardTitle>Discord Admin Request Metrics</CardTitle>
          <CardDescription>
            Specific metrics for the Discord Admin Request plugin
          </CardDescription>
        </CardHeader>
        <CardContent>
          <div class="grid grid-cols-1 md:grid-cols-3 gap-4">
            <div class="text-center">
              <div class="text-3xl font-bold text-blue-600">{{ metrics.admin_requests_sent || 0 }}</div>
              <p class="text-sm text-muted-foreground">Admin Requests Sent</p>
            </div>
            <div class="text-center">
              <div class="text-3xl font-bold text-green-600">{{ metrics.discord_messages_sent || 0 }}</div>
              <p class="text-sm text-muted-foreground">Discord Messages Sent</p>
            </div>
            <div class="text-center">
              <div class="text-3xl font-bold text-yellow-600">{{ metrics.ping_cooldowns || 0 }}</div>
              <p class="text-sm text-muted-foreground">Ping Cooldowns Triggered</p>
            </div>
          </div>
        </CardContent>
      </Card>

      <!-- Coming Soon Card -->
      <Card>
        <CardHeader>
          <CardTitle>Advanced Analytics</CardTitle>
          <CardDescription>
            More detailed metrics and charts coming soon
          </CardDescription>
        </CardHeader>
        <CardContent>
          <div class="text-center py-8">
            <BarChart3 class="w-16 h-16 mx-auto text-muted-foreground mb-4" />
            <p class="text-muted-foreground">
              Advanced charts, performance graphs, and historical data will be available here.
            </p>
          </div>
        </CardContent>
      </Card>
    </div>
  </div>
</template>
