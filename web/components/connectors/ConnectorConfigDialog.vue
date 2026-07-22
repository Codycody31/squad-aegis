<script setup lang="ts">
import { computed, ref, watch } from "vue";
import type { ConnectorDefinition, ConnectorInstance } from "~/types";
import { initializeConfigFromSchema } from "~/composables/useConfigSchema";
import { Button } from "~/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "~/components/ui/dialog";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "~/components/ui/select";
import { Input } from "~/components/ui/input";
import { Label } from "~/components/ui/label";
import { Switch } from "~/components/ui/switch";
import { toast } from "~/components/ui/toast";

const props = defineProps<{
  mode: "create" | "edit";
  availableConnectors: ConnectorDefinition[];
  connector?: ConnectorInstance | null;
}>();

const emit = defineEmits<{
  (e: "saved"): void;
}>();

const open = defineModel<boolean>("open", { default: false });

const selectedConnector = ref<string>("");
const connectorConfig = ref<Record<string, any>>({});

/** Match a running instance id (e.g. `discord`) to an available definition (canonical or legacy). */
const connectorDefForInstance = (instanceId: string) => {
  return props.availableConnectors.find(
    (c: any) =>
      c.id === instanceId ||
      (Array.isArray(c.legacy_ids) && c.legacy_ids.includes(instanceId))
  );
};

const configFields = computed(() => {
  if (props.mode === "edit") {
    if (!props.connector) return [];
    return (
      connectorDefForInstance(props.connector.id)?.config_schema?.fields || []
    );
  }
  if (!selectedConnector.value) return [];
  return (
    props.availableConnectors.find((c) => c.id === selectedConnector.value)
      ?.config_schema?.fields || []
  );
});

watch(open, (isOpen) => {
  if (!isOpen) return;
  if (props.mode === "edit" && props.connector) {
    const baseConfig = (props.connector.config || {}) as Record<
      string,
      unknown
    >;
    const connectorDef = connectorDefForInstance(props.connector.id);
    connectorConfig.value = connectorDef?.config_schema?.fields
      ? initializeConfigFromSchema(baseConfig, connectorDef.config_schema.fields)
      : { ...baseConfig };
  } else {
    selectedConnector.value = "";
    connectorConfig.value = {};
  }
});

// Handle connector selection change
const onConnectorSelect = (connectorId: any) => {
  selectedConnector.value = connectorId || "";
  const connector = props.availableConnectors.find(
    (c) => c.id === connectorId
  );
  if (connector?.config_schema?.fields) {
    connectorConfig.value = initializeConfigFromSchema(
      {},
      connector.config_schema.fields
    );
  }
};

// Create new connector instance
const createConnector = async () => {
  if (!selectedConnector.value) {
    toast({
      title: "Error",
      description: "Please select a connector",
      variant: "destructive",
    });
    return;
  }

  try {
    await useAuthFetchImperative("/api/connectors", {
      method: "POST",
      body: {
        connector_id: selectedConnector.value,
        config: connectorConfig.value,
      },
    });

    toast({
      title: "Success",
      description: "Connector created successfully",
    });

    open.value = false;
    emit("saved");
  } catch (error: any) {
    console.error("Failed to create connector:", error);
    toast({
      title: "Error",
      description: error.data?.message || "Failed to create connector",
      variant: "destructive",
    });
  }
};

// Save connector configuration
const saveConnectorConfig = async () => {
  if (!props.connector) return;

  try {
    await useAuthFetchImperative(`/api/connectors/${props.connector.id}`, {
      method: "PUT",
      body: {
        config: connectorConfig.value,
      },
    });

    toast({
      title: "Success",
      description: "Connector configuration updated successfully",
    });

    open.value = false;
    emit("saved");
  } catch (error: any) {
    console.error("Failed to update connector config:", error);
    toast({
      title: "Error",
      description:
        error.data?.message || "Failed to update connector configuration",
      variant: "destructive",
    });
  }
};

