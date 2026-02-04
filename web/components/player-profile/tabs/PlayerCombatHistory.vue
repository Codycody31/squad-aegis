<script setup lang="ts">
import { ref, onMounted, watch } from "vue";
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
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "~/components/ui/select";
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from "~/components/ui/tooltip";
import {
  Loader2,
  ChevronLeft,
  ChevronRight,
  Swords,
  Skull,
  Target,
  AlertTriangle,
  Crosshair,
  Heart,
} from "lucide-vue-next";
import type { CombatHistoryEntry } from "~/types/player";

const props = defineProps<{
  playerId: string;
}>();

const runtimeConfig = useRuntimeConfig();
const loading = ref(false);
const error = ref<string | null>(null);

const events = ref<CombatHistoryEntry[]>([]);
const page = ref(1);
const limit = ref(50);
const eventTypeFilter = ref("all");

async function fetchCombatHistory() {
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
      `${runtimeConfig.public.backendApi}/players/${props.playerId}/combat?${params}`,
      {
        headers: {
          Authorization: `Bearer ${token}`,
        },
      }
    );

    if (!response.ok) throw new Error("Failed to fetch combat history");

    const data = await response.json();
    events.value = data.data.events || [];
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

function getOtherPlayerId(entry: CombatHistoryEntry): string | null {
  return entry.other_steam_id || entry.other_eos_id || null;
}

const filteredEvents = computed(() => {
  if (eventTypeFilter.value === "all") return events.value;
  return events.value.filter((e) => e.event_type === eventTypeFilter.value);
});

watch(eventTypeFilter, () => {
  page.value = 1;
});

function nextPage() {
  page.value++;
  fetchCombatHistory();
}

function prevPage() {
  if (page.value > 1) {
    page.value--;
    fetchCombatHistory();
  }
}

onMounted(() => {
  fetchCombatHistory();
});
</script>

