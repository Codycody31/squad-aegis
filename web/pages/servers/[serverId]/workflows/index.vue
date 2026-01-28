<script setup lang="ts">
import { ref, onMounted, computed } from "vue";
import {
    Plus,
    Play,
    Pause,
    Trash2,
    Eye,
    Clock,
    Activity,
    Zap,
    GitBranch,
    ExternalLink,
    Upload,
} from "lucide-vue-next";
import { Button } from "~/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "~/components/ui/card";
import { Badge } from "~/components/ui/badge";
import {
    Table,
    TableBody,
    TableCell,
    TableHead,
    TableHeader,
    TableRow,
} from "~/components/ui/table";
import {
    Dialog,
    DialogContent,
    DialogDescription,
    DialogFooter,
    DialogHeader,
    DialogTitle,
    DialogTrigger,
} from "~/components/ui/dialog";
import { Input } from "~/components/ui/input";
import { Label } from "~/components/ui/label";
import { Textarea } from "~/components/ui/textarea";
import { Switch } from "~/components/ui/switch";
import {
    Select,
    SelectContent,
    SelectItem,
    SelectTrigger,
    SelectValue,
} from "~/components/ui/select";
import { useToast } from "~/components/ui/toast";
import {
    Sheet,
    SheetContent,
    SheetDescription,
    SheetHeader,
    SheetTitle,
} from "~/components/ui/sheet";
import WorkflowEditor from "~/components/WorkflowEditor.vue";
import WorkflowExecutions from "~/components/WorkflowExecutions.vue";
import WorkflowKVStore from "~/components/WorkflowKVStore.vue";

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
    variables?: any[];
}


const route = useRoute();
const serverId = route.params.serverId as string;
const { toast } = useToast();

// State
const loading = ref<boolean>(true);
const workflows = ref<Workflow[]>([]);
const error = ref<string | null>(null);
const showCreateDialog = ref<boolean>(false);
const showImportDialog = ref<boolean>(false);
const isCreating = ref<boolean>(false);
const isUpdating = ref<boolean>(false);
const isExecuting = ref<boolean>(false);
const isImporting = ref<boolean>(false);
const importJsonText = ref<string>("");
const importError = ref<string>("");
const importFile = ref<File | null>(null);
const showEditDialog = ref<boolean>(false);
const selectedWorkflow = ref<Workflow | null>(null);
const showExecutionDialog = ref<boolean>(false);
const showKVStoreSheet = ref<boolean>(false);
const selectedWorkflowForKV = ref<Workflow | null>(null);

