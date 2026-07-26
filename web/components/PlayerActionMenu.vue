<script setup lang="ts">
import { ref } from "vue";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import PermissionDropdownMenuItem from "@/components/PermissionDropdownMenuItem.vue";
import { Button } from "@/components/ui/button";
import PlayerActionDialog from "@/components/player-actions/PlayerActionDialog.vue";
import { toast } from "~/components/ui/toast";
import type { Player } from "~/types";
import { UI_PERMISSIONS } from "~/constants/permissions";
import {
  getPlayerEOSID,
  getPlayerProfileID,
  type PlayerAdminAction,
} from "~/composables/usePlayerAdminActions";

const authStore = useAuthStore();

const props = defineProps<{
  player: Player;
  serverId: string;
}>();

const emit = defineEmits<{
  (e: "action-completed"): void;
}>();

const showActionDialog = ref(false);
const pendingAction = ref<PlayerAdminAction | null>(null);

function openActionDialog(action: PlayerAdminAction) {
  pendingAction.value = action;
  showActionDialog.value = true;
}

// Function to copy text to clipboard
async function handleCopy(text: string) {
  if (!process.client) return;
  const ok = await copyToClipboard(text);
  if (ok) {
    toast({
      title: "Copied to clipboard",
      description: "The text has been copied to your clipboard.",
    });
  } else {
    toast({
      title: "Copy failed",
      description: "Unable to access the clipboard in this browser.",
      variant: "destructive",
    });
  }
}
</script>

<template>
  <DropdownMenu>
    <DropdownMenuTrigger asChild>
      <Button variant="outline" size="sm" class="h-8 w-8 p-0">
        <span class="sr-only">Open menu</span>
        <Icon name="lucide:more-vertical" class="h-4 w-4" />
      </Button>
    </DropdownMenuTrigger>
    <DropdownMenuContent align="end">
      <RouterLink :to="'/players/' + getPlayerProfileID(player)" as-child>
        <DropdownMenuItem>
          <Icon name="lucide:user" class="mr-2 h-4 w-4 text-blue-500" />
          <span>View Player Profile</span>
        </DropdownMenuItem>
      </RouterLink>
      <DropdownMenuItem @click="handleCopy(player.steam_id)">
        <Icon name="lucide:copy" class="mr-2 h-4 w-4 text-yellow-500" />
        <span>Copy Steam ID</span>
      </DropdownMenuItem>
      <DropdownMenuItem @click="handleCopy(getPlayerEOSID(player))">
        <Icon name="lucide:copy" class="mr-2 h-4 w-4 text-yellow-500" />
        <span>Copy EOS ID</span>
      </DropdownMenuItem>
      <template v-if="player.sinceDisconnect == ''">
        <PermissionDropdownMenuItem
          @click="openActionDialog('warn')"
          :permission="UI_PERMISSIONS.PLAYERS_WARN"
          :server-id="serverId as string"
        >
          <Icon
            name="lucide:alert-triangle"
            class="mr-2 h-4 w-4 text-yellow-500"
          />
          <span>Warn Player</span>
        </PermissionDropdownMenuItem>
        <PermissionDropdownMenuItem
          @click="openActionDialog('move')"
          :permission="UI_PERMISSIONS.PLAYERS_MOVE"
          :server-id="serverId as string"
        >
          <Icon name="lucide:move" class="mr-2 h-4 w-4 text-blue-500" />
          <span>Move to Other Team</span>
        </PermissionDropdownMenuItem>
        <PermissionDropdownMenuItem
          @click="openActionDialog('remove-from-squad')"
          :permission="UI_PERMISSIONS.PLAYERS_KICK"
          :server-id="serverId as string"
          :has-permission="
            authStore.hasPermission(
              serverId as string,
              UI_PERMISSIONS.PLAYERS_KICK,
            ) && player.squadId != 0
          "
          :tooltip-message="
            player.squadId === 0 ? 'Player is not in a squad' : undefined
          "
        >
          <Icon name="lucide:user-minus" class="mr-2 h-4 w-4 text-red-500" />
          <span>Remove from Squad</span>
        </PermissionDropdownMenuItem>
        <PermissionDropdownMenuItem
          @click="openActionDialog('kick')"
          :permission="UI_PERMISSIONS.PLAYERS_KICK"
          :server-id="serverId as string"
        >
          <Icon name="lucide:log-out" class="mr-2 h-4 w-4 text-orange-500" />
          <span>Kick Player</span>
        </PermissionDropdownMenuItem>
      </template>
      <PermissionDropdownMenuItem
        @click="openActionDialog('ban')"
        :permission="UI_PERMISSIONS.BANS_CREATE"
        :server-id="serverId as string"
      >
        <Icon name="lucide:ban" class="mr-2 h-4 w-4 text-red-500" />
        <span>Ban Player</span>
      </PermissionDropdownMenuItem>
    </DropdownMenuContent>
  </DropdownMenu>

  <PlayerActionDialog
    v-model:open="showActionDialog"
    :action="pendingAction"
    :player="player"
    :server-id="serverId"
    @completed="emit('action-completed')"
  />
</template>
