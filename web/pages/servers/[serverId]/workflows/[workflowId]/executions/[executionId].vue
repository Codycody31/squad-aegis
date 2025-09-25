<template>
  <div class="p-6 space-y-6">
    <!-- Header -->
    <div class="flex items-center justify-between">
      <RouterLink :to="`/servers/${serverId}/workflows`">
        <Button variant="outline" size="sm">
          <ArrowLeft class="w-4 h-4 mr-2" />
          Back
        </Button>
      </RouterLink>
      <div>
        <h1 class="text-3xl font-bold">Workflow Execution</h1>
        <p class="text-muted-foreground">Execution ID: {{ executionId }}</p>
      </div>
      <Button @click="refreshData" :disabled="loading">
        <RefreshCw class="h-4 w-4 mr-2" :class="{ 'animate-spin': loading }" />
        Refresh
      </Button>
    </div>

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
      class="border border-red-200 bg-red-50 p-4 rounded-lg"
    >
      <div class="flex items-center gap-2 text-red-800">
        <AlertCircle class="h-4 w-4" />
        <h3 class="font-medium">Error</h3>
      </div>
      <p class="text-red-700 mt-1">{{ error }}</p>
    </div>

    <!-- Execution Details -->
    <div v-else-if="execution" class="space-y-6">
      <!-- Status Card -->
      <Card>
        <CardHeader>
          <CardTitle class="flex items-center gap-2">
            <component
              :is="getStatusIcon(execution.status)"
              :class="getStatusColor(execution.status)"
              class="h-5 w-5"
            />
            Execution Status
          </CardTitle>
        </CardHeader>
        <CardContent class="space-y-4">
          <div class="grid grid-cols-1 md:grid-cols-3 gap-4">
            <div>
              <Label class="text-sm font-medium">Status</Label>
              <p :class="getStatusColor(execution.status)" class="font-medium">
                {{ execution.status }}
              </p>
            </div>
            <div>
              <Label class="text-sm font-medium">Started At</Label>
              <p>{{ formatDateTime(execution.started_at) }}</p>
            </div>
            <div>
              <Label class="text-sm font-medium">Duration</Label>
              <p>
                {{
                  formatDuration(execution.started_at, execution.completed_at)
                }}
              </p>
            </div>
          </div>

          <div v-if="execution.trigger_data" class="space-y-2">
            <Label class="text-sm font-medium">Trigger Data</Label>
            <pre class="bg-muted p-3 rounded-md text-sm overflow-auto">{{
              JSON.stringify(execution.trigger_data, null, 2)
            }}</pre>
          </div>

          <div v-if="execution.error" class="space-y-2">
            <Label class="text-sm font-medium text-red-600"
              >Error Message</Label
            >
            <div class="border border-red-200 bg-red-50 p-3 rounded-lg">
              <div class="flex items-center gap-2 text-red-800">
                <AlertCircle class="h-4 w-4" />
                <p class="text-red-700">{{ execution.error }}</p>
              </div>
            </div>
          </div>
        </CardContent>
      </Card>

      <!-- Execution Logs & Messages -->
      <Card>
        <CardHeader>
          <div class="flex items-center justify-between">
            <div class="flex items-center gap-4">
              <CardTitle>Execution Details</CardTitle>
              <div class="flex border rounded-lg p-1 bg-muted">
                <button
                  @click="activeTab = 'logs'"
                  :class="[
                    'px-3 py-1 text-sm font-medium rounded-md transition-colors',
                    activeTab === 'logs'
                      ? 'bg-background text-foreground shadow-sm'
                      : 'text-muted-foreground hover:text-foreground',
                  ]"
                >
                  Step Logs
                </button>
                <button
                  @click="activeTab = 'messages'"
                  :class="[
                    'px-3 py-1 text-sm font-medium rounded-md transition-colors',
                    activeTab === 'messages'
                      ? 'bg-background text-foreground shadow-sm'
                      : 'text-muted-foreground hover:text-foreground',
                  ]"
                >
                  Log Messages
                </button>
              </div>
            </div>
            <div class="flex items-center gap-2">
              <Button
                v-if="activeTab === 'logs'"
                variant="outline"
                size="sm"
                @click="expandAllSteps"
                :disabled="logsLoading"
                title="Expand all steps (Ctrl+E)"
              >
                <ChevronDown class="h-4 w-4 mr-2" />
                Expand All
              </Button>
              <Button
                v-if="activeTab === 'logs'"
                variant="outline"
                size="sm"
                @click="collapseAllSteps"
                :disabled="logsLoading"
                title="Collapse all steps (Ctrl+C)"
              >
                <ChevronUp class="h-4 w-4 mr-2" />
                Collapse All
              </Button>
              <Button
                variant="outline"
                size="sm"
                @click="activeTab === 'logs' ? loadLogs() : loadMessages()"
                :disabled="activeTab === 'logs' ? logsLoading : messagesLoading"
              >
                <RefreshCw
                  class="h-4 w-4 mr-2"
                  :class="{
                    'animate-spin':
                      activeTab === 'logs' ? logsLoading : messagesLoading,
                  }"
                />
                Refresh {{ activeTab === "logs" ? "Logs" : "Messages" }}
              </Button>
              <Select
                v-if="activeTab === 'logs'"
                v-model="logsPageSize"
                @update:model-value="onPageSizeChange"
              >
                <SelectTrigger class="w-24">
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
                <SelectTrigger class="w-24">
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
          <div v-if="activeTab === 'logs'">
            <!-- Logs Loading -->
            <div
              v-if="logsLoading && logs.length === 0"
              class="flex items-center justify-center py-8"
            >
              <Loader2 class="h-6 w-6 animate-spin" />
            </div>

            <!-- Logs List -->
            <div v-else-if="logs.length > 0" class="space-y-4">
              <div
                v-for="log in logs"
                :key="log.step_order"
                class="border rounded-lg overflow-hidden"
              >
                <!-- Step Header (Always Visible) -->
                <div
                  class="p-4 cursor-pointer hover:bg-muted/50 transition-colors"
                  @click="toggleStepCollapse(log.step_order)"
                >
                  <div class="flex items-start justify-between">
                    <div class="flex items-center gap-2">
                      <component
                        :is="
                          isStepCollapsed(log.step_order)
                            ? ChevronRight
                            : ChevronDown
                        "
                        class="h-4 w-4 text-muted-foreground transition-transform"
                      />
                      <Badge variant="outline"
                        >Step {{ log.step_order + 1 }}</Badge
                      >
                      <span class="font-medium">{{ log.step_name }}</span>
                      <span class="text-sm text-muted-foreground">
                        ({{ log.step_type }})
                      </span>
                    </div>
                    <div class="flex items-center gap-4">
                      <div class="flex items-center gap-2 text-sm">
                        <span
                          :class="getStatusColor(log.step_status)"
                          class="font-medium"
                        >
                          {{ log.step_status }}
                        </span>
                        <span class="text-muted-foreground">•</span>
                        <span class="text-muted-foreground">{{
                          formatDurationMs(log.step_duration_ms)
                        }}</span>
                      </div>
                      <div
                        class="flex items-center gap-2 text-sm text-muted-foreground"
                      >
                        <Clock class="h-4 w-4" />
                        {{ formatDateTime(log.event_time) }}
                      </div>
                    </div>
                  </div>
                </div>

                <!-- Step Details (Collapsible) -->
                <div
                  v-if="!isStepCollapsed(log.step_order)"
                  class="px-4 pb-4 space-y-3 border-t bg-muted/25"
                >
                  <div
                    class="grid grid-cols-1 md:grid-cols-3 gap-4 text-sm pt-3"
                  >
                    <div>
                      <Label class="text-xs font-medium">Status</Label>
                      <p
                        :class="getStatusColor(log.step_status)"
                        class="font-medium"
                      >
                        {{ log.step_status }}
                      </p>
                    </div>
                    <div>
                      <Label class="text-xs font-medium">Duration</Label>
                      <p>{{ formatDurationMs(log.step_duration_ms) }}</p>
                    </div>
                    <div v-if="log.step_output">
                      <Label class="text-xs font-medium">Output Size</Label>
                      <p>
                        {{ JSON.stringify(log.step_output).length }} characters
                      </p>
                    </div>
                  </div>

                  <!-- Step Input -->
                  <div v-if="log.step_input" class="space-y-2">
                    <Label class="text-xs font-medium">Input</Label>
                    <pre
                      class="bg-muted p-2 rounded text-xs overflow-auto max-h-32"
                      >{{ JSON.stringify(log.step_input, null, 2) }}</pre
                    >
                  </div>

                  <!-- Step Output -->
                  <div v-if="log.step_output" class="space-y-2">
                    <Label class="text-xs font-medium">Output</Label>
                    <pre
                      class="bg-muted p-2 rounded text-xs overflow-auto max-h-32"
                      >{{ JSON.stringify(log.step_output, null, 2) }}</pre
                    >
                  </div>

                  <!-- Trigger Event Data -->
                  <div v-if="log.trigger_event_data" class="space-y-2">
                    <Label class="text-xs font-medium">Trigger Event</Label>
                    <pre
                      class="bg-muted p-2 rounded text-xs overflow-auto max-h-32"
                      >{{
                        JSON.stringify(log.trigger_event_data, null, 2)
                      }}</pre
                    >
                  </div>

                  <!-- Variables -->
                  <div
                    v-if="
                      log.variables && Object.keys(log.variables).length > 0
                    "
                    class="space-y-2"
                  >
                    <Label class="text-xs font-medium">Variables</Label>
                    <pre
                      class="bg-muted p-2 rounded text-xs overflow-auto max-h-32"
                      >{{ JSON.stringify(log.variables, null, 2) }}</pre
                    >
                  </div>

                  <!-- Metadata -->
                  <div v-if="log.metadata" class="space-y-2">
                    <Label class="text-xs font-medium">Metadata</Label>
                    <pre
                      class="bg-muted p-2 rounded text-xs overflow-auto max-h-32"
                      >{{ JSON.stringify(log.metadata, null, 2) }}</pre
                    >
                  </div>

                  <!-- Step Error -->
                  <div v-if="log.error" class="space-y-2">
                    <Label class="text-xs font-medium text-red-600"
                      >Error</Label
                    >
                    <div
                      class="border border-red-200 bg-red-50 p-2 rounded text-sm"
                    >
                      <div class="flex items-center gap-2 text-red-800">
                        <AlertCircle class="h-4 w-4" />
                        <p class="text-red-700">{{ log.error }}</p>
                      </div>
                    </div>
                  </div>
                </div>
              </div>

              <!-- Pagination -->
              <div
                v-if="totalLogs > logs.length"
                class="flex justify-center pt-4"
              >
                <Button
                  variant="outline"
                  @click="loadMoreLogs"
                  :disabled="logsLoading"
                >
                  <ChevronDown class="h-4 w-4 mr-2" />
                  Load More Logs ({{ totalLogs - logs.length }} remaining)
                </Button>
              </div>
            </div>

            <!-- No Logs -->
            <div v-else class="text-center py-8 text-muted-foreground">
              <FileText class="h-12 w-12 mx-auto mb-4 opacity-50" />
              <p>No execution logs found</p>
            </div>
          </div>

          <!-- Log Messages Tab -->
          <div v-else-if="activeTab === 'messages'">
            <!-- Messages Loading -->
            <div
              v-if="messagesLoading && messages.length === 0"
              class="flex items-center justify-center py-8"
            >
              <Loader2 class="h-6 w-6 animate-spin" />
            </div>

            <!-- Messages List -->
            <div v-else-if="messages.length > 0" class="space-y-2">
              <div
                v-for="message in messages"
                :key="`${message.step_id}-${message.log_time}`"
                class="border rounded-lg p-4 hover:bg-muted/25 transition-colors"
              >
                <div class="flex items-start justify-between gap-4">
                  <div class="flex items-start gap-3 flex-1 min-w-0">
                    <!-- Log Level Badge -->
                    <Badge
                      variant="outline"
                      :class="getLogLevelColor(message.log_level)"
                      class="shrink-0 font-mono text-xs"
                    >
                      {{ message.log_level }}
                    </Badge>

                    <!-- Message Content -->
                    <div class="flex-1 min-w-0">
                      <div class="flex items-center gap-2 text-sm mb-1">
                        <span class="font-medium text-foreground">{{
                          message.step_name
                        }}</span>
                        <span class="text-muted-foreground">•</span>
                        <span class="text-muted-foreground">{{
                          formatDateTime(message.log_time)
                        }}</span>
                      </div>
                      <p class="text-sm text-foreground break-words">
                        {{ message.message }}
                      </p>

                      <!-- Variables and Metadata (if present) -->
                      <div
                        v-if="
                          message.variables &&
                          Object.keys(message.variables).length > 0
                        "
                        class="mt-2"
                      >
                        <details class="text-xs">
                          <summary
                            class="text-muted-foreground cursor-pointer hover:text-foreground"
                          >
                            Variables
                          </summary>
                          <pre
                            class="bg-muted p-2 rounded mt-1 overflow-auto max-h-24"
                            >{{
                              JSON.stringify(message.variables, null, 2)
                            }}</pre
                          >
                        </details>
                      </div>

                      <div
                        v-if="
                          message.metadata &&
                          Object.keys(message.metadata).length > 0
                        "
                        class="mt-2"
                      >
                        <details class="text-xs">
                          <summary
                            class="text-muted-foreground cursor-pointer hover:text-foreground"
                          >
                            Metadata
                          </summary>
                          <pre
                            class="bg-muted p-2 rounded mt-1 overflow-auto max-h-24"
                            >{{
                              JSON.stringify(message.metadata, null, 2)
                            }}</pre
                          >
                        </details>
                      </div>
                    </div>
                  </div>
                </div>
              </div>

              <!-- Messages Pagination -->
              <div
                v-if="totalMessages > messages.length"
                class="flex justify-center pt-4"
              >
                <Button
                  variant="outline"
                  @click="loadMoreMessages"
                  :disabled="messagesLoading"
                >
                  <ChevronDown class="h-4 w-4 mr-2" />
                  Load More Messages ({{ totalMessages - messages.length }}
                  remaining)
                </Button>
              </div>
            </div>

            <!-- No Messages -->
            <div v-else class="text-center py-8 text-muted-foreground">
              <FileText class="h-12 w-12 mx-auto mb-4 opacity-50" />
              <p>No log messages found</p>
              <p class="text-sm mt-2">
                Log messages are generated by
                <code class="bg-muted px-1 rounded text-xs">log()</code>,
                <code class="bg-muted px-1 rounded text-xs">log_debug()</code>,
                <code class="bg-muted px-1 rounded text-xs">log_warn()</code>,
                and
                <code class="bg-muted px-1 rounded text-xs">log_error()</code>
                functions in Lua scripts.
              </p>
            </div>
          </div>
        </CardContent>
      </Card>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, onUnmounted, computed } from "vue";
