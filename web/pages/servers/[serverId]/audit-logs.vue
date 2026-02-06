<script setup lang="ts">
import { ref, onMounted, computed } from "vue";
import { Button } from "~/components/ui/button";
import { Input } from "~/components/ui/input";
import { Card, CardContent, CardHeader, CardTitle } from "~/components/ui/card";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "~/components/ui/table";
import { Badge } from "~/components/ui/badge";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "~/components/ui/select";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "~/components/ui/dialog";
import { Separator } from "~/components/ui/separator";

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

// Interfaces
interface AuditLog {
  id: string;
  server_id: string;
  server_name: string;
  user_id: string;
  username: string;
  action: string;
  changes: any;
  timestamp: string;
}

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

// Computed property for filtered audit logs
const filteredAuditLogs = computed(() => {
  return auditLogs.value;
});

// Function to fetch audit logs
async function fetchAuditLogs() {
  loading.value = true;
  error.value = null;

  const runtimeConfig = useRuntimeConfig();

  // Build query parameters
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
    const { data, error: fetchError } = await useAuthFetch<AuditLogsResponse>(
      `${
        runtimeConfig.public.backendApi
      }/servers/${serverId}/audit-logs?${queryParams.toString()}`,
      {
        method: "GET",
      }
    );

    if (fetchError.value) {
      throw new Error(fetchError.value.message || "Failed to fetch audit logs");
    }

    if (data.value && data.value.data) {
      auditLogs.value = data.value.data.logs || [];
      totalItems.value = data.value.data.pagination.total;
      totalPages.value = data.value.data.pagination.pages;
    }
  } catch (err: any) {
    error.value = err.message || "An error occurred while fetching audit logs";
  } finally {
    loading.value = false;
  }
}

// Function to handle page change
function changePage(page: number) {
  currentPage.value = page;
  fetchAuditLogs();
}

// Function to apply filters
function applyFilters() {
  currentPage.value = 1; // Reset to first page when filters change
  fetchAuditLogs();
}

// Function to reset filters
function resetFilters() {
  searchQuery.value = "";
  actionTypeFilter.value = "all";
  userFilter.value = "all";
  dateFilter.value = "all";
  applyFilters();
}

// Function to view log details
function viewLogDetails(log: AuditLog) {
  selectedLog.value = log;
  showLogDetailsDialog.value = true;
}

// Format date
function formatDate(dateString: string): string {
  return new Date(dateString).toLocaleString();
}

// Format date relative
function formatDateRelative(dateString: string): string {
  const date = new Date(dateString);
  const now = new Date();
  const diffInHours =
    Math.abs(now.getTime() - date.getTime()) / (1000 * 60 * 60);

  if (diffInHours < 1) {
    const diffInMinutes = Math.floor(diffInHours * 60);
    return `${diffInMinutes}m ago`;
  } else if (diffInHours < 24) {
    return `${Math.floor(diffInHours)}h ago`;
  } else if (diffInHours < 168) {
    const diffInDays = Math.floor(diffInHours / 24);
    return `${diffInDays}d ago`;
  } else {
    return date.toLocaleDateString();
  }
}

// Get badge color based on action type
function getActionBadgeColor(actionType: string): string {
  const actionColors: Record<string, string> = {
    "server:ban:create":
      "bg-red-50 text-red-700 ring-red-600/20 dark:bg-red-900/20 dark:text-red-400",
    "server:ban:delete":
      "bg-green-50 text-green-700 ring-green-600/20 dark:bg-green-900/20 dark:text-green-400",
    "server:rcon:execute":
      "bg-purple-50 text-purple-700 ring-purple-600/20 dark:bg-purple-900/20 dark:text-purple-400",
    "server:rcon:command:kick":
      "bg-orange-50 text-orange-700 ring-orange-600/20 dark:bg-orange-900/20 dark:text-orange-400",
    "server:rcon:command:warn":
      "bg-yellow-50 text-yellow-700 ring-yellow-600/20 dark:bg-yellow-900/20 dark:text-yellow-400",
    "server:rcon:command:move":
      "bg-blue-50 text-blue-700 ring-blue-600/20 dark:bg-blue-900/20 dark:text-blue-400",
    "server:role:create":
      "bg-blue-50 text-blue-700 ring-blue-600/20 dark:bg-blue-900/20 dark:text-blue-400",
    "server:role:delete":
      "bg-red-50 text-red-700 ring-red-600/20 dark:bg-red-900/20 dark:text-red-400",
  };

  return (
    actionColors[actionType] ||
    "bg-gray-50 text-gray-700 ring-gray-600/20 dark:bg-gray-900/20 dark:text-gray-400"
  );
}