<template>
  <Card>
    <CardHeader class="pb-3">
      <div
        class="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-3"
      >
        <CardTitle class="text-lg flex items-center gap-2">
          <Swords class="h-5 w-5" />
          Combat History
        </CardTitle>
        <Select v-model="eventTypeFilter">
          <SelectTrigger class="w-full sm:w-40">
            <SelectValue placeholder="Type" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="all">All Events</SelectItem>
            <SelectItem value="kill">Kills</SelectItem>
            <SelectItem value="death">Deaths</SelectItem>
            <SelectItem value="wounded">Downed</SelectItem>
            <SelectItem value="wounded_by">Downed By</SelectItem>
            <SelectItem value="damaged">Damaged</SelectItem>
            <SelectItem value="damaged_by">Damaged By</SelectItem>
          </SelectContent>
        </Select>
      </div>
    </CardHeader>
    <CardContent>
      <div v-if="loading" class="flex justify-center py-8">
        <Loader2 class="h-8 w-8 animate-spin text-muted-foreground" />
      </div>

      <div v-else-if="error" class="text-center py-8 text-destructive">
        {{ error }}
      </div>

      <div
        v-else-if="filteredEvents.length === 0"
        class="text-center py-8 text-muted-foreground"
      >
        No combat history available
      </div>

      <div v-else>
        <!-- Desktop Table -->
        <div class="hidden md:block overflow-x-auto">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Time</TableHead>
                <TableHead>Event</TableHead>
                <TableHead>Player</TableHead>
                <TableHead>Weapon</TableHead>
                <TableHead>Damage</TableHead>
                <TableHead>Server</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              <TableRow
                v-for="(event, index) in filteredEvents"
                :key="index"
                class="hover:bg-muted/50"
                :class="{
                  'bg-yellow-500/10': event.teamkill,
                }"
              >
                <TableCell>
                  <div class="text-sm">{{ formatDate(event.event_time) }}</div>
                  <div class="text-xs text-muted-foreground">
                    {{ getTimeAgo(event.event_time) }}
                  </div>
                </TableCell>
                <TableCell>
                  <div class="flex items-center gap-2">
                    <Badge
                      v-if="event.event_type === 'kill'"
                      variant="default"
                      class="bg-green-600"
                    >
                      <Target class="h-3 w-3 mr-1" />
                      Kill
                    </Badge>
                    <Badge
                      v-else-if="event.event_type === 'death'"
                      variant="default"
                      class="bg-red-600"
                    >
                      <Skull class="h-3 w-3 mr-1" />
                      Death
                    </Badge>
                    <Badge
                      v-else-if="event.event_type === 'wounded'"
                      variant="default"
                      class="bg-orange-500"
                    >
                      <Crosshair class="h-3 w-3 mr-1" />
                      Downed
                    </Badge>
                    <Badge
                      v-else-if="event.event_type === 'wounded_by'"
                      variant="default"
                      class="bg-orange-700"
                    >
                      <Heart class="h-3 w-3 mr-1" />
                      Downed By
                    </Badge>
                    <Badge
                      v-else-if="event.event_type === 'damaged'"
                      variant="default"
                      class="bg-blue-500"
                    >
                      <Crosshair class="h-3 w-3 mr-1" />
                      Hit
                    </Badge>
                    <Badge
                      v-else-if="event.event_type === 'damaged_by'"
                      variant="default"
                      class="bg-blue-700"
                    >
                      <Heart class="h-3 w-3 mr-1" />
                      Hit By
                    </Badge>
                    <TooltipProvider v-if="event.teamkill">
                      <Tooltip>
                        <TooltipTrigger>
                          <AlertTriangle class="h-4 w-4 text-yellow-500" />
                        </TooltipTrigger>
                        <TooltipContent>
                          <p>Friendly Fire</p>
                        </TooltipContent>
                      </Tooltip>
                    </TooltipProvider>
                  </div>
                </TableCell>
                <TableCell>
                  <div class="text-sm font-medium">
                    <NuxtLink
                      v-if="getOtherPlayerId(event)"
                      :to="`/players/${getOtherPlayerId(event)}`"
                      class="text-primary hover:underline"
                    >
                      {{ event.other_name || "Unknown" }}
                    </NuxtLink>
                    <span v-else>{{ event.other_name || "Unknown" }}</span>
                  </div>
                  <div class="text-xs text-muted-foreground">
                    {{ event.other_team }}
                    <span v-if="event.other_squad">
                      / {{ event.other_squad }}
                    </span>
                  </div>
                </TableCell>
                <TableCell>
                  <code class="text-xs bg-muted px-2 py-0.5 rounded">
                    {{ event.weapon || "Unknown" }}
                  </code>
                </TableCell>
                <TableCell>
                  <span class="text-sm">{{ Math.round(event.damage) }}</span>
                </TableCell>
                <TableCell>
                  <div class="text-sm">
                    {{ event.server_name || "Unknown Server" }}
                  </div>
                </TableCell>
              </TableRow>
            </TableBody>
          </Table>
        </div>

        <!-- Mobile Cards -->
        <div class="md:hidden space-y-3">
          <div
            v-for="(event, index) in filteredEvents"
            :key="index"
            class="border rounded-lg p-3 hover:bg-muted/30 transition-colors"
            :class="{ 'border-yellow-500/50 bg-yellow-500/5': event.teamkill }"
          >
            <div class="flex items-center justify-between mb-2">
              <div class="flex items-center gap-2">
                <Badge
                  v-if="event.event_type === 'kill'"
                  variant="default"
                  class="bg-green-600"
                >
                  <Target class="h-3 w-3 mr-1" />
                  Kill
                </Badge>
                <Badge
                  v-else-if="event.event_type === 'death'"
                  variant="default"
                  class="bg-red-600"
                >
                  <Skull class="h-3 w-3 mr-1" />
                  Death
                </Badge>
                <Badge
                  v-else-if="event.event_type === 'wounded'"
                  variant="default"
                  class="bg-orange-500"
                >
                  <Crosshair class="h-3 w-3 mr-1" />
                  Downed
                </Badge>
                <Badge
                  v-else-if="event.event_type === 'wounded_by'"
                  variant="default"
                  class="bg-orange-700"
                >
                  <Heart class="h-3 w-3 mr-1" />
                  Downed By
                </Badge>
                <Badge
                  v-else-if="event.event_type === 'damaged'"
                  variant="default"
                  class="bg-blue-500"
                >
                  <Crosshair class="h-3 w-3 mr-1" />
                  Hit
                </Badge>
                <Badge
                  v-else-if="event.event_type === 'damaged_by'"
                  variant="default"
                  class="bg-blue-700"
                >
                  <Heart class="h-3 w-3 mr-1" />
                  Hit By
                </Badge>
                <AlertTriangle
                  v-if="event.teamkill"
                  class="h-4 w-4 text-yellow-500"
                />
              </div>
              <span class="text-xs text-muted-foreground">
                {{ getTimeAgo(event.event_time) }}
              </span>
            </div>
            <div class="space-y-1 text-sm">
              <div>
                <span class="text-muted-foreground">
                  {{ ['kill', 'wounded', 'damaged'].includes(event.event_type) ? "Victim" : "Attacker" }}:
                </span>
                <NuxtLink
                  v-if="getOtherPlayerId(event)"
                  :to="`/players/${getOtherPlayerId(event)}`"
                  class="text-primary hover:underline ml-1"
                >
                  {{ event.other_name || "Unknown" }}
                </NuxtLink>
                <span v-else class="ml-1">{{
                  event.other_name || "Unknown"
                }}</span>
              </div>
              <div>
                <span class="text-muted-foreground">Weapon: </span>
                <code class="text-xs bg-muted px-1 rounded">{{
                  event.weapon || "Unknown"
                }}</code>
              </div>
              <div>
                <span class="text-muted-foreground">Damage: </span>
                {{ Math.round(event.damage) }}
              </div>
              <div>
                <span class="text-muted-foreground">Server: </span>
                {{ event.server_name || "Unknown" }}
              </div>
              <div>
                <span class="text-muted-foreground">Time: </span>
                {{ formatDate(event.event_time) }}
              </div>
            </div>
          </div>
        </div>

        <!-- Pagination -->
        <div class="flex items-center justify-between mt-4 pt-4 border-t">
          <div class="text-sm text-muted-foreground">Page {{ page }}</div>
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
              :disabled="filteredEvents.length < limit"
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
