<template>
  <div class="space-y-4">
    <div class="flex justify-between items-center">
      <div>
        <h3 class="text-lg font-medium">Execution History</h3>
        <p class="text-sm text-muted-foreground">
          View past workflow executions and their status
        </p>
      </div>
      <Button @click="loadExecutions(false)" :disabled="loading">
        <RefreshCw class="h-4 w-4 mr-2" :class="{ 'animate-spin': loading }" />
        Refresh
      </Button>
    </div>

    <!-- Loading State -->
    <div
      v-if="loading && executions.length === 0"
      class="flex items-center justify-center py-8"
    >
      <Loader2 class="h-6 w-6 animate-spin" />
    </div>

    <!-- Error State -->
    <div
      v-else-if="error"
      class="border border-red-200 bg-red-50 p-4 rounded-lg"
    >
      <div class="flex items-center gap-2 text-red-800">
        <AlertCircle class="h-4 w-4" />
        <h3 class="font-medium">Error</h3>
      </div>
      <p class="text-red-700 mt-1">{{ error }}</p>
    </div>

    <!-- Executions List -->
    <div v-else-if="executions.length > 0" class="space-y-3">
      <div
        v-for="execution in executions"
        :key="execution.id"
        class="border rounded-lg p-4 hover:bg-muted/50 transition-colors cursor-pointer"
      >
        <div class="flex items-start justify-between">
          <div class="space-y-2">
            <div class="flex items-center gap-2">
              <component
                :is="getStatusIcon(execution.status)"
                :class="getStatusColor(execution.status)"
                class="h-4 w-4"
              />
              <span
                class="font-medium"
                :class="getStatusColor(execution.status)"
              >
                {{ execution.status }}
              </span>
              <Badge variant="outline" class="text-xs">
                {{ execution.id.substring(0, 8) }}
              </Badge>
            </div>

            <div class="text-sm text-muted-foreground">
              Started: {{ formatDateTime(execution.started_at) }}
            </div>

            <div
              v-if="execution.completed_at"
              class="text-sm text-muted-foreground"
            >
              Duration:
              {{ formatDuration(execution.started_at, execution.completed_at) }}
            </div>

            <div
              v-if="execution.trigger_data"
              class="text-xs text-muted-foreground"
            >
              Trigger: {{ getTriggerDescription(execution.trigger_data) }}
            </div>
          </div>

          <div class="flex items-center gap-2">
            <RouterLink 
              :to="`/servers/${props.serverId}/workflows/${props.workflowId}/executions/${execution.execution_id}`"
            >
              <Button
                size="sm"
              >
                <ExternalLink class="h-4 w-4 mr-1" />
                Open in New Tab
              </Button>
            </RouterLink>
          </div>
        </div>

        <div v-if="execution.error" class="mt-3 text-sm">
          <div class="border border-red-200 bg-red-50 p-2 rounded text-red-700">
            <div class="flex items-center gap-1">
              <AlertCircle class="h-3 w-3" />
              <span class="font-medium">Error:</span>
            </div>
            <p class="mt-1 text-xs">{{ execution.error }}</p>
          </div>
        </div>
      </div>

      <!-- Pagination -->
      <div v-if="hasMore" class="flex justify-center pt-4">
        <Button variant="outline" @click="loadMore" :disabled="loading">
          <ChevronDown class="h-4 w-4 mr-2" />
          Load More Executions
        </Button>
      </div>
    </div>

    <!-- No Executions -->
    <div v-else class="text-center py-8 text-muted-foreground">
      <PlayCircle class="h-12 w-12 mx-auto mb-4 opacity-50" />
      <p>No executions found</p>
      <p class="text-sm">This workflow hasn't been executed yet</p>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from "vue";
import { useRouter } from "vue-router";
import {
  RefreshCw,
  Loader2,
  AlertCircle,
  ChevronDown,
  PlayCircle,
  ExternalLink,
  CheckCircle,
  XCircle,
  Clock,
} from "lucide-vue-next";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";

interface WorkflowExecution {
  id: string;
  workflow_id: string;
  status: string;
  started_at: string;
  completed_at?: string;
  trigger_data?: any;
  error?: string;
}

interface ApiResponse<T = any> {
  success: boolean;
  message?: string;
  data: T;
}

interface Props {
  workflowId: string;
  serverId: string;
  noHeader?: boolean;
}

const props = defineProps<Props>();
const router = useRouter();

// State
const loading = ref(false);
const error = ref<string | null>(null);
const executions = ref<WorkflowExecution[]>([]);
const offset = ref(0);
const limit = ref(20);
const hasMore = ref(true);

// Methods
const getStatusIcon = (status: string) => {
  switch (status) {
    case "completed":
      return CheckCircle;
    case "failed":
    case "error":
      return XCircle;
    case "running":
    case "executing":
      return PlayCircle;
    default:
      return Clock;
  }
};

const getStatusColor = (status: string) => {
  switch (status) {
    case "completed":
      return "text-green-600";
    case "failed":
    case "error":
      return "text-red-600";
    case "running":
    case "executing":
      return "text-blue-600";
    default:
      return "text-gray-600";
  }
};

const formatDateTime = (dateStr: string) => {
  if (!dateStr) return "N/A";
  try {
    return new Date(dateStr).toLocaleString();
  } catch {
    return dateStr;
  }
};

const formatDuration = (startStr: string, endStr?: string) => {
  if (!startStr) return "N/A";
  if (!endStr) return "In progress...";

  try {
    const start = new Date(startStr);
    const end = new Date(endStr);
    const diffMs = end.getTime() - start.getTime();

    if (diffMs < 1000) return `${diffMs}ms`;
    if (diffMs < 60000) return `${(diffMs / 1000).toFixed(1)}s`;
    if (diffMs < 3600000) return `${(diffMs / 60000).toFixed(1)}m`;
    return `${(diffMs / 3600000).toFixed(1)}h`;
  } catch {
    return "N/A";
  }
};

const getTriggerDescription = (triggerData: any) => {
  if (!triggerData) return "Unknown";

  if (triggerData.trigger) {
    return triggerData.trigger;
  }

  if (triggerData.event_type) {
    return triggerData.event_type;
  }

  return "Manual";
};

const loadExecutions = async (append = false) => {
  try {
    const runtimeConfig = useRuntimeConfig();
    const cookieToken = useCookie(
      runtimeConfig.public.sessionCookieName as string
    );
    const token = cookieToken.value;

    loading.value = true;
    error.value = null;

    if (!append) {
      executions.value = [];
      offset.value = 0;
    }

    const response = await $fetch<
      ApiResponse<{
        executions: WorkflowExecution[];
        limit: number;
        offset: number;
      }>
    >(
      `/api/servers/${props.serverId}/workflows/${props.workflowId}/executions`,
      {
        query: {
          limit: limit.value,
          offset: offset.value,
        },
        headers: {
          Authorization: `Bearer ${token}`,
        },
      }
    );

    if (response.code === 200) {
      const newExecutions = response.data.executions || [];

      if (append) {
        executions.value.push(...newExecutions);
      } else {
        executions.value = newExecutions;
      }

      hasMore.value = newExecutions.length === limit.value;
    } else {
      error.value = response.message || "Failed to load executions";
    }
  } catch (err: any) {
    error.value = err.message || "Failed to load executions";
  } finally {
    loading.value = false;
  }
};

const loadMore = () => {
  offset.value += limit.value;
  loadExecutions(true);
};

// Lifecycle
onMounted(() => {
  loadExecutions();
});
</script>
