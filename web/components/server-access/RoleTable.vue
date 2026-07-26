<script setup lang="ts">
import { Badge } from "~/components/ui/badge";
import { Button } from "~/components/ui/button";
import DataTable from "~/components/data-table/DataTable.vue";
import type { DataTableColumn } from "~/components/data-table/types";
import {
  formatDate,
  formatPermissionDisplay,
  getPermissionCategoryColor,
  type PermissionGroup,
  type ServerRole,
} from "./shared";

defineProps<{
  roles: ServerRole[];
  loading?: boolean;
  permissionGroups: PermissionGroup[];
}>();

const emit = defineEmits<{
  (e: "edit", role: ServerRole): void;
  (e: "delete", role: ServerRole): void;
}>();

const columns: DataTableColumn<ServerRole>[] = [
  { key: "name", label: "Role Name" },
  {
    key: "type",
    label: "Type",
    accessor: (row) => (row.is_admin ? "Admin" : "Non-Admin"),
  },
  { key: "permissions", label: "Permissions" },
  {
    key: "created_at",
    label: "Created At",
    accessor: (row) => formatDate(row.created_at),
  },
];
</script>

<template>
  <DataTable
    :columns="columns"
    :rows="roles"
    row-key="id"
    :loading="loading"
    empty-text="No roles found. Create a role to get started."
  >
    <template #cell-name="{ row }">
      <span class="font-medium">{{ row.name }}</span>
    </template>

    <template #cell-type="{ row }">
      <Badge :variant="row.is_admin ? 'default' : 'secondary'" class="text-xs">
        {{ row.is_admin ? "Admin" : "Non-Admin" }}
      </Badge>
    </template>

    <template #cell-permissions="{ row }">
      <div class="flex flex-wrap gap-1 max-w-md">
        <Badge
          v-for="permission in row.permissions.slice(0, 5)"
          :key="permission"
          variant="outline"
          :class="['text-xs', getPermissionCategoryColor(permission)]"
        >
          {{ formatPermissionDisplay(permission, permissionGroups) }}
        </Badge>
        <Badge
          v-if="row.permissions.length > 5"
          variant="outline"
          class="text-xs"
        >
          +{{ row.permissions.length - 5 }} more
        </Badge>
      </div>
    </template>

    <template #actions="{ row }">
      <div class="flex gap-2 justify-end">
        <Button
          variant="outline"
          size="sm"
          class="text-xs"
          :disabled="loading"
          @click="emit('edit', row)"
        >
          Edit
        </Button>
        <Button
          variant="destructive"
          size="sm"
          class="text-xs"
          :disabled="loading"
          @click="emit('delete', row)"
        >
          Remove
        </Button>
      </div>
    </template>
  </DataTable>
</template>
