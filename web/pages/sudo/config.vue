<script setup lang="ts">
import { ref, onMounted } from "vue";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "~/components/ui/card";
import type { SystemConfig } from "~/types";

definePageMeta({ middleware: "auth", layout: "sudo" });

const runtimeConfig = useRuntimeConfig();
const authStore = useAuthStore();

if (!authStore.user?.super_admin) navigateTo("/dashboard");

const loading = ref(true);
const config = ref<SystemConfig | null>(null);

const fetchConfig = async () => {
  loading.value = true;
  try {
    const res = await useAuthFetchImperative<any>(`${runtimeConfig.public.backendApi}/sudo/system/config`);
    config.value = res.data.data;
  } catch (err: any) {
    console.error("Error fetching config:", err);
  } finally {
    loading.value = false;
  }
};

onMounted(fetchConfig);
</script>

<template>
  <div class="p-6 space-y-6">
    <h1 class="text-3xl font-bold">Configuration Viewer</h1>
    <p class="text-muted-foreground">Read-only view of instance configuration (sensitive values masked)</p>

    <div v-if="loading" class="flex items-center justify-center py-12">
      <div class="text-muted-foreground">Loading configuration...</div>
    </div>

    <div v-else-if="config" class="space-y-4">
      <Card>
        <CardHeader>
          <CardTitle>Application</CardTitle>
        </CardHeader>
        <CardContent>
          <pre class="text-sm bg-muted p-4 rounded-lg overflow-auto">{{ JSON.stringify(config.app, null, 2) }}</pre>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>Database</CardTitle>
        </CardHeader>
        <CardContent>
          <pre class="text-sm bg-muted p-4 rounded-lg overflow-auto">{{ JSON.stringify(config.database, null, 2) }}</pre>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>ClickHouse</CardTitle>
        </CardHeader>
        <CardContent>
          <pre class="text-sm bg-muted p-4 rounded-lg overflow-auto">{{ JSON.stringify(config.clickhouse, null, 2) }}</pre>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>Storage</CardTitle>
        </CardHeader>
        <CardContent>
          <pre class="text-sm bg-muted p-4 rounded-lg overflow-auto">{{ JSON.stringify(config.storage, null, 2) }}</pre>
        </CardContent>
      </Card>
    </div>
  </div>
</template>

