<script setup lang="ts">
import { ref, onMounted, computed } from "vue";
import { useRoute } from "vue-router";
import {
  RefreshCw,
  Loader2,
  AlertCircle,
  ChevronDown,
  ChevronUp,
  ArrowLeft,
} from "lucide-vue-next";
import { Card, CardContent, CardHeader, CardTitle } from "~/components/ui/card";
import { Button } from "~/components/ui/button";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "~/components/ui/select";
import PageHeader from "~/components/layout/PageHeader.vue";
import ExecutionDetailsCard from "~/components/workflows/ExecutionDetailsCard.vue";
import ExecutionStepList from "~/components/workflows/ExecutionStepList.vue";
import ExecutionMessageList from "~/components/workflows/ExecutionMessageList.vue";
import type {
  ApiResponse,
  WorkflowExecution,
  WorkflowExecutionLog,
  WorkflowLogMessage,
} from "~/components/workflows/execution";
import type { PageInfo } from "~/types";

const route = useRoute();

// Route parameters
const serverId = computed(() => route.params.serverId as string);
const workflowId = computed(() => route.params.workflowId as string);
const executionId = computed(() => route.params.executionId as string);

// State
const loading = ref(false);
const logsLoading = ref(false);
const messagesLoading = ref(false);
const error = ref<string | null>(null);
const execution = ref<WorkflowExecution | null>(null);
const logs = ref<WorkflowExecutionLog[]>([]);
const messages = ref<WorkflowLogMessage[]>([]);
const logsCursor = ref<string | null>(null);
const messagesCursor = ref<string | null>(null);
const logsHasMore = ref(false);
const messagesHasMore = ref(false);
const logsPageSize = ref("100");
const messagesPageSize = ref("100");
const activeTab = ref("logs"); // "logs" or "messages"

const stepList = ref<InstanceType<typeof ExecutionStepList> | null>(null);

const loadExecution = async () => {
  try {
    loading.value = true;
    error.value = null;

    const response = await useAuthFetchImperative<
      ApiResponse<{ execution: WorkflowExecution }>
    >(
      `/api/servers/${serverId.value}/workflows/${workflowId.value}/executions/${executionId.value}`
    );

    if (response.code === 200) {
      execution.value = response.data.execution;
    } else {
      error.value = response.message || "Failed to load execution";
    }
  } catch (err: any) {
    error.value = extractApiErrorMessage(err, "Failed to load execution");
  } finally {
    loading.value = false;
  }
};

const loadMessages = async (append = false) => {
  try {
    messagesLoading.value = true;
    if (!append) {
      messages.value = [];
      messagesCursor.value = null;
      messagesHasMore.value = false;
    }

    const response = await useAuthFetchImperative<
      ApiResponse<{
        execution: WorkflowExecution;
        messages: WorkflowLogMessage[];
        page_info: PageInfo;
      }>
    >(
      `/api/servers/${serverId.value}/workflows/${workflowId.value}/executions/${executionId.value}/messages`,
      {
        query: {
          limit: parseInt(messagesPageSize.value),
          cursor: append ? messagesCursor.value || undefined : undefined,
        },
      }
    );

    if (response.code === 200) {
      let messagesData = response.data.messages;

      // Handle case where messages might be returned as an object with numeric keys
      if (
        messagesData &&
        typeof messagesData === "object" &&
        !Array.isArray(messagesData)
      ) {
        messagesData = Object.values(messagesData);
      }

      if (append) {
        messages.value.push(...(messagesData || []));
      } else {
        messages.value = messagesData || [];
      }
      messagesCursor.value = response.data.page_info?.next_cursor ?? null;
      messagesHasMore.value = response.data.page_info?.has_more ?? false;
    } else {
      error.value = response.message || "Failed to load messages";
    }
  } catch (err: any) {
    error.value = extractApiErrorMessage(err, "Failed to load messages");
  } finally {
    messagesLoading.value = false;
  }
};

const loadMoreMessages = () => {
  if (messagesHasMore.value && messagesCursor.value) {
    loadMessages(true);
  }
};

const onMessagePageSizeChange = () => {
  loadMessages();
};

