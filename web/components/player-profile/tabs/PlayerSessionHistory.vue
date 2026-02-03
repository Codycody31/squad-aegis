<script setup lang="ts">
import { ref, onMounted } from "vue";
import { Card, CardContent, CardHeader, CardTitle } from "~/components/ui/card";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "~/components/ui/table";
import { Button } from "~/components/ui/button";
import { Badge } from "~/components/ui/badge";
import { Loader2, ChevronLeft, ChevronRight, Network } from "lucide-vue-next";
import type { SessionHistoryEntry } from "~/types/player";

const props = defineProps<{
  playerId: string;
}>();

const runtimeConfig = useRuntimeConfig();
const loading = ref(false);
const error = ref<string | null>(null);

const sessions = ref<SessionHistoryEntry[]>([]);
const canViewIP = ref(false);
const page = ref(1);
const limit = ref(50);

async function fetchSessions() {
  loading.value = true;
  error.value = null;

  const cookieToken = useCookie(
    runtimeConfig.public.sessionCookieName as string
  );
  const token = cookieToken.value;

  if (!token) {
    error.value = "Authentication required";
    loading.value = false;
    return;
  }

  try {
    const params = new URLSearchParams({
      page: page.value.toString(),
      limit: limit.value.toString(),
    });

    const response = await fetch(
      `${runtimeConfig.public.backendApi}/players/${props.playerId}/sessions?${params}`,
      {
        headers: {
          Authorization: `Bearer ${token}`,
        },
      }
    );

    if (!response.ok) throw new Error("Failed to fetch session history");

    const data = await response.json();
    sessions.value = data.data.sessions || [];
    canViewIP.value = data.data.can_view_ip || false;
  } catch (err: any) {
    error.value = err.message;
  } finally {
    loading.value = false;
  }
}

function formatDate(dateString: string): string {
  return new Date(dateString).toLocaleString();
}

function getTimeAgo(dateString: string): string {
  const date = new Date(dateString);
  const now = new Date();
  const diff = now.getTime() - date.getTime();
  const days = Math.floor(diff / (1000 * 60 * 60 * 24));
  if (days > 30) return `${Math.floor(days / 30)} months ago`;
  if (days > 0) return `${days} days ago`;
  const hours = Math.floor(diff / (1000 * 60 * 60));
  if (hours > 0) return `${hours} hours ago`;
  const minutes = Math.floor(diff / (1000 * 60));
  return `${minutes} minutes ago`;
}

function nextPage() {
  page.value++;
  fetchSessions();
}

function prevPage() {
  if (page.value > 1) {
    page.value--;
    fetchSessions();
  }
}

onMounted(() => {
  fetchSessions();
});
</script>

<template>
  <Card>
    <CardHeader class="pb-3">
      <CardTitle class="text-lg flex items-center gap-2">
        <Network class="h-5 w-5" />
        Session History
      </CardTitle>
    </CardHeader>
    <CardContent>
      <div v-if="loading" class="flex justify-center py-8">
        <Loader2 class="h-8 w-8 animate-spin text-muted-foreground" />
      </div>

      <div v-else-if="error" class="text-center py-8 text-destructive">
        {{ error }}
      </div>

      <div v-else-if="sessions.length === 0" class="text-center py-8 text-muted-foreground">
        No session history available
      </div>

      <div v-else>
        <!-- Desktop Table -->
        <div class="hidden md:block overflow-x-auto">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Time</TableHead>
                <TableHead>Server</TableHead>
                <TableHead v-if="canViewIP">IP Address</TableHead>
                <TableHead>Event</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              <TableRow
                v-for="(session, index) in sessions"
                :key="index"
                class="hover:bg-muted/50"
              >
                <TableCell>
                  <div class="text-sm">{{ formatDate(session.event_time) }}</div>
                  <div class="text-xs text-muted-foreground">
                    {{ getTimeAgo(session.event_time) }}
                  </div>
                </TableCell>
                <TableCell>
                  <div class="text-sm font-medium">
                    {{ session.server_name || "Unknown Server" }}
                  </div>
                </TableCell>
                <TableCell v-if="canViewIP">
                  <code class="text-xs bg-muted px-2 py-0.5 rounded">
                    {{ session.ip || "N/A" }}
                  </code>
                </TableCell>
                <TableCell>
                  <Badge
                    :variant="session.event_type === 'connected' ? 'default' : 'secondary'"
                  >
                    {{ session.event_type }}
                  </Badge>
                </TableCell>
              </TableRow>
            </TableBody>
          </Table>
        </div>

        <!-- Mobile Cards -->
        <div class="md:hidden space-y-3">
          <div
            v-for="(session, index) in sessions"
            :key="index"
            class="border rounded-lg p-3 hover:bg-muted/30 transition-colors"
          >
            <div class="flex items-center justify-between mb-2">
              <Badge
                :variant="session.event_type === 'connected' ? 'default' : 'secondary'"
              >
                {{ session.event_type }}
              </Badge>
              <span class="text-xs text-muted-foreground">
                {{ getTimeAgo(session.event_time) }}
              </span>
            </div>
            <div class="space-y-1 text-sm">
              <div>
                <span class="text-muted-foreground">Server: </span>
                {{ session.server_name || "Unknown" }}
              </div>
              <div>
                <span class="text-muted-foreground">Time: </span>
                {{ formatDate(session.event_time) }}
              </div>
              <div v-if="canViewIP && session.ip">
                <span class="text-muted-foreground">IP: </span>
                <code class="text-xs bg-muted px-1 rounded">{{ session.ip }}</code>
              </div>
            </div>
          </div>
        </div>

        <!-- Pagination -->
        <div class="flex items-center justify-between mt-4 pt-4 border-t">
          <div class="text-sm text-muted-foreground">
            Page {{ page }}
          </div>
          <div class="flex gap-2">
            <Button
              variant="outline"
              size="sm"
              :disabled="page <= 1"
              @click="prevPage"
            >
              <ChevronLeft class="h-4 w-4" />
            </Button>
            <Button
              variant="outline"
              size="sm"
              :disabled="sessions.length < limit"
              @click="nextPage"
            >
              <ChevronRight class="h-4 w-4" />
            </Button>
          </div>
        </div>
      </div>
    </CardContent>
  </Card>
</template>
