<script setup lang="ts">
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@/components/ui/tooltip";
import type { Permission } from "@/constants/permissions";
import { getPermissionDeniedMessage } from "@/lib/permissions";

interface Props {
  /**
   * The permission required for this action
   */
  permission?: Permission | string;
  /**
   * Custom message to display instead of auto-generated one
   */
  message?: string;
  /**
   * Whether the user has permission (if false, tooltip will be shown)
   */
  hasPermission: boolean;
  /**
   * Disable the tooltip even if user lacks permission
   */
  disabled?: boolean;
}

const props = defineProps<Props>();

const tooltipMessage = computed(() => {
  if (props.message) {
    return props.message;
  }
  if (props.permission) {
    return getPermissionDeniedMessage(props.permission);
  }
  return "You do not have permission to perform this action. Please contact your system administrator.";
});
</script>

<template>
  <!-- Only show tooltip if user lacks permission and not disabled -->
  <Tooltip v-if="!hasPermission && !disabled" :delay-duration="200">
    <TooltipTrigger as-child>
      <slot />
    </TooltipTrigger>
    <TooltipContent
      side="top"
      align="center"
      :side-offset="4"
      class="max-w-xs text-center"
    >
      {{ tooltipMessage }}
    </TooltipContent>
  </Tooltip>
  <!-- If user has permission or tooltip is disabled, just render the slot -->
  <slot v-else />
</template>
