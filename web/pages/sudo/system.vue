<script setup lang="ts">
import { ref, onMounted } from "vue";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "~/components/ui/card";
import { Badge } from "~/components/ui/badge";
import { Alert, AlertDescription } from "~/components/ui/alert";
import type { SystemHealth } from "~/types";

definePageMeta({ middleware: "auth", layout: "sudo" });

const runtimeConfig = useRuntimeConfig();
const authStore = useAuthStore();

if (!authStore.user?.super_admin) navigateTo("/dashboard");

const loading = ref(true);
const health = ref<SystemHealth | null>(null);

const fetchHealth = async () => {
  loading.value = true;
  try {
    const res = await useAuthFetchImperative<any>(`${runtimeConfig.public.backendApi}/sudo/system/health`);
    health.value = res.data.data;
  } catch (err: any) {
    console.error("Error fetching health:", err);
  } finally {
    loading.value = false;
  }
};

onMounted(fetchHealth);

const getStatusVariant = (status: string) => {
  switch (status) {
    case "healthy": return "default";
    case "degraded": return "secondary";
    case "unhealthy": return "destructive";
    default: return "outline";
  }
};
</script>

<template>
  <div class="p-6 space-y-6">
    <h1 class="text-3xl font-bold">System Health Monitor</h1>

    <div v-if="loading" class="flex items-center justify-center py-12">
      <div class="text-muted-foreground">Loading system health...</div>
    </div>

    <div v-else-if="health" class="space-y-4">
      <Alert>
        <AlertDescription>
          <div class="flex items-center gap-2">
            <span class="font-semibold">Overall Status:</span>
            <Badge :variant="getStatusVariant(health.overall)">{{ health.overall }}</Badge>
          </div>
        </AlertDescription>
      </Alert>

      <div class="grid gap-4 md:grid-cols-2">
        <Card>
          <CardHeader>
            <CardTitle>PostgreSQL</CardTitle>
            <CardDescription>Primary database</CardDescription>
          </CardHeader>
          <CardContent>
            <div class="space-y-2">
              <div class="flex justify-between">
                <span>Status:</span>
                <Badge :variant="getStatusVariant(health.postgresql.status)">{{ health.postgresql.status }}</Badge>
              </div>
              <div class="flex justify-between">
                <span>Latency:</span>
                <span>{{ health.postgresql.latency }}ms</span>
              </div>
              <p class="text-sm text-muted-foreground">{{ health.postgresql.message }}</p>
            </div>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>ClickHouse</CardTitle>
            <CardDescription>Analytics database</CardDescription>
          </CardHeader>
          <CardContent>
            <div class="space-y-2">
              <div class="flex justify-between">
                <span>Status:</span>
                <Badge :variant="getStatusVariant(health.clickhouse.status)">{{ health.clickhouse.status }}</Badge>
              </div>
              <div class="flex justify-between">
                <span>Latency:</span>
                <span>{{ health.clickhouse.latency }}ms</span>
              </div>
              <p class="text-sm text-muted-foreground">{{ health.clickhouse.message }}</p>
            </div>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>Valkey</CardTitle>
            <CardDescription>Cache layer</CardDescription>
          </CardHeader>
          <CardContent>
            <div class="space-y-2">
              <div class="flex justify-between">
                <span>Status:</span>
                <Badge :variant="getStatusVariant(health.valkey.status)">{{ health.valkey.status }}</Badge>
              </div>
              <div class="flex justify-between">
                <span>Latency:</span>
                <span>{{ health.valkey.latency }}ms</span>
              </div>
              <p class="text-sm text-muted-foreground">{{ health.valkey.message }}</p>
            </div>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>Storage</CardTitle>
            <CardDescription>File storage backend</CardDescription>
          </CardHeader>
          <CardContent>
            <div class="space-y-2">
              <div class="flex justify-between">
                <span>Status:</span>
                <Badge :variant="getStatusVariant(health.storage.status)">{{ health.storage.status }}</Badge>
              </div>
              <div class="flex justify-between">
                <span>Latency:</span>
                <span>{{ health.storage.latency }}ms</span>
              </div>
              <p class="text-sm text-muted-foreground">{{ health.storage.message }}</p>
            </div>
          </CardContent>
        </Card>
      </div>
    </div>
  </div>
</template>

