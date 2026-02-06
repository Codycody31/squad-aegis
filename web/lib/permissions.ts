import type { Permission } from "@/constants/permissions";

/**
 * Format a permission string into a human-readable display name
 * Example: "ui:players:kick" -> "Kick Players"
 */
export function formatPermissionName(permission: Permission | string): string {
  // Handle wildcard
  if (permission === "*") {
    return "Full Access";
  }

  // Split permission by colons
  const parts = permission.split(":");

  if (parts.length < 2) {
    return permission;
  }

  // Get the category (ui, api, rcon)
  const category = parts[0];

  // Get the action parts (everything after category)
  const actionParts = parts.slice(1);

  // Handle wildcards like "ui:*"
  if (actionParts.length === 1 && actionParts[0] === "*") {
    const categoryNames: Record<string, string> = {
      ui: "All UI Permissions",
      api: "All API Permissions",
      rcon: "All RCON Permissions",
      system: "All System Permissions",
    };
    return categoryNames[category] || "Full Access";
  }

  // Convert snake_case or kebab-case to Title Case
  const formatted = actionParts
    .map((part) =>
      part
        .replace(/_/g, " ")
        .replace(/-/g, " ")
        .split(" ")
        .map((word) => word.charAt(0).toUpperCase() + word.slice(1))
        .join(" ")
    )
    .join(" ");

  return formatted;
}

/**
 * Get a user-friendly description of what permission is required
 * @param permission The permission that is required
 * @returns A message to display to users
 */
export function getPermissionDeniedMessage(permission: Permission | string): string {
  const permissionName = formatPermissionName(permission);
  return `This action requires the "${permissionName}" permission. Please contact your system administrator to request access.`;
}

/**
 * Get a user-friendly description for multiple permissions
 * @param permissions Array of permissions required
 * @returns A message to display to users
 */
export function getMultiplePermissionsDeniedMessage(
  permissions: (Permission | string)[]
): string {
  if (permissions.length === 0) {
    return "This action requires additional permissions. Please contact your system administrator.";
  }

  if (permissions.length === 1) {
    return getPermissionDeniedMessage(permissions[0]);
  }

  const formattedPermissions = permissions.map(formatPermissionName);
  const lastPermission = formattedPermissions.pop();

  return `This action requires one of the following permissions: ${formattedPermissions.join(", ")} or ${lastPermission}. Please contact your system administrator to request access.`;
}
