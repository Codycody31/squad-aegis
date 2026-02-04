<script setup lang="ts">
import { Card, CardContent, CardHeader, CardTitle } from "~/components/ui/card";
import type { PlayerStatistics, RecentServerInfo } from "~/types/player";

const props = defineProps<{
  statistics: PlayerStatistics;
  totalSessions: number;
  totalPlayTime: number;
  recentServers: RecentServerInfo[];
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

function getTimeAgo(dateString: string): string {
  const date = new Date(dateString);
  const now = new Date();
  const diff = now.getTime() - date.getTime();
  const days = Math.floor(diff / (1000 * 60 * 60 * 24));
  if (days > 30) return `${Math.floor(days / 30)} months ago`;
  if (days > 0) return `${days} days ago`;
  const hours = Math.floor(diff / (1000 * 60 * 60));
  if (hours > 0) return `${hours} hours ago`;
  return "Recently";
}
</script>

<template>
  <div class="space-y-6">
    <Card>
      <CardHeader class="pb-3">
        <CardTitle class="text-lg">Combat Statistics</CardTitle>
      </CardHeader>
      <CardContent>
        <div class="grid md:grid-cols-2 gap-6">
          <div>
            <h3 class="text-base font-semibold mb-4">Combat</h3>
            <div class="space-y-3">
              <div class="flex justify-between">
                <span class="text-muted-foreground">Kills</span>
                <span class="font-medium text-green-500">{{ statistics.kills }}</span>
              </div>
              <div class="flex justify-between">
                <span class="text-muted-foreground">Deaths</span>
                <span class="font-medium">{{ statistics.deaths }}</span>
              </div>
              <div class="flex justify-between">
                <span class="text-muted-foreground">K/D Ratio</span>
                <span class="font-medium">{{ statistics.kd_ratio.toFixed(2) }}</span>
              </div>
              <div class="flex justify-between">
                <span class="text-destructive">Teamkills</span>
                <span class="font-medium text-destructive">{{ statistics.teamkills }}</span>
              </div>
            </div>
          </div>

          <div>
            <h3 class="text-base font-semibold mb-4">Support</h3>
            <div class="space-y-3">
              <div class="flex justify-between">
                <span class="text-muted-foreground">Revives Given</span>
                <span class="font-medium text-green-500">{{ statistics.revives }}</span>
              </div>
              <div class="flex justify-between">
                <span class="text-muted-foreground">Times Revived</span>
                <span class="font-medium">{{ statistics.times_revived }}</span>
              </div>
            </div>
          </div>

          <div>
            <h3 class="text-base font-semibold mb-4">Damage</h3>
            <div class="space-y-3">
              <div class="flex justify-between">
                <span class="text-muted-foreground">Damage Dealt</span>
                <span class="font-medium">{{ statistics.damage_dealt.toFixed(0) }}</span>
              </div>
              <div class="flex justify-between">
                <span class="text-muted-foreground">Damage Taken</span>
                <span class="font-medium">{{ statistics.damage_taken.toFixed(0) }}</span>
              </div>
            </div>
          </div>

          <div>
            <h3 class="text-base font-semibold mb-4">Activity</h3>
            <div class="space-y-3">
              <div class="flex justify-between">
                <span class="text-muted-foreground">Total Sessions</span>
                <span class="font-medium">{{ totalSessions }}</span>
              </div>
              <div class="flex justify-between">
                <span class="text-muted-foreground">Play Time</span>
                <span class="font-medium">{{ formatPlayTime(totalPlayTime) }}</span>
              </div>
            </div>
          </div>
        </div>
      </CardContent>
    </Card>

    <!-- Recent Servers -->
    <Card>
      <CardHeader class="pb-3">
        <CardTitle class="text-lg">Recent Servers</CardTitle>
      </CardHeader>
      <CardContent>
        <div v-if="recentServers.length === 0" class="text-center py-4 text-muted-foreground">
          No server history available
        </div>
        <div v-else class="space-y-3">
          <div
            v-for="server in recentServers"
            :key="server.server_id"
            class="flex items-center justify-between p-3 rounded-lg border hover:bg-muted/30 transition-colors"
          >
            <div>
              <div class="font-medium">{{ server.server_name }}</div>
              <div class="text-xs text-muted-foreground">
                {{ server.sessions }} sessions
              </div>
            </div>
            <div class="text-right text-sm">
              <div class="text-muted-foreground">{{ getTimeAgo(server.last_seen) }}</div>
            </div>
          </div>
        </div>
      </CardContent>
    </Card>
  </div>
</template>