import { useRoute, useRouter } from "vue-router";
import {
  ChevronRight,
  RefreshCw,
  Loader2,
  AlertCircle,
  Clock,
  ChevronDown,
  ChevronUp,
  FileText,
  CheckCircle,
  XCircle,
  PlayCircle,
  ArrowLeft,
} from "lucide-vue-next";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Label } from "@/components/ui/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";

interface WorkflowExecution {
  id: string;
  workflow_id: string;
  status: string;
  started_at: string;
  completed_at?: string;
  trigger_data?: any;
  error?: string;
}

interface WorkflowExecutionLog {
  execution_id: string;
  workflow_id: string;
  server_id: string;
  event_time: string;
  trigger_event_type: string;
  trigger_event_data?: any;
  status: string;
  step_name: string;
  step_type: string;
  step_order: number;
  step_status: string;
  step_input?: any;
  step_output?: any;
  step_duration_ms: number;
  variables?: any;
  metadata?: any;
  error?: string;
}

interface WorkflowLogMessage {
  execution_id: string;
  workflow_id: string;
  server_id: string;
  step_id: string;
  step_name: string;
  log_time: string;
  log_level: string;
  message: string;
  variables?: any;
  metadata?: any;
}

interface ApiResponse<T = any> {
  success: boolean;
  message?: string;
  data: T;
  code?: number;
}

