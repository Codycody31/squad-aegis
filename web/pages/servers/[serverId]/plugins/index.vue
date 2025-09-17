<script setup lang="ts">
import { ref, onMounted, computed, watch } from "vue";
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
import {
    Combobox,
    ComboboxAnchor,
    ComboboxEmpty,
    ComboboxInput,
    ComboboxItem,
    ComboboxList,
    ComboboxTrigger,
} from "~/components/ui/combobox";
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
    Clock,
    Check,
    ChevronsUpDown,
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

// Combobox state
const pluginSearchQuery = ref("");
const showPluginDropdown = ref(false);

// Computed filtered plugins
const filteredAvailablePlugins = computed(() => {
    // Get plugins that already exist on this server
    const existingPluginIds = new Set(plugins.value.map((p) => p.plugin_id));

    // Filter out plugins that already exist and don't allow multiple instances
    let availableForCreation = availablePlugins.value.filter((plugin) => {
        // If plugin already exists and doesn't allow multiple instances, exclude it
        if (
            existingPluginIds.has(plugin.id) &&
            !plugin.allow_multiple_instances
        ) {
            return false;
        }
        return true;
    });

    // Filter by search query if provided
    if (pluginSearchQuery.value.trim()) {
        const query = pluginSearchQuery.value.toLowerCase().trim();
        availableForCreation = availableForCreation.filter(
            (plugin) =>
                plugin.name.toLowerCase().includes(query) ||
                plugin.description.toLowerCase().includes(query) ||
                plugin.id.toLowerCase().includes(query),
        );
    }

    return availableForCreation;
});

// Get selected plugin object
const selectedPluginObject = computed(() => {
    return availablePlugins.value.find((p) => p.id === selectedPlugin.value);
});

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
        plugins.value = (response as any).data.plugins || [];
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
        availablePlugins.value = (response as any).data.plugins || [];
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
    if (!selectedPlugin.value) {
        toast({
            title: "Error",
            description: "Please select a plugin",
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
                config: pluginConfig.value,
            },
        });

        toast({
            title: "Success",
            description: "Plugin created successfully",
        });

        showAddDialog.value = false;
        selectedPlugin.value = "";
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
const togglePlugin = async (plugin: any, newState: boolean) => {
    const action = newState ? "enable" : "disable";

    try {
        await $fetch(
            `/api/servers/${serverId}/plugins/${plugin.id}/${action}`,
            {
                method: "POST",
                headers: {
                    Authorization: `Bearer ${authStore.token}`,
                },
            },
        );

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
        // Reload plugins to revert UI state on error
        await loadPlugins();
    }
};

// Delete plugin instance
const deletePlugin = async (plugin: any) => {
    if (
        !confirm(`Are you sure you want to delete the plugin "${plugin.name}"?`)
    ) {
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

    // Deep copy and convert values to proper types
    const config = JSON.parse(JSON.stringify(plugin.config || {}));
    const pluginDef = availablePlugins.value.find(
        (p) => p.id === plugin.plugin_id,
    );

    if (pluginDef?.config_schema?.fields) {
        // Initialize config with schema-aware defaults
        initializeConfigFromSchema(config, pluginDef.config_schema.fields);
    }

    pluginConfig.value = config;
    showConfigDialog.value = true;
};

// Initialize config values based on schema
const initializeConfigFromSchema = (config: any, fields: any[]) => {
    fields.forEach((field: any) => {
        if (field.type === "bool") {
            if (config[field.name] !== undefined) {
                if (typeof config[field.name] === "string") {
                    config[field.name] =
                        config[field.name] === "true" ||
                        config[field.name] === "1";
                } else {
                    config[field.name] = Boolean(config[field.name]);
                }
            } else {
                config[field.name] =
                    field.default !== undefined
                        ? Boolean(field.default)
                        : false;
            }
        } else if (field.sensitive && config[field.name] === "***MASKED***") {
            config[field.name] = "";
        } else if (field.type === "arraystring") {
            if (config[field.name] && !Array.isArray(config[field.name])) {
                if (typeof config[field.name] === "string") {
                    config[field.name] = config[field.name]
                        .split(",")
                        .map((s: string) => s.trim())
                        .filter((s: string) => s.length > 0);
                }
            } else if (!config[field.name]) {
                config[field.name] = field.default || [];
            }
        } else if (field.type === "arrayint") {
            if (config[field.name] && !Array.isArray(config[field.name])) {
                if (typeof config[field.name] === "string") {
                    config[field.name] = config[field.name]
                        .split(",")
                        .map((s: string) => parseInt(s.trim()))
                        .filter((n: number) => !isNaN(n));
                }
            } else if (!config[field.name]) {
                config[field.name] = field.default || [];
            }
        } else if (field.type === "arraybool") {
            if (!config[field.name]) {
                config[field.name] = field.default || [];
            }
        } else if (field.type === "arrayobject") {
            if (!config[field.name]) {
                config[field.name] = field.default || [];
            }
            // Initialize nested objects in array
            if (
                Array.isArray(config[field.name]) &&
                field.nested &&
                field.nested.length > 0
            ) {
                config[field.name].forEach((item: any) => {
                    if (typeof item === "object" && item !== null) {
                        initializeConfigFromSchema(item, field.nested);
                    }
                });
            }
        } else if (field.type === "object") {
            if (!config[field.name]) {
                config[field.name] = field.default || {};
            }
            if (field.nested && field.nested.length > 0) {
                initializeConfigFromSchema(config[field.name], field.nested);
            }
        } else if (field.type === "int") {
            if (config[field.name] === undefined) {
                config[field.name] =
                    field.default !== undefined ? field.default : 0;
            }
        } else if (field.type === "string") {
            if (config[field.name] === undefined) {
                config[field.name] =
                    field.default !== undefined ? field.default : "";
            }
        }
    });
};

// Save plugin configuration
const savePluginConfig = async () => {
    if (!currentPlugin.value) return;

    try {
        await $fetch(
            `/api/servers/${serverId}/plugins/${currentPlugin.value.id}`,
            {
                method: "PUT",
                headers: {
                    Authorization: `Bearer ${authStore.token}`,
                },
                body: {
                    config: pluginConfig.value,
                },
            },
        );

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
            description:
                error.data?.message || "Failed to update plugin configuration",
            variant: "destructive",
        });
    }
};

