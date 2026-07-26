<script setup lang="ts">
import { onMounted, ref } from "vue";
import type { ConnectorDefinition, ConnectorInstance } from "~/types";
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
} from "~/components/ui/dialog";
import { Switch } from "~/components/ui/switch";
import { toast } from "~/components/ui/toast";
import DataTable from "~/components/data-table/DataTable.vue";
import type { DataTableColumn } from "~/components/data-table/types";
import {
  AlertCircle,
  CheckCircle,
  Clock,
  Settings,
  Trash2,
  Wifi,
  WifiOff,
} from "lucide-vue-next";

const props = defineProps<{
  availableConnectors: ConnectorDefinition[];
}>();

const emit = defineEmits<{
  (e: "configure", connector: ConnectorInstance): void;
}>();

const loading = ref(true);
const connectors = ref<ConnectorInstance[]>([]);
const deleteConnectorTarget = ref<ConnectorInstance | null>(null);

/** Match a running instance id (e.g. `discord`) to an available definition (canonical or legacy). */
const connectorDefForInstance = (instanceId: string) => {
  return props.availableConnectors.find(
    (c: any) =>
      c.id === instanceId ||
      (Array.isArray(c.legacy_ids) && c.legacy_ids.includes(instanceId))
  );
};

// Status color mapping
const getStatusColor = (status: string) => {
  switch (status) {
    case "running":
      return "bg-green-100 text-green-800 dark:bg-green-900/20 dark:text-green-400";
    case "stopped":
      return "bg-gray-100 text-gray-800 dark:bg-gray-900/20 dark:text-gray-400";
    case "error":
      return "bg-red-100 text-red-800 dark:bg-red-900/20 dark:text-red-400";
    case "starting":
      return "bg-yellow-100 text-yellow-800 dark:bg-yellow-900/20 dark:text-yellow-400";
    case "disabled":
      return "bg-gray-100 text-gray-600 dark:bg-gray-900/20 dark:text-gray-500";
    default:
      return "bg-gray-100 text-gray-800 dark:bg-gray-900/20 dark:text-gray-400";
  }
};

const getStatusIcon = (status: string) => {
  switch (status) {
    case "running":
      return CheckCircle;
    case "error":
      return AlertCircle;
    case "starting":
      return Clock;
    case "stopped":
      return WifiOff;
    default:
      return Wifi;
  }
};

// Load configured connectors
const loadConnectors = async () => {
  loading.value = true;
  try {
    const response = await useAuthFetchImperative<any>("/api/connectors");
    connectors.value = response.data.connectors || [];
  } catch (error: any) {
    console.error("Failed to load connectors:", error);
    toast({
      title: "Error",
      description: "Failed to load connectors",
      variant: "destructive",
    });
  } finally {
    loading.value = false;
  }
};

// Delete connector instance
const confirmDeleteConnector = (connector: ConnectorInstance) => {
  deleteConnectorTarget.value = connector;
};

const cancelDeleteConnector = () => {
  deleteConnectorTarget.value = null;
};

const deleteConnector = async () => {
  const connector = deleteConnectorTarget.value;
  if (!connector) return;

  try {
    await useAuthFetchImperative(`/api/connectors/${connector.id}`, {
      method: "DELETE",
    });

    toast({
      title: "Success",
      description: "Connector deleted successfully",
    });

    deleteConnectorTarget.value = null;
    await loadConnectors();
  } catch (error: any) {
    console.error("Failed to delete connector:", error);
    toast({
      title: "Error",
      description: error.data?.message || "Failed to delete connector",
      variant: "destructive",
    });
  }
};

const columns: DataTableColumn<ConnectorInstance>[] = [
  { key: "type", label: "Type" },
  { key: "name", label: "Name" },
  { key: "status", label: "Status" },
  { key: "enabled", label: "Enabled" },
  { key: "last_error", label: "Last Error" },
];

onMounted(loadConnectors);

defineExpose({ refresh: loadConnectors });
</script>

<template>
  <Card>
    <CardHeader class="pb-2 sm:pb-3">
      <CardTitle class="text-base sm:text-lg">Active Connectors</CardTitle>
      <CardDescription class="text-xs sm:text-sm">
        Global service connectors available to all plugins
      </CardDescription>
    </CardHeader>
    <CardContent>
      <DataTable
        :columns="columns"
        :rows="connectors"
        row-key="id"
        :loading="loading && connectors.length === 0"
      >
        <template #empty>
          <div class="text-center py-6 sm:py-8">
            <p class="text-sm sm:text-base text-muted-foreground">
              No connector instances configured
            </p>
            <p class="text-xs text-muted-foreground mt-2">
              Use <span class="font-medium">Add Connector</span> to enable
              Discord or other registered connectors.
            </p>
          </div>
        </template>
        <template #cell-type="{ row }">
          <div class="flex items-center space-x-2">
            <component :is="getStatusIcon(row.status)" class="w-4 h-4" />
            <span class="font-medium text-sm sm:text-base">{{ row.id }}</span>
          </div>
        </template>
        <template #cell-name="{ row }">
          <div class="flex flex-col">
            <span class="text-sm sm:text-base">{{
              connectorDefForInstance(row.id)?.name || row.id
            }}</span>
            <span class="text-xs sm:text-sm text-muted-foreground">
              {{ connectorDefForInstance(row.id)?.description }}
            </span>
          </div>
        </template>
        <template #cell-status="{ row }">
          <Badge :class="getStatusColor(row.status)" class="text-xs">
            <component :is="getStatusIcon(row.status)" class="w-3 h-3 mr-1" />
            {{ row.status }}
          </Badge>
        </template>
        <template #cell-enabled="{ row }">
          <Switch :model-value="row.enabled" disabled />
        </template>
        <template #cell-last_error="{ row }">
          <span
            v-if="row.last_error"
            class="text-xs sm:text-sm text-red-600 dark:text-red-400"
            :title="row.last_error"
          >
            {{
              row.last_error.length > 50
                ? row.last_error.substring(0, 50) + "..."
                : row.last_error
            }}
          </span>
          <span v-else class="text-xs sm:text-sm text-muted-foreground"
            >None</span
          >
        </template>
        <template #actions="{ row }">
          <div class="flex items-center justify-end space-x-2">
            <Button
              variant="outline"
              size="sm"
              class="text-xs"
              @click="emit('configure', row)"
            >
              <Settings class="w-4 h-4" />
            </Button>
            <Button
              variant="destructive"
              size="sm"
              class="text-xs"
              @click="confirmDeleteConnector(row)"
            >
              <Trash2 class="w-4 h-4" />
            </Button>
          </div>
        </template>
      </DataTable>
    </CardContent>
  </Card>

  <Dialog
    :open="deleteConnectorTarget !== null"
    @update:open="(open) => !open && cancelDeleteConnector()"
  >
    <DialogContent>
      <DialogHeader>
        <DialogTitle>Delete connector instance?</DialogTitle>
        <DialogDescription>
          Delete the connector instance
          <span class="font-medium">"{{ deleteConnectorTarget?.id }}"</span>?
          Any plugin that depends on this connector will stop receiving it.
        </DialogDescription>
      </DialogHeader>
      <DialogFooter>
        <Button variant="outline" @click="cancelDeleteConnector">Cancel</Button>
        <Button variant="destructive" @click="deleteConnector">Delete</Button>
      </DialogFooter>
    </DialogContent>
  </Dialog>
</template>
