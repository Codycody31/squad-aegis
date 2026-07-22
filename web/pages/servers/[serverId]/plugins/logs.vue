<script setup lang="ts">
import { Button } from "~/components/ui/button";
import { ArrowLeft } from "lucide-vue-next";
import PageHeader from "~/components/layout/PageHeader.vue";
import PluginLogViewer from "~/components/plugins/PluginLogViewer.vue";

definePageMeta({
    middleware: ["auth"],
});

const route = useRoute();
const router = useRouter();
const serverId = String(route.params.serverId);

// Go back to plugins list
const goBack = () => {
    router.push(`/servers/${serverId}/plugins`);
};
</script>

<template>
    <div class="flex h-screen flex-col overflow-hidden p-4 md:p-6">
        <PageHeader
            title="All Plugin Logs"
            description="Aggregated live log stream from every plugin on this server"
        >
            <template #actions>
                <Button variant="outline" size="sm" @click="goBack">
                    <ArrowLeft class="w-4 h-4 sm:mr-2" />
                    <span class="hidden sm:inline">Back</span>
                </Button>
            </template>
        </PageHeader>

        <PluginLogViewer :server-id="serverId" class="min-h-0 flex-1" />
    </div>
</template>
