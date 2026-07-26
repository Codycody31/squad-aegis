<script setup lang="ts">
import { Badge } from "~/components/ui/badge";
import { Button } from "~/components/ui/button";
import DataTable from "~/components/data-table/DataTable.vue";
import type { DataTableColumn } from "~/components/data-table/types";
import {
  formatDate,
  type ServerAdmin,
  type ServerRole,
  type User,
} from "./shared";

const props = defineProps<{
  admins: ServerAdmin[];
  loading?: boolean;
  users: User[];
  roles: ServerRole[];
  emptyText?: string;
}>();

const emit = defineEmits<{
  (e: "edit", admin: ServerAdmin): void;
  (e: "remove", admin: ServerAdmin): void;
}>();

// Get display name for an admin
const getAdminDisplayName = (admin: ServerAdmin): string => {
  if (admin.steam_id) {
    return `Steam ID: ${admin.steam_id}`;
  } else if (admin.eos_id) {
    return `EOS ID: ${admin.eos_id}`;
  } else if (admin.user_id) {
    const user = props.users.find((u) => u.id === admin.user_id);
    return user ? `${user.name} (${user.username})` : "Unknown User";
  }
  return "Unknown";
};

// Get role name by ID
const getRoleName = (roleId: string): string => {
  const role = props.roles.find((r) => r.id === roleId);
  return role ? role.name : "Unknown Role";
};

// Format expiration status
function formatExpirationStatus(admin: ServerAdmin): {
  text: string;
  variant: "default" | "destructive" | "secondary" | "outline";
  class: string;
} {
  if (!admin.expires_at) {
    return { text: "Permanent", variant: "default", class: "" };
  }

  const expiresAt = new Date(admin.expires_at);
  const now = new Date();

  if (expiresAt < now) {
    return { text: "Expired", variant: "destructive", class: "" };
  }

  const timeUntilExpiry = expiresAt.getTime() - now.getTime();
  const daysUntilExpiry = Math.floor(timeUntilExpiry / (1000 * 60 * 60 * 24));

  if (daysUntilExpiry <= 7) {
    return {
      text: `Expires in ${daysUntilExpiry} day(s)`,
      variant: "secondary",
      class: "text-orange-600",
    };
  }

  return {
    text: `Expires ${formatDate(admin.expires_at)}`,
    variant: "outline",
    class: "text-blue-600",
  };
}

const columns: DataTableColumn<ServerAdmin>[] = [
  {
    key: "admin",
    label: "Admin",
    accessor: (row) => getAdminDisplayName(row),
  },
  { key: "role", label: "Role" },
  { key: "status", label: "Status" },
  { key: "notes", label: "Notes" },
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
    :rows="admins"
    row-key="id"
    :loading="loading"
    :empty-text="emptyText ?? 'No admins found. Add an admin to get started.'"
  >
    <template #cell-admin="{ row }">
      <span class="font-medium">{{ getAdminDisplayName(row) }}</span>
    </template>

    <template #cell-role="{ row }">
      <Badge variant="outline" class="text-xs">
        {{ getRoleName(row.server_role_id) }}
      </Badge>
    </template>

    <template #cell-status="{ row }">
      <Badge
        :variant="formatExpirationStatus(row).variant"
        :class="formatExpirationStatus(row).class"
        class="text-xs"
      >
        {{ formatExpirationStatus(row).text }}
      </Badge>
    </template>

    <template #cell-notes="{ row }">
      <div class="max-w-[200px] truncate">{{ row.notes || "-" }}</div>
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
          @click="emit('remove', row)"
        >
          Remove
        </Button>
      </div>
    </template>
  </DataTable>
</template>
