<script setup lang="ts">
import { ref, onMounted, watch } from "vue";
import { useRoute, useRouter } from "vue-router";
import { Button } from "~/components/ui/button";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "~/components/ui/tabs";
import { useAuthStore } from "@/stores/auth";
import type { PlayerProfile } from "~/types/player";
import PageHeader from "~/components/layout/PageHeader.vue";

// Import new components
import PlayerAlertBanner from "~/components/player-profile/PlayerAlertBanner.vue";
import PlayerHeaderCard from "~/components/player-profile/PlayerHeaderCard.vue";
import PlayerRiskIndicators from "~/components/player-profile/PlayerRiskIndicators.vue";
import PlayerStatsGrid from "~/components/player-profile/PlayerStatsGrid.vue";
import PlayerChatHistory from "~/components/player-profile/tabs/PlayerChatHistory.vue";
import PlayerViolations from "~/components/player-profile/tabs/PlayerViolations.vue";
import PlayerTeamkillAnalysis from "~/components/player-profile/tabs/PlayerTeamkillAnalysis.vue";
import PlayerSessionHistory from "~/components/player-profile/tabs/PlayerSessionHistory.vue";
import PlayerNameHistory from "~/components/player-profile/tabs/PlayerNameHistory.vue";
import PlayerRelatedPlayers from "~/components/player-profile/tabs/PlayerRelatedPlayers.vue";
import PlayerStatistics from "~/components/player-profile/tabs/PlayerStatistics.vue";
import PlayerCombatHistory from "~/components/player-profile/tabs/PlayerCombatHistory.vue";

const authStore = useAuthStore();
const runtimeConfig = useRuntimeConfig();
const route = useRoute();
const router = useRouter();

const loading = ref(true);
const error = ref<string | null>(null);
const player = ref<PlayerProfile | null>(null);
const { cblData, cblLoading, fetchCBLData, reset: resetCBL } = useCBL();

interface PlayerResponse {
  data: {
    player: PlayerProfile;
  };
}

async function fetchPlayerProfile() {
  loading.value = true;
  error.value = null;
  resetCBL();

  const playerId = route.params.playerId as string;

  try {
    const data = await useAuthFetchImperative<PlayerResponse>(
      `${runtimeConfig.public.backendApi}/players/${playerId}`
    );
    player.value = data.data.player;
  } catch (err: any) {
    error.value =
      extractApiErrorMessage(err, "An error occurred while fetching player profile");
  } finally {
    loading.value = false;
  }
}

async function loadPlayerProfile() {
  player.value = null;
  await fetchPlayerProfile();

  const resolvedPlayerId =
    player.value?.steam_id || player.value?.eos_id || player.value?.epic_id;
  const routePlayerId = route.params.playerId as string;
  if (resolvedPlayerId && resolvedPlayerId !== routePlayerId) {
    await router.replace(`/players/${resolvedPlayerId}`);
    return;
  }

  if (player.value && player.value.steam_id) {
    await fetchCBLData(player.value.steam_id);
  }
}

onMounted(async () => {
  if (!authStore.isLoggedIn) {
    navigateTo("/login");
    return;
  }
  await loadPlayerProfile();
});

watch(
  () => route.params.playerId,
  async (newPlayerId, oldPlayerId) => {
    if (!authStore.isLoggedIn || !newPlayerId || newPlayerId === oldPlayerId) {
      return;
    }

    await loadPlayerProfile();
  }
);
</script>

