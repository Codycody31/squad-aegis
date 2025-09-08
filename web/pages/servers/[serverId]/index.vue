<script setup lang="ts">
import { ref, onMounted, computed } from "vue";
import { Button } from "~/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardFooter,
  CardHeader,
  CardTitle,
} from "~/components/ui/card";
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
import { Progress } from "~/components/ui/progress";
import { useAuthStore } from "~/stores/auth";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "~/components/ui/dialog";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "~/components/ui/select";
import { toast } from "~/components/ui/toast";

const route = useRoute();
const serverId = route.params.serverId;
const authStore = useAuthStore();

// State variables
const loading = ref(true);
const error = ref<string | null>(null);
const serverInfo = ref<any>(null);
const playerCount = ref<{ current: number; max: number }>({
  current: 0,
  max: 64,
});
const activeTab = ref("overview");

const rconServerInfo = ref<any>(null);
const rconServerInfoLoading = ref(false);
const rconServerInfoError = ref<string | null>(null);

// New state variables for teams and squads
const teamsData = ref<Team[]>([]);
const loadingTeams = ref(false);
const errorTeams = ref<string | null>(null);

// State variables for map change dialog
const showMapChangeDialog = ref(false);
const availableLayers = ref<
  { name: string; mod: string; isVanilla: boolean }[]
>([]);
const selectedLayer = ref("");
const loadingLayers = ref(false);
const changingLayer = ref(false);

// Define interfaces for teams and squads data
interface Player {
  steam_id: string;
  name: string;
  squadId: number | null;
  isSquadLeader: boolean;
  role: string;
  kills?: number;
  deaths?: number;
  team?: string;
  squad?: string;
}

interface Squad {
  id: number;
  name: string;
  size: number;
  locked: boolean;
  leader: Player | null;
  players: Player[];
}

interface Team {
  id: number;
  name: string;
  squads: Squad[];
  players: Player[]; // Unassigned players
}

interface TeamsResponse {
  data: {
    teams: Team[];
  };
}

// Fetch server information
async function fetchServerInfo() {
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

  try {
    const { data: responseData, error: fetchError } = await useFetch(
      `${runtimeConfig.public.backendApi}/servers/${serverId}`,
      {
        method: "GET",
        headers: {
          Authorization: `Bearer ${token}`,
        },
      }
    );

    if (fetchError.value) {
      throw new Error(
        fetchError.value.message || "Failed to fetch server information"
      );
    }

    if (responseData.value && responseData.value.data) {
      const serverData = responseData.value.data;
      serverInfo.value = serverData;
    }
  } catch (err: any) {
    error.value =
      err.message || "An error occurred while fetching server information";
    console.error(err);
  } finally {
    loading.value = false;
  }
}

// fetch server metrics
async function fetchServerMetrics() {
  const runtimeConfig = useRuntimeConfig();
  const cookieToken = useCookie(
    runtimeConfig.public.sessionCookieName as string
  );
  const token = cookieToken.value;
  if (!token) {
    error.value = "Authentication required";
    return;
  }

  const { data: responseData, error: fetchError } = await useFetch(
    `${runtimeConfig.public.backendApi}/servers/${serverId}/metrics`,
    {
      method: "GET",
      headers: {
        Authorization: `Bearer ${token}`,
      },
    }
  );

  if (fetchError.value) {
    throw new Error(
      fetchError.value.message || "Failed to fetch server metrics"
    );
  }

  if (responseData.value && responseData.value.data) {
    serverInfo.value.metrics = responseData.value.data.metrics;
    console.log(serverInfo.value);
  }
}

