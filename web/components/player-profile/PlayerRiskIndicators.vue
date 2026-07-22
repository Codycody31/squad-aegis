<script setup lang="ts">
import { computed, ref } from "vue";
import { Card, CardContent } from "~/components/ui/card";
import { Badge } from "~/components/ui/badge";
import {
  ShieldAlert,
  AlertTriangle,
  AlertCircle,
  Info,
  Skull,
  Users,
  Ban,
  FileWarning,
  ChevronDown,
  ChevronUp,
  Crosshair,
} from "lucide-vue-next";
import type {
  RiskIndicator,
  ViolationSummary,
  TeamkillMetrics,
  CBLUser,
  CBLBan,
} from "~/types/player";

const props = defineProps<{
  riskIndicators: RiskIndicator[];
  violationSummary: ViolationSummary;
  teamkillMetrics: TeamkillMetrics;
  cblData?: CBLUser | null;
  nameCount: number;
}>();

function getSeverityIcon(severity: string) {
  switch (severity) {
    case "critical":
      return ShieldAlert;
    case "high":
      return AlertTriangle;
    case "medium":
      return AlertCircle;
    default:
      return Info;
  }
}

function getSeverityColor(severity: string) {
  switch (severity) {
    case "critical":
      return "bg-destructive/20 border-destructive text-destructive";
    case "high":
      return "bg-orange-500/20 border-orange-500 text-orange-600 dark:text-orange-400";
    case "medium":
      return "bg-yellow-500/20 border-yellow-500 text-yellow-600 dark:text-yellow-400";
    default:
      return "bg-blue-500/20 border-blue-500 text-blue-600 dark:text-blue-400";
  }
}

function getCBLColor(riskRating: number) {
  if (riskRating >= 8) return "text-destructive";
  if (riskRating >= 6) return "text-orange-500";
  if (riskRating >= 4) return "text-yellow-500";
  if (riskRating >= 2) return "text-blue-500";
  return "text-green-500";
}

function getCBLBadgeVariant(
  riskRating: number
): "destructive" | "secondary" | "default" {
  if (riskRating >= 6) return "destructive";
  if (riskRating >= 3) return "secondary";
  return "default";
}

const totalViolations = computed(
  () =>
    props.violationSummary.total_warns +
    props.violationSummary.total_kicks +
    props.violationSummary.total_bans
);

const tkPercentage = computed(() => {
  return (props.teamkillMetrics.teamkill_ratio * 100).toFixed(1);
});

const totalFriendlyFire = computed(() => {
  return (
    (props.teamkillMetrics.total_teamkills || 0) +
    (props.teamkillMetrics.total_team_wounds || 0) +
    (props.teamkillMetrics.total_team_damage || 0)
  );
});

const recentFriendlyFire = computed(() => {
  return (
    (props.teamkillMetrics.recent_teamkills || 0) +
    (props.teamkillMetrics.recent_team_wounds || 0) +
    (props.teamkillMetrics.recent_team_damage || 0)
  );
});

// Teamkill context: how many teamkills count toward the (competitive) TK rate
// vs. how many were excluded because they happened on seeding/training layers.
const competitiveTeamkills = computed(
  () => props.teamkillMetrics.total_teamkills || 0
);

const seedingTeamkills = computed(
  () => props.teamkillMetrics.non_competitive?.seeding_teamkills ?? 0
);
const trainingTeamkills = computed(
  () => props.teamkillMetrics.non_competitive?.training_teamkills ?? 0
);
const excludedTeamkills = computed(
  () => seedingTeamkills.value + trainingTeamkills.value
);

const tkContextTotal = computed(
  () => competitiveTeamkills.value + excludedTeamkills.value
);

function contextPct(count: number): number {
  return tkContextTotal.value > 0 ? (count / tkContextTotal.value) * 100 : 0;
}

// Friendly fire (TKs + wounds + damage) excluded from the headline card.
const excludedFriendlyFire = computed(() => {
  const nc = props.teamkillMetrics.non_competitive;
  if (!nc) return 0;
  return (
    (nc.seeding_teamkills || 0) +
    (nc.training_teamkills || 0) +
    (nc.seeding_team_wounds || 0) +
    (nc.training_team_wounds || 0) +
    (nc.seeding_team_damage || 0) +
    (nc.training_team_damage || 0)
  );
});

const showCblBans = ref(false);

const cblActiveBans = computed<CBLBan[]>(() => props.cblData?.activeBans ?? []);
const cblExpiredBans = computed<CBLBan[]>(
  () => props.cblData?.expiredBans ?? []
);
const cblBanCount = computed(
  () => cblActiveBans.value.length + cblExpiredBans.value.length
);

function cblBanOrg(b: CBLBan): string {
  return b.banList?.organisation?.name || b.banList?.name || "Unknown source";
}

function cblBanDate(d: string | null): string {
  if (!d) return "permanent";
  const parsed = new Date(d);
  return isNaN(parsed.getTime()) ? d : parsed.toLocaleDateString();
}
</script>

