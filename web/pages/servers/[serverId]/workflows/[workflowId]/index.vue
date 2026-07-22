<script setup lang="ts">
import { ref, onMounted } from "vue";
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
import { Card, CardContent, CardHeader, CardTitle } from "~/components/ui/card";
import { Button } from "~/components/ui/button";
import { toast } from "~/components/ui/toast";
import PageHeader from "~/components/layout/PageHeader.vue";
import WorkflowEditor from "~/components/WorkflowEditor.vue";
import WorkflowExecutionTimeline from "~/components/WorkflowExecutionTimeline.vue";
import WorkflowKVStore from "~/components/WorkflowKVStore.vue";
import WorkflowSettingsCard from "~/components/workflows/WorkflowSettingsCard.vue";
import WorkflowMetadataCard from "~/components/workflows/WorkflowMetadataCard.vue";
import WorkflowDangerZone from "~/components/workflows/WorkflowDangerZone.vue";
import {
  actionTypes,
  defaultWorkflowDefinition,
  eventTypes,
  stepTypes,
  type Workflow,
  type WorkflowDefinition,
} from "~/components/workflows/types";

const route = useRoute();
const router = useRouter();

// Route params
const serverId = route.params.serverId as string;
const workflowId = route.params.workflowId as string;

// State
const loading = ref(false);
const error = ref<string | null>(null);
const workflow = ref<Workflow | null>(null);
const activeTab = ref<"executions" | "editor" | "settings" | "kvstore">(
  "executions"
);
const isUpdating = ref(false);

const workflowDefinition = ref<WorkflowDefinition>(defaultWorkflowDefinition());

const tabs = [
  { key: "executions", label: "Executions", icon: Activity },
  { key: "editor", label: "Editor", icon: Edit },
  { key: "settings", label: "Settings", icon: Settings },
  { key: "kvstore", label: "KV Store", icon: Database },
] as const;

// Methods
const loadWorkflow = async () => {
  loading.value = true;
  error.value = null;

  const runtimeConfig = useRuntimeConfig();

  try {
    const response = await useAuthFetchImperative<{
      data: { workflow: Workflow };
    }>(
      `${runtimeConfig.public.backendApi}/servers/${serverId}/workflows/${workflowId}`
    );

    if (response && response.data.workflow) {
      workflow.value = response.data.workflow;

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
    error.value = extractApiErrorMessage(err, "Error fetching workflow");
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

  try {
    await useAuthFetchImperative(
      `${runtimeConfig.public.backendApi}/servers/${serverId}/workflows/${workflowId}`,
      {
        method: "PUT",
        headers: {
          "Content-Type": "application/json",
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

const toggleWorkflow = async () => {
  if (!workflow.value) return;

  const newEnabled = !workflow.value.enabled;
  isUpdating.value = true;
  const runtimeConfig = useRuntimeConfig();

  try {
    await useAuthFetchImperative(
      `${runtimeConfig.public.backendApi}/servers/${serverId}/workflows/${workflowId}`,
      {
        method: "PUT",
        headers: {
          "Content-Type": "application/json",
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

const exportWorkflow = () => {
  if (!workflow.value) return;

  const exportData = {
    name: workflow.value.name,
    description: workflow.value.description,
    enabled: workflow.value.enabled,
    ...workflow.value.definition,
  };

  const dataStr = JSON.stringify(exportData, null, 2);
  const dataUri =
    "data:application/json;charset=utf-8," + encodeURIComponent(dataStr);
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

const onDeleted = () => {
  router.push(`/servers/${serverId}/workflows`);
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

<template>
  <div class="p-4 md:p-6">
    <!-- Header -->
    <div class="mb-2">
      <RouterLink :to="`/servers/${serverId}/workflows`">
        <Button variant="ghost" size="sm">
          <ArrowLeft class="mr-2 h-4 w-4" />
          Back to Workflows
        </Button>
      </RouterLink>
    </div>
    <PageHeader
      :title="workflow?.name || 'Loading...'"
      :description="workflow?.description || 'No description'"
    >
      <template #actions>
        <Button
          v-if="workflow"
          variant="outline"
          :disabled="isUpdating"
          @click="toggleWorkflow"
        >
          <component :is="workflow.enabled ? Pause : Play" class="mr-2 h-4 w-4" />
          {{ workflow.enabled ? "Disable" : "Enable" }}
        </Button>
        <Button v-if="workflow" variant="outline" @click="exportWorkflow">
          <Download class="mr-2 h-4 w-4" />
          <span class="hidden sm:inline">Export</span>
        </Button>
      </template>
    </PageHeader>

    <!-- Loading State -->
    <div
      v-if="loading && !workflow"
      class="flex items-center justify-center py-12"
    >
      <Loader2 class="h-8 w-8 animate-spin" />
    </div>

    <!-- Error State -->
    <div
      v-else-if="error"
      class="rounded-lg border border-red-200 bg-red-50 p-4"
    >
      <div class="flex items-center gap-2 text-red-800">
        <AlertCircle class="h-4 w-4" />
        <h3 class="font-medium">Error</h3>
      </div>
      <p class="mt-1 text-red-700">{{ error }}</p>
      <Button variant="outline" class="mt-3" @click="loadWorkflow">
        Retry
      </Button>
    </div>

    <!-- Tabs -->
    <div v-else-if="workflow" class="space-y-4">
      <!-- Tab Navigation -->
      <div class="border-b">
        <div class="flex overflow-x-auto">
          <button
            v-for="tab in tabs"
            :key="tab.key"
            :class="[
              'whitespace-nowrap border-b-2 px-4 py-2 text-sm font-medium transition-colors',
              activeTab === tab.key
                ? 'border-primary text-foreground'
                : 'border-transparent text-muted-foreground hover:text-foreground',
            ]"
            @click="activeTab = tab.key"
          >
            <div class="flex items-center gap-2">
              <component :is="tab.icon" class="h-4 w-4" />
              {{ tab.label }}
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
              <div class="mt-6 flex justify-end gap-2">
                <Button :disabled="isUpdating" @click="saveWorkflow">
                  {{ isUpdating ? "Saving..." : "Save Changes" }}
                </Button>
              </div>
            </CardContent>
          </Card>
        </div>

        <!-- Settings Tab -->
        <div v-else-if="activeTab === 'settings'" class="space-y-6">
          <WorkflowSettingsCard
            :server-id="serverId"
            :workflow-id="workflowId"
            :workflow="workflow"
            @saved="loadWorkflow"
          />
          <WorkflowMetadataCard :workflow="workflow" />
          <WorkflowDangerZone
            :server-id="serverId"
            :workflow-id="workflowId"
            @deleted="onDeleted"
          />
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
