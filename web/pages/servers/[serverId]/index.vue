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
const recentLogs = ref<any[]>([]);
const recentPlayers = ref<any[]>([]);
const serverStatus = ref<"online" | "offline" | "restarting">("offline");
const cpuUsage = ref(0);
const memoryUsage = ref(0);
const diskUsage = ref(0);
const uptime = ref("");
const mapInfo = ref({ name: "", image: "", timeRemaining: "" });
const activeTab = ref("overview");

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
    interface ServerResponse {
      message: string;
      code: number;
      data: {
        server: any;
        status: string;
        metrics?: {
          uptime?: string;
          cpu?: number;
          memory?: number;
          disk?: number;
          playerCount?: number;
          maxPlayers?: number;
          currentMap?: string;
        };
      };
    }

    const { data: responseData, error: fetchError } =
      await useFetch<ServerResponse>(
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
      serverInfo.value = serverData.server;

      // Set server status
      serverStatus.value = serverData.status as
        | "online"
        | "offline"
        | "restarting";

      // Set metrics if available
      if (serverData.metrics) {
        // Set uptime
        uptime.value = serverData.metrics.uptime || "0h 0m";

        // Set resource usage
        cpuUsage.value =
          serverData.metrics.cpu || Math.floor(Math.random() * 60) + 10;
        memoryUsage.value =
          serverData.metrics.memory || Math.floor(Math.random() * 70) + 20;
        diskUsage.value =
          serverData.metrics.disk || Math.floor(Math.random() * 50) + 30;

        // Set map info
        if (serverData.metrics.currentMap) {
          mapInfo.value = {
            name: serverData.metrics.currentMap,
            image: `/maps/${serverData.metrics.currentMap
              .toLowerCase()
              .replace(/\s+/g, "-")}.jpg`,
            timeRemaining: "45m", // This would come from the server
          };
        } else {
          mapInfo.value = {
            name: "Unknown",
            image: "",
            timeRemaining: "Unknown",
          };
        }

        // Set player count
        if (serverData.metrics.playerCount !== undefined) {
          playerCount.value = {
            current: serverData.metrics.playerCount,
            max: serverData.metrics.maxPlayers || 64,
          };
        } else {
          // Set demo player count
          playerCount.value = {
            current: Math.floor(Math.random() * 50) + 10,
            max: 64,
          };
        }
      } else {
        // Set demo metrics
        cpuUsage.value = Math.floor(Math.random() * 60) + 10;
        memoryUsage.value = Math.floor(Math.random() * 70) + 20;
        diskUsage.value = Math.floor(Math.random() * 50) + 30;

        // Calculate uptime (demo)
        const hours = Math.floor(Math.random() * 72);
        const minutes = Math.floor(Math.random() * 60);
        uptime.value = `${hours}h ${minutes}m`;

        // Set map info (demo)
        mapInfo.value = {
          name: "Goose Bay",
          image: "/maps/goose-bay.jpg",
          timeRemaining: "45m",
        };

        // Set player count (demo)
        playerCount.value = {
          current: Math.floor(Math.random() * 50) + 10,
          max: 64,
        };
      }
    }
  } catch (err: any) {
    error.value =
      err.message || "An error occurred while fetching server information";
    console.error(err);
  } finally {
    loading.value = false;
  }
}

// Fetch recent audit logs
async function fetchRecentLogs() {
  const runtimeConfig = useRuntimeConfig();
  const cookieToken = useCookie(
    runtimeConfig.public.sessionCookieName as string
  );
  const token = cookieToken.value;

  if (!token) return;

  try {
    interface AuditLogsResponse {
      message: string;
      code: number;
      data: {
        logs: any[];
        pagination: {
          total: number;
          pages: number;
          page: number;
          limit: number;
        };
      };
    }

    const { data: responseData } = await useFetch<AuditLogsResponse>(
      `${runtimeConfig.public.backendApi}/servers/${serverId}/audit-logs?limit=5`,
      {
        method: "GET",
        headers: {
          Authorization: `Bearer ${token}`,
        },
      }
    );

    if (responseData.value && responseData.value.data) {
      recentLogs.value = responseData.value.data.logs || [];
    }
  } catch (err) {
    console.error("Failed to fetch recent logs:", err);
  }
}

