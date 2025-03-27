<script setup lang="ts">
import { ref, onMounted, onUnmounted, computed } from "vue";
import { Button } from "~/components/ui/button";
import { Input } from "~/components/ui/input";
import { Card, CardContent, CardHeader, CardTitle } from "~/components/ui/card";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "~/components/ui/table";
import { Badge } from "~/components/ui/badge";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "~/components/ui/select";
import { 
  Dialog, 
  DialogContent, 
  DialogDescription, 
  DialogFooter, 
  DialogHeader, 
  DialogTitle,
  DialogTrigger
} from "~/components/ui/dialog";
import { 
  DropdownMenu, 
  DropdownMenuContent, 
  DropdownMenuItem, 
  DropdownMenuTrigger 
} from "~/components/ui/dropdown-menu";
import { Textarea } from "~/components/ui/textarea";
import { useToast } from "~/components/ui/toast";
import { useForm } from "vee-validate";
import { toTypedSchema } from "@vee-validate/zod";
import { z } from "zod";

const authStore = useAuthStore();
const route = useRoute();
const serverId = route.params.serverId;
const { toast } = useToast();

const loading = ref(true);
const error = ref<string | null>(null);
const connectedPlayers = ref<Player[]>([]);
const teams = ref<Team[]>([]);
const refreshInterval = ref<NodeJS.Timeout | null>(null);
const searchQuery = ref("");
const copiedId = ref<string | null>(null);
const filterTeam = ref<string>("all");
const filterSquad = ref<string>("all");
const filterRole = ref<string>("all");

// Action dialog state
const showActionDialog = ref(false);
const actionType = ref<'kick' | 'ban' | 'warn' | 'move' | null>(null);
const selectedPlayer = ref<Player | null>(null);
const actionReason = ref("");
const actionDuration = ref(""); // For ban duration
const targetTeamId = ref<number | null>(null); // For move action
const isActionLoading = ref(false);

// Add to the interface section
interface Player {
  playerId: number;
  eosId: string;
  steamId: string;
  name: string;
  teamId: number;
  squadId: number;
  isSquadLeader: boolean;
  role: string;
}

interface PlayersResponse {
  data: {
    players: {
      onlinePlayers: Player[];
    };
    teams: Array<{
      id: number;
      name: string;
    }>;
  };
}

interface BanList {
  id: string;
  name: string;
  description: string;
  isGlobal: boolean;
}

// Get unique squads from players
const squads = computed(() => {
  const uniqueSquads = new Map();
  
  connectedPlayers.value.forEach(player => {
    if (player.squadId && !uniqueSquads.has(player.squadId)) {
      uniqueSquads.set(player.squadId, {
        id: player.squadId,
        name: `Squad ${player.squadId}`
      });
    }
  });
  
  return Array.from(uniqueSquads.values());
});

// Get unique roles from players
const roles = computed(() => {
  const uniqueRoles = new Set();
  
  connectedPlayers.value.forEach(player => {
    if (player.role) {
      uniqueRoles.add(player.role);
    }
  });
  
  return Array.from(uniqueRoles) as string[];
});

// Computed property for filtered players
const filteredPlayers = computed(() => {
  let filtered = connectedPlayers.value;
  
  // Apply search filter
  if (searchQuery.value.trim()) {
    const query = searchQuery.value.toLowerCase();
    filtered = filtered.filter(player => 
      player.name.toLowerCase().includes(query) || 
      player.steamId.includes(query) ||
      player.eosId.toLowerCase().includes(query)
    );
  }
  
  // Apply team filter
  if (filterTeam.value !== "all") {
    const teamId = parseInt(filterTeam.value);
    filtered = filtered.filter(player => player.teamId === teamId);
  }
  
  // Apply squad filter
  if (filterSquad.value !== "all") {
    if (filterSquad.value === "none") {
      filtered = filtered.filter(player => !player.squadId);
    } else {
      const squadId = parseInt(filterSquad.value);
      filtered = filtered.filter(player => player.squadId === squadId);
    }
  }
  
  // Apply role filter
  if (filterRole.value !== "all") {
    filtered = filtered.filter(player => player.role === filterRole.value);
  }
  
  return filtered;
});

// Add to the refs section
const banLists = ref<BanList[]>([]);