// Handle plugin selection change
const onPluginSelect = (pluginId: any) => {
    selectedPlugin.value = pluginId || "";
    const plugin = availablePlugins.value.find((p) => p.id === pluginId);
    if (plugin?.config_schema?.fields) {
        // Initialize config with schema-aware defaults
        pluginConfig.value = {};
        initializeConfigFromSchema(
            pluginConfig.value,
            plugin.config_schema.fields,
        );
    }
};

// Handle combobox plugin selection
const onComboboxPluginSelect = (plugin: any) => {
    selectedPlugin.value = plugin.id;
    showPluginDropdown.value = false;

    if (plugin?.config_schema?.fields) {
        // Initialize config with schema-aware defaults
        pluginConfig.value = {};
        initializeConfigFromSchema(
            pluginConfig.value,
            plugin.config_schema.fields,
        );
    }
};

// Handle opening add dialog
const openAddDialog = () => {
    showAddDialog.value = true;
    selectedPlugin.value = "";
    pluginSearchQuery.value = "";
    pluginConfig.value = {};
};

// Watch for dialog close to reset search
watch(showAddDialog, (newValue) => {
    if (!newValue) {
        pluginSearchQuery.value = "";
        selectedPlugin.value = "";
        pluginConfig.value = {};
    }
});

// Get input type for field
const getInputType = (field: any) => {
    switch (field.type) {
        case "string":
            return field.sensitive ||
                field.name.toLowerCase().includes("password")
                ? "password"
                : "text";
        case "int":
        case "number":
            return "number";
        default:
            return "text";
    }
};

// Array bool helper functions
const updateArrayBool = (
    fieldName: string,
    index: number,
    checked: boolean,
) => {
    if (!pluginConfig.value[fieldName]) {
        pluginConfig.value[fieldName] = [];
    }
    pluginConfig.value[fieldName][index] = checked;
};

const addArrayBoolItem = (fieldName: string) => {
    if (!pluginConfig.value[fieldName]) {
        pluginConfig.value[fieldName] = [];
    }
    pluginConfig.value[fieldName].push(false);
};

const removeArrayBoolItem = (fieldName: string, index: number) => {
    if (
        pluginConfig.value[fieldName] &&
        Array.isArray(pluginConfig.value[fieldName])
    ) {
        pluginConfig.value[fieldName].splice(index, 1);
    }
};

// Array object helper functions
const addArrayObjectItem = (fieldName: string, nestedFields: any[]) => {
    if (!pluginConfig.value[fieldName]) {
        pluginConfig.value[fieldName] = [];
    }

    const newItem: Record<string, any> = {};
    initializeConfigFromSchema(newItem, nestedFields);

    pluginConfig.value[fieldName].push(newItem);
};

const removeArrayObjectItem = (fieldName: string, index: number) => {
    if (
        pluginConfig.value[fieldName] &&
        Array.isArray(pluginConfig.value[fieldName])
    ) {
        pluginConfig.value[fieldName].splice(index, 1);
    }
};

