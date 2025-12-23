<script setup lang="ts">
import { ref, onMounted } from "vue";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "~/components/ui/card";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "~/components/ui/table";
import type { MetricsTimeline, ServerActivity } from "~/types";

definePageMeta({ middleware: "auth", layout: "sudo" });

const runtimeConfig = useRuntimeConfig();
const authStore = useAuthStore();

if (!authStore.user?.super_admin) navigateTo("/dashboard");

const loading = ref(true);
const timeline = ref<MetricsTimeline | null>(null);
const activities = ref<ServerActivity[]>([]);

const fetchData = async () => {
  loading.value = true;
  try {
    const [timelineRes, activitiesRes] = await Promise.all([
      $fetch<any>(`${runtimeConfig.public.backendApi}/sudo/metrics/timeline`, {
        headers: { Authorization: `Bearer ${authStore.token}` },
      }),
      $fetch<any>(`${runtimeConfig.public.backendApi}/sudo/metrics/servers`, {
        headers: { Authorization: `Bearer ${authStore.token}` },
      }),
    ]);
    timeline.value = timelineRes.data.data;
    activities.value = activitiesRes.data.data;
  } catch (err: any) {
    console.error("Error fetching metrics:", err);
  } finally {
    loading.value = false;
  }
};

onMounted(fetchData);
</script>

<template>
  <div class="p-6 space-y-6">
    <h1 class="text-3xl font-bold">Analytics Dashboard</h1>

    <div v-if="loading" class="flex items-center justify-center py-12">
      <div class="text-muted-foreground">Loading analytics...</div>
    </div>

    <div v-else>
      <Card>
        <CardHeader>
          <CardTitle>Server Activity</CardTitle>
          <CardDescription>Activity breakdown per server (last 30 days)</CardDescription>
        </CardHeader>
        <CardContent>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Server</TableHead>
                <TableHead>Events</TableHead>
                <TableHead>Messages</TableHead>
                <TableHead>Players</TableHead>
                <TableHead>Workflows</TableHead>
                <TableHead>Avg Players</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              <TableRow v-for="activity in activities" :key="activity.server_id">
                <TableCell class="font-medium">{{ activity.server_name }}</TableCell>
                <TableCell>{{ activity.total_events.toLocaleString() }}</TableCell>
                <TableCell>{{ activity.chat_messages.toLocaleString() }}</TableCell>
                <TableCell>{{ activity.unique_players.toLocaleString() }}</TableCell>
                <TableCell>{{ activity.workflow_runs.toLocaleString() }}</TableCell>
                <TableCell>{{ activity.avg_player_count.toFixed(1) }}</TableCell>
              </TableRow>
            </TableBody>
          </Table>
        </CardContent>
      </Card>
    </div>
  </div>
</template>