// Format action type for display
function formatActionType(actionType: string): string {
  const actionTypeMap: Record<string, string> = {
    "server:ban:create": "Ban Created",
    "server:ban:delete": "Ban Removed",
    "server:rcon:execute": "RCON Command",
    "server:rcon:command:kick": "Player Kicked",
    "server:rcon:command:warn": "Player Warned",
    "server:rcon:command:move": "Player Moved",
    "server:role:create": "Role Created",
    "server:role:delete": "Role Deleted",
  };

  return (
    actionTypeMap[actionType] ||
    actionType
      .split(":")
      .pop()
      ?.split("_")
      .map((word) => word.charAt(0).toUpperCase() + word.slice(1))
      .join(" ") ||
    actionType
  );
}

// Get action description
function getActionDescription(log: AuditLog): string {
  if (!log.changes) return "";

  if (typeof log.changes === "string") {
    try {
      const parsed = JSON.parse(log.changes);
      if (parsed.description) return parsed.description;
      if (parsed.target) return `Target: ${parsed.target}`;
      if (parsed.command) return `Command: ${parsed.command}`;
      return parsed.toString();
    } catch (e) {
      return log.changes;
    }
  }

  if (typeof log.changes === "object") {
    if (log.changes.description) return log.changes.description;
    if (log.changes.target) return `Target: ${log.changes.target}`;
    if (log.changes.command) return `Command: ${log.changes.command}`;
    if (log.changes.reason) return `Reason: ${log.changes.reason}`;
    return "";
  }

  return String(log.changes);
}

// Format JSON for display
function formatJsonForDisplay(data: any): string {
  if (!data) return "No additional data";

  try {
    if (typeof data === "string") {
      // Try to parse if it's a JSON string
      const parsed = JSON.parse(data);
      return JSON.stringify(parsed, null, 2);
    }
    return JSON.stringify(data, null, 2);
  } catch (e) {
    return typeof data === "string" ? data : String(data);
  }
}

// Check if changes contain useful data
function hasDetailedChanges(changes: any): boolean {
  if (!changes) return false;

  if (typeof changes === "string") {
    try {
      const parsed = JSON.parse(changes);
      return Object.keys(parsed).length > 0;
    } catch (e) {
      return changes.length > 0;
    }
  }

  if (typeof changes === "object") {
    return Object.keys(changes).length > 0;
  }

  return false;
}

// Copy text to clipboard
function copyToClipboard(text: string) {
  if (typeof window !== "undefined" && window.navigator?.clipboard) {
    window.navigator.clipboard.writeText(text);
  }
}

// Setup initial data load
onMounted(() => {
  fetchAuditLogs();
});
</script>