<template>
  <div class="grid grid-cols-2 md:grid-cols-4 lg:grid-cols-6 gap-3 mb-6">
    <!-- CBL Risk Rating -->
    <Card v-if="cblData" class="col-span-1">
      <CardContent class="p-4 text-center">
        <div class="text-xs text-muted-foreground mb-1">CBL Risk</div>
        <div class="text-3xl font-bold" :class="getCBLColor(cblData.riskRating)">
          {{ cblData.riskRating }}/10
        </div>
        <Badge :variant="getCBLBadgeVariant(cblData.riskRating)" class="mt-1">
          {{ cblData.reputationPoints }} pts
        </Badge>
      </CardContent>
    </Card>

    <!-- CBL Ban History -->
    <Card v-if="cblData && cblBanCount > 0" class="col-span-2 md:col-span-4 lg:col-span-6">
      <CardContent class="p-4">
        <button
          type="button"
          class="flex w-full items-center justify-between text-left"
          :aria-expanded="showCblBans"
          @click="showCblBans = !showCblBans"
        >
          <span class="text-sm font-medium">
            CBL Ban History
            <span class="text-muted-foreground">
              ({{ cblActiveBans.length }} active, {{ cblExpiredBans.length }} expired)
            </span>
          </span>
          <span class="text-xs text-muted-foreground">
            <ChevronUp v-if="showCblBans" class="h-4 w-4 text-muted-foreground" /><ChevronDown v-else class="h-4 w-4 text-muted-foreground" />
          </span>
        </button>

        <div v-if="showCblBans" class="mt-3 space-y-2">
          <div
            v-for="b in cblActiveBans"
            :key="b.id"
            class="rounded border border-destructive/40 bg-destructive/5 p-2 text-sm"
          >
            <div class="flex items-center justify-between">
              <Badge variant="destructive">Active</Badge>
              <span class="text-xs text-muted-foreground">{{ cblBanOrg(b) }}</span>
            </div>
            <div class="mt-1">{{ b.reason || "No reason provided" }}</div>
            <div class="mt-1 text-xs text-muted-foreground">
              {{ cblBanDate(b.created) }} → {{ cblBanDate(b.expires) }}
            </div>
          </div>

          <div
            v-for="b in cblExpiredBans"
            :key="b.id"
            class="rounded border border-border bg-muted/30 p-2 text-sm text-muted-foreground"
          >
            <div class="flex items-center justify-between">
              <Badge variant="secondary">Expired</Badge>
              <span class="text-xs">{{ cblBanOrg(b) }}</span>
            </div>
            <div class="mt-1">{{ b.reason || "No reason provided" }}</div>
            <div class="mt-1 text-xs">
              {{ cblBanDate(b.created) }} → {{ cblBanDate(b.expires) }}
            </div>
          </div>
        </div>
      </CardContent>
    </Card>

    <!-- Teamkill Rate (competitive / real-match only) -->
    <Card class="col-span-1">
      <CardContent class="p-4 text-center">
        <div class="text-xs text-muted-foreground mb-1">TK Rate</div>
        <div
          class="text-3xl font-bold"
          :class="{
            'text-destructive': teamkillMetrics.teamkill_ratio > 0.1,
            'text-orange-500':
              teamkillMetrics.teamkill_ratio > 0.05 &&
              teamkillMetrics.teamkill_ratio <= 0.1,
            'text-green-500': teamkillMetrics.teamkill_ratio <= 0.05,
          }"
        >
          {{ tkPercentage }}%
        </div>
        <div class="text-xs text-muted-foreground">
          {{ competitiveTeamkills }} match TK{{
            competitiveTeamkills === 1 ? "" : "s"
          }}
        </div>
        <div
          v-if="excludedTeamkills > 0"
          class="text-[11px] text-muted-foreground/70"
        >
          +{{ excludedTeamkills }} on seed/training
        </div>
      </CardContent>
    </Card>

    <!-- Friendly Fire (Total Team Damage/Wounds/Kills) -->
    <Card class="col-span-1">
      <CardContent class="p-4 text-center">
        <div class="text-xs text-muted-foreground mb-1">Friendly Fire</div>
        <div
          class="text-3xl font-bold"
          :class="{
            'text-destructive': totalFriendlyFire >= 20,
            'text-orange-500': totalFriendlyFire >= 10 && totalFriendlyFire < 20,
            'text-yellow-500': totalFriendlyFire >= 5 && totalFriendlyFire < 10,
            'text-muted-foreground': totalFriendlyFire < 5,
          }"
        >
          {{ totalFriendlyFire }}
        </div>
        <div class="text-xs text-muted-foreground">
          {{ recentFriendlyFire }} recent
        </div>
        <div
          v-if="excludedFriendlyFire > 0"
          class="text-[11px] text-muted-foreground/70"
        >
          +{{ excludedFriendlyFire }} seed/training
        </div>
      </CardContent>
    </Card>

    <!-- Recent TKs -->
    <Card class="col-span-1">
      <CardContent class="p-4 text-center">
        <div class="text-xs text-muted-foreground mb-1">Recent TKs</div>
        <div
          class="text-3xl font-bold"
          :class="{
            'text-destructive': teamkillMetrics.recent_teamkills >= 5,
            'text-orange-500':
              teamkillMetrics.recent_teamkills >= 3 &&
              teamkillMetrics.recent_teamkills < 5,
            'text-muted-foreground': teamkillMetrics.recent_teamkills < 3,
          }"
        >
          {{ teamkillMetrics.recent_teamkills }}
        </div>
        <div class="text-xs text-muted-foreground">last 7 days</div>
      </CardContent>
    </Card>

    <!-- Violations -->
    <Card class="col-span-1">
      <CardContent class="p-4 text-center">
        <div class="text-xs text-muted-foreground mb-1">Violations</div>
        <div
          class="text-3xl font-bold"
          :class="{
            'text-destructive': totalViolations >= 10,
            'text-orange-500': totalViolations >= 5 && totalViolations < 10,
            'text-muted-foreground': totalViolations < 5,
          }"
        >
          {{ totalViolations }}
        </div>
        <div class="text-xs text-muted-foreground space-x-2">
          <span class="text-yellow-500"
            >{{ violationSummary.total_warns }}W</span
          >
          <span class="text-orange-500"
            >{{ violationSummary.total_kicks }}K</span
          >
          <span class="text-destructive"
            >{{ violationSummary.total_bans }}B</span
          >
        </div>
      </CardContent>
    </Card>

    <!-- Names Used -->
    <Card class="col-span-1">
      <CardContent class="p-4 text-center">
        <div class="text-xs text-muted-foreground mb-1">Names Used</div>
        <div
          class="text-3xl font-bold"
          :class="{
            'text-orange-500': nameCount > 5,
            'text-yellow-500': nameCount > 3 && nameCount <= 5,
            'text-muted-foreground': nameCount <= 3,
          }"
        >
          {{ nameCount }}
        </div>
        <div class="text-xs text-muted-foreground">aliases</div>
      </CardContent>
    </Card>
  </div>

  <!-- Teamkill context: what counts toward the rate vs. what was excluded -->
  <div
    v-if="excludedTeamkills > 0"
    class="mb-6 rounded-lg border border-border bg-card p-4"
  >
    <div class="mb-3 flex items-center justify-between gap-2">
      <div class="flex items-center gap-2">
        <Crosshair class="h-4 w-4 text-muted-foreground" />
        <span class="text-sm font-medium">Teamkill context</span>
      </div>
      <span class="text-xs text-muted-foreground">
        {{ competitiveTeamkills }} of {{ tkContextTotal }} count toward TK rate
      </span>
    </div>

    <div class="flex h-2.5 w-full overflow-hidden rounded-full bg-muted">
      <div
        v-if="competitiveTeamkills > 0"
        class="bg-destructive motion-safe:transition-all"
        :style="{ width: contextPct(competitiveTeamkills) + '%' }"
      />
      <div
        v-if="seedingTeamkills > 0"
        class="bg-muted-foreground/50 motion-safe:transition-all"
        :style="{ width: contextPct(seedingTeamkills) + '%' }"
      />
      <div
        v-if="trainingTeamkills > 0"
        class="bg-muted-foreground/30 motion-safe:transition-all"
        :style="{ width: contextPct(trainingTeamkills) + '%' }"
      />
    </div>

    <div class="mt-3 flex flex-wrap gap-x-4 gap-y-1.5 text-xs">
      <span class="inline-flex items-center gap-1.5">
        <span class="h-2 w-2 rounded-full bg-destructive" />
        <span class="text-foreground">Match</span>
        <span class="text-muted-foreground">{{ competitiveTeamkills }}</span>
      </span>
      <span
        v-if="seedingTeamkills > 0"
        class="inline-flex items-center gap-1.5"
      >
        <span class="h-2 w-2 rounded-full bg-muted-foreground/50" />
        <span class="text-foreground">Seeding</span>
        <span class="text-muted-foreground">{{ seedingTeamkills }}</span>
      </span>
      <span
        v-if="trainingTeamkills > 0"
        class="inline-flex items-center gap-1.5"
      >
        <span class="h-2 w-2 rounded-full bg-muted-foreground/30" />
        <span class="text-foreground">Training</span>
        <span class="text-muted-foreground">{{ trainingTeamkills }}</span>
      </span>
    </div>

    <p class="mt-2 text-xs text-muted-foreground">
      Seeding and training teamkills (e.g. Jensen's Range) don't count toward the
      TK rate or risk flags.
    </p>
  </div>

  <!-- Risk Indicator Badges -->
  <div v-if="riskIndicators.length > 0" class="flex flex-wrap gap-2 mb-6">
    <div
      v-for="indicator in riskIndicators"
      :key="indicator.type"
      class="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-full border text-xs font-medium"
      :class="getSeverityColor(indicator.severity)"
    >
      <component :is="getSeverityIcon(indicator.severity)" class="h-3.5 w-3.5" />
      <span>{{ indicator.description }}</span>
    </div>
  </div>
</template>
