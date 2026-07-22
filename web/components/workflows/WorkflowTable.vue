<script setup lang="ts">
import { Activity, Eye, ExternalLink, Pause, Play, Trash2 } from "lucide-vue-next";
import { Badge } from "~/components/ui/badge";
import { Button } from "~/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "~/components/ui/card";
import DataTable from "~/components/data-table/DataTable.vue";
import type { DataTableColumn } from "~/components/data-table/types";
import type { Workflow } from "~/components/workflows/types";

defineProps<{
  workflows: Workflow[];
  loading?: boolean;
}>();

const emit = defineEmits<{
  (e: "open", workflow: Workflow): void;
  (e: "toggle", workflow: Workflow): void;
  (e: "delete", workflow: Workflow): void;
}>();

const columns: DataTableColumn<Workflow>[] = [
  { key: "name", label: "Name", sortable: true },
  {
    key: "description",
    label: "Description",
    accessor: (w) => w.description || "No description",
  },
  { key: "status", label: "Status", accessor: (w) => (w.enabled ? "Active" : "Inactive") },
  {
    key: "triggers",
    label: "Triggers",
    accessor: (w) => w.definition?.triggers?.length || 0,
  },
  {
    key: "steps",
    label: "Steps",
    accessor: (w) => w.definition?.steps?.length || 0,
  },
  {
    key: "updated_at",
    label: "Last Updated",
    sortable: true,
    accessor: (w) => w.updated_at,
    class: "text-xs sm:text-sm",
  },
];

// Format date for display
function formatDate(dateString: string) {
  return new Date(dateString).toLocaleString();
}
</script>

<template>
  <Card>
    <CardHeader>
      <CardTitle class="text-base sm:text-lg">Workflows</CardTitle>
    </CardHeader>
    <CardContent>
      <DataTable
        :columns="columns"
        :rows="workflows"
        row-key="id"
        :loading="loading"
        @row-click="emit('open', $event)"
      >
        <template #cell-name="{ row }">
          <div class="flex items-center gap-2 font-medium">
            <span>{{ row.name }}</span>
            <ExternalLink class="h-3 w-3 text-muted-foreground" />
          </div>
        </template>

        <template #cell-status="{ row }">
          <Badge :variant="row.enabled ? 'default' : 'secondary'">
            {{ row.enabled ? "Active" : "Inactive" }}
          </Badge>
        </template>

        <template #cell-updated_at="{ row }">
          {{ formatDate(row.updated_at) }}
        </template>

        <template #actions="{ row }">
          <div class="flex items-center justify-end gap-2">
            <Button
              variant="ghost"
              size="sm"
              title="View Details"
              @click="emit('open', row)"
            >
              <Eye class="h-4 w-4" />
            </Button>
            <Button
              variant="ghost"
              size="sm"
              title="Toggle Enable/Disable"
              @click="emit('toggle', row)"
            >
              <component :is="row.enabled ? Pause : Play" class="h-4 w-4" />
            </Button>
            <Button
              variant="ghost"
              size="sm"
              title="Delete Workflow"
              @click="emit('delete', row)"
            >
              <Trash2 class="h-4 w-4" />
            </Button>
          </div>
        </template>

        <template #empty>
          <div class="py-6 text-center text-muted-foreground sm:py-8">
            <Activity
              class="mx-auto mb-3 h-12 w-12 opacity-25 sm:mb-4 sm:h-16 sm:w-16"
            />
            <p class="text-sm sm:text-base">No workflows found</p>
            <p class="mt-1 text-xs sm:text-sm">
              Create your first workflow to automate server actions
            </p>
          </div>
        </template>
      </DataTable>
    </CardContent>
  </Card>
</template>
