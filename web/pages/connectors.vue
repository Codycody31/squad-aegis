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
  WifiOff,
  Upload,
  RefreshCw,
} from "lucide-vue-next";

type ConnectorPackage = {
  connector_id: string;
  name: string;
  description?: string;
  version: string;
  source: string;
  distribution?: string;
  official?: boolean;
  install_state: string;
  min_host_api_version?: number;
  required_capabilities?: string[];
  last_error?: string;
  unsafe?: boolean;
};

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

const connectorPackages = ref<ConnectorPackage[]>([]);
const loadingPackages = ref(false);
const uploadingPackage = ref(false);
const selectedConnectorBundle = ref<File | null>(null);

/** Match a running instance id (e.g. `discord`) to an available definition (canonical or legacy). */
const connectorDefForInstance = (instanceId: string) => {
  return availableConnectors.value.find(
    (c: any) =>
      c.id === instanceId ||
      (Array.isArray(c.legacy_ids) && c.legacy_ids.includes(instanceId)),
  );
};

const getPackageStateVariant = (state: string) => {
  switch (state) {
    case "ready":
      return "default";
    case "pending_restart":
      return "secondary";
    case "error":
      return "destructive";
    default:
      return "outline";
  }
};

const getConnectorSourceLabel = (pkg: ConnectorPackage) => {
  if (pkg.source === "bundled") return "Bundled";
  return "Sideload Native";
};

const formatConnectorRuntime = (pkg: ConnectorPackage) => {
  const parts: string[] = [];
  if (pkg.min_host_api_version) {
    parts.push(`API >= ${pkg.min_host_api_version}`);
  }
  if (pkg.required_capabilities?.length) {
    parts.push(pkg.required_capabilities.join(", "));
  }
  return parts.join(" · ") || "—";
};

