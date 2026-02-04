<script setup lang="ts">
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
import type { RuleViolation, ViolationSummary } from "~/types/player";

const props = defineProps<{
  violations: RuleViolation[];
  summary: ViolationSummary;
}>();

function formatDate(dateString: string): string {
  return new Date(dateString).toLocaleString();
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
  const minutes = Math.floor(diff / (1000 * 60));
  return `${minutes} minutes ago`;
}

function getViolationBadgeVariant(
  actionType: string
): "default" | "destructive" | "outline" | "secondary" {
  switch (actionType.toUpperCase()) {
    case "BAN":
      return "destructive";
    case "KICK":
      return "secondary";
    case "WARN":
      return "outline";
    default:
      return "default";
  }
}
</script>

<template>
  <Card>
    <CardHeader class="pb-3">
      <div class="flex items-center justify-between">
        <CardTitle class="text-lg">Rule Violations</CardTitle>
        <div class="flex gap-2 text-sm">
          <Badge variant="outline" class="text-yellow-600">
            {{ summary.total_warns }} Warns
          </Badge>
          <Badge variant="secondary" class="text-orange-600">
            {{ summary.total_kicks }} Kicks
          </Badge>
          <Badge variant="destructive">
            {{ summary.total_bans }} Bans
          </Badge>
        </div>
      </div>
    </CardHeader>
    <CardContent>
      <div v-if="violations.length === 0" class="text-center py-8 text-muted-foreground">
        No violations found
      </div>

      <div v-else>
        <!-- Desktop Table -->
        <div class="hidden md:block overflow-x-auto">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Date</TableHead>
                <TableHead>Action</TableHead>
                <TableHead>Rule</TableHead>
                <TableHead>Server</TableHead>
                <TableHead>Admin</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              <TableRow
                v-for="violation in violations"
                :key="violation.violation_id"
                class="hover:bg-muted/50"
              >
                <TableCell>
                  <div class="text-sm">{{ formatDate(violation.created_at) }}</div>
                  <div class="text-xs text-muted-foreground">
                    {{ getTimeAgo(violation.created_at) }}
                  </div>
                </TableCell>
                <TableCell>
                  <Badge :variant="getViolationBadgeVariant(violation.action_type)">
                    {{ violation.action_type }}
                  </Badge>
                </TableCell>
                <TableCell>
                  <div v-if="violation.rule_name" class="text-sm">
                    {{ violation.rule_name }}
                  </div>
                  <div v-else class="text-sm text-muted-foreground">
                    No rule specified
                  </div>
                </TableCell>
                <TableCell>
                  <div class="text-sm">
                    {{ violation.server_name || "Unknown Server" }}
                  </div>
                </TableCell>
                <TableCell>
                  <div class="text-sm">
                    {{ violation.admin_name || "System" }}
                  </div>
                </TableCell>
              </TableRow>
            </TableBody>
          </Table>
        </div>

        <!-- Mobile Cards -->
        <div class="md:hidden space-y-3">
          <div
            v-for="violation in violations"
            :key="violation.violation_id"
            class="border rounded-lg p-3 hover:bg-muted/30 transition-colors"
          >
            <div class="flex items-center gap-2 mb-2">
              <Badge :variant="getViolationBadgeVariant(violation.action_type)">
                {{ violation.action_type }}
              </Badge>
              <span class="text-xs text-muted-foreground">
                {{ getTimeAgo(violation.created_at) }}
              </span>
            </div>
            <div class="space-y-1 text-sm">
              <div>
                <span class="text-muted-foreground">Date: </span>
                {{ formatDate(violation.created_at) }}
              </div>
              <div v-if="violation.rule_name">
                <span class="text-muted-foreground">Rule: </span>
                {{ violation.rule_name }}
              </div>
              <div>
                <span class="text-muted-foreground">Server: </span>
                {{ violation.server_name || "Unknown" }}
              </div>
              <div>
                <span class="text-muted-foreground">Admin: </span>
                {{ violation.admin_name || "System" }}
              </div>
            </div>
          </div>
        </div>
      </div>
    </CardContent>
  </Card>
</template>
