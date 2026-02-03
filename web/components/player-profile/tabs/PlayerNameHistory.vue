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
import { User } from "lucide-vue-next";
import type { NameHistoryEntry } from "~/types/player";

const props = defineProps<{
  nameHistory: NameHistoryEntry[];
  currentName: string;
}>();

function formatDate(dateString: string): string {
  return new Date(dateString).toLocaleDateString();
}

function getTimeAgo(dateString: string): string {
  const date = new Date(dateString);
  const now = new Date();
  const diff = now.getTime() - date.getTime();
  const days = Math.floor(diff / (1000 * 60 * 60 * 24));
  if (days > 365) return `${Math.floor(days / 365)} years ago`;
  if (days > 30) return `${Math.floor(days / 30)} months ago`;
  if (days > 0) return `${days} days ago`;
  return "Today";
}
</script>

<template>
  <Card>
    <CardHeader class="pb-3">
      <CardTitle class="text-lg flex items-center gap-2">
        <User class="h-5 w-5" />
        Name History ({{ nameHistory.length }} names)
      </CardTitle>
    </CardHeader>
    <CardContent>
      <div v-if="nameHistory.length === 0" class="text-center py-8 text-muted-foreground">
        No name history available
      </div>

      <div v-else>
        <!-- Desktop Table -->
        <div class="hidden md:block overflow-x-auto">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Name</TableHead>
                <TableHead>First Used</TableHead>
                <TableHead>Last Used</TableHead>
                <TableHead class="text-right">Sessions</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              <TableRow
                v-for="entry in nameHistory"
                :key="entry.name"
                class="hover:bg-muted/50"
              >
                <TableCell>
                  <div class="flex items-center gap-2">
                    <span class="font-medium">{{ entry.name }}</span>
                    <Badge v-if="entry.name === currentName" variant="default">
                      Current
                    </Badge>
                  </div>
                </TableCell>
                <TableCell>
                  <div class="text-sm">{{ formatDate(entry.first_used) }}</div>
                </TableCell>
                <TableCell>
                  <div class="text-sm">{{ formatDate(entry.last_used) }}</div>
                  <div class="text-xs text-muted-foreground">
                    {{ getTimeAgo(entry.last_used) }}
                  </div>
                </TableCell>
                <TableCell class="text-right">
                  <Badge variant="outline">{{ entry.session_count }}</Badge>
                </TableCell>
              </TableRow>
            </TableBody>
          </Table>
        </div>

        <!-- Mobile Cards -->
        <div class="md:hidden space-y-3">
          <div
            v-for="entry in nameHistory"
            :key="entry.name"
            class="border rounded-lg p-3 hover:bg-muted/30 transition-colors"
          >
            <div class="flex items-center justify-between mb-2">
              <div class="flex items-center gap-2">
                <span class="font-medium">{{ entry.name }}</span>
                <Badge v-if="entry.name === currentName" variant="default">
                  Current
                </Badge>
              </div>
              <Badge variant="outline">{{ entry.session_count }} sessions</Badge>
            </div>
            <div class="text-xs text-muted-foreground">
              {{ formatDate(entry.first_used) }} - {{ formatDate(entry.last_used) }}
              ({{ getTimeAgo(entry.last_used) }})
            </div>
          </div>
        </div>
      </div>
    </CardContent>
  </Card>
</template>