const route = useRoute();
const router = useRouter();

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
const totalLogs = ref(0);
const totalMessages = ref(0);
const logsOffset = ref(0);
const messagesOffset = ref(0);
const logsPageSize = ref("50");
const messagesPageSize = ref("50");
const collapsedSteps = ref<Set<number>>(new Set());
const activeTab = ref("logs"); // "logs" or "messages"

// Computed
const serverName = ref("Server");
const workflowName = ref("Workflow");

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

const getLogLevelColor = (level: string) => {
  switch (level.toUpperCase()) {
    case "DEBUG":
      return "text-gray-600 bg-gray-100";
    case "INFO":
      return "text-blue-600 bg-blue-100";
    case "WARN":
      return "text-yellow-600 bg-yellow-100";
    case "ERROR":
      return "text-red-600 bg-red-100";
    default:
      return "text-gray-600 bg-gray-100";
  }
};

const getLogLevelIcon = (level: string) => {
  switch (level.toUpperCase()) {
    case "DEBUG":
      return "text-gray-500";
    case "INFO":
      return "text-blue-500";
    case "WARN":
      return "text-yellow-500";
    case "ERROR":
      return "text-red-500";
    default:
      return "text-gray-500";
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

const formatDurationMs = (durationMs: number) => {
  if (durationMs === 0) return "0ms";
  if (durationMs < 1000) return `${durationMs}ms`;
  if (durationMs < 60000) return `${(durationMs / 1000).toFixed(1)}s`;
  if (durationMs < 3600000) return `${(durationMs / 60000).toFixed(1)}m`;
  return `${(durationMs / 3600000).toFixed(1)}h`;
};

const toggleStepCollapse = (stepOrder: number) => {
  if (collapsedSteps.value.has(stepOrder)) {
    collapsedSteps.value.delete(stepOrder);
  } else {
    collapsedSteps.value.add(stepOrder);
  }
};

const isStepCollapsed = (stepOrder: number) => {
  return collapsedSteps.value.has(stepOrder);
};

const collapseAllSteps = () => {
  logs.value.forEach((log) => {
    collapsedSteps.value.add(log.step_order);
  });
};

const expandAllSteps = () => {
  collapsedSteps.value.clear();
};

const loadExecution = async () => {
  try {
    const runtimeConfig = useRuntimeConfig();
    const cookieToken = useCookie(
      runtimeConfig.public.sessionCookieName as string
    );
    const token = cookieToken.value;

    loading.value = true;
    error.value = null;

    const response = await $fetch<
      ApiResponse<{ execution: WorkflowExecution }>
    >(
      `/api/servers/${serverId.value}/workflows/${workflowId.value}/executions/${executionId.value}`,
      {
        headers: {
          Authorization: token ? `Bearer ${token}` : "",
        },
      }
    );

    if (response.code === 200) {
      execution.value = response.data.execution;
    } else {
      error.value = response.message || "Failed to load execution";
    }
  } catch (err: any) {
    error.value = err.message || "Failed to load execution";
  } finally {
    loading.value = false;
  }
};

const loadMessages = async (append = false) => {
  try {
    const runtimeConfig = useRuntimeConfig();
    const cookieToken = useCookie(
      runtimeConfig.public.sessionCookieName as string
    );
    const token = cookieToken.value;

    messagesLoading.value = true;
    if (!append) {
      messages.value = [];
      messagesOffset.value = 0;
    }

    const response = await $fetch<
      ApiResponse<{
        execution: WorkflowExecution;
        messages: WorkflowLogMessage[];
        limit: number;
        offset: number;
      }>
    >(
      `/api/servers/${serverId.value}/workflows/${workflowId.value}/executions/${executionId.value}/messages`,
      {
        query: {
          limit: parseInt(messagesPageSize.value),
          offset: messagesOffset.value,
        },
        headers: {
          Authorization: token ? `Bearer ${token}` : "",
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
      totalMessages.value = messagesData?.length || 0;
    } else {
      error.value = response.message || "Failed to load messages";
    }
  } catch (err: any) {
    error.value = err.message || "Failed to load messages";
  } finally {
    messagesLoading.value = false;
  }
};

const loadMoreMessages = () => {
  messagesOffset.value += parseInt(messagesPageSize.value);
  loadMessages(true);
};

const onMessagePageSizeChange = () => {
  loadMessages();
};

const loadLogs = async (append = false) => {
  try {
    const runtimeConfig = useRuntimeConfig();
    const cookieToken = useCookie(
      runtimeConfig.public.sessionCookieName as string
    );
    const token = cookieToken.value;

    logsLoading.value = true;
    if (!append) {
      logs.value = [];
      logsOffset.value = 0;
    }

    const response = await $fetch<
      ApiResponse<{
        execution: WorkflowExecution;
        logs: WorkflowExecutionLog[];
        limit: number;
        offset: number;
      }>
    >(
      `/api/servers/${serverId.value}/workflows/${workflowId.value}/executions/${executionId.value}/logs`,
      {
        query: {
          limit: parseInt(logsPageSize.value),
          offset: logsOffset.value,
        },
        headers: {
          Authorization: token ? `Bearer ${token}` : "",
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
      totalLogs.value = logsData?.length || 0;
    } else {
      error.value = response.message || "Failed to load logs";
    }
  } catch (err: any) {
    error.value = err.message || "Failed to load logs";
  } finally {
    logsLoading.value = false;
  }
};

const loadMoreLogs = () => {
  logsOffset.value += parseInt(logsPageSize.value);
  loadLogs(true);
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
  layout: "default",
});

useHead({
  title: `Workflow Execution - ${executionId.value}`,
});
</script>
