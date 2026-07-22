<script setup lang="ts">
import { ref, onMounted, computed } from "vue";
import { Plus, Upload } from "lucide-vue-next";
import { Button } from "~/components/ui/button";
import { toast } from "~/components/ui/toast";
import PageHeader from "~/components/layout/PageHeader.vue";
import StatCard from "~/components/StatCard.vue";
import WorkflowTable from "~/components/workflows/WorkflowTable.vue";
import WorkflowFormDialog from "~/components/workflows/WorkflowFormDialog.vue";
import WorkflowImportDialog from "~/components/workflows/WorkflowImportDialog.vue";
import type {
  Workflow,
  WorkflowFormValues,
} from "~/components/workflows/types";

const route = useRoute();
const serverId = route.params.serverId as string;

// State
const loading = ref<boolean>(true);
const workflows = ref<Workflow[]>([]);
const error = ref<string | null>(null);
const showCreateDialog = ref<boolean>(false);
const showImportDialog = ref<boolean>(false);
const createPrefill = ref<WorkflowFormValues | null>(null);

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
    error.value = extractApiErrorMessage(err, "Error fetching workflows");
    toast({
      title: "Error",
      description: error.value,
      variant: "destructive",
    });
  } finally {
    loading.value = false;
  }
}

// Toggle workflow enabled/disabled
async function toggleWorkflow(workflow: Workflow) {
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
          enabled: !workflow.enabled,
          definition: workflow.definition,
        }),
      }
    );

    toast({
      title: "Success",
      description: "Workflow updated successfully",
    });

    await fetchWorkflows();
  } catch (err: any) {
    toast({
      title: "Error",
      description: err.data?.error || "Failed to update workflow",
      variant: "destructive",
    });
  }
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

// Navigate to workflow detail page
function navigateToWorkflow(workflowId: string) {
  navigateTo(`/servers/${serverId}/workflows/${workflowId}`);
}

function openCreateDialog() {
  createPrefill.value = null;
  showCreateDialog.value = true;
}

// Imported JSON prefills the create dialog
function onImported(values: WorkflowFormValues) {
  createPrefill.value = values;
  showCreateDialog.value = true;
}

// Computed properties
const activeWorkflows = computed(() =>
  workflows.value.filter((w) => w.enabled)
);
const totalTriggers = computed(() =>
  workflows.value.reduce(
    (acc, w) => acc + (w.definition?.triggers?.length || 0),
    0
  )
);
const totalSteps = computed(() =>
  workflows.value.reduce(
    (acc, w) => acc + (w.definition?.steps?.length || 0),
    0
  )
);

onMounted(() => {
  fetchWorkflows();
});

definePageMeta({
  middleware: "auth",
});
</script>

<template>
  <div class="p-4 md:p-6">
    <PageHeader
      title="Workflows"
      description="Automate server actions with event-driven workflows"
    >
      <template #actions>
        <Button variant="outline" class="gap-2" @click="showImportDialog = true">
          <Upload class="h-4 w-4" />
          <span class="hidden sm:inline">Import Workflow</span>
          <span class="sm:hidden">Import</span>
        </Button>
        <Button class="gap-2" @click="openCreateDialog">
          <Plus class="h-4 w-4" />
          Create Workflow
        </Button>
      </template>
    </PageHeader>

    <!-- Statistics Cards -->
    <div
      class="mb-4 grid grid-cols-1 gap-3 sm:grid-cols-2 sm:gap-4 lg:grid-cols-4"
    >
      <StatCard
        label="Total Workflows"
        :value="workflows.length"
        icon="lucide:activity"
        :loading="loading"
      />
      <StatCard
        label="Active Workflows"
        :value="activeWorkflows.length"
        icon="lucide:zap"
        icon-class="bg-green-500/10 text-green-600"
        :loading="loading"
      />
      <StatCard
        label="Total Triggers"
        :value="totalTriggers"
        icon="lucide:activity"
        :loading="loading"
      />
      <StatCard
        label="Total Steps"
        :value="totalSteps"
        icon="lucide:git-branch"
        :loading="loading"
      />
    </div>

    <!-- Error State -->
    <div v-if="error" class="py-6 text-center sm:py-8">
      <p class="break-words px-2 text-sm text-red-600 sm:text-base">
        {{ error }}
      </p>
      <Button variant="outline" class="mt-3 sm:mt-4" @click="fetchWorkflows">
        Retry
      </Button>
    </div>

    <!-- Workflows Table -->
    <WorkflowTable
      v-else
      :workflows="workflows"
      :loading="loading"
      @open="navigateToWorkflow($event.id)"
      @toggle="toggleWorkflow"
      @delete="deleteWorkflow($event.id)"
    />

    <!-- Create Workflow Dialog -->
    <WorkflowFormDialog
      v-model:open="showCreateDialog"
      mode="create"
      :server-id="serverId"
      :prefill="createPrefill"
      @saved="fetchWorkflows"
    />

    <!-- Import Workflow Dialog -->
    <WorkflowImportDialog
      v-model:open="showImportDialog"
      @imported="onImported"
    />
  </div>
</template>
