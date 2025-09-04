<script setup lang="ts">
import { ref, onMounted, onUnmounted, computed, watch } from "vue";
import { Button } from "~/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "~/components/ui/card";
import { Input } from "~/components/ui/input";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "~/components/ui/tabs";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "~/components/ui/dialog";
import { useToast } from "~/components/ui/toast";
import PlayerActionMenu from "~/components/PlayerActionMenu.vue";
import type { Player } from "~/types";
import { Textarea } from "~/components/ui/textarea";
import { Progress } from "~/components/ui/progress";
import { Switch } from "~/components/ui/switch";
import { Label } from "~/components/ui/label";

const route = useRoute();
const serverId = route.params.serverId;
const { toast } = useToast();

const loading = ref(true);
const error = ref<string | null>(null);
const teams = ref<Team[]>([]);
const refreshInterval = ref<NodeJS.Timeout | null>(null);
const activeTab = ref("");
const searchQuery = ref("");
const isAutoRefreshEnabled = ref(true);
const lastUpdated = ref<number | null>(null);

// Action dialog state (unified with connected players)
const showActionDialog = ref(false);
const actionType = ref<'kick' | 'ban' | 'warn' | 'move' | 'remove-from-squad' | null>(null);
const selectedPlayer = ref<Player | null>(null);
const actionReason = ref("");
const actionDuration = ref(1); // For ban duration
const targetTeamId = ref<number | null>(null); // For move action
const isActionLoading = ref(false);

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
      lastUpdated.value = Date.now();
    }
  } catch (err: any) {
    error.value = err.message || "An error occurred while fetching teams data";
    console.error(err);
  } finally {
    loading.value = false;
  }
}

