<script setup lang="ts">
import { ref, onMounted, onUnmounted } from "vue";
import { Button } from "~/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "~/components/ui/card";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "~/components/ui/tabs";

const route = useRoute();
const serverId = route.params.serverId;

const loading = ref(true);
const error = ref<string | null>(null);
const teams = ref<Team[]>([]);
const refreshInterval = ref<NodeJS.Timeout | null>(null);
const activeTab = ref("team1");

interface Player {
  steamId: string;
  name: string;
  squadId: number | null;
  isSquadLeader: boolean;
  role: string;
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

// Function to fetch teams and squads data
async function fetchTeamsData() {
  loading.value = true;
  error.value = null;

  const runtimeConfig = useRuntimeConfig();
  const cookieToken = useCookie(runtimeConfig.public.sessionCookieName as string);
  const token = cookieToken.value;

  if (!token) {
    error.value = "Authentication required";
    loading.value = false;
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
      teams.value = data.value.data.teams || [];
      
      // Set active tab to first team if available
      if (teams.value.length > 0 && !activeTab.value) {
        activeTab.value = `team${teams.value[0].id}`;
      }
    }
  } catch (err: any) {
    error.value = err.message || "An error occurred while fetching teams data";
    console.error(err);
  } finally {
    loading.value = false;
  }
}

// Function to get player count in a team
function getTeamPlayerCount(team: Team): number {
  let count = team.players.length; // Unassigned players
  
  // Add players in squads
  team.squads.forEach(squad => {
    count += squad.players.length;
  });
  
  return count;
}

// Function to get squad leader name
function getSquadLeaderName(squad: Squad): string {
  if (squad.leader) {
    return squad.leader.name;
  }
  
  const leader = squad.players.find(player => player.isSquadLeader);
  return leader ? leader.name : "No Leader";
}

// Setup auto-refresh
onMounted(() => {
  fetchTeamsData();
  
  // Refresh data every 30 seconds
  refreshInterval.value = setInterval(() => {
    fetchTeamsData();
  }, 30000);
});

// Clear interval on component unmount
onUnmounted(() => {
  if (refreshInterval.value) {
    clearInterval(refreshInterval.value);
  }
});

// Manual refresh function
function refreshData() {
  fetchTeamsData();
}
</script>

<template>
  <div class="p-4">
    <div class="flex justify-between items-center mb-4">
      <h1 class="text-2xl font-bold">Teams & Squads</h1>
      <Button @click="refreshData" :disabled="loading">
        {{ loading ? "Refreshing..." : "Refresh" }}
      </Button>
    </div>

    <div v-if="error" class="bg-red-500 text-white p-4 rounded mb-4">
      {{ error }}
    </div>

    <div v-if="loading && teams.length === 0" class="text-center py-8">
      <div class="animate-spin h-8 w-8 border-4 border-primary border-t-transparent rounded-full mx-auto mb-4"></div>
      <p>Loading teams data...</p>
    </div>

    <div v-else-if="teams.length === 0" class="text-center py-8">
      <p>No teams data available</p>
    </div>

    <div v-else>
      <Tabs v-model="activeTab" class="w-full">
        <TabsList class="grid" :style="{ 'grid-template-columns': `repeat(${teams.length}, minmax(0, 1fr))` }">
          <TabsTrigger 
            v-for="team in teams" 
            :key="team.id" 
            :value="`team${team.id}`"
            class="team-tab"
          >
            {{ team.name }} ({{ getTeamPlayerCount(team) }})
          </TabsTrigger>
        </TabsList>

        <div v-for="team in teams" :key="team.id">
          <TabsContent :value="`team${team.id}`">
            <div class="mb-4">
              <div class="flex justify-between items-center mb-2">
                <h2 class="text-xl font-semibold">{{ team.name }}</h2>
              </div>

              <!-- Squads Section -->
              <div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4 mb-6">
                <Card v-for="squad in team.squads" :key="squad.id" :class="{ 'border-yellow-500': squad.locked }">
                  <CardHeader class="pb-2">
                    <CardTitle class="flex justify-between items-center">
                      <span>
                        Squad {{ squad.id }}: {{ squad.name }}
                        <span v-if="squad.locked" class="text-yellow-500 ml-2">(Locked)</span>
                      </span>
                      <span class="text-sm">{{ squad.players.length }}/9</span>
                    </CardTitle>
                    <div class="text-sm opacity-70">Leader: {{ getSquadLeaderName(squad) }}</div>
                  </CardHeader>
                  <CardContent>
                    <table class="w-full text-sm">
                      <thead>
                        <tr class="border-b border-gray-700">
                          <th class="text-left py-1">Player</th>
                          <th class="text-left py-1">Role</th>
                        </tr>
                      </thead>
                      <tbody>
                        <tr v-for="player in squad.players" :key="player.steamId" class="border-b border-gray-800">
                          <td class="py-1">
                            <div class="flex items-center">
                              <span v-if="player.isSquadLeader" class="mr-1 text-yellow-500">★</span>
                              {{ player.name }}
                            </div>
                          </td>
                          <td class="py-1">{{ player.role || 'Rifleman' }}</td>
                        </tr>
                        <tr v-if="squad.players.length === 0">
                          <td colspan="3" class="text-center py-2 opacity-70">No players in squad</td>
                        </tr>
                      </tbody>
                    </table>
                  </CardContent>
                </Card>
              </div>

              <!-- Unassigned Players Section -->
              <Card v-if="team.players.length > 0">
                <CardHeader>
                  <CardTitle>Unassigned Players ({{ team.players.length }})</CardTitle>
                </CardHeader>
                <CardContent>
                  <div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
                    <div v-for="player in team.players" :key="player.steamId" class="p-2 border border-gray-700 rounded">
                      <div class="font-medium">{{ player.name }}</div>
                      <div class="text-sm opacity-70">
                        <span>{{ player.role || 'Rifleman' }}</span>
                        <span class="float-right">K/D: {{ player.kills }}/{{ player.deaths }}</span>
                      </div>
                    </div>
                  </div>
                </CardContent>
              </Card>
            </div>
          </TabsContent>
        </div>
      </Tabs>
    </div>
  </div>
</template>

<style scoped>
.team-tab {
  font-weight: 500;
}

.team-tab[data-state="active"] {
  font-weight: 700;
}
</style>
