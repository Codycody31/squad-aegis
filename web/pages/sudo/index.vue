<script setup lang="ts">
import { ref, onMounted } from "vue";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "~/components/ui/card";
import { Badge } from "~/components/ui/badge";
import { Button } from "~/components/ui/button";
import { Progress } from "~/components/ui/progress";
import type { MetricsOverview, SystemHealth } from "~/types";

definePageMeta({
  middleware: "auth",
  layout: "sudo",
});

const runtimeConfig = useRuntimeConfig();
const authStore = useAuthStore();

// Redirect if not superadmin
if (!authStore.user?.super_admin) {
  navigateTo("/dashboard");
}

const loading = ref(true);
const metricsOverview = ref<MetricsOverview | null>(null);
const systemHealth = ref<SystemHealth | null>(null);
const error = ref<string | null>(null);

const fetchData = async () => {
  loading.value = true;
  error.value = null;

  try {
    const [metricsRes, healthRes] = await Promise.all([
      $fetch<any>(`${runtimeConfig.public.backendApi}/sudo/metrics/overview`, {
        headers: { Authorization: `Bearer ${authStore.token}` },
      }),
      $fetch<any>(`${runtimeConfig.public.backendApi}/sudo/system/health`, {
        headers: { Authorization: `Bearer ${authStore.token}` },
      }),
    ]);

    metricsOverview.value = metricsRes.data.data;
    systemHealth.value = healthRes.data.data;
  } catch (err: any) {
    error.value = err.message || "Failed to load dashboard data";
    console.error("Error fetching dashboard data:", err);
  } finally {
    loading.value = false;
  }
};

onMounted(() => {
  fetchData();
});

const getStatusColor = (status: string) => {
  switch (status) {
    case "healthy":
      return "bg-green-500";
    case "degraded":
      return "bg-yellow-500";
    case "unhealthy":
      return "bg-red-500";
    default:
      return "bg-gray-500";
  }
};

const getStatusBadgeVariant = (status: string) => {
  switch (status) {
    case "healthy":
      return "default";
    case "degraded":
      return "secondary";
    case "unhealthy":
      return "destructive";
    default:
      return "outline";
  }
};
</script>