async function fetchRconServerInfo() {
  rconServerInfoLoading.value = true;
  rconServerInfoError.value = null;

  const runtimeConfig = useRuntimeConfig();
  const cookieToken = useCookie(
    runtimeConfig.public.sessionCookieName as string
  );
  const token = cookieToken.value;

  if (!token) {
    error.value = "Authentication required";
    rconServerInfoLoading.value = false;
    return;
  }

  try {
    const { data: responseData, error: fetchError } = await useFetch(
      `${runtimeConfig.public.backendApi}/servers/${serverId}/rcon/server-info`,
      {
        method: "GET",
        headers: {
          Authorization: `Bearer ${token}`,
        },
      }
    );

    if (fetchError.value) {
      throw new Error(
        fetchError.value.message || "Failed to fetch rcon server information"
      );
    }

    if (responseData.value && responseData.value.data) {
      const serverData = responseData.value.data.serverInfo;
      rconServerInfo.value = serverData;
    }
  } catch (err: any) {
    rconServerInfoError.value =
      err.message || "An error occurred while fetching rcon server information";
    console.error(err);
  } finally {
    rconServerInfoLoading.value = false;
  }
}

// Fetch teams and squads data
async function fetchTeamsData() {
  loadingTeams.value = true;
  errorTeams.value = null;

  const runtimeConfig = useRuntimeConfig();
  const cookieToken = useCookie(
    runtimeConfig.public.sessionCookieName as string
  );
  const token = cookieToken.value;

  if (!token) {
    errorTeams.value = "Authentication required";
    loadingTeams.value = false;
    return;
  }

  try {
    const { data, error: fetchError } = await useFetch<TeamsResponse>(
      `${runtimeConfig.public.backendApi}/servers/${serverId}/rcon/server-population`,
      {
        method: "GET",
        headers: {
          Authorization: `Bearer ${token}`,
        },
      }
    );

    if (fetchError.value) {
      throw new Error(fetchError.value.message || "Failed to fetch teams data");
    }

    if (data.value && data.value.data) {
      teamsData.value = data.value.data.teams || [];

      // Update player count based on actual data
      if (teamsData.value.length > 0) {
        const totalPlayers = teamsData.value.reduce((total, team) => {
          return total + getTeamPlayerCount(team);
        }, 0);

        playerCount.value.current = totalPlayers;
      }
    }
  } catch (err: any) {
    errorTeams.value =
      err.message || "An error occurred while fetching teams data";
    console.error(err);
  } finally {
    loadingTeams.value = false;
  }
}

// Function to get player count in a team
function getTeamPlayerCount(team: Team): number {
  let count = team.players.length; // Unassigned players

  // Add players in squads
  team.squads.forEach((squad) => {
    count += squad.players.length;
  });

  return count;
}

// Get all connected players from teams data
const connectedPlayers = computed(() => {
  if (teamsData.value.length === 0) return [];

  const allPlayers: Player[] = [];

  teamsData.value.forEach((team) => {
    // Add unassigned players
    team.players.forEach((player) => {
      allPlayers.push({
        ...player,
        team: team.name,
        squad: "Unassigned",
      });
    });

    // Add players in squads
    team.squads.forEach((squad) => {
      squad.players.forEach((player) => {
        allPlayers.push({
          ...player,
          team: team.name,
          squad: squad.name,
        });
      });
    });
  });

  return allPlayers;
});

// Add this computed property after the connectedPlayers computed property
const formattedPlayerCount = computed(() => {
  if (!rconServerInfo.value) return null;

  // Get values from rconServerInfo
  const playerCount = rconServerInfo.value.player_count || 0;
  const maxPlayers = rconServerInfo.value.max_players || 0;
  const publicQueue = rconServerInfo.value.public_queue || 0;
  const playerReserveCount = rconServerInfo.value.player_reserve_count || 0;

  // Calculate total queue and reserved slots
  const totalQueue = publicQueue;

  // Format as: "current(+queue)/max(reserved)"
  if (totalQueue > 0) {
    return `${playerCount}(+${totalQueue})/${maxPlayers - playerReserveCount}${
      playerReserveCount !== 0 ? `(+${playerReserveCount})` : ""
    }`;
  }

  // If no queue, just show current/max(reserved)
  return `${playerCount}/${maxPlayers - playerReserveCount}${
    playerReserveCount !== 0 ? `(+${playerReserveCount})` : ""
  }`;
});

