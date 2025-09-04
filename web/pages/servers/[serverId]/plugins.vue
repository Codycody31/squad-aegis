<script setup lang="ts">
import { ref, onMounted, computed } from "vue";
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
import { Textarea } from "~/components/ui/textarea";
import { Switch } from "~/components/ui/switch";
import { toast } from "~/components/ui/toast";
import { useAuthStore } from "~/stores/auth";
import { 
  Settings, 
  Plus, 
  Play, 
  Pause, 
  Trash2, 
  BarChart3,
  FileText,
  AlertCircle,
  CheckCircle,
  Clock
} from "lucide-vue-next";

definePageMeta({
  middleware: ["auth"],
});

const route = useRoute();
const serverId = route.params.serverId;
const authStore = useAuthStore();

// State variables
const loading = ref(true);
const plugins = ref<any[]>([]);
const availablePlugins = ref<any[]>([]);
const selectedPlugin = ref<string>("");
const showAddDialog = ref(false);
const showConfigDialog = ref(false);
const currentPlugin = ref<any>(null);
const pluginConfig = ref<Record<string, any>>({});
const pluginName = ref("");

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
    default:
      return CheckCircle;
  }
};

// Load plugins for this server
const loadPlugins = async () => {
  try {
    const response = await $fetch(`/api/servers/${serverId}/plugins`, {
      headers: {
        Authorization: `Bearer ${authStore.token}`,
      },
    });
    plugins.value = response.data.plugins || [];
  } catch (error: any) {
    console.error("Failed to load plugins:", error);
    toast({
      title: "Error",
      description: "Failed to load plugins",
      variant: "destructive",
    });
  }
};

// Load available plugin definitions
const loadAvailablePlugins = async () => {
  try {
    const response = await $fetch("/api/plugins/available", {
      headers: {
        Authorization: `Bearer ${authStore.token}`,
      },
    });
    availablePlugins.value = response.data.plugins || [];
  } catch (error: any) {
    console.error("Failed to load available plugins:", error);
    toast({
      title: "Error",
      description: "Failed to load available plugins",
      variant: "destructive",
    });
  }
};

// Create new plugin instance
const createPlugin = async () => {
  if (!selectedPlugin.value || !pluginName.value) {
    toast({
      title: "Error",
      description: "Please select a plugin and enter a name",
      variant: "destructive",
    });
    return;
  }

  try {
    await $fetch(`/api/servers/${serverId}/plugins`, {
      method: "POST",
      headers: {
        Authorization: `Bearer ${authStore.token}`,
      },
      body: {
        plugin_id: selectedPlugin.value,
        name: pluginName.value,
        config: pluginConfig.value,
      },
    });

    toast({
      title: "Success",
      description: "Plugin created successfully",
    });

    showAddDialog.value = false;
    selectedPlugin.value = "";
    pluginName.value = "";
    pluginConfig.value = {};
    await loadPlugins();
  } catch (error: any) {
    console.error("Failed to create plugin:", error);
    toast({
      title: "Error",
      description: error.data?.message || "Failed to create plugin",
      variant: "destructive",
    });
  }
};

// Toggle plugin enabled/disabled
const togglePlugin = async (plugin: any) => {
  const action = plugin.enabled ? "disable" : "enable";
  
  try {
    await $fetch(`/api/servers/${serverId}/plugins/${plugin.id}/${action}`, {
      method: "POST",
      headers: {
        Authorization: `Bearer ${authStore.token}`,
      },
    });

    toast({
      title: "Success",
      description: `Plugin ${action}d successfully`,
    });

    await loadPlugins();
  } catch (error: any) {
    console.error(`Failed to ${action} plugin:`, error);
    toast({
      title: "Error",
      description: error.data?.message || `Failed to ${action} plugin`,
      variant: "destructive",
    });
  }
};

// Delete plugin instance
const deletePlugin = async (plugin: any) => {
  if (!confirm(`Are you sure you want to delete the plugin "${plugin.name}"?`)) {
    return;
  }

  try {
    await $fetch(`/api/servers/${serverId}/plugins/${plugin.id}`, {
      method: "DELETE",
      headers: {
        Authorization: `Bearer ${authStore.token}`,
      },
    });

    toast({
      title: "Success",
      description: "Plugin deleted successfully",
    });

    await loadPlugins();
  } catch (error: any) {
    console.error("Failed to delete plugin:", error);
    toast({
      title: "Error",
      description: error.data?.message || "Failed to delete plugin",
      variant: "destructive",
    });
  }
};

// Configure plugin
const configurePlugin = (plugin: any) => {
  currentPlugin.value = plugin;
  pluginConfig.value = { ...plugin.config };
  showConfigDialog.value = true;
};