// Render field component for recursive rendering
const renderConfigField = (
    field: any,
    config: any,
    fieldPath: string = "",
    isNested: boolean = false,
) => {
    const fieldKey = isNested ? fieldPath : field.name;

    return {
        field,
        config,
        fieldKey,
        isNested,
        getValue: () => {
            if (isNested) {
                const keys = fieldPath.split(".");
                let value = config;
                for (const key of keys) {
                    if (value && typeof value === "object") {
                        value = value[key];
                    } else {
                        return undefined;
                    }
                }
                return value;
            }
            return config[field.name];
        },
        setValue: (value: any) => {
            if (isNested) {
                const keys = fieldPath.split(".");
                let current = config;
                for (let i = 0; i < keys.length - 1; i++) {
                    if (!current[keys[i]]) {
                        current[keys[i]] = {};
                    }
                    current = current[keys[i]];
                }
                current[keys[keys.length - 1]] = value;
            } else {
                config[field.name] = value;
            }
        },
    };
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
    <div class="p-4">
        <div
            class="flex flex-col sm:flex-row sm:items-center justify-between gap-4 mb-4"
        >
            <div>
                <h1 class="text-2xl font-bold">Server Plugins</h1>
                <p class="text-muted-foreground">
                    Manage plugins for this server
                </p>
            </div>
            <div class="flex items-center gap-2">
                <Button
                    variant="outline"
                    @click="$router.push(`/servers/${serverId}/plugins/logs`)"
                    class="flex items-center"
                >
                    <FileText class="w-4 h-4 mr-2" />
                    View All Logs
                </Button>
                <Dialog v-model:open="showAddDialog">
                    <DialogTrigger as-child>
                        <Button @click="openAddDialog">
                            <Plus class="w-4 h-4 mr-2" />
                            Add Plugin
                        </Button>
                    </DialogTrigger>
                    <DialogContent
                        class="sm:max-w-2xl max-h-[90vh] flex flex-col"
                    >
                        <DialogHeader>
                            <DialogTitle>Add New Plugin</DialogTitle>
                            <DialogDescription>
                                Select a plugin to add to this server and
                                configure its settings.
                            </DialogDescription>
                        </DialogHeader>

                        <div class="space-y-4 overflow-y-auto flex-1 pr-2">
                            <div>
                                <Label for="plugin-select">Plugin</Label>
                                <Combobox v-model:open="showPluginDropdown">
                                    <ComboboxAnchor as-child>
                                        <ComboboxTrigger
                                            class="flex h-9 w-full items-center justify-between whitespace-nowrap rounded-md border border-input bg-transparent px-3 py-2 text-sm shadow-sm ring-offset-background focus:outline-none focus:ring-1 focus:ring-ring disabled:cursor-not-allowed disabled:opacity-50"
                                            :class="
                                                !selectedPlugin &&
                                                'text-muted-foreground'
                                            "
                                        >
                                            {{
                                                selectedPluginObject?.name ||
                                                "Select a plugin..."
                                            }}
                                            <ChevronsUpDown
                                                class="ml-2 h-4 w-4 shrink-0 opacity-50"
                                            />
                                        </ComboboxTrigger>
                                    </ComboboxAnchor>
                                    <ComboboxList
                                        class="w-[--reka-combobox-trigger-width] p-0 z-50 min-w-32 overflow-hidden rounded-md border bg-popover text-popover-foreground shadow-md"
                                    >
                                        <div class="border-b border-border">
                                            <Input
                                                v-model="pluginSearchQuery"
                                                placeholder="Search plugins..."
                                                class="h-9 border-0 focus:ring-0 rounded-none px-3"
                                            />
                                        </div>
                                        <ComboboxEmpty
                                            class="py-6 text-center text-sm text-muted-foreground"
                                        >
                                            No plugins found.
                                        </ComboboxEmpty>
                                        <div class="max-h-60 overflow-auto">
                                            <ComboboxItem
                                                v-for="plugin in filteredAvailablePlugins"
                                                :key="plugin.id"
                                                :value="plugin.id"
                                                @select="
                                                    () =>
                                                        onComboboxPluginSelect(
                                                            plugin,
                                                        )
                                                "
                                                class="relative flex cursor-default select-none items-center rounded-sm px-2 py-1.5 text-sm outline-none hover:bg-accent hover:text-accent-foreground data-[highlighted]:bg-accent data-[highlighted]:text-accent-foreground"
                                            >
                                                <Check
                                                    :class="[
                                                        'mr-2 h-4 w-4',
                                                        selectedPlugin ===
                                                        plugin.id
                                                            ? 'opacity-100'
                                                            : 'opacity-0',
                                                    ]"
                                                />
                                                <div
                                                    class="flex flex-col flex-1"
                                                >
                                                    <span class="font-medium">{{
                                                        plugin.name
                                                    }}</span>
                                                    <span
                                                        class="text-sm text-muted-foreground"
                                                        >{{
                                                            plugin.description
                                                        }}</span
                                                    >
                                                </div>
                                            </ComboboxItem>
                                        </div>
                                    </ComboboxList>
                                </Combobox>
                            </div>

                            <!-- Dynamic configuration fields -->
                            <div v-if="selectedPlugin" class="space-y-4">
                                <h4 class="font-medium">Configuration</h4>
                                <div
                                    v-for="field in availablePlugins.find(
                                        (p) => p.id === selectedPlugin,
                                    )?.config_schema?.fields || []"
                                    :key="field.name"
                                    class="space-y-2"
                                >
                                    <Label :for="`config-${field.name}`">
                                        {{ field.name }}
                                        <span
                                            v-if="field.required"
                                            class="text-red-500"
                                            >*</span
                                        >
                                    </Label>
                                    <p
                                        v-if="field.description"
                                        class="text-sm text-muted-foreground"
                                    >
                                        {{ field.description }}
                                    </p>

                                    <!-- String/Text input -->
                                    <!-- String/Text input -->
                                    <Input
                                        v-if="field.type === 'string'"
                                        :id="`config-${field.name}`"
                                        v-model="pluginConfig[field.name]"
                                        :type="getInputType(field)"
                                        :placeholder="
                                            field.sensitive &&
                                            pluginConfig[field.name] === ''
                                                ? 'Leave empty to keep current value'
                                                : field.sensitive
                                                  ? 'Enter new sensitive value...'
                                                  : ''
                                        "
                                    />

                                    <!-- Number input -->
                                    <Input
                                        v-else-if="
                                            field.type === 'int' ||
                                            field.type === 'number'
                                        "
                                        :id="`config-${field.name}`"
                                        v-model.number="
                                            pluginConfig[field.name]
                                        "
                                        type="number"
                                    />

                                    <!-- Boolean switch -->
                                    <div
                                        v-else-if="field.type === 'bool'"
                                        class="flex items-center space-x-2"
                                    >
                                        <Switch
                                            :id="`config-${field.name}`"
                                            :model-value="
                                                !!pluginConfig[field.name]
                                            "
                                            @update:model-value="
                                                (checked: boolean) =>
                                                    (pluginConfig[field.name] =
                                                        checked)
                                            "
                                        />
                                        <Label :for="`config-${field.name}`">{{
                                            field.name
                                        }}</Label>
                                    </div>

                                    <!-- Select dropdown for options -->
                                    <Select
                                        v-else-if="
                                            field.options &&
                                            field.options.length > 0
                                        "
                                        :model-value="pluginConfig[field.name]"
                                        @update:model-value="
                                            (value) =>
                                                (pluginConfig[field.name] =
                                                    value)
                                        "
                                    >
                                        <SelectTrigger
                                            :id="`config-${field.name}`"
                                        >
                                            <SelectValue
                                                placeholder="Select an option"
                                            />
                                        </SelectTrigger>
                                        <SelectContent>
                                            <SelectItem
                                                v-for="option in field.options"
                                                :key="option"
                                                :value="option"
                                            >
                                                {{ option }}
                                            </SelectItem>
                                        </SelectContent>
                                    </Select>

                                    <!-- Array of strings -->
                                    <Textarea
                                        v-else-if="
                                            field.type === 'arraystring' ||
                                            field.type === 'array_string'
                                        "
                                        :id="`config-${field.name}`"
                                        :model-value="
                                            Array.isArray(
                                                pluginConfig[field.name],
                                            )
                                                ? pluginConfig[field.name].join(
                                                      ', ',
                                                  )
                                                : pluginConfig[field.name] || ''
                                        "
                                        placeholder="Enter values separated by commas"
                                        @input="
                                            pluginConfig[field.name] = (
                                                $event.target as HTMLTextAreaElement
                                            ).value
                                                .split(',')
                                                .map((s: string) => s.trim())
                                                .filter((s) => s.length > 0)
                                        "
                                    />

                                    <!-- Array of integers -->
                                    <Textarea
                                        v-else-if="field.type === 'arrayint'"
                                        :id="`config-${field.name}`"
                                        :model-value="
                                            Array.isArray(
                                                pluginConfig[field.name],
                                            )
                                                ? pluginConfig[field.name].join(
                                                      ', ',
                                                  )
                                                : pluginConfig[field.name] || ''
                                        "
                                        placeholder="Enter integer values separated by commas"
                                        @input="
                                            pluginConfig[field.name] = (
                                                $event.target as HTMLTextAreaElement
                                            ).value
                                                .split(',')
                                                .map((s: string) =>
                                                    parseInt(s.trim()),
                                                )
                                                .filter((n) => !isNaN(n))
                                        "
                                    />

                                    <!-- Array of booleans -->
                                    <div
                                        v-else-if="field.type === 'arraybool'"
                                        class="space-y-2"
                                    >
                                        <div
                                            v-for="(
                                                item, index
                                            ) in pluginConfig[field.name] || []"
                                            :key="index"
                                            class="flex items-center space-x-2"
                                        >
                                            <Switch
                                                :checked="!!item"
                                                @update:checked="
                                                    (checked: boolean) =>
                                                        updateArrayBool(
                                                            field.name,
                                                            index,
                                                            checked,
                                                        )
                                                "
                                            />
                                            <Label>Item {{ index + 1 }}</Label>
                                            <Button
                                                variant="outline"
                                                size="sm"
                                                @click="
                                                    removeArrayBoolItem(
                                                        field.name,
                                                        index,
                                                    )
                                                "
                                            >
                                                Remove
                                            </Button>
                                        </div>
                                        <Button
                                            variant="outline"
                                            size="sm"
                                            @click="
                                                addArrayBoolItem(field.name)
                                            "
                                        >
                                            Add Item
                                        </Button>
                                    </div>

                                    <!-- Object configuration -->
                                    <div
                                        v-else-if="field.type === 'object'"
                                        class="space-y-4 p-4 border rounded-lg"
                                    >
                                        <h5 class="font-medium">
                                            {{ field.name }} Configuration
                                        </h5>
                                        <div
                                            v-for="nestedField in field.nested ||
                                            []"
                                            :key="nestedField.name"
                                            class="space-y-2"
                                        >
                                            <Label
                                                :for="`config-${field.name}-${nestedField.name}`"
                                            >
                                                {{ nestedField.name }}
                                                <span
                                                    v-if="nestedField.required"
                                                    class="text-red-500"
                                                    >*</span
                                                >
                                            </Label>
                                            <p
                                                v-if="nestedField.description"
                                                class="text-sm text-muted-foreground"
                                            >
                                                {{ nestedField.description }}
                                            </p>

                                            <!-- Nested field inputs based on type -->
                                            <Input
                                                v-if="
                                                    nestedField.type ===
                                                    'string'
                                                "
                                                :id="`config-${field.name}-${nestedField.name}`"
                                                v-model="
                                                    (pluginConfig[field.name] =
                                                        pluginConfig[
                                                            field.name
                                                        ] || {})[
                                                        nestedField.name
                                                    ]
                                                "
                                                :type="
                                                    nestedField.sensitive ||
                                                    nestedField.name
                                                        .toLowerCase()
                                                        .includes('password')
                                                        ? 'password'
                                                        : 'text'
                                                "
                                                :placeholder="
                                                    nestedField.sensitive
                                                        ? 'Enter sensitive value...'
                                                        : ''
                                                "
                                            />
                                            <Input
                                                v-else-if="
                                                    nestedField.type === 'int'
                                                "
                                                :id="`config-${field.name}-${nestedField.name}`"
                                                v-model.number="
                                                    (pluginConfig[field.name] =
                                                        pluginConfig[
                                                            field.name
                                                        ] || {})[
                                                        nestedField.name
                                                    ]
                                                "
                                                type="number"
                                            />
                                            <div
                                                v-else-if="
                                                    nestedField.type === 'bool'
                                                "
                                                class="flex items-center space-x-2"
                                            >
                                                <Switch
                                                    :id="`config-${field.name}-${nestedField.name}`"
                                                    :checked="
                                                        !!(pluginConfig[
                                                            field.name
                                                        ] || {})[
                                                            nestedField.name
                                                        ]
                                                    "
                                                    @update:checked="
                                                        (checked: boolean) => {
                                                            pluginConfig[
                                                                field.name
                                                            ] =
                                                                pluginConfig[
                                                                    field.name
                                                                ] || {};
                                                            pluginConfig[
                                                                field.name
                                                            ][
                                                                nestedField.name
                                                            ] = checked;
                                                        }
                                                    "
                                                />
                                                <Label
                                                    :for="`config-${field.name}-${nestedField.name}`"
                                                    >{{
                                                        nestedField.name
                                                    }}</Label
                                                >
                                            </div>
                                        </div>
                                    </div>

                                    <!-- Array of objects -->
                                    <div
                                        v-else-if="field.type === 'arrayobject'"
                                        class="space-y-4"
                                    >
                                        <div
                                            v-for="(
                                                item, index
                                            ) in pluginConfig[field.name] || []"
                                            :key="index"
                                            class="p-4 border rounded-lg space-y-2"
                                        >
                                            <div
                                                class="flex justify-between items-center"
                                            >
                                                <h6 class="font-medium">
                                                    {{ field.name }} Item
                                                    {{ index + 1 }}
                                                </h6>
                                                <Button
                                                    variant="outline"
                                                    size="sm"
                                                    @click="
                                                        removeArrayObjectItem(
                                                            field.name,
                                                            index,
                                                        )
                                                    "
                                                >
                                                    Remove
                                                </Button>
                                            </div>

                                            <div
                                                v-for="nestedField in field.nested ||
                                                []"
                                                :key="nestedField.name"
                                                class="space-y-2"
                                            >
                                                <Label
                                                    :for="`config-${field.name}-${index}-${nestedField.name}`"
                                                >
                                                    {{ nestedField.name }}
                                                    <span
                                                        v-if="
                                                            nestedField.required
                                                        "
                                                        class="text-red-500"
                                                        >*</span
                                                    >
                                                </Label>

                                                <Input
                                                    v-if="
                                                        nestedField.type ===
                                                        'string'
                                                    "
                                                    :id="`config-${field.name}-${index}-${nestedField.name}`"
                                                    v-model="
                                                        item[nestedField.name]
                                                    "
                                                    type="text"
                                                />
                                                <Input
                                                    v-else-if="
                                                        nestedField.type ===
                                                        'int'
                                                    "
                                                    :id="`config-${field.name}-${index}-${nestedField.name}`"
                                                    v-model.number="
                                                        item[nestedField.name]
                                                    "
                                                    type="number"
                                                />
                                                <div
                                                    v-else-if="
                                                        nestedField.type ===
                                                        'bool'
                                                    "
                                                    class="flex items-center space-x-2"
                                                >
                                                    <Switch
                                                        :id="`config-${field.name}-${index}-${nestedField.name}`"
                                                        :checked="
                                                            !!item[
                                                                nestedField.name
                                                            ]
                                                        "
                                                        @update:checked="
                                                            (
                                                                checked: boolean,
                                                            ) =>
                                                                (item[
                                                                    nestedField.name
                                                                ] = checked)
                                                        "
                                                    />
                                                    <Label
                                                        :for="`config-${field.name}-${index}-${nestedField.name}`"
                                                        >{{
                                                            nestedField.name
                                                        }}</Label
                                                    >
                                                </div>
                                            </div>
                                        </div>
                                        <Button
                                            variant="outline"
                                            size="sm"
                                            @click="
                                                addArrayObjectItem(
                                                    field.name,
                                                    field.nested || [],
                                                )
                                            "
                                        >
                                            Add {{ field.name }} Item
                                        </Button>
                                    </div>
                                </div>
                            </div>
                        </div>

                        <DialogFooter class="flex-shrink-0 pt-4">
                            <Button
                                variant="outline"
                                @click="showAddDialog = false"
                                >Cancel</Button
                            >
                            <Button @click="createPlugin">Create Plugin</Button>
                        </DialogFooter>
                    </DialogContent>
                </Dialog>
            </div>
        </div>

        <!-- Plugins List -->
        <Card class="mb-4">
            <CardHeader>
                <CardTitle>Active Plugins</CardTitle>
                <CardDescription>
                    Plugins currently configured for this server
                </CardDescription>
            </CardHeader>
            <CardContent>
                <div
                    v-if="loading"
                    class="flex items-center justify-center py-8"
                >
                    <div
                        class="animate-spin rounded-full h-8 w-8 border-b-2 border-primary"
                    ></div>
                </div>

                <div v-else-if="plugins.length === 0" class="text-center py-8">
                    <p class="text-muted-foreground">
                        No plugins configured for this server
                    </p>
                </div>

                <div v-else class="overflow-x-auto">
                    <Table>
                        <TableHeader>
                            <TableRow>
                                <TableHead>Name</TableHead>
                                <TableHead>Description</TableHead>
                                <TableHead class="hidden md:table-cell"
                                    >Enabled</TableHead
                                >
                                <TableHead class="hidden lg:table-cell"
                                    >Last Error</TableHead
                                >
                                <TableHead class="text-right"
                                    >Actions</TableHead
                                >
                            </TableRow>
                        </TableHeader>
                        <TableBody>
                            <TableRow
                                v-for="plugin in plugins"
                                :key="plugin.id"
                                class="hover:bg-muted/50"
                            >
                                <TableCell>
                                    <div class="flex flex-col">
                                        <span class="font-medium">{{
                                            plugin.plugin_name
                                        }}</span>
                                    </div>
                                </TableCell>
                                <TableCell class="font-medium">
                                    <span class="text-sm text-muted-foreground">
                                        {{
                                            availablePlugins.find(
                                                (p) =>
                                                    p.id === plugin.plugin_id,
                                            )?.description
                                        }}
                                    </span>
                                </TableCell>
                                <TableCell class="hidden md:table-cell">
                                    <Switch
                                        :checked="plugin.enabled"
                                        :model-value="plugin.enabled"
                                        @update:model-value="
                                            (newState: boolean) =>
                                                togglePlugin(plugin, newState)
                                        "
                                    />
                                </TableCell>
                                <TableCell class="hidden lg:table-cell">
                                    <span
                                        v-if="plugin.last_error"
                                        class="text-sm text-red-600 dark:text-red-400"
                                        :title="plugin.last_error"
                                    >
                                        {{
                                            plugin.last_error.length > 50
                                                ? plugin.last_error.substring(
                                                      0,
                                                      50,
                                                  ) + "..."
                                                : plugin.last_error
                                        }}
                                    </span>
                                    <span v-else class="text-muted-foreground"
                                        >None</span
                                    >
                                </TableCell>
                                <TableCell class="text-right">
                                    <div
                                        class="flex items-center justify-end space-x-1 sm:space-x-2"
                                    >
                                        <Button
                                            variant="outline"
                                            size="sm"
                                            @click="configurePlugin(plugin)"
                                            class="hidden sm:inline-flex"
                                        >
                                            <Settings class="w-4 h-4" />
                                        </Button>
                                        <Button
                                            variant="outline"
                                            size="sm"
                                            @click="
                                                $router.push(
                                                    `/servers/${serverId}/plugins/${plugin.id}/logs`,
                                                )
                                            "
                                            class="hidden sm:inline-flex"
                                        >
                                            <FileText class="w-4 h-4" />
                                        </Button>
                                        <Button
                                            variant="destructive"
                                            size="sm"
                                            @click="deletePlugin(plugin)"
                                            class="hidden sm:inline-flex"
                                        >
                                            <Trash2 class="w-4 h-4" />
                                        </Button>
                                        <!-- Mobile dropdown menu -->
                                        <div class="sm:hidden">
                                            <Button variant="outline" size="sm">
                                                <Settings class="w-4 h-4" />
                                            </Button>
                                        </div>
                                    </div>
                                </TableCell>
                            </TableRow>
                        </TableBody>
                    </Table>
                </div>
            </CardContent>
        </Card>

        <!-- Configuration Dialog -->
        <Dialog v-model:open="showConfigDialog">
            <DialogContent class="sm:max-w-2xl max-h-[90vh] flex flex-col">
                <DialogHeader>
                    <DialogTitle
                        >Configure {{ currentPlugin?.name }}</DialogTitle
                    >
                    <DialogDescription>
                        Update the configuration for this plugin instance.
                    </DialogDescription>
                </DialogHeader>

                <div
                    v-if="currentPlugin"
                    class="space-y-4 overflow-y-auto flex-1 pr-2"
                >
                    <div
                        v-for="field in availablePlugins.find(
                            (p) => p.id === currentPlugin.plugin_id,
                        )?.config_schema?.fields || []"
                        :key="field.name"
                        class="space-y-2"
                    >
                        <Label :for="`edit-config-${field.name}`">
                            {{ field.name }}
                            <span v-if="field.required" class="text-red-500"
                                >*</span
                            >
                        </Label>
                        <p
                            v-if="field.description"
                            class="text-sm text-muted-foreground"
                        >
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
                            v-else-if="
                                field.type === 'int' || field.type === 'number'
                            "
                            :id="`edit-config-${field.name}`"
                            v-model.number="pluginConfig[field.name]"
                            type="number"
                        />

                        <!-- Boolean switch -->
                        <div
                            v-else-if="field.type === 'bool'"
                            class="flex items-center space-x-2"
                        >
                            <Switch
                                :id="`edit-config-${field.name}`"
                                :model-value="!!pluginConfig[field.name]"
                                @update:model-value="
                                    (checked: boolean) =>
                                        (pluginConfig[field.name] = checked)
                                "
                            />
                            <Label :for="`edit-config-${field.name}`">{{
                                field.name
                            }}</Label>
                        </div>

                        <!-- Select dropdown for options -->
                        <Select
                            v-else-if="
                                field.options && field.options.length > 0
                            "
                            :model-value="pluginConfig[field.name]"
                            @update:model-value="
                                (value) => (pluginConfig[field.name] = value)
                            "
                        >
                            <SelectTrigger :id="`edit-config-${field.name}`">
                                <SelectValue placeholder="Select an option" />
                            </SelectTrigger>
                            <SelectContent>
                                <SelectItem
                                    v-for="option in field.options"
                                    :key="option"
                                    :value="option"
                                >
                                    {{ option }}
                                </SelectItem>
                            </SelectContent>
                        </Select>

                        <!-- Array of strings -->
                        <Textarea
                            v-else-if="
                                field.type === 'arraystring' ||
                                field.type === 'array_string'
                            "
                            :id="`edit-config-${field.name}`"
                            :model-value="
                                Array.isArray(pluginConfig[field.name])
                                    ? pluginConfig[field.name].join(', ')
                                    : pluginConfig[field.name] || ''
                            "
                            placeholder="Enter values separated by commas"
                            @input="
                                pluginConfig[field.name] = (
                                    $event.target as HTMLTextAreaElement
                                ).value
                                    .split(',')
                                    .map((s: string) => s.trim())
                                    .filter((s) => s.length > 0)
                            "
                        />

                        <!-- Array of integers -->
                        <Textarea
                            v-else-if="field.type === 'arrayint'"
                            :id="`edit-config-${field.name}`"
                            :model-value="
                                Array.isArray(pluginConfig[field.name])
                                    ? pluginConfig[field.name].join(', ')
                                    : pluginConfig[field.name] || ''
                            "
                            placeholder="Enter integer values separated by commas"
                            @input="
                                pluginConfig[field.name] = (
                                    $event.target as HTMLTextAreaElement
                                ).value
                                    .split(',')
                                    .map((s: string) => parseInt(s.trim()))
                                    .filter((n) => !isNaN(n))
                            "
                        />

                        <!-- Array of booleans -->
                        <div
                            v-else-if="field.type === 'arraybool'"
                            class="space-y-2"
                        >
                            <div
                                v-for="(item, index) in pluginConfig[
                                    field.name
                                ] || []"
                                :key="index"
                                class="flex items-center space-x-2"
                            >
                                <Switch
                                    :checked="!!item"
                                    @update:checked="
                                        (checked: boolean) =>
                                            updateArrayBool(
                                                field.name,
                                                index,
                                                checked,
                                            )
                                    "
                                />
                                <Label>Item {{ index + 1 }}</Label>
                                <Button
                                    variant="outline"
                                    size="sm"
                                    @click="
                                        removeArrayBoolItem(field.name, index)
                                    "
                                >
                                    Remove
                                </Button>
                            </div>
                            <Button
                                variant="outline"
                                size="sm"
                                @click="addArrayBoolItem(field.name)"
                            >
                                Add Item
                            </Button>
                        </div>

                        <!-- Object configuration -->
                        <div
                            v-else-if="field.type === 'object'"
                            class="space-y-4 p-4 border rounded-lg"
                        >
                            <h5 class="font-medium">
                                {{ field.name }} Configuration
                            </h5>
                            <div
                                v-for="nestedField in field.nested || []"
                                :key="nestedField.name"
                                class="space-y-2"
                            >
                                <Label
                                    :for="`edit-config-${field.name}-${nestedField.name}`"
                                >
                                    {{ nestedField.name }}
                                    <span
                                        v-if="nestedField.required"
                                        class="text-red-500"
                                        >*</span
                                    >
                                </Label>
                                <p
                                    v-if="nestedField.description"
                                    class="text-sm text-muted-foreground"
                                >
                                    {{ nestedField.description }}
                                </p>

                                <!-- Nested field inputs based on type -->
                                <Input
                                    v-if="nestedField.type === 'string'"
                                    :id="`edit-config-${field.name}-${nestedField.name}`"
                                    v-model="
                                        (pluginConfig[field.name] =
                                            pluginConfig[field.name] || {})[
                                            nestedField.name
                                        ]
                                    "
                                    type="text"
                                />
                                <Input
                                    v-else-if="nestedField.type === 'int'"
                                    :id="`edit-config-${field.name}-${nestedField.name}`"
                                    v-model.number="
                                        (pluginConfig[field.name] =
                                            pluginConfig[field.name] || {})[
                                            nestedField.name
                                        ]
                                    "
                                    type="number"
                                />
                                <div
                                    v-else-if="nestedField.type === 'bool'"
                                    class="flex items-center space-x-2"
                                >
                                    <Switch
                                        :id="`edit-config-${field.name}-${nestedField.name}`"
                                        :checked="
                                            !!(pluginConfig[field.name] || {})[
                                                nestedField.name
                                            ]
                                        "
                                        @update:checked="
                                            (checked: boolean) => {
                                                pluginConfig[field.name] =
                                                    pluginConfig[field.name] ||
                                                    {};
                                                pluginConfig[field.name][
                                                    nestedField.name
                                                ] = checked;
                                            }
                                        "
                                    />
                                    <Label
                                        :for="`edit-config-${field.name}-${nestedField.name}`"
                                        >{{ nestedField.name }}</Label
                                    >
                                </div>
                            </div>
                        </div>

                        <!-- Array of objects -->
                        <div
                            v-else-if="field.type === 'arrayobject'"
                            class="space-y-4"
                        >
                            <div
                                v-for="(item, index) in pluginConfig[
                                    field.name
                                ] || []"
                                :key="index"
                                class="p-4 border rounded-lg space-y-2"
                            >
                                <div class="flex justify-between items-center">
                                    <h6 class="font-medium">
                                        {{ field.name }} Item {{ index + 1 }}
                                    </h6>
                                    <Button
                                        variant="outline"
                                        size="sm"
                                        @click="
                                            removeArrayObjectItem(
                                                field.name,
                                                index,
                                            )
                                        "
                                    >
                                        Remove
                                    </Button>
                                </div>

                                <div
                                    v-for="nestedField in field.nested || []"
                                    :key="nestedField.name"
                                    class="space-y-2"
                                >
                                    <Label
                                        :for="`edit-config-${field.name}-${index}-${nestedField.name}`"
                                    >
                                        {{ nestedField.name }}
                                        <span
                                            v-if="nestedField.required"
                                            class="text-red-500"
                                            >*</span
                                        >
                                    </Label>

                                    <Input
                                        v-if="nestedField.type === 'string'"
                                        :id="`edit-config-${field.name}-${index}-${nestedField.name}`"
                                        v-model="item[nestedField.name]"
                                        type="text"
                                    />
                                    <Input
                                        v-else-if="nestedField.type === 'int'"
                                        :id="`edit-config-${field.name}-${index}-${nestedField.name}`"
                                        v-model.number="item[nestedField.name]"
                                        type="number"
                                    />
                                    <div
                                        v-else-if="nestedField.type === 'bool'"
                                        class="flex items-center space-x-2"
                                    >
                                        <Switch
                                            :id="`edit-config-${field.name}-${index}-${nestedField.name}`"
                                            :checked="!!item[nestedField.name]"
                                            @update:checked="
                                                (checked: boolean) =>
                                                    (item[nestedField.name] =
                                                        checked)
                                            "
                                        />
                                        <Label
                                            :for="`edit-config-${field.name}-${index}-${nestedField.name}`"
                                            >{{ nestedField.name }}</Label
                                        >
                                    </div>
                                </div>
                            </div>
                            <Button
                                variant="outline"
                                size="sm"
                                @click="
                                    addArrayObjectItem(
                                        field.name,
                                        field.nested || [],
                                    )
                                "
                            >
                                Add {{ field.name }} Item
                            </Button>
                        </div>
                    </div>
                </div>

                <DialogFooter class="flex-shrink-0 pt-4">
                    <Button variant="outline" @click="showConfigDialog = false"
                        >Cancel</Button
                    >
                    <Button @click="savePluginConfig"
                        >Save Configuration</Button
                    >
                </DialogFooter>
            </DialogContent>
        </Dialog>
    </div>
</template>