const submit = () => {
  if (props.mode === "edit") {
    saveConnectorConfig();
  } else {
    createConnector();
  }
};
</script>

<template>
  <Dialog v-model:open="open">
    <DialogContent
      class="w-[95vw] sm:max-w-2xl max-h-[90vh] overflow-y-auto p-4 sm:p-6"
    >
      <DialogHeader>
        <DialogTitle class="text-base sm:text-lg">
          {{
            mode === "edit"
              ? `Configure ${connector?.id} Connector`
              : "Add New Connector"
          }}
        </DialogTitle>
        <DialogDescription class="text-xs sm:text-sm">
          {{
            mode === "edit"
              ? "Update the configuration for this connector."
              : "Select a connector type and configure its settings."
          }}
        </DialogDescription>
      </DialogHeader>

      <div class="space-y-4">
        <div v-if="mode === 'create'">
          <Label for="connector-select">Connector Type</Label>
          <Select
            :model-value="selectedConnector"
            @update:model-value="onConnectorSelect"
          >
            <SelectTrigger>
              <SelectValue placeholder="Select a connector type" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem
                v-for="def in availableConnectors"
                :key="def.id"
                :value="def.id"
              >
                <div class="flex flex-col">
                  <span class="font-medium">{{ def.name }}</span>
                  <span class="text-sm text-muted-foreground">{{
                    def.description
                  }}</span>
                </div>
              </SelectItem>
            </SelectContent>
          </Select>
        </div>

        <!-- Dynamic configuration fields -->
        <div
          v-if="mode === 'edit' || selectedConnector"
          class="space-y-4"
        >
          <h4 v-if="mode === 'create'" class="font-medium">Configuration</h4>
          <div
            v-for="field in configFields"
            :key="field.name"
            class="space-y-2"
          >
            <Label :for="`config-${field.name}`">
              {{ field.name }}
              <span v-if="field.required" class="text-red-500">*</span>
            </Label>
            <p v-if="field.description" class="text-sm text-muted-foreground">
              {{ field.description }}
            </p>

            <!-- String/Text input -->
            <Input
              v-if="field.type === 'string'"
              :id="`config-${field.name}`"
              v-model="connectorConfig[field.name]"
              :type="
                field.sensitive ||
                field.name.toLowerCase().includes('password')
                  ? 'password'
                  : 'text'
              "
              :placeholder="
                mode === 'edit'
                  ? field.sensitive &&
                    connectorConfig[field.name] === '***MASKED***'
                    ? 'Leave empty to keep current value'
                    : field.sensitive
                      ? 'Enter new sensitive value...'
                      : ''
                  : field.sensitive
                    ? 'Enter sensitive value...'
                    : ''
              "
            />

            <!-- Number input -->
            <Input
              v-else-if="field.type === 'int' || field.type === 'number'"
              :id="`config-${field.name}`"
              v-model.number="connectorConfig[field.name]"
              type="number"
            />

            <!-- Boolean switch -->
            <div
              v-else-if="field.type === 'bool'"
              class="flex items-center space-x-2"
            >
              <Switch
                :id="`config-${field.name}`"
                :model-value="connectorConfig[field.name]"
                @update:model-value="connectorConfig[field.name] = $event"
              />
              <Label :for="`config-${field.name}`">{{ field.name }}</Label>
            </div>

            <!-- Unsupported field type fallback -->
            <p v-else class="text-xs text-yellow-700 dark:text-yellow-400">
              Unsupported field type "{{ field.type }}". This connector cannot
              be configured from the UI; edit it via API or contact your
              administrator.
            </p>
          </div>
        </div>
      </div>

      <DialogFooter>
        <Button variant="outline" @click="open = false">Cancel</Button>
        <Button @click="submit">
          {{ mode === "edit" ? "Save Configuration" : "Create Connector" }}
        </Button>
      </DialogFooter>
    </DialogContent>
  </Dialog>
</template>
