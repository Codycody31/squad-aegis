<script setup lang="ts">
import { Button } from "@/components/ui/button";
import PermissionTooltip from "@/components/PermissionTooltip.vue";
import type { Permission } from "@/constants/permissions";
import type { ButtonVariants } from "@/components/ui/button";

interface Props {
  /**
   * The permission required for this button action
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
  hasPermission?: boolean;
  /**
   * Custom tooltip message to display when permission is denied
   */
  tooltipMessage?: string;
  /**
   * Button variant
   */
  variant?: ButtonVariants["variant"];
  /**
   * Button size
   */
  size?: ButtonVariants["size"];
  /**
   * Additional classes
   */
  class?: string;
  /**
   * Additional disabled state (independent of permissions)
   */
  disabled?: boolean;
  /**
   * Button type attribute
   */
  type?: "button" | "submit" | "reset";
}

const props = defineProps<Props>();

const authStore = useAuthStore();

// Determine if user has permission
const userHasPermission = computed(() => {
  // If hasPermission prop is explicitly provided, use it
  if (props.hasPermission !== undefined) {
    return props.hasPermission;
  }

  // Otherwise check via authStore
  if (!props.serverId) {
    // If no serverId, check if super admin
    return authStore.isSuperAdmin;
  }

  return authStore.hasPermission(props.serverId, props.permission);
});

// Button should be disabled if user lacks permission OR if disabled prop is true
const isDisabled = computed(() => {
  return !userHasPermission.value || props.disabled;
});
</script>

<template>
  <PermissionTooltip
    :permission="permission"
    :message="tooltipMessage"
    :has-permission="userHasPermission"
  >
    <Button
      :variant="variant"
      :size="size"
      :class="class"
      :disabled="isDisabled"
      :type="type"
    >
      <slot />
    </Button>
  </PermissionTooltip>
</template>
