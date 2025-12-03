<script setup lang="ts">
import { ref, computed, onMounted } from "vue";
import { useRouter } from "vue-router";
import { Button } from "~/components/ui/button";
import { Input } from "~/components/ui/input";
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from "~/components/ui/card";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "~/components/ui/table";
import { Badge } from "~/components/ui/badge";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "~/components/ui/tabs";
import { useAuthStore } from "@/stores/auth";

const authStore = useAuthStore();
const runtimeConfig = useRuntimeConfig();
const router = useRouter();

const loading = ref(false);
const statsLoading = ref(true);
const error = ref<string | null>(null);
const searchQuery = ref("");
const players = ref<PlayerSearchResult[]>([]);
const stats = ref<PlayerStatsSummary | null>(null);

interface PlayerSearchResult {
  steam_id: string;
  eos_id: string;
  player_name: string;
  last_seen: string | null;
  first_seen: string | null;
}

interface TopPlayerStats {
  steam_id: string;
  eos_id: string;
  player_name: string;
  kills: number;
  deaths: number;
  kd_ratio: number;
  teamkills: number;
  revives: number;
}

interface PlayerStatsSummary {
  top_players: TopPlayerStats[];
  top_teamkillers: TopPlayerStats[];
  top_medics: TopPlayerStats[];
  most_recent_players: PlayerSearchResult[];
  total_players: number;
  total_kills: number;
  total_deaths: number;
  total_teamkills: number;
}

interface PlayersResponse {
  data: {
    players: PlayerSearchResult[];
    count: number;
  };
}

interface StatsResponse {
  data: {
    stats: PlayerStatsSummary;
  };
}

// Function to fetch player statistics
async function fetchPlayerStats() {
  statsLoading.value = true;

  const cookieToken = useCookie(
    runtimeConfig.public.sessionCookieName as string
  );
  const token = cookieToken.value;

  if (!token) {
    statsLoading.value = false;
    return;
  }

  try {
    const response = await fetch(
      `${runtimeConfig.public.backendApi}/players/stats`,
      {
        method: "GET",
        headers: {
          "Content-Type": "application/json",
          Authorization: `Bearer ${token}`,
        },
        credentials: "include",
      }
    );

    if (!response.ok) {
      throw new Error("Failed to fetch player statistics");
    }

    const data: StatsResponse = await response.json();
    stats.value = data.data.stats;
  } catch (err: any) {
    console.error("Failed to load statistics:", err);
  } finally {
    statsLoading.value = false;
  }
}

// Function to search players
async function searchPlayers() {
  if (!searchQuery.value.trim()) {
    error.value = "Please enter a search query (player name, Steam ID, or EOS ID)";
    return;
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
    players.value = [];
    return;
  }

  try {
    const response = await fetch(
      `${runtimeConfig.public.backendApi}/players?search=${encodeURIComponent(searchQuery.value)}`,
      {
        method: "GET",
        headers: {
          "Content-Type": "application/json",
          Authorization: `Bearer ${token}`,
        },
        credentials: "include",
      }
    );

    if (!response.ok) {
      const errorData = await response.json();
      throw new Error(errorData.message || "Failed to search players");
    }

    const data: PlayersResponse = await response.json();
    players.value = data.data.players || [];
  } catch (err: any) {
    error.value = err.message || "An error occurred while searching";
    players.value = [];
  } finally {
    loading.value = false;
  }
}

// Function to format date
function formatDate(dateString: string | null): string {
  if (!dateString) return "N/A";
  const date = new Date(dateString);
  return date.toLocaleString();
}

// Function to get time ago
function getTimeAgo(dateString: string | null): string {
  if (!dateString) return "N/A";
  const date = new Date(dateString);
  const now = new Date();
  const diff = now.getTime() - date.getTime();
  
  const seconds = Math.floor(diff / 1000);
  const minutes = Math.floor(seconds / 60);
  const hours = Math.floor(minutes / 60);
  const days = Math.floor(hours / 24);
  const months = Math.floor(days / 30);
  const years = Math.floor(days / 365);

  if (years > 0) return `${years} year${years > 1 ? "s" : ""} ago`;
  if (months > 0) return `${months} month${months > 1 ? "s" : ""} ago`;
  if (days > 0) return `${days} day${days > 1 ? "s" : ""} ago`;
  if (hours > 0) return `${hours} hour${hours > 1 ? "s" : ""} ago`;
  if (minutes > 0) return `${minutes} minute${minutes > 1 ? "s" : ""} ago`;
  return "Just now";
}

