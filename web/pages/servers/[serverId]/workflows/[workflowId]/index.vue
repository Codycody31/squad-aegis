<template>
    <div class="p-3 sm:p-4 lg:p-6 overflow-x-hidden">
        <!-- Header -->
        <div class="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between mb-4 sm:mb-6">
            <div class="flex-1 min-w-0">
                <div class="flex items-center gap-2 mb-2">
                    <RouterLink :to="`/servers/${serverId}/workflows`">
                        <Button variant="ghost" size="sm" class="text-sm sm:text-base">
                            <ArrowLeft class="w-4 h-4 mr-2" />
                            Back to Workflows
                        </Button>
                    </RouterLink>
                </div>
                <h1 class="text-xl sm:text-2xl lg:text-3xl font-bold truncate">
                    {{ workflow?.name || "Loading..." }}
                </h1>
                <p class="text-xs sm:text-sm text-muted-foreground mt-1 break-words">
                    {{ workflow?.description || "No description" }}
                </p>
            </div>
            <div class="flex items-center gap-2 shrink-0">
                <Button
                    v-if="workflow"
                    @click="toggleWorkflow"
                    :disabled="isUpdating"
                    variant="outline"
                    class="text-sm sm:text-base"
                >
                    <component :is="workflow.enabled ? Pause : Play" class="w-4 h-4 mr-2" />
                    {{ workflow.enabled ? "Disable" : "Enable" }}
                </Button>
                <Button
                    v-if="workflow"
                    @click="exportWorkflow"
                    variant="outline"
                    class="text-sm sm:text-base"
                >
                    <Download class="w-4 h-4 mr-2" />
                    <span class="hidden sm:inline">Export</span>
                </Button>
            </div>
        </div>

        <!-- Loading State -->
        <div v-if="loading && !workflow" class="flex items-center justify-center py-12">
            <Loader2 class="h-8 w-8 animate-spin" />
        </div>

        <!-- Error State -->
        <div v-else-if="error" class="border border-red-200 bg-red-50 p-4 rounded-lg">
            <div class="flex items-center gap-2 text-red-800">
                <AlertCircle class="h-4 w-4" />
                <h3 class="font-medium">Error</h3>
            </div>
            <p class="text-red-700 mt-1">{{ error }}</p>
            <Button @click="loadWorkflow" variant="outline" class="mt-3">
                Retry
            </Button>
        </div>

        <!-- Tabs -->
        <div v-else-if="workflow" class="space-y-4">
            <!-- Tab Navigation -->
            <div class="border-b">
                <div class="flex overflow-x-auto">
                    <button
                        @click="activeTab = 'executions'"
                        :class="[
                            'px-4 py-2 text-sm font-medium border-b-2 transition-colors whitespace-nowrap',
                            activeTab === 'executions'
                                ? 'border-primary text-foreground'
                                : 'border-transparent text-muted-foreground hover:text-foreground',
                        ]"
                    >
                        <div class="flex items-center gap-2">
                            <Activity class="w-4 h-4" />
                            Executions
                        </div>
                    </button>
                    <button
                        @click="activeTab = 'editor'"
                        :class="[
                            'px-4 py-2 text-sm font-medium border-b-2 transition-colors whitespace-nowrap',
                            activeTab === 'editor'
                                ? 'border-primary text-foreground'
                                : 'border-transparent text-muted-foreground hover:text-foreground',
                        ]"
                    >
                        <div class="flex items-center gap-2">
                            <Edit class="w-4 h-4" />
                            Editor
                        </div>
                    </button>
                    <button
                        @click="activeTab = 'settings'"
                        :class="[
                            'px-4 py-2 text-sm font-medium border-b-2 transition-colors whitespace-nowrap',
                            activeTab === 'settings'
                                ? 'border-primary text-foreground'
                                : 'border-transparent text-muted-foreground hover:text-foreground',
                        ]"
                    >
                        <div class="flex items-center gap-2">
                            <Settings class="w-4 h-4" />
                            Settings
                        </div>
                    </button>
                    <button
                        @click="activeTab = 'kvstore'"
                        :class="[
                            'px-4 py-2 text-sm font-medium border-b-2 transition-colors whitespace-nowrap',
                            activeTab === 'kvstore'
                                ? 'border-primary text-foreground'
                                : 'border-transparent text-muted-foreground hover:text-foreground',
                        ]"
                    >
                        <div class="flex items-center gap-2">
                            <Database class="w-4 h-4" />
                            KV Store
                        </div>
                    </button>
                </div>
            </div>

            <!-- Tab Content -->
            <div class="mt-6">
                <!-- Executions Tab -->
                <div v-if="activeTab === 'executions'">
                    <WorkflowExecutionTimeline
                        :workflow-id="workflowId"
                        :server-id="serverId"
                    />
                </div>

                <!-- Editor Tab -->
                <div v-else-if="activeTab === 'editor'">
                    <Card>
                        <CardHeader>
                            <CardTitle>Workflow Editor</CardTitle>
                            <p class="text-sm text-muted-foreground">
                                Modify triggers, steps, and variables for this workflow
                            </p>
                        </CardHeader>
                        <CardContent>
                            <WorkflowEditor
                                v-model="workflowDefinition"
                                :event-types="eventTypes"
                                :step-types="stepTypes"
                                :action-types="actionTypes"
                                :workflow-id="workflow.id"
                                :workflow-name="workflow.name"
                                :workflow-description="workflow.description"
                                :server-id="serverId"
                            />
                            <div class="flex justify-end gap-2 mt-6">
                                <Button
                                    @click="saveWorkflow"
                                    :disabled="isUpdating"
                                >
                                    {{ isUpdating ? "Saving..." : "Save Changes" }}
                                </Button>
                            </div>
                        </CardContent>
                    </Card>
                </div>

                <!-- Settings Tab -->
                <div v-else-if="activeTab === 'settings'">
                    <div class="space-y-6">
                        <!-- Basic Settings -->
                        <Card>
                            <CardHeader>
                                <CardTitle>Basic Settings</CardTitle>
                                <p class="text-sm text-muted-foreground">
                                    Manage workflow name, description, and status
                                </p>
                            </CardHeader>
                            <CardContent class="space-y-4">
                                <div class="space-y-2">
                                    <Label for="workflow-name">Name</Label>
                                    <Input
                                        id="workflow-name"
                                        v-model="workflowName"
                                        placeholder="Enter workflow name"
                                    />
                                </div>
                                <div class="space-y-2">
                                    <Label for="workflow-description">Description</Label>
                                    <Textarea
                                        id="workflow-description"
                                        v-model="workflowDescription"
                                        placeholder="Enter workflow description"
                                        rows="3"
                                    />
                                </div>
                                <div class="flex items-center justify-between">
                                    <div class="space-y-0.5">
                                        <Label>Workflow Status</Label>
                                        <p class="text-sm text-muted-foreground">
                                            {{ workflow.enabled ? "Workflow is active and will process events" : "Workflow is paused and won't process events" }}
                                        </p>
                                    </div>
                                    <Switch
                                        :checked="workflowEnabled"
                                        @update:checked="workflowEnabled = $event"
                                    />
                                </div>
                                <div class="flex justify-end">
                                    <Button
                                        @click="saveBasicSettings"
                                        :disabled="isUpdating"
                                    >
                                        {{ isUpdating ? "Saving..." : "Save Settings" }}
                                    </Button>
                                </div>
                            </CardContent>
                        </Card>

                        <!-- Metadata -->
                        <Card>
                            <CardHeader>
                                <CardTitle>Metadata</CardTitle>
                            </CardHeader>
                            <CardContent class="space-y-3 text-sm">
                                <div class="flex justify-between">
                                    <span class="text-muted-foreground">Workflow ID:</span>
                                    <span class="font-mono">{{ workflow.id }}</span>
                                </div>
                                <div class="flex justify-between">
                                    <span class="text-muted-foreground">Created By:</span>
                                    <span>{{ workflow.created_by }}</span>
                                </div>
                                <div class="flex justify-between">
                                    <span class="text-muted-foreground">Created At:</span>
                                    <span>{{ formatDateTime(workflow.created_at) }}</span>
                                </div>
                                <div class="flex justify-between">
                                    <span class="text-muted-foreground">Last Updated:</span>
                                    <span>{{ formatDateTime(workflow.updated_at) }}</span>
                                </div>
                            </CardContent>
                        </Card>

                        <!-- Danger Zone -->
                        <Card class="border-red-200">
                            <CardHeader>
                                <CardTitle class="text-red-600">Danger Zone</CardTitle>
                                <p class="text-sm text-muted-foreground">
                                    Irreversible actions for this workflow
                                </p>
                            </CardHeader>
                            <CardContent class="space-y-3">
                                <div class="flex items-center justify-between p-3 border border-red-200 rounded-lg">
                                    <div>
                                        <p class="font-medium">Delete Workflow</p>
                                        <p class="text-sm text-muted-foreground">
                                            Permanently delete this workflow and all execution history
                                        </p>
                                    </div>
                                    <Button
                                        @click="deleteWorkflow"
                                        variant="destructive"
                                        :disabled="isDeleting"
                                    >
                                        {{ isDeleting ? "Deleting..." : "Delete" }}
                                    </Button>
                                </div>
                            </CardContent>
                        </Card>
                    </div>
                </div>

                <!-- KV Store Tab -->
                <div v-else-if="activeTab === 'kvstore'">
                    <Card>
                        <CardHeader>
                            <CardTitle>KV Store Management</CardTitle>
                            <p class="text-sm text-muted-foreground">
                                Manage persistent key-value storage for this workflow
                            </p>
                        </CardHeader>
                        <CardContent>
                            <WorkflowKVStore
                                :server-id="serverId"
                                :workflow-id="workflowId"
                                :workflow-name="workflow.name"
                            />
                        </CardContent>
                    </Card>
                </div>
            </div>
        </div>
    </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, watch } from "vue";