<template>
  <div class="p-4 md:p-6">
    <PageHeader title="Player Profile">
      <template #actions>
        <Button variant="outline" @click="router.push('/players')">
          &larr; Back to Search
        </Button>
      </template>
    </PageHeader>

    <!-- Loading state -->
    <div v-if="loading" class="flex justify-center items-center py-12">
      <div class="text-center">
        <div
          class="animate-spin rounded-full h-12 w-12 border-b-2 border-primary mx-auto mb-4"
        ></div>
        <p class="text-muted-foreground">Loading player profile...</p>
      </div>
    </div>

    <!-- Error state -->
    <div
      v-else-if="error"
      class="p-4 bg-destructive/15 text-destructive rounded-md"
    >
      {{ error }}
    </div>

    <!-- Player Profile -->
    <div v-else-if="player">
      <!-- Alert Banner (active bans, critical risks) -->
      <PlayerAlertBanner
        :active-bans="player.active_bans || []"
        :risk-indicators="player.risk_indicators || []"
      />

      <!-- Header Card (identity, external links) -->
      <PlayerHeaderCard :player="player" />

      <!-- Risk Indicators -->
      <PlayerRiskIndicators
        :risk-indicators="player.risk_indicators || []"
        :violation-summary="player.violation_summary || { total_warns: 0, total_kicks: 0, total_bans: 0 }"
        :teamkill-metrics="player.teamkill_metrics || { total_teamkills: 0, teamkills_per_session: 0, teamkill_ratio: 0, recent_teamkills: 0, total_team_wounds: 0, total_team_damage: 0, recent_team_wounds: 0, recent_team_damage: 0 }"
        :cbl-data="cblData"
        :name-count="player.name_history?.length || 1"
      />

      <!-- Stats Grid -->
      <PlayerStatsGrid
        :statistics="player.statistics"
        :teamkill-metrics="player.teamkill_metrics || { total_teamkills: 0, teamkills_per_session: 0, teamkill_ratio: 0, recent_teamkills: 0, total_team_wounds: 0, total_team_damage: 0, recent_team_wounds: 0, recent_team_damage: 0 }"
        :total-sessions="player.total_sessions"
        :total-play-time="player.total_play_time"
      />

      <!-- Tabbed sections -->
      <Tabs default-value="chat" class="space-y-4">
        <TabsList class="grid grid-cols-4 lg:grid-cols-8 w-full">
          <TabsTrigger value="chat" class="text-xs sm:text-sm">Chat</TabsTrigger>
          <TabsTrigger value="combat" class="text-xs sm:text-sm"
            >Combat</TabsTrigger
          >
          <TabsTrigger value="violations" class="text-xs sm:text-sm"
            >Violations</TabsTrigger
          >
          <TabsTrigger value="teamkills" class="text-xs sm:text-sm"
            >Teamkills</TabsTrigger
          >
          <TabsTrigger value="sessions" class="text-xs sm:text-sm hidden lg:flex"
            >Sessions</TabsTrigger
          >
          <TabsTrigger value="names" class="text-xs sm:text-sm hidden lg:flex"
            >Names</TabsTrigger
          >
          <TabsTrigger value="related" class="text-xs sm:text-sm hidden lg:flex"
            >Related</TabsTrigger
          >
          <TabsTrigger value="stats" class="text-xs sm:text-sm hidden lg:flex"
            >Stats</TabsTrigger
          >
        </TabsList>

        <!-- Mobile-only additional tabs -->
        <TabsList class="grid grid-cols-4 w-full lg:hidden">
          <TabsTrigger value="sessions" class="text-xs sm:text-sm">Sessions</TabsTrigger>
          <TabsTrigger value="names" class="text-xs sm:text-sm">Names</TabsTrigger>
          <TabsTrigger value="related" class="text-xs sm:text-sm"
            >Related</TabsTrigger
          >
          <TabsTrigger value="stats" class="text-xs sm:text-sm">Stats</TabsTrigger>
        </TabsList>

        <TabsContent value="chat">
          <PlayerChatHistory
            :player-id="(route.params.playerId as string)"
            :initial-messages="player.chat_history || []"
          />
        </TabsContent>

        <TabsContent value="combat">
          <PlayerCombatHistory :player-id="(route.params.playerId as string)" />
        </TabsContent>

        <TabsContent value="violations">
          <PlayerViolations
            :violations="player.violations || []"
            :summary="player.violation_summary || { total_warns: 0, total_kicks: 0, total_bans: 0 }"
          />
        </TabsContent>

        <TabsContent value="teamkills">
          <PlayerTeamkillAnalysis
            :player-id="(route.params.playerId as string)"
            :metrics="player.teamkill_metrics || { total_teamkills: 0, teamkills_per_session: 0, teamkill_ratio: 0, recent_teamkills: 0, total_team_wounds: 0, total_team_damage: 0, recent_team_wounds: 0, recent_team_damage: 0 }"
            :weapon-stats="player.weapon_stats || []"
          />
        </TabsContent>

        <TabsContent value="sessions">
          <PlayerSessionHistory :player-id="(route.params.playerId as string)" />
        </TabsContent>

        <TabsContent value="names">
          <PlayerNameHistory
            :name-history="player.name_history || []"
            :current-name="player.player_name"
          />
        </TabsContent>

        <TabsContent value="related">
          <PlayerRelatedPlayers :player-id="(route.params.playerId as string)" />
        </TabsContent>

        <TabsContent value="stats">
          <PlayerStatistics
            :statistics="player.statistics"
            :total-sessions="player.total_sessions"
            :total-play-time="player.total_play_time"
            :recent-servers="player.recent_servers || []"
          />
        </TabsContent>
      </Tabs>
    </div>
  </div>
</template>
