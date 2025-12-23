<script setup lang="ts">
import { ref, onMounted } from "vue";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "~/components/ui/card";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "~/components/ui/table";
import { Button } from "~/components/ui/button";
import { Badge } from "~/components/ui/badge";
import type { SessionInfo } from "~/types";

definePageMeta({ middleware: "auth", layout: "sudo" });

const runtimeConfig = useRuntimeConfig();
const authStore = useAuthStore();

if (!authStore.user?.super_admin) navigateTo("/dashboard");

const loading = ref(true);
const sessions = ref<SessionInfo[]>([]);

const fetchSessions = async () => {
  loading.value = true;
  try {
    const res = await $fetch<any>(`${runtimeConfig.public.backendApi}/sudo/sessions`, {
      headers: { Authorization: `Bearer ${authStore.token}` },
    });
    sessions.value = res.data.sessions;
  } catch (err: any) {
    console.error("Error fetching sessions:", err);
  } finally {
    loading.value = false;
  }
};

const deleteSession = async (sessionId: string) => {
  if (!confirm("Are you sure you want to force logout this session?")) return;
  
  try {
    await $fetch(`${runtimeConfig.public.backendApi}/sudo/sessions/${sessionId}`, {
      method: "DELETE",
      headers: { Authorization: `Bearer ${authStore.token}` },
    });
    await fetchSessions();
  } catch (err: any) {
    console.error("Error deleting session:", err);
  }
};

onMounted(fetchSessions);
</script>

<template>
  <div class="p-6 space-y-6">
    <h1 class="text-3xl font-bold">Session Management</h1>

    <Card>
      <CardHeader>
        <CardTitle>Active Sessions</CardTitle>
        <CardDescription>Manage user login sessions</CardDescription>
      </CardHeader>
      <CardContent>
        <div v-if="loading" class="flex items-center justify-center py-12">
          <div class="text-muted-foreground">Loading sessions...</div>
        </div>

        <div v-else>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>User</TableHead>
                <TableHead>Last Seen</TableHead>
                <TableHead>IP Address</TableHead>
                <TableHead>Time Remaining</TableHead>
                <TableHead class="text-right">Actions</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              <TableRow v-for="session in sessions" :key="session.id">
                <TableCell class="font-medium">{{ session.username }}</TableCell>
                <TableCell>{{ new Date(session.last_seen).toLocaleString() }}</TableCell>
                <TableCell>{{ session.last_seen_ip }}</TableCell>
                <TableCell>
                  <Badge :variant="session.is_expired ? 'destructive' : 'default'">
                    {{ session.time_remaining }}
                  </Badge>
                </TableCell>
                <TableCell class="text-right">
                  <Button @click="deleteSession(session.id)" size="sm" variant="destructive">
                    <Icon name="mdi:logout" class="mr-2 h-4 w-4" />
                    Force Logout
                  </Button>
                </TableCell>
              </TableRow>
            </TableBody>
          </Table>
        </div>
      </CardContent>
    </Card>
  </div>
</template>

