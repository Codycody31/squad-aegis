<script setup lang="ts">
import { ref, computed } from "vue";
import { Button } from "~/components/ui/button";
import { Input } from "~/components/ui/input";
import { Card, CardContent, CardHeader, CardTitle } from "~/components/ui/card";
import { Badge } from "~/components/ui/badge";
import { useAuthStore } from "@/stores/auth";
import PageHeader from "~/components/layout/PageHeader.vue";
import DataTable from "~/components/data-table/DataTable.vue";
import type { DataTableColumn } from "~/components/data-table/types";
import AddServerDialog from "~/components/servers/AddServerDialog.vue";
import { toast } from "~/components/ui/toast";

const runtimeConfig = useRuntimeConfig();
const loading = ref(true);
const error = ref<string | null>(null);
const servers = ref<Server[]>([]);
const searchQuery = ref("");
const authStore = useAuthStore();

const isSuperAdmin = computed(() => {
  return authStore.user?.super_admin || false;
});

interface Server {
  id: string;
  name: string;
  ip_address: string;
  game_port: number;
  rcon_ip_address: string | null;
  rcon_port: number;
  enabled: boolean;
  created_at: string;
  updated_at: string;
}

interface ServersResponse {
  data: {
    servers: Server[];
  };
}

const filteredServers = computed(() => {
  if (!searchQuery.value.trim()) {
    return servers.value;
  }

  const query = searchQuery.value.toLowerCase();
  return servers.value.filter(
    (server) =>
      server.name.toLowerCase().includes(query) ||
      server.ip_address.toLowerCase().includes(query)
  );
});

const columns: DataTableColumn<Server>[] = [
  { key: "name", label: "Name", sortable: true },
  { key: "ip_address", label: "IP Address" },
  { key: "game_port", label: "Game Port", hideOnMobile: true },
  {
    key: "rcon",
    label: "RCON",
    accessor: (s) => `${s.rcon_ip_address || s.ip_address}:${s.rcon_port}`,
  },
  { key: "created_at", label: "Created", sortable: true },
];

async function fetchServers() {
  loading.value = true;
  error.value = null;

  try {
    const data = await useAuthFetchImperative<ServersResponse>(
      `${runtimeConfig.public.backendApi}/servers`
    );

    servers.value = data.data?.servers || [];

    // Sort by creation date (most recent first)
    servers.value.sort((a, b) => {
      return (
        new Date(b.created_at).getTime() - new Date(a.created_at).getTime()
      );
    });

    // Keep the sidebar switcher in sync when servers are added or removed
    useServersStore().fetchServers(true);
  } catch (err: any) {
    error.value = extractApiErrorMessage(
      err,
      "An error occurred while fetching servers data"
    );
    console.error(err);
  } finally {
    loading.value = false;
  }
}

async function deleteServer(serverId: string) {
  if (
    !confirm(
      "Are you sure you want to delete this server? This action cannot be undone."
    )
  ) {
    return;
  }

  loading.value = true;
  error.value = null;

  try {
    await useAuthFetchImperative(
      `${runtimeConfig.public.backendApi}/servers/${serverId}`,
      { method: "DELETE" }
    );

    toast({ title: "Server deleted" });
    fetchServers();
  } catch (err: any) {
    error.value = extractApiErrorMessage(
      err,
      "An error occurred while deleting the server"
    );
    console.error(err);
    loading.value = false;
  }
}

function formatDate(dateString: string): string {
  return new Date(dateString).toLocaleString();
}

fetchServers();
</script>

<template>
  <div class="p-4 md:p-6">
    <PageHeader
      title="Servers"
      description="Squad servers managed by this Aegis instance."
    >
      <template #actions>
        <AddServerDialog v-if="isSuperAdmin" @created="fetchServers" />
        <Button
          @click="fetchServers"
          :disabled="loading"
          variant="outline"
          class="w-full sm:w-auto"
        >
          {{ loading ? "Refreshing..." : "Refresh" }}
        </Button>
      </template>
    </PageHeader>

    <div
      v-if="error"
      class="mb-4 rounded-md bg-destructive/15 p-4 text-sm text-destructive"
    >
      {{ error }}
    </div>

    <Card>
      <CardHeader class="pb-2 sm:pb-3">
        <CardTitle class="text-base sm:text-lg">Server List</CardTitle>
      </CardHeader>
      <CardContent>
        <div class="mb-3 flex items-center space-x-2 sm:mb-4">
          <Input
            v-model="searchQuery"
            placeholder="Search by name or IP address..."
            class="flex-grow"
          />
        </div>

        <DataTable
          :columns="columns"
          :rows="filteredServers"
          row-key="id"
          :loading="loading && servers.length === 0"
          :row-to="(s) => `/servers/${s.id}`"
        >
          <template #empty>
            <div class="py-10 text-center">
              <template v-if="servers.length === 0">
                <Icon
                  name="mdi:server-plus"
                  class="mx-auto mb-3 h-10 w-10 text-muted-foreground"
                />
                <p class="font-medium">No servers yet</p>
                <p class="mx-auto mt-1 max-w-md text-sm text-muted-foreground">
                  Add a Squad server to manage it from Aegis: live player
                  lists, RCON console, bans, plugins, and automation. You'll
                  need the server's RCON credentials and log access.
                </p>
                <div v-if="isSuperAdmin" class="mt-4">
                  <AddServerDialog @created="fetchServers" />
                </div>
              </template>
              <p v-else class="text-sm text-muted-foreground">
                No servers match your search
              </p>
            </div>
          </template>
          <template #cell-name="{ row }">
            <div class="flex items-center gap-2 font-medium">
              {{ row.name }}
              <Badge v-if="!row.enabled" variant="secondary">Offline</Badge>
            </div>
          </template>
          <template #cell-created_at="{ row }">
            {{ formatDate(row.created_at) }}
          </template>
          <template #actions="{ row }">
            <div class="flex justify-end gap-2">
              <Button variant="outline" size="sm" as-child>
                <NuxtLink :to="`/servers/${row.id}`">Manage</NuxtLink>
              </Button>
              <Button
                v-if="isSuperAdmin"
                variant="destructive"
                size="sm"
                :disabled="loading"
                @click="deleteServer(row.id)"
              >
                Delete
              </Button>
            </div>
          </template>
        </DataTable>
      </CardContent>
    </Card>
  </div>
</template>