// Save plugin configuration
const savePluginConfig = async () => {
  if (!currentPlugin.value) return;

  try {
    await $fetch(`/api/servers/${serverId}/plugins/${currentPlugin.value.id}`, {
      method: "PUT",
      headers: {
        Authorization: `Bearer ${authStore.token}`,
      },
      body: {
        config: pluginConfig.value,
      },
    });

    toast({
      title: "Success",
      description: "Plugin configuration updated successfully",
    });

    showConfigDialog.value = false;
    currentPlugin.value = null;
    await loadPlugins();
  } catch (error: any) {
    console.error("Failed to update plugin config:", error);
    toast({
      title: "Error",
      description: error.data?.message || "Failed to update plugin configuration",
      variant: "destructive",
    });
  }
};

// Handle plugin selection change
const onPluginSelect = (pluginId: string) => {
  selectedPlugin.value = pluginId;
  const plugin = availablePlugins.value.find(p => p.id === pluginId);
  if (plugin?.config_schema?.fields) {
    // Initialize config with default values
    pluginConfig.value = {};
    plugin.config_schema.fields.forEach((field: any) => {
      if (field.default !== undefined) {
        pluginConfig.value[field.name] = field.default;
      }
    });
  }
};

// Render config field based on type
const renderConfigField = (field: any) => {
  switch (field.type) {
    case "string":
      return "text";
    case "int":
    case "number":
      return "number";
    case "bool":
      return "checkbox";
    default:
      return "text";
  }
};

onMounted(async () => {
  loading.value = true;
  try {
    await Promise.all([loadPlugins(), loadAvailablePlugins()]);
  } finally {
    loading.value = false;
  }
});
</script>

