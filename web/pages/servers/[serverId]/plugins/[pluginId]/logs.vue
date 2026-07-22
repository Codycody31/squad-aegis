<script setup lang="ts">
import { ref, onMounted } from "vue";
import { Button } from "~/components/ui/button";
import { toast } from "~/components/ui/toast";
import { ArrowLeft } from "lucide-vue-next";
import PageHeader from "~/components/layout/PageHeader.vue";
import PluginLogViewer from "~/components/plugins/PluginLogViewer.vue";

definePageMeta({
    middleware: ["auth"],
});

const route = useRoute();
const router = useRouter();
const serverId = String(route.params.serverId);
const pluginId = String(route.params.pluginId);
const runtimeConfig = useRuntimeConfig();

const plugin = ref<any>(null);

// Load plugin details
const loadPlugin = async () => {
    try {
        const response = await useAuthFetchImperative<any>(
            `${runtimeConfig.public.backendApi}/servers/${serverId}/plugins/${pluginId}`,
        );
        plugin.value = response.data.plugin;
    } catch (error: any) {
        console.error("Failed to load plugin:", error);
        toast({
            title: "Error",
            description: "Failed to load plugin details",
            variant: "destructive",
        });
    }
};

// Go back to plugins list
const goBack = () => {
    router.push(`/servers/${serverId}/plugins`);
};

onMounted(loadPlugin);
</script>

<template>
    <div class="flex h-screen flex-col overflow-hidden p-4 md:p-6">
        <PageHeader
            :title="`${plugin?.plugin_name || 'Plugin'} Plugin Logs`"
            description="Live log stream for this plugin instance"
        >
            <template #actions>
                <Button variant="outline" size="sm" @click="goBack">
                    <ArrowLeft class="w-4 h-4 sm:mr-2" />
                    <span class="hidden sm:inline">Back</span>
                </Button>
            </template>
        </PageHeader>

        <PluginLogViewer
            :server-id="serverId"
            :plugin-id="pluginId"
            :plugin-name="plugin?.name"
            class="min-h-0 flex-1"
        />
    </div>
</template>