const loadLogs = async (append = false) => {
  try {
    logsLoading.value = true;
    if (!append) {
      logs.value = [];
      logsCursor.value = null;
      logsHasMore.value = false;
    }

    const response = await useAuthFetchImperative<
      ApiResponse<{
        execution: WorkflowExecution;
        logs: WorkflowExecutionLog[];
        page_info: PageInfo;
      }>
    >(
      `/api/servers/${serverId.value}/workflows/${workflowId.value}/executions/${executionId.value}/logs`,
      {
        query: {
          limit: parseInt(logsPageSize.value),
          cursor: append ? logsCursor.value || undefined : undefined,
        },
      }
    );

    if (response.code === 200) {
      let logsData = response.data.logs;

      // Handle case where logs might be returned as an object with numeric keys
      if (
        logsData &&
        typeof logsData === "object" &&
        !Array.isArray(logsData)
      ) {
        logsData = Object.values(logsData);
      }

      if (append) {
        logs.value.push(...(logsData || []));
      } else {
        logs.value = logsData || [];
      }
      logsCursor.value = response.data.page_info?.next_cursor ?? null;
      logsHasMore.value = response.data.page_info?.has_more ?? false;
    } else {
      error.value = response.message || "Failed to load logs";
    }
  } catch (err: any) {
    error.value = extractApiErrorMessage(err, "Failed to load logs");
  } finally {
    logsLoading.value = false;
  }
};

const loadMoreLogs = () => {
  if (logsHasMore.value && logsCursor.value) {
    loadLogs(true);
  }
};

const onPageSizeChange = () => {
  loadLogs();
};

const refreshData = async () => {
  await Promise.all([loadExecution(), loadLogs(), loadMessages()]);
};

// Lifecycle
onMounted(() => {
  refreshData();
});

// Page metadata
definePageMeta({
  middleware: ["auth"],
  layout: "default",
});

useHead({
  title: `Workflow Execution - ${executionId.value}`,
});
</script>