const refreshAll = async () => {
  loading.value = true;
  try {
    await Promise.all([loadConnectors(), loadAvailableConnectors(), loadConnectorPackages()]);
  } finally {
    loading.value = false;
  }
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
  try {
    const response = await useAuthFetchImperative("/api/connectors");
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
    const response = await useAuthFetchImperative("/api/connectors/available");
    availableConnectors.value = response.data.connectors || [];
  } catch (error: any) {
    console.error("Failed to load available connectors:", error);
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
    await useAuthFetchImperative(`/api/connectors/${connector.id}`, {
      method: "DELETE",
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
  const connectorDef = connectorDefForInstance(connector.id);
  
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
    await useAuthFetchImperative(`/api/connectors/${currentConnector.value.id}`, {
      method: "PUT",
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

const loadConnectorPackages = async () => {
  loadingPackages.value = true;
  try {
    const response = await useAuthFetchImperative<any>("/api/connectors/packages/installed");
    connectorPackages.value = response.data?.connectors || [];
  } catch (error: any) {
    console.error("Failed to load connector packages:", error);
    toast({
      title: "Error",
      description: error.data?.message || "Failed to load installed connector packages",
      variant: "destructive",
    });
  } finally {
    loadingPackages.value = false;
  }
};

const onConnectorBundleSelected = (event: Event) => {
  const input = event.target as HTMLInputElement;
  selectedConnectorBundle.value = input.files?.[0] || null;
};

const uploadConnectorBundle = async () => {
  if (!selectedConnectorBundle.value) {
    toast({
      title: "Missing bundle",
      description: "Choose a connector .zip bundle to upload",
      variant: "destructive",
    });
    return;
  }

  uploadingPackage.value = true;
  try {
    const formData = new FormData();
    formData.append("bundle", selectedConnectorBundle.value);

    const response = await useAuthFetchImperative<any>("/api/connectors/packages/upload", {
      method: "POST",
      body: formData,
    });

    toast({
      title: "Uploaded",
      description: response.message || "Connector package uploaded successfully",
    });

    selectedConnectorBundle.value = null;
    await Promise.all([loadConnectorPackages(), loadAvailableConnectors()]);
  } catch (error: any) {
    console.error("Failed to upload connector bundle:", error);
    toast({
      title: "Error",
      description: error.data?.message || "Failed to upload connector bundle",
      variant: "destructive",
    });
  } finally {
    uploadingPackage.value = false;
  }
};

const deleteConnectorPackage = async (pkg: ConnectorPackage) => {
  if (
    !confirm(
      `Delete sideloaded connector package "${pkg.name}" (${pkg.connector_id})? Remove any running connector instance for this id first.`,
    )
  ) {
    return;
  }

  try {
    await useAuthFetchImperative(`/api/connectors/packages/installed/${encodeURIComponent(pkg.connector_id)}`, {
      method: "DELETE",
    });
    toast({
      title: "Deleted",
      description: "Connector package removed",
    });
    await Promise.all([loadConnectorPackages(), loadAvailableConnectors()]);
  } catch (error: any) {
    console.error("Failed to delete connector package:", error);
    toast({
      title: "Error",
      description: error.data?.message || "Failed to delete connector package",
      variant: "destructive",
    });
  }
};

onMounted(async () => {
  loading.value = true;
  try {
    await Promise.all([loadConnectors(), loadAvailableConnectors(), loadConnectorPackages()]);
  } finally {
    loading.value = false;
  }
});
</script>

<template>
  <div class="p-3 sm:p-4">
    <div class="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-3 sm:gap-0 mb-3 sm:mb-4">
      <div>
        <h1 class="text-xl sm:text-2xl lg:text-3xl font-bold">Global Connectors</h1>
        <p class="text-xs sm:text-sm text-muted-foreground mt-1">
          Manage global service connectors (Discord, Slack, etc.)
        </p>
      </div>
      <div class="flex flex-col sm:flex-row gap-2 w-full sm:w-auto">
        <Button variant="outline" class="w-full sm:w-auto text-sm sm:text-base" @click="refreshAll">
          <RefreshCw class="w-4 h-4 mr-2" />
          Refresh
        </Button>
        <Dialog v-model:open="showAddDialog">
        <DialogTrigger as-child>
          <Button class="w-full sm:w-auto text-sm sm:text-base">
            <Plus class="w-4 h-4 mr-2" />
            Add Connector
          </Button>
        </DialogTrigger>
        <DialogContent class="w-[95vw] sm:max-w-2xl max-h-[90vh] overflow-y-auto p-4 sm:p-6">
          <DialogHeader>
            <DialogTitle class="text-base sm:text-lg">Add New Connector</DialogTitle>
            <DialogDescription class="text-xs sm:text-sm">
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
    </div>

    <!-- Sideloaded connector packages (.so) -->
    <Card class="mb-4 sm:mb-6">
      <CardHeader class="pb-2 sm:pb-3">
        <CardTitle class="text-base sm:text-lg">Sideloaded connector packages</CardTitle>
        <CardDescription class="text-xs sm:text-sm">
          Upload a signed <code class="rounded bg-muted px-1 text-xs">.zip</code> with
          <code class="rounded bg-muted px-1 text-xs">manifest.json</code> and either a Linux
          <code class="rounded bg-muted px-1 text-xs">.so</code> package with entry
          <code class="rounded bg-muted px-1 text-xs">GetAegisConnector</code>. Then add a connector instance above if the connector needs config.
        </CardDescription>
      </CardHeader>
      <CardContent class="space-y-4">
        <div class="flex flex-col gap-3 md:flex-row md:items-center">
          <Input type="file" accept=".zip" @change="onConnectorBundleSelected" />
          <Button :disabled="uploadingPackage || !selectedConnectorBundle" @click="uploadConnectorBundle">
            <Upload class="w-4 h-4 mr-2" />
            {{ uploadingPackage ? "Uploading…" : "Upload bundle" }}
          </Button>
          <Button variant="ghost" size="sm" :disabled="loadingPackages" @click="loadConnectorPackages">
            <RefreshCw class="w-4 h-4 mr-1" />
            Packages
          </Button>
        </div>
        <p class="text-xs sm:text-sm text-muted-foreground">
          Unsigned bundles are rejected unless unsafe sideload is enabled on the server. After upload, restart Aegis if the package says pending restart.
        </p>

        <div v-if="loadingPackages" class="py-6 text-center text-muted-foreground text-sm">
          Loading installed packages…
        </div>
        <div v-else-if="connectorPackages.length === 0" class="text-sm text-muted-foreground py-2">
          No sideloaded connector packages installed.
        </div>
        <div v-else class="overflow-x-auto rounded-md border">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead class="text-xs sm:text-sm">Package</TableHead>
                <TableHead class="text-xs sm:text-sm">Source</TableHead>
                <TableHead class="text-xs sm:text-sm">Version</TableHead>
                <TableHead class="text-xs sm:text-sm">State</TableHead>
                <TableHead class="hidden lg:table-cell text-xs sm:text-sm">Runtime</TableHead>
                <TableHead class="hidden lg:table-cell text-xs sm:text-sm">Last error</TableHead>
                <TableHead class="text-right text-xs sm:text-sm">Actions</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              <TableRow v-for="pkg in connectorPackages" :key="pkg.connector_id">
                <TableCell>
                  <div class="flex flex-col max-w-[220px] sm:max-w-xs">
                    <span class="font-medium text-sm truncate">{{ pkg.name || pkg.connector_id }}</span>
                    <span class="text-xs text-muted-foreground truncate" :title="pkg.connector_id">{{ pkg.connector_id }}</span>
                  </div>
                </TableCell>
                <TableCell>
                  <div class="flex flex-wrap gap-1">
                    <Badge variant="outline" class="text-xs">{{ getConnectorSourceLabel(pkg) }}</Badge>
                    <Badge v-if="pkg.official" variant="default" class="text-xs">Official</Badge>
                    <Badge v-if="pkg.unsafe" variant="destructive" class="text-xs">Unsafe</Badge>
                  </div>
                </TableCell>
                <TableCell class="text-sm">{{ pkg.version || "—" }}</TableCell>
                <TableCell>
                  <Badge :variant="getPackageStateVariant(pkg.install_state)" class="text-xs capitalize">
                    {{ String(pkg.install_state || "").replace(/_/g, " ") }}
                  </Badge>
                </TableCell>
                <TableCell class="hidden lg:table-cell text-xs text-muted-foreground">
                  {{ formatConnectorRuntime(pkg) }}
                </TableCell>
                <TableCell class="hidden lg:table-cell">
                  <span v-if="pkg.last_error" class="text-xs text-destructive line-clamp-2">{{ pkg.last_error }}</span>
                  <span v-else class="text-xs text-muted-foreground">None</span>
                </TableCell>
                <TableCell class="text-right">
                  <Button
                    v-if="pkg.source === 'native'"
                    size="sm"
                    variant="destructive"
                    @click="deleteConnectorPackage(pkg)"
                  >
                    Delete
                  </Button>
                </TableCell>
              </TableRow>
            </TableBody>
          </Table>
        </div>
      </CardContent>
    </Card>

    <!-- Connectors List -->
    <Card>
      <CardHeader class="pb-2 sm:pb-3">
        <CardTitle class="text-base sm:text-lg">Active Connectors</CardTitle>
        <CardDescription class="text-xs sm:text-sm">
          Global service connectors available to all plugins
        </CardDescription>
      </CardHeader>
      <CardContent>
        <div v-if="loading" class="flex items-center justify-center py-6 sm:py-8">
          <div class="animate-spin rounded-full h-8 w-8 border-b-2 border-primary"></div>
        </div>
        
        <div v-else-if="connectors.length === 0" class="text-center py-6 sm:py-8">
          <p class="text-sm sm:text-base text-muted-foreground">No connector instances configured</p>
          <p class="text-xs text-muted-foreground mt-2">
            Use <span class="font-medium">Add Connector</span> to enable Discord or other registered connectors.
          </p>
        </div>
        
        <template v-else>
          <!-- Desktop Table View -->
          <div class="hidden md:block w-full overflow-x-auto">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead class="text-xs sm:text-sm">Type</TableHead>
                  <TableHead class="text-xs sm:text-sm">Name</TableHead>
                  <TableHead class="text-xs sm:text-sm">Status</TableHead>
                  <TableHead class="text-xs sm:text-sm">Enabled</TableHead>
                  <TableHead class="text-xs sm:text-sm">Last Error</TableHead>
                  <TableHead class="text-right text-xs sm:text-sm">Actions</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                <TableRow v-for="connector in connectors" :key="connector.id" class="hover:bg-muted/50">
                  <TableCell>
                    <div class="flex items-center space-x-2">
                      <component :is="getStatusIcon(connector.status)" class="w-4 h-4" />
                      <span class="font-medium text-sm sm:text-base">{{ connector.id }}</span>
                    </div>
                  </TableCell>
                  <TableCell>
                    <div class="flex flex-col">
                      <span class="text-sm sm:text-base">{{ connectorDefForInstance(connector.id)?.name || connector.id }}</span>
                      <span class="text-xs sm:text-sm text-muted-foreground">
                        {{ connectorDefForInstance(connector.id)?.description }}
                      </span>
                    </div>
                  </TableCell>
                  <TableCell>
                    <Badge :class="getStatusColor(connector.status)" class="text-xs">
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
                      class="text-xs sm:text-sm text-red-600 dark:text-red-400"
                      :title="connector.last_error"
                    >
                      {{ connector.last_error.length > 50 ? connector.last_error.substring(0, 50) + '...' : connector.last_error }}
                    </span>
                    <span v-else class="text-xs sm:text-sm text-muted-foreground">None</span>
                  </TableCell>
                  <TableCell class="text-right">
                    <div class="flex items-center justify-end space-x-2">
                      <Button
                        variant="outline"
                        size="sm"
                        @click="configureConnector(connector)"
                        class="text-xs"
                      >
                        <Settings class="w-4 h-4" />
                      </Button>
                      <Button
                        variant="destructive"
                        size="sm"
                        @click="deleteConnector(connector)"
                        class="text-xs"
                      >
                        <Trash2 class="w-4 h-4" />
                      </Button>
                    </div>
                  </TableCell>
                </TableRow>
              </TableBody>
            </Table>
          </div>

          <!-- Mobile Card View -->
          <div class="md:hidden space-y-3">
            <div
              v-for="connector in connectors"
              :key="connector.id"
              class="border rounded-lg p-3 sm:p-4 hover:bg-muted/30 transition-colors"
            >
              <div class="flex items-start justify-between gap-2 mb-2">
                <div class="flex-1 min-w-0">
                  <div class="flex items-center gap-2 mb-1">
                    <component :is="getStatusIcon(connector.status)" class="w-4 h-4" />
                    <span class="font-semibold text-sm sm:text-base">{{ connector.id }}</span>
                  </div>
                  <div class="space-y-1.5">
                    <div>
                      <span class="text-xs text-muted-foreground">Name: </span>
                      <span class="text-xs sm:text-sm">{{ connectorDefForInstance(connector.id)?.name || connector.id }}</span>
                    </div>
                    <div v-if="connectorDefForInstance(connector.id)?.description">
                      <span class="text-xs text-muted-foreground">Description: </span>
                      <span class="text-xs sm:text-sm">{{ connectorDefForInstance(connector.id)?.description }}</span>
                    </div>
                    <div class="flex items-center gap-2 mt-2">
                      <Badge :class="getStatusColor(connector.status)" class="text-xs">
                        <component :is="getStatusIcon(connector.status)" class="w-3 h-3 mr-1" />
                        {{ connector.status }}
                      </Badge>
                      <div class="flex items-center gap-1">
                        <Switch :model-value="connector.enabled" disabled />
                        <span class="text-xs text-muted-foreground">{{ connector.enabled ? 'Enabled' : 'Disabled' }}</span>
                      </div>
                    </div>
                    <div v-if="connector.last_error" class="mt-2">
                      <span class="text-xs text-muted-foreground">Last Error: </span>
                      <span class="text-xs text-red-600 dark:text-red-400 break-words">
                        {{ connector.last_error.length > 100 ? connector.last_error.substring(0, 100) + '...' : connector.last_error }}
                      </span>
                    </div>
                  </div>
                </div>
              </div>
              <div class="flex items-center justify-end gap-2 pt-2 border-t">
                <Button
                  variant="outline"
                  size="sm"
                  @click="configureConnector(connector)"
                  class="h-8 text-xs flex-1"
                >
                  <Settings class="w-3 h-3 mr-1" />
                  Configure
                </Button>
                <Button
                  variant="destructive"
                  size="sm"
                  @click="deleteConnector(connector)"
                  class="h-8 text-xs flex-1"
                >
                  <Trash2 class="w-3 h-3 mr-1" />
                  Delete
                </Button>
              </div>
            </div>
          </div>
        </template>
      </CardContent>
    </Card>

    <!-- Configuration Dialog -->
    <Dialog v-model:open="showConfigDialog">
      <DialogContent class="w-[95vw] sm:max-w-2xl max-h-[90vh] overflow-y-auto p-4 sm:p-6">
        <DialogHeader>
          <DialogTitle class="text-base sm:text-lg">Configure {{ currentConnector?.id }} Connector</DialogTitle>
          <DialogDescription class="text-xs sm:text-sm">
            Update the configuration for this connector.
          </DialogDescription>
        </DialogHeader>
        
        <div v-if="currentConnector" class="space-y-4">
          <div 
            v-for="field in connectorDefForInstance(currentConnector.id)?.config_schema?.fields || []"
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
