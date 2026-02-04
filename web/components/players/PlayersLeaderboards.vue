<script setup lang="ts">
import { ref } from "vue";
import { useRouter } from "vue-router";
import { Card, CardContent } from "~/components/ui/card";
import { Badge } from "~/components/ui/badge";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "~/components/ui/tabs";
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

interface PlayerSearchResult {
  steam_id: string;
  eos_id: string;
  player_name: string;
  last_seen: string | null;
}

interface PlayerStats {
  top_players: TopPlayerStats[];
  top_teamkillers: TopPlayerStats[];
  top_medics: TopPlayerStats[];
  most_recent_players: PlayerSearchResult[];
}

const props = defineProps<{
  stats: PlayerStats | null;
  loading?: boolean;
}>();

const router = useRouter();
const isOpen = ref(false);

function viewPlayer(player: TopPlayerStats | PlayerSearchResult) {
  const playerId = player.steam_id || player.eos_id;
  if (playerId) {
    router.push(`/players/${playerId}`);
  }
}

function getTimeAgo(dateString: string | null): string {
  if (!dateString) return "N/A";
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
</script>

<template>
  <Collapsible v-model:open="isOpen" class="mt-4">
    <Card>
      <CollapsibleTrigger class="w-full">
        <div class="flex items-center justify-between px-4 py-3 cursor-pointer hover:bg-muted/30 transition-colors">
          <span class="text-sm font-medium flex items-center gap-2">
            <svg
              xmlns="http://www.w3.org/2000/svg"
              class="h-4 w-4 text-primary"
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              stroke-width="2"
            >
              <path d="M8 21h8" />
              <path d="M12 17v4" />
              <path d="m18 6-2-2H8L6 6" />
              <path d="M6 9a6 6 0 0 0 12 0v-3H6Z" />
            </svg>
            Leaderboards
          </span>
          <svg
            xmlns="http://www.w3.org/2000/svg"
            class="h-4 w-4 transition-transform"
            :class="{ 'rotate-180': isOpen }"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            stroke-width="2"
          >
            <path d="m6 9 6 6 6-6" />
          </svg>
        </div>
      </CollapsibleTrigger>

      <CollapsibleContent>
        <CardContent v-if="loading" class="py-8">
          <div class="flex justify-center">
            <div class="animate-spin rounded-full h-8 w-8 border-b-2 border-primary"></div>
          </div>
          <p class="text-center text-sm text-muted-foreground mt-3">Loading leaderboards...</p>
        </CardContent>

        <CardContent v-else-if="stats">
          <Tabs default-value="top-players">
            <TabsList class="grid w-full grid-cols-2 sm:grid-cols-4">
              <TabsTrigger value="top-players" class="text-xs sm:text-sm">Top Players</TabsTrigger>
              <TabsTrigger value="top-teamkillers" class="text-xs sm:text-sm">Top TKers</TabsTrigger>
              <TabsTrigger value="top-medics" class="text-xs sm:text-sm">Top Medics</TabsTrigger>
              <TabsTrigger value="recent" class="text-xs sm:text-sm">Recent</TabsTrigger>
            </TabsList>

            <!-- Top Players Tab -->
            <TabsContent value="top-players">
              <div class="pt-4">
                <p class="text-xs text-muted-foreground mb-3">Top 10 Players by K/D Ratio (min 10 kills)</p>

                <!-- Desktop Table -->
                <div class="hidden md:block overflow-x-auto">
                  <Table>
                    <TableHeader>
                      <TableRow>
                        <TableHead class="w-12">#</TableHead>
                        <TableHead>Player</TableHead>
                        <TableHead class="text-right">K/D</TableHead>
                        <TableHead class="text-right">Kills</TableHead>
                        <TableHead class="text-right">Deaths</TableHead>
                      </TableRow>
                    </TableHeader>
                    <TableBody>
                      <TableRow
                        v-for="(player, index) in stats.top_players"
                        :key="player.steam_id || player.eos_id"
                        class="cursor-pointer hover:bg-muted/50"
                        @click="viewPlayer(player)"
                      >
                        <TableCell>
                          <Badge v-if="index === 0" variant="default" class="text-xs px-1.5">1</Badge>
                          <Badge v-else-if="index === 1" variant="secondary" class="text-xs px-1.5">2</Badge>
                          <Badge v-else-if="index === 2" variant="outline" class="text-xs px-1.5">3</Badge>
                          <span v-else class="text-xs text-muted-foreground pl-1">{{ index + 1 }}</span>
                        </TableCell>
                        <TableCell class="font-medium text-sm">{{ player.player_name }}</TableCell>
                        <TableCell class="text-right font-bold text-green-500 text-sm">{{ player.kd_ratio.toFixed(2) }}</TableCell>
                        <TableCell class="text-right text-sm">{{ player.kills }}</TableCell>
                        <TableCell class="text-right text-sm">{{ player.deaths }}</TableCell>
                      </TableRow>
                    </TableBody>
                  </Table>
                </div>

                <!-- Mobile Cards -->
                <div class="md:hidden space-y-2">
                  <div
                    v-for="(player, index) in stats.top_players"
                    :key="player.steam_id || player.eos_id"
                    class="border rounded-lg p-3 cursor-pointer hover:bg-muted/30"
                    @click="viewPlayer(player)"
                  >
                    <div class="flex items-center gap-2 mb-1">
                      <Badge v-if="index === 0" variant="default" class="text-xs px-1.5">1</Badge>
                      <Badge v-else-if="index === 1" variant="secondary" class="text-xs px-1.5">2</Badge>
                      <Badge v-else-if="index === 2" variant="outline" class="text-xs px-1.5">3</Badge>
                      <span v-else class="text-xs text-muted-foreground">{{ index + 1 }}</span>
                      <span class="font-medium text-sm">{{ player.player_name }}</span>
                    </div>
                    <div class="flex gap-4 text-xs">
                      <span>K/D: <span class="text-green-500 font-bold">{{ player.kd_ratio.toFixed(2) }}</span></span>
                      <span>Kills: {{ player.kills }}</span>
                      <span>Deaths: {{ player.deaths }}</span>
                    </div>
                  </div>
                </div>
              </div>
            </TabsContent>

            <!-- Top Teamkillers Tab -->
            <TabsContent value="top-teamkillers">
              <div class="pt-4">
                <p class="text-xs text-muted-foreground mb-3">Players with most teamkills</p>

                <div class="hidden md:block overflow-x-auto">
                  <Table>
                    <TableHeader>
                      <TableRow>
                        <TableHead class="w-12">#</TableHead>
                        <TableHead>Player</TableHead>
                        <TableHead class="text-right">Teamkills</TableHead>
                      </TableRow>
                    </TableHeader>
                    <TableBody>
                      <TableRow
                        v-for="(player, index) in stats.top_teamkillers"
                        :key="player.steam_id || player.eos_id"
                        class="cursor-pointer hover:bg-muted/50"
                        @click="viewPlayer(player)"
                      >
                        <TableCell class="text-xs text-muted-foreground">{{ index + 1 }}</TableCell>
                        <TableCell class="font-medium text-sm">{{ player.player_name }}</TableCell>
                        <TableCell class="text-right font-bold text-destructive text-sm">{{ player.teamkills }}</TableCell>
                      </TableRow>
                    </TableBody>
                  </Table>
                </div>

                <div class="md:hidden space-y-2">
                  <div
                    v-for="(player, index) in stats.top_teamkillers"
                    :key="player.steam_id || player.eos_id"
                    class="border rounded-lg p-3 cursor-pointer hover:bg-muted/30"
                    @click="viewPlayer(player)"
                  >
                    <div class="flex items-center justify-between">
                      <div class="flex items-center gap-2">
                        <span class="text-xs text-muted-foreground">{{ index + 1 }}</span>
                        <span class="font-medium text-sm">{{ player.player_name }}</span>
                      </div>
                      <span class="text-sm font-bold text-destructive">{{ player.teamkills }} TKs</span>
                    </div>
                  </div>
                </div>
              </div>
            </TabsContent>

            <!-- Top Medics Tab -->
            <TabsContent value="top-medics">
              <div class="pt-4">
                <p class="text-xs text-muted-foreground mb-3">Players with most revives</p>

                <div class="hidden md:block overflow-x-auto">
                  <Table>
                    <TableHeader>
                      <TableRow>
                        <TableHead class="w-12">#</TableHead>
                        <TableHead>Player</TableHead>
                        <TableHead class="text-right">Revives</TableHead>
                      </TableRow>
                    </TableHeader>
                    <TableBody>
                      <TableRow
                        v-for="(player, index) in stats.top_medics"
                        :key="player.steam_id || player.eos_id"
                        class="cursor-pointer hover:bg-muted/50"
                        @click="viewPlayer(player)"
                      >
                        <TableCell>
                          <Badge v-if="index === 0" variant="default" class="text-xs px-1.5">1</Badge>
                          <Badge v-else-if="index === 1" variant="secondary" class="text-xs px-1.5">2</Badge>
                          <Badge v-else-if="index === 2" variant="outline" class="text-xs px-1.5">3</Badge>
                          <span v-else class="text-xs text-muted-foreground pl-1">{{ index + 1 }}</span>
                        </TableCell>
                        <TableCell class="font-medium text-sm">{{ player.player_name }}</TableCell>
                        <TableCell class="text-right font-bold text-green-500 text-sm">{{ player.revives }}</TableCell>
                      </TableRow>
                    </TableBody>
                  </Table>
                </div>

                <div class="md:hidden space-y-2">
                  <div
                    v-for="(player, index) in stats.top_medics"
                    :key="player.steam_id || player.eos_id"
                    class="border rounded-lg p-3 cursor-pointer hover:bg-muted/30"
                    @click="viewPlayer(player)"
                  >
                    <div class="flex items-center gap-2 mb-1">
                      <Badge v-if="index === 0" variant="default" class="text-xs px-1.5">1</Badge>
                      <Badge v-else-if="index === 1" variant="secondary" class="text-xs px-1.5">2</Badge>
                      <Badge v-else-if="index === 2" variant="outline" class="text-xs px-1.5">3</Badge>
                      <span v-else class="text-xs text-muted-foreground">{{ index + 1 }}</span>
                      <span class="font-medium text-sm">{{ player.player_name }}</span>
                    </div>
                    <span class="text-xs">Revives: <span class="text-green-500 font-bold">{{ player.revives }}</span></span>
                  </div>
                </div>
              </div>
            </TabsContent>

            <!-- Recent Players Tab -->
            <TabsContent value="recent">
              <div class="pt-4">
                <p class="text-xs text-muted-foreground mb-3">Last 10 players seen on servers</p>

                <div class="hidden md:block overflow-x-auto">
                  <Table>
                    <TableHeader>
                      <TableRow>
                        <TableHead>Player</TableHead>
                        <TableHead class="text-right">Last Seen</TableHead>
                      </TableRow>
                    </TableHeader>
                    <TableBody>
                      <TableRow
                        v-for="player in stats.most_recent_players"
                        :key="player.steam_id || player.eos_id"
                        class="cursor-pointer hover:bg-muted/50"
                        @click="viewPlayer(player)"
                      >
                        <TableCell class="font-medium text-sm">{{ player.player_name }}</TableCell>
                        <TableCell class="text-right">
                          <div class="text-xs">{{ getTimeAgo(player.last_seen) }}</div>
                          <div class="text-xs text-muted-foreground">{{ formatDate(player.last_seen) }}</div>
                        </TableCell>
                      </TableRow>
                    </TableBody>
                  </Table>
                </div>

                <div class="md:hidden space-y-2">
                  <div
                    v-for="player in stats.most_recent_players"
                    :key="player.steam_id || player.eos_id"
                    class="border rounded-lg p-3 cursor-pointer hover:bg-muted/30"
                    @click="viewPlayer(player)"
                  >
                    <div class="font-medium text-sm mb-1">{{ player.player_name }}</div>
                    <div class="text-xs text-muted-foreground">
                      {{ getTimeAgo(player.last_seen) }} - {{ formatDate(player.last_seen) }}
                    </div>
                  </div>
                </div>
              </div>
            </TabsContent>
          </Tabs>
        </CardContent>
      </CollapsibleContent>
    </Card>
  </Collapsible>
</template>