// Add to the form schema
const formSchema = toTypedSchema(
  z.object({
    steamId: z.string().min(17, "Steam ID must be at least 17 characters").max(17, "Steam ID must be exactly 17 characters").regex(/^\d+$/, "Steam ID must contain only numbers"),
    reason: z.string().min(1, "Reason is required"),
    duration: z.number().min(0, "Duration must be at least 0"),
    ruleId: z.string().optional(),
    banListId: z.string().optional(),
  }),
);

// Add to the form initial values
const form = useForm({
  validationSchema: formSchema,
  initialValues: {
    steamId: "",
    reason: "",
    duration: 24,
    ruleId: "",
    banListId: "",
  },
});

// Function to fetch connected players data
async function fetchConnectedPlayers() {
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
    const { data, error: fetchError } = await useFetch<PlayersResponse>(
      `${runtimeConfig.public.backendApi}/servers/${serverId}/rcon/server-population`,
      {
        method: "GET",
        headers: {
          Authorization: `Bearer ${token}`,
        },
      }
    );

    if (fetchError.value) {
      throw new Error(fetchError.value.message || "Failed to fetch connected players data");
    }

    if (data.value && data.value.data && data.value.data.players) {
      connectedPlayers.value = data.value.data.players.onlinePlayers || [];
      teams.value = data.value.data.teams || [];
      
      // Sort by team, then squad, then name
      connectedPlayers.value.sort((a, b) => {
        if (a.teamId !== b.teamId) {
          return a.teamId - b.teamId;
        }
        
        if (a.squadId !== b.squadId) {
          return (a.squadId || 999) - (b.squadId || 999);
        }
        
        return a.name.localeCompare(b.name);
      });
    }
  } catch (err: any) {
    error.value = err.message || "An error occurred while fetching connected players data";
    console.error(err);
  } finally {
    loading.value = false;
  }
}

// Get team name by ID
function getTeamName(teamId: number): string {
  const team = teams.value.find(t => t.id === teamId);
  return team ? team.name : `Team ${teamId}`;
}

// Get squad name by ID
function getSquadName(teamId: number, squadId: number | null): string {
  if (!squadId) return "Unassigned";
  const squad = teams.value.find(t => t.id === teamId)?.squads.find(s => s.id === squadId);
  return squad ? squad.name : `Squad ${squadId}`;
}

// Add function to fetch ban lists
async function fetchBanLists() {
  const runtimeConfig = useRuntimeConfig();
  const cookieToken = useCookie(runtimeConfig.public.sessionCookieName as string);
  const token = cookieToken.value;

  if (!token) return;

  try {
    const { data, error: fetchError } = await useFetch(
      `${runtimeConfig.public.backendApi}/ban-lists`,
      {
        method: "GET",
        headers: {
          Authorization: `Bearer ${token}`,
        },
      }
    );

    if (fetchError.value) {
      console.error("Failed to fetch ban lists:", fetchError.value);
      return;
    }

    if (data.value && data.value.data) {
      banLists.value = data.value.data.banLists || [];
    }
  } catch (err: any) {
    console.error("Failed to fetch ban lists:", err);
  }
}

// Add to onMounted
onMounted(() => {
  fetchConnectedPlayers();
  fetchServerRules();
  fetchBanLists();

  // Refresh data every 30 seconds
  refreshInterval.value = setInterval(() => {
    fetchConnectedPlayers();
  }, 30000);
});

// Manual refresh function
function refreshData() {
  fetchConnectedPlayers();
}

// Function to copy text to clipboard
function copyToClipboard(text: string) {
  if (process.client) {
    navigator.clipboard.writeText(text)
      .then(() => {
        copiedId.value = text;
        setTimeout(() => {
          copiedId.value = null;
        }, 2000);
      })
      .catch((err) => {
        console.error("Failed to copy text: ", err);
      });
  }
}

// Reset all filters
function resetFilters() {
  searchQuery.value = "";
  filterTeam.value = "all";
  filterSquad.value = "all";
  filterRole.value = "all";
}

// Open action dialog
function openActionDialog(player: Player, action: 'kick' | 'ban' | 'warn' | 'move') {
  selectedPlayer.value = player;
  actionType.value = action;
  actionReason.value = "";
  actionDuration.value = action === 'ban' ? "1" : "";
  showActionDialog.value = true;
}

