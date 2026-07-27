<script setup lang="ts">
import { ref, computed } from "vue";
import {
  AlertCircle,
  ChevronDown,
  ChevronRight,
  Clock,
  FileText,
  Loader2,
} from "lucide-vue-next";
import { Badge } from "~/components/ui/badge";
import { Button } from "~/components/ui/button";
import { Label } from "~/components/ui/label";
import {
  formatDateTime,
  formatDurationMs,
  getStatusColor,
  syntaxHighlight,
  type WorkflowExecutionLog,
} from "~/components/workflows/execution";

const props = defineProps<{
  logs: WorkflowExecutionLog[];
  loading: boolean;
  hasMore: boolean;
}>();

const emit = defineEmits<{
  (e: "load-more"): void;
}>();

const collapsedSteps = ref<Set<number>>(new Set());

// Keep the API's stable cursor order. Re-sorting by status would move entries
// across the `(step_order, event_time, entry_id)` boundary used by pagination.
const sortedLogs = computed(() => {
  return [...props.logs].sort((a, b) => {
    if (a.step_order !== b.step_order) {
      return a.step_order - b.step_order;
    }

    const eventTimeOrder =
      new Date(a.event_time).getTime() - new Date(b.event_time).getTime();
    if (eventTimeOrder !== 0) {
      return eventTimeOrder;
    }

    return a.entry_id.localeCompare(b.entry_id);
  });
});

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
  props.logs.forEach((log) => {
    collapsedSteps.value.add(log.step_order);
  });
};

const expandAllSteps = () => {
  collapsedSteps.value.clear();
};

defineExpose({ expandAllSteps, collapseAllSteps });
</script>

