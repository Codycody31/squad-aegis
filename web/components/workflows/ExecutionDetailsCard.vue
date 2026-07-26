<script setup lang="ts">
import { AlertCircle } from "lucide-vue-next";
import { Card, CardContent, CardHeader, CardTitle } from "~/components/ui/card";
import { Label } from "~/components/ui/label";
import {
  formatDateTime,
  formatDuration,
  getStatusColor,
  getStatusIcon,
  syntaxHighlight,
  type WorkflowExecution,
} from "~/components/workflows/execution";

defineProps<{
  execution: WorkflowExecution;
}>();
</script>

<template>
  <Card>
    <CardHeader>
      <CardTitle class="flex items-center gap-2 text-base sm:text-lg">
        <component
          :is="getStatusIcon(execution.status)"
          :class="getStatusColor(execution.status)"
          class="h-4 w-4 sm:h-5 sm:w-5"
        />
        Execution Status
      </CardTitle>
    </CardHeader>
    <CardContent class="space-y-3 sm:space-y-4">
      <div class="grid grid-cols-1 gap-3 sm:gap-4 md:grid-cols-3">
        <div>
          <Label class="text-xs font-medium sm:text-sm">Status</Label>
          <p
            :class="getStatusColor(execution.status)"
            class="text-sm font-medium sm:text-base"
          >
            {{ execution.status }}
          </p>
        </div>
        <div>
          <Label class="text-xs font-medium sm:text-sm">Started At</Label>
          <p class="text-xs sm:text-sm">
            {{ formatDateTime(execution.started_at) }}
          </p>
        </div>
        <div>
          <Label class="text-xs font-medium sm:text-sm">Duration</Label>
          <p class="text-xs sm:text-sm">
            {{ formatDuration(execution.started_at, execution.completed_at) }}
          </p>
        </div>
      </div>

      <div v-if="execution.trigger_data" class="space-y-2">
        <Label class="text-xs font-medium sm:text-sm">Trigger Data</Label>
        <div
          class="overflow-auto whitespace-pre-wrap break-words rounded-md bg-muted p-2 font-mono text-xs sm:p-3 sm:text-sm"
          v-html="
            syntaxHighlight(JSON.stringify(execution.trigger_data, null, 2))
          "
        ></div>
      </div>

      <div v-if="execution.error" class="space-y-2">
        <Label class="text-xs font-medium text-red-600 sm:text-sm">
          Error Message
        </Label>
        <div class="rounded-lg border border-red-200 bg-red-50 p-2 sm:p-3">
          <div class="flex items-center gap-2 text-red-800">
            <AlertCircle class="h-3 w-3 shrink-0 sm:h-4 sm:w-4" />
            <p class="break-words text-xs text-red-700 sm:text-sm">
              {{ execution.error }}
            </p>
          </div>
        </div>
      </div>
    </CardContent>
  </Card>
</template>