<template>
  <div class="container mx-auto py-6">
    <div class="flex items-center justify-between mb-6">
      <div>
        <h1 class="text-3xl font-bold">Server Plugins</h1>
        <p class="text-muted-foreground">
          Manage plugins for this server
        </p>
      </div>
      <Dialog v-model:open="showAddDialog">
        <DialogTrigger as-child>
          <Button>
            <Plus class="w-4 h-4 mr-2" />
            Add Plugin
          </Button>
        </DialogTrigger>
        <DialogContent class="max-w-2xl">
          <DialogHeader>
            <DialogTitle>Add New Plugin</DialogTitle>
            <DialogDescription>
              Select a plugin to add to this server and configure its settings.
            </DialogDescription>
          </DialogHeader>
          
          <div class="space-y-4">
            <div>
              <Label for="plugin-select">Plugin</Label>
              <Select :model-value="selectedPlugin" @update:model-value="onPluginSelect">
                <SelectTrigger>
                  <SelectValue placeholder="Select a plugin" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem 
                    v-for="plugin in availablePlugins" 
                    :key="plugin.id" 
                    :value="plugin.id"
                  >
                    <div class="flex flex-col">
                      <span class="font-medium">{{ plugin.name }}</span>
                      <span class="text-sm text-muted-foreground">{{ plugin.description }}</span>
                    </div>
                  </SelectItem>
                </SelectContent>
              </Select>
            </div>

            <div>
              <Label for="plugin-name">Instance Name</Label>
              <Input 
                id="plugin-name"
                v-model="pluginName" 
                placeholder="Enter a name for this plugin instance"
              />
            </div>

            <!-- Dynamic configuration fields -->
            <div v-if="selectedPlugin" class="space-y-4">
              <h4 class="font-medium">Configuration</h4>
              <div 
                v-for="field in availablePlugins.find(p => p.id === selectedPlugin)?.config_schema?.fields || []"
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
                  v-model="pluginConfig[field.name]"
                  :type="renderConfigField(field)"
                />
                
                <!-- Number input -->
                <Input
                  v-else-if="field.type === 'int' || field.type === 'number'"
                  :id="`config-${field.name}`"
                  v-model.number="pluginConfig[field.name]"
                  type="number"
                />
                
                <!-- Boolean switch -->
                <div v-else-if="field.type === 'bool'" class="flex items-center space-x-2">
                  <Switch
                    :id="`config-${field.name}`"
                    :checked="pluginConfig[field.name]"
                    @update:checked="pluginConfig[field.name] = $event"
                  />
                  <Label :for="`config-${field.name}`">{{ field.name }}</Label>
                </div>
                
                <!-- Array of strings -->
                <Textarea
                  v-else-if="field.type === 'array_string'"
                  :id="`config-${field.name}`"
                  v-model="pluginConfig[field.name]"
                  placeholder="Enter values separated by commas"
                  @input="pluginConfig[field.name] = $event.target.value.split(',').map(s => s.trim())"
                />
              </div>
            </div>
          </div>

          <DialogFooter>
            <Button variant="outline" @click="showAddDialog = false">Cancel</Button>
            <Button @click="createPlugin">Create Plugin</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>

    <!-- Plugins List -->
    <Card>
      <CardHeader>
        <CardTitle>Active Plugins</CardTitle>
        <CardDescription>
          Plugins currently configured for this server
        </CardDescription>
      </CardHeader>
      <CardContent>
        <div v-if="loading" class="flex items-center justify-center py-8">
          <div class="animate-spin rounded-full h-8 w-8 border-b-2 border-primary"></div>
        </div>
        
        <div v-else-if="plugins.length === 0" class="text-center py-8">
          <p class="text-muted-foreground">No plugins configured for this server</p>
        </div>
        
        <Table v-else>
          <TableHeader>
            <TableRow>
              <TableHead>Name</TableHead>
              <TableHead>Plugin</TableHead>
              <TableHead>Status</TableHead>
              <TableHead>Enabled</TableHead>
              <TableHead>Last Error</TableHead>
              <TableHead class="text-right">Actions</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            <TableRow v-for="plugin in plugins" :key="plugin.id">
              <TableCell class="font-medium">{{ plugin.name }}</TableCell>
              <TableCell>
                <div class="flex flex-col">
                  <span>{{ plugin.plugin_id }}</span>
                  <span class="text-sm text-muted-foreground">
                    {{ availablePlugins.find(p => p.id === plugin.plugin_id)?.description }}
                  </span>
                </div>
              </TableCell>
              <TableCell>
                <Badge :class="getStatusColor(plugin.status)">
                  <component :is="getStatusIcon(plugin.status)" class="w-3 h-3 mr-1" />
                  {{ plugin.status }}
                </Badge>
              </TableCell>
              <TableCell>
                <Switch
                  :checked="plugin.enabled"
                  @update:checked="togglePlugin(plugin)"
                />
              </TableCell>
              <TableCell>
                <span 
                  v-if="plugin.last_error" 
                  class="text-sm text-red-600 dark:text-red-400"
                  :title="plugin.last_error"
                >
                  {{ plugin.last_error.length > 50 ? plugin.last_error.substring(0, 50) + '...' : plugin.last_error }}
                </span>
                <span v-else class="text-muted-foreground">None</span>
              </TableCell>
              <TableCell class="text-right">
                <div class="flex items-center justify-end space-x-2">
                  <Button
                    variant="outline"
                    size="sm"
                    @click="configurePlugin(plugin)"
                  >
                    <Settings class="w-4 h-4" />
                  </Button>
                  <Button
                    variant="outline"
                    size="sm"
                    @click="$router.push(`/servers/${serverId}/plugins/${plugin.id}/metrics`)"
                  >
                    <BarChart3 class="w-4 h-4" />
                  </Button>
                  <Button
                    variant="outline"
                    size="sm"
                    @click="$router.push(`/servers/${serverId}/plugins/${plugin.id}/logs`)"
                  >
                    <FileText class="w-4 h-4" />
                  </Button>
                  <Button
                    variant="destructive"
                    size="sm"
                    @click="deletePlugin(plugin)"
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
          <DialogTitle>Configure {{ currentPlugin?.name }}</DialogTitle>
          <DialogDescription>
            Update the configuration for this plugin instance.
          </DialogDescription>
        </DialogHeader>
        
        <div v-if="currentPlugin" class="space-y-4">
          <div 
            v-for="field in availablePlugins.find(p => p.id === currentPlugin.plugin_id)?.config_schema?.fields || []"
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
              v-model="pluginConfig[field.name]"
              type="text"
            />
            
            <!-- Number input -->
            <Input
              v-else-if="field.type === 'int' || field.type === 'number'"
              :id="`edit-config-${field.name}`"
              v-model.number="pluginConfig[field.name]"
              type="number"
            />
            
            <!-- Boolean switch -->
            <div v-else-if="field.type === 'bool'" class="flex items-center space-x-2">
              <Switch
                :id="`edit-config-${field.name}`"
                :checked="pluginConfig[field.name]"
                @update:checked="pluginConfig[field.name] = $event"
              />
              <Label :for="`edit-config-${field.name}`">{{ field.name }}</Label>
            </div>
            
            <!-- Array of strings -->
            <Textarea
              v-else-if="field.type === 'array_string'"
              :id="`edit-config-${field.name}`"
              :model-value="Array.isArray(pluginConfig[field.name]) ? pluginConfig[field.name].join(', ') : pluginConfig[field.name]"
              placeholder="Enter values separated by commas"
              @input="pluginConfig[field.name] = $event.target.value.split(',').map(s => s.trim())"
            />
          </div>
        </div>

        <DialogFooter>
          <Button variant="outline" @click="showConfigDialog = false">Cancel</Button>
          <Button @click="savePluginConfig">Save Configuration</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  </div>
</template>