import { useRoute, useRouter } from "vue-router";
import {
    ArrowLeft,
    Activity,
    Edit,
    Settings,
    Database,
    Loader2,
    AlertCircle,
    Play,
    Pause,
    Download,
} from "lucide-vue-next";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import { Switch } from "@/components/ui/switch";
import { useToast } from "@/components/ui/toast";
import WorkflowEditor from "@/components/WorkflowEditor.vue";
import WorkflowExecutionTimeline from "@/components/WorkflowExecutionTimeline.vue";
import WorkflowKVStore from "@/components/WorkflowKVStore.vue";

interface WorkflowTrigger {
    id: string;
    name: string;
    event_type: string;
    conditions?: any[];
    enabled: boolean;
}

interface WorkflowStep {
    id: string;
    name: string;
    type: string;
    enabled: boolean;
    config: Record<string, any>;
    on_error?: any;
    next_steps?: string[];
}

interface WorkflowDefinition {
    version: string;
    triggers: WorkflowTrigger[];
    variables: Record<string, any>;
    steps: WorkflowStep[];
    error_handling?: any;
}

interface Workflow {
    id: string;
    server_id: string;
    name: string;
    description?: string;
    enabled: boolean;
    definition: WorkflowDefinition;
    created_by: string;
    created_at: string;
    updated_at: string;
}

