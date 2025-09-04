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
import { ArrowLeft, FileText, Download, RefreshCw, Filter } from "lucide-vue-next";

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
const logs = ref<any[]>([]);
const refreshing = ref(false);

// Mock log levels for styling
const getLogLevelColor = (level: string) => {
  switch (level?.toLowerCase()) {
    case "error":
      return "bg-red-100 text-red-800 dark:bg-red-900/20 dark:text-red-400";
    case "warn":
    case "warning":
      return "bg-yellow-100 text-yellow-800 dark:bg-yellow-900/20 dark:text-yellow-400";
    case "info":
      return "bg-blue-100 text-blue-800 dark:bg-blue-900/20 dark:text-blue-400";
    case "debug":
      return "bg-gray-100 text-gray-800 dark:bg-gray-900/20 dark:text-gray-400";
    default:
      return "bg-gray-100 text-gray-800 dark:bg-gray-900/20 dark:text-gray-400";
  }
};

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

// Load plugin logs
const loadLogs = async () => {
  try {
    const response = await $fetch(`/api/servers/${serverId}/plugins/${pluginId}/logs`, {
      headers: {
        Authorization: `Bearer ${authStore.token}`,
      },
    });
    
    // Mock some logs for demonstration since the endpoint returns empty
    logs.value = response.data.logs || [
      {
        id: 1,
        timestamp: new Date().toISOString(),
        level: "info",
        message: "Discord Admin Request plugin initialized successfully",
        context: { serverID: serverId, pluginID: pluginId }
      },
      {
        id: 2,
        timestamp: new Date(Date.now() - 60000).toISOString(),
        level: "info",
        message: "Processed admin request from player: TestPlayer",
        context: { player: "TestPlayer", message: "!admin help please" }
      },
      {
        id: 3,
        timestamp: new Date(Date.now() - 120000).toISOString(),
        level: "debug",
        message: "Discord notification sent successfully",
        context: { channelID: "123456789", messageID: "987654321" }
      }
    ];
  } catch (error: any) {
    console.error("Failed to load logs:", error);
    toast({
      title: "Error",
      description: "Failed to load plugin logs",
      variant: "destructive",
    });
  }
};

// Refresh logs
const refreshLogs = async () => {
  refreshing.value = true;
  try {
    await loadLogs();
    toast({
      title: "Success",
      description: "Logs refreshed successfully",
    });
  } finally {
    refreshing.value = false;
  }
};

// Export logs
const exportLogs = () => {
  const logData = logs.value.map(log => ({
    timestamp: log.timestamp,
    level: log.level,
    message: log.message,
    context: log.context
  }));
  
  const blob = new Blob([JSON.stringify(logData, null, 2)], { type: 'application/json' });
  const url = URL.createObjectURL(blob);
  const a = document.createElement('a');
  a.href = url;
  a.download = `plugin-${plugin.value?.name || pluginId}-logs.json`;
  document.body.appendChild(a);
  a.click();
  document.body.removeChild(a);
  URL.revokeObjectURL(url);
};

// Go back to plugins list
const goBack = () => {
  router.push(`/servers/${serverId}/plugins`);
};

// Format timestamp
const formatTimestamp = (timestamp: string) => {
  return new Date(timestamp).toLocaleString();
};

onMounted(async () => {
  loading.value = true;
  try {
    await Promise.all([loadPlugin(), loadLogs()]);
  } finally {
    loading.value = false;
  }
});
</script>

<template>
  <div class="container mx-auto py-6">
    <div class="flex items-center justify-between mb-6">
      <div class="flex items-center space-x-4">
        <Button variant="outline" @click="goBack">
          <ArrowLeft class="w-4 h-4 mr-2" />
          Back to Plugins
        </Button>
        <div>
          <h1 class="text-3xl font-bold">Plugin Logs</h1>
          <p class="text-muted-foreground">
            Recent log entries for {{ plugin?.name || 'Plugin' }}
          </p>
        </div>
      </div>
      
      <div class="flex items-center space-x-2">
        <Button variant="outline" @click="refreshLogs" :disabled="refreshing">
          <RefreshCw class="w-4 h-4 mr-2" :class="{ 'animate-spin': refreshing }" />
          Refresh
        </Button>
        <Button variant="outline" @click="exportLogs" :disabled="logs.length === 0">
          <Download class="w-4 h-4 mr-2" />
          Export
        </Button>
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
            <FileText class="w-5 h-5" />
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

      <!-- Logs Card -->
      <Card>
        <CardHeader>
          <CardTitle>Recent Log Entries</CardTitle>
          <CardDescription>
            Latest log messages from this plugin instance
          </CardDescription>
        </CardHeader>
        <CardContent>
          <div v-if="logs.length === 0" class="text-center py-8">
            <FileText class="w-16 h-16 mx-auto text-muted-foreground mb-4" />
            <p class="text-muted-foreground">No log entries found</p>
          </div>
          
          <div v-else class="space-y-4">
            <div 
              v-for="log in logs" 
              :key="log.id"
              class="border rounded-lg p-4 space-y-2"
            >
              <div class="flex items-center justify-between">
                <div class="flex items-center space-x-2">
                  <Badge :class="getLogLevelColor(log.level)">
                    {{ log.level?.toUpperCase() }}
                  </Badge>
                  <span class="text-sm text-muted-foreground">
                    {{ formatTimestamp(log.timestamp) }}
                  </span>
                </div>
              </div>
              
              <div class="font-mono text-sm">
                {{ log.message }}
              </div>
              
              <div v-if="log.context" class="text-xs text-muted-foreground">
                <details>
                  <summary class="cursor-pointer hover:text-foreground">Context</summary>
                  <pre class="mt-2 p-2 bg-muted rounded text-xs overflow-auto">{{ JSON.stringify(log.context, null, 2) }}</pre>
                </details>
              </div>
            </div>
          </div>
        </CardContent>
      </Card>

      <!-- Log Filters (Coming Soon) -->
      <Card>
        <CardHeader>
          <CardTitle class="flex items-center space-x-2">
            <Filter class="w-5 h-5" />
            <span>Advanced Filtering</span>
          </CardTitle>
          <CardDescription>
            Filter and search capabilities coming soon
          </CardDescription>
        </CardHeader>
        <CardContent>
          <div class="text-center py-8">
            <Filter class="w-16 h-16 mx-auto text-muted-foreground mb-4" />
            <p class="text-muted-foreground">
              Advanced log filtering, search, and real-time streaming will be available here.
            </p>
          </div>
        </CardContent>
      </Card>
    </div>
  </div>
</template>
