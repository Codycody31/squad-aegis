<script setup lang="ts">
import { ref, onMounted } from "vue";
import { Button } from "~/components/ui/button";
import { toast } from "~/components/ui/toast";
import { Plus, FileText } from "lucide-vue-next";
import PageHeader from "~/components/layout/PageHeader.vue";
import PluginTable from "~/components/plugins/PluginTable.vue";
import AddPluginDialog from "~/components/plugins/AddPluginDialog.vue";
import PluginConfigDialog from "~/components/plugins/PluginConfigDialog.vue";
import PluginKVStore from "~/components/PluginKVStore.vue";
import PluginCommandsModal from "~/components/PluginCommandsModal.vue";
import type { PluginInstance, PluginDefinition } from "~/types";
import {
    Sheet,
    SheetContent,
    SheetDescription,
    SheetHeader,
    SheetTitle,
} from "~/components/ui/sheet";

definePageMeta({
    middleware: ["auth"],
});

const route = useRoute();
const router = useRouter();
const serverId = String(route.params.serverId);

// State variables
const loading = ref(true);
const plugins = ref<PluginInstance[]>([]);
const availablePlugins = ref<PluginDefinition[]>([]);
const showAddDialog = ref(false);
const showConfigDialog = ref(false);
const currentPlugin = ref<PluginInstance | null>(null);
const showKVStoreSheet = ref(false);
const selectedPluginForKV = ref<PluginInstance | null>(null);
const showCommandsModal = ref(false);
const selectedPluginForCommands = ref<PluginInstance | null>(null);

// Load plugins for this server
const loadPlugins = async () => {
    try {
        const response = await useAuthFetchImperative(
            `/api/servers/${serverId}/plugins`,
        );
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
        const response = await useAuthFetchImperative("/api/plugins/available");
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

// Toggle plugin enabled/disabled
const togglePlugin = async (plugin: PluginInstance, newState: boolean) => {
    const action = newState ? "enable" : "disable";

    try {
        await useAuthFetchImperative(
            `/api/servers/${serverId}/plugins/${plugin.id}/${action}`,
            {
                method: "POST",
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
const deletePlugin = async (plugin: PluginInstance) => {
    if (
        !confirm(
            `Are you sure you want to delete the plugin "${plugin.plugin_name}"?`,
        )
    ) {
        return;
    }

    try {
        await useAuthFetchImperative(
            `/api/servers/${serverId}/plugins/${plugin.id}`,
            {
                method: "DELETE",
            },
        );

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
const configurePlugin = (plugin: PluginInstance) => {
    currentPlugin.value = plugin;
    showConfigDialog.value = true;
};

const viewPluginLogs = (plugin: PluginInstance) => {
    router.push(`/servers/${serverId}/plugins/${plugin.id}/logs`);
};

const openKVStoreSheet = (plugin: PluginInstance) => {
    selectedPluginForKV.value = plugin;
    showKVStoreSheet.value = true;
};

const openCommandsModal = (plugin: PluginInstance) => {
    selectedPluginForCommands.value = plugin;
    showCommandsModal.value = true;
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
    <div class="p-4 md:p-6">
        <PageHeader title="Plugins" description="Manage plugins for this server">
            <template #actions>
                <Button
                    variant="outline"
                    @click="router.push(`/servers/${serverId}/plugins/logs`)"
                    class="flex items-center"
                >
                    <FileText class="w-4 h-4 mr-2" />
                    View All Logs
                </Button>
                <Button @click="showAddDialog = true">
                    <Plus class="w-4 h-4 mr-2" />
                    Add Plugin
                </Button>
            </template>
        </PageHeader>

        <!-- Plugins List -->
        <PluginTable
            :plugins="plugins"
            :available-plugins="availablePlugins"
            :loading="loading"
            @toggle="togglePlugin"
            @configure="configurePlugin"
            @view-logs="viewPluginLogs"
            @commands="openCommandsModal"
            @kv-store="openKVStoreSheet"
            @delete="deletePlugin"
        />

        <!-- Add Plugin Dialog -->
        <AddPluginDialog
            v-model:open="showAddDialog"
            :server-id="serverId"
            :available-plugins="availablePlugins"
            :existing-plugins="plugins"
            @added="loadPlugins"
        />

        <!-- Configuration Dialog -->
        <PluginConfigDialog
            v-model:open="showConfigDialog"
            :server-id="serverId"
            :plugin="currentPlugin"
            :available-plugins="availablePlugins"
            @saved="loadPlugins"
        />

        <!-- Plugin Data KV Store Sheet -->
        <Sheet v-model:open="showKVStoreSheet">
            <SheetContent class="sm:max-w-4xl overflow-y-auto">
                <SheetHeader>
                    <SheetTitle>Plugin Data Management</SheetTitle>
                    <SheetDescription>
                        Manage persistent data storage for
                        {{ selectedPluginForKV?.plugin_id }}
                    </SheetDescription>
                </SheetHeader>
                <div class="mt-6">
                    <PluginKVStore
                        v-if="selectedPluginForKV"
                        :server-id="String(serverId)"
                        :plugin-id="selectedPluginForKV.id"
                        :plugin-name="selectedPluginForKV.plugin_id"
                    />
                </div>
            </SheetContent>
        </Sheet>

        <!-- Plugin Commands Modal -->
        <PluginCommandsModal
            v-if="selectedPluginForCommands"
            v-model:open="showCommandsModal"
            :server-id="String(serverId)"
            :plugin-id="selectedPluginForCommands.id"
            :plugin-name="selectedPluginForCommands.plugin_name"
        />
    </div>
</template>