const route = useRoute();
const router = useRouter();
const { toast } = useToast();

// Route params
const serverId = route.params.serverId as string;
const workflowId = route.params.workflowId as string;

// State
const loading = ref(false);
const error = ref<string | null>(null);
const workflow = ref<Workflow | null>(null);
const activeTab = ref<"executions" | "editor" | "settings" | "kvstore">("executions");
const isUpdating = ref(false);
const isDeleting = ref(false);

// Form state for settings
const workflowName = ref("");
const workflowDescription = ref("");
const workflowEnabled = ref(true);
const workflowDefinition = ref<WorkflowDefinition>({
    version: "1.0",
    triggers: [],
    variables: {},
    steps: [],
    error_handling: {
        default_action: "stop",
        max_retries: 3,
        retry_delay_ms: 1000,
    },
});

// Available event types for triggers
const eventTypes = [
    { value: "RCON_CHAT_MESSAGE", label: "Chat Message" },
    { value: "RCON_PLAYER_WARNED", label: "Player Warned" },
    { value: "RCON_PLAYER_KICKED", label: "Player Kicked" },
    { value: "RCON_PLAYER_BANNED", label: "Player Banned" },
    { value: "RCON_SQUAD_CREATED", label: "Squad Created" },
    { value: "RCON_SERVER_INFO", label: "Server Info" },
    { value: "LOG_PLAYER_CONNECTED", label: "Player Connected" },
    { value: "LOG_JOIN_SUCCEEDED", label: "Player Join Succeeded" },
    { value: "LOG_PLAYER_DISCONNECTED", label: "Player Disconnected" },
    { value: "LOG_PLAYER_DIED", label: "Player Died" },
    { value: "LOG_PLAYER_WOUNDED", label: "Player Wounded" },
    { value: "LOG_ADMIN_BROADCAST", label: "Admin Broadcast" },
    { value: "LOG_GAME_EVENT_UNIFIED", label: "Game Event" },
];

