<script setup lang="ts">
import { computed } from "vue";
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
} from "lucide-vue-next";
import type {
  RiskIndicator,
  ViolationSummary,
  TeamkillMetrics,
  CBLUser,
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
</script>

<template>
  <div class="grid grid-cols-2 md:grid-cols-4 lg:grid-cols-5 gap-3 mb-6">
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

    <!-- Teamkill Rate -->
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
          {{ teamkillMetrics.total_teamkills }} total TKs
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
