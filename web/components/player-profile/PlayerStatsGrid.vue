<script setup lang="ts">
import { Card, CardContent, CardHeader, CardTitle } from "~/components/ui/card";
import type { PlayerStatistics, TeamkillMetrics } from "~/types/player";

const props = defineProps<{
  statistics: PlayerStatistics;
  teamkillMetrics: TeamkillMetrics;
  totalSessions: number;
  totalPlayTime: number;
}>();

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
</script>

<template>
  <div class="grid grid-cols-2 md:grid-cols-3 lg:grid-cols-6 gap-4 mb-6">
    <!-- K/D Ratio -->
    <Card>
      <CardHeader class="pb-2">
        <CardTitle class="text-xs font-medium text-muted-foreground">
          K/D Ratio
        </CardTitle>
      </CardHeader>
      <CardContent>
        <div class="text-2xl font-bold">
          {{ statistics.kd_ratio.toFixed(2) }}
        </div>
        <div class="text-xs text-muted-foreground">
          {{ statistics.kills }} / {{ statistics.deaths }}
        </div>
      </CardContent>
    </Card>

    <!-- Kills -->
    <Card>
      <CardHeader class="pb-2">
        <CardTitle class="text-xs font-medium text-muted-foreground">
          Kills
        </CardTitle>
      </CardHeader>
      <CardContent>
        <div class="text-2xl font-bold text-green-500">
          {{ statistics.kills }}
        </div>
      </CardContent>
    </Card>

    <!-- Deaths -->
    <Card>
      <CardHeader class="pb-2">
        <CardTitle class="text-xs font-medium text-muted-foreground">
          Deaths
        </CardTitle>
      </CardHeader>
      <CardContent>
        <div class="text-2xl font-bold">
          {{ statistics.deaths }}
        </div>
      </CardContent>
    </Card>

    <!-- Teamkills -->
    <Card>
      <CardHeader class="pb-2">
        <CardTitle class="text-xs font-medium text-muted-foreground">
          Teamkills
        </CardTitle>
      </CardHeader>
      <CardContent>
        <div class="text-2xl font-bold text-destructive">
          {{ statistics.teamkills }}
        </div>
        <div class="text-xs text-muted-foreground">
          {{ teamkillMetrics.teamkills_per_session.toFixed(2) }}/session
        </div>
      </CardContent>
    </Card>

    <!-- Revives -->
    <Card>
      <CardHeader class="pb-2">
        <CardTitle class="text-xs font-medium text-muted-foreground">
          Revives
        </CardTitle>
      </CardHeader>
      <CardContent>
        <div class="text-2xl font-bold text-green-500">
          {{ statistics.revives }}
        </div>
        <div class="text-xs text-muted-foreground">
          Revived {{ statistics.times_revived }}x
        </div>
      </CardContent>
    </Card>

    <!-- Sessions -->
    <Card>
      <CardHeader class="pb-2">
        <CardTitle class="text-xs font-medium text-muted-foreground">
          Sessions
        </CardTitle>
      </CardHeader>
      <CardContent>
        <div class="text-2xl font-bold">
          {{ totalSessions }}
        </div>
        <div class="text-xs text-muted-foreground">
          {{ formatPlayTime(totalPlayTime) }}
        </div>
      </CardContent>
    </Card>
  </div>
</template>
