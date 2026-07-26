<script setup lang="ts">
import { ref, computed, watch } from "vue";
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
    Combobox,
    ComboboxAnchor,
    ComboboxEmpty,
    ComboboxItem,
    ComboboxList,
    ComboboxTrigger,
} from "~/components/ui/combobox";
import { Input } from "~/components/ui/input";
import { Label } from "~/components/ui/label";
import { toast } from "~/components/ui/toast";
import { Check, ChevronsUpDown } from "lucide-vue-next";
import PluginConfigForm from "~/components/plugins/PluginConfigForm.vue";
import { initializeConfigFromSchema } from "~/composables/useConfigSchema";
import type { PluginDefinition, PluginInstance } from "~/types";

const props = defineProps<{
    serverId: string;
    /** Available plugin definitions (loaded once by the page). */
    availablePlugins: PluginDefinition[];
    /** Plugin instances already on this server. */
    existingPlugins: PluginInstance[];
}>();

const emit = defineEmits<{
    (e: "added"): void;
}>();

const open = defineModel<boolean>("open", { default: false });

const selectedPlugin = ref<string>("");
const pluginConfig = ref<Record<string, any>>({});

// Combobox state
const pluginSearchQuery = ref("");
const showPluginDropdown = ref(false);

// Computed filtered plugins
const filteredAvailablePlugins = computed(() => {
    // Get plugins that already exist on this server
    const existingPluginIds = new Set(
        props.existingPlugins.map((p) => p.plugin_id),
    );

    // Filter out plugins that already exist and don't allow multiple instances
    let availableForCreation = props.availablePlugins.filter((plugin) => {
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
    return props.availablePlugins.find((p) => p.id === selectedPlugin.value);
});

const selectedPluginFields = computed(
    () => selectedPluginObject.value?.config_schema?.fields || [],
);

// Handle combobox plugin selection
const onComboboxPluginSelect = (plugin: any) => {
    selectedPlugin.value = plugin.id;
    showPluginDropdown.value = false;

    if (plugin?.config_schema?.fields) {
        pluginConfig.value = initializeConfigFromSchema(
            {},
            plugin.config_schema.fields,
        );
    }
};

// Reset selection whenever the dialog opens or closes
watch(open, () => {
    pluginSearchQuery.value = "";
    selectedPlugin.value = "";
    pluginConfig.value = {};
});

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
        await useAuthFetchImperative(`/api/servers/${props.serverId}/plugins`, {
            method: "POST",
            body: {
                plugin_id: selectedPlugin.value,
                config: pluginConfig.value,
            },
        });

        toast({
            title: "Success",
            description: "Plugin created successfully",
        });

        open.value = false;
        selectedPlugin.value = "";
        pluginConfig.value = {};
        emit("added");
    } catch (error: any) {
        console.error("Failed to create plugin:", error);
        toast({
            title: "Error",
            description: error.data?.message || "Failed to create plugin",
            variant: "destructive",
        });
    }
};
</script>

<template>
    <Dialog v-model:open="open">
        <DialogContent class="sm:max-w-2xl max-h-[90vh] flex flex-col">
            <DialogHeader>
                <DialogTitle>Add New Plugin</DialogTitle>
                <DialogDescription>
                    Select a plugin to add to this server and configure its
                    settings.
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
                                    !selectedPlugin && 'text-muted-foreground'
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
                                        () => onComboboxPluginSelect(plugin)
                                    "
                                    class="relative flex cursor-default select-none items-center rounded-sm px-2 py-1.5 text-sm outline-none hover:bg-accent hover:text-accent-foreground data-[highlighted]:bg-accent data-[highlighted]:text-accent-foreground"
                                >
                                    <Check
                                        :class="[
                                            'mr-2 h-4 w-4',
                                            selectedPlugin === plugin.id
                                                ? 'opacity-100'
                                                : 'opacity-0',
                                        ]"
                                    />
                                    <div class="flex flex-col flex-1">
                                        <span class="font-medium">{{
                                            plugin.name
                                        }}</span>
                                        <span
                                            class="text-sm text-muted-foreground"
                                            >{{ plugin.description }}</span
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
                    <PluginConfigForm
                        v-model="pluginConfig"
                        :fields="selectedPluginFields"
                    />
                </div>
            </div>

            <DialogFooter class="flex-shrink-0 pt-4">
                <Button variant="outline" @click="open = false">Cancel</Button>
                <Button @click="createPlugin">Create Plugin</Button>
            </DialogFooter>
        </DialogContent>
    </Dialog>
</template>
