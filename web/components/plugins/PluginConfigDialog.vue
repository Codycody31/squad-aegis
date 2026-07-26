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
    Select,
    SelectContent,
    SelectItem,
    SelectTrigger,
    SelectValue,
} from "~/components/ui/select";
import { Label } from "~/components/ui/label";
import { toast } from "~/components/ui/toast";
import PluginConfigForm from "~/components/plugins/PluginConfigForm.vue";
import { initializeConfigFromSchema } from "~/composables/useConfigSchema";
import type { PluginDefinition, PluginInstance } from "~/types";

const props = defineProps<{
    serverId: string;
    /** Plugin instance being configured. */
    plugin: PluginInstance | null;
    /** Available plugin definitions (used to resolve the config schema). */
    availablePlugins: PluginDefinition[];
}>();

const emit = defineEmits<{
    (e: "saved"): void;
}>();

const open = defineModel<boolean>("open", { default: false });

const pluginConfig = ref<Record<string, any>>({});
const pluginLogLevel = ref<string>("info");

const pluginDefinition = computed(() =>
    props.availablePlugins.find((p) => p.id === props.plugin?.plugin_id),
);

const configFields = computed(
    () => pluginDefinition.value?.config_schema?.fields || [],
);

// Initialize config state from the plugin instance whenever the dialog opens
watch(open, (isOpen) => {
    if (!isOpen || !props.plugin) return;

    const baseConfig = (props.plugin.config || {}) as Record<string, unknown>;

    pluginConfig.value = pluginDefinition.value?.config_schema?.fields
        ? initializeConfigFromSchema(
              baseConfig,
              pluginDefinition.value.config_schema.fields,
          )
        : { ...baseConfig };
    pluginLogLevel.value = props.plugin.log_level || "info";
});

// Save plugin configuration
const savePluginConfig = async () => {
    if (!props.plugin) return;

    try {
        await useAuthFetchImperative(
            `/api/servers/${props.serverId}/plugins/${props.plugin.id}`,
            {
                method: "PUT",
                body: {
                    config: pluginConfig.value,
                    log_level: pluginLogLevel.value,
                },
            },
        );

        toast({
            title: "Success",
            description: "Plugin configuration updated successfully",
        });

        open.value = false;
        emit("saved");
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
</script>

<template>
    <Dialog v-model:open="open">
        <DialogContent class="sm:max-w-2xl max-h-[90vh] flex flex-col">
            <DialogHeader>
                <DialogTitle>Configure {{ plugin?.plugin_name }}</DialogTitle>
                <DialogDescription>
                    Update the configuration for this plugin instance.
                </DialogDescription>
            </DialogHeader>

            <div v-if="plugin" class="space-y-4 overflow-y-auto flex-1 pr-2">
                <!-- Log Level Configuration -->
                <div class="space-y-2 p-4 border rounded-lg bg-muted/30">
                    <Label for="edit-log-level"> Log Level </Label>
                    <p class="text-sm text-muted-foreground">
                        Only logs at or above this level will be stored. All
                        logs are always published to the event bus for live
                        viewing.
                    </p>
                    <Select
                        v-model="pluginLogLevel"
                        @update:model-value="(value) => (pluginLogLevel = value as string)"
                    >
                        <SelectTrigger id="edit-log-level">
                            <SelectValue placeholder="Select log level" />
                        </SelectTrigger>
                        <SelectContent>
                            <SelectItem value="debug"
                                >Debug (All Logs)</SelectItem
                            >
                            <SelectItem value="info"
                                >Info (Info, Warn, Error)</SelectItem
                            >
                            <SelectItem value="warn"
                                >Warn (Warn, Error)</SelectItem
                            >
                            <SelectItem value="error"
                                >Error (Error Only)</SelectItem
                            >
                        </SelectContent>
                    </Select>
                </div>

                <PluginConfigForm
                    v-model="pluginConfig"
                    :fields="configFields"
                    id-prefix="edit-config"
                />
            </div>

            <DialogFooter class="flex-shrink-0 pt-4">
                <Button variant="outline" @click="open = false">Cancel</Button>
                <Button @click="savePluginConfig">Save Configuration</Button>
            </DialogFooter>
        </DialogContent>
    </Dialog>
</template>
