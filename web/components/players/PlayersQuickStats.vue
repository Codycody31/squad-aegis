<script setup lang="ts">
import { Card, CardContent } from "~/components/ui/card";

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
  <div class="grid grid-cols-2 md:grid-cols-4 gap-2 mb-4">
    <!-- Loading State -->
    <template v-if="loading">
      <Card v-for="i in 4" :key="i">
        <CardContent class="py-3 px-4">
          <div class="animate-pulse">
            <div class="h-3 bg-muted rounded w-16 mb-1"></div>
            <div class="h-5 bg-muted rounded w-12"></div>
          </div>
        </CardContent>
      </Card>
    </template>

    <!-- Stats Cards -->
    <template v-else-if="stats">
      <Card>
        <CardContent class="py-3 px-4">
          <p class="text-xs text-muted-foreground">Players</p>
          <p class="text-lg font-semibold">{{ formatNumber(stats.total_players) }}</p>
        </CardContent>
      </Card>

      <Card>
        <CardContent class="py-3 px-4">
          <p class="text-xs text-muted-foreground">Kills</p>
          <p class="text-lg font-semibold">{{ formatNumber(stats.total_kills) }}</p>
        </CardContent>
      </Card>

      <Card>
        <CardContent class="py-3 px-4">
          <p class="text-xs text-muted-foreground">Deaths</p>
          <p class="text-lg font-semibold">{{ formatNumber(stats.total_deaths) }}</p>
        </CardContent>
      </Card>

      <Card>
        <CardContent class="py-3 px-4">
          <p class="text-xs text-muted-foreground">Teamkills</p>
          <p class="text-lg font-semibold text-destructive">{{ formatNumber(stats.total_teamkills) }}</p>
        </CardContent>
      </Card>
    </template>
  </div>
</template>
