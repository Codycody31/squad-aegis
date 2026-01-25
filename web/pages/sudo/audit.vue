<script setup lang="ts">
import { ref, onMounted } from "vue";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "~/components/ui/card";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "~/components/ui/table";
import { Input } from "~/components/ui/input";
import { Button } from "~/components/ui/button";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "~/components/ui/select";
import type { GlobalAuditLog } from "~/types";

definePageMeta({ middleware: "auth", layout: "sudo" });

const runtimeConfig = useRuntimeConfig();
const authStore = useAuthStore();

if (!authStore.user?.super_admin) navigateTo("/dashboard");

const loading = ref(true);
const logs = ref<GlobalAuditLog[]>([]);
const searchQuery = ref("");
const actionFilter = ref("all");
const currentPage = ref(1);
const totalPages = ref(1);

const fetchLogs = async () => {
  loading.value = true;
  try {
    const res = await useAuthFetchImperative<any>(`${runtimeConfig.public.backendApi}/sudo/audit/logs`, {
      params: {
        page: currentPage.value,
        limit: 50,
        search: searchQuery.value,
        action: actionFilter.value,
      },
    });
    logs.value = res.data.logs;
    totalPages.value = res.data.pagination.total_pages;
  } catch (err: any) {
    console.error("Error fetching audit logs:", err);
  } finally {
    loading.value = false;
  }
};

onMounted(fetchLogs);
</script>

<template>
  <div class="p-6 space-y-6">
    <h1 class="text-3xl font-bold">Global Audit Logs</h1>

    <Card>
      <CardHeader>
        <div class="flex items-center justify-between">
          <div>
            <CardTitle>Audit Logs</CardTitle>
            <CardDescription>View all admin actions across all servers</CardDescription>
          </div>
          <div class="flex items-center gap-2">
            <Input v-model="searchQuery" placeholder="Search..." class="w-64" @input="fetchLogs" />
            <Button @click="fetchLogs" size="sm"><Icon name="mdi:refresh" /></Button>
          </div>
        </div>
      </CardHeader>
      <CardContent>
        <div v-if="loading" class="flex items-center justify-center py-12">
          <div class="text-muted-foreground">Loading logs...</div>
        </div>

        <div v-else>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Timestamp</TableHead>
                <TableHead>Server</TableHead>
                <TableHead>User</TableHead>
                <TableHead>Action</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              <TableRow v-for="log in logs" :key="log.id">
                <TableCell>{{ new Date(log.timestamp).toLocaleString() }}</TableCell>
                <TableCell>{{ log.server_name || "N/A" }}</TableCell>
                <TableCell>{{ log.username || "System" }}</TableCell>
                <TableCell>{{ log.action }}</TableCell>
              </TableRow>
            </TableBody>
          </Table>

          <div v-if="totalPages > 1" class="flex items-center justify-between mt-4">
            <div class="text-sm text-muted-foreground">Page {{ currentPage }} of {{ totalPages }}</div>
            <div class="flex gap-2">
              <Button @click="currentPage--; fetchLogs()" :disabled="currentPage === 1" size="sm">Previous</Button>
              <Button @click="currentPage++; fetchLogs()" :disabled="currentPage === totalPages" size="sm">Next</Button>
            </div>
          </div>
        </div>
      </CardContent>
    </Card>
  </div>
</template>