<template>
  <div class="p-3 sm:p-6 space-y-4 sm:space-y-6">
    <div class="flex flex-col sm:flex-row sm:justify-between sm:items-center gap-3 sm:gap-0">
      <div>
        <h1 class="text-2xl sm:text-3xl font-bold tracking-tight">Audit Logs</h1>
        <p class="text-sm sm:text-base text-muted-foreground">
          Track all administrative actions and changes
        </p>
      </div>
      <Button @click="fetchAuditLogs" :disabled="loading" class="gap-2 w-full sm:w-auto">
        <svg
          v-if="loading"
          class="animate-spin h-4 w-4"
          xmlns="http://www.w3.org/2000/svg"
          fill="none"
          viewBox="0 0 24 24"
        >
          <circle
            class="opacity-25"
            cx="12"
            cy="12"
            r="10"
            stroke="currentColor"
            stroke-width="4"
          ></circle>
          <path
            class="opacity-75"
            fill="currentColor"
            d="m4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
          ></path>
        </svg>
        <span class="hidden sm:inline">{{ loading ? "Refreshing..." : "Refresh" }}</span>
        <span class="sm:hidden">{{ loading ? "Refreshing..." : "Refresh" }}</span>
      </Button>
    </div>

    <div
      v-if="error"
      class="bg-destructive/15 text-destructive border border-destructive/20 p-3 sm:p-4 rounded-lg text-sm sm:text-base"
    >
      {{ error }}
    </div>

    <!-- Filters Card -->
    <Card>
      <CardHeader class="pb-3 sm:pb-4">
        <CardTitle class="text-base sm:text-lg">Filters</CardTitle>
      </CardHeader>
      <CardContent>
        <div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-3 sm:gap-4 mb-4 sm:mb-6">
          <div class="space-y-2">
            <label class="text-xs sm:text-sm font-medium">Search</label>
            <Input
              v-model="searchQuery"
              placeholder="Search logs..."
              class="w-full text-sm sm:text-base"
              @keyup.enter="applyFilters"
            />
          </div>

          <div class="space-y-2">
            <label class="text-xs sm:text-sm font-medium">Action Type</label>
            <Select v-model="actionTypeFilter">
              <SelectTrigger class="text-sm sm:text-base">
                <SelectValue placeholder="Filter by action" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem
                  v-for="actionType in actionTypes"
                  :key="actionType.value"
                  :value="actionType.value"
                  class="text-sm sm:text-base"
                >
                  {{ actionType.label }}
                </SelectItem>
              </SelectContent>
            </Select>
          </div>

          <div class="space-y-2">
            <label class="text-xs sm:text-sm font-medium">User</label>
            <Select v-model="userFilter">
              <SelectTrigger class="text-sm sm:text-base">
                <SelectValue placeholder="Filter by user" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="all" class="text-sm sm:text-base">All Users</SelectItem>
                <SelectItem
                  v-for="user in uniqueUsers"
                  :key="user.id"
                  :value="user.id"
                  class="text-sm sm:text-base"
                >
                  {{ user.username }}
                </SelectItem>
              </SelectContent>
            </Select>
          </div>

          <div class="space-y-2">
            <label class="text-xs sm:text-sm font-medium">Date Range</label>
            <Select v-model="dateFilter">
              <SelectTrigger class="text-sm sm:text-base">
                <SelectValue placeholder="Filter by date" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem
                  v-for="option in dateFilterOptions"
                  :key="option.value"
                  :value="option.value"
                  class="text-sm sm:text-base"
                >
                  {{ option.label }}
                </SelectItem>
              </SelectContent>
            </Select>
          </div>
        </div>

        <div class="flex flex-col sm:flex-row justify-between gap-2 sm:gap-0">
          <Button variant="outline" @click="resetFilters" class="w-full sm:w-auto text-sm sm:text-base">
            Reset Filters
          </Button>
          <Button @click="applyFilters" class="w-full sm:w-auto text-sm sm:text-base"> Apply Filters </Button>
        </div>
      </CardContent>
    </Card>

    <!-- Main Content Card -->
    <Card>
      <CardHeader class="pb-3 sm:pb-4">
        <div class="flex justify-between items-center">
          <div>
            <CardTitle class="text-base sm:text-lg">Activity History</CardTitle>
            <p class="text-xs sm:text-sm text-muted-foreground mt-1">
              {{ totalItems }} total logs found
            </p>
          </div>
        </div>
      </CardHeader>
      <CardContent>
        <div v-if="loading && auditLogs.length === 0" class="text-center py-8 sm:py-12">
          <div
            class="animate-spin h-8 w-8 border-4 border-primary border-t-transparent rounded-full mx-auto mb-4"
          ></div>
          <p class="text-sm sm:text-base text-muted-foreground">Loading audit logs...</p>
        </div>

        <div v-else-if="auditLogs.length === 0" class="text-center py-8 sm:py-12">
          <div class="text-muted-foreground mb-2">
            <svg
              class="h-10 w-10 sm:h-12 sm:w-12 mx-auto mb-4"
              fill="none"
              stroke="currentColor"
              viewBox="0 0 24 24"
            >
              <path
                stroke-linecap="round"
                stroke-linejoin="round"
                stroke-width="1.5"
                d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z"
              />
            </svg>
          </div>
          <p class="text-base sm:text-lg font-medium">No audit logs found</p>
          <p class="text-sm sm:text-base text-muted-foreground">
            Try adjusting your filters or refresh the page.
          </p>
        </div>

        <div v-else class="space-y-3 sm:space-y-4">
          <!-- Desktop Table View -->
          <div class="hidden md:block rounded-md border overflow-x-auto">
            <Table>
              <TableHeader>
                <TableRow class="bg-muted/50">
                  <TableHead class="w-[180px] text-xs sm:text-sm">Time</TableHead>
                  <TableHead class="w-[140px] text-xs sm:text-sm">User</TableHead>
                  <TableHead class="w-[160px] text-xs sm:text-sm">Action</TableHead>
                  <TableHead class="w-[160px] text-xs sm:text-sm">Type</TableHead>
                  <TableHead class="w-[100px] text-xs sm:text-sm">Details</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                <TableRow
                  v-for="log in filteredAuditLogs"
                  :key="log.id"
                  class="hover:bg-muted/30 transition-colors"
                >
                  <TableCell class="font-mono text-xs sm:text-sm">
                    {{ formatDate(log.timestamp) }}
                  </TableCell>
                  <TableCell>
                    <Badge
                      variant="outline"
                      :class="getActionBadgeColor(log.action)"
                      class="font-medium text-xs"
                    >
                      {{ log.username }}
                    </Badge>
                  </TableCell>
                  <TableCell>
                    <Badge
                      variant="outline"
                      :class="getActionBadgeColor(log.action)"
                      class="font-medium text-xs"
                    >
                      {{ log.action }}
                    </Badge>
                  </TableCell>
                  <TableCell>
                    <Badge
                      variant="outline"
                      :class="getActionBadgeColor(log.action)"
                      class="font-medium text-xs"
                    >
                      {{ formatActionType(log.action) }}
                    </Badge>
                  </TableCell>
                  <TableCell class="text-right">
                    <Button
                      variant="ghost"
                      size="sm"
                      @click="viewLogDetails(log)"
                      :disabled="!hasDetailedChanges(log.changes)"
                      class="gap-1 text-xs"
                    >
                      <svg
                        class="h-3 w-3 sm:h-4 sm:w-4"
                        fill="none"
                        stroke="currentColor"
                        viewBox="0 0 24 24"
                      >
                        <path
                          stroke-linecap="round"
                          stroke-linejoin="round"
                          stroke-width="2"
                          d="M15 12a3 3 0 11-6 0 3 3 0 016 0z"
                        />
                        <path
                          stroke-linecap="round"
                          stroke-linejoin="round"
                          stroke-width="2"
                          d="M2.458 12C3.732 7.943 7.523 5 12 5c4.478 0 8.268 2.943 9.542 7-1.274 4.057-5.064 7-9.542 7-4.477 0-8.268-2.943-9.542-7z"
                        />
                      </svg>
                      View
                    </Button>
                  </TableCell>
                </TableRow>
              </TableBody>
            </Table>
          </div>

          <!-- Mobile Card View -->
          <div class="md:hidden space-y-3">
            <div
              v-for="log in filteredAuditLogs"
              :key="log.id"
              class="border rounded-lg p-3 sm:p-4 hover:bg-muted/30 transition-colors"
            >
              <div class="flex items-start justify-between gap-2 mb-2">
                <div class="flex-1 min-w-0">
                  <p class="font-mono text-xs text-muted-foreground mb-1">
                    {{ formatDate(log.timestamp) }}
                  </p>
                  <Badge
                    variant="outline"
                    :class="getActionBadgeColor(log.action)"
                    class="font-medium text-xs mb-1"
                  >
                    {{ formatActionType(log.action) }}
                  </Badge>
                </div>
                <Button
                  variant="ghost"
                  size="sm"
                  @click="viewLogDetails(log)"
                  :disabled="!hasDetailedChanges(log.changes)"
                  class="h-8 w-8 p-0 flex-shrink-0"
                >
                  <svg
                    class="h-4 w-4"
                    fill="none"
                    stroke="currentColor"
                    viewBox="0 0 24 24"
                  >
                    <path
                      stroke-linecap="round"
                      stroke-linejoin="round"
                      stroke-width="2"
                      d="M15 12a3 3 0 11-6 0 3 3 0 016 0z"
                    />
                    <path
                      stroke-linecap="round"
                      stroke-linejoin="round"
                      stroke-width="2"
                      d="M2.458 12C3.732 7.943 7.523 5 12 5c4.478 0 8.268 2.943 9.542 7-1.274 4.057-5.064 7-9.542 7-4.477 0-8.268-2.943-9.542-7z"
                    />
                  </svg>
                </Button>
              </div>
              <div class="space-y-1.5">
                <div class="flex items-center gap-2">
                  <span class="text-xs text-muted-foreground">User:</span>
                  <Badge
                    variant="outline"
                    :class="getActionBadgeColor(log.action)"
                    class="font-medium text-xs"
                  >
                    {{ log.username }}
                  </Badge>
                </div>
                <div class="flex items-center gap-2">
                  <span class="text-xs text-muted-foreground">Action:</span>
                  <span class="text-xs font-medium break-all">{{ log.action }}</span>
                </div>
              </div>
            </div>
          </div>

          <!-- Pagination -->
          <div class="flex flex-col sm:flex-row justify-between items-center gap-3 sm:gap-0 pt-3 sm:pt-4">
            <div class="text-xs sm:text-sm text-muted-foreground text-center sm:text-left">
              Showing {{ Math.min(auditLogs.length, itemsPerPage) }} of
              {{ totalItems }} entries
            </div>

            <div v-if="totalPages > 1" class="flex items-center gap-2">
              <Button
                variant="outline"
                size="sm"
                @click="changePage(Math.max(1, currentPage - 1))"
                :disabled="currentPage === 1"
                class="text-xs sm:text-sm"
              >
                <svg
                  class="h-3 w-3 sm:h-4 sm:w-4 sm:mr-1"
                  fill="none"
                  stroke="currentColor"
                  viewBox="0 0 24 24"
                >
                  <path
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    stroke-width="2"
                    d="M15 19l-7-7 7-7"
                  />
                </svg>
                <span class="hidden sm:inline">Previous</span>
                <span class="sm:hidden">Prev</span>
              </Button>

              <div class="flex items-center gap-1 px-2">
                <span class="text-xs sm:text-sm font-medium">{{ currentPage }}</span>
                <span class="text-xs sm:text-sm text-muted-foreground">/ {{ totalPages }}</span>
              </div>

              <Button
                variant="outline"
                size="sm"
                @click="changePage(Math.min(totalPages, currentPage + 1))"
                :disabled="currentPage === totalPages"
                class="text-xs sm:text-sm"
              >
                <span class="hidden sm:inline">Next</span>
                <span class="sm:hidden">Next</span>
                <svg
                  class="h-3 w-3 sm:h-4 sm:w-4 sm:ml-1"
                  fill="none"
                  stroke="currentColor"
                  viewBox="0 0 24 24"
                >
                  <path
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    stroke-width="2"
                    d="M9 5l7 7-7 7"
                  />
                </svg>
              </Button>
            </div>
          </div>
        </div>
      </CardContent>
    </Card>

    <!-- Enhanced Log Details Dialog -->
    <Dialog v-model:open="showLogDetailsDialog">
      <DialogContent class="w-[95vw] sm:max-w-[700px] max-h-[85vh] sm:max-h-[80vh] p-4 sm:p-6">
        <DialogHeader>
          <DialogTitle class="flex items-center gap-2 text-base sm:text-lg">
            <svg
              class="h-4 w-4 sm:h-5 sm:w-5"
              fill="none"
              stroke="currentColor"
              viewBox="0 0 24 24"
            >
              <path
                stroke-linecap="round"
                stroke-linejoin="round"
                stroke-width="2"
                d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z"
              />
            </svg>
            Audit Log Details
          </DialogTitle>
          <DialogDescription class="text-xs sm:text-sm">
            Complete information about this administrative action.
          </DialogDescription>
        </DialogHeader>

        <div class="max-h-[60vh] sm:max-h-[60vh] overflow-y-auto" v-if="selectedLog">
          <div class="space-y-4 sm:space-y-6 pr-2 sm:pr-4">
            <!-- Header Info -->
            <div class="grid grid-cols-1 sm:grid-cols-2 gap-4 sm:gap-6">
              <div class="space-y-3">
                <div>
                  <h3 class="text-xs sm:text-sm font-medium text-muted-foreground">
                    Timestamp
                  </h3>
                  <p class="text-xs sm:text-sm font-mono break-all">
                    {{ formatDate(selectedLog.timestamp) }}
                  </p>
                </div>
                <div>
                  <h3 class="text-xs sm:text-sm font-medium text-muted-foreground">
                    User
                  </h3>
                  <div class="flex items-center gap-2 mt-1">
                    <div
                      class="h-6 w-6 rounded-full bg-primary/10 flex items-center justify-center flex-shrink-0"
                    >
                      <span class="text-xs font-medium text-primary">{{
                        selectedLog.username.charAt(0).toUpperCase()
                      }}</span>
                    </div>
                    <span class="font-medium text-sm sm:text-base break-all">{{ selectedLog.username }}</span>
                  </div>
                </div>
              </div>

              <div class="space-y-3">
                <div>
                  <h3 class="text-xs sm:text-sm font-medium text-muted-foreground">
                    Action Type
                  </h3>
                  <Badge
                    variant="outline"
                    :class="getActionBadgeColor(selectedLog.action)"
                    class="font-medium mt-1 text-xs"
                  >
                    {{ formatActionType(selectedLog.action) }}
                  </Badge>
                </div>
                <div>
                  <h3 class="text-xs sm:text-sm font-medium text-muted-foreground">
                    Action
                  </h3>
                  <Badge
                    variant="outline"
                    :class="getActionBadgeColor(selectedLog.action)"
                    class="font-medium mt-1 text-xs break-all"
                  >
                    {{ selectedLog.action }}
                  </Badge>
                </div>
                <div>
                  <h3 class="text-xs sm:text-sm font-medium text-muted-foreground">
                    Log ID
                  </h3>
                  <p class="text-xs font-mono text-muted-foreground break-all">
                    {{ selectedLog.id }}
                  </p>
                </div>
              </div>
            </div>

            <Separator />

            <!-- Raw Data -->
            <div v-if="hasDetailedChanges(selectedLog.changes)">
              <h3 class="text-xs sm:text-sm font-medium text-muted-foreground mb-2">
                Raw Data
              </h3>
              <div class="bg-muted/50 rounded-md border">
                <div class="p-2 sm:p-3 border-b bg-muted/30">
                  <div class="flex items-center justify-between">
                    <span class="text-xs font-medium text-muted-foreground"
                      >JSON</span
                    >
                    <Button
                      variant="ghost"
                      size="sm"
                      class="h-6 px-2 text-xs"
                      @click="
                        copyToClipboard(
                          formatJsonForDisplay(selectedLog.changes)
                        )
                      "
                    >
                      Copy
                    </Button>
                  </div>
                </div>
                <div class="max-h-[250px] sm:max-h-[300px] overflow-y-auto">
                  <pre
                    class="p-2 sm:p-3 text-xs font-mono whitespace-pre-wrap break-words"
                    >{{ formatJsonForDisplay(selectedLog.changes) }}</pre
                  >
                </div>
              </div>
            </div>

            <div v-else>
              <div class="text-center py-6 sm:py-8 text-muted-foreground">
                <svg
                  class="h-6 w-6 sm:h-8 sm:w-8 mx-auto mb-2"
                  fill="none"
                  stroke="currentColor"
                  viewBox="0 0 24 24"
                >
                  <path
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    stroke-width="2"
                    d="M20 13V6a2 2 0 00-2-2H6a2 2 0 00-2 2v7m16 0v5a2 2 0 01-2 2H6a2 2 0 01-2-2v-5m16 0h-2.586a1 1 0 00-.707.293l-2.414 2.414a1 1 0 01-.707.293h-3.172a1 1 0 01-.707-.293l-2.414-2.414A1 1 0 006.586 13H4"
                  />
                </svg>
                <p class="text-xs sm:text-sm">
                  No additional data available for this action
                </p>
              </div>
            </div>
          </div>
        </div>
      </DialogContent>
    </Dialog>
  </div>
</template>

<style scoped>
/* Enhanced styles for better presentation */
</style>