// Fetch recent players
async function fetchRecentPlayers() {
  const runtimeConfig = useRuntimeConfig();
  const cookieToken = useCookie(
    runtimeConfig.public.sessionCookieName as string
  );
  const token = cookieToken.value;

  if (!token) return;

  try {
    interface PlayersResponse {
      message: string;
      code: number;
      data: {
        players: any[];
        pagination?: {
          total: number;
          pages: number;
          page: number;
          limit: number;
        };
      };
    }

    const { data: responseData } = await useFetch<PlayersResponse>(
      `${runtimeConfig.public.backendApi}/servers/${serverId}/players/recent?limit=5`,
      {
        method: "GET",
        headers: {
          Authorization: `Bearer ${token}`,
        },
      }
    );

    if (responseData.value && responseData.value.data) {
      recentPlayers.value = responseData.value.data.players || [];
    } else {
      // Demo data if API doesn't return anything
      recentPlayers.value = [
        {
          id: "1",
          name: "Player1",
          squad: "Alpha",
          team: "US",
          joinTime: new Date().toISOString(),
        },
        {
          id: "2",
          name: "Player2",
          squad: "Bravo",
          team: "RU",
          joinTime: new Date().toISOString(),
        },
        {
          id: "3",
          name: "Player3",
          squad: "Charlie",
          team: "US",
          joinTime: new Date().toISOString(),
        },
        {
          id: "4",
          name: "Player4",
          squad: "Delta",
          team: "RU",
          joinTime: new Date().toISOString(),
        },
        {
          id: "5",
          name: "Player5",
          squad: "Echo",
          team: "US",
          joinTime: new Date().toISOString(),
        },
      ];
    }
  } catch (err) {
    console.error("Failed to fetch recent players:", err);
    // Demo data if API fails
    recentPlayers.value = [
      {
        id: "1",
        name: "Player1",
        squad: "Alpha",
        team: "US",
        joinTime: new Date().toISOString(),
      },
      {
        id: "2",
        name: "Player2",
        squad: "Bravo",
        team: "RU",
        joinTime: new Date().toISOString(),
      },
      {
        id: "3",
        name: "Player3",
        squad: "Charlie",
        team: "US",
        joinTime: new Date().toISOString(),
      },
      {
        id: "4",
        name: "Player4",
        squad: "Delta",
        team: "RU",
        joinTime: new Date().toISOString(),
      },
      {
        id: "5",
        name: "Player5",
        squad: "Echo",
        team: "US",
        joinTime: new Date().toISOString(),
      },
    ];
  }
}

// Format date
function formatDate(dateString: string): string {
  return new Date(dateString).toLocaleString();
}

// Get badge color based on server status
function getStatusBadgeColor(status: string): string {
  const statusColors: Record<string, string> = {
    online: "bg-green-50 text-green-700 ring-green-600/20",
    offline: "bg-red-50 text-red-700 ring-red-600/20",
    restarting: "bg-yellow-50 text-yellow-700 ring-yellow-600/20",
  };

  return statusColors[status] || "bg-gray-50 text-gray-700 ring-gray-600/20";
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

  return (
    actionColors[actionType] || "bg-gray-50 text-gray-700 ring-gray-600/20"
  );
}

// Format action type for display
function formatActionType(actionType: string): string {
  return actionType
    .split("_")
    .map((word) => word.charAt(0).toUpperCase() + word.slice(1))
    .join(" ");
}

// Server control functions
function startServer() {
  serverStatus.value = "restarting";
  setTimeout(() => {
    serverStatus.value = "online";
  }, 3000);
}

