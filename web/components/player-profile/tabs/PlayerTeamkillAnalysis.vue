<script setup lang="ts">
import { ref, onMounted } from "vue";
import { Card, CardContent, CardHeader, CardTitle } from "~/components/ui/card";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "~/components/ui/table";
import { Badge } from "~/components/ui/badge";
import { Loader2, Skull, Target } from "lucide-vue-next";
import type { TeamkillVictim, TeamkillMetrics, WeaponStat } from "~/types/player";

const props = defineProps<{
  playerId: string;
  metrics: TeamkillMetrics;
  weaponStats: WeaponStat[];
}>();

const runtimeConfig = useRuntimeConfig();
const loading = ref(false);
const error = ref<string | null>(null);

const victims = ref<TeamkillVictim[]>([]);
const tkWeapons = ref<{ weapon: string; tk_count: number }[]>([]);

async function fetchTeamkillAnalysis() {
  loading.value = true;
  error.value = null;

  const cookieToken = useCookie(
    runtimeConfig.public.sessionCookieName as string
  );
  const token = cookieToken.value;

  if (!token) {
    error.value = "Authentication required";
    loading.value = false;
    return;
  }

  try {
    const response = await fetch(
      `${runtimeConfig.public.backendApi}/players/${props.playerId}/teamkills`,
      {
        headers: {
          Authorization: `Bearer ${token}`,
        },
      }
    );

    if (!response.ok) throw new Error("Failed to fetch teamkill analysis");

    const data = await response.json();
    victims.value = data.data.victims || [];
    tkWeapons.value = data.data.tk_weapons || [];
  } catch (err: any) {
    error.value = err.message;
  } finally {
    loading.value = false;
  }
}

function formatDate(dateString: string): string {
  return new Date(dateString).toLocaleDateString();
}

onMounted(() => {
  fetchTeamkillAnalysis();
});
</script>

<template>
  <div class="space-y-6">
    <!-- Summary Cards -->
    <div class="grid grid-cols-2 md:grid-cols-4 gap-4">
      <Card>
        <CardContent class="p-4 text-center">
          <div class="text-xs text-muted-foreground mb-1">Total TKs</div>
          <div class="text-2xl font-bold text-destructive">
            {{ metrics.total_teamkills }}
          </div>
        </CardContent>
      </Card>
      <Card>
        <CardContent class="p-4 text-center">
          <div class="text-xs text-muted-foreground mb-1">TKs per Session</div>
          <div class="text-2xl font-bold">
            {{ metrics.teamkills_per_session.toFixed(2) }}
          </div>
        </CardContent>
      </Card>
      <Card>
        <CardContent class="p-4 text-center">
          <div class="text-xs text-muted-foreground mb-1">TK Ratio</div>
          <div class="text-2xl font-bold">
            {{ (metrics.teamkill_ratio * 100).toFixed(1) }}%
          </div>
        </CardContent>
      </Card>
      <Card>
        <CardContent class="p-4 text-center">
          <div class="text-xs text-muted-foreground mb-1">Recent (7d)</div>
          <div
            class="text-2xl font-bold"
            :class="{
              'text-destructive': metrics.recent_teamkills >= 5,
              'text-orange-500': metrics.recent_teamkills >= 3,
            }"
          >
            {{ metrics.recent_teamkills }}
          </div>
        </CardContent>
      </Card>
    </div>

    <div v-if="loading" class="flex justify-center py-8">
      <Loader2 class="h-8 w-8 animate-spin text-muted-foreground" />
    </div>

    <div v-else-if="error" class="text-center py-8 text-destructive">
      {{ error }}
    </div>

    <div v-else class="grid md:grid-cols-2 gap-6">
      <!-- Top Victims -->
      <Card>
        <CardHeader class="pb-3">
          <CardTitle class="text-lg flex items-center gap-2">
            <Skull class="h-5 w-5 text-destructive" />
            Most Teamkilled Players
          </CardTitle>
        </CardHeader>
        <CardContent>
          <div v-if="victims.length === 0" class="text-center py-4 text-muted-foreground">
            No teamkill data available
          </div>
          <Table v-else>
            <TableHeader>
              <TableRow>
                <TableHead>Player</TableHead>
                <TableHead class="text-right">TKs</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              <TableRow v-for="victim in victims.slice(0, 10)" :key="victim.victim_steam">
                <TableCell>
                  <div class="font-medium">{{ victim.victim_name }}</div>
                  <div class="text-xs text-muted-foreground">
                    First: {{ formatDate(victim.first_tk) }} -
                    Last: {{ formatDate(victim.last_tk) }}
                  </div>
                </TableCell>
                <TableCell class="text-right">
                  <Badge
                    :variant="victim.tk_count >= 3 ? 'destructive' : 'secondary'"
                  >
                    {{ victim.tk_count }}
                  </Badge>
                </TableCell>
              </TableRow>
            </TableBody>
          </Table>
        </CardContent>
      </Card>

      <!-- TK Weapons -->
      <Card>
        <CardHeader class="pb-3">
          <CardTitle class="text-lg flex items-center gap-2">
            <Target class="h-5 w-5 text-orange-500" />
            Teamkill Weapons
          </CardTitle>
        </CardHeader>
        <CardContent>
          <div v-if="tkWeapons.length === 0" class="text-center py-4 text-muted-foreground">
            No weapon data available
          </div>
          <Table v-else>
            <TableHeader>
              <TableRow>
                <TableHead>Weapon</TableHead>
                <TableHead class="text-right">TKs</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              <TableRow v-for="weapon in tkWeapons" :key="weapon.weapon">
                <TableCell class="font-medium">{{ weapon.weapon }}</TableCell>
                <TableCell class="text-right">
                  <Badge variant="outline">{{ weapon.tk_count }}</Badge>
                </TableCell>
              </TableRow>
            </TableBody>
          </Table>
        </CardContent>
      </Card>
    </div>

    <!-- All Weapon Stats -->
    <Card>
      <CardHeader class="pb-3">
        <CardTitle class="text-lg">Weapon Statistics</CardTitle>
      </CardHeader>
      <CardContent>
        <div v-if="weaponStats.length === 0" class="text-center py-4 text-muted-foreground">
          No weapon statistics available
        </div>
        <div v-else class="overflow-x-auto">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Weapon</TableHead>
                <TableHead class="text-right">Kills</TableHead>
                <TableHead class="text-right">Teamkills</TableHead>
                <TableHead class="text-right">TK %</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              <TableRow v-for="stat in weaponStats" :key="stat.weapon">
                <TableCell class="font-medium">{{ stat.weapon }}</TableCell>
                <TableCell class="text-right text-green-500">
                  {{ stat.kills }}
                </TableCell>
                <TableCell class="text-right text-destructive">
                  {{ stat.teamkills }}
                </TableCell>
                <TableCell class="text-right">
                  {{
                    stat.kills > 0
                      ? ((stat.teamkills / stat.kills) * 100).toFixed(1)
                      : "0"
                  }}%
                </TableCell>
              </TableRow>
            </TableBody>
          </Table>
        </div>
      </CardContent>
    </Card>
  </div>
</template>
