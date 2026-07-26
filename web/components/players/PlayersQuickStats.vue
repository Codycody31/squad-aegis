<script setup lang="ts">
import StatCard from "~/components/StatCard.vue";

interface PlayerStats {
  total_players: number;
  total_kills: number;
  total_deaths: number;
  total_teamkills: number;
}

defineProps<{
  stats: PlayerStats | null;
  loading?: boolean;
}>();

function formatNumber(num: number): string {
  if (num >= 1000000) {
    return (num / 1000000).toFixed(1) + "M";
  }
  if (num >= 1000) {
    return (num / 1000).toFixed(1) + "K";
  }
  return num.toLocaleString();
}
</script>

<template>
  <div v-if="loading || stats" class="mb-4 grid grid-cols-2 gap-2 md:grid-cols-4">
    <StatCard
      label="Players"
      :value="stats ? formatNumber(stats.total_players) : undefined"
      :loading="loading"
      icon="mdi:account-group"
    />
    <StatCard
      label="Kills"
      :value="stats ? formatNumber(stats.total_kills) : undefined"
      :loading="loading"
      icon="mdi:target"
    />
    <StatCard
      label="Deaths"
      :value="stats ? formatNumber(stats.total_deaths) : undefined"
      :loading="loading"
      icon="mdi:skull-outline"
    />
    <StatCard
      label="Teamkills"
      :value="stats ? formatNumber(stats.total_teamkills) : undefined"
      :loading="loading"
      icon="mdi:alert-octagon-outline"
      icon-class="bg-destructive/10 text-destructive"
    />
  </div>
</template>