// Fetch available layers
async function fetchAvailableLayers() {
  loadingLayers.value = true;

  const runtimeConfig = useRuntimeConfig();
  const cookieToken = useCookie(
    runtimeConfig.public.sessionCookieName as string
  );
  const token = cookieToken.value;

  if (!token) {
    toast({
      title: "Error",
      description: "Authentication required",
      variant: "destructive",
    });
    loadingLayers.value = false;
    return;
  }

  try {
    interface LayersResponse {
      data: {
        layers: { name: string; mod: string; isVanilla: boolean }[];
      };
    }

    const { data: responseData, error: fetchError } =
      await useFetch<LayersResponse>(
        `${runtimeConfig.public.backendApi}/servers/${serverId}/rcon/available-layers`,
        {
          method: "GET",
          headers: {
            Authorization: `Bearer ${token}`,
          },
        }
      );

    if (fetchError.value) {
      throw new Error(
        fetchError.value.message || "Failed to fetch available layers"
      );
    }

    if (responseData.value && responseData.value.data) {
      availableLayers.value = responseData.value.data.layers || [];

      // Set current layer as default selected
      if (serverInfo.value?.metrics?.current?.map) {
        selectedLayer.value = `${serverInfo.value.metrics.current.map}`;
      }
    }
  } catch (err: any) {
    toast({
      title: "Error",
      description: err.message || "Failed to fetch available layers",
      variant: "destructive",
    });
    console.error(err);
  } finally {
    loadingLayers.value = false;
  }
}

// Change server layer
async function changeServerLayer() {
  if (!selectedLayer.value) {
    toast({
      title: "Error",
      description: "Please select a layer",
      variant: "destructive",
    });
    return;
  }

  changingLayer.value = true;

  const runtimeConfig = useRuntimeConfig();
  const cookieToken = useCookie(
    runtimeConfig.public.sessionCookieName as string
  );
  const token = cookieToken.value;

  if (!token) {
    toast({
      title: "Error",
      description: "Authentication required",
      variant: "destructive",
    });
    changingLayer.value = false;
    return;
  }

  try {
    interface LayerChangeResponse {
      data: {
        success: boolean;
        message?: string;
      };
    }

    const { data: responseData, error: fetchError } =
      await useFetch<LayerChangeResponse>(
        `${runtimeConfig.public.backendApi}/servers/${serverId}/rcon/execute`,
        {
          method: "POST",
          headers: {
            Authorization: `Bearer ${token}`,
          },
          body: {
            command: `AdminChangeLayer ${selectedLayer.value}`,
          },
        }
      );

    if (fetchError.value) {
      throw new Error(fetchError.value.message || "Failed to change layer");
    }

    toast({
      title: "Success",
      description: "Layer change initiated successfully",
    });

    // Close dialog and refresh server info
    showMapChangeDialog.value = false;
    fetchServerInfo();
  } catch (err: any) {
    toast({
      title: "Error",
      description: err.message || "Failed to change layer",
      variant: "destructive",
    });
    console.error(err);
  } finally {
    changingLayer.value = false;
  }
}

// Open layer change dialog
function openMapChangeDialog() {
  showMapChangeDialog.value = true;
  fetchAvailableLayers();
}

function refresh() {
  fetchServerInfo();
  fetchTeamsData();
  fetchServerMetrics();
  fetchRconServerInfo();
}

refresh();
</script>