<template>
  <div>
    <!-- Logs Loading -->
    <div
      v-if="loading && logs.length === 0"
      class="flex items-center justify-center py-8"
    >
      <Loader2 class="h-6 w-6 animate-spin" />
    </div>

    <!-- Logs List -->
    <div v-else-if="logs.length > 0" class="space-y-4">
      <div
        v-for="log in sortedLogs"
        :key="log.entry_id"
        class="overflow-hidden rounded-lg border"
      >
        <!-- Step Header (Always Visible) -->
        <div
          class="cursor-pointer p-3 transition-colors hover:bg-muted/50 sm:p-4"
          @click="toggleStepCollapse(log.step_order)"
        >
          <div
            class="flex flex-col gap-2 sm:flex-row sm:items-start sm:justify-between sm:gap-0"
          >
            <div class="flex flex-wrap items-center gap-2">
              <component
                :is="isStepCollapsed(log.step_order) ? ChevronRight : ChevronDown"
                class="h-3 w-3 shrink-0 text-muted-foreground transition-transform sm:h-4 sm:w-4"
              />
              <Badge variant="outline" class="text-xs sm:text-sm">
                Step {{ log.step_order + 1 }}
              </Badge>
              <span class="text-sm font-medium sm:text-base">
                {{ log.step_name }}
              </span>
              <span class="text-xs text-muted-foreground sm:text-sm">
                ({{ log.step_type }})
              </span>
            </div>
            <div class="flex flex-col gap-2 sm:flex-row sm:items-center sm:gap-4">
              <div class="flex items-center gap-2 text-xs sm:text-sm">
                <span :class="getStatusColor(log.step_status)" class="font-medium">
                  {{ log.step_status }}
                </span>
                <span class="text-muted-foreground">•</span>
                <span class="text-muted-foreground">
                  {{ formatDurationMs(log.step_duration_ms) }}
                </span>
              </div>
              <div
                class="flex items-center gap-2 text-xs text-muted-foreground sm:text-sm"
              >
                <Clock class="h-3 w-3 sm:h-4 sm:w-4" />
                {{ formatDateTime(log.event_time) }}
              </div>
            </div>
          </div>
        </div>

        <!-- Step Details (Collapsible) -->
        <div
          v-if="!isStepCollapsed(log.step_order)"
          class="space-y-2 border-t bg-muted/25 px-3 pb-3 sm:space-y-3 sm:px-4 sm:pb-4"
        >
          <div
            class="grid grid-cols-1 gap-3 pt-2 text-xs sm:gap-4 sm:pt-3 sm:text-sm md:grid-cols-3"
          >
            <div>
              <Label class="text-xs font-medium">Status</Label>
              <p :class="getStatusColor(log.step_status)" class="font-medium">
                {{ log.step_status }}
              </p>
            </div>
            <div>
              <Label class="text-xs font-medium">Duration</Label>
              <p>{{ formatDurationMs(log.step_duration_ms) }}</p>
            </div>
            <div v-if="log.step_output">
              <Label class="text-xs font-medium">Output Size</Label>
              <p>{{ JSON.stringify(log.step_output).length }} characters</p>
            </div>
          </div>

          <!-- For Running Steps: Show Input, Trigger Event, Variables, Metadata (but NOT Output) -->
          <template
            v-if="
              log.step_status.toLowerCase() === 'running' ||
              log.step_status.toLowerCase() === 'executing'
            "
          >
            <!-- Step Input -->
            <div v-if="log.step_input" class="space-y-2">
              <Label class="text-xs font-medium">Input</Label>
              <div
                class="max-h-32 overflow-auto whitespace-pre-wrap break-words rounded bg-muted p-2 font-mono text-xs"
                v-html="
                  syntaxHighlight(JSON.stringify(log.step_input, null, 2))
                "
              ></div>
            </div>

            <!-- Variables -->
            <div
              v-if="log.variables && Object.keys(log.variables).length > 0"
              class="space-y-2"
            >
              <Label class="text-xs font-medium">Variables</Label>
              <div
                class="max-h-32 overflow-auto whitespace-pre-wrap break-words rounded bg-muted p-2 font-mono text-xs"
                v-html="syntaxHighlight(JSON.stringify(log.variables, null, 2))"
              ></div>
            </div>

            <!-- Metadata -->
            <div v-if="log.metadata" class="space-y-2">
              <Label class="text-xs font-medium">Metadata</Label>
              <div
                class="max-h-32 overflow-auto whitespace-pre-wrap break-words rounded bg-muted p-2 font-mono text-xs"
                v-html="syntaxHighlight(JSON.stringify(log.metadata, null, 2))"
              ></div>
            </div>
          </template>

          <!-- For Completed Steps: Show Output only (no Input, Trigger Event, Variables, or Metadata) -->
          <template
            v-else-if="
              log.step_status.toLowerCase() === 'completed' ||
              log.step_status.toLowerCase() === 'failed' ||
              log.step_status.toLowerCase() === 'error'
            "
          >
            <!-- Step Output -->
            <div
              v-if="log.step_output && Object.keys(log.step_output).length > 0"
              class="space-y-2"
            >
              <Label class="text-xs font-medium">Output</Label>
              <div
                class="max-h-32 overflow-auto whitespace-pre-wrap break-words rounded bg-muted p-2 font-mono text-xs"
                v-html="
                  syntaxHighlight(JSON.stringify(log.step_output, null, 2))
                "
              ></div>
            </div>
          </template>

          <!-- Step Error -->
          <div v-if="log.error" class="space-y-2">
            <Label class="text-xs font-medium text-red-600">Error</Label>
            <div class="rounded border border-red-200 bg-red-50 p-2 text-sm">
              <div class="flex items-center gap-2 text-red-800">
                <AlertCircle class="h-4 w-4" />
                <p class="text-red-700">{{ log.error }}</p>
              </div>
            </div>
          </div>
        </div>
      </div>

      <!-- Pagination -->
      <div v-if="hasMore" class="flex justify-center pt-4">
        <Button
          variant="outline"
          :disabled="loading"
          class="w-full text-xs sm:w-auto sm:text-sm"
          @click="emit('load-more')"
        >
          <ChevronDown class="mr-1 h-3 w-3 sm:mr-2 sm:h-4 sm:w-4" />
          Load More Logs
        </Button>
      </div>
    </div>

    <!-- No Logs -->
    <div v-else class="py-6 text-center text-muted-foreground sm:py-8">
      <FileText class="mx-auto mb-3 h-8 w-8 opacity-50 sm:mb-4 sm:h-12 sm:w-12" />
      <p class="text-sm sm:text-base">No execution logs found</p>
    </div>
  </div>
</template>
