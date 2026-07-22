<script setup lang="ts">
import { ref, computed, onMounted } from "vue";
import { Button } from "~/components/ui/button";
import { Input } from "~/components/ui/input";
import { Card, CardContent, CardHeader, CardTitle } from "~/components/ui/card";
import { Badge } from "~/components/ui/badge";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "~/components/ui/select";
import PageHeader from "~/components/layout/PageHeader.vue";
import DataTable from "~/components/data-table/DataTable.vue";
import type { DataTableColumn } from "~/components/data-table/types";
import AuditLogDetailsDialog from "~/components/audit/AuditLogDetailsDialog.vue";
import {
  type AuditLog,
  getActionBadgeColor,
  formatActionType,
  getActionDescription,
  hasDetailedChanges,
} from "~/utils/audit";

definePageMeta({ middleware: ["auth"] });

const route = useRoute();
const serverId = route.params.serverId;

const loading = ref(true);
const error = ref<string | null>(null);
const auditLogs = ref<AuditLog[]>([]);
const searchQuery = ref("");
const actionTypeFilter = ref("all");
const userFilter = ref("all");
const dateFilter = ref("all");
const currentPage = ref(1);
const itemsPerPage = ref(20);
const totalItems = ref(0);
const totalPages = ref(0);
const selectedLog = ref<AuditLog | null>(null);
const showLogDetailsDialog = ref(false);

interface AuditLogsResponse {
  data: {
    logs: AuditLog[];
    pagination: {
      total: number;
      pages: number;
      page: number;
      limit: number;
    };
  };
}

// Action type options
const actionTypes = [
  { value: "all", label: "All Actions" },

  // Server Bans
  { value: "server:ban:create", label: "Server Ban Create" },
  { value: "server:ban:delete", label: "Server Ban Delete" },

  // Server Rcon
  { value: "server:rcon:execute", label: "Server RCON Execute" },
  { value: "server:rcon:command:kick", label: "Server RCON Command Kick" },
  { value: "server:rcon:command:warn", label: "Server RCON Command Warn" },
  { value: "server:rcon:command:move", label: "Server RCON Command Move" },

  // Server Roles
  { value: "server:role:create", label: "Server Role Create" },
  { value: "server:role:delete", label: "Server Role Delete" },
];

// Date filter options
const dateFilterOptions = [
  { value: "all", label: "All Time" },
  { value: "today", label: "Today" },
  { value: "yesterday", label: "Yesterday" },
  { value: "week", label: "This Week" },
  { value: "month", label: "This Month" },
];

// Unique users from audit logs
const uniqueUsers = computed(() => {
  const users = new Map();
  auditLogs.value.forEach((log) => {
    if (!users.has(log.user_id)) {
      users.set(log.user_id, {
        id: log.user_id,
        username: log.username,
      });
    }
  });
  return Array.from(users.values());
});

const columns: DataTableColumn<AuditLog>[] = [
  { key: "timestamp", label: "Time", class: "w-[180px]" },
  { key: "username", label: "User", class: "w-[140px]" },
  { key: "action", label: "Action", class: "w-[160px]" },
  { key: "description", label: "Details" },
];

async function fetchAuditLogs() {
  loading.value = true;
  error.value = null;

  const runtimeConfig = useRuntimeConfig();

  const queryParams = new URLSearchParams();
  queryParams.append("page", currentPage.value.toString());
  queryParams.append("limit", itemsPerPage.value.toString());

  if (searchQuery.value) {
    queryParams.append("search", searchQuery.value);
  }

  if (actionTypeFilter.value !== "all") {
    queryParams.append("actionType", actionTypeFilter.value);
  }

  if (userFilter.value !== "all") {
    queryParams.append("userId", userFilter.value);
  }

  if (dateFilter.value !== "all") {
    queryParams.append("dateFilter", dateFilter.value);
  }

  try {
    const data = await useAuthFetchImperative<AuditLogsResponse>(
      `${runtimeConfig.public.backendApi}/servers/${serverId}/audit-logs?${queryParams.toString()}`
    );

    auditLogs.value = data.data?.logs || [];
    totalItems.value = data.data?.pagination.total ?? 0;
    totalPages.value = data.data?.pagination.pages ?? 0;
  } catch (err: any) {
    error.value = extractApiErrorMessage(
      err,
      "An error occurred while fetching audit logs"
    );
  } finally {
    loading.value = false;
  }
}

function changePage(page: number) {
  currentPage.value = page;
  fetchAuditLogs();
}

function applyFilters() {
  currentPage.value = 1;
  fetchAuditLogs();
}

function resetFilters() {
  searchQuery.value = "";
  actionTypeFilter.value = "all";
  userFilter.value = "all";
  dateFilter.value = "all";
  applyFilters();
}

function viewLogDetails(log: AuditLog) {
  selectedLog.value = log;
  showLogDetailsDialog.value = true;
}

function formatDate(dateString: string): string {
  return new Date(dateString).toLocaleString();
}

onMounted(() => {
  fetchAuditLogs();
});
</script>

