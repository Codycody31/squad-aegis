# Permission-Based Tooltips Implementation Guide

This guide explains how to use the permission tooltip system in Squad Aegis UI.

## Overview

The permission tooltip system provides user-friendly feedback when features are disabled due to insufficient permissions. Instead of hiding features (previous pattern), we now show them as disabled with explanatory tooltips.

## Components

### 1. PermissionButton
A button wrapper that automatically disables based on permissions and shows a tooltip.

```vue
<PermissionButton
  :permission="UI_PERMISSIONS.PLAYERS_KICK"
  :server-id="serverId"
  @click="kickPlayer"
>
  Kick Player
</PermissionButton>
```

**Props:**
- `permission` (required): The permission string (e.g., `UI_PERMISSIONS.PLAYERS_KICK`)
- `serverId` (optional): The server ID for server-specific permissions
- `hasPermission` (optional): Explicitly set permission state (overrides auto-check)
- `tooltipMessage` (optional): Custom tooltip message
- `variant`, `size`, `class`: Standard Button props
- `disabled`: Additional disabled state (independent of permissions)

### 2. PermissionDropdownMenuItem
For dropdown menu items that require permissions.

```vue
<PermissionDropdownMenuItem
  @click="handleAction"
  :permission="UI_PERMISSIONS.BANS_CREATE"
  :server-id="serverId"
>
  <Icon name="lucide:ban" class="mr-2 h-4 w-4" />
  <span>Ban Player</span>
</PermissionDropdownMenuItem>
```

**Props:** Same as PermissionButton

### 3. PermissionContextMenuItem
For context menu items that require permissions.

```vue
<PermissionContextMenuItem
  @click="handleAction"
  :permission="UI_PERMISSIONS.PLAYERS_WARN"
  :server-id="serverId"
>
  <Icon name="lucide:alert-triangle" class="mr-2 h-4 w-4" />
  <span>Warn Players</span>
</PermissionContextMenuItem>
```

**Props:** Same as PermissionButton

### 4. PermissionTooltip
Low-level wrapper for any element that needs a permission tooltip.

```vue
<PermissionTooltip
  :permission="UI_PERMISSIONS.CONSOLE_EXECUTE"
  :has-permission="canExecute"
>
  <CustomComponent :disabled="!canExecute" />
</PermissionTooltip>
```

## Utility Functions

### formatPermissionName(permission: string): string
Converts permission strings to human-readable names.

```typescript
formatPermissionName("ui:players:kick") // Returns: "Kick Players"
formatPermissionName("ui:*") // Returns: "All UI Permissions"
```

### getPermissionDeniedMessage(permission: string): string
Generates a user-friendly denial message.

```typescript
getPermissionDeniedMessage("ui:bans:create")
// Returns: 'This action requires the "Bans Create" permission. Please contact your system administrator to request access.'
```

## Migration from Old Pattern

### Before (hiding with v-if):
```vue
<DropdownMenuItem
  @click="kickPlayer"
  v-if="authStore.hasPermission(serverId, UI_PERMISSIONS.PLAYERS_KICK)"
>
  Kick Player
</DropdownMenuItem>
```

### After (showing disabled with tooltip):
```vue
<PermissionDropdownMenuItem
  @click="kickPlayer"
  :permission="UI_PERMISSIONS.PLAYERS_KICK"
  :server-id="serverId"
>
  Kick Player
</PermissionDropdownMenuItem>
```

## Complex Cases

### Combining Multiple Conditions
When you need both permission check AND another condition:

```vue
<PermissionDropdownMenuItem
  @click="removeFromSquad"
  :permission="UI_PERMISSIONS.PLAYERS_KICK"
  :server-id="serverId"
  :has-permission="authStore.hasPermission(serverId, UI_PERMISSIONS.PLAYERS_KICK) && player.squadId != 0"
  :tooltip-message="player.squadId === 0 ? 'Player is not in a squad' : undefined"
>
  Remove from Squad
</PermissionDropdownMenuItem>
```

### Custom Messages
```vue
<PermissionButton
  :permission="UI_PERMISSIONS.CONSOLE_EXECUTE"
  :server-id="serverId"
  tooltip-message="You need console execution rights. Contact admin@example.com to request access."
>
  Execute Command
</PermissionButton>
```

## Best Practices

1. **Always use server-id for server-specific permissions**: Most UI permissions are server-scoped
2. **Use descriptive custom messages when helpful**: For complex scenarios, clarify what the user needs
3. **Keep disabled state logic simple**: Let the component handle permission checking
4. **Don't combine v-if with permission components**: The whole point is to show (disabled) rather than hide

## Setup Required

The app must be wrapped in `TooltipProvider` (already done in app.vue):

```vue
<TooltipProvider>
  <NuxtLayout>
    <NuxtPage />
  </NuxtLayout>
</TooltipProvider>
```

## Permission Constants

All permissions are defined in `/web/constants/permissions.ts`:

- `UI_PERMISSIONS.*` - UI/frontend permissions
- `API_PERMISSIONS.*` - API endpoint permissions
- `RCON_PERMISSIONS.*` - Squad server RCON permissions

## Examples in Codebase

See these files for complete examples:
- `/web/components/PlayerActionMenu.vue` - DropdownMenu with permissions
- `/web/components/BulkPlayerActionMenu.vue` - ContextMenu with permissions
