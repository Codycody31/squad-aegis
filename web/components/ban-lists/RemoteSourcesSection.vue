<script setup lang="ts">
import { onMounted, ref } from "vue";
import { Badge } from "~/components/ui/badge";
import { Button } from "~/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "~/components/ui/card";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "~/components/ui/dialog";
import { Input } from "~/components/ui/input";
import { Label } from "~/components/ui/label";
import { Switch } from "~/components/ui/switch";
import { toast } from "~/components/ui/toast";
import DataTable from "~/components/data-table/DataTable.vue";
import type { DataTableColumn } from "~/components/data-table/types";
import { ExternalLink, Plus, Trash2 } from "lucide-vue-next";

const runtimeConfig = useRuntimeConfig();

const loading = ref(true);
const remoteSources = ref<any[]>([]);
const showDialog = ref(false);

const form = ref({
  name: "",
  url: "",
  sync_enabled: true,
  sync_interval_minutes: 180,
});

const resetForm = () => {
  form.value = {
    name: "",
    url: "",
    sync_enabled: true,
    sync_interval_minutes: 180,
  };
};

const loadRemoteSources = async () => {
  loading.value = true;
  try {
    const response = await useAuthFetchImperative<any>(
      `${runtimeConfig.public.backendApi}/remote-ban-sources`
    );
    remoteSources.value = response.data.sources || [];
  } catch (error: any) {
    console.error("Failed to load remote sources:", error);
    toast({
      title: "Error",
      description: "Failed to load remote sources",
      variant: "destructive",
    });
  } finally {
    loading.value = false;
  }
};

const createRemoteSource = async () => {
  try {
    await useAuthFetchImperative(
      `${runtimeConfig.public.backendApi}/remote-ban-sources`,
      {
        method: "POST",
        body: form.value,
      }
    );

    toast({
      title: "Success",
      description: "Remote source created successfully",
    });

    showDialog.value = false;
    resetForm();
    await loadRemoteSources();
  } catch (error: any) {
    console.error("Failed to create remote source:", error);
    toast({
      title: "Error",
      description: "Failed to create remote source",
      variant: "destructive",
    });
  }
};

const deleteRemoteSource = async (remoteSource: any) => {
  if (
    !confirm(
      `Are you sure you want to delete the remote source "${remoteSource.name}"?`
    )
  ) {
    return;
  }

  try {
    await useAuthFetchImperative(
      `${runtimeConfig.public.backendApi}/remote-ban-sources/${remoteSource.id}`,
      {
        method: "DELETE",
      }
    );

    toast({
      title: "Success",
      description: "Remote source deleted successfully",
    });

    await loadRemoteSources();
  } catch (error: any) {
    console.error("Failed to delete remote source:", error);
    toast({
      title: "Error",
      description: "Failed to delete remote source",
      variant: "destructive",
    });
  }
};

const columns: DataTableColumn<any>[] = [
  { key: "name", label: "Name" },
  { key: "url", label: "URL" },
  { key: "sync_status", label: "Sync Status" },
  {
    key: "sync_interval_minutes",
    label: "Interval",
    accessor: (row) => `${row.sync_interval_minutes} min`,
  },
  {
    key: "last_synced_at",
    label: "Last Sync",
    accessor: (row) =>
      row.last_synced_at
        ? new Date(row.last_synced_at).toLocaleString()
        : "Never",
  },
];

onMounted(loadRemoteSources);

defineExpose({ refresh: loadRemoteSources });
</script>

<template>
  <Card>
    <CardHeader class="pb-2 sm:pb-3">
      <div
        class="flex flex-col sm:flex-row sm:justify-between sm:items-center gap-3 sm:gap-0"
      >
        <div>
          <CardTitle class="flex items-center gap-2 text-base sm:text-lg">
            <ExternalLink class="h-4 w-4 sm:h-5 sm:w-5" />
            Remote Ban Sources
          </CardTitle>
          <CardDescription class="text-xs sm:text-sm">
            Configure external ban list sources to automatically sync bans from
            other communities.
          </CardDescription>
        </div>
        <Dialog v-model:open="showDialog">
          <DialogTrigger asChild>
            <Button class="w-full sm:w-auto text-sm sm:text-base">
              <Plus class="h-4 w-4 mr-2" />
              Add Remote Source
            </Button>
          </DialogTrigger>
          <DialogContent
            class="w-[95vw] sm:max-w-[500px] max-h-[90vh] overflow-y-auto p-4 sm:p-6"
          >
            <DialogHeader>
              <DialogTitle class="text-base sm:text-lg"
                >Add Remote Ban Source</DialogTitle
              >
              <DialogDescription class="text-xs sm:text-sm">
                Add a remote ban source to automatically sync bans from other
                communities.
              </DialogDescription>
            </DialogHeader>
            <div class="grid gap-4 py-4">
              <div>
                <Label htmlFor="remote_name">Name</Label>
                <Input
                  id="remote_name"
                  v-model="form.name"
                  placeholder="Community Ban List"
                />
              </div>
              <div>
                <Label htmlFor="remote_source_url">URL</Label>
                <Input
                  id="remote_source_url"
                  v-model="form.url"
                  placeholder="https://example.com/bans.cfg"
                />
              </div>
              <div class="flex items-center space-x-2">
                <Switch id="sync_enabled" v-model="form.sync_enabled" />
                <Label htmlFor="sync_enabled">Enable Automatic Sync</Label>
              </div>
              <div>
                <Label htmlFor="sync_interval">Sync Interval (minutes)</Label>
                <Input
                  id="sync_interval"
                  v-model="form.sync_interval_minutes"
                  type="number"
                  min="60"
                  max="1440"
                  placeholder="180"
                />
              </div>
            </div>
            <DialogFooter>
              <Button variant="outline" @click="showDialog = false">
                Cancel
              </Button>
              <Button @click="createRemoteSource"> Add Remote Source </Button>
            </DialogFooter>
          </DialogContent>
        </Dialog>
      </div>
    </CardHeader>
    <CardContent>
      <DataTable
        :columns="columns"
        :rows="remoteSources"
        row-key="id"
        :loading="loading && remoteSources.length === 0"
        empty-text="No remote sources configured. Add remote sources to sync bans from other communities."
      >
        <template #cell-name="{ row }">
          <span class="font-medium">{{ row.name }}</span>
        </template>
        <template #cell-url="{ row }">
          <span class="font-mono text-xs sm:text-sm break-all">{{
            row.url
          }}</span>
        </template>
        <template #cell-sync_status="{ row }">
          <Badge
            :variant="row.sync_enabled ? 'default' : 'secondary'"
            class="text-xs"
          >
            {{ row.sync_enabled ? "Enabled" : "Disabled" }}
          </Badge>
        </template>
        <template #actions="{ row }">
          <Button
            variant="destructive"
            size="sm"
            class="text-xs"
            @click="deleteRemoteSource(row)"
          >
            <Trash2 class="h-4 w-4" />
          </Button>
        </template>
      </DataTable>
    </CardContent>
  </Card>
</template>
