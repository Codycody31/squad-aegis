<script setup lang="ts">
import { onMounted, ref } from "vue";
import type { ConnectorDefinition, ConnectorInstance } from "~/types";
import { Button } from "~/components/ui/button";
import { toast } from "~/components/ui/toast";
import PageHeader from "~/components/layout/PageHeader.vue";
import ConnectorCatalogTable from "~/components/connectors/ConnectorCatalogTable.vue";
import ConfiguredConnectorsTable from "~/components/connectors/ConfiguredConnectorsTable.vue";
import ConnectorConfigDialog from "~/components/connectors/ConnectorConfigDialog.vue";
import { Plus, RefreshCw } from "lucide-vue-next";

definePageMeta({
  middleware: ["auth", "sudo"],
});

const availableConnectors = ref<ConnectorDefinition[]>([]);

const catalogTable = ref<InstanceType<typeof ConnectorCatalogTable> | null>(
  null
);
const configuredTable = ref<InstanceType<
  typeof ConfiguredConnectorsTable
> | null>(null);

const dialogOpen = ref(false);
const dialogMode = ref<"create" | "edit">("create");
const dialogConnector = ref<ConnectorInstance | null>(null);

// Load available connector definitions
const loadAvailableConnectors = async () => {
  try {
    const response = await useAuthFetchImperative<any>(
      "/api/connectors/available"
    );
    availableConnectors.value = response.data.connectors || [];
  } catch (error: any) {
    console.error("Failed to load available connectors:", error);
    toast({
      title: "Error",
      description:
        error.data?.message || "Failed to load available connectors",
      variant: "destructive",
    });
  }
};

const openAddDialog = () => {
  dialogMode.value = "create";
  dialogConnector.value = null;
  dialogOpen.value = true;
};

const configureConnector = (connector: ConnectorInstance) => {
  dialogMode.value = "edit";
  dialogConnector.value = connector;
  dialogOpen.value = true;
};

const refreshAll = async () => {
  await Promise.all([
    configuredTable.value?.refresh(),
    catalogTable.value?.refresh(),
    loadAvailableConnectors(),
  ]);
};

onMounted(loadAvailableConnectors);
</script>

<template>
  <div class="p-4 md:p-6">
    <PageHeader
      title="Global Connectors"
      description="Manage global service connectors (Discord, Slack, etc.)"
    >
      <template #actions>
        <Button
          variant="outline"
          class="w-full sm:w-auto text-sm sm:text-base"
          @click="refreshAll"
        >
          <RefreshCw class="w-4 h-4 mr-2" />
          Refresh
        </Button>
        <Button
          class="w-full sm:w-auto text-sm sm:text-base"
          @click="openAddDialog"
        >
          <Plus class="w-4 h-4 mr-2" />
          Add Connector
        </Button>
      </template>
    </PageHeader>

    <div class="space-y-4 sm:space-y-6">
      <ConnectorCatalogTable
        ref="catalogTable"
        @changed="loadAvailableConnectors"
      />

      <ConfiguredConnectorsTable
        ref="configuredTable"
        :available-connectors="availableConnectors"
        @configure="configureConnector"
      />
    </div>

    <ConnectorConfigDialog
      v-model:open="dialogOpen"
      :mode="dialogMode"
      :available-connectors="availableConnectors"
      :connector="dialogConnector"
      @saved="configuredTable?.refresh()"
    />
  </div>
</template>
