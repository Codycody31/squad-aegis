<script setup lang="ts">
import { ref, onMounted } from "vue";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "~/components/ui/card";
import { Button } from "~/components/ui/button";
import { Badge } from "~/components/ui/badge";
import PageHeader from "~/components/layout/PageHeader.vue";
import DataTable from "~/components/data-table/DataTable.vue";
import type { DataTableColumn } from "~/components/data-table/types";
import type { SessionInfo } from "~/types";

definePageMeta({ middleware: ["auth", "sudo"], layout: "sudo" });

const runtimeConfig = useRuntimeConfig();

const loading = ref(true);
const sessions = ref<SessionInfo[]>([]);

const columns: DataTableColumn<SessionInfo>[] = [
  { key: "username", label: "User" },
  {
    key: "last_seen",
    label: "Last Seen",
    accessor: (s) => new Date(s.last_seen).toLocaleString(),
  },
  { key: "last_seen_ip", label: "IP Address" },
  { key: "time_remaining", label: "Time Remaining" },
];

const fetchSessions = async () => {
  loading.value = true;
  try {
    const res = await useAuthFetchImperative<any>(
      `${runtimeConfig.public.backendApi}/sudo/sessions`
    );
    sessions.value = res.data.sessions;
  } catch (err: any) {
    console.error("Error fetching sessions:", err);
  } finally {
    loading.value = false;
  }
};

const deleteSession = async (sessionId: string) => {
  if (!confirm("Are you sure you want to force logout this session?")) return;

  try {
    await useAuthFetchImperative(
      `${runtimeConfig.public.backendApi}/sudo/sessions/${sessionId}`,
      {
        method: "DELETE",
      }
    );
    await fetchSessions();
  } catch (err: any) {
    console.error("Error deleting session:", err);
  }
};

onMounted(fetchSessions);
</script>

<template>
  <div class="space-y-6 p-4 md:p-6">
    <PageHeader
      title="Session Management"
      description="Active sign-ins across the instance."
    />

    <Card>
      <CardHeader>
        <CardTitle>Active Sessions</CardTitle>
        <CardDescription>Manage user login sessions</CardDescription>
      </CardHeader>
      <CardContent>
        <DataTable
          :columns="columns"
          :rows="sessions"
          row-key="id"
          :loading="loading && sessions.length === 0"
          empty-text="No active sessions."
        >
          <template #cell-username="{ row }">
            <span class="font-medium">{{ row.username }}</span>
          </template>
          <template #cell-time_remaining="{ row }">
            <Badge :variant="row.is_expired ? 'destructive' : 'default'">
              {{ row.time_remaining }}
            </Badge>
          </template>
          <template #actions="{ row }">
            <Button
              @click="deleteSession(row.id)"
              size="sm"
              variant="destructive"
            >
              <Icon name="mdi:logout" class="mr-2 h-4 w-4" />
              Force Logout
            </Button>
          </template>
        </DataTable>
      </CardContent>
    </Card>
  </div>
</template>
