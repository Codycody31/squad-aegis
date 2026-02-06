<script setup lang="ts">
import {
  DropdownMenuItem,
} from "@/components/ui/dropdown-menu";
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@/components/ui/tooltip";
import type { Permission } from "@/constants/permissions";
import { getPermissionDeniedMessage } from "@/lib/permissions";
import type { HTMLAttributes } from "vue";

interface Props {
  /**
   * The permission required for this action
   */
  permission: Permission | string;
  /**
   * The server ID to check permissions against
   */
  serverId?: string;
  /**
   * Whether the user has the required permission
   * If not provided, will use authStore to check
   */
  hasPermission?: boolean | null;
  /**
   * Custom tooltip message to display when permission is denied
   */
  tooltipMessage?: string;
  /**
   * Additional classes
   */
  class?: HTMLAttributes["class"];
  /**
   * Inset padding for nested items
   */
  inset?: boolean;
}

const props = withDefaults(defineProps<Props>(), {
  hasPermission: null,
});

const authStore = useAuthStore();

// Determine if user has permission
const userHasPermission = computed(() => {
  // If hasPermission prop is explicitly provided, use it
  if (props.hasPermission != null) {
    return props.hasPermission;
  }

  // Otherwise check via authStore
  if (!props.serverId) {
    // If no serverId, check if super admin
    return authStore.isSuperAdmin;
  }

  return authStore.hasPermission(props.serverId, props.permission);
});

const tooltipMessage = computed(() => {
  if (props.tooltipMessage) {
    return props.tooltipMessage;
  }
  return getPermissionDeniedMessage(props.permission);
});
</script>

<template>
  <!-- Wrap in tooltip only if user lacks permission -->
  <Tooltip v-if="!userHasPermission" :delay-duration="200">
    <TooltipTrigger as-child>
      <DropdownMenuItem
        :disabled="!userHasPermission"
        :class="class"
        :inset="inset"
      >
        <slot />
      </DropdownMenuItem>
    </TooltipTrigger>
    <TooltipContent
      side="right"
      align="center"
      :side-offset="8"
      class="max-w-xs text-center"
    >
      {{ tooltipMessage }}
    </TooltipContent>
  </Tooltip>
  <!-- If user has permission, just render the item normally -->
  <DropdownMenuItem
    v-else
    :disabled="false"
    :class="class"
    :inset="inset"
  >
    <slot />
  </DropdownMenuItem>
</template>
