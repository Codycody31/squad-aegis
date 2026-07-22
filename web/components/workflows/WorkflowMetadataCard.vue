<script setup lang="ts">
import { Card, CardContent, CardHeader, CardTitle } from "~/components/ui/card";
import type { Workflow } from "~/components/workflows/types";

defineProps<{
  workflow: Workflow;
}>();

const formatDateTime = (dateStr: string) => {
  if (!dateStr) return "N/A";
  try {
    return new Date(dateStr).toLocaleString();
  } catch {
    return dateStr;
  }
};
</script>

<template>
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
</template>
