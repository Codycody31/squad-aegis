<script setup lang="ts">
import { Card, CardContent, CardHeader, CardTitle } from "~/components/ui/card";
import { Badge } from "~/components/ui/badge";
import type { ActiveBan, RuleViolation, ViolationSummary } from "~/types/player";

const props = defineProps<{
  violations: RuleViolation[];
  summary: ViolationSummary;
  activeBans: ActiveBan[];
}>();

const recentViolations = computed(() => props.violations.slice(0, 3));

function formatDate(value: string): string {
  return new Date(value).toLocaleDateString();
}

const actionVariant = (action: RuleViolation["action_type"]) =>
  action === "BAN" ? "destructive" : action === "KICK" ? "default" : "secondary";
</script>

<template>
  <Card>
    <CardHeader class="p-3 pb-1">
      <CardTitle class="text-sm">Moderation history</CardTitle>
    </CardHeader>
    <CardContent class="p-3 pt-1">
      <div class="mb-2 flex flex-wrap gap-2 text-xs">
        <Badge variant="secondary">{{ summary.total_warns }} warns</Badge>
        <Badge variant="secondary">{{ summary.total_kicks }} kicks</Badge>
        <Badge
          :variant="summary.total_bans > 0 ? 'destructive' : 'secondary'"
        >
          {{ summary.total_bans }} bans
        </Badge>
      </div>

      <div
        v-if="activeBans.length > 0"
        class="mb-2 rounded border border-destructive/50 bg-destructive/10 p-2 text-xs"
      >
        <p class="font-medium text-destructive">
          {{ activeBans.length }} active ban{{ activeBans.length > 1 ? "s" : "" }}
        </p>
        <p class="mt-0.5 text-muted-foreground">
          {{ activeBans[0].server_name }}: {{ activeBans[0].reason }}
          <span v-if="activeBans[0].permanent"> (permanent)</span>
        </p>
      </div>

      <ul v-if="recentViolations.length > 0" class="space-y-1.5">
        <li
          v-for="violation in recentViolations"
          :key="violation.violation_id"
          class="flex items-start justify-between gap-2 text-xs"
        >
          <div class="min-w-0">
            <Badge :variant="actionVariant(violation.action_type)" class="mr-1">
              {{ violation.action_type }}
            </Badge>
            <span class="text-muted-foreground">
              {{ violation.rule_name || violation.reason || "No reason recorded" }}
            </span>
          </div>
          <span class="shrink-0 text-muted-foreground">
            {{ formatDate(violation.created_at) }}
          </span>
        </li>
      </ul>
      <p v-else class="text-xs text-muted-foreground">
        No recorded violations.
      </p>
    </CardContent>
  </Card>
</template>
