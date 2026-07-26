<script setup lang="ts">
import { ref, onMounted } from "vue";
import { useAuthStore } from "@/stores/auth";

import PageHeader from "~/components/layout/PageHeader.vue";
import PlayersSearchHero from "~/components/players/PlayersSearchHero.vue";
import PlayersQuickStats from "~/components/players/PlayersQuickStats.vue";
import PlayersSearchResults from "~/components/players/PlayersSearchResults.vue";
import PlayersSuspectedAlts from "~/components/players/PlayersSuspectedAlts.vue";
import PlayersLeaderboards from "~/components/players/PlayersLeaderboards.vue";

const authStore = useAuthStore();
const runtimeConfig = useRuntimeConfig();

const loading = ref(false);
const statsLoading = ref(true);
const error = ref<string | null>(null);
const searchQuery = ref("");
const hasSearched = ref(false);
const players = ref<PlayerSearchResult[]>([]);
const stats = ref<PlayerStatsSummary | null>(null);

interface PlayerSearchResult {
  steam_id: string;
  eos_id: string;
  epic_id?: string;
  player_name: string;
  last_seen: string | null;
  first_seen: string | null;
}

interface TopPlayerStats {
  steam_id: string;
  eos_id: string;
  epic_id?: string;
  player_name: string;
  kills: number;
  deaths: number;
  kd_ratio: number;
  teamkills: number;
  revives: number;
}

interface PlayerStatsSummary {
  top_players: TopPlayerStats[];
  top_teamkillers: TopPlayerStats[];
  top_medics: TopPlayerStats[];
  most_recent_players: PlayerSearchResult[];
  total_players: number;
  total_kills: number;
  total_deaths: number;
  total_teamkills: number;
}

interface PlayersResponse {
  data: {
    players: PlayerSearchResult[];
    count: number;
  };
}

interface StatsResponse {
  data: {
    stats: PlayerStatsSummary;
  };
}

async function fetchPlayerStats() {
  statsLoading.value = true;

  try {
    const data = await useAuthFetchImperative<StatsResponse>(
      `${runtimeConfig.public.backendApi}/players/stats`
    );
    stats.value = data.data.stats;
  } catch (err: any) {
    console.error("Failed to load statistics:", err);
  } finally {
    statsLoading.value = false;
  }
}

async function searchPlayers() {
  if (!searchQuery.value.trim()) {
    error.value = "Please enter a search query";
    return;
  }

  loading.value = true;
  error.value = null;
  hasSearched.value = true;

  try {
    const data = await useAuthFetchImperative<PlayersResponse>(
      `${runtimeConfig.public.backendApi}/players?search=${encodeURIComponent(searchQuery.value)}`
    );
    players.value = data.data.players || [];
  } catch (err: any) {
    error.value = extractApiErrorMessage(err, "An error occurred while searching");
    players.value = [];
  } finally {
    loading.value = false;
  }
}

// Redirect to login if not authenticated
if (!authStore.isLoggedIn) {
  navigateTo("/login");
}

onMounted(() => {
  fetchPlayerStats();
});
</script>

<template>
  <div class="p-4 md:p-6">
    <PageHeader
      title="Players"
      description="Search player profiles, history, and suspected alt accounts."
    />

    <!-- Search Hero (Primary Focus) -->
    <PlayersSearchHero
      v-model="searchQuery"
      :loading="loading"
      @search="searchPlayers"
    />

    <!-- Quick Stats -->
    <PlayersQuickStats :stats="stats" :loading="statsLoading" />

    <!-- Error Message -->
    <div
      v-if="error"
      class="mb-4 p-3 bg-destructive/15 text-destructive rounded-md text-sm"
    >
      {{ error }}
    </div>

    <!-- Main Content: Two Column Layout -->
    <div class="grid grid-cols-1 lg:grid-cols-3 gap-4 sm:gap-6">
      <!-- Search Results (2/3 width on desktop) -->
      <div class="lg:col-span-2">
        <PlayersSearchResults
          :players="players"
          :loading="loading"
          :searched="hasSearched"
        />
      </div>

      <!-- Suspected Alt Accounts (1/3 width on desktop, super admin only) -->
      <div v-if="authStore.user?.super_admin" class="lg:col-span-1">
        <PlayersSuspectedAlts />
      </div>
    </div>

    <!-- Leaderboards (Collapsible, Secondary) -->
    <PlayersLeaderboards :stats="stats" :loading="statsLoading" />
  </div>
</template>
