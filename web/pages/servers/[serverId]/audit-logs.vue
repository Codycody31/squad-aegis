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
  serverId: string;
  userId: string;
  username: string;
  actionType: string;
  actionDetails: string;
  ipAddress: string;
  createdAt: string;
  metadata?: Record<string, any>;
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
  auditLogs.value.forEach(log => {
    if (!users.has(log.userId)) {
      users.set(log.userId, {
        id: log.userId,
        username: log.username
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
  const cookieToken = useCookie(
    runtimeConfig.public.sessionCookieName as string
  );
  const token = cookieToken.value;

  if (!token) {
    error.value = "Authentication required";
    loading.value = false;
    return;
  }

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
    const { data, error: fetchError } = await useFetch<AuditLogsResponse>(
      `${runtimeConfig.public.backendApi}/servers/${serverId}/audit-logs?${queryParams.toString()}`,
      {
        method: "GET",
        headers: {
          Authorization: `Bearer ${token}`,
        },
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
    console.error(err);
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

// Get badge color based on action type
function getActionBadgeColor(actionType: string): string {
  const actionColors: Record<string, string> = {
    login: "bg-green-50 text-green-700 ring-green-600/20",
    logout: "bg-gray-50 text-gray-700 ring-gray-600/20",
    server_create: "bg-blue-50 text-blue-700 ring-blue-600/20",
    server_update: "bg-yellow-50 text-yellow-700 ring-yellow-600/20",
    server_delete: "bg-red-50 text-red-700 ring-red-600/20",
    rcon_command: "bg-purple-50 text-purple-700 ring-purple-600/20",
    ban_add: "bg-red-50 text-red-700 ring-red-600/20",
    ban_remove: "bg-green-50 text-green-700 ring-green-600/20",
    admin_add: "bg-blue-50 text-blue-700 ring-blue-600/20",
    admin_remove: "bg-red-50 text-red-700 ring-red-600/20",
    role_add: "bg-blue-50 text-blue-700 ring-blue-600/20",
    role_remove: "bg-red-50 text-red-700 ring-red-600/20",
  };

  return actionColors[actionType] || "bg-gray-50 text-gray-700 ring-gray-600/20";
}

// Format action type for display
function formatActionType(actionType: string): string {
  return actionType
    .split("_")
    .map(word => word.charAt(0).toUpperCase() + word.slice(1))
    .join(" ");
}

// Setup initial data load
onMounted(() => {
  fetchAuditLogs();
});
</script>

<template>
  <div class="p-4">
    <div class="flex justify-between items-center mb-4">
      <h1 class="text-2xl font-bold">Audit Logs</h1>
      <Button @click="fetchAuditLogs" :disabled="loading">
        {{ loading ? "Refreshing..." : "Refresh" }}
      </Button>
    </div>

    <div v-if="error" class="bg-red-500 text-white p-4 rounded mb-4">
      {{ error }}
    </div>

    <Card class="mb-4">
      <CardHeader class="pb-2">
        <CardTitle>Filters</CardTitle>
      </CardHeader>
      <CardContent>
        <div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4 mb-4">
          <div>
            <Input 
              v-model="searchQuery" 
              placeholder="Search logs..." 
              class="w-full"
              @keyup.enter="applyFilters"
            />
          </div>
          
          <div>
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
          
          <div>
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
          
          <div>
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
        
        <div class="flex justify-between">
          <Button variant="outline" @click="resetFilters">
            Reset Filters
          </Button>
          <Button @click="applyFilters">
            Apply Filters
          </Button>
        </div>
      </CardContent>
    </Card>

    <Card class="mb-4">
      <CardHeader class="pb-2">
        <CardTitle>Server Activity</CardTitle>
        <p class="text-sm text-muted-foreground">
          View a history of actions performed on this server.
        </p>
      </CardHeader>
      <CardContent>
        <div v-if="loading && auditLogs.length === 0" class="text-center py-8">
          <div class="animate-spin h-8 w-8 border-4 border-primary border-t-transparent rounded-full mx-auto mb-4"></div>
          <p>Loading audit logs...</p>
        </div>

        <div v-else-if="auditLogs.length === 0" class="text-center py-8">
          <p>No audit logs found</p>
        </div>

        <div v-else class="overflow-x-auto">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Time</TableHead>
                <TableHead>User</TableHead>
                <TableHead>Action</TableHead>
                <TableHead>Details</TableHead>
                <TableHead class="text-right">Actions</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              <TableRow 
                v-for="log in filteredAuditLogs" 
                :key="log.id"
                class="hover:bg-muted/50"
              >
                <TableCell>{{ formatDate(log.createdAt) }}</TableCell>
                <TableCell>{{ log.username }}</TableCell>
                <TableCell>
                  <Badge 
                    variant="outline" 
                    :class="getActionBadgeColor(log.actionType)"
                  >
                    {{ formatActionType(log.actionType) }}
                  </Badge>
                </TableCell>
                <TableCell>
                  <div class="max-w-xs truncate">{{ log.actionDetails }}</div>
                </TableCell>
                <TableCell class="text-right">
                  <Button 
                    variant="outline" 
                    size="sm"
                    @click="viewLogDetails(log)"
                  >
                    View
                  </Button>
                </TableCell>
              </TableRow>
            </TableBody>
          </Table>
          
          <div class="mt-4 flex justify-between items-center">
            <div class="text-sm text-muted-foreground">
              Showing {{ auditLogs.length }} of {{ totalItems }} logs
            </div>
            
            <div v-if="totalPages > 1" class="flex items-center space-x-2">
              <Button 
                variant="outline" 
                size="sm"
                @click="changePage(Math.max(1, currentPage - 1))"
                :disabled="currentPage === 1"
              >
                Previous
              </Button>
              
              <span class="text-sm">
                Page {{ currentPage }} of {{ totalPages }}
              </span>
              
              <Button 
                variant="outline" 
                size="sm"
                @click="changePage(Math.min(totalPages, currentPage + 1))"
                :disabled="currentPage === totalPages"
              >
                Next
              </Button>
            </div>
          </div>
        </div>
      </CardContent>
    </Card>

    <!-- Log Details Dialog -->
    <Dialog v-model:open="showLogDetailsDialog">
      <DialogContent class="sm:max-w-[600px]">
        <DialogHeader>
          <DialogTitle>Audit Log Details</DialogTitle>
          <DialogDescription>
            Detailed information about this action.
          </DialogDescription>
        </DialogHeader>
        
        <div v-if="selectedLog" class="py-4">
          <div class="grid grid-cols-2 gap-4 mb-4">
            <div>
              <h3 class="text-sm font-medium">Time</h3>
              <p>{{ formatDate(selectedLog.createdAt) }}</p>
            </div>
            <div>
              <h3 class="text-sm font-medium">User</h3>
              <p>{{ selectedLog.username }}</p>
            </div>
            <div>
              <h3 class="text-sm font-medium">Action</h3>
              <Badge 
                variant="outline" 
                :class="getActionBadgeColor(selectedLog.actionType)"
              >
                {{ formatActionType(selectedLog.actionType) }}
              </Badge>
            </div>
            <div>
              <h3 class="text-sm font-medium">IP Address</h3>
              <p>{{ selectedLog.ipAddress }}</p>
            </div>
          </div>
          
          <div class="mb-4">
            <h3 class="text-sm font-medium mb-1">Details</h3>
            <p class="text-sm">{{ selectedLog.actionDetails }}</p>
          </div>
          
          <div v-if="selectedLog.metadata">
            <h3 class="text-sm font-medium mb-1">Additional Data</h3>
            <pre class="bg-muted p-2 rounded text-xs overflow-auto max-h-[200px]">{{ JSON.stringify(selectedLog.metadata, null, 2) }}</pre>
          </div>
        </div>
      </DialogContent>
    </Dialog>

    <Card>
      <CardHeader>
        <CardTitle>About Audit Logs</CardTitle>
      </CardHeader>
      <CardContent>
        <p class="text-sm text-muted-foreground">
          Audit logs provide a detailed history of all actions performed on your server. This includes administrative actions, RCON commands, user management, and more.
        </p>
        <p class="text-sm text-muted-foreground mt-2">
          Use the filters above to narrow down the logs by action type, user, or date range. Click on "View" to see detailed information about each action.
        </p>
        <p class="text-sm text-muted-foreground mt-2">
          Audit logs are an important tool for server security and accountability, allowing you to track who did what and when.
        </p>
      </CardContent>
    </Card>
  </div>
</template>

<style scoped>
/* Add any page-specific styles here */
</style>