<template>
  <div class="p-4">
    <div class="flex justify-between items-center mb-4">
      <h1 class="text-2xl font-bold">Server Dashboard</h1>
      <div class="flex items-center space-x-2">
        <Button @click="refresh" :disabled="loading">
          {{ loading ? "Refreshing..." : "Refresh" }}
        </Button>
      </div>
    </div>

    <div v-if="error" class="bg-red-500 text-white p-4 rounded mb-4">
      {{ error }}
    </div>

    <div v-if="loading && !serverInfo" class="text-center py-8">
      <div
        class="animate-spin h-8 w-8 border-4 border-primary border-t-transparent rounded-full mx-auto mb-4"
      ></div>
      <p>Loading server information...</p>
    </div>

    <div v-else>
      <Tabs v-model="activeTab" class="w-full">
        <TabsList>
          <TabsTrigger value="overview">Overview</TabsTrigger>
          <TabsTrigger value="players">Players</TabsTrigger>
        </TabsList>

        <!-- Overview Tab -->
        <TabsContent value="overview">
          <div class="grid grid-cols-1 md:grid-cols-3 gap-4 mb-4">
            <!-- Server Info Card -->
            <Card>
              <CardHeader>
                <CardTitle>Server Information</CardTitle>
              </CardHeader>
              <CardContent>
                <div class="space-y-2">
                  <div class="flex justify-between">
                    <span class="text-sm font-medium">Name:</span>
                    <span class="text-sm">{{
                      serverInfo?.server?.name || "Unknown"
                    }}</span>
                  </div>
                  <div class="flex justify-between">
                    <span class="text-sm font-medium">IP Address:</span>
                    <span class="text-sm">{{
                      serverInfo?.server?.ip_address || "Unknown"
                    }}</span>
                  </div>
                  <div class="flex justify-between">
                    <span class="text-sm font-medium">Game Port:</span>
                    <span class="text-sm">{{
                      serverInfo?.server?.game_port || "Unknown"
                    }}</span>
                  </div>
                  <div class="flex justify-between">
                    <span class="text-sm font-medium">RCON IP Address:</span>
                    <span class="text-sm">{{
                      serverInfo?.server?.rcon_ip_address || "Unknown"
                    }}</span>
                  </div>
                  <div class="flex justify-between">
                    <span class="text-sm font-medium">RCON Port:</span>
                    <span class="text-sm">{{
                      serverInfo?.server?.rcon_port || "Unknown"
                    }}</span>
                  </div>
                  <div class="flex justify-between">
                    <span class="text-sm font-medium">Server Version:</span>
                    <span class="text-sm">{{
                      rconServerInfo?.version || rconServerInfo?.game_version || "Unknown"
                    }}</span>
                  </div>
                  <div class="flex justify-between">
                    <span class="text-sm font-medium">License Status:</span>
                    <span class="text-sm">
                      <Badge 
                        :variant="rconServerInfo?.licensed_server ? 'default' : 'destructive'"
                        v-if="rconServerInfo?.licensed_server !== undefined"
                      >
                        {{ rconServerInfo?.licensed_server ? 'Licensed' : 'Unlicensed' }}
                      </Badge>
                      <span v-else>Unknown</span>
                    </span>
                  </div>
                </div>
              </CardContent>
            </Card>

            <!-- Current Map Card -->
            <Card>
              <CardHeader>
                <CardTitle>Current Map</CardTitle>
              </CardHeader>
              <CardContent>
                <div class="text-center">
                  <div
                    class="relative w-full h-32 bg-gray-200 rounded-md mb-2 overflow-hidden"
                  >
                    <div
                      class="absolute inset-0 flex items-center justify-center text-gray-500"
                    >
                      Map Preview
                    </div>
                    <img
                      :src="`https://raw.githubusercontent.com/mahtoid/SquadMaps/refs/heads/master/img/maps/thumbnails/${serverInfo.metrics?.current?.layer}.jpg`"
                      class="absolute inset-0 w-full h-full object-cover"
                    />
                  </div>
                  <h3 class="text-lg font-medium">
                    {{ serverInfo.metrics?.current?.map }}
                    {{
                      serverInfo.metrics?.next
                        ? `-> ${serverInfo.metrics?.next?.map}`
                        : ""
                    }}
                  </h3>
                  <p class="text-sm text-muted-foreground">
                    <template
                      v-if="
                        rconServerInfo?.match_timeout * 60 -
                          rconServerInfo?.playtime >
                        0
                      "
                    >
                      Time Remaining:
                      {{
                        Math.floor(
                          (rconServerInfo.match_timeout * 60 -
                            rconServerInfo.playtime) /
                            60
                        )
                      }}
                      minutes
                    </template>
                    <template v-else> Time Remaining: 0 minutes </template>
                  </p>
                </div>
              </CardContent>
              <CardFooter>
                <div class="w-full">
                  <Button
                    variant="outline"
                    size="sm"
                    class="w-full"
                    @click="openMapChangeDialog"
                  >
                    Change Layer
                  </Button>
                </div>
              </CardFooter>
            </Card>

            <!-- Player Count Card -->
            <Card>
              <CardHeader>
                <CardTitle>Player Count</CardTitle>
              </CardHeader>
              <CardContent>
                <div class="text-center">
                  <div class="text-3xl font-bold mb-2">
                    {{
                      formattedPlayerCount ||
                      `${serverInfo.metrics?.players?.total} / ${serverInfo.metrics?.players?.max}`
                    }}
                  </div>
                  <Progress
                    :value="
                      (serverInfo.metrics?.players?.total /
                        serverInfo.metrics?.players?.max) *
                      100
                    "
                    class="h-2 mb-4"
                  />
                  <div class="grid grid-cols-2 gap-2">
                    <div class="bg-blue-50 p-2 rounded-md">
                      <div class="text-sm font-medium text-blue-700">
                        {{ teamsData[0]?.name || "Team 1" }}
                      </div>
                      <div class="text-xl font-bold text-blue-800">
                        {{ serverInfo.metrics?.players?.teams?.[1] || 0 }}
                      </div>
                    </div>
                    <div class="bg-red-50 p-2 rounded-md">
                      <div class="text-sm font-medium text-red-700">
                        {{ teamsData[1]?.name || "Team 2" }}
                      </div>
                      <div class="text-xl font-bold text-red-800">
                        {{ serverInfo.metrics?.players?.teams?.[2] || 0 }}
                      </div>
                    </div>
                  </div>
                </div>
              </CardContent>
              <CardFooter>
                <NuxtLink
                  :to="`/servers/${serverId}/connected-players`"
                  class="w-full"
                >
                  <Button variant="outline" size="sm" class="w-full">
                    View All Players
                  </Button>
                </NuxtLink>
              </CardFooter>
            </Card>
          </div>

          <!-- Quick Actions -->
          <Card class="mb-4">
            <CardHeader>
              <CardTitle>Quick Actions</CardTitle>
              <CardDescription>Common server management tasks</CardDescription>
            </CardHeader>
            <CardContent>
              <div class="grid grid-cols-2 md:grid-cols-4 gap-4">
                <NuxtLink :to="`/servers/${serverId}/console`" class="w-full">
                  <Button
                    variant="outline"
                    class="w-full h-full flex flex-col items-center justify-center p-4"
                  >
                    <div class="text-xl mb-2">💻</div>
                    <div class="text-sm">Console</div>
                  </Button>
                </NuxtLink>
                <NuxtLink
                  :to="`/servers/${serverId}/banned-players`"
                  class="w-full"
                >
                  <Button
                    variant="outline"
                    class="w-full h-full flex flex-col items-center justify-center p-4"
                  >
                    <div class="text-xl mb-2">🚫</div>
                    <div class="text-sm">Bans</div>
                  </Button>
                </NuxtLink>
                <NuxtLink
                  :to="`/servers/${serverId}/users-and-roles`"
                  class="w-full"
                >
                  <Button
                    variant="outline"
                    class="w-full h-full flex flex-col items-center justify-center p-4"
                  >
                    <div class="text-xl mb-2">👥</div>
                    <div class="text-sm">Users & Roles</div>
                  </Button>
                </NuxtLink>
                <NuxtLink
                  :to="`/servers/${serverId}/audit-logs`"
                  class="w-full"
                >
                  <Button
                    variant="outline"
                    class="w-full h-full flex flex-col items-center justify-center p-4"
                  >
                    <div class="text-xl mb-2">📋</div>
                    <div class="text-sm">Audit Logs</div>
                  </Button>
                </NuxtLink>
              </div>
            </CardContent>
          </Card>
        </TabsContent>

        <!-- Players Tab -->
        <TabsContent value="players">
          <div class="grid grid-cols-1 md:grid-cols-2 gap-4 mb-4">
            <!-- Connected Players Card -->
            <Card>
              <CardHeader>
                <CardTitle>Connected Players</CardTitle>
                <CardDescription>Currently online players</CardDescription>
              </CardHeader>
              <CardContent>
                <div
                  v-if="loadingTeams && connectedPlayers.length === 0"
                  class="text-center py-4"
                >
                  <div
                    class="animate-spin h-6 w-6 border-4 border-primary border-t-transparent rounded-full mx-auto mb-2"
                  ></div>
                  <p class="text-sm">Loading players...</p>
                </div>
                <div v-else-if="errorTeams" class="text-red-500 text-sm py-2">
                  {{ errorTeams }}
                </div>
                <div
                  v-else-if="connectedPlayers.length === 0"
                  class="text-center py-4"
                >
                  <p class="text-sm text-muted-foreground">
                    No players connected
                  </p>
                </div>
                <Table v-else>
                  <TableHeader>
                    <TableRow>
                      <TableHead>Name</TableHead>
                      <TableHead>Squad</TableHead>
                      <TableHead>Team</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    <TableRow
                      v-for="player in connectedPlayers.slice(0, 5)"
                      :key="player.steam_id"
                    >
                      <TableCell>
                        <div class="flex items-center">
                          <span
                            v-if="player.isSquadLeader"
                            class="mr-1 text-yellow-500"
                            >★</span
                          >
                          {{ player.name }}
                        </div>
                      </TableCell>
                      <TableCell>{{ player.squad }}</TableCell>
                      <TableCell>{{ player.team }}</TableCell>
                    </TableRow>
                  </TableBody>
                </Table>
              </CardContent>
              <CardFooter>
                <NuxtLink
                  :to="`/servers/${serverId}/connected-players`"
                  class="w-full"
                >
                  <Button variant="outline" size="sm" class="w-full">
                    View All Connected Players
                  </Button>
                </NuxtLink>
              </CardFooter>
            </Card>

            <!-- Teams & Squads Card -->
            <Card>
              <CardHeader>
                <CardTitle>Teams & Squads</CardTitle>
                <CardDescription
                  >Team balance and squad distribution</CardDescription
                >
              </CardHeader>
              <CardContent>
                <div
                  v-if="loadingTeams && teamsData.length === 0"
                  class="text-center py-4"
                >
                  <div
                    class="animate-spin h-6 w-6 border-4 border-primary border-t-transparent rounded-full mx-auto mb-2"
                  ></div>
                  <p class="text-sm">Loading teams data...</p>
                </div>
                <div v-else-if="errorTeams" class="text-red-500 text-sm py-2">
                  {{ errorTeams }}
                </div>
                <div
                  v-else-if="teamsData.length === 0"
                  class="text-center py-4"
                >
                  <p class="text-sm text-muted-foreground">
                    No teams data available
                  </p>
                </div>
                <div v-else class="space-y-4">
                  <div>
                    <h3 class="text-sm font-medium mb-2">Team Balance</h3>
                    <div class="flex h-4 mb-2 bg-gray-200 rounded-full overflow-hidden">
                      <template v-if="teamsData.length >= 2">
                        <div
                          class="h-full bg-blue-500 transition-all duration-300"
                          :style="`width: ${Math.max(5, ((serverInfo.metrics?.players?.teams?.[teamsData[0]?.id] || 0) / Math.max(1, serverInfo.metrics?.players?.max || 64)) * 100)}%`"
                        ></div>
                        <div
                          class="h-full bg-red-500 transition-all duration-300"
                          :style="`width: ${Math.max(5, ((serverInfo.metrics?.players?.teams?.[teamsData[1]?.id] || 0) / Math.max(1, serverInfo.metrics?.players?.max || 64)) * 100)}%`"
                        ></div>
                      </template>
                      <template v-else>
                        <div class="h-full bg-blue-500 w-1/2"></div>
                        <div class="h-full bg-red-500 w-1/2"></div>
                      </template>
                    </div>
                    <div class="flex justify-between text-xs">
                      <span v-for="team in teamsData" :key="team.id">
                        {{ team.name }}:
                        {{ serverInfo.metrics?.players?.teams?.[team.id] || 0 }}
                      </span>
                    </div>
                  </div>

                  <div>
                    <h3 class="text-sm font-medium mb-2">Squad Distribution</h3>
                    <div class="space-y-2">
                      <div v-for="team in teamsData" :key="team.id">
                        <h4 class="text-xs font-medium mb-1">
                          {{ team.name }}
                        </h4>
                        <div
                          v-for="squad in team.squads"
                          :key="squad.id"
                          class="mb-2"
                        >
                          <div class="flex justify-between text-xs mb-1">
                            <span>{{ squad.name }}</span>
                            <span>{{ squad.players.length }} / 9</span>
                          </div>
                          <Progress
                            :value="(squad.players.length / 9) * 100"
                            class="h-2"
                          />
                        </div>
                      </div>
                    </div>
                  </div>
                </div>
              </CardContent>
              <CardFooter>
                <NuxtLink
                  :to="`/servers/${serverId}/teams-and-squads`"
                  class="w-full"
                >
                  <Button variant="outline" size="sm" class="w-full">
                    Manage Teams & Squads
                  </Button>
                </NuxtLink>
              </CardFooter>
            </Card>
          </div>
        </TabsContent>
      </Tabs>
    </div>

    <!-- Map Change Dialog -->
    <Dialog v-model:open="showMapChangeDialog">
      <DialogContent class="sm:max-w-[425px]">
        <DialogHeader>
          <DialogTitle>Change Server Layer</DialogTitle>
          <DialogDescription>
            Select a new layer to change to. This will immediately change the
            layer on the server.
          </DialogDescription>
        </DialogHeader>

        <div class="py-4">
          <div v-if="loadingLayers" class="text-center py-4">
            <div
              class="animate-spin h-6 w-6 border-4 border-primary border-t-transparent rounded-full mx-auto mb-2"
            ></div>
            <p class="text-sm">Loading available layers...</p>
          </div>
          <div
            v-else-if="availableLayers.length === 0"
            class="text-center py-4"
          >
            <p class="text-sm text-muted-foreground">No layers available</p>
          </div>
          <div v-else>
            <div class="space-y-4">
              <div class="space-y-2">
                <label for="layer-select" class="text-sm font-medium"
                  >Select Layer</label
                >
                <Select v-model="selectedLayer">
                  <SelectTrigger id="layer-select" class="w-full">
                    <SelectValue placeholder="Select a layer" />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem
                      v-for="layer in availableLayers"
                      :key="layer.name"
                      :value="layer.name"
                    >
                      {{ layer.name }}
                    </SelectItem>
                  </SelectContent>
                </Select>
              </div>
            </div>
          </div>
        </div>

        <DialogFooter>
          <Button variant="outline" @click="showMapChangeDialog = false"
            >Cancel</Button
          >
          <Button
            @click="changeServerLayer"
            :disabled="loadingLayers || changingLayer || !selectedLayer"
          >
            {{ changingLayer ? "Changing..." : "Change Layer" }}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  </div>
</template>

<style scoped>
/* Add any page-specific styles here */
</style>
