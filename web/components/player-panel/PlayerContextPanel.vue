<script setup lang="ts">
import { ref, watch, computed } from "vue";
import { Sheet, SheetContent, SheetHeader, SheetTitle } from "~/components/ui/sheet";
import { Skeleton } from "~/components/ui/skeleton";
import PermissionButton from "~/components/PermissionButton.vue";
import PlayerActionDialog from "~/components/player-actions/PlayerActionDialog.vue";
import PlayerPanelHeader from "./PlayerPanelHeader.vue";
import PlayerPanelSession from "./PlayerPanelSession.vue";
import PlayerPanelModerationSummary from "./PlayerPanelModerationSummary.vue";
import PlayerAlertBanner from "~/components/player-profile/PlayerAlertBanner.vue";
import PlayerRiskIndicators from "~/components/player-profile/PlayerRiskIndicators.vue";
import type { Player } from "~/types";
import type { PlayerProfile } from "~/types/player";
import { UI_PERMISSIONS } from "~/constants/permissions";
import type { PlayerAdminAction } from "~/composables/usePlayerAdminActions";

const props = defineProps<{
  player: Player | null;
  serverId: string;
}>();

const open = defineModel<boolean>("open", { default: false });

const emit = defineEmits<{
  (e: "action-completed"): void;
}>();

const authStore = useAuthStore();
const runtimeConfig = useRuntimeConfig();

const profile = ref<PlayerProfile | null>(null);
const profileLoading = ref(false);
const permissionDenied = ref(false);
const profileError = ref(false);
const { cblData, cblLoading, fetchCBLData, reset: resetCBL } = useCBL();

const showActionDialog = ref(false);
const pendingAction = ref<PlayerAdminAction | null>(null);

const isLive = computed(() => props.player?.sinceDisconnect === "");

async function loadProfile() {
  const p = props.player;
  if (!p) return;
  const id = p.steam_id || p.eosId || p.eos_id;

  profile.value = null;
  permissionDenied.value = false;
  profileError.value = false;
  resetCBL();

  if (!id) return;

  profileLoading.value = true;
  try {
    const data = await useAuthFetchImperative<{
      data: { player: PlayerProfile };
    }>(`${runtimeConfig.public.backendApi}/players/${id}`);
    profile.value = data.data.player;
    if (profile.value?.steam_id) {
      fetchCBLData(profile.value.steam_id);
    }
  } catch (err: any) {
    if (err?.response?.status === 403 || err?.statusCode === 403) {
      permissionDenied.value = true;
    } else {
      profileError.value = true;
    }
  } finally {
    profileLoading.value = false;
  }
}

watch(
  [open, () => props.player],
  ([isOpen, player], [wasOpen, oldPlayer]) => {
    if (isOpen && player && (!wasOpen || player !== oldPlayer)) {
      loadProfile();
    }
  }
);

function startAction(action: PlayerAdminAction) {
  pendingAction.value = action;
  showActionDialog.value = true;
}

function onActionCompleted() {
  emit("action-completed");
  // Refresh so the just-issued warn/kick/ban shows up in the history
  loadProfile();
}

const defaultTeamkillMetrics = {
  total_teamkills: 0,
  teamkills_per_session: 0,
  teamkill_ratio: 0,
  recent_teamkills: 0,
  total_team_wounds: 0,
  total_team_damage: 0,
  recent_team_wounds: 0,
  recent_team_damage: 0,
};
</script>

