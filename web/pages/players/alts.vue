<script setup lang="ts">
import { ref, onMounted, computed } from "vue";
import { useRouter } from "vue-router";
import { useAuthStore } from "@/stores/auth";
import { Card, CardContent } from "~/components/ui/card";
import { Badge } from "~/components/ui/badge";
import { Button } from "~/components/ui/button";
import { Input } from "~/components/ui/input";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "~/components/ui/table";
import {
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
} from "~/components/ui/collapsible";
import {
  Loader2,
  ChevronDown,
  ChevronLeft,
  ChevronRight,
  Users,
  AlertTriangle,
  Search,
  ExternalLink,
} from "lucide-vue-next";
import type {
  AltAccountGroup,
  AltAccountGroupsResponse,
} from "~/types/player";

const authStore = useAuthStore();
const runtimeConfig = useRuntimeConfig();
const router = useRouter();

// Redirect non-super admins
if (!authStore.user?.super_admin) {
  navigateTo("/players");
}

const loading = ref(true);
const error = ref<string | null>(null);
const altGroups = ref<AltAccountGroup[]>([]);
const totalGroups = ref(0);
const page = ref(1);
const limit = 20;
const expandedGroups = ref<Set<string>>(new Set());
const searchFilter = ref("");

async function fetchAltGroups(resetPage = false) {
  if (resetPage) {
    page.value = 1;
    altGroups.value = [];
  }

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
    const response = await fetch(
      `${runtimeConfig.public.backendApi}/players/alt-groups?page=${page.value}&limit=${limit}`,
      {
        method: "GET",
        headers: {
          "Content-Type": "application/json",
          Authorization: `Bearer ${token}`,
        },
        credentials: "include",
      }
    );

    if (response.status === 403) {
      navigateTo("/players");
      return;
    }

    if (!response.ok) {
      throw new Error("Failed to fetch alt account groups");
    }

    const data: { data: AltAccountGroupsResponse } = await response.json();
    altGroups.value = data.data.alt_groups || [];
    totalGroups.value = data.data.total_groups || 0;
  } catch (err: any) {
    error.value = err.message || "Failed to load alt account groups";
  } finally {
    loading.value = false;
  }
}

function toggleGroup(groupId: string) {
  if (expandedGroups.value.has(groupId)) {
    expandedGroups.value.delete(groupId);
  } else {
    expandedGroups.value.add(groupId);
  }
  expandedGroups.value = new Set(expandedGroups.value);
}

function viewPlayer(player: { steam_id: string; eos_id: string }) {
  const playerId = player.steam_id || player.eos_id;
  if (playerId) {
    router.push(`/players/${playerId}`);
  }
}

function getTimeAgo(dateString: string | null): string {
  if (!dateString) return "Unknown";
  const date = new Date(dateString);
  const now = new Date();
  const diff = now.getTime() - date.getTime();

  const minutes = Math.floor(diff / 60000);
  const hours = Math.floor(minutes / 60);
  const days = Math.floor(hours / 24);

  if (days > 0) return `${days}d ago`;
  if (hours > 0) return `${hours}h ago`;
  if (minutes > 0) return `${minutes}m ago`;
  return "Just now";
}

function formatDate(dateString: string | null): string {
  if (!dateString) return "N/A";
  return new Date(dateString).toLocaleString();
}

function nextPage() {
  if (page.value * limit < totalGroups.value) {
    page.value++;
    fetchAltGroups();
  }
}

function prevPage() {
  if (page.value > 1) {
    page.value--;
    fetchAltGroups();
  }
}

const filteredGroups = computed(() => {
  if (!searchFilter.value.trim()) return altGroups.value;

  const query = searchFilter.value.toLowerCase();
  return altGroups.value.filter((group) =>
    group.players.some(
      (player) =>
        player.player_name?.toLowerCase().includes(query) ||
        player.steam_id?.toLowerCase().includes(query) ||
        player.eos_id?.toLowerCase().includes(query)
    )
  );
});

const groupStats = computed(() => {
  const bannedGroups = altGroups.value.filter((g) =>
    g.players.some((p) => p.is_banned)
  ).length;
  const totalPlayers = altGroups.value.reduce(
    (sum, g) => sum + g.players.length,
    0
  );
  return { bannedGroups, totalPlayers };
});

onMounted(() => {
  fetchAltGroups();
});
</script>