<template>
  <div class="p-4 md:p-6">
    <!-- Breadcrumbs -->
    <nav
      class="mb-3 flex items-center gap-2 overflow-x-auto text-sm text-muted-foreground"
    >
      <RouterLink
        :to="`/servers/${serverId}/workflows`"
        class="whitespace-nowrap transition-colors hover:text-foreground"
      >
        Workflows
      </RouterLink>
      <span>/</span>
      <RouterLink
        :to="`/servers/${serverId}/workflows/${workflowId}`"
        class="whitespace-nowrap transition-colors hover:text-foreground"
      >
        Workflow
      </RouterLink>
      <span>/</span>
      <span class="truncate font-medium text-foreground">
        Execution {{ executionId.substring(0, 8) }}
      </span>
    </nav>

    <PageHeader title="Workflow Execution" :description="executionId">
      <template #actions>
        <RouterLink :to="`/servers/${serverId}/workflows/${workflowId}`">
          <Button variant="outline" size="sm">
            <ArrowLeft class="mr-2 h-4 w-4" />
            Back to Workflow
          </Button>
        </RouterLink>
        <Button size="sm" :disabled="loading" @click="refreshData">
          <RefreshCw class="mr-2 h-4 w-4" :class="{ 'animate-spin': loading }" />
          Refresh
        </Button>
      </template>
    </PageHeader>

    <!-- Loading State -->
    <div
      v-if="loading && !execution"
      class="flex items-center justify-center py-12"
    >
      <Loader2 class="h-8 w-8 animate-spin" />
    </div>

    <!-- Error State -->
    <div
      v-else-if="error"
      class="rounded-lg border border-red-200 bg-red-50 p-3 sm:p-4"
    >
      <div class="flex items-center gap-2 text-red-800">
        <AlertCircle class="h-3 w-3 shrink-0 sm:h-4 sm:w-4" />
        <h3 class="text-sm font-medium sm:text-base">Error</h3>
      </div>
      <p class="mt-1 break-words text-xs text-red-700 sm:text-sm">
        {{ error }}
      </p>
    </div>

    <!-- Execution Details -->
    <div v-else-if="execution" class="space-y-6">
      <!-- Status Card -->
      <ExecutionDetailsCard :execution="execution" />

      <!-- Execution Logs & Messages -->
      <Card>
        <CardHeader>
          <div
            class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between sm:gap-0"
          >
            <div class="flex flex-col gap-2 sm:flex-row sm:items-center sm:gap-4">
              <CardTitle class="text-base sm:text-lg">
                Execution Details
              </CardTitle>
              <div class="flex rounded-lg border bg-muted p-1">
                <button
                  :class="[
                    'rounded-md px-2 py-1 text-xs font-medium transition-colors sm:px-3 sm:text-sm',
                    activeTab === 'logs'
                      ? 'bg-background text-foreground shadow-sm'
                      : 'text-muted-foreground hover:text-foreground',
                  ]"
                  @click="activeTab = 'logs'"
                >
                  Step Logs
                </button>
                <button
                  :class="[
                    'rounded-md px-2 py-1 text-xs font-medium transition-colors sm:px-3 sm:text-sm',
                    activeTab === 'messages'
                      ? 'bg-background text-foreground shadow-sm'
                      : 'text-muted-foreground hover:text-foreground',
                  ]"
                  @click="activeTab = 'messages'"
                >
                  Log Messages
                </button>
              </div>
            </div>
            <div class="flex flex-wrap items-center gap-2">
              <Button
                v-if="activeTab === 'logs'"
                variant="outline"
                size="sm"
                :disabled="logsLoading"
                title="Expand all steps (Ctrl+E)"
                class="text-xs sm:text-sm"
                @click="stepList?.expandAllSteps()"
              >
                <ChevronDown class="mr-1 h-3 w-3 sm:mr-2 sm:h-4 sm:w-4" />
                <span class="hidden sm:inline">Expand All</span>
                <span class="sm:hidden">Expand</span>
              </Button>
              <Button
                v-if="activeTab === 'logs'"
                variant="outline"
                size="sm"
                :disabled="logsLoading"
                title="Collapse all steps (Ctrl+C)"
                class="text-xs sm:text-sm"
                @click="stepList?.collapseAllSteps()"
              >
                <ChevronUp class="mr-1 h-3 w-3 sm:mr-2 sm:h-4 sm:w-4" />
                <span class="hidden sm:inline">Collapse All</span>
                <span class="sm:hidden">Collapse</span>
              </Button>
              <Button
                variant="outline"
                size="sm"
                :disabled="activeTab === 'logs' ? logsLoading : messagesLoading"
                class="text-xs sm:text-sm"
                @click="activeTab === 'logs' ? loadLogs() : loadMessages()"
              >
                <RefreshCw
                  class="mr-1 h-3 w-3 sm:mr-2 sm:h-4 sm:w-4"
                  :class="{
                    'animate-spin':
                      activeTab === 'logs' ? logsLoading : messagesLoading,
                  }"
                />
                <span class="hidden sm:inline">
                  Refresh {{ activeTab === "logs" ? "Logs" : "Messages" }}
                </span>
                <span class="sm:hidden">Refresh</span>
              </Button>
              <Select
                v-if="activeTab === 'logs'"
                v-model="logsPageSize"
                @update:model-value="onPageSizeChange"
              >
                <SelectTrigger class="w-20 text-xs sm:w-24 sm:text-sm">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="25">25</SelectItem>
                  <SelectItem value="50">50</SelectItem>
                  <SelectItem value="100">100</SelectItem>
                  <SelectItem value="200">200</SelectItem>
                </SelectContent>
              </Select>
              <Select
                v-if="activeTab === 'messages'"
                v-model="messagesPageSize"
                @update:model-value="onMessagePageSizeChange"
              >
                <SelectTrigger class="w-20 text-xs sm:w-24 sm:text-sm">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="25">25</SelectItem>
                  <SelectItem value="50">50</SelectItem>
                  <SelectItem value="100">100</SelectItem>
                  <SelectItem value="200">200</SelectItem>
                </SelectContent>
              </Select>
            </div>
          </div>
        </CardHeader>
        <CardContent>
          <!-- Step Logs Tab -->
          <ExecutionStepList
            v-if="activeTab === 'logs'"
            ref="stepList"
            :logs="logs"
            :loading="logsLoading"
            :has-more="logsHasMore"
            @load-more="loadMoreLogs"
          />

          <!-- Log Messages Tab -->
          <ExecutionMessageList
            v-else-if="activeTab === 'messages'"
            :messages="messages"
            :loading="messagesLoading"
            :has-more="messagesHasMore"
            @load-more="loadMoreMessages"
          />
        </CardContent>
      </Card>
    </div>
  </div>
</template>