<template>
  <Sheet v-model:open="open">
    <SheetContent
      side="right"
      class="flex w-full flex-col gap-0 overflow-y-auto p-0 sm:max-w-[480px]"
    >
      <SheetHeader class="sr-only">
        <SheetTitle>Player details</SheetTitle>
      </SheetHeader>

      <template v-if="player">
        <div class="space-y-3 p-4 pb-24">
          <PlayerPanelHeader :player="player" :profile="profile" />
          <PlayerPanelSession :player="player" />

          <template v-if="profileLoading">
            <Skeleton class="h-24 w-full" />
            <Skeleton class="h-40 w-full" />
          </template>

          <template v-else-if="profile">
            <PlayerAlertBanner
              :active-bans="profile.active_bans || []"
              :risk-indicators="profile.risk_indicators || []"
            />
            <PlayerPanelModerationSummary
              :violations="profile.violations || []"
              :summary="
                profile.violation_summary || {
                  total_warns: 0,
                  total_kicks: 0,
                  total_bans: 0,
                }
              "
              :active-bans="profile.active_bans || []"
            />
            <PlayerRiskIndicators
              :risk-indicators="profile.risk_indicators || []"
              :violation-summary="
                profile.violation_summary || {
                  total_warns: 0,
                  total_kicks: 0,
                  total_bans: 0,
                }
              "
              :teamkill-metrics="
                profile.teamkill_metrics || defaultTeamkillMetrics
              "
              :cbl-data="cblData"
              :name-count="profile.name_history?.length || 1"
            />
            <p
              v-if="cblLoading"
              class="text-xs text-muted-foreground"
            >
              Checking Community Ban List…
            </p>
          </template>

          <p
            v-else-if="permissionDenied"
            class="rounded border bg-muted p-3 text-xs text-muted-foreground"
          >
            You don't have permission to view player history. Actions below are
            still available.
          </p>
          <p
            v-else-if="profileError"
            class="rounded border bg-muted p-3 text-xs text-muted-foreground"
          >
            No player history on record yet.
          </p>
        </div>

        <!-- Action bar -->
        <div
          class="sticky bottom-0 mt-auto flex flex-wrap justify-end gap-2 border-t bg-background p-3"
        >
          <PermissionButton
            v-if="isLive"
            :permission="UI_PERMISSIONS.PLAYERS_WARN"
            :server-id="serverId"
            variant="outline"
            size="sm"
            @click="startAction('warn')"
          >
            <Icon name="lucide:alert-triangle" class="mr-1 h-4 w-4" />
            Warn
          </PermissionButton>
          <PermissionButton
            v-if="isLive"
            :permission="UI_PERMISSIONS.PLAYERS_MOVE"
            :server-id="serverId"
            variant="outline"
            size="sm"
            @click="startAction('move')"
          >
            <Icon name="lucide:move" class="mr-1 h-4 w-4" />
            Move
          </PermissionButton>
          <PermissionButton
            v-if="isLive"
            :permission="UI_PERMISSIONS.PLAYERS_KICK"
            :server-id="serverId"
            :has-permission="
              authStore.hasPermission(serverId, UI_PERMISSIONS.PLAYERS_KICK) &&
              player.squadId != 0
            "
            :tooltip-message="
              player.squadId === 0 ? 'Player is not in a squad' : undefined
            "
            variant="outline"
            size="sm"
            @click="startAction('remove-from-squad')"
          >
            <Icon name="lucide:user-minus" class="mr-1 h-4 w-4" />
            Unsquad
          </PermissionButton>
          <PermissionButton
            v-if="isLive"
            :permission="UI_PERMISSIONS.PLAYERS_KICK"
            :server-id="serverId"
            variant="outline"
            size="sm"
            @click="startAction('kick')"
          >
            <Icon name="lucide:log-out" class="mr-1 h-4 w-4" />
            Kick
          </PermissionButton>
          <PermissionButton
            :permission="UI_PERMISSIONS.BANS_CREATE"
            :server-id="serverId"
            variant="destructive"
            size="sm"
            @click="startAction('ban')"
          >
            <Icon name="lucide:ban" class="mr-1 h-4 w-4" />
            Ban
          </PermissionButton>
        </div>
      </template>
    </SheetContent>
  </Sheet>

  <!-- Sibling of the Sheet (not nested in SheetContent) so the two portals
       don't fight over focus; the sheet stays open behind the dialog. -->
  <PlayerActionDialog
    v-model:open="showActionDialog"
    :action="pendingAction"
    :player="player"
    :server-id="serverId"
    @completed="onActionCompleted"
  />
</template>