// Form state
const newWorkflow = ref({
    name: "",
    description: "",
    enabled: true,
    definition: {
        version: "1.0",
        triggers: [],
        variables: {},
        steps: [],
        error_handling: {
            default_action: "stop",
            max_retries: 3,
            retry_delay_ms: 1000,
        },
    } as WorkflowDefinition,
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

// Available step types for workflow actions
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

// Available action types for action steps
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

// Fetch workflows from API
async function fetchWorkflows() {
    loading.value = true;
    error.value = null;

    const runtimeConfig = useRuntimeConfig();

    try {
        const response = await useAuthFetchImperative<{
            data: { workflows: Workflow[] };
            error?: string;
        }>(`${runtimeConfig.public.backendApi}/servers/${serverId}/workflows`);

        if (response.error) {
            throw new Error(response.error);
        }

        if (response.data && response.data.workflows) {
            workflows.value = response.data.workflows;
        }
    } catch (err: any) {
        error.value = err.message || "Error fetching workflows";
        toast({
            title: "Error",
            description: error.value,
            variant: "destructive",
        });
    } finally {
        loading.value = false;
    }
}

// Create new workflow
async function createWorkflow() {
    if (!newWorkflow.value.name.trim()) {
        toast({
            title: "Validation Error",
            description: "Workflow name is required",
            variant: "destructive",
        });
        return;
    }

    isCreating.value = true;
    const runtimeConfig = useRuntimeConfig();

    try {
        await useAuthFetchImperative(
            `${runtimeConfig.public.backendApi}/servers/${serverId}/workflows`,
            {
                method: "POST",
                headers: {
                    "Content-Type": "application/json",
                },
                body: JSON.stringify(newWorkflow.value),
            },
        );

        toast({
            title: "Success",
            description: "Workflow created successfully",
        });

        showCreateDialog.value = false;
        resetForm();
        await fetchWorkflows();
    } catch (err: any) {
        toast({
            title: "Error",
            description: err.data?.error || "Failed to create workflow",
            variant: "destructive",
        });
    } finally {
        isCreating.value = false;
    }
}

// Update workflow
async function updateWorkflow(workflow: Workflow) {
    isUpdating.value = true;
    const runtimeConfig = useRuntimeConfig();

    try {
        await useAuthFetchImperative(
            `${runtimeConfig.public.backendApi}/servers/${serverId}/workflows/${workflow.id}`,
            {
                method: "PUT",
                headers: {
                    "Content-Type": "application/json",
                },
                body: JSON.stringify({
                    name: workflow.name,
                    description: workflow.description,
                    enabled: workflow.enabled,
                    definition: workflow.definition,
                }),
            },
        );

        toast({
            title: "Success",
            description: "Workflow updated successfully",
        });

        showEditDialog.value = false;
        await fetchWorkflows();
    } catch (err: any) {
        toast({
            title: "Error",
            description: err.data?.error || "Failed to update workflow",
            variant: "destructive",
        });
    } finally {
        isUpdating.value = false;
    }
}

// Toggle workflow enabled/disabled
async function toggleWorkflow(workflow: Workflow) {
    const newWorkflow = { ...workflow, enabled: !workflow.enabled };
    await updateWorkflow(newWorkflow);
}

// Delete workflow
async function deleteWorkflow(workflowId: string) {
    if (!confirm("Are you sure you want to delete this workflow?")) {
        return;
    }

    const runtimeConfig = useRuntimeConfig();

    try {
        await useAuthFetchImperative(
            `${runtimeConfig.public.backendApi}/servers/${serverId}/workflows/${workflowId}`,
            {
                method: "DELETE",
            },
        );

        toast({
            title: "Success",
            description: "Workflow deleted successfully",
        });

        await fetchWorkflows();
    } catch (err: any) {
        toast({
            title: "Error",
            description: err.data?.error || "Failed to delete workflow",
            variant: "destructive",
        });
    }
}


// Navigate to workflow detail page
function navigateToWorkflow(workflowId: string) {
    navigateTo(`/servers/${serverId}/workflows/${workflowId}`);
}

// Reset form
function resetForm() {
    newWorkflow.value = {
        name: "",
        description: "",
        enabled: true,
        definition: {
            version: "1.0",
            triggers: [],
            variables: {},
            steps: [],
            error_handling: {
                default_action: "stop",
                max_retries: 3,
                retry_delay_ms: 1000,
            },
        },
    };
}

function openImportDialog() {
    importJsonText.value = "";
    importError.value = "";
    importFile.value = null;
    showImportDialog.value = true;
}

function closeImportDialog() {
    showImportDialog.value = false;
    importJsonText.value = "";
    importError.value = "";
    importFile.value = null;
}

function handleFileUpload(event: Event) {
    const target = event.target as HTMLInputElement;
    const file = target.files?.[0];

    if (!file) return;

    importFile.value = file;

    const reader = new FileReader();
    reader.onload = (e) => {
        const content = e.target?.result as string;
        importJsonText.value = content;
    };
    reader.readAsText(file);
}

async function importWorkflow() {
    importError.value = "";
    isImporting.value = true;

    if (!importJsonText.value.trim()) {
        importError.value = "Please provide JSON content";
        isImporting.value = false;
        return;
    }

    try {
        const parsed = JSON.parse(importJsonText.value);

        // Validate required structure
        if (!parsed.version) {
            importError.value = "Missing required field: version";
            isImporting.value = false;
            return;
        }

        if (!Array.isArray(parsed.triggers)) {
            importError.value = "Missing or invalid triggers array";
            isImporting.value = false;
            return;
        }

        if (!Array.isArray(parsed.steps)) {
            importError.value = "Missing or invalid steps array";
            isImporting.value = false;
            return;
        }

        // Set the imported definition to newWorkflow
        newWorkflow.value.definition = {
            version: parsed.version,
            triggers: parsed.triggers || [],
            variables: parsed.variables || {},
            steps: parsed.steps || [],
            error_handling: parsed.error_handling || {
                default_action: "stop",
                max_retries: 3,
                retry_delay_ms: 1000,
            },
        };

        // If name/description are provided in the import, use them
        if (parsed.name) newWorkflow.value.name = parsed.name;
        if (parsed.description)
            newWorkflow.value.description = parsed.description;
        if (typeof parsed.enabled === "boolean")
            newWorkflow.value.enabled = parsed.enabled;

        closeImportDialog();
        showCreateDialog.value = true;

        toast({
            title: "Workflow Imported",
            description:
                "Workflow has been imported successfully. Review and create it.",
        });
    } catch (error) {
        if (error instanceof SyntaxError) {
            importError.value = `Invalid JSON: ${error.message}`;
        } else {
            importError.value = `Import error: ${
                error instanceof Error ? error.message : "Unknown error"
            }`;
        }
    } finally {
        isImporting.value = false;
    }
}

// Format date for display
function formatDate(dateString: string) {
    return new Date(dateString).toLocaleString();
}


// Computed properties
const activeWorkflows = computed(() =>
    workflows.value.filter((w) => w.enabled),
);
const inactiveWorkflows = computed(() =>
    workflows.value.filter((w) => !w.enabled),
);
const totalTriggers = computed(() =>
    workflows.value.reduce((acc, w) => acc + (w.definition?.triggers?.length || 0), 0),
);
const totalSteps = computed(() =>
    workflows.value.reduce((acc, w) => acc + (w.definition?.steps?.length || 0), 0),
);

onMounted(() => {
    fetchWorkflows();
});

definePageMeta({
    middleware: "auth",
});
</script>

<template>
    <div class="p-3 sm:p-4 lg:p-6 overflow-x-hidden">
        <div
            class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between mb-3 sm:mb-4"
        >
            <div>
                <h1 class="text-xl sm:text-2xl lg:text-3xl font-bold">Workflows</h1>
                <p class="text-xs sm:text-sm text-muted-foreground">
                    Automate server actions with event-driven workflows
                </p>
            </div>
            <div class="flex flex-col sm:flex-row items-stretch sm:items-center gap-2">
                <Button
                    @click="openImportDialog"
                    variant="outline"
                    class="flex items-center gap-2 w-full sm:w-auto text-sm sm:text-base"
                >
                    <Upload class="w-4 h-4" />
                    <span class="hidden sm:inline">Import Workflow</span>
                    <span class="sm:hidden">Import</span>
                </Button>
                <Button
                    @click="showCreateDialog = true"
                    class="flex items-center gap-2 w-full sm:w-auto text-sm sm:text-base"
                >
                    <Plus class="w-4 h-4" />
                    Create Workflow
                </Button>
            </div>
        </div>

        <!-- Statistics Cards -->
        <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-3 sm:gap-4 mb-4 sm:mb-6 lg:mb-8">
            <Card>
                <CardHeader
                    class="flex flex-row items-center justify-between space-y-0 pb-2"
                >
                    <CardTitle class="text-xs sm:text-sm font-medium"
                        >Total Workflows</CardTitle
                    >
                    <Activity class="w-3 h-3 sm:w-4 sm:h-4 text-muted-foreground" />
                </CardHeader>
                <CardContent>
                    <div class="text-xl sm:text-2xl font-bold">{{ workflows.length }}</div>
                </CardContent>
            </Card>

            <Card>
                <CardHeader
                    class="flex flex-row items-center justify-between space-y-0 pb-2"
                >
                    <CardTitle class="text-xs sm:text-sm font-medium"
                        >Active Workflows</CardTitle
                    >
                    <Zap class="w-3 h-3 sm:w-4 sm:h-4 text-green-600" />
                </CardHeader>
                <CardContent>
                    <div class="text-xl sm:text-2xl font-bold text-green-600">
                        {{ activeWorkflows.length }}
                    </div>
                </CardContent>
            </Card>

            <Card>
                <CardHeader
                    class="flex flex-row items-center justify-between space-y-0 pb-2"
                >
                    <CardTitle class="text-xs sm:text-sm font-medium"
                        >Total Triggers</CardTitle
                    >
                    <Activity class="w-3 h-3 sm:w-4 sm:h-4 text-muted-foreground" />
                </CardHeader>
                <CardContent>
                    <div class="text-xl sm:text-2xl font-bold">{{ totalTriggers }}</div>
                </CardContent>
            </Card>

            <Card>
                <CardHeader
                    class="flex flex-row items-center justify-between space-y-0 pb-2"
                >
                    <CardTitle class="text-xs sm:text-sm font-medium"
                        >Total Steps</CardTitle
                    >
                    <GitBranch class="w-3 h-3 sm:w-4 sm:h-4 text-muted-foreground" />
                </CardHeader>
                <CardContent>
                    <div class="text-xl sm:text-2xl font-bold">{{ totalSteps }}</div>
                </CardContent>
            </Card>
        </div>

        <!-- Loading State -->
        <div v-if="loading" class="flex justify-center py-6 sm:py-8">
            <div
                class="animate-spin rounded-full h-16 w-16 sm:h-32 sm:w-32 border-b-2 border-gray-900"
            ></div>
        </div>

        <!-- Error State -->
        <div v-else-if="error" class="text-center py-6 sm:py-8">
            <p class="text-sm sm:text-base text-red-600 break-words px-2">{{ error }}</p>
            <Button @click="fetchWorkflows" variant="outline" class="mt-3 sm:mt-4 w-full sm:w-auto text-sm sm:text-base">
                Retry
            </Button>
        </div>

        <!-- Workflows Table -->
        <div v-else class="overflow-x-hidden">
            <Card>
                <CardHeader>
                    <CardTitle class="text-base sm:text-lg">Workflows</CardTitle>
                </CardHeader>
                <CardContent class="overflow-x-hidden">
                    <!-- Desktop Table View -->
                    <Table v-if="workflows.length > 0" class="hidden md:table">
                        <TableHeader>
                            <TableRow>
                                <TableHead>Name</TableHead>
                                <TableHead>Description</TableHead>
                                <TableHead>Status</TableHead>
                                <TableHead>Triggers</TableHead>
                                <TableHead>Steps</TableHead>
                                <TableHead>Last Updated</TableHead>
                                <TableHead>Actions</TableHead>
                            </TableRow>
                        </TableHeader>
                        <TableBody>
                            <TableRow
                                v-for="workflow in workflows"
                                :key="workflow.id"
                                class="cursor-pointer hover:bg-muted/50"
                                @click="navigateToWorkflow(workflow.id)"
                            >
                                <TableCell class="font-medium">
                                    <div class="flex items-center gap-2">
                                        <span>{{ workflow.name }}</span>
                                        <ExternalLink class="w-3 h-3 text-muted-foreground" />
                                    </div>
                                </TableCell>
                                <TableCell>{{
                                    workflow.description || "No description"
                                }}</TableCell>
                                <TableCell>
                                    <Badge
                                        :variant="
                                            workflow.enabled
                                                ? 'default'
                                                : 'secondary'
                                        "
                                    >
                                        {{
                                            workflow.enabled
                                                ? "Active"
                                                : "Inactive"
                                        }}
                                    </Badge>
                                </TableCell>
                                <TableCell>{{
                                    workflow.definition?.triggers?.length || 0
                                }}</TableCell>
                                <TableCell>{{
                                    workflow.definition?.steps?.length || 0
                                }}</TableCell>
                                <TableCell class="text-xs sm:text-sm">{{
                                    formatDate(workflow.updated_at)
                                }}</TableCell>
                                <TableCell>
                                    <div class="flex items-center gap-2">
                                        <Button
                                            @click.stop="navigateToWorkflow(workflow.id)"
                                            variant="ghost"
                                            size="sm"
                                            title="View Details"
                                        >
                                            <Eye class="w-4 h-4" />
                                        </Button>
                                        <Button
                                            @click.stop="toggleWorkflow(workflow)"
                                            variant="ghost"
                                            size="sm"
                                            title="Toggle Enable/Disable"
                                        >
                                            <component
                                                :is="
                                                    workflow.enabled
                                                        ? Pause
                                                        : Play
                                                "
                                                class="w-4 h-4"
                                            />
                                        </Button>
                                        <Button
                                            @click.stop="deleteWorkflow(workflow.id)"
                                            variant="ghost"
                                            size="sm"
                                            title="Delete Workflow"
                                        >
                                            <Trash2 class="w-4 h-4" />
                                        </Button>
                                    </div>
                                </TableCell>
                            </TableRow>
                        </TableBody>
                    </Table>

                    <!-- Mobile Card View -->
                    <div v-if="workflows.length > 0" class="md:hidden space-y-3 -mx-1 px-1">
                        <Card
                            v-for="workflow in workflows"
                            :key="workflow.id"
                            class="p-3 sm:p-4 cursor-pointer hover:bg-muted/50"
                            @click="navigateToWorkflow(workflow.id)"
                        >
                            <div class="space-y-3">
                                <div class="flex items-start justify-between gap-2">
                                    <div class="flex-1 min-w-0">
                                        <h3 class="font-medium text-sm sm:text-base truncate">
                                            {{ workflow.name }}
                                        </h3>
                                        <p class="text-xs text-muted-foreground mt-1 line-clamp-2">
                                            {{ workflow.description || "No description" }}
                                        </p>
                                    </div>
                                    <Badge
                                        :variant="
                                            workflow.enabled
                                                ? 'default'
                                                : 'secondary'
                                        "
                                        class="shrink-0 text-xs"
                                    >
                                        {{
                                            workflow.enabled
                                                ? "Active"
                                                : "Inactive"
                                        }}
                                    </Badge>
                                </div>
                                <div class="grid grid-cols-2 gap-3 text-xs sm:text-sm">
                                    <div>
                                        <span class="text-muted-foreground">Triggers:</span>
                                        <span class="ml-1 font-medium">{{
                                            workflow.definition?.triggers?.length || 0
                                        }}</span>
                                    </div>
                                    <div>
                                        <span class="text-muted-foreground">Steps:</span>
                                        <span class="ml-1 font-medium">{{
                                            workflow.definition?.steps?.length || 0
                                        }}</span>
                                    </div>
                                </div>
                                <div class="text-xs text-muted-foreground">
                                    Updated: {{ formatDate(workflow.updated_at) }}
                                </div>
                                <div class="flex items-center gap-1 pt-2 border-t overflow-x-auto pb-1">
                                    <Button
                                        @click.stop="navigateToWorkflow(workflow.id)"
                                        variant="ghost"
                                        size="sm"
                                        class="h-8 w-8 p-0 shrink-0"
                                        title="View Details"
                                    >
                                        <Eye class="h-3 w-3" />
                                    </Button>
                                    <Button
                                        @click.stop="toggleWorkflow(workflow)"
                                        variant="ghost"
                                        size="sm"
                                        class="h-8 w-8 p-0 shrink-0"
                                        :title="workflow.enabled ? 'Pause' : 'Resume'"
                                    >
                                        <component
                                            :is="
                                                workflow.enabled
                                                    ? Pause
                                                    : Play
                                            "
                                            class="h-3 w-3"
                                        />
                                    </Button>
                                    <Button
                                        @click.stop="deleteWorkflow(workflow.id)"
                                        variant="ghost"
                                        size="sm"
                                        class="h-8 w-8 p-0 shrink-0"
                                        title="Delete"
                                    >
                                        <Trash2 class="h-3 w-3" />
                                    </Button>
                                </div>
                            </div>
                        </Card>
                    </div>

                    <div v-else class="text-center py-6 sm:py-8 text-muted-foreground">
                        <Activity class="w-12 h-12 sm:w-16 sm:h-16 mx-auto mb-3 sm:mb-4 opacity-25" />
                        <p class="text-sm sm:text-base">No workflows found</p>
                        <p class="text-xs sm:text-sm mt-1">
                            Create your first workflow to automate server
                            actions
                        </p>
                    </div>
                </CardContent>
            </Card>
        </div>

        <!-- Create Workflow Dialog -->
        <Dialog v-model:open="showCreateDialog">
            <DialogContent class="w-[95vw] sm:max-w-4xl max-h-[90vh] overflow-y-auto p-4 sm:p-6">
                <DialogHeader>
                    <DialogTitle>Create New Workflow</DialogTitle>
                    <DialogDescription>
                        Create an automated workflow to respond to server events
                    </DialogDescription>
                </DialogHeader>

                <div class="space-y-4">
                    <div class="grid grid-cols-1 sm:grid-cols-2 gap-4">
                        <div class="space-y-2">
                            <Label for="name" class="text-xs sm:text-sm">Name</Label>
                            <Input
                                id="name"
                                v-model="newWorkflow.name"
                                placeholder="Enter workflow name"
                                class="text-sm sm:text-base"
                            />
                        </div>
                        <div class="space-y-2">
                            <Label for="enabled" class="text-xs sm:text-sm">Status</Label>
                            <div class="flex items-center space-x-2">
                                <Switch
                                    id="enabled"
                                    v-model:checked="newWorkflow.enabled"
                                />
                                <Label for="enabled" class="text-xs sm:text-sm">{{
                                    newWorkflow.enabled ? "Active" : "Inactive"
                                }}</Label>
                            </div>
                        </div>
                    </div>

                    <div class="space-y-2">
                        <Label for="description" class="text-xs sm:text-sm">Description</Label>
                        <Textarea
                            id="description"
                            v-model="newWorkflow.description"
                            placeholder="Enter workflow description"
                            rows="3"
                            class="text-sm sm:text-base"
                        />
                    </div>

                    <!-- Workflow Editor Component -->
                    <WorkflowEditor
                        v-model="newWorkflow.definition"
                        :event-types="eventTypes"
                        :step-types="stepTypes"
                        :action-types="actionTypes"
                        :server-id="serverId"
                    />
                </div>

                <DialogFooter>
                    <Button variant="outline" @click="showCreateDialog = false"
                        >Cancel</Button
                    >
                    <Button @click="createWorkflow" :disabled="isCreating">
                        {{ isCreating ? "Creating..." : "Create Workflow" }}
                    </Button>
                </DialogFooter>
            </DialogContent>
        </Dialog>

        <!-- Import Workflow Dialog -->
        <Dialog v-model:open="showImportDialog">
            <DialogContent class="w-[95vw] sm:max-w-3xl max-h-[80vh] overflow-y-auto p-4 sm:p-6">
                <DialogHeader>
                    <DialogTitle>Import Workflow</DialogTitle>
                    <DialogDescription>
                        Import a workflow from a JSON file or paste JSON
                        directly
                    </DialogDescription>
                </DialogHeader>

                <div class="space-y-4">
                    <!-- File Upload Section -->
                    <div class="space-y-2">
                        <Label for="workflow-file">Upload JSON File</Label>
                        <div class="flex items-center gap-2">
                            <Input
                                id="workflow-file"
                                type="file"
                                accept=".json,application/json"
                                @change="handleFileUpload"
                                class="cursor-pointer"
                            />
                        </div>
                        <p class="text-xs text-muted-foreground">
                            Select a JSON file containing workflow configuration
                        </p>
                    </div>

                    <div class="relative">
                        <div class="absolute inset-0 flex items-center">
                            <span class="w-full border-t" />
                        </div>
                        <div
                            class="relative flex justify-center text-xs uppercase"
                        >
                            <span
                                class="bg-background px-2 text-muted-foreground"
                            >
                                Or paste JSON
                            </span>
                        </div>
                    </div>

                    <!-- Text Input Section -->
                    <div class="space-y-2">
                        <Label>Workflow JSON</Label>
                        <Textarea
                            v-model="importJsonText"
                            placeholder="Paste your workflow JSON here..."
                            rows="15"
                            class="font-mono text-sm"
                        />
                        <p class="text-xs text-muted-foreground">
                            Ensure the JSON includes version, triggers, steps,
                            and variables fields
                        </p>
                    </div>

                    <!-- Error Display -->
                    <div
                        v-if="importError"
                        class="p-3 bg-destructive/10 border border-destructive/20 rounded-md"
                    >
                        <p class="text-sm text-destructive font-medium">
                            Import Error:
                        </p>
                        <p class="text-sm text-destructive">
                            {{ importError }}
                        </p>
                    </div>
                </div>

                <DialogFooter>
                    <Button variant="outline" @click="closeImportDialog">
                        Cancel
                    </Button>
                    <Button
                        @click="importWorkflow"
                        :disabled="!importJsonText.trim() || isImporting"
                    >
                        {{ isImporting ? "Importing..." : "Import Workflow" }}
                    </Button>
                </DialogFooter>
            </DialogContent>
        </Dialog>

        <!-- Workflow Executions Dialog -->
        <!-- Edit Workflow Dialog -->
        <Dialog v-model:open="showEditDialog">
            <DialogContent
                v-if="selectedWorkflow"
                class="w-[95vw] sm:max-w-4xl max-h-[90vh] overflow-y-auto p-4 sm:p-6"
            >
                <DialogHeader>
                    <DialogTitle>Edit Workflow</DialogTitle>
                    <DialogDescription>
                        Modify workflow settings and definition
                    </DialogDescription>
                </DialogHeader>

                <div class="space-y-4">
                    <div class="grid grid-cols-1 sm:grid-cols-2 gap-4">
                        <div class="space-y-2">
                            <Label for="edit-name" class="text-xs sm:text-sm">Name</Label>
                            <Input
                                id="edit-name"
                                v-model="selectedWorkflow.name"
                                class="text-sm sm:text-base"
                            />
                        </div>
                        <div class="space-y-2">
                            <Label for="edit-enabled" class="text-xs sm:text-sm">Status</Label>
                            <div class="flex items-center space-x-2">
                                <Switch
                                    id="edit-enabled"
                                    :model-value="selectedWorkflow.enabled"
                                    @update:model-value="
                                        (val) =>
                                            selectedWorkflow &&
                                            (selectedWorkflow.enabled = val)
                                    "
                                />
                                <Label for="edit-enabled" class="text-xs sm:text-sm">{{
                                    selectedWorkflow.enabled
                                        ? "Active"
                                        : "Inactive"
                                }}</Label>
                            </div>
                        </div>
                    </div>

                    <div class="space-y-2">
                        <Label for="edit-description" class="text-xs sm:text-sm">Description</Label>
                        <Textarea
                            id="edit-description"
                            v-model="selectedWorkflow.description"
                            rows="3"
                            class="text-sm sm:text-base"
                        />
                    </div>

                    <!-- Workflow Editor Component -->
                    <WorkflowEditor
                        v-model="selectedWorkflow.definition"
                        :event-types="eventTypes"
                        :step-types="stepTypes"
                        :action-types="actionTypes"
                        :workflow-id="selectedWorkflow.id"
                        :workflow-name="selectedWorkflow.name"
                        :workflow-description="selectedWorkflow.description"
                        :server-id="serverId"
                    />
                </div>

                <DialogFooter>
                    <Button variant="outline" @click="showEditDialog = false"
                        >Cancel</Button
                    >
                    <Button
                        @click="updateWorkflow(selectedWorkflow)"
                        :disabled="isUpdating"
                    >
                        {{ isUpdating ? "Updating..." : "Update Workflow" }}
                    </Button>
                </DialogFooter>
            </DialogContent>
        </Dialog>

        <!-- Execution History Dialog -->
        <Dialog v-model:open="showExecutionDialog">
            <DialogContent
                v-if="selectedWorkflow"
                class="w-[95vw] sm:max-w-4xl max-h-[90vh] overflow-y-auto p-4 sm:p-6"
            >
                <WorkflowExecutions
                    :workflow-id="selectedWorkflow.id"
                    :server-id="selectedWorkflow.server_id"
                    :no-header="true"
                />
                <DialogFooter>
                    <Button
                        variant="outline"
                        @click="showExecutionDialog = false"
                        >Close</Button
                    >
                </DialogFooter>
            </DialogContent>
        </Dialog>

        <!-- KV Store Sheet -->
        <Sheet v-model:open="showKVStoreSheet">
            <SheetContent class="w-[95vw] sm:max-w-4xl overflow-y-auto p-4 sm:p-6">
                <SheetHeader>
                    <SheetTitle>KV Store Management</SheetTitle>
                    <SheetDescription>
                        Manage persistent key-value storage for
                        {{ selectedWorkflowForKV?.name }}
                    </SheetDescription>
                </SheetHeader>
                <div class="mt-6">
                    <WorkflowKVStore
                        v-if="selectedWorkflowForKV"
                        :server-id="serverId"
                        :workflow-id="selectedWorkflowForKV.id"
                        :workflow-name="selectedWorkflowForKV.name"
                    />
                </div>
            </SheetContent>
        </Sheet>
    </div>
</template>
