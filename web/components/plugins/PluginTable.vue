<script setup lang="ts">
import { Button } from "~/components/ui/button";
import {
    Card,
    CardContent,
    CardDescription,
    CardHeader,
    CardTitle,
} from "~/components/ui/card";
import { Badge } from "~/components/ui/badge";
import { Switch } from "~/components/ui/switch";
import {
    DropdownMenu,
    DropdownMenuContent,
    DropdownMenuItem,
    DropdownMenuTrigger,
} from "~/components/ui/dropdown-menu";
import {
    Settings,
    Trash2,
    FileText,
    Database,
    MoreVertical,
    Terminal,
} from "lucide-vue-next";
import DataTable from "~/components/data-table/DataTable.vue";
import type { DataTableColumn } from "~/components/data-table/types";
import type { PluginDefinition, PluginInstance } from "~/types";

const props = defineProps<{
    plugins: PluginInstance[];
    /** Available plugin definitions (used for metadata lookups). */
    availablePlugins: PluginDefinition[];
    loading?: boolean;
}>();

const emit = defineEmits<{
    (e: "toggle", plugin: PluginInstance, newState: boolean): void;
    (e: "configure", plugin: PluginInstance): void;
    (e: "view-logs", plugin: PluginInstance): void;
    (e: "commands", plugin: PluginInstance): void;
    (e: "kv-store", plugin: PluginInstance): void;
    (e: "delete", plugin: PluginInstance): void;
}>();

const getPluginMetadata = (plugin: any) => {
    return (
        props.availablePlugins.find((p) => p.id === plugin.plugin_id) || plugin
    );
};

const getPluginSourceLabel = (plugin: any) => {
    const metadata = getPluginMetadata(plugin);
    if (metadata.source === "bundled") {
        return "Bundled";
    }
    return "Sideload Native";
};

const formatInstallState = (state: string) => {
    if (!state) {
        return "";
    }
    return state.replaceAll("_", " ");
};

const columns: DataTableColumn<PluginInstance>[] = [
    { key: "name", label: "Name", accessor: (row) => row.plugin_name },
    {
        key: "description",
        label: "Description",
        accessor: (row) => getPluginMetadata(row).description,
    },
    { key: "enabled", label: "Enabled", hideOnMobile: true },
    {
        key: "log_level",
        label: "Log Level",
        class: "hidden xl:table-cell",
        hideOnMobile: true,
    },
    {
        key: "last_error",
        label: "Last Error",
        class: "hidden lg:table-cell",
        hideOnMobile: true,
    },
];
</script>

<template>
    <Card class="mb-4">
        <CardHeader>
            <CardTitle>Active Plugins</CardTitle>
            <CardDescription>
                Plugins currently configured for this server
            </CardDescription>
        </CardHeader>
        <CardContent>
            <DataTable
                :columns="columns"
                :rows="plugins"
                row-key="id"
                :loading="loading"
                empty-text="No plugins configured for this server"
            >
                <template #cell-name="{ row }">
                    <div class="flex flex-col gap-2">
                        <span class="font-medium">{{ row.plugin_name }}</span>
                        <div class="flex flex-wrap gap-2">
                            <Badge variant="outline">
                                {{ getPluginSourceLabel(row) }}
                            </Badge>
                            <Badge
                                v-if="getPluginMetadata(row).official"
                                variant="default"
                            >
                                Official
                            </Badge>
                            <Badge
                                v-if="
                                    row.install_state &&
                                    row.install_state !== 'ready'
                                "
                                variant="secondary"
                                class="capitalize"
                            >
                                {{ formatInstallState(row.install_state) }}
                            </Badge>
                        </div>
                    </div>
                </template>

                <template #cell-description="{ row }">
                    <span class="text-sm text-muted-foreground">
                        {{
                            getPluginMetadata(row).description ||
                            "Plugin metadata is unavailable until the package is ready."
                        }}
                    </span>
                </template>

                <template #cell-enabled="{ row }">
                    <Switch
                        :modelValue="row.enabled"
                        @update:modelValue="
                            (newState: boolean) => emit('toggle', row, newState)
                        "
                    />
                </template>

                <template #cell-log_level="{ row }">
                    <Badge variant="outline" class="capitalize">
                        {{ row.log_level || "info" }}
                    </Badge>
                </template>

                <template #cell-last_error="{ row }">
                    <span
                        v-if="row.last_error"
                        class="text-sm text-red-600 dark:text-red-400"
                        :title="row.last_error"
                    >
                        {{
                            row.last_error.length > 50
                                ? row.last_error.substring(0, 50) + "..."
                                : row.last_error
                        }}
                    </span>
                    <span v-else class="text-muted-foreground">None</span>
                </template>

                <template #actions="{ row }">
                    <div
                        class="flex items-center justify-end space-x-1 sm:space-x-2"
                    >
                        <Button
                            variant="outline"
                            size="sm"
                            @click="emit('configure', row)"
                            class="hidden sm:inline-flex"
                        >
                            <Settings class="w-4 h-4" />
                        </Button>
                        <Button
                            variant="outline"
                            size="sm"
                            @click="emit('view-logs', row)"
                            class="hidden sm:inline-flex"
                        >
                            <FileText class="w-4 h-4" />
                        </Button>
                        <Button
                            variant="outline"
                            size="sm"
                            @click="emit('commands', row)"
                            class="hidden sm:inline-flex"
                            title="Run Commands"
                        >
                            <Terminal class="w-4 h-4" />
                        </Button>
                        <Button
                            variant="outline"
                            size="sm"
                            @click="emit('kv-store', row)"
                            class="hidden sm:inline-flex"
                            title="Manage Plugin Data"
                        >
                            <Database class="w-4 h-4" />
                        </Button>
                        <Button
                            variant="destructive"
                            size="sm"
                            @click="emit('delete', row)"
                            class="hidden sm:inline-flex"
                        >
                            <Trash2 class="w-4 h-4" />
                        </Button>
                        <!-- Mobile dropdown menu -->
                        <DropdownMenu>
                            <DropdownMenuTrigger as-child>
                                <Button
                                    variant="outline"
                                    size="sm"
                                    class="sm:hidden"
                                >
                                    <MoreVertical class="w-4 h-4" />
                                </Button>
                            </DropdownMenuTrigger>
                            <DropdownMenuContent align="end">
                                <DropdownMenuItem
                                    @click="emit('configure', row)"
                                >
                                    <Settings class="w-4 h-4 mr-2" />
                                    Configure
                                </DropdownMenuItem>
                                <DropdownMenuItem
                                    @click="emit('view-logs', row)"
                                >
                                    <FileText class="w-4 h-4 mr-2" />
                                    View Logs
                                </DropdownMenuItem>
                                <DropdownMenuItem @click="emit('commands', row)">
                                    <Terminal class="w-4 h-4 mr-2" />
                                    Run Commands
                                </DropdownMenuItem>
                                <DropdownMenuItem @click="emit('kv-store', row)">
                                    <Database class="w-4 h-4 mr-2" />
                                    Manage Data
                                </DropdownMenuItem>
                                <DropdownMenuItem
                                    @click="emit('delete', row)"
                                    class="text-destructive focus:text-destructive"
                                >
                                    <Trash2 class="w-4 h-4 mr-2" />
                                    Delete
                                </DropdownMenuItem>
                            </DropdownMenuContent>
                        </DropdownMenu>
                    </div>
                </template>
            </DataTable>
        </CardContent>
    </Card>
</template>
