<script setup lang="ts">
import { Button } from "~/components/ui/button";
import { toast } from "~/components/ui/toast";
import type { Player } from "~/types";
import type { PlayerProfile } from "~/types/player";
import { getPlayerProfileID } from "~/composables/usePlayerAdminActions";

const props = defineProps<{
  player: Player;
  profile: PlayerProfile | null;
}>();

const steamId = computed(() => props.player.steam_id || props.profile?.steam_id || "");
const eosId = computed(
  () => props.player.eosId || props.player.eos_id || props.profile?.eos_id || ""
);

async function handleCopy(text: string) {
  const ok = await copyToClipboard(text);
  toast(
    ok
      ? { title: "Copied to clipboard" }
      : {
          title: "Copy failed",
          description: "Unable to access the clipboard in this browser.",
          variant: "destructive",
        }
  );
}

function formatPlaytime(seconds: number): string {
  const hours = Math.floor(seconds / 3600);
  if (hours >= 24) return `${Math.floor(hours / 24)}d ${hours % 24}h`;
  return `${hours}h ${Math.floor((seconds % 3600) / 60)}m`;
}

function formatDate(value: string | null | undefined): string {
  return value ? new Date(value).toLocaleDateString() : "—";
}
</script>

<template>
  <div>
    <div class="flex items-start justify-between gap-2">
      <div class="min-w-0">
        <h2 class="truncate text-lg font-semibold">
          {{ player.name || profile?.player_name || "Unknown player" }}
        </h2>
        <p v-if="profile" class="mt-0.5 text-xs text-muted-foreground">
          First seen {{ formatDate(profile.first_seen) }} · Last seen
          {{ formatDate(profile.last_seen) }} ·
          {{ formatPlaytime(profile.total_play_time) }} played over
          {{ profile.total_sessions }} sessions
        </p>
      </div>
      <NuxtLink
        :to="`/players/${getPlayerProfileID(player)}`"
        class="shrink-0 text-sm font-medium text-primary hover:underline"
      >
        Full profile →
      </NuxtLink>
    </div>

    <div class="mt-2 flex flex-wrap items-center gap-1.5">
      <button
        v-if="steamId"
        class="inline-flex items-center gap-1 rounded border bg-muted px-1.5 py-0.5 font-mono text-xs hover:bg-muted/70"
        title="Copy Steam ID"
        @click="handleCopy(steamId)"
      >
        Steam {{ steamId }}
        <Icon name="lucide:copy" class="h-3 w-3" />
      </button>
      <button
        v-if="eosId"
        class="inline-flex max-w-full items-center gap-1 rounded border bg-muted px-1.5 py-0.5 font-mono text-xs hover:bg-muted/70"
        title="Copy EOS ID"
        @click="handleCopy(eosId)"
      >
        <span class="truncate">EOS {{ eosId }}</span>
        <Icon name="lucide:copy" class="h-3 w-3 shrink-0" />
      </button>
    </div>

    <div class="mt-2 flex flex-wrap gap-2">
      <Button
        v-if="steamId"
        variant="outline"
        size="sm"
        class="h-7 text-xs"
        as-child
      >
        <a
          :href="`https://steamcommunity.com/profiles/${steamId}`"
          target="_blank"
          rel="noopener noreferrer"
        >
          Steam
          <Icon name="lucide:external-link" class="ml-1 h-3 w-3" />
        </a>
      </Button>
      <Button variant="outline" size="sm" class="h-7 text-xs" as-child>
        <a
          :href="`https://www.battlemetrics.com/players?filter[search]=${player.name}`"
          target="_blank"
          rel="noopener noreferrer"
        >
          BattleMetrics
          <Icon name="lucide:external-link" class="ml-1 h-3 w-3" />
        </a>
      </Button>
      <Button
        v-if="steamId"
        variant="outline"
        size="sm"
        class="h-7 text-xs"
        as-child
      >
        <a
          :href="`https://communitybanlist.com/search/${steamId}`"
          target="_blank"
          rel="noopener noreferrer"
        >
          CBL
          <Icon name="lucide:external-link" class="ml-1 h-3 w-3" />
        </a>
      </Button>
    </div>
  </div>
</template>
