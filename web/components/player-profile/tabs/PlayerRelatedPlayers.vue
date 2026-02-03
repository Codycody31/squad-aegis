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
import { Button } from "~/components/ui/button";
import { Loader2, Users, AlertTriangle, ExternalLink } from "lucide-vue-next";
import type { RelatedPlayer } from "~/types/player";

const props = defineProps<{
  playerId: string;
}>();

const router = useRouter();
const runtimeConfig = useRuntimeConfig();
const loading = ref(false);
const error = ref<string | null>(null);
const permissionDenied = ref(false);

const relatedPlayers = ref<RelatedPlayer[]>([]);

async function fetchRelatedPlayers() {
  loading.value = true;
  error.value = null;
  permissionDenied.value = false;

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
      `${runtimeConfig.public.backendApi}/players/${props.playerId}/related`,
      {
        headers: {
          Authorization: `Bearer ${token}`,
        },
      }
    );

    if (response.status === 403) {
      permissionDenied.value = true;
      loading.value = false;
      return;
    }

    if (!response.ok) throw new Error("Failed to fetch related players");

    const data = await response.json();
    relatedPlayers.value = data.data.related_players || [];
  } catch (err: any) {
    error.value = err.message;
  } finally {
    loading.value = false;
  }
}

function viewPlayer(steamId: string, eosId: string) {
  const id = steamId || eosId;
  if (id) {
    router.push(`/players/${id}`);
  }
}

onMounted(() => {
  fetchRelatedPlayers();
});
</script>

<template>
  <Card>
    <CardHeader class="pb-3">
      <CardTitle class="text-lg flex items-center gap-2">
        <Users class="h-5 w-5" />
        Related Players (Same IP)
      </CardTitle>
    </CardHeader>
    <CardContent>
      <div v-if="loading" class="flex justify-center py-8">
        <Loader2 class="h-8 w-8 animate-spin text-muted-foreground" />
      </div>

      <div v-else-if="permissionDenied" class="text-center py-8">
        <AlertTriangle class="h-12 w-12 text-muted-foreground mx-auto mb-3" />
        <p class="text-muted-foreground">
          You do not have permission to view related players.
        </p>
        <p class="text-xs text-muted-foreground mt-1">
          This feature requires super admin privileges.
        </p>
      </div>

      <div v-else-if="error" class="text-center py-8 text-destructive">
        {{ error }}
      </div>

      <div v-else-if="relatedPlayers.length === 0" class="text-center py-8 text-muted-foreground">
        No related players found (no shared IP addresses detected)
      </div>

      <div v-else>
        <div class="bg-orange-500/10 border border-orange-500/30 rounded-lg p-3 mb-4">
          <div class="flex items-start gap-2">
            <AlertTriangle class="h-5 w-5 text-orange-500 mt-0.5" />
            <div class="text-sm">
              <p class="font-medium text-orange-600 dark:text-orange-400">
                Potential Alt Accounts
              </p>
              <p class="text-muted-foreground text-xs mt-1">
                These players have connected from the same IP address. This may indicate
                alt accounts, but could also be players from the same household or network.
              </p>
            </div>
          </div>
        </div>

        <!-- Desktop Table -->
        <div class="hidden md:block overflow-x-auto">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Player</TableHead>
                <TableHead>Shared Sessions</TableHead>
                <TableHead>Status</TableHead>
                <TableHead></TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              <TableRow
                v-for="player in relatedPlayers"
                :key="player.steam_id || player.eos_id"
                class="hover:bg-muted/50"
              >
                <TableCell>
                  <div class="font-medium">{{ player.player_name }}</div>
                  <div class="text-xs text-muted-foreground">
                    {{ player.steam_id || player.eos_id }}
                  </div>
                </TableCell>
                <TableCell>
                  <Badge variant="outline">{{ player.shared_sessions }}</Badge>
                </TableCell>
                <TableCell>
                  <Badge v-if="player.is_banned" variant="destructive">
                    BANNED
                  </Badge>
                  <Badge v-else variant="secondary">Active</Badge>
                </TableCell>
                <TableCell>
                  <Button
                    variant="ghost"
                    size="sm"
                    @click="viewPlayer(player.steam_id, player.eos_id)"
                  >
                    <ExternalLink class="h-4 w-4" />
                  </Button>
                </TableCell>
              </TableRow>
            </TableBody>
          </Table>
        </div>

        <!-- Mobile Cards -->
        <div class="md:hidden space-y-3">
          <div
            v-for="player in relatedPlayers"
            :key="player.steam_id || player.eos_id"
            class="border rounded-lg p-3 hover:bg-muted/30 transition-colors cursor-pointer"
            @click="viewPlayer(player.steam_id, player.eos_id)"
          >
            <div class="flex items-center justify-between mb-2">
              <span class="font-medium">{{ player.player_name }}</span>
              <Badge v-if="player.is_banned" variant="destructive">BANNED</Badge>
              <Badge v-else variant="secondary">Active</Badge>
            </div>
            <div class="text-xs text-muted-foreground">
              {{ player.shared_sessions }} shared sessions
            </div>
          </div>
        </div>
      </div>
    </CardContent>
  </Card>
</template>
