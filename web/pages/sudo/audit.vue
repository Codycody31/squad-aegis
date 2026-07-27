<script setup lang="ts">
import { ref, onMounted, onUnmounted, watch } from "vue";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "~/components/ui/card";
import { Input } from "~/components/ui/input";
import { Button } from "~/components/ui/button";
import PageHeader from "~/components/layout/PageHeader.vue";
import DataTable from "~/components/data-table/DataTable.vue";
import type { DataTableColumn } from "~/components/data-table/types";
import type { GlobalAuditLog, PageInfo } from "~/types";

definePageMeta({ middleware: ["auth", "sudo"], layout: "sudo" });

const runtimeConfig = useRuntimeConfig();

const loading = ref(true);
const logs = ref<GlobalAuditLog[]>([]);
const searchQuery = ref("");
const actionFilter = ref("all");
const {
  limit,
  cursor,
  page: currentPage,
  canGoPrevious,
  canGoNext,
  setPageInfo,
  reset: resetPagination,
  next: advancePage,
  previous: returnToPreviousPage,
} = useCursorPagination(50);
let searchTimer: ReturnType<typeof setTimeout> | null = null;

const columns: DataTableColumn<GlobalAuditLog>[] = [
  {
    key: "timestamp",
    label: "Timestamp",
    accessor: (log) => new Date(log.timestamp).toLocaleString(),
  },
  { key: "server_name", label: "Server", accessor: (log) => log.server_name || "N/A" },
  { key: "username", label: "User", accessor: (log) => log.username || "System" },
  { key: "action", label: "Action" },
];

const fetchLogs = async () => {
  loading.value = true;
  try {
    const res = await useAuthFetchImperative<{
      data: {
        logs: GlobalAuditLog[];
        page_info: PageInfo;
      };
    }>(
      `${runtimeConfig.public.backendApi}/sudo/audit/logs`,
      {
        params: {
          limit: limit.value,
          cursor: cursor.value || undefined,
          search: searchQuery.value,
          action:
            actionFilter.value === "all" ? undefined : actionFilter.value,
        },
      }
    );
    logs.value = res.data.logs || [];
    setPageInfo(res.data.page_info);
  } catch (err: any) {
    console.error("Error fetching audit logs:", err);
  } finally {
    loading.value = false;
  }
};

function nextPage() {
  if (advancePage()) {
    fetchLogs();
  }
}

function previousPage() {
  if (returnToPreviousPage()) {
    fetchLogs();
  }
}

watch(searchQuery, () => {
  resetPagination();
  if (searchTimer) {
    clearTimeout(searchTimer);
  }
  searchTimer = setTimeout(fetchLogs, 350);
});

watch(actionFilter, () => {
  resetPagination();
  fetchLogs();
});

onMounted(fetchLogs);
onUnmounted(() => {
  if (searchTimer) {
    clearTimeout(searchTimer);
  }
});
</script>

<template>
  <div class="space-y-6 p-4 md:p-6">
    <PageHeader
      title="Global Audit Logs"
      description="Admin actions across every server on this instance."
    />

    <Card>
      <CardHeader>
        <div class="flex items-center justify-between gap-2">
          <div>
            <CardTitle>Audit Logs</CardTitle>
            <CardDescription>
              View all admin actions across all servers
            </CardDescription>
          </div>
          <div class="flex items-center gap-2">
            <Input
              v-model="searchQuery"
              placeholder="Search..."
              class="w-64"
            />
            <Button @click="fetchLogs" size="sm">
              <Icon name="mdi:refresh" />
            </Button>
          </div>
        </div>
      </CardHeader>
      <CardContent>
        <DataTable
          :columns="columns"
          :rows="logs"
          row-key="id"
          sort-mode="server"
          :loading="loading && logs.length === 0"
          empty-text="No audit logs found."
        />

        <div
          v-if="canGoPrevious || canGoNext"
          class="mt-4 flex items-center justify-between"
        >
          <div class="text-sm text-muted-foreground">
            Page {{ currentPage }} · {{ logs.length }} entries
          </div>
          <div class="flex gap-2">
            <Button
              @click="previousPage"
              :disabled="!canGoPrevious || loading"
              size="sm"
            >
              Previous
            </Button>
            <Button
              @click="nextPage"
              :disabled="!canGoNext || loading"
              size="sm"
            >
              Next
            </Button>
          </div>
        </div>
      </CardContent>
    </Card>
  </div>
</template>