// Available step types
const stepTypes = [
    {
        value: "action",
        label: "Action",
        description: "Perform an action like RCON command or HTTP request",
    },
    {
        value: "condition",
        label: "Condition",
        description: "Check conditions and branch execution",
    },
    {
        value: "variable",
        label: "Variable",
        description: "Set or modify workflow variables",
    },
    {
        value: "delay",
        label: "Delay",
        description: "Wait for a specified amount of time",
    },
];

// Available action types
const actionTypes = [
    {
        value: "rcon_command",
        label: "RCON Command",
        description: "Execute an RCON command",
    },
    {
        value: "admin_broadcast",
        label: "Admin Broadcast",
        description: "Send an admin broadcast message",
    },
    {
        value: "chat_message",
        label: "Chat Message",
        description: "Send a chat message to players",
    },
    {
        value: "kick_player",
        label: "Kick Player",
        description: "Kick a player from the server",
    },
    {
        value: "ban_player",
        label: "Ban Player",
        description: "Ban a player from the server",
    },
    {
        value: "ban_player_with_evidence",
        label: "Ban Player with Evidence",
        description: "Ban a player and link the triggering event as evidence",
    },
    {
        value: "warn_player",
        label: "Warn Player",
        description: "Send a warning to a player",
    },
    {
        value: "http_request",
        label: "HTTP Request",
        description: "Make an HTTP request to external API",
    },
    {
        value: "webhook",
        label: "Webhook",
        description: "Send data to webhook endpoint",
    },
    {
        value: "discord_message",
        label: "Discord Message",
        description: "Send a message to Discord channel",
    },
    {
        value: "log_message",
        label: "Log Message",
        description: "Write a message to the logs",
    },
    {
        value: "set_variable",
        label: "Set Variable",
        description: "Set a workflow variable",
    },
    {
        value: "lua_script",
        label: "Lua Script",
        description: "Execute custom Lua script with workflow context",
    },
];

// Methods
const loadWorkflow = async () => {
    loading.value = true;
    error.value = null;

    const runtimeConfig = useRuntimeConfig();
    const cookieToken = useCookie(
        runtimeConfig.public.sessionCookieName as string
    );
    const token = cookieToken.value;

    if (!token) {
        error.value = "Authentication required";
        loading.value = false;
        return;
    }

    try {
        const response = await $fetch<{ workflow: Workflow }>(
            `${runtimeConfig.public.backendApi}/servers/${serverId}/workflows/${workflowId}`,
            {
                headers: {
                    Authorization: `Bearer ${token}`,
                },
            }
        );

        if (response && response.data.workflow) {
            workflow.value = response.data.workflow;
            workflowName.value = response.data.workflow.name;
            workflowDescription.value = response.data.workflow.description || "";
            workflowEnabled.value = response.data.workflow.enabled;
            
            // Ensure definition has all required properties
            const def = response.data.workflow.definition || {};
            workflowDefinition.value = {
                version: def.version || "1.0",
                triggers: def.triggers || [],
                variables: def.variables || {},
                steps: def.steps || [],
                error_handling: def.error_handling || {
                    default_action: "stop",
                    max_retries: 3,
                    retry_delay_ms: 1000,
                },
            };
        }
    } catch (err: any) {
        error.value = err.message || "Error fetching workflow";
        toast({
            title: "Error",
            description: error.value,
            variant: "destructive",
        });
    } finally {
        loading.value = false;
    }
};

const saveWorkflow = async () => {
    if (!workflow.value) return;

    isUpdating.value = true;
    const runtimeConfig = useRuntimeConfig();
    const cookieToken = useCookie(
        runtimeConfig.public.sessionCookieName as string
    );
    const token = cookieToken.value;

    try {
        await $fetch(
            `${runtimeConfig.public.backendApi}/servers/${serverId}/workflows/${workflowId}`,
            {
                method: "PUT",
                headers: {
                    "Content-Type": "application/json",
                    Authorization: `Bearer ${token}`,
                },
                body: JSON.stringify({
                    name: workflow.value.name,
                    description: workflow.value.description,
                    enabled: workflow.value.enabled,
                    definition: workflowDefinition.value,
                }),
            }
        );

        toast({
            title: "Success",
            description: "Workflow updated successfully",
        });

        await loadWorkflow();
    } catch (err: any) {
        toast({
            title: "Error",
            description: err.data?.error || "Failed to update workflow",
            variant: "destructive",
        });
    } finally {
        isUpdating.value = false;
    }
};