<template>
  <div class="p-6 space-y-6">
    <div>
      <h1 class="text-3xl font-bold">Instance Management</h1>
      <p class="text-muted-foreground">Superadmin dashboard and system overview</p>
    </div>

    <div v-if="loading" class="flex items-center justify-center py-12">
      <div class="text-muted-foreground">Loading dashboard...</div>
    </div>

    <div v-else-if="error" class="p-4 bg-destructive/10 border border-destructive rounded-lg">
      <p class="text-destructive">{{ error }}</p>
      <Button @click="fetchData" class="mt-2" size="sm">Retry</Button>
    </div>

    <div v-else class="space-y-6">
      <!-- System Health Overview -->
      <div>
        <h2 class="text-xl font-semibold mb-4">System Health</h2>
        <div class="grid gap-4 md:grid-cols-2 lg:grid-cols-4">
          <Card v-if="systemHealth">
            <CardHeader class="flex flex-row items-center justify-between space-y-0 pb-2">
              <CardTitle class="text-sm font-medium">PostgreSQL</CardTitle>
              <Badge :variant="getStatusBadgeVariant(systemHealth.postgresql.status)">
                {{ systemHealth.postgresql.status }}
              </Badge>
            </CardHeader>
            <CardContent>
              <div class="text-2xl font-bold">{{ systemHealth.postgresql.latency }}ms</div>
              <p class="text-xs text-muted-foreground">{{ systemHealth.postgresql.message }}</p>
            </CardContent>
          </Card>

          <Card v-if="systemHealth">
            <CardHeader class="flex flex-row items-center justify-between space-y-0 pb-2">
              <CardTitle class="text-sm font-medium">ClickHouse</CardTitle>
              <Badge :variant="getStatusBadgeVariant(systemHealth.clickhouse.status)">
                {{ systemHealth.clickhouse.status }}
              </Badge>
            </CardHeader>
            <CardContent>
              <div class="text-2xl font-bold">{{ systemHealth.clickhouse.latency }}ms</div>
              <p class="text-xs text-muted-foreground">{{ systemHealth.clickhouse.message }}</p>
            </CardContent>
          </Card>

          <Card v-if="systemHealth">
            <CardHeader class="flex flex-row items-center justify-between space-y-0 pb-2">
              <CardTitle class="text-sm font-medium">Valkey</CardTitle>
              <Badge :variant="getStatusBadgeVariant(systemHealth.valkey.status)">
                {{ systemHealth.valkey.status }}
              </Badge>
            </CardHeader>
            <CardContent>
              <div class="text-2xl font-bold">{{ systemHealth.valkey.latency }}ms</div>
              <p class="text-xs text-muted-foreground">{{ systemHealth.valkey.message }}</p>
            </CardContent>
          </Card>

          <Card v-if="systemHealth">
            <CardHeader class="flex flex-row items-center justify-between space-y-0 pb-2">
              <CardTitle class="text-sm font-medium">Storage</CardTitle>
              <Badge :variant="getStatusBadgeVariant(systemHealth.storage.status)">
                {{ systemHealth.storage.status }}
              </Badge>
            </CardHeader>
            <CardContent>
              <div class="text-2xl font-bold">{{ systemHealth.storage.latency }}ms</div>
              <p class="text-xs text-muted-foreground">{{ systemHealth.storage.message }}</p>
            </CardContent>
          </Card>
        </div>
      </div>

      <!-- Key Metrics -->
      <div>
        <h2 class="text-xl font-semibold mb-4">Instance Metrics</h2>
        <div class="grid gap-4 md:grid-cols-2 lg:grid-cols-4">
          <Card>
            <CardHeader class="flex flex-row items-center justify-between space-y-0 pb-2">
              <CardTitle class="text-sm font-medium">Total Servers</CardTitle>
              <Icon name="mdi:server" class="h-4 w-4 text-muted-foreground" />
            </CardHeader>
            <CardContent>
              <div class="text-2xl font-bold">{{ metricsOverview?.total_servers || 0 }}</div>
              <p class="text-xs text-muted-foreground">
                {{ metricsOverview?.active_servers || 0 }} active
              </p>
            </CardContent>
          </Card>

          <Card>
            <CardHeader class="flex flex-row items-center justify-between space-y-0 pb-2">
              <CardTitle class="text-sm font-medium">Storage Used</CardTitle>
              <Icon name="mdi:database" class="h-4 w-4 text-muted-foreground" />
            </CardHeader>
            <CardContent>
              <div class="text-2xl font-bold">{{ metricsOverview?.storage_used_readable || "0 B" }}</div>
              <p class="text-xs text-muted-foreground">Total storage consumption</p>
            </CardContent>
          </Card>

          <Card>
            <CardHeader class="flex flex-row items-center justify-between space-y-0 pb-2">
              <CardTitle class="text-sm font-medium">Total Events</CardTitle>
              <Icon name="mdi:chart-line" class="h-4 w-4 text-muted-foreground" />
            </CardHeader>
            <CardContent>
              <div class="text-2xl font-bold">{{ metricsOverview?.total_events.toLocaleString() || 0 }}</div>
              <p class="text-xs text-muted-foreground">
                {{ metricsOverview?.events_this_week.toLocaleString() || 0 }} this week
              </p>
            </CardContent>
          </Card>

          <Card>
            <CardHeader class="flex flex-row items-center justify-between space-y-0 pb-2">
              <CardTitle class="text-sm font-medium">Total Players</CardTitle>
              <Icon name="mdi:account-group" class="h-4 w-4 text-muted-foreground" />
            </CardHeader>
            <CardContent>
              <div class="text-2xl font-bold">{{ metricsOverview?.total_players.toLocaleString() || 0 }}</div>
              <p class="text-xs text-muted-foreground">Unique players tracked</p>
            </CardContent>
          </Card>
        </div>
      </div>

      <!-- Quick Links -->
      <div>
        <h2 class="text-xl font-semibold mb-4">Management Pages</h2>
        <div class="grid gap-4 md:grid-cols-2 lg:grid-cols-3">
          <Card class="hover:bg-accent cursor-pointer transition-colors" @click="navigateTo('/sudo/storage')">
            <CardHeader>
              <CardTitle class="flex items-center gap-2">
                <Icon name="mdi:folder" class="h-5 w-5" />
                Storage Management
              </CardTitle>
              <CardDescription>Manage files and view storage usage</CardDescription>
            </CardHeader>
          </Card>

          <Card class="hover:bg-accent cursor-pointer transition-colors" @click="navigateTo('/sudo/metrics')">
            <CardHeader>
              <CardTitle class="flex items-center gap-2">
                <Icon name="mdi:chart-bar" class="h-5 w-5" />
                Analytics
              </CardTitle>
              <CardDescription>View instance-wide metrics and analytics</CardDescription>
            </CardHeader>
          </Card>

          <Card class="hover:bg-accent cursor-pointer transition-colors" @click="navigateTo('/sudo/system')">
            <CardHeader>
              <CardTitle class="flex items-center gap-2">
                <Icon name="mdi:monitor-dashboard" class="h-5 w-5" />
                System Health
              </CardTitle>
              <CardDescription>Monitor system health and performance</CardDescription>
            </CardHeader>
          </Card>

          <Card class="hover:bg-accent cursor-pointer transition-colors" @click="navigateTo('/sudo/audit')">
            <CardHeader>
              <CardTitle class="flex items-center gap-2">
                <Icon name="mdi:file-document-multiple" class="h-5 w-5" />
                Audit Logs
              </CardTitle>
              <CardDescription>View global audit logs and activity</CardDescription>
            </CardHeader>
          </Card>

          <Card class="hover:bg-accent cursor-pointer transition-colors" @click="navigateTo('/sudo/sessions')">
            <CardHeader>
              <CardTitle class="flex items-center gap-2">
                <Icon name="mdi:account-key" class="h-5 w-5" />
                Sessions
              </CardTitle>
              <CardDescription>Manage active user sessions</CardDescription>
            </CardHeader>
          </Card>

          <Card class="hover:bg-accent cursor-pointer transition-colors" @click="navigateTo('/sudo/database')">
            <CardHeader>
              <CardTitle class="flex items-center gap-2">
                <Icon name="mdi:database-cog" class="h-5 w-5" />
                Database Stats
              </CardTitle>
              <CardDescription>View database statistics and health</CardDescription>
            </CardHeader>
          </Card>
        </div>
      </div>
    </div>
  </div>
</template>