<template>
  <div class="container mx-auto p-3 sm:p-4 lg:p-6">
    <!-- Header -->
    <div class="flex items-center gap-3 mb-6">
      <Button variant="ghost" size="icon" @click="router.push('/players')">
        <ChevronLeft class="h-5 w-5" />
      </Button>
      <div>
        <h1 class="text-xl sm:text-2xl font-bold flex items-center gap-2">
          <Users class="h-6 w-6 text-orange-500" />
          Suspected Alt Accounts
        </h1>
        <p class="text-sm text-muted-foreground mt-1">
          Players who have connected from the same IP address
        </p>
      </div>
    </div>

    <!-- Warning Banner -->
    <Card class="mb-6 border-orange-500/30 bg-orange-500/5">
      <CardContent class="py-4">
        <div class="flex items-start gap-3">
          <AlertTriangle class="h-5 w-5 text-orange-500 mt-0.5 shrink-0" />
          <div class="text-sm">
            <p class="font-medium text-orange-600 dark:text-orange-400">
              Important Notice
            </p>
            <p class="text-muted-foreground mt-1">
              Shared IP addresses may indicate alt accounts, but can also be
              legitimate cases such as players from the same household, LAN
              parties, internet cafes, or shared networks. Use this information
              as one factor among many when investigating.
            </p>
          </div>
        </div>
      </CardContent>
    </Card>

    <!-- Stats Summary -->
    <div class="grid grid-cols-2 sm:grid-cols-4 gap-3 mb-6">
      <Card>
        <CardContent class="py-3 px-4">
          <p class="text-xs text-muted-foreground">Total Groups</p>
          <p class="text-lg font-semibold">{{ totalGroups }}</p>
        </CardContent>
      </Card>
      <Card>
        <CardContent class="py-3 px-4">
          <p class="text-xs text-muted-foreground">Loaded Groups</p>
          <p class="text-lg font-semibold">{{ altGroups.length }}</p>
        </CardContent>
      </Card>
      <Card>
        <CardContent class="py-3 px-4">
          <p class="text-xs text-muted-foreground">Players in Groups</p>
          <p class="text-lg font-semibold">{{ groupStats.totalPlayers }}</p>
        </CardContent>
      </Card>
      <Card>
        <CardContent class="py-3 px-4">
          <p class="text-xs text-muted-foreground">Groups with Bans</p>
          <p class="text-lg font-semibold text-destructive">
            {{ groupStats.bannedGroups }}
          </p>
        </CardContent>
      </Card>
    </div>

    <!-- Search/Filter -->
    <div class="mb-4">
      <div class="relative max-w-md">
        <Search
          class="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground"
        />
        <Input
          v-model="searchFilter"
          placeholder="Filter by player name or ID..."
          class="pl-10"
        />
      </div>
    </div>

    <!-- Loading State -->
    <div v-if="loading && altGroups.length === 0" class="py-12 text-center">
      <Loader2 class="h-8 w-8 animate-spin mx-auto text-muted-foreground" />
      <p class="text-sm text-muted-foreground mt-3">
        Loading alt account groups...
      </p>
    </div>

    <!-- Error State -->
    <Card v-else-if="error" class="py-8">
      <CardContent class="text-center">
        <p class="text-destructive">{{ error }}</p>
        <Button variant="outline" size="sm" class="mt-4" @click="fetchAltGroups(true)">
          Retry
        </Button>
      </CardContent>
    </Card>

    <!-- Empty State -->
    <Card v-else-if="altGroups.length === 0" class="py-8">
      <CardContent class="text-center text-muted-foreground">
        <Users class="h-12 w-12 mx-auto mb-3 opacity-50" />
        <p>No suspected alt accounts found</p>
      </CardContent>
    </Card>

    <!-- Alt Groups List -->
    <div v-else class="space-y-3">
      <Card
        v-for="group in filteredGroups"
        :key="group.group_id"
        class="overflow-hidden"
        :class="{
          'border-destructive/30': group.players.some((p) => p.is_banned),
        }"
      >
        <Collapsible
          :open="expandedGroups.has(group.group_id)"
          @update:open="toggleGroup(group.group_id)"
        >
          <CollapsibleTrigger class="w-full">
            <div
              class="flex items-center justify-between p-4 hover:bg-muted/30 transition-colors cursor-pointer"
            >
              <div class="flex items-center gap-4">
                <!-- Player Avatars -->
                <div class="flex -space-x-2">
                  <div
                    v-for="(player, idx) in group.players.slice(0, 4)"
                    :key="idx"
                    class="w-10 h-10 rounded-full bg-muted border-2 border-background flex items-center justify-center text-sm font-medium"
                    :class="{
                      'bg-destructive/20 text-destructive border-destructive/30':
                        player.is_banned,
                    }"
                    :title="player.player_name || 'Unknown'"
                  >
                    {{ player.player_name?.charAt(0)?.toUpperCase() || "?" }}
                  </div>
                  <div
                    v-if="group.players.length > 4"
                    class="w-10 h-10 rounded-full bg-muted border-2 border-background flex items-center justify-center text-sm"
                  >
                    +{{ group.players.length - 4 }}
                  </div>
                </div>

                <!-- Group Info -->
                <div class="text-left">
                  <div class="flex items-center gap-2">
                    <span class="font-medium">
                      {{ group.players.length }} Players
                    </span>
                    <Badge
                      v-if="group.players.some((p) => p.is_banned)"
                      variant="destructive"
                      class="text-xs"
                    >
                      {{ group.players.filter((p) => p.is_banned).length }}
                      Banned
                    </Badge>
                  </div>
                  <p class="text-sm text-muted-foreground">
                    Last activity: {{ getTimeAgo(group.last_activity) }}
                    <span class="hidden sm:inline">
                      ({{ formatDate(group.last_activity) }})
                    </span>
                  </p>
                </div>
              </div>

              <ChevronDown
                class="h-5 w-5 transition-transform"
                :class="{ 'rotate-180': expandedGroups.has(group.group_id) }"
              />
            </div>
          </CollapsibleTrigger>

          <CollapsibleContent>
            <div class="border-t">
              <!-- Desktop Table -->
              <div class="hidden md:block">
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>Player</TableHead>
                      <TableHead>Steam ID</TableHead>
                      <TableHead>EOS ID</TableHead>
                      <TableHead class="text-center">Sessions</TableHead>
                      <TableHead>Last Seen</TableHead>
                      <TableHead>Status</TableHead>
                      <TableHead class="w-10"></TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    <TableRow
                      v-for="player in group.players"
                      :key="player.steam_id || player.eos_id"
                      class="cursor-pointer hover:bg-muted/50"
                      @click="viewPlayer(player)"
                    >
                      <TableCell class="font-medium">
                        {{ player.player_name || "Unknown" }}
                      </TableCell>
                      <TableCell>
                        <code
                          v-if="player.steam_id"
                          class="text-xs bg-muted px-1.5 py-0.5 rounded"
                        >
                          {{ player.steam_id }}
                        </code>
                        <span v-else class="text-muted-foreground">-</span>
                      </TableCell>
                      <TableCell>
                        <code
                          v-if="player.eos_id"
                          class="text-xs bg-muted px-1.5 py-0.5 rounded"
                        >
                          {{ player.eos_id.slice(0, 16) }}...
                        </code>
                        <span v-else class="text-muted-foreground">-</span>
                      </TableCell>
                      <TableCell class="text-center">
                        {{ player.shared_sessions }}
                      </TableCell>
                      <TableCell>
                        <div class="text-sm">
                          {{ getTimeAgo(player.last_seen) }}
                        </div>
                        <div class="text-xs text-muted-foreground">
                          {{ formatDate(player.last_seen) }}
                        </div>
                      </TableCell>
                      <TableCell>
                        <Badge v-if="player.is_banned" variant="destructive">
                          Banned
                        </Badge>
                        <Badge v-else variant="outline" class="text-green-600">
                          Active
                        </Badge>
                      </TableCell>
                      <TableCell>
                        <ExternalLink class="h-4 w-4 text-muted-foreground" />
                      </TableCell>
                    </TableRow>
                  </TableBody>
                </Table>
              </div>

              <!-- Mobile Cards -->
              <div class="md:hidden p-3 space-y-2">
                <div
                  v-for="player in group.players"
                  :key="player.steam_id || player.eos_id"
                  class="p-3 rounded-lg border hover:bg-muted/30 cursor-pointer"
                  @click="viewPlayer(player)"
                >
                  <div class="flex items-center justify-between mb-2">
                    <span class="font-medium">
                      {{ player.player_name || "Unknown" }}
                    </span>
                    <Badge v-if="player.is_banned" variant="destructive">
                      Banned
                    </Badge>
                    <Badge v-else variant="outline" class="text-green-600">
                      Active
                    </Badge>
                  </div>
                  <div class="space-y-1 text-sm text-muted-foreground">
                    <div v-if="player.steam_id">
                      Steam:
                      <code class="text-xs bg-muted px-1 rounded">
                        {{ player.steam_id }}
                      </code>
                    </div>
                    <div class="flex justify-between">
                      <span>Sessions: {{ player.shared_sessions }}</span>
                      <span>{{ getTimeAgo(player.last_seen) }}</span>
                    </div>
                  </div>
                </div>
              </div>
            </div>
          </CollapsibleContent>
        </Collapsible>
      </Card>

      <!-- Pagination -->
      <div class="flex items-center justify-between pt-4">
        <p class="text-sm text-muted-foreground">
          Showing {{ altGroups.length }} of {{ totalGroups }} groups
        </p>
        <div class="flex gap-2">
          <Button
            variant="outline"
            size="sm"
            :disabled="page <= 1 || loading"
            @click="prevPage"
          >
            <ChevronLeft class="h-4 w-4 mr-1" />
            Previous
          </Button>
          <Button
            variant="outline"
            size="sm"
            :disabled="page * limit >= totalGroups || loading"
            @click="nextPage"
          >
            Next
            <ChevronRight class="h-4 w-4 ml-1" />
          </Button>
        </div>
      </div>
    </div>
  </div>
</template>