const saveBasicSettings = async () => {
    if (!workflow.value) return;

    isUpdating.value = true;
    const runtimeConfig = useRuntimeConfig();
    const cookieToken = useCookie(
        runtimeConfig.public.sessionCookieName as string
    );
    const token = cookieToken.value;

    try {
        await $fetch(
            `${runtimeConfig.public.backendApi}/servers/${serverId}/workflows/${workflowId}`,
            {
                method: "PUT",
                headers: {
                    "Content-Type": "application/json",
                    Authorization: `Bearer ${token}`,
                },
                body: JSON.stringify({
                    name: workflowName.value,
                    description: workflowDescription.value,
                    enabled: workflowEnabled.value,
                    definition: workflow.value.definition,
                }),
            }
        );

        toast({
            title: "Success",
            description: "Settings updated successfully",
        });

        await loadWorkflow();
    } catch (err: any) {
        toast({
            title: "Error",
            description: err.data?.error || "Failed to update settings",
            variant: "destructive",
        });
    } finally {
        isUpdating.value = false;
    }
};

const toggleWorkflow = async () => {
    if (!workflow.value) return;

    const newEnabled = !workflow.value.enabled;
    isUpdating.value = true;
    const runtimeConfig = useRuntimeConfig();
    const cookieToken = useCookie(
        runtimeConfig.public.sessionCookieName as string
    );
    const token = cookieToken.value;

    try {
        await $fetch(
            `${runtimeConfig.public.backendApi}/servers/${serverId}/workflows/${workflowId}`,
            {
                method: "PUT",
                headers: {
                    "Content-Type": "application/json",
                    Authorization: `Bearer ${token}`,
                },
                body: JSON.stringify({
                    name: workflow.value.name,
                    description: workflow.value.description,
                    enabled: newEnabled,
                    definition: workflow.value.definition,
                }),
            }
        );

        toast({
            title: "Success",
            description: `Workflow ${newEnabled ? "enabled" : "disabled"} successfully`,
        });

        await loadWorkflow();
    } catch (err: any) {
        toast({
            title: "Error",
            description: err.data?.error || "Failed to toggle workflow",
            variant: "destructive",
        });
    } finally {
        isUpdating.value = false;
    }
};

const deleteWorkflow = async () => {
    if (!confirm("Are you sure you want to delete this workflow? This action cannot be undone.")) {
        return;
    }

    isDeleting.value = true;
    const runtimeConfig = useRuntimeConfig();
    const cookieToken = useCookie(
        runtimeConfig.public.sessionCookieName as string
    );
    const token = cookieToken.value;

    try {
        await $fetch(
            `${runtimeConfig.public.backendApi}/servers/${serverId}/workflows/${workflowId}`,
            {
                method: "DELETE",
                headers: {
                    Authorization: `Bearer ${token}`,
                },
            }
        );

        toast({
            title: "Success",
            description: "Workflow deleted successfully",
        });

        router.push(`/servers/${serverId}/workflows`);
    } catch (err: any) {
        toast({
            title: "Error",
            description: err.data?.error || "Failed to delete workflow",
            variant: "destructive",
        });
    } finally {
        isDeleting.value = false;
    }
};

const exportWorkflow = () => {
    if (!workflow.value) return;

    const exportData = {
        name: workflow.value.name,
        description: workflow.value.description,
        enabled: workflow.value.enabled,
        ...workflow.value.definition,
    };

    const dataStr = JSON.stringify(exportData, null, 2);
    const dataUri = "data:application/json;charset=utf-8," + encodeURIComponent(dataStr);
    const exportFileDefaultName = `workflow-${workflow.value.name.replace(/\s+/g, "-").toLowerCase()}.json`;

    const linkElement = document.createElement("a");
    linkElement.setAttribute("href", dataUri);
    linkElement.setAttribute("download", exportFileDefaultName);
    linkElement.click();

    toast({
        title: "Success",
        description: "Workflow exported successfully",
    });
};

const formatDateTime = (dateStr: string) => {
    if (!dateStr) return "N/A";
    try {
        return new Date(dateStr).toLocaleString();
    } catch {
        return dateStr;
    }
};

// Lifecycle
onMounted(() => {
    loadWorkflow();
});

// Page metadata
definePageMeta({
    middleware: "auth",
});
</script>