function stopServer() {
  serverStatus.value = "restarting";
  setTimeout(() => {
    serverStatus.value = "offline";
  }, 3000);
}

function restartServer() {
  serverStatus.value = "restarting";
  setTimeout(() => {
    serverStatus.value = "online";
  }, 3000);
}

// Setup initial data load
onMounted(() => {
  fetchServerInfo();
  fetchRecentLogs();
  fetchRecentPlayers();
});
</script>

<template>
  <div class="p-4">
    <div class="flex justify-between items-center mb-4">
      <h1 class="text-2xl font-bold">Server Dashboard</h1>
      <div class="flex items-center space-x-2">
        <!-- TODO: Make Dropdown for server status, showing server ping, and status for game and rcon ports-->
        <!-- <Badge 
          variant="outline" 
          :class="getStatusBadgeColor(serverStatus)"
        >
          {{ serverStatus.charAt(0).toUpperCase() + serverStatus.slice(1) }}
        </Badge> -->
        <Button @click="fetchServerInfo" :disabled="loading">
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
        <!-- <TabsList class="grid w-full grid-cols-2 md:grid-cols-4"> -->
        <TabsList>
          <TabsTrigger value="overview">Overview</TabsTrigger>
          <!-- <TabsTrigger value="performance">Performance</TabsTrigger> -->
          <TabsTrigger value="players">Players</TabsTrigger>
          <!-- <TabsTrigger value="activity">Activity</TabsTrigger> -->
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
                      serverInfo?.name || "Unknown"
                    }}</span>
                  </div>
                  <div class="flex justify-between">
                    <span class="text-sm font-medium">IP Address:</span>
                    <span class="text-sm">{{
                      serverInfo?.ip_address || "Unknown"
                    }}</span>
                  </div>
                  <div class="flex justify-between">
                    <span class="text-sm font-medium">Game Port:</span>
                    <span class="text-sm">{{
                      serverInfo?.game_port || "Unknown"
                    }}</span>
                  </div>
                  <div class="flex justify-between">
                    <span class="text-sm font-medium">RCON Port:</span>
                    <span class="text-sm">{{
                      serverInfo?.rcon_port || "Unknown"
                    }}</span>
                  </div>
                  <!-- <div class="flex justify-between">
                    <span class="text-sm font-medium">Status:</span>
                    <Badge
                      variant="outline"
                      :class="getStatusBadgeColor(serverStatus)"
                    >
                      {{
                        serverStatus.charAt(0).toUpperCase() +
                        serverStatus.slice(1)
                      }}
                    </Badge>
                  </div>
                  <div class="flex justify-between">
                    <span class="text-sm font-medium">Uptime:</span>
                    <span class="text-sm">{{ uptime }}</span>
                  </div> -->
                </div>
              </CardContent>
              <!-- <CardFooter>
                <div class="flex space-x-2 w-full">
                  <Button
                    variant="outline"
                    size="sm"
                    class="flex-1"
                    :disabled="serverStatus === 'online'"
                    @click="startServer"
                  >
                    Start
                  </Button>
                  <Button
                    variant="outline"
                    size="sm"
                    class="flex-1"
                    :disabled="serverStatus === 'offline'"
                    @click="stopServer"
                  >
                    Stop
                  </Button>
                  <Button
                    variant="outline"
                    size="sm"
                    class="flex-1"
                    :disabled="serverStatus === 'offline'"
                    @click="restartServer"
                  >
                    Restart
                  </Button>
                </div>
              </CardFooter> -->
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
                  </div>
                  <h3 class="text-lg font-medium">{{ mapInfo.name }}</h3>
                  <!-- <p class="text-sm text-muted-foreground">
                    Time Remaining: {{ mapInfo.timeRemaining }}
                  </p> -->
                </div>
              </CardContent>
              <!-- <CardFooter>
                <div class="w-full">
                  <Button variant="outline" size="sm" class="w-full">
                    Change Map
                  </Button>
                </div>
              </CardFooter> -->
            </Card>

            <!-- Player Count Card -->
            <Card>
              <CardHeader>
                <CardTitle>Player Count</CardTitle>
              </CardHeader>
              <CardContent>
                <div class="text-center">
                  <div class="text-3xl font-bold mb-2">
                    {{ playerCount.current }} / {{ playerCount.max }}
                  </div>
                  <Progress
                    :value="(playerCount.current / playerCount.max) * 100"
                    class="h-2 mb-4"
                  />
                  <div class="grid grid-cols-2 gap-2">
                    <div class="bg-blue-50 p-2 rounded-md">
                      <div class="text-sm font-medium text-blue-700">
                        Team 1
                      </div>
                      <div class="text-xl font-bold text-blue-800">
                        {{ Math.floor(playerCount.current / 2) }}
                      </div>
                    </div>
                    <div class="bg-red-50 p-2 rounded-md">
                      <div class="text-sm font-medium text-red-700">Team 2</div>
                      <div class="text-xl font-bold text-red-800">
                        {{ Math.ceil(playerCount.current / 2) }}
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

        <!-- Performance Tab -->
        <TabsContent value="performance">
          <div class="grid grid-cols-1 md:grid-cols-3 gap-4 mb-4">
            <!-- CPU Usage Card -->
            <Card>
              <CardHeader>
                <CardTitle>CPU Usage</CardTitle>
              </CardHeader>
              <CardContent>
                <div class="text-center">
                  <div class="text-3xl font-bold mb-2">{{ cpuUsage }}%</div>
                  <Progress :value="cpuUsage" class="h-2" />
                </div>
              </CardContent>
            </Card>

            <!-- Memory Usage Card -->
            <Card>
              <CardHeader>
                <CardTitle>Memory Usage</CardTitle>
              </CardHeader>
              <CardContent>
                <div class="text-center">
                  <div class="text-3xl font-bold mb-2">{{ memoryUsage }}%</div>
                  <Progress :value="memoryUsage" class="h-2" />
                </div>
              </CardContent>
            </Card>

            <!-- Disk Usage Card -->
            <Card>
              <CardHeader>
                <CardTitle>Disk Usage</CardTitle>
              </CardHeader>
              <CardContent>
                <div class="text-center">
                  <div class="text-3xl font-bold mb-2">{{ diskUsage }}%</div>
                  <Progress :value="diskUsage" class="h-2" />
                </div>
              </CardContent>
            </Card>
          </div>

          <!-- Performance History Card -->
          <Card>
            <CardHeader>
              <CardTitle>Performance History</CardTitle>
              <CardDescription>Server performance over time</CardDescription>
            </CardHeader>
            <CardContent>
              <div
                class="h-64 flex items-center justify-center bg-muted rounded-md"
              >
                <p class="text-muted-foreground">
                  Performance chart will be displayed here
                </p>
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
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>Name</TableHead>
                      <TableHead>Squad</TableHead>
                      <TableHead>Team</TableHead>
                      <TableHead>Actions</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    <TableRow v-for="player in recentPlayers" :key="player.id">
                      <TableCell>{{ player.name }}</TableCell>
                      <TableCell>{{ player.squad }}</TableCell>
                      <TableCell>{{ player.team }}</TableCell>
                      <TableCell>
                        <Button variant="outline" size="sm"> Kick </Button>
                      </TableCell>
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
                <div class="space-y-4">
                  <div>
                    <h3 class="text-sm font-medium mb-2">Team Balance</h3>
                    <div class="flex h-4 mb-2">
                      <div
                        class="bg-blue-500 rounded-l-full"
                        :style="`width: ${
                          (Math.floor(playerCount.current / 2) /
                            playerCount.current) *
                          100
                        }%`"
                      ></div>
                      <div
                        class="bg-red-500 rounded-r-full"
                        :style="`width: ${
                          (Math.ceil(playerCount.current / 2) /
                            playerCount.current) *
                          100
                        }%`"
                      ></div>
                    </div>
                    <div class="flex justify-between text-xs">
                      <span
                        >Team 1: {{ Math.floor(playerCount.current / 2) }}</span
                      >
                      <span
                        >Team 2: {{ Math.ceil(playerCount.current / 2) }}</span
                      >
                    </div>
                  </div>

                  <div>
                    <h3 class="text-sm font-medium mb-2">Squad Distribution</h3>
                    <div class="space-y-2">
                      <div
                        v-for="(squad, index) in [
                          'Alpha',
                          'Bravo',
                          'Charlie',
                          'Delta',
                          'Echo',
                        ]"
                        :key="squad"
                      >
                        <div class="flex justify-between text-xs mb-1">
                          <span>{{ squad }}</span>
                          <span
                            >{{ Math.floor(Math.random() * 5) + 1 }} / 9</span
                          >
                        </div>
                        <Progress
                          :value="
                            ((Math.floor(Math.random() * 5) + 1) / 9) * 100
                          "
                          class="h-2"
                        />
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

        <!-- Activity Tab -->
        <TabsContent value="activity">
          <div class="grid grid-cols-1 gap-4">
            <!-- Recent Activity Card -->
            <Card>
              <CardHeader>
                <CardTitle>Recent Activity</CardTitle>
                <CardDescription>Latest actions on this server</CardDescription>
              </CardHeader>
              <CardContent>
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>Time</TableHead>
                      <TableHead>User</TableHead>
                      <TableHead>Action</TableHead>
                      <TableHead>Details</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    <TableRow
                      v-for="(log, index) in recentLogs"
                      :key="log.id || index"
                    >
                      <TableCell>{{
                        formatDate(log.createdAt || log.timestamp)
                      }}</TableCell>
                      <TableCell>{{ log.username || "System" }}</TableCell>
                      <TableCell>
                        <Badge
                          variant="outline"
                          :class="
                            getActionBadgeColor(log.actionType || log.action)
                          "
                        >
                          {{ formatActionType(log.actionType || log.action) }}
                        </Badge>
                      </TableCell>
                      <TableCell>
                        <div class="max-w-xs truncate">
                          {{ log.actionDetails || JSON.stringify(log.changes) }}
                        </div>
                      </TableCell>
                    </TableRow>
                    <!-- Demo data if no logs are available -->
                    <TableRow
                      v-if="recentLogs.length === 0"
                      v-for="i in 5"
                      :key="i"
                    >
                      <TableCell>{{
                        formatDate(new Date().toISOString())
                      }}</TableCell>
                      <TableCell>{{
                        i % 2 === 0 ? "Admin" : "System"
                      }}</TableCell>
                      <TableCell>
                        <Badge
                          variant="outline"
                          :class="
                            getActionBadgeColor(
                              i % 2 === 0 ? 'rcon_command' : 'server_update'
                            )
                          "
                        >
                          {{
                            formatActionType(
                              i % 2 === 0 ? "rcon_command" : "server_update"
                            )
                          }}
                        </Badge>
                      </TableCell>
                      <TableCell>
                        <div class="max-w-xs truncate">
                          {{
                            i % 2 === 0
                              ? "Executed RCON command"
                              : "Updated server settings"
                          }}
                        </div>
                      </TableCell>
                    </TableRow>
                  </TableBody>
                </Table>
              </CardContent>
              <CardFooter>
                <NuxtLink
                  :to="`/servers/${serverId}/audit-logs`"
                  class="w-full"
                >
                  <Button variant="outline" size="sm" class="w-full">
                    View All Activity
                  </Button>
                </NuxtLink>
              </CardFooter>
            </Card>
          </div>
        </TabsContent>
      </Tabs>
    </div>
  </div>
</template>

<style scoped>
/* Add any page-specific styles here */
</style>
