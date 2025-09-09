<script setup lang="ts">
import { ref, onMounted } from "vue";
import { Button } from "~/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "~/components/ui/card";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "~/components/ui/table";
import { Badge } from "~/components/ui/badge";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
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
import { useAuthStore } from "~/stores/auth";
import { 
  Settings, 
  Plus, 
  Trash2, 
  AlertCircle,
  CheckCircle,
  Clock,
  Wifi,
  WifiOff
} from "lucide-vue-next";

definePageMeta({
  middleware: ["auth"],
});

const authStore = useAuthStore();

// State variables
const loading = ref(true);
const connectors = ref<any[]>([]);
const availableConnectors = ref<any[]>([]);
const selectedConnector = ref<string>("");
const showAddDialog = ref(false);
const showConfigDialog = ref(false);
const currentConnector = ref<any>(null);
const connectorConfig = ref<Record<string, any>>({});

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
  try {
    const response = await $fetch("/api/connectors", {
      headers: {
        Authorization: `Bearer ${authStore.token}`,
      },
    });
    connectors.value = response.data.connectors || [];
  } catch (error: any) {
    console.error("Failed to load connectors:", error);
    toast({
      title: "Error",
      description: "Failed to load connectors",
      variant: "destructive",
    });
  }
};

// Load available connector definitions
const loadAvailableConnectors = async () => {
  try {
    const response = await $fetch("/api/connectors/available", {
      headers: {
        Authorization: `Bearer ${authStore.token}`,
      },
    });
    availableConnectors.value = response.data.connectors || [];
    console.log("Available connectors:", availableConnectors.value);
  } catch (error: any) {
    console.error("Failed to load available connectors:", error);
    console.error("Error details:", error.data);
    toast({
      title: "Error",
      description: error.data?.message || "Failed to load available connectors",
      variant: "destructive",
    });
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
    await $fetch("/api/connectors", {
      method: "POST",
      headers: {
        Authorization: `Bearer ${authStore.token}`,
      },
      body: {
        connector_id: selectedConnector.value,
        config: connectorConfig.value,
      },
    });

    toast({
      title: "Success",
      description: "Connector created successfully",
    });

    showAddDialog.value = false;
    selectedConnector.value = "";
    connectorConfig.value = {};
    await loadConnectors();
  } catch (error: any) {
    console.error("Failed to create connector:", error);
    toast({
      title: "Error",
      description: error.data?.message || "Failed to create connector",
      variant: "destructive",
    });
  }
};