// Close action dialog
function closeActionDialog() {
  showActionDialog.value = false;
  selectedPlayer.value = null;
  actionType.value = null;
  actionReason.value = "";
  actionDuration.value = "";
  targetTeamId.value = null;
}

// Get action title
function getActionTitle() {
  if (!actionType.value || !selectedPlayer.value) return "";
  
  const actionMap = {
    kick: "Kick",
    ban: "Ban",
    warn: "Warn",
    move: "Move"
  };
  
  return `${actionMap[actionType.value]} ${selectedPlayer.value.name}`;
}

// Execute player action
async function executePlayerAction() {
  if (!actionType.value || !selectedPlayer.value) return;
  
  isActionLoading.value = true;
  const runtimeConfig = useRuntimeConfig();
  const cookieToken = useCookie(runtimeConfig.public.sessionCookieName as string);
  const token = cookieToken.value;
  
  if (!token) {
    toast({
      title: "Authentication Error",
      description: "You must be logged in to perform this action",
      variant: "destructive"
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
          steamId: selectedPlayer.value.steamId,
          reason: actionReason.value
        };
        break;
      case 'ban':
        endpoint = `${runtimeConfig.public.backendApi}/servers/${serverId}/bans`;
        payload = {
          steamId: selectedPlayer.value.steamId,
          reason: actionReason.value,
          duration: actionDuration.value
        };
        break;
      case 'warn':
        endpoint = `${runtimeConfig.public.backendApi}/servers/${serverId}/rcon/warn-player`;
        payload = {
          steamId: selectedPlayer.value.steamId,
          message: actionReason.value
        };
        break;
      case 'move':
        endpoint = `${runtimeConfig.public.backendApi}/servers/${serverId}/rcon/move-player`;
        payload = {
          steamId: selectedPlayer.value.steamId
        };
        break;
    }
    
    const { data, error: fetchError } = await useFetch(endpoint, {
      method: "POST",
      headers: {
        Authorization: `Bearer ${token}`,
        "Content-Type": "application/json"
      },
      body: JSON.stringify(payload)
    });
    
    if (fetchError.value) {
      throw new Error(fetchError.value.message || `Failed to ${actionType.value} player`);
    }
    
    // Show success message
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
    } else {
      successMessage += actionType.value + 'ed';
    }
    
    toast({
      title: "Success",
      description: successMessage,
      variant: "default"
    });
    
    // Refresh player list
    fetchConnectedPlayers();
    
  } catch (err: any) {
    console.error(err);
    toast({
      title: "Error",
      description: err.message || `Failed to ${actionType.value} player`,
      variant: "destructive"
    });
  } finally {
    isActionLoading.value = false;
    closeActionDialog();
  }
}
</script>

