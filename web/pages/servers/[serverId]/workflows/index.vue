<script setup lang="ts">
import { ref, onMounted, computed } from "vue";
import {
  Plus,
  Play,
  Pause,
  Trash2,
  Edit,
  Eye,
  Clock,
  Activity,
  Zap,
  GitBranch,
  ExternalLink,
  CirclePlay,
  BookText,
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
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetFooter,
  SheetHeader,
  SheetTitle,
  SheetTrigger,
} from "~/components/ui/sheet";
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
import WorkflowEditor from "~/components/WorkflowEditor.vue";
import WorkflowExecutions from "~/components/WorkflowExecutions.vue";

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

interface WorkflowExecution {
  id: string;
  workflow_id: string;
  execution_id: string;
  status: "RUNNING" | "COMPLETED" | "FAILED" | "CANCELLED";
  started_at: string;
  completed_at?: string;
  error_message?: string;
}

const route = useRoute();
const serverId = route.params.serverId as string;
const { toast } = useToast();

// State
const loading = ref<boolean>(true);
const workflows = ref<Workflow[]>([]);
const selectedWorkflow = ref<Workflow | null>(null);
const executions = ref<WorkflowExecution[]>([]);
const error = ref<string | null>(null);
const showCreateDialog = ref<boolean>(false);
const showEditDialog = ref<boolean>(false);
const showExecutionDialog = ref<boolean>(false);
const isCreating = ref<boolean>(false);
const isUpdating = ref<boolean>(false);
const isExecuting = ref<boolean>(false);

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
  { value: "LOG_PLAYER_CONNECTED", label: "Player Connected" },
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
  {
    value: "lua",
    label: "Lua Script",
    description: "Execute custom Lua script with full workflow context",
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
    const { data, error: fetchError } = await $fetch<{ workflows: Workflow[] }>(
      `${runtimeConfig.public.backendApi}/servers/${serverId}/workflows`,
      {
        headers: {
          Authorization: `Bearer ${token}`,
        },
      }
    );

    if (fetchError) {
      throw new Error(fetchError);
    }

    if (data && data.workflows) {
      workflows.value = data.workflows;
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
  const cookieToken = useCookie(
    runtimeConfig.public.sessionCookieName as string
  );
  const token = cookieToken.value;

  try {
    await $fetch(
      `${runtimeConfig.public.backendApi}/servers/${serverId}/workflows`,
      {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          Authorization: `Bearer ${token}`,
        },
        body: JSON.stringify(newWorkflow.value),
      }
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
  const cookieToken = useCookie(
    runtimeConfig.public.sessionCookieName as string
  );
  const token = cookieToken.value;

  try {
    await $fetch(
      `${runtimeConfig.public.backendApi}/servers/${serverId}/workflows/${workflow.id}`,
      {
        method: "PUT",
        headers: {
          "Content-Type": "application/json",
          Authorization: `Bearer ${token}`,
        },
        body: JSON.stringify({
          name: workflow.name,
          description: workflow.description,
          enabled: workflow.enabled,
          definition: workflow.definition,
        }),
      }
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

    await fetchWorkflows();
  } catch (err: any) {
    toast({
      title: "Error",
      description: err.data?.error || "Failed to delete workflow",
      variant: "destructive",
    });
  }
}

// Execute workflow manually
async function executeWorkflow(workflowId: string) {
  if (!confirm("Are you sure you want to execute this workflow manually?")) {
    return;
  }

  isExecuting.value = true;
  const runtimeConfig = useRuntimeConfig();
  const cookieToken = useCookie(
    runtimeConfig.public.sessionCookieName as string
  );
  const token = cookieToken.value;

  try {
    await $fetch(
      `${runtimeConfig.public.backendApi}/servers/${serverId}/workflows/${workflowId}/execute`,
      {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          Authorization: `Bearer ${token}`,
        },
        body: JSON.stringify({
          trigger_event: {
            manual_trigger: true,
          },
          variables: {},
        }),
      }
    );

    toast({
      title: "Success",
      description: "Workflow execution initiated",
    });
  } catch (err: any) {
    toast({
      title: "Error",
      description: err.data?.error || "Failed to execute workflow",
      variant: "destructive",
    });
  } finally {
    isExecuting.value = false;
  }
}

// Fetch workflow executions
async function fetchExecutions(workflowId: string) {
  const runtimeConfig = useRuntimeConfig();
  const cookieToken = useCookie(
    runtimeConfig.public.sessionCookieName as string
  );
  const token = cookieToken.value;

  try {
    const { data } = await $fetch<{ executions: WorkflowExecution[] }>(
      `${runtimeConfig.public.backendApi}/servers/${serverId}/workflows/${workflowId}/executions`,
      {
        headers: {
          Authorization: `Bearer ${token}`,
        },
      }
    );

    if (data && data.executions) {
      executions.value = data.executions;
    }
  } catch (err: any) {
    toast({
      title: "Error",
      description: "Failed to fetch workflow executions",
      variant: "destructive",
    });
  }
}

// Open edit dialog
function openEditDialog(workflow: Workflow) {
  selectedWorkflow.value = { ...workflow };
  showEditDialog.value = true;
}

// Open execution history dialog
async function openExecutionDialog(workflow: Workflow) {
  selectedWorkflow.value = workflow;
  await fetchExecutions(workflow.id);
  showExecutionDialog.value = true;
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

// Format date for display
function formatDate(dateString: string) {
  return new Date(dateString).toLocaleString();
}

// Get status badge variant
function getStatusVariant(status: string) {
  switch (status) {
    case "COMPLETED":
      return "default";
    case "RUNNING":
      return "secondary";
    case "FAILED":
      return "destructive";
    case "CANCELLED":
      return "outline";
    default:
      return "secondary";
  }
}

// Computed properties
const activeWorkflows = computed(() =>
  workflows.value.filter((w) => w.enabled)
);
const inactiveWorkflows = computed(() =>
  workflows.value.filter((w) => !w.enabled)
);
const totalTriggers = computed(() =>
  workflows.value.reduce((acc, w) => acc + w.definition.triggers.length, 0)
);
const totalSteps = computed(() =>
  workflows.value.reduce((acc, w) => acc + w.definition.steps.length, 0)
);

onMounted(() => {
  fetchWorkflows();
});

definePageMeta({
  middleware: "auth",
});
</script>

<template>
  <div class="p-4">
    <div
      class="flex flex-col gap-3 md:flex-row md:items-center md:justify-between mb-4"
    >
      <div>
        <h1 class="text-3xl font-bold">Workflows</h1>
        <p class="text-muted-foreground">
          Automate server actions with event-driven workflows
        </p>
      </div>
      <Button @click="showCreateDialog = true" class="flex items-center gap-2">
        <Plus class="w-4 h-4" />
        Create Workflow
      </Button>
    </div>

    <!-- Statistics Cards -->
    <div class="grid grid-cols-1 md:grid-cols-4 gap-4 mb-8">
      <Card>
        <CardHeader
          class="flex flex-row items-center justify-between space-y-0 pb-2"
        >
          <CardTitle class="text-sm font-medium">Total Workflows</CardTitle>
          <Activity class="w-4 h-4 text-muted-foreground" />
        </CardHeader>
        <CardContent>
          <div class="text-2xl font-bold">{{ workflows.length }}</div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader
          class="flex flex-row items-center justify-between space-y-0 pb-2"
        >
          <CardTitle class="text-sm font-medium">Active Workflows</CardTitle>
          <Zap class="w-4 h-4 text-green-600" />
        </CardHeader>
        <CardContent>
          <div class="text-2xl font-bold text-green-600">
            {{ activeWorkflows.length }}
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader
          class="flex flex-row items-center justify-between space-y-0 pb-2"
        >
          <CardTitle class="text-sm font-medium">Total Triggers</CardTitle>
          <Activity class="w-4 h-4 text-muted-foreground" />
        </CardHeader>
        <CardContent>
          <div class="text-2xl font-bold">{{ totalTriggers }}</div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader
          class="flex flex-row items-center justify-between space-y-0 pb-2"
        >
          <CardTitle class="text-sm font-medium">Total Steps</CardTitle>
          <GitBranch class="w-4 h-4 text-muted-foreground" />
        </CardHeader>
        <CardContent>
          <div class="text-2xl font-bold">{{ totalSteps }}</div>
        </CardContent>
      </Card>
    </div>

    <!-- Loading State -->
    <div v-if="loading" class="flex justify-center py-8">
      <div
        class="animate-spin rounded-full h-32 w-32 border-b-2 border-gray-900"
      ></div>
    </div>

    <!-- Error State -->
    <div v-else-if="error" class="text-center py-8">
      <p class="text-red-600">{{ error }}</p>
      <Button @click="fetchWorkflows" variant="outline" class="mt-4">
        Retry
      </Button>
    </div>

    <!-- Workflows Table -->
    <div v-else>
      <Card>
        <CardHeader>
          <CardTitle>Workflows</CardTitle>
        </CardHeader>
        <CardContent>
          <Table v-if="workflows.length > 0">
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
              <TableRow v-for="workflow in workflows" :key="workflow.id">
                <TableCell class="font-medium">{{ workflow.name }}</TableCell>
                <TableCell>{{
                  workflow.description || "No description"
                }}</TableCell>
                <TableCell>
                  <Badge :variant="workflow.enabled ? 'default' : 'secondary'">
                    {{ workflow.enabled ? "Active" : "Inactive" }}
                  </Badge>
                </TableCell>
                <TableCell>{{ workflow.definition.triggers.length }}</TableCell>
                <TableCell>{{ workflow.definition.steps.length }}</TableCell>
                <TableCell>{{ formatDate(workflow.updated_at) }}</TableCell>
                <TableCell>
                  <div class="flex items-center gap-2">
                    <Button
                      @click="openExecutionDialog(workflow)"
                      variant="ghost"
                      size="sm"
                    >
                      <BookText class="w-4 h-4" />
                    </Button>
                    <Button
                      @click="executeWorkflow(workflow.id)"
                      variant="ghost"
                      size="sm"
                      :disabled="isExecuting"
                    >
                      <CirclePlay class="w-4 h-4" />
                    </Button>
                    <Button
                      @click="toggleWorkflow(workflow)"
                      variant="ghost"
                      size="sm"
                    >
                      <component
                        :is="workflow.enabled ? Pause : Play"
                        class="w-4 h-4"
                      />
                    </Button>
                    <Button
                      @click="openEditDialog(workflow)"
                      variant="ghost"
                      size="sm"
                    >
                      <Edit class="w-4 h-4" />
                    </Button>
                    <Button
                      @click="deleteWorkflow(workflow.id)"
                      variant="ghost"
                      size="sm"
                    >
                      <Trash2 class="w-4 h-4" />
                    </Button>
                  </div>
                </TableCell>
              </TableRow>
            </TableBody>
          </Table>

          <div v-else class="text-center py-8 text-muted-foreground">
            <Activity class="w-16 h-16 mx-auto mb-4 opacity-25" />
            <p>No workflows found</p>
            <p class="text-sm">
              Create your first workflow to automate server actions
            </p>
          </div>
        </CardContent>
      </Card>
    </div>

    <!-- Create Workflow Dialog -->
    <Dialog v-model:open="showCreateDialog">
      <DialogContent class="max-w-4xl max-h-[90vh] overflow-y-auto">
        <DialogHeader>
          <DialogTitle>Create New Workflow</DialogTitle>
          <DialogDescription>
            Create an automated workflow to respond to server events
          </DialogDescription>
        </DialogHeader>

        <div class="space-y-4">
          <div class="grid grid-cols-2 gap-4">
            <div class="space-y-2">
              <Label for="name">Name</Label>
              <Input
                id="name"
                v-model="newWorkflow.name"
                placeholder="Enter workflow name"
              />
            </div>
            <div class="space-y-2">
              <Label for="enabled">Status</Label>
              <div class="flex items-center space-x-2">
                <Switch id="enabled" v-model:checked="newWorkflow.enabled" />
                <Label for="enabled">{{
                  newWorkflow.enabled ? "Active" : "Inactive"
                }}</Label>
              </div>
            </div>
          </div>

          <div class="space-y-2">
            <Label for="description">Description</Label>
            <Textarea
              id="description"
              v-model="newWorkflow.description"
              placeholder="Enter workflow description"
              rows="3"
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

    <!-- Edit Workflow Dialog -->
    <Dialog v-model:open="showEditDialog">
      <DialogContent
        v-if="selectedWorkflow"
        class="max-w-4xl max-h-[90vh] overflow-y-auto"
      >
        <DialogHeader>
          <DialogTitle>Edit Workflow</DialogTitle>
          <DialogDescription>
            Modify workflow settings and definition
          </DialogDescription>
        </DialogHeader>

        <div class="space-y-4">
          <div class="grid grid-cols-2 gap-4">
            <div class="space-y-2">
              <Label for="edit-name">Name</Label>
              <Input id="edit-name" v-model="selectedWorkflow.name" />
            </div>
            <div class="space-y-2">
              <Label for="edit-enabled">Status</Label>
              <div class="flex items-center space-x-2">
                <Switch
                  id="edit-enabled"
                  :model-value="selectedWorkflow.enabled"
                  @update:model-value="
                    (val) => selectedWorkflow && (selectedWorkflow.enabled = val)
                  "
                />
                <Label for="edit-enabled">{{
                  selectedWorkflow.enabled ? "Active" : "Inactive"
                }}</Label>
              </div>
            </div>
          </div>

          <div class="space-y-2">
            <Label for="edit-description">Description</Label>
            <Textarea
              id="edit-description"
              v-model="selectedWorkflow.description"
              rows="3"
            />
          </div>

          <!-- Workflow Editor Component -->
          <WorkflowEditor
            v-model="selectedWorkflow.definition"
            :event-types="eventTypes"
            :step-types="stepTypes"
            :action-types="actionTypes"
            :workflow-id="selectedWorkflow.id"
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
      <DialogContent v-if="selectedWorkflow" class="max-w-4xl max-h-[90vh] overflow-y-auto">
        <WorkflowExecutions 
          :workflow-id="selectedWorkflow.id" 
          :server-id="selectedWorkflow.server_id"
          :no-header="true"
        />
        <DialogFooter>
          <Button variant="outline" @click="showExecutionDialog = false"
            >Close</Button
          >
        </DialogFooter>
      </DialogContent>
    </Dialog>
  </div>
</template>
