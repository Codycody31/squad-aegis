<script setup lang="ts">
import { useRouter } from "vue-router";
import { Card, CardContent, CardHeader, CardTitle } from "~/components/ui/card";
import { Button } from "~/components/ui/button";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "~/components/ui/table";

interface PlayerSearchResult {
  steam_id: string;
  eos_id: string;
  player_name: string;
  last_seen: string | null;
  first_seen: string | null;
}

const props = defineProps<{
  players: PlayerSearchResult[];
  loading?: boolean;
  searched?: boolean;
}>();

const router = useRouter();

function viewPlayer(player: PlayerSearchResult) {
  const playerId = player.steam_id || player.eos_id;
  if (playerId) {
    router.push(`/players/${playerId}`);
  }
}

function formatDate(dateString: string | null): string {
  if (!dateString) return "N/A";
  const date = new Date(dateString);
  return date.toLocaleString();
}

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
</script>

<template>
  <Card>
    <CardHeader class="pb-3">
      <CardTitle class="text-base sm:text-lg">
        <span v-if="players.length > 0">Search Results ({{ players.length }})</span>
        <span v-else>Search Results</span>
      </CardTitle>
    </CardHeader>
    <CardContent>
      <!-- Loading State -->
      <div v-if="loading" class="py-8">
        <div class="flex justify-center">
          <div
            class="animate-spin rounded-full h-8 w-8 border-b-2 border-primary"
          ></div>
        </div>
        <p class="text-center text-sm text-muted-foreground mt-3">
          Searching players...
        </p>
      </div>

      <!-- Results Table (Desktop) -->
      <div v-else-if="players.length > 0" class="hidden md:block w-full overflow-x-auto">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead class="text-xs sm:text-sm">Player Name</TableHead>
              <TableHead class="text-xs sm:text-sm">Steam ID</TableHead>
              <TableHead class="text-xs sm:text-sm">Last Seen</TableHead>
              <TableHead class="text-right text-xs sm:text-sm">Actions</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            <TableRow
              v-for="player in players"
              :key="player.steam_id || player.eos_id"
              class="cursor-pointer hover:bg-muted/50"
              @click="viewPlayer(player)"
            >
              <TableCell class="font-medium text-sm">
                {{ player.player_name || "Unknown" }}
              </TableCell>
              <TableCell>
                <code class="text-xs bg-muted px-2 py-1 rounded">
                  {{ player.steam_id || player.eos_id || "N/A" }}
                </code>
              </TableCell>
              <TableCell>
                <div v-if="player.last_seen">
                  <div class="text-xs sm:text-sm">{{ getTimeAgo(player.last_seen) }}</div>
                  <div class="text-xs text-muted-foreground">
                    {{ formatDate(player.last_seen) }}
                  </div>
                </div>
                <span v-else class="text-xs text-muted-foreground">N/A</span>
              </TableCell>
              <TableCell class="text-right">
                <Button
                  size="sm"
                  variant="outline"
                  @click.stop="viewPlayer(player)"
                  class="text-xs"
                >
                  View Profile
                </Button>
              </TableCell>
            </TableRow>
          </TableBody>
        </Table>
      </div>

      <!-- Results Cards (Mobile) -->
      <div v-else-if="players.length > 0" class="md:hidden space-y-3">
        <div
          v-for="player in players"
          :key="player.steam_id || player.eos_id"
          class="border rounded-lg p-3 hover:bg-muted/30 transition-colors cursor-pointer"
          @click="viewPlayer(player)"
        >
          <div class="flex items-start justify-between gap-2 mb-2">
            <div class="flex-1 min-w-0">
              <div class="font-semibold text-sm mb-1">
                {{ player.player_name || "Unknown" }}
              </div>
              <div class="space-y-1">
                <div>
                  <span class="text-xs text-muted-foreground">ID: </span>
                  <code class="text-xs bg-muted px-1.5 py-0.5 rounded">
                    {{ player.steam_id || player.eos_id || "N/A" }}
                  </code>
                </div>
                <div v-if="player.last_seen">
                  <span class="text-xs text-muted-foreground">Last Seen: </span>
                  <span class="text-xs">{{ getTimeAgo(player.last_seen) }}</span>
                </div>
              </div>
            </div>
          </div>
          <div class="flex items-center justify-end pt-2 border-t">
            <Button
              size="sm"
              variant="outline"
              @click.stop="viewPlayer(player)"
              class="w-full h-8 text-xs"
            >
              View Profile
            </Button>
          </div>
        </div>
      </div>

      <!-- Empty State - No Search Yet -->
      <div
        v-else-if="!searched"
        class="py-8 text-center text-muted-foreground"
      >
        <svg
          xmlns="http://www.w3.org/2000/svg"
          class="h-12 w-12 mx-auto mb-3 opacity-50"
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          stroke-width="1.5"
        >
          <circle cx="11" cy="11" r="8" />
          <path d="m21 21-4.3-4.3" />
        </svg>
        <p class="text-sm">Enter a search query above to find players</p>
      </div>

      <!-- Empty State - No Results -->
      <div v-else class="py-8 text-center text-muted-foreground">
        <svg
          xmlns="http://www.w3.org/2000/svg"
          class="h-12 w-12 mx-auto mb-3 opacity-50"
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          stroke-width="1.5"
        >
          <circle cx="12" cy="12" r="10" />
          <path d="m15 9-6 6" />
          <path d="m9 9 6 6" />
        </svg>
        <p class="text-sm mb-1">No players found</p>
        <p class="text-xs">Try a different search query</p>
      </div>
    </CardContent>
  </Card>
</template>