// Function to navigate to player profile
function viewPlayerProfile(player: PlayerSearchResult | TopPlayerStats) {
  const playerId = player.steam_id || player.eos_id;
  if (playerId) {
    router.push(`/players/${playerId}`);
  }
}

// Handle Enter key press in search input
function handleKeyPress(event: KeyboardEvent) {
  if (event.key === "Enter") {
    searchPlayers();
  }
}

// Function to format large numbers
function formatNumber(num: number): string {
  if (num >= 1000000) {
    return (num / 1000000).toFixed(1) + "M";
  }
  if (num >= 1000) {
    return (num / 1000).toFixed(1) + "K";
  }
  return num.toString();
}

// Redirect to login if not authenticated
if (!authStore.isLoggedIn) {
  navigateTo("/login");
}

onMounted(() => {
  fetchPlayerStats();
});
</script>

<template>
  <div class="container mx-auto p-3 sm:p-4 lg:p-6">
    <div class="flex justify-between items-center mb-4 sm:mb-6">
      <h1 class="text-xl sm:text-2xl lg:text-3xl font-bold">Player Profiles</h1>
    </div>

    <!-- Overall Statistics -->
    <div v-if="stats && !statsLoading" class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-3 sm:gap-4 mb-4 sm:mb-6">
      <Card>
        <CardHeader class="pb-2">
          <CardTitle class="text-xs sm:text-sm font-medium text-muted-foreground">Total Players</CardTitle>
        </CardHeader>
        <CardContent>
          <div class="text-xl sm:text-2xl font-bold">{{ formatNumber(stats.total_players) }}</div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader class="pb-2">
          <CardTitle class="text-xs sm:text-sm font-medium text-muted-foreground">Total Kills</CardTitle>
        </CardHeader>
        <CardContent>
          <div class="text-xl sm:text-2xl font-bold">{{ formatNumber(stats.total_kills) }}</div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader class="pb-2">
          <CardTitle class="text-xs sm:text-sm font-medium text-muted-foreground">Total Deaths</CardTitle>
        </CardHeader>
        <CardContent>
          <div class="text-xl sm:text-2xl font-bold">{{ formatNumber(stats.total_deaths) }}</div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader class="pb-2">
          <CardTitle class="text-xs sm:text-sm font-medium text-muted-foreground">Total Teamkills</CardTitle>
        </CardHeader>
        <CardContent>
          <div class="text-xl sm:text-2xl font-bold text-destructive">{{ formatNumber(stats.total_teamkills) }}</div>
        </CardContent>
      </Card>
    </div>

    <!-- Statistics Tabs -->
    <Tabs v-if="stats && !statsLoading" default-value="top-players" class="mb-4 sm:mb-6">
      <TabsList class="grid w-full grid-cols-2 sm:grid-cols-4">
        <TabsTrigger value="top-players" class="text-xs sm:text-sm">Top Players</TabsTrigger>
        <TabsTrigger value="top-teamkillers" class="text-xs sm:text-sm">Top Teamkillers</TabsTrigger>
        <TabsTrigger value="top-medics" class="text-xs sm:text-sm">Top Medics</TabsTrigger>
        <TabsTrigger value="recent" class="text-xs sm:text-sm">Most Recent</TabsTrigger>
      </TabsList>

      <TabsContent value="top-players">
        <Card>
          <CardHeader class="pb-2 sm:pb-3">
            <CardTitle class="text-base sm:text-lg">Top 10 Players by K/D Ratio</CardTitle>
            <CardDescription class="text-xs sm:text-sm">Minimum 10 kills required</CardDescription>
          </CardHeader>
          <CardContent>
            <!-- Desktop Table View -->
            <div class="hidden md:block w-full overflow-x-auto">
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead class="text-xs sm:text-sm">Rank</TableHead>
                    <TableHead class="text-xs sm:text-sm">Player</TableHead>
                    <TableHead class="text-right text-xs sm:text-sm">K/D Ratio</TableHead>
                    <TableHead class="text-right text-xs sm:text-sm">Kills</TableHead>
                    <TableHead class="text-right text-xs sm:text-sm">Deaths</TableHead>
                    <TableHead class="text-right text-xs sm:text-sm">Actions</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  <TableRow
                    v-for="(player, index) in stats.top_players"
                    :key="player.steam_id || player.eos_id"
                    class="cursor-pointer hover:bg-muted/50"
                    @click="viewPlayerProfile(player)"
                  >
                    <TableCell>
                      <Badge v-if="index === 0" variant="default" class="text-xs">🥇</Badge>
                      <Badge v-else-if="index === 1" variant="secondary" class="text-xs">🥈</Badge>
                      <Badge v-else-if="index === 2" variant="outline" class="text-xs">🥉</Badge>
                      <span v-else class="text-xs sm:text-sm text-muted-foreground">{{ index + 1 }}</span>
                    </TableCell>
                    <TableCell class="font-medium text-sm sm:text-base">{{ player.player_name }}</TableCell>
                    <TableCell class="text-right font-bold text-green-500 text-xs sm:text-sm">{{ player.kd_ratio.toFixed(2) }}</TableCell>
                    <TableCell class="text-right text-xs sm:text-sm">{{ player.kills }}</TableCell>
                    <TableCell class="text-right text-xs sm:text-sm">{{ player.deaths }}</TableCell>
                    <TableCell class="text-right">
                      <RouterLink :to="`/players/${player.steam_id || player.eos_id}`" as-child>
                        <Button size="sm" variant="ghost" class="text-xs">
                        View
                      </Button>
                      </RouterLink>
                    </TableCell>
                  </TableRow>
                </TableBody>
              </Table>
            </div>

            <!-- Mobile Card View -->
            <div class="md:hidden space-y-3">
              <div
                v-for="(player, index) in stats.top_players"
                :key="player.steam_id || player.eos_id"
                class="border rounded-lg p-3 sm:p-4 hover:bg-muted/30 transition-colors cursor-pointer"
                @click="viewPlayerProfile(player)"
              >
                <div class="flex items-start justify-between gap-2 mb-2">
                  <div class="flex-1 min-w-0">
                    <div class="flex items-center gap-2 mb-1">
                      <Badge v-if="index === 0" variant="default" class="text-xs">🥇</Badge>
                      <Badge v-else-if="index === 1" variant="secondary" class="text-xs">🥈</Badge>
                      <Badge v-else-if="index === 2" variant="outline" class="text-xs">🥉</Badge>
                      <span v-else class="text-xs text-muted-foreground">{{ index + 1 }}</span>
                      <span class="font-semibold text-sm sm:text-base">{{ player.player_name }}</span>
                    </div>
                    <div class="space-y-1.5">
                      <div>
                        <span class="text-xs text-muted-foreground">K/D Ratio: </span>
                        <span class="text-xs sm:text-sm font-bold text-green-500">{{ player.kd_ratio.toFixed(2) }}</span>
                      </div>
                      <div>
                        <span class="text-xs text-muted-foreground">Kills: </span>
                        <span class="text-xs sm:text-sm">{{ player.kills }}</span>
                        <span class="text-xs text-muted-foreground ml-2">Deaths: </span>
                        <span class="text-xs sm:text-sm">{{ player.deaths }}</span>
                      </div>
                    </div>
                  </div>
                </div>
                <div class="flex items-center justify-end gap-2 pt-2 border-t">
                  <RouterLink :to="`/players/${player.steam_id || player.eos_id}`" class="w-full">
                    <Button size="sm" variant="ghost" class="w-full h-8 text-xs">
                      View Profile
                    </Button>
                  </RouterLink>
                </div>
              </div>
            </div>
          </CardContent>
        </Card>
      </TabsContent>

      <TabsContent value="top-teamkillers">
        <Card>
          <CardHeader class="pb-2 sm:pb-3">
            <CardTitle class="text-base sm:text-lg">Top 10 Teamkillers</CardTitle>
            <CardDescription class="text-xs sm:text-sm">Players with most teamkills</CardDescription>
          </CardHeader>
          <CardContent>
            <!-- Desktop Table View -->
            <div class="hidden md:block w-full overflow-x-auto">
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead class="text-xs sm:text-sm">Rank</TableHead>
                    <TableHead class="text-xs sm:text-sm">Player</TableHead>
                    <TableHead class="text-right text-xs sm:text-sm">Teamkills</TableHead>
                    <TableHead class="text-right text-xs sm:text-sm">Actions</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  <TableRow
                    v-for="(player, index) in stats.top_teamkillers"
                    :key="player.steam_id || player.eos_id"
                    class="cursor-pointer hover:bg-muted/50"
                    @click="viewPlayerProfile(player)"
                  >
                    <TableCell class="text-xs sm:text-sm">{{ index + 1 }}</TableCell>
                    <TableCell class="font-medium text-sm sm:text-base">{{ player.player_name }}</TableCell>
                    <TableCell class="text-right font-bold text-destructive text-xs sm:text-sm">{{ player.teamkills }}</TableCell>
                    <TableCell class="text-right">
                      <RouterLink :to="`/players/${player.steam_id || player.eos_id}`" as-child>
                        <Button size="sm" variant="ghost" class="text-xs">
                        View
                      </Button>
                      </RouterLink>
                    </TableCell>
                  </TableRow>
                </TableBody>
              </Table>
            </div>

            <!-- Mobile Card View -->
            <div class="md:hidden space-y-3">
              <div
                v-for="(player, index) in stats.top_teamkillers"
                :key="player.steam_id || player.eos_id"
                class="border rounded-lg p-3 sm:p-4 hover:bg-muted/30 transition-colors cursor-pointer"
                @click="viewPlayerProfile(player)"
              >
                <div class="flex items-start justify-between gap-2 mb-2">
                  <div class="flex-1 min-w-0">
                    <div class="flex items-center gap-2 mb-1">
                      <span class="text-xs text-muted-foreground">#{{ index + 1 }}</span>
                      <span class="font-semibold text-sm sm:text-base">{{ player.player_name }}</span>
                    </div>
                    <div>
                      <span class="text-xs text-muted-foreground">Teamkills: </span>
                      <span class="text-xs sm:text-sm font-bold text-destructive">{{ player.teamkills }}</span>
                    </div>
                  </div>
                </div>
                <div class="flex items-center justify-end gap-2 pt-2 border-t">
                  <RouterLink :to="`/players/${player.steam_id || player.eos_id}`" class="w-full">
                    <Button size="sm" variant="ghost" class="w-full h-8 text-xs">
                      View Profile
                    </Button>
                  </RouterLink>
                </div>
              </div>
            </div>
          </CardContent>
        </Card>
      </TabsContent>

      <TabsContent value="top-medics">
        <Card>
          <CardHeader class="pb-2 sm:pb-3">
            <CardTitle class="text-base sm:text-lg">Top 10 Medics</CardTitle>
            <CardDescription class="text-xs sm:text-sm">Players with most revives</CardDescription>
          </CardHeader>
          <CardContent>
            <!-- Desktop Table View -->
            <div class="hidden md:block w-full overflow-x-auto">
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead class="text-xs sm:text-sm">Rank</TableHead>
                    <TableHead class="text-xs sm:text-sm">Player</TableHead>
                    <TableHead class="text-right text-xs sm:text-sm">Revives</TableHead>
                    <TableHead class="text-right text-xs sm:text-sm">Actions</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  <TableRow
                    v-for="(player, index) in stats.top_medics"
                    :key="player.steam_id || player.eos_id"
                    class="cursor-pointer hover:bg-muted/50"
                    @click="viewPlayerProfile(player)"
                  >
                    <TableCell>
                      <Badge v-if="index === 0" variant="default" class="text-xs">🥇</Badge>
                      <Badge v-else-if="index === 1" variant="secondary" class="text-xs">🥈</Badge>
                      <Badge v-else-if="index === 2" variant="outline" class="text-xs">🥉</Badge>
                      <span v-else class="text-xs sm:text-sm text-muted-foreground">{{ index + 1 }}</span>
                    </TableCell>
                    <TableCell class="font-medium text-sm sm:text-base">{{ player.player_name }}</TableCell>
                    <TableCell class="text-right font-bold text-green-500 text-xs sm:text-sm">{{ player.revives }}</TableCell>
                    <TableCell class="text-right">
                      <RouterLink :to="`/players/${player.steam_id || player.eos_id}`" as-child>
                        <Button size="sm" variant="ghost" class="text-xs">
                        View
                      </Button>
                      </RouterLink>
                    </TableCell>
                  </TableRow>
                </TableBody>
              </Table>
            </div>

            <!-- Mobile Card View -->
            <div class="md:hidden space-y-3">
              <div
                v-for="(player, index) in stats.top_medics"
                :key="player.steam_id || player.eos_id"
                class="border rounded-lg p-3 sm:p-4 hover:bg-muted/30 transition-colors cursor-pointer"
                @click="viewPlayerProfile(player)"
              >
                <div class="flex items-start justify-between gap-2 mb-2">
                  <div class="flex-1 min-w-0">
                    <div class="flex items-center gap-2 mb-1">
                      <Badge v-if="index === 0" variant="default" class="text-xs">🥇</Badge>
                      <Badge v-else-if="index === 1" variant="secondary" class="text-xs">🥈</Badge>
                      <Badge v-else-if="index === 2" variant="outline" class="text-xs">🥉</Badge>
                      <span v-else class="text-xs text-muted-foreground">{{ index + 1 }}</span>
                      <span class="font-semibold text-sm sm:text-base">{{ player.player_name }}</span>
                    </div>
                    <div>
                      <span class="text-xs text-muted-foreground">Revives: </span>
                      <span class="text-xs sm:text-sm font-bold text-green-500">{{ player.revives }}</span>
                    </div>
                  </div>
                </div>
                <div class="flex items-center justify-end gap-2 pt-2 border-t">
                  <RouterLink :to="`/players/${player.steam_id || player.eos_id}`" class="w-full">
                    <Button size="sm" variant="ghost" class="w-full h-8 text-xs">
                      View Profile
                    </Button>
                  </RouterLink>
                </div>
              </div>
            </div>
          </CardContent>
        </Card>
      </TabsContent>

      <TabsContent value="recent">
        <Card>
          <CardHeader class="pb-2 sm:pb-3">
            <CardTitle class="text-base sm:text-lg">Most Recent Players</CardTitle>
            <CardDescription class="text-xs sm:text-sm">Last 10 players seen on servers</CardDescription>
          </CardHeader>
          <CardContent>
            <!-- Desktop Table View -->
            <div class="hidden md:block w-full overflow-x-auto">
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead class="text-xs sm:text-sm">Player</TableHead>
                    <TableHead class="text-xs sm:text-sm">Last Seen</TableHead>
                    <TableHead class="text-right text-xs sm:text-sm">Actions</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  <TableRow
                    v-for="player in stats.most_recent_players"
                    :key="player.steam_id || player.eos_id"
                    class="cursor-pointer hover:bg-muted/50"
                    @click="viewPlayerProfile(player)"
                  >
                    <TableCell class="font-medium text-sm sm:text-base">{{ player.player_name }}</TableCell>
                    <TableCell>
                      <div class="text-xs sm:text-sm">{{ getTimeAgo(player.last_seen) }}</div>
                      <div class="text-xs text-muted-foreground">{{ formatDate(player.last_seen) }}</div>
                    </TableCell>
                    <TableCell class="text-right">
                      <RouterLink :to="`/players/${player.steam_id || player.eos_id}`" as-child>
                        <Button size="sm" variant="ghost" class="text-xs">
                        View
                      </Button>
                      </RouterLink>
                    </TableCell>
                  </TableRow>
                </TableBody>
              </Table>
            </div>

            <!-- Mobile Card View -->
            <div class="md:hidden space-y-3">
              <div
                v-for="player in stats.most_recent_players"
                :key="player.steam_id || player.eos_id"
                class="border rounded-lg p-3 sm:p-4 hover:bg-muted/30 transition-colors cursor-pointer"
                @click="viewPlayerProfile(player)"
              >
                <div class="flex items-start justify-between gap-2 mb-2">
                  <div class="flex-1 min-w-0">
                    <div class="font-semibold text-sm sm:text-base mb-1">
                      {{ player.player_name }}
                    </div>
                    <div>
                      <div class="text-xs sm:text-sm">{{ getTimeAgo(player.last_seen) }}</div>
                      <div class="text-xs text-muted-foreground">{{ formatDate(player.last_seen) }}</div>
                    </div>
                  </div>
                </div>
                <div class="flex items-center justify-end gap-2 pt-2 border-t">
                  <RouterLink :to="`/players/${player.steam_id || player.eos_id}`" class="w-full">
                    <Button size="sm" variant="ghost" class="w-full h-8 text-xs">
                      View Profile
                    </Button>
                  </RouterLink>
                </div>
              </div>
            </div>
          </CardContent>
        </Card>
      </TabsContent>
    </Tabs>

    <!-- Loading State for Stats -->
    <div v-if="statsLoading" class="mb-4 sm:mb-6">
      <Card>
        <CardContent class="py-8 sm:py-12">
          <div class="flex justify-center items-center">
            <div class="animate-spin rounded-full h-12 w-12 border-b-2 border-primary"></div>
          </div>
          <p class="text-center text-xs sm:text-sm text-muted-foreground mt-4">Loading player statistics...</p>
        </CardContent>
      </Card>
    </div>

    <!-- Search Section -->
    <Card class="mb-4 sm:mb-6">
      <CardHeader class="pb-2 sm:pb-3">
        <CardTitle class="text-base sm:text-lg">Search Players</CardTitle>
      </CardHeader>
      <CardContent>
        <div class="flex flex-col sm:flex-row gap-2 sm:gap-4">
          <Input
            v-model="searchQuery"
            placeholder="Search by player name, Steam ID, or EOS ID..."
            class="flex-1 text-sm sm:text-base"
            @keypress="handleKeyPress"
          />
          <Button @click="searchPlayers" :disabled="loading" class="w-full sm:w-auto text-sm sm:text-base">
            {{ loading ? "Searching..." : "Search" }}
          </Button>
        </div>
        <p class="text-xs sm:text-sm text-muted-foreground mt-2">
          Enter a player name, Steam ID, or EOS ID to search for players
        </p>
      </CardContent>
    </Card>

    <div v-if="error" class="mb-3 sm:mb-4 p-3 sm:p-4 bg-destructive/15 text-destructive rounded-md text-sm sm:text-base">
      {{ error }}
    </div>

    <Card v-if="players.length > 0">
      <CardHeader class="pb-2 sm:pb-3">
        <CardTitle class="text-base sm:text-lg">Search Results ({{ players.length }})</CardTitle>
      </CardHeader>
      <CardContent>
        <!-- Desktop Table View -->
        <div class="hidden md:block w-full overflow-x-auto">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead class="text-xs sm:text-sm">Player Name</TableHead>
                <TableHead class="text-xs sm:text-sm">Steam ID</TableHead>
                <TableHead class="text-xs sm:text-sm">EOS ID</TableHead>
                <TableHead class="text-xs sm:text-sm">Last Seen</TableHead>
                <TableHead class="text-xs sm:text-sm">First Seen</TableHead>
                <TableHead class="text-right text-xs sm:text-sm">Actions</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              <TableRow
                v-for="player in players"
                :key="player.steam_id || player.eos_id"
                class="cursor-pointer hover:bg-muted/50"
                @click="viewPlayerProfile(player)"
              >
                <TableCell class="font-medium text-sm sm:text-base">
                  {{ player.player_name || "Unknown" }}
                </TableCell>
                <TableCell>
                  <code class="text-xs bg-muted px-2 py-1 rounded">
                    {{ player.steam_id || "N/A" }}
                  </code>
                </TableCell>
                <TableCell>
                  <code class="text-xs bg-muted px-2 py-1 rounded">
                    {{ player.eos_id || "N/A" }}
                  </code>
                </TableCell>
                <TableCell>
                  <div v-if="player.last_seen">
                    <div class="text-xs sm:text-sm">{{ getTimeAgo(player.last_seen) }}</div>
                    <div class="text-xs text-muted-foreground">
                      {{ formatDate(player.last_seen) }}
                    </div>
                  </div>
                  <span v-else class="text-xs sm:text-sm text-muted-foreground">N/A</span>
                </TableCell>
                <TableCell>
                  <div v-if="player.first_seen">
                    <div class="text-xs sm:text-sm">{{ formatDate(player.first_seen) }}</div>
                  </div>
                  <span v-else class="text-xs sm:text-sm text-muted-foreground">N/A</span>
                </TableCell>
                <TableCell class="text-right">
                  <Button
                    size="sm"
                    variant="outline"
                    @click.stop="viewPlayerProfile(player)"
                    class="text-xs"
                  >
                    View Profile
                  </Button>
                </TableCell>
              </TableRow>
            </TableBody>
          </Table>
        </div>

        <!-- Mobile Card View -->
        <div class="md:hidden space-y-3">
          <div
            v-for="player in players"
            :key="player.steam_id || player.eos_id"
            class="border rounded-lg p-3 sm:p-4 hover:bg-muted/30 transition-colors cursor-pointer"
            @click="viewPlayerProfile(player)"
          >
            <div class="flex items-start justify-between gap-2 mb-2">
              <div class="flex-1 min-w-0">
                <div class="font-semibold text-sm sm:text-base mb-1">
                  {{ player.player_name || "Unknown" }}
                </div>
                <div class="space-y-1.5">
                  <div>
                    <span class="text-xs text-muted-foreground">Steam ID: </span>
                    <code class="text-xs bg-muted px-1.5 py-0.5 rounded">
                      {{ player.steam_id || "N/A" }}
                    </code>
                  </div>
                  <div>
                    <span class="text-xs text-muted-foreground">EOS ID: </span>
                    <code class="text-xs bg-muted px-1.5 py-0.5 rounded">
                      {{ player.eos_id || "N/A" }}
                    </code>
                  </div>
                  <div v-if="player.last_seen">
                    <span class="text-xs text-muted-foreground">Last Seen: </span>
                    <span class="text-xs sm:text-sm">{{ getTimeAgo(player.last_seen) }}</span>
                    <div class="text-xs text-muted-foreground">{{ formatDate(player.last_seen) }}</div>
                  </div>
                  <div v-if="player.first_seen">
                    <span class="text-xs text-muted-foreground">First Seen: </span>
                    <span class="text-xs sm:text-sm">{{ formatDate(player.first_seen) }}</span>
                  </div>
                </div>
              </div>
            </div>
            <div class="flex items-center justify-end gap-2 pt-2 border-t">
              <Button
                size="sm"
                variant="outline"
                @click.stop="viewPlayerProfile(player)"
                class="w-full h-8 text-xs"
              >
                View Profile
              </Button>
            </div>
          </div>
        </div>
      </CardContent>
    </Card>

    <Card v-else-if="!loading && searchQuery && players.length === 0">
      <CardContent class="py-8 sm:py-12">
        <div class="text-center text-muted-foreground">
          <p class="text-base sm:text-lg mb-2">No players found</p>
          <p class="text-xs sm:text-sm">Try searching with a different query</p>
        </div>
      </CardContent>
    </Card>

    <Card v-else-if="!searchQuery">
      <CardContent class="py-8 sm:py-12">
        <div class="text-center text-muted-foreground">
          <p class="text-base sm:text-lg mb-2">Search for Players</p>
          <p class="text-xs sm:text-sm">
            Enter a player name, Steam ID, or EOS ID above to get started
          </p>
        </div>
      </CardContent>
    </Card>
  </div>
</template>