// Delete connector instance
const deleteConnector = async (connector: any) => {
  if (!confirm(`Are you sure you want to delete the connector "${connector.id}"?`)) {
    return;
  }

  try {
    await $fetch(`/api/connectors/${connector.id}`, {
      method: "DELETE",
      headers: {
        Authorization: `Bearer ${authStore.token}`,
      },
    });

    toast({
      title: "Success",
      description: "Connector deleted successfully",
    });

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

// Configure connector
const configureConnector = (connector: any) => {
  currentConnector.value = connector;
  // Copy config and initialize with schema-aware defaults
  const config = JSON.parse(JSON.stringify(connector.config || {}));
  const connectorDef = availableConnectors.value.find(c => c.id === connector.id);
  
  if (connectorDef?.config_schema?.fields) {
    initializeConfigFromSchema(config, connectorDef.config_schema.fields);
  }
  
  connectorConfig.value = config;
  showConfigDialog.value = true;
};

// Initialize config values based on schema
const initializeConfigFromSchema = (config: any, fields: any[]) => {
  fields.forEach((field: any) => {
    if (field.type === 'bool') {
      if (config[field.name] !== undefined) {
        if (typeof config[field.name] === 'string') {
          config[field.name] = config[field.name] === 'true' || config[field.name] === '1';
        } else {
          config[field.name] = Boolean(config[field.name]);
        }
      } else {
        config[field.name] = field.default !== undefined ? Boolean(field.default) : false;
      }
    } else if (field.sensitive && config[field.name] === '***MASKED***') {
      config[field.name] = '';
    } else if (field.type === 'arraystring') {
      if (config[field.name] && !Array.isArray(config[field.name])) {
        if (typeof config[field.name] === 'string') {
          config[field.name] = config[field.name].split(',').map((s: string) => s.trim()).filter((s: string) => s.length > 0);
        }
      } else if (!config[field.name]) {
        config[field.name] = field.default || [];
      }
    } else if (field.type === 'arrayint') {
      if (config[field.name] && !Array.isArray(config[field.name])) {
        if (typeof config[field.name] === 'string') {
          config[field.name] = config[field.name].split(',').map((s: string) => parseInt(s.trim())).filter((n: number) => !isNaN(n));
        }
      } else if (!config[field.name]) {
        config[field.name] = field.default || [];
      }
    } else if (field.type === 'arraybool') {
      if (!config[field.name]) {
        config[field.name] = field.default || [];
      }
    } else if (field.type === 'arrayobject') {
      if (!config[field.name]) {
        config[field.name] = field.default || [];
      }
      // Initialize nested objects in array
      if (Array.isArray(config[field.name]) && field.nested && field.nested.length > 0) {
        config[field.name].forEach((item: any) => {
          if (typeof item === 'object' && item !== null) {
            initializeConfigFromSchema(item, field.nested);
          }
        });
      }
    } else if (field.type === 'object') {
      if (!config[field.name]) {
        config[field.name] = field.default || {};
      }
      if (field.nested && field.nested.length > 0) {
        initializeConfigFromSchema(config[field.name], field.nested);
      }
    } else if (field.type === 'int') {
      if (config[field.name] === undefined) {
        config[field.name] = field.default !== undefined ? field.default : 0;
      }
    } else if (field.type === 'string') {
      if (config[field.name] === undefined) {
        config[field.name] = field.default !== undefined ? field.default : '';
      }
    }
  });
};

// Save connector configuration
const saveConnectorConfig = async () => {
  if (!currentConnector.value) return;

  try {
    await $fetch(`/api/connectors/${currentConnector.value.id}`, {
      method: "PUT",
      headers: {
        Authorization: `Bearer ${authStore.token}`,
      },
      body: {
        config: connectorConfig.value,
      },
    });

    toast({
      title: "Success",
      description: "Connector configuration updated successfully",
    });

    showConfigDialog.value = false;
    currentConnector.value = null;
    await loadConnectors();
  } catch (error: any) {
    console.error("Failed to update connector config:", error);
    toast({
      title: "Error",
      description: error.data?.message || "Failed to update connector configuration",
      variant: "destructive",
    });
  }
};

// Handle connector selection change
const onConnectorSelect = (connectorId: any) => {
  selectedConnector.value = connectorId || "";
  const connector = availableConnectors.value.find(c => c.id === connectorId);
  if (connector?.config_schema?.fields) {
    // Initialize config with schema-aware defaults
    connectorConfig.value = {};
    initializeConfigFromSchema(connectorConfig.value, connector.config_schema.fields);
  }
};

onMounted(async () => {
  loading.value = true;
  try {
    await Promise.all([loadConnectors(), loadAvailableConnectors()]);
  } finally {
    loading.value = false;
  }
});
</script>

<template>
  <div class="p-4">
    <div class="flex flex-col sm:flex-row sm:items-center justify-between gap-4 mb-4">
      <div>
        <h1 class="text-3xl font-bold">Global Connectors</h1>
        <p class="text-muted-foreground">
          Manage global service connectors (Discord, Slack, etc.)
        </p>
      </div>
      <Dialog v-model:open="showAddDialog">
        <DialogTrigger as-child>
          <Button>
            <Plus class="w-4 h-4 mr-2" />
            Add Connector
          </Button>
        </DialogTrigger>
        <DialogContent class="max-w-2xl">
          <DialogHeader>
            <DialogTitle>Add New Connector</DialogTitle>
            <DialogDescription>
              Select a connector type and configure its settings.
            </DialogDescription>
          </DialogHeader>
          
          <div class="space-y-4">
            <div>
              <Label for="connector-select">Connector Type</Label>
              <Select :model-value="selectedConnector" @update:model-value="onConnectorSelect">
                <SelectTrigger>
                  <SelectValue placeholder="Select a connector type" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem 
                    v-for="connector in availableConnectors" 
                    :key="connector.id" 
                    :value="connector.id"
                  >
                    <div class="flex flex-col">
                      <span class="font-medium">{{ connector.name }}</span>
                      <span class="text-sm text-muted-foreground">{{ connector.description }}</span>
                    </div>
                  </SelectItem>
                </SelectContent>
              </Select>
            </div>

            <!-- Dynamic configuration fields -->
            <div v-if="selectedConnector" class="space-y-4">
              <h4 class="font-medium">Configuration</h4>
              <div 
                v-for="field in availableConnectors.find(c => c.id === selectedConnector)?.config_schema?.fields || []"
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
                  :type="field.sensitive || field.name.toLowerCase().includes('password') ? 'password' : 'text'"
                  :placeholder="field.sensitive ? 'Enter sensitive value...' : ''"
                />
                
                <!-- Number input -->
                <Input
                  v-else-if="field.type === 'int' || field.type === 'number'"
                  :id="`config-${field.name}`"
                  v-model.number="connectorConfig[field.name]"
                  type="number"
                />
                
                <!-- Boolean switch -->
                <div v-else-if="field.type === 'bool'" class="flex items-center space-x-2">
                  <Switch
                    :id="`config-${field.name}`"
                    :model-value="connectorConfig[field.name]"
                    @update:model-value="connectorConfig[field.name] = $event"
                  />
                  <Label :for="`config-${field.name}`">{{ field.name }}</Label>
                </div>
              </div>
            </div>
          </div>

          <DialogFooter>
            <Button variant="outline" @click="showAddDialog = false">Cancel</Button>
            <Button @click="createConnector">Create Connector</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>

    <!-- Connectors List -->
    <Card>
      <CardHeader>
        <CardTitle>Active Connectors</CardTitle>
        <CardDescription>
          Global service connectors available to all plugins
        </CardDescription>
      </CardHeader>
      <CardContent>
        <div v-if="loading" class="flex items-center justify-center py-8">
          <div class="animate-spin rounded-full h-8 w-8 border-b-2 border-primary"></div>
        </div>
        
        <div v-else-if="connectors.length === 0" class="text-center py-8">
          <p class="text-muted-foreground">No connectors configured</p>
          <div class="mt-4 p-4 bg-muted rounded-lg text-sm">
            <p class="font-medium mb-2">Debug Information:</p>
            <p>Available connector types: {{ availableConnectors.length }}</p>
            <p v-if="availableConnectors.length > 0">
              Types: {{ availableConnectors.map(c => c.id).join(', ') }}
            </p>
            <p v-else class="text-red-600">
              No connector types available - check server logs for registration errors
            </p>
          </div>
        </div>
        
        <Table v-else>
          <TableHeader>
            <TableRow>
              <TableHead>Type</TableHead>
              <TableHead>Name</TableHead>
              <TableHead>Status</TableHead>
              <TableHead>Enabled</TableHead>
              <TableHead>Last Error</TableHead>
              <TableHead class="text-right">Actions</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            <TableRow v-for="connector in connectors" :key="connector.id">
              <TableCell>
                <div class="flex items-center space-x-2">
                  <component :is="getStatusIcon(connector.status)" class="w-4 h-4" />
                  <span class="font-medium">{{ connector.id }}</span>
                </div>
              </TableCell>
              <TableCell>
                <div class="flex flex-col">
                  <span>{{ availableConnectors.find(c => c.id === connector.id)?.name || connector.id }}</span>
                  <span class="text-sm text-muted-foreground">
                    {{ availableConnectors.find(c => c.id === connector.id)?.description }}
                  </span>
                </div>
              </TableCell>
              <TableCell>
                <Badge :class="getStatusColor(connector.status)">
                  <component :is="getStatusIcon(connector.status)" class="w-3 h-3 mr-1" />
                  {{ connector.status }}
                </Badge>
              </TableCell>
              <TableCell>
                <Switch :model-value="connector.enabled" disabled />
              </TableCell>
              <TableCell>
                <span 
                  v-if="connector.last_error" 
                  class="text-sm text-red-600 dark:text-red-400"
                  :title="connector.last_error"
                >
                  {{ connector.last_error.length > 50 ? connector.last_error.substring(0, 50) + '...' : connector.last_error }}
                </span>
                <span v-else class="text-muted-foreground">None</span>
              </TableCell>
              <TableCell class="text-right">
                <div class="flex items-center justify-end space-x-2">
                  <Button
                    variant="outline"
                    size="sm"
                    @click="configureConnector(connector)"
                  >
                    <Settings class="w-4 h-4" />
                  </Button>
                  <Button
                    variant="destructive"
                    size="sm"
                    @click="deleteConnector(connector)"
                  >
                    <Trash2 class="w-4 h-4" />
                  </Button>
                </div>
              </TableCell>
            </TableRow>
          </TableBody>
        </Table>
      </CardContent>
    </Card>

    <!-- Configuration Dialog -->
    <Dialog v-model:open="showConfigDialog">
      <DialogContent class="max-w-2xl">
        <DialogHeader>
          <DialogTitle>Configure {{ currentConnector?.id }} Connector</DialogTitle>
          <DialogDescription>
            Update the configuration for this connector.
          </DialogDescription>
        </DialogHeader>
        
        <div v-if="currentConnector" class="space-y-4">
          <div 
            v-for="field in availableConnectors.find(c => c.id === currentConnector.id)?.config_schema?.fields || []"
            :key="field.name"
            class="space-y-2"
          >
            <Label :for="`edit-config-${field.name}`">
              {{ field.name }}
              <span v-if="field.required" class="text-red-500">*</span>
            </Label>
            <p v-if="field.description" class="text-sm text-muted-foreground">
              {{ field.description }}
            </p>
            
            <!-- String/Text input -->
            <Input
              v-if="field.type === 'string'"
              :id="`edit-config-${field.name}`"
              v-model="connectorConfig[field.name]"
              :type="field.sensitive || field.name.toLowerCase().includes('password') ? 'password' : 'text'"
              :placeholder="field.sensitive && connectorConfig[field.name] === '***MASKED***' ? 'Leave empty to keep current value' : field.sensitive ? 'Enter new sensitive value...' : ''"
            />
            
            <!-- Number input -->
            <Input
              v-else-if="field.type === 'int' || field.type === 'number'"
              :id="`edit-config-${field.name}`"
              v-model.number="connectorConfig[field.name]"
              type="number"
            />
            
            <!-- Boolean switch -->
            <div v-else-if="field.type === 'bool'" class="flex items-center space-x-2">
              <Switch
                :id="`edit-config-${field.name}`"
                :model-value="connectorConfig[field.name]"
                @update:model-value="connectorConfig[field.name] = $event"
              />
              <Label :for="`edit-config-${field.name}`">{{ field.name }}</Label>
            </div>
          </div>
        </div>

        <DialogFooter>
          <Button variant="outline" @click="showConfigDialog = false">Cancel</Button>
          <Button @click="saveConnectorConfig">Save Configuration</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  </div>
</template>
