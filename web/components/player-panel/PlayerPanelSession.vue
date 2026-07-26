<script setup lang="ts">
import { Card, CardContent } from "~/components/ui/card";
import { Badge } from "~/components/ui/badge";
import type { Player } from "~/types";

const props = defineProps<{
  player: Player;
}>();

const isLive = computed(() => props.player.sinceDisconnect === "");
</script>

<template>
  <Card>
    <CardContent class="p-3">
      <template v-if="isLive">
        <p class="mb-1 text-xs font-medium text-muted-foreground">
          Currently on server
        </p>
        <div class="flex flex-wrap items-center gap-2 text-sm">
          <Badge variant="outline">
            {{ player.teamId ? `Team ${player.teamId}` : "Team unknown" }}
          </Badge>
          <Badge variant="outline">
            {{ player.squadId ? `Squad ${player.squadId}` : "Unassigned" }}
          </Badge>
          <Badge v-if="player.isSquadLeader" variant="default">
            <Icon name="lucide:star" class="mr-1 h-3 w-3" />
            Squad Leader
          </Badge>
        </div>
        <p v-if="player.role" class="mt-1.5 truncate font-mono text-xs text-muted-foreground">
          {{ player.role }}
        </p>
      </template>
      <template v-else-if="player.sinceDisconnect">
        <p class="text-sm text-muted-foreground">
          <Icon name="mdi:account-off" class="mr-1 inline h-4 w-4" />
          Disconnected {{ player.sinceDisconnect }} ago
        </p>
      </template>
    </CardContent>
  </Card>
</template>