<template>
  <div class="space-y-4 p-4 md:p-6">
    <PageHeader
      title="Audit Logs"
      description="Who did what on this server, and when."
    >
      <template #actions>
        <Button
          @click="fetchAuditLogs"
          :disabled="loading"
          class="w-full sm:w-auto"
        >
          {{ loading ? "Refreshing..." : "Refresh" }}
        </Button>
      </template>
    </PageHeader>

    <div
      v-if="error"
      class="rounded-lg border border-destructive/20 bg-destructive/15 p-4 text-sm text-destructive"
    >
      {{ error }}
    </div>

    <!-- Filters Card -->
    <Card>
      <CardHeader class="pb-3 sm:pb-4">
        <CardTitle class="text-base sm:text-lg">Filters</CardTitle>
      </CardHeader>
      <CardContent>
        <div class="mb-4 grid grid-cols-1 gap-3 sm:gap-4 md:grid-cols-2 lg:grid-cols-4">
          <div class="space-y-2">
            <label class="text-xs font-medium sm:text-sm">Search</label>
            <Input
              v-model="searchQuery"
              placeholder="Search logs..."
              class="w-full"
              @keyup.enter="applyFilters"
            />
          </div>

          <div class="space-y-2">
            <label class="text-xs font-medium sm:text-sm">Action Type</label>
            <Select v-model="actionTypeFilter">
              <SelectTrigger>
                <SelectValue placeholder="Filter by action" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem
                  v-for="actionType in actionTypes"
                  :key="actionType.value"
                  :value="actionType.value"
                >
                  {{ actionType.label }}
                </SelectItem>
              </SelectContent>
            </Select>
          </div>

          <div class="space-y-2">
            <label class="text-xs font-medium sm:text-sm">User</label>
            <Select v-model="userFilter">
              <SelectTrigger>
                <SelectValue placeholder="Filter by user" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="all">All Users</SelectItem>
                <SelectItem
                  v-for="user in uniqueUsers"
                  :key="user.id"
                  :value="user.id"
                >
                  {{ user.username }}
                </SelectItem>
              </SelectContent>
            </Select>
          </div>

          <div class="space-y-2">
            <label class="text-xs font-medium sm:text-sm">Date Range</label>
            <Select v-model="dateFilter">
              <SelectTrigger>
                <SelectValue placeholder="Filter by date" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem
                  v-for="option in dateFilterOptions"
                  :key="option.value"
                  :value="option.value"
                >
                  {{ option.label }}
                </SelectItem>
              </SelectContent>
            </Select>
          </div>
        </div>

        <div class="flex flex-col justify-between gap-2 sm:flex-row">
          <Button variant="outline" @click="resetFilters" class="w-full sm:w-auto">
            Reset Filters
          </Button>
          <Button @click="applyFilters" class="w-full sm:w-auto">
            Apply Filters
          </Button>
        </div>
      </CardContent>
    </Card>

    <!-- Activity History -->
    <Card>
      <CardHeader class="pb-3 sm:pb-4">
        <CardTitle class="text-base sm:text-lg">Activity History</CardTitle>
        <p class="mt-1 text-xs text-muted-foreground sm:text-sm">
          {{ totalItems }} total logs found
        </p>
      </CardHeader>
      <CardContent>
        <DataTable
          :columns="columns"
          :rows="auditLogs"
          row-key="id"
          sort-mode="server"
          :loading="loading && auditLogs.length === 0"
          empty-text="No audit logs found. Try adjusting your filters."
        >
          <template #cell-timestamp="{ row }">
            <span class="font-mono text-xs sm:text-sm">
              {{ formatDate(row.timestamp) }}
            </span>
          </template>
          <template #cell-username="{ row }">
            {{ row.username }}
          </template>
          <template #cell-action="{ row }">
            <Badge
              variant="outline"
              :class="getActionBadgeColor(row.action)"
              class="text-xs font-medium"
              :title="row.action"
            >
              {{ formatActionType(row.action) }}
            </Badge>
          </template>
          <template #cell-description="{ row }">
            <span class="block max-w-md truncate text-xs text-muted-foreground sm:text-sm">
              {{ getActionDescription(row) || "—" }}
            </span>
          </template>
          <template #actions="{ row }">
            <Button
              variant="ghost"
              size="sm"
              :disabled="!hasDetailedChanges(row.changes)"
              class="gap-1 text-xs"
              @click="viewLogDetails(row)"
            >
              <Icon name="lucide:eye" class="h-4 w-4" />
              View
            </Button>
          </template>
        </DataTable>

        <!-- Server-side pagination -->
        <div
          class="flex flex-col items-center justify-between gap-3 pt-4 sm:flex-row"
        >
          <div class="text-center text-xs text-muted-foreground sm:text-left sm:text-sm">
            Showing {{ Math.min(auditLogs.length, itemsPerPage) }} of
            {{ totalItems }} entries
          </div>

          <div v-if="totalPages > 1" class="flex items-center gap-2">
            <Button
              variant="outline"
              size="sm"
              :disabled="currentPage === 1"
              @click="changePage(Math.max(1, currentPage - 1))"
            >
              <Icon name="lucide:chevron-left" class="h-4 w-4" />
              Previous
            </Button>

            <div class="flex items-center gap-1 px-2 text-xs sm:text-sm">
              <span class="font-medium">{{ currentPage }}</span>
              <span class="text-muted-foreground">/ {{ totalPages }}</span>
            </div>

            <Button
              variant="outline"
              size="sm"
              :disabled="currentPage === totalPages"
              @click="changePage(Math.min(totalPages, currentPage + 1))"
            >
              Next
              <Icon name="lucide:chevron-right" class="h-4 w-4" />
            </Button>
          </div>
        </div>
      </CardContent>
    </Card>

    <AuditLogDetailsDialog
      v-model:open="showLogDetailsDialog"
      :log="selectedLog"
    />
  </div>
</template>