// Return squad players with the squad leader at the top (without mutating original array)
function getSortedSquadPlayers(squad: Squad): Player[] {
  return [...squad.players].sort((a, b) => {
    if (a.isSquadLeader && !b.isSquadLeader) return -1;
    if (!a.isSquadLeader && b.isSquadLeader) return 1;
    // Fallback secondary sort by name for stable ordering
    return a.name.localeCompare(b.name);
  });
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

// Function to get squad leader name
function getSquadLeaderName(squad: Squad): string {
  if (squad.leader) {
    return squad.leader.name;
  }

  const leader = squad.players.find((player) => player.isSquadLeader);
  return leader ? leader.name : "No Leader";
}

function sortSquadsForDisplay(squads: Squad[]): Squad[] {
  return [...squads].sort((a, b) => {
    // Unlocked first, then by size desc, then by id asc
    if (a.locked !== b.locked) return a.locked ? 1 : -1;
    if (a.players.length !== b.players.length) return b.players.length - a.players.length;
    return a.id - b.id;
  });
}

const filteredTeams = computed<Team[]>(() => {
  const query = searchQuery.value.trim().toLowerCase();
  if (!query) return teams.value;

  return teams.value.map((team) => {
    // Filter squads
    const filteredSquads = team.squads
      .map((squad) => {
        const squadNameMatches = squad.name.toLowerCase().includes(query);
        const players = squadNameMatches
          ? squad.players
          : squad.players.filter(
              (p) =>
                p.name.toLowerCase().includes(query) ||
                (p.role || "").toLowerCase().includes(query)
            );
        return { ...squad, players } as Squad;
      })
      .filter((squad) => squad.players.length > 0 || squad.name.toLowerCase().includes(query));

    // Filter unassigned players
    const filteredUnassigned = team.players.filter(
      (p) => p.name.toLowerCase().includes(query) || (p.role || "").toLowerCase().includes(query)
    );

    return {
      ...team,
      squads: sortSquadsForDisplay(filteredSquads),
      players: filteredUnassigned,
    } as Team;
  });
});

const lastUpdatedText = computed(() => {
  if (!lastUpdated.value) return "never";
  const diffMs = Date.now() - lastUpdated.value;
  const seconds = Math.floor(diffMs / 1000);
  if (seconds < 5) return "just now";
  if (seconds < 60) return `${seconds}s ago`;
  const minutes = Math.floor(seconds / 60);
  if (minutes < 60) return `${minutes}m ago`;
  const hours = Math.floor(minutes / 60);
  return `${hours}h ago`;
});

// Setup auto-refresh
onMounted(() => {
  fetchTeamsData();

  if (isAutoRefreshEnabled.value) {
    refreshInterval.value = setInterval(() => {
      fetchTeamsData();
    }, 30000);
  }
});

watch(isAutoRefreshEnabled, (enabled) => {
  if (refreshInterval.value) {
    clearInterval(refreshInterval.value);
    refreshInterval.value = null;
  }
  if (enabled) {
    refreshInterval.value = setInterval(() => {
      fetchTeamsData();
    }, 30000);
  }
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

function openActionDialog(player: Player, action: 'kick' | 'ban' | 'warn' | 'move' | 'remove-from-squad') {
  selectedPlayer.value = player;
  actionType.value = action;
  actionReason.value = "";
  actionDuration.value = action === 'ban' ? 1 : 0;
  showActionDialog.value = true;
}

function closeActionDialog() {
  showActionDialog.value = false;
  selectedPlayer.value = null;
  actionType.value = null;
  actionReason.value = "";
  actionDuration.value = 0;
  targetTeamId.value = null;
}

function getActionTitle() {
  if (!actionType.value || !selectedPlayer.value) return "";
  const actionMap = {
    kick: "Kick",
    ban: "Ban",
    warn: "Warn",
    move: "Move",
    "remove-from-squad": "Remove from Squad",
  } as const;
  return `${actionMap[actionType.value]} ${selectedPlayer.value.name}`;
}

async function executePlayerAction() {
  if (!actionType.value || !selectedPlayer.value) return;

  isActionLoading.value = true;
  const runtimeConfig = useRuntimeConfig();
  const cookieToken = useCookie(
    runtimeConfig.public.sessionCookieName as string
  );
  const token = cookieToken.value;

  if (!token) {
    toast({
      title: "Authentication Error",
      description: "You must be logged in to perform this action",
      variant: "destructive",
    });
    isActionLoading.value = false;
    closeActionDialog();
    return;
  }

  try {
    let endpoint = "";
    let payload: any = {};

    switch (actionType.value) {
      case 'kick':
        endpoint = `${runtimeConfig.public.backendApi}/servers/${serverId}/rcon/kick-player`;
        payload = {
          steam_id: selectedPlayer.value.steam_id,
          reason: actionReason.value,
        };
        break;
      case 'ban':
        endpoint = `${runtimeConfig.public.backendApi}/servers/${serverId}/bans`;
        payload = {
          steam_id: selectedPlayer.value.steam_id,
          reason: actionReason.value,
          duration: actionDuration.value,
        };
        break;
      case 'warn':
        endpoint = `${runtimeConfig.public.backendApi}/servers/${serverId}/rcon/warn-player`;
        payload = {
          steam_id: selectedPlayer.value.steam_id,
          message: actionReason.value,
        };
        break;
      case 'move':
        endpoint = `${runtimeConfig.public.backendApi}/servers/${serverId}/rcon/move-player`;
        payload = {
          steam_id: selectedPlayer.value.steam_id,
        };
        break;
      case 'remove-from-squad':
        endpoint = `${runtimeConfig.public.backendApi}/servers/${serverId}/rcon/execute`;
        payload = {
          command: `AdminRemovePlayerFromSquadById ${selectedPlayer.value.playerId}`,
        };
        break;
    }

    const { data, error: fetchError } = await useFetch(endpoint, {
      method: "POST",
      headers: {
        Authorization: `Bearer ${token}`,
        "Content-Type": "application/json",
      },
      body: JSON.stringify(payload),
    });

    if (fetchError.value) {
      throw new Error(fetchError.value.message || `Failed to ${actionType.value} player`);
    }

    let successMessage = `Player ${selectedPlayer.value.name} has been `;
    if (actionType.value === 'move') {
      successMessage += 'moved';
    } else if (actionType.value === 'ban') {
      successMessage += 'banned';
      if (actionDuration.value) {
        const days = actionDuration.value;
        successMessage += ` for ${days} ${days === 1 ? 'day' : 'days'}`;
      } else {
        successMessage += ' permanently';
      }
    } else if (actionType.value === 'remove-from-squad') {
      successMessage += 'removed from squad';
    } else {
      successMessage += actionType.value + 'ed';
    }

    toast({
      title: "Success",
      description: successMessage,
      variant: "default",
    });

    // Refresh data
    fetchTeamsData();
  } catch (err: any) {
    console.error(err);
    toast({
      title: "Error",
      description: err.message || `Failed to ${actionType.value} player`,
      variant: "destructive",
    });
  } finally {
    isActionLoading.value = false;
    closeActionDialog();
  }
}
</script>

<template>
  <div class="p-4">
    <div class="flex flex-col gap-3 md:flex-row md:items-center md:justify-between mb-4">
      <div>
        <h1 class="text-2xl font-bold">Teams & Squads</h1>
        <div class="text-xs opacity-60 mt-1">Updated {{ lastUpdatedText }}</div>
      </div>
      <div class="flex gap-2 items-center w-full md:w-auto">
        <div class="relative flex-1 md:flex-none md:w-72">
          <Icon name="lucide:search" class="absolute left-2 top-2.5 h-4 w-4 opacity-60" />
          <Input v-model="searchQuery" placeholder="Search players, roles, squads" class="pl-8" />
        </div>
        <div class="flex items-center gap-2">
          <Label for="autorefresh" class="text-sm whitespace-nowrap">Auto-refresh</Label>
          <Switch id="autorefresh" v-model:checked="isAutoRefreshEnabled" />
        </div>
        <Button @click="refreshData" :disabled="loading">
          <Icon v-if="loading" name="lucide:loader-2" class="h-4 w-4 mr-2 animate-spin" />
          {{ loading ? "Refreshing..." : "Refresh" }}
        </Button>
      </div>
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
        <TabsList class="grid" :style="{
          'grid-template-columns': `repeat(${filteredTeams.length}, minmax(0, 1fr))`,
        }">
          <TabsTrigger v-for="team in filteredTeams" :key="team.id" :value="`team${team.id}`" class="team-tab">
            {{ team.name }} ({{ getTeamPlayerCount(team) }})
          </TabsTrigger>
        </TabsList>

        <div v-for="team in filteredTeams" :key="team.id">
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
                      <span class="flex items-center gap-2">
                        <span>Squad {{ squad.id }}: {{ squad.name }}</span>
                        <Icon v-if="squad.locked" name="lucide:lock" class="h-4 w-4 text-yellow-500" />
                      </span>
                      <span class="text-sm">{{ squad.players.length }}/9</span>
                    </CardTitle>
                    <div class="text-sm opacity-70">
                      Leader: {{ getSquadLeaderName(squad) }}
                    </div>
                    <div class="mt-2">
                      <Progress :modelValue="Math.min(100, Math.round((squad.players.length / 9) * 100))" />
                    </div>
                  </CardHeader>
                  <CardContent>
                    <table class="w-full text-sm">
                      <thead>
                        <tr class="border-b border-gray-700">
                          <th class="text-left py-1">Player</th>
                          <th class="text-left py-1">Role</th>
                          <th class="text-right py-1">Actions</th>
                        </tr>
                      </thead>
                      <tbody>
                        <tr v-for="player in getSortedSquadPlayers(squad)" :key="player.steam_id" class="border-b border-gray-800">
                          <td class="py-1">
                            <div class="flex items-center">
                              <span v-if="player.isSquadLeader" class="mr-1 text-yellow-500">★</span>
                              {{ player.name }}
                            </div>
                          </td>
                          <td class="py-1">{{ player.role || "Rifleman" }}</td>
                          <td class="py-1 text-right">
                            <PlayerActionMenu :player="player" :serverId="serverId as string"
                              @warn="openActionDialog(player, 'warn')" @move="openActionDialog(player, 'move')"
                              @kick="openActionDialog(player, 'kick')" @ban="openActionDialog(player, 'ban')"
                              @remove-from-squad="openActionDialog(player, 'remove-from-squad')" />
                          </td>
                        </tr>
                        <tr v-if="squad.players.length === 0">
                          <td colspan="3" class="text-center py-2 opacity-70">
                            No players in squad
                          </td>
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
                    <div v-for="player in team.players" :key="player.steam_id" class="p-2 border border-gray-700 rounded flex items-center justify-between">
                      <div>
                        <div class="font-medium flex items-center gap-2">
                          <span class="truncate max-w-[12rem]">{{ player.name }}</span>
                        </div>
                        <div class="text-xs opacity-70">
                          <span>{{ player.role || "Rifleman" }}</span>
                        </div>
                      </div>
                      <div>
                        <PlayerActionMenu :player="player" :serverId="serverId as string"
                          @warn="openActionDialog(player, 'warn')"
                          @move="openActionDialog(player, 'move')"
                          @kick="openActionDialog(player, 'kick')"
                          @ban="openActionDialog(player, 'ban')"
                          @remove-from-squad="openActionDialog(player, 'remove-from-squad')" />
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

  <!-- Unified Action Dialog -->
  <Dialog v-model:open="showActionDialog">
    <DialogContent class="sm:max-w-[425px]">
      <DialogHeader>
        <DialogTitle>{{ getActionTitle() }}</DialogTitle>
        <DialogDescription>
          <template v-if="actionType === 'kick'">
            Kick this player from the server. They will be able to rejoin.
          </template>
          <template v-else-if="actionType === 'ban'">
            Ban this player from the server for a specified duration.
          </template>
          <template v-else-if="actionType === 'warn'">
            Send a warning message to this player.
          </template>
          <template v-else-if="actionType === 'move'">
            Force this player to switch to another team.
          </template>
          <template v-else-if="actionType === 'remove-from-squad'">
            Remove this player from their squad.
          </template>
        </DialogDescription>
      </DialogHeader>

      <div class="grid gap-4 py-4">
        <div v-if="actionType === 'ban'" class="grid grid-cols-4 items-center gap-4">
          <label for="duration" class="text-right col-span-1">Duration</label>
          <Input id="duration" v-model="actionDuration" placeholder="7" class="col-span-3" type="number" />
          <div class="col-span-1"></div>
          <div class="text-xs text-muted-foreground col-span-3">
            Ban duration in days. Use 0 for a permanent ban.
          </div>
        </div>

        <div v-if="actionType !== 'move' && actionType !== 'remove-from-squad'" class="grid grid-cols-4 items-center gap-4">
          <label for="reason" class="text-right col-span-1">
            {{ actionType === 'warn' ? 'Message' : 'Reason' }}
          </label>
          <Textarea id="reason" v-model="actionReason"
            :placeholder="actionType === 'warn' ? 'Warning message' : 'Reason for action'" class="col-span-3"
            rows="3" />
        </div>

        <div v-if="actionType === 'remove-from-squad'" class="text-sm text-muted-foreground">
          Are you sure you want to remove {{ selectedPlayer?.name }} from their squad?
        </div>
      </div>

      <DialogFooter>
        <Button variant="outline" @click="closeActionDialog">Cancel</Button>
        <Button :variant="actionType === 'warn' || actionType === 'move' ? 'default' : 'destructive'"
          @click="executePlayerAction" :disabled="isActionLoading">
          <span v-if="isActionLoading" class="mr-2">
            <Icon name="lucide:loader-2" class="h-4 w-4 animate-spin" />
          </span>
          <template v-if="actionType === 'kick'">Kick Player</template>
          <template v-else-if="actionType === 'ban'">Ban Player</template>
          <template v-else-if="actionType === 'warn'">Send Warning</template>
          <template v-else-if="actionType === 'move'">Move Player</template>
          <template v-else-if="actionType === 'remove-from-squad'">Remove from Squad</template>
        </Button>
      </DialogFooter>
    </DialogContent>
  </Dialog>
</template>

<style scoped>
.team-tab {
  font-weight: 500;
}

.team-tab[data-state="active"] {
  font-weight: 700;
}
</style>
