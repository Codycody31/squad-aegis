<script setup lang="ts">
import { computed } from "vue";
import { Card, CardContent, CardHeader, CardTitle } from "~/components/ui/card";
import { Button } from "~/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "~/components/ui/dropdown-menu";
import { ExternalLink, Copy, ChevronDown, Check, Link2, AlertCircle } from "lucide-vue-next";
import { Badge } from "~/components/ui/badge";
import type { PlayerProfile, NameHistoryEntry } from "~/types/player";

const props = defineProps<{
  player: PlayerProfile;
}>();

const copied = ref<string | null>(null);

function copyToClipboard(text: string, field: string) {
  navigator.clipboard.writeText(text);
  copied.value = field;
  setTimeout(() => {
    copied.value = null;
  }, 2000);
}

function formatDate(dateString: string | null): string {
  if (!dateString) return "N/A";
  return new Date(dateString).toLocaleString();
}

function formatPlayTime(seconds: number): string {
  if (seconds === 0) return "N/A";
  const days = Math.floor(seconds / 86400);
  const hours = Math.floor((seconds % 86400) / 3600);
  const minutes = Math.floor((seconds % 3600) / 60);
  const parts = [];
  if (days > 0) parts.push(`${days}d`);
  if (hours > 0) parts.push(`${hours}h`);
  if (minutes > 0) parts.push(`${minutes}m`);
  return parts.join(" ") || "< 1m";
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

const hasLinkedIdentities = computed(() => {
  return (
    (props.player.all_steam_ids && props.player.all_steam_ids.length > 1) ||
    (props.player.all_eos_ids && props.player.all_eos_ids.length > 1)
  );
});
</script>

<template>
  <Card class="mb-6">
    <CardHeader class="pb-3">
      <div class="flex flex-col sm:flex-row sm:items-center gap-2">
        <CardTitle class="text-2xl">{{ player.player_name }}</CardTitle>
        <!-- Name History Dropdown -->
        <DropdownMenu v-if="player.name_history && player.name_history.length > 1">
          <DropdownMenuTrigger as-child>
            <Button variant="ghost" size="sm" class="gap-1">
              <span class="text-xs text-muted-foreground">
                {{ player.name_history.length }} aliases
              </span>
              <ChevronDown class="h-3 w-3" />
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="start" class="w-64">
            <DropdownMenuItem
              v-for="entry in player.name_history"
              :key="entry.name"
              class="flex flex-col items-start"
            >
              <span class="font-medium">{{ entry.name }}</span>
              <span class="text-xs text-muted-foreground">
                Used {{ entry.session_count }} times
              </span>
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      </div>
    </CardHeader>
    <CardContent>
      <!-- Linked Identities Notice -->
      <div
        v-if="hasLinkedIdentities"
        class="mb-4 p-3 rounded-lg bg-blue-500/10 border border-blue-500/20"
      >
        <div class="flex items-start gap-2">
          <Link2 class="h-4 w-4 text-blue-500 mt-0.5 shrink-0" />
          <div class="text-sm">
            <p class="font-medium text-blue-600 dark:text-blue-400">
              Linked Identities Detected
            </p>
            <p class="text-muted-foreground text-xs mt-1">
              This player has used multiple authentication methods. All stats are consolidated.
            </p>
            <div class="mt-2 flex flex-wrap gap-2">
              <div v-if="player.all_steam_ids && player.all_steam_ids.length > 1">
                <span class="text-xs text-muted-foreground">Steam IDs: </span>
                <Badge
                  v-for="steamId in player.all_steam_ids"
                  :key="steamId"
                  variant="secondary"
                  class="text-xs mr-1"
                >
                  {{ steamId }}
                </Badge>
              </div>
              <div v-if="player.all_eos_ids && player.all_eos_ids.length > 1">
                <span class="text-xs text-muted-foreground">EOS IDs: </span>
                <Badge
                  v-for="eosId in player.all_eos_ids"
                  :key="eosId"
                  variant="secondary"
                  class="text-xs mr-1"
                >
                  {{ eosId.slice(0, 12) }}...
                </Badge>
              </div>
            </div>
          </div>
        </div>
      </div>

      <!-- Identity Status Badge -->
      <div
        v-if="player.identity_status === 'pending'"
        class="mb-4 p-2 rounded-lg bg-yellow-500/10 border border-yellow-500/20"
      >
        <div class="flex items-center gap-2 text-xs text-yellow-600 dark:text-yellow-400">
          <AlertCircle class="h-3 w-3" />
          <span>Identity resolution pending - stats may be incomplete</span>
        </div>
      </div>

      <!-- IDs with copy buttons -->
      <div class="grid grid-cols-1 md:grid-cols-2 gap-4 mb-4">
        <div>
          <div class="text-xs text-muted-foreground mb-1">Steam ID</div>
          <div class="flex items-center gap-2">
            <code class="text-sm bg-muted px-2 py-1 rounded flex-1 truncate">
              {{ player.steam_id || "N/A" }}
            </code>
            <Button
              v-if="player.steam_id"
              variant="ghost"
              size="icon"
              class="h-8 w-8"
              @click="copyToClipboard(player.steam_id, 'steam')"
            >
              <Check v-if="copied === 'steam'" class="h-4 w-4 text-green-500" />
              <Copy v-else class="h-4 w-4" />
            </Button>
          </div>
        </div>
        <div>
          <div class="text-xs text-muted-foreground mb-1">EOS ID</div>
          <div class="flex items-center gap-2">
            <code class="text-sm bg-muted px-2 py-1 rounded flex-1 truncate">
              {{ player.eos_id || "N/A" }}
            </code>
            <Button
              v-if="player.eos_id"
              variant="ghost"
              size="icon"
              class="h-8 w-8"
              @click="copyToClipboard(player.eos_id, 'eos')"
            >
              <Check v-if="copied === 'eos'" class="h-4 w-4 text-green-500" />
              <Copy v-else class="h-4 w-4" />
            </Button>
          </div>
        </div>
      </div>

      <!-- Time info -->
      <div class="grid grid-cols-2 md:grid-cols-4 gap-4 mb-4 text-sm">
        <div>
          <div class="text-xs text-muted-foreground">First Seen</div>
          <div class="font-medium">{{ formatDate(player.first_seen) }}</div>
        </div>
        <div>
          <div class="text-xs text-muted-foreground">Last Seen</div>
          <div class="font-medium">{{ getTimeAgo(player.last_seen) }}</div>
          <div class="text-xs text-muted-foreground">
            {{ formatDate(player.last_seen) }}
          </div>
        </div>
        <div>
          <div class="text-xs text-muted-foreground">Play Time</div>
          <div class="font-medium">
            {{ formatPlayTime(player.total_play_time) }}
          </div>
        </div>
        <div>
          <div class="text-xs text-muted-foreground">Sessions</div>
          <div class="font-medium">{{ player.total_sessions }}</div>
        </div>
      </div>

      <!-- External Links -->
      <div class="pt-3 border-t">
        <div class="text-xs text-muted-foreground mb-2">External Links</div>
        <div class="flex flex-wrap gap-2">
          <a
            v-if="player.steam_id"
            :href="`https://steamcommunity.com/profiles/${player.steam_id}`"
            target="_blank"
            rel="noopener noreferrer"
            class="inline-flex items-center gap-1.5 px-3 py-1.5 text-xs bg-[#171a21] hover:bg-[#1b2838] text-white rounded-md transition-colors"
          >
            <ExternalLink class="h-3 w-3" />
            <span>Steam</span>
          </a>
          <a
            v-if="player.steam_id"
            :href="`https://www.battlemetrics.com/players?filter[search]=${player.player_name}`"
            target="_blank"
            rel="noopener noreferrer"
            class="inline-flex items-center gap-1.5 px-3 py-1.5 text-xs bg-[#f26a21] hover:bg-[#d85a15] text-white rounded-md transition-colors"
          >
            <ExternalLink class="h-3 w-3" />
            <span>BattleMetrics</span>
          </a>
          <a
            v-if="player.steam_id"
            :href="`https://communitybanlist.com/search/${player.steam_id}`"
            target="_blank"
            rel="noopener noreferrer"
            class="inline-flex items-center gap-1.5 px-3 py-1.5 text-xs bg-purple-600 hover:bg-purple-700 text-white rounded-md transition-colors"
          >
            <ExternalLink class="h-3 w-3" />
            <span>CBL</span>
          </a>
        </div>
      </div>
    </CardContent>
  </Card>
</template>