<template>
  <div class="p-4">
    <div class="flex justify-between items-center mb-4">
      <h1 class="text-2xl font-bold">Connected Players</h1>
      <Button @click="refreshData" :disabled="loading">
        {{ loading ? "Refreshing..." : "Refresh" }}
      </Button>
    </div>

    <div v-if="error" class="bg-red-500 text-white p-4 rounded mb-4">
      {{ error }}
    </div>

    <Card class="mb-4">
      <CardHeader class="pb-2">
        <CardTitle>Player List</CardTitle>
        <p class="text-sm text-muted-foreground">
          View players currently connected to the server. Data refreshes automatically every 30 seconds.
        </p>
      </CardHeader>
      <CardContent>
        <div class="flex flex-col md:flex-row gap-4 mb-4">
          <div class="flex-grow">
            <Input 
              v-model="searchQuery" 
              placeholder="Search by name, Steam ID, or EOS ID..." 
              class="w-full"
            />
          </div>
          
          <div class="flex flex-wrap gap-2">
            <Select v-model="filterTeam">
              <SelectTrigger class="w-[140px]">
                <SelectValue placeholder="Team" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="all">All Teams</SelectItem>
                <SelectItem 
                  v-for="team in teams" 
                  :key="team.id" 
                  :value="team.id.toString()"
                >
                  {{ team.name }}
                </SelectItem>
              </SelectContent>
            </Select>
            
            <Select v-model="filterSquad">
              <SelectTrigger class="w-[140px]">
                <SelectValue placeholder="Squad" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="all">All Squads</SelectItem>
                <SelectItem value="none">Unassigned</SelectItem>
                <SelectItem 
                  v-for="squad in squads" 
                  :key="squad.id" 
                  :value="squad.id.toString()"
                >
                  {{ squad.name }}
                </SelectItem>
              </SelectContent>
            </Select>
            
            <Select v-model="filterRole">
              <SelectTrigger class="w-[140px]">
                <SelectValue placeholder="Role" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="all">All Roles</SelectItem>
                <SelectItem 
                  v-for="role in roles" 
                  :key="role" 
                  :value="role"
                >
                  {{ role }}
                </SelectItem>
              </SelectContent>
            </Select>
            
            <Button variant="outline" size="icon" @click="resetFilters" title="Reset Filters">
              <Icon name="lucide:x" class="h-4 w-4" />
            </Button>
          </div>
        </div>

        <div class="text-sm text-muted-foreground mb-2">
          Showing {{ filteredPlayers.length }} of {{ connectedPlayers.length }} players
        </div>

        <div v-if="loading && connectedPlayers.length === 0" class="text-center py-8">
          <div class="animate-spin h-8 w-8 border-4 border-primary border-t-transparent rounded-full mx-auto mb-4"></div>
          <p>Loading connected players...</p>
        </div>

        <div v-else-if="connectedPlayers.length === 0" class="text-center py-8">
          <p>No connected players found</p>
        </div>

        <div v-else-if="filteredPlayers.length === 0" class="text-center py-8">
          <p>No players match your filters</p>
        </div>

        <div v-else class="overflow-x-auto">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Name</TableHead>
                <TableHead>Team</TableHead>
                <TableHead>Squad</TableHead>
                <TableHead>Role</TableHead>
                <TableHead class="text-right">Actions</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              <TableRow 
                v-for="player in filteredPlayers" 
                :key="player.playerId"
                class="hover:bg-muted/50"
              >
                <TableCell class="font-medium">
                  <div class="flex items-center">
                    <span v-if="player.isSquadLeader" class="mr-1 text-yellow-500">★</span>
                    {{ player.name }}
                  </div>
                </TableCell>
                <TableCell>
                  <Badge 
                    variant="outline" 
                    :class="{ 
                      'bg-red-50 text-red-700 ring-red-600/20': player.teamId === 1,
                      'bg-blue-50 text-blue-700 ring-blue-600/20': player.teamId === 2 
                    }"
                  >
                    {{ getTeamName(player.teamId) }}
                  </Badge>
                </TableCell>
                <TableCell>
                  <Badge v-if="player.squadId" variant="outline">
                    {{ getSquadName(player.teamId, player.squadId) }}
                  </Badge>
                  <span v-else class="text-muted-foreground text-xs">Unassigned</span>
                </TableCell>
                <TableCell>{{ player.role || 'Unknown' }}</TableCell>
                <TableCell class="text-right">
                  <div class="flex items-center justify-end gap-2">
                    <Button variant="outline" size="sm" class="h-8 w-8 p-0" @click="copyToClipboard(player.steamId)">
                      <span class="sr-only">Copy Steam ID</span>
                      <Icon 
                        :name="copiedId === player.steamId ? 'lucide:check' : 'lucide:clipboard-copy'" 
                        class="h-4 w-4" 
                        :class="{ 'text-green-500': copiedId === player.steamId }"
                      />
                    </Button>
                    
                    <DropdownMenu>
                      <DropdownMenuTrigger asChild>
                        <Button variant="outline" size="sm" class="h-8 w-8 p-0">
                          <span class="sr-only">Open menu</span>
                          <Icon name="lucide:more-vertical" class="h-4 w-4" />
                        </Button>
                      </DropdownMenuTrigger>
                      <DropdownMenuContent align="end">
                        <DropdownMenuItem @click="openActionDialog(player, 'warn')" v-if="authStore.getServerPermission(serverId as string, 'warn')">
                          <Icon name="lucide:alert-triangle" class="mr-2 h-4 w-4 text-yellow-500" />
                          <span>Warn Player</span>
                        </DropdownMenuItem>
                        <DropdownMenuItem @click="openActionDialog(player, 'move')" v-if="authStore.getServerPermission(serverId as string, 'forceteamchange')">
                          <Icon name="lucide:move" class="mr-2 h-4 w-4 text-blue-500" />
                          <span>Move to Other Team</span>
                        </DropdownMenuItem>
                        <DropdownMenuItem @click="openActionDialog(player, 'kick')" v-if="authStore.getServerPermission(serverId as string, 'kick')">
                          <Icon name="lucide:log-out" class="mr-2 h-4 w-4 text-orange-500" />
                          <span>Kick Player</span>
                        </DropdownMenuItem>
                        <DropdownMenuItem @click="openActionDialog(player, 'ban')" v-if="authStore.getServerPermission(serverId as string, 'ban')">
                          <Icon name="lucide:ban" class="mr-2 h-4 w-4 text-red-500" />
                          <span>Ban Player</span>
                        </DropdownMenuItem>
                      </DropdownMenuContent>
                    </DropdownMenu>
                  </div>
                </TableCell>
              </TableRow>
            </TableBody>
          </Table>
        </div>
      </CardContent>
    </Card>

    <Card>
      <CardHeader>
        <CardTitle>Player Statistics</CardTitle>
      </CardHeader>
      <CardContent>
        <div class="grid grid-cols-1 md:grid-cols-3 gap-4">
          <div class="bg-muted/30 p-4 rounded-lg">
            <div class="text-2xl font-bold">{{ connectedPlayers.length }}</div>
            <div class="text-sm text-muted-foreground">Total Players</div>
          </div>
          
          <div class="bg-muted/30 p-4 rounded-lg">
            <div class="text-2xl font-bold">{{ teams.length }}</div>
            <div class="text-sm text-muted-foreground">Teams</div>
          </div>
          
          <div class="bg-muted/30 p-4 rounded-lg">
            <div class="text-2xl font-bold">{{ squads.length }}</div>
            <div class="text-sm text-muted-foreground">Squads</div>
          </div>
        </div>
        
        <div class="mt-4 text-sm text-muted-foreground">
          <p>This page shows players currently connected to the server. You can filter players by team, squad, role, or search for specific players.</p>
          <p class="mt-2">Squad leaders are marked with a star (★) next to their name.</p>
          <p class="mt-2">You can warn, kick, ban, or move players using the actions menu.</p>
        </div>
      </CardContent>
    </Card>
    
    <!-- Action Dialog -->
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
          </DialogDescription>
        </DialogHeader>
        
        <div class="grid gap-4 py-4">
          <div v-if="actionType === 'ban'" class="grid grid-cols-4 items-center gap-4">
            <label for="duration" class="text-right col-span-1">Duration</label>
            <Input
              id="duration"
              v-model="actionDuration"
              placeholder="7"
              class="col-span-3"
            />
            <div class="col-span-1"></div>
            <div class="text-xs text-muted-foreground col-span-3">
              Ban duration in days. Use 0 for a permanent ban.
            </div>
          </div>
        
          
          <div v-if="actionType !== 'move'" class="grid grid-cols-4 items-center gap-4">
            <label for="reason" class="text-right col-span-1">
              {{ actionType === 'warn' ? 'Message' : 'Reason' }}
            </label>
            <Textarea
              id="reason"
              v-model="actionReason"
              :placeholder="actionType === 'warn' ? 'Warning message' : 'Reason for action'"
              class="col-span-3"
              rows="3"
            />
          </div>

          <div class="grid grid-cols-4 items-center gap-4">
            <label for="banList" class="text-right col-span-1">Ban List</label>
            <Select v-model="form.values.banListId">
              <SelectTrigger class="col-span-3">
                <SelectValue placeholder="Select a ban list (optional)" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="">No Ban List</SelectItem>
                <SelectItem
                  v-for="banList in banLists"
                  :key="banList.id"
                  :value="banList.id"
                >
                  {{ banList.name }}
                  <Badge v-if="banList.isGlobal" variant="secondary" class="ml-2">Global</Badge>
                </SelectItem>
              </SelectContent>
            </Select>
          </div>
        </div>
        
        <DialogFooter>
          <Button variant="outline" @click="closeActionDialog">Cancel</Button>
          <Button 
            variant="destructive" 
            @click="executePlayerAction"
            :disabled="isActionLoading"
          >
            <span v-if="isActionLoading" class="mr-2">
              <Icon name="lucide:loader-2" class="h-4 w-4 animate-spin" />
            </span>
            Ban Player
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  </div>
</template>

<style scoped>
/* Add any page-specific styles here */
</style>
