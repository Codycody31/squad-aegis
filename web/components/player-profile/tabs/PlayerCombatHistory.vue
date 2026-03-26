<script setup lang="ts">
import { computed, ref, onMounted, watch } from "vue";
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

interface CombatHistoryDisplayEntry extends CombatHistoryEntry {
  row_id: string;
  grouped_count: number;
  total_damage: number;
  min_damage: number;
  max_damage: number;
}

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
const burstEventTypes = new Set<CombatHistoryEntry["event_type"]>([
  "damaged",
  "damaged_by",
]);

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

function formatDate(dateString: string, includeMilliseconds = false): string {
  const date = new Date(dateString);
  if (Number.isNaN(date.getTime())) return dateString;

  return date.toLocaleString(undefined, {
    year: "numeric",
    month: "numeric",
    day: "numeric",
    hour: "numeric",
    minute: "2-digit",
    second: "2-digit",
    ...(includeMilliseconds ? { fractionalSecondDigits: 3 as const } : {}),
  });
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

function getOtherPlayerKey(entry: CombatHistoryEntry): string {
  return entry.other_steam_id || entry.other_eos_id || entry.other_name || "unknown";
}

function getSecondBucket(eventTime: string): string {
  const date = new Date(eventTime);
  if (Number.isNaN(date.getTime())) return eventTime;
  return Math.floor(date.getTime() / 1000).toString();
}

function buildBurstGroupKey(entry: CombatHistoryEntry): string {
  return [
    entry.event_type,
    entry.server_id,
    entry.chain_id || "",
    getSecondBucket(entry.event_time),
    getOtherPlayerKey(entry),
    entry.weapon || "",
    entry.teamkill ? "1" : "0",
  ].join("|");
}

function shouldMergeBurst(
  current: CombatHistoryDisplayEntry,
  next: CombatHistoryEntry
): boolean {
  if (!burstEventTypes.has(current.event_type) || !burstEventTypes.has(next.event_type)) {
    return false;
  }

  return buildBurstGroupKey(current) === buildBurstGroupKey(next);
}

const filteredEvents = computed(() => {
  if (eventTypeFilter.value === "all") return events.value;
  return events.value.filter((e) => e.event_type === eventTypeFilter.value);
});

const displayEvents = computed<CombatHistoryDisplayEntry[]>(() => {
  const grouped: CombatHistoryDisplayEntry[] = [];
  const seenEventIds = new Set<string>();

  for (const event of filteredEvents.value) {
    if (event.event_id) {
      const dedupeKey = `${event.event_type}:${event.event_id}`;
      if (seenEventIds.has(dedupeKey)) continue;
      seenEventIds.add(dedupeKey);
    }

    const previous = grouped[grouped.length - 1];
    if (previous && shouldMergeBurst(previous, event)) {
      previous.grouped_count += 1;
      previous.total_damage += event.damage;
      previous.min_damage = Math.min(previous.min_damage, event.damage);
      previous.max_damage = Math.max(previous.max_damage, event.damage);
      continue;
    }

    grouped.push({
      ...event,
      row_id:
        event.event_id ||
        `${event.event_type}:${event.event_time}:${event.server_id}:${grouped.length}`,
      grouped_count: 1,
      total_damage: event.damage,
      min_damage: event.damage,
      max_damage: event.damage,
    });
  }

  return grouped;
});

function getEventLabel(type: CombatHistoryEntry["event_type"]): string {
  switch (type) {
    case "kill":
      return "Kill";
    case "death":
      return "Death";
    case "wounded":
      return "Downed";
    case "wounded_by":
      return "Downed By";
    case "damaged":
      return "Hit";
    case "damaged_by":
      return "Hit By";
  }
}

function getEventBadgeClass(type: CombatHistoryEntry["event_type"]): string {
  switch (type) {
    case "kill":
      return "bg-green-600";
    case "death":
      return "bg-red-600";
    case "wounded":
      return "bg-orange-500";
    case "wounded_by":
      return "bg-orange-700";
    case "damaged":
      return "bg-blue-500";
    case "damaged_by":
      return "bg-blue-700";
  }
}

function getEventIcon(type: CombatHistoryEntry["event_type"]) {
  switch (type) {
    case "kill":
      return Target;
    case "death":
      return Skull;
    case "wounded":
      return Crosshair;
    case "wounded_by":
      return Heart;
    case "damaged":
      return Crosshair;
    case "damaged_by":
      return Heart;
  }
}

function getTimeMeta(entry: CombatHistoryDisplayEntry): string {
  if (entry.grouped_count === 1) return getTimeAgo(entry.event_time);
  return `${getTimeAgo(entry.event_time)} • ${entry.grouped_count} hits merged`;
}

function getDamageValue(entry: CombatHistoryDisplayEntry): number {
  return entry.grouped_count > 1 ? entry.total_damage : entry.damage;
}

function getDamageMeta(entry: CombatHistoryDisplayEntry): string {
  if (entry.grouped_count <= 1) return "";

  const min = Math.round(entry.min_damage);
  const max = Math.round(entry.max_damage);
  if (min === max) {
    return `${entry.grouped_count} hits`;
  }

  return `${entry.grouped_count} hits • ${min}-${max} each`;
}

watch(eventTypeFilter, () => {
  page.value = 1;
});

watch(
  () => props.playerId,
  (newPlayerId, oldPlayerId) => {
    if (!newPlayerId || newPlayerId === oldPlayerId) return;

    page.value = 1;
    eventTypeFilter.value = "all";
    events.value = [];
    fetchCombatHistory();
  }
);

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
        v-else-if="displayEvents.length === 0"
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
                v-for="event in displayEvents"
                :key="event.row_id"
                class="hover:bg-muted/50"
                :class="{
                  'bg-yellow-500/10': event.teamkill,
                }"
              >
                <TableCell>
                  <div class="text-sm">
                    {{ formatDate(event.event_time, event.grouped_count > 1) }}
                  </div>
                  <div class="text-xs text-muted-foreground">
                    {{ getTimeMeta(event) }}
                  </div>
                </TableCell>
                <TableCell>
                  <div class="flex items-center gap-2">
                    <Badge
                      variant="default"
                      :class="getEventBadgeClass(event.event_type)"
                    >
                      <component
                        :is="getEventIcon(event.event_type)"
                        class="h-3 w-3 mr-1"
                      />
                      {{ getEventLabel(event.event_type) }}
                    </Badge>
                    <Badge
                      v-if="event.grouped_count > 1"
                      variant="secondary"
                      class="font-mono"
                    >
                      x{{ event.grouped_count }}
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
                  <div class="text-sm">{{ Math.round(getDamageValue(event)) }}</div>
                  <div
                    v-if="event.grouped_count > 1"
                    class="text-xs text-muted-foreground"
                  >
                    {{ getDamageMeta(event) }}
                  </div>
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
            v-for="event in displayEvents"
            :key="event.row_id"
            class="border rounded-lg p-3 hover:bg-muted/30 transition-colors"
            :class="{ 'border-yellow-500/50 bg-yellow-500/5': event.teamkill }"
          >
            <div class="flex items-center justify-between mb-2">
              <div class="flex items-center gap-2">
                <Badge
                  variant="default"
                  :class="getEventBadgeClass(event.event_type)"
                >
                  <component
                    :is="getEventIcon(event.event_type)"
                    class="h-3 w-3 mr-1"
                  />
                  {{ getEventLabel(event.event_type) }}
                </Badge>
                <Badge
                  v-if="event.grouped_count > 1"
                  variant="secondary"
                  class="font-mono"
                >
                  x{{ event.grouped_count }}
                </Badge>
                <AlertTriangle
                  v-if="event.teamkill"
                  class="h-4 w-4 text-yellow-500"
                />
              </div>
              <span class="text-xs text-muted-foreground">
                {{ getTimeMeta(event) }}
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
                {{ Math.round(getDamageValue(event)) }}
                <span
                  v-if="event.grouped_count > 1"
                  class="text-xs text-muted-foreground ml-1"
                >
                  ({{ getDamageMeta(event) }})
                </span>
              </div>
              <div>
                <span class="text-muted-foreground">Server: </span>
                {{ event.server_name || "Unknown" }}
              </div>
              <div>
                <span class="text-muted-foreground">Time: </span>
                {{ formatDate(event.event_time, event.grouped_count > 1) }}
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
              :disabled="events.length < limit"
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
