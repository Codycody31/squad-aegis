import { getPermissionCategory } from "@/constants/permissions";

// Shared types for the Users & Roles page components

export interface PermissionDefinition {
  id: string;
  code: string;
  category: string;
  name: string;
  description: string;
  squad_permission?: string;
}

export interface PermissionGroup {
  category: string;
  category_name: string;
  permissions: PermissionDefinition[];
}

export interface RoleTemplate {
  id: string;
  name: string;
  description: string;
  is_system: boolean;
  is_admin: boolean;
  permissions: string[];
}

export interface ServerRole {
  id: string;
  serverId: string;
  name: string;
  permissions: string[];
  is_admin: boolean;
  created_at: string;
}

export interface ServerAdmin {
  id: string;
  server_id: string;
  user_id?: string;
  steam_id?: string;
  eos_id?: string;
  username: string;
  server_role_id: string;
  expires_at?: string;
  notes?: string;
  is_active?: boolean;
  is_expired?: boolean;
  created_at: string;
}

export interface User {
  id: string;
  username: string;
  email: string;
  superAdmin: boolean;
  created_at: string;
  name?: string;
}

// Format date
export function formatDate(dateString: string): string {
  return new Date(dateString).toLocaleString();
}

// Format permission for display
export function formatPermissionDisplay(
  permCode: string,
  groups: PermissionGroup[]
): string {
  // Try to find in loaded permissions by exact code
  for (const group of groups) {
    const perm = group.permissions.find((p) => p.code === permCode);
    if (perm) {
      return perm.name;
    }
  }

  // For old-style permissions (without prefix), try to find matching rcon permission
  if (!permCode.includes(":")) {
    const rconCode = `rcon:${permCode.toLowerCase()}`;
    for (const group of groups) {
      const perm = group.permissions.find((p) => p.code === rconCode);
      if (perm) {
        return perm.name;
      }
    }
    // Still not found - capitalize the old permission name
    return permCode.charAt(0).toUpperCase() + permCode.slice(1);
  }

  // New format with colon - extract the part after category
  const parts = permCode.split(":");
  if (parts.length >= 2) {
    return parts.slice(1).join(" ").replace(/_/g, " ");
  }
  return permCode;
}

// Get permission category badge color
export function getPermissionCategoryColor(permCode: string): string {
  const category = getPermissionCategory(permCode);
  switch (category) {
    case "ui":
      return "bg-blue-100 text-blue-800 dark:bg-blue-900 dark:text-blue-200";
    case "rcon":
      return "bg-purple-100 text-purple-800 dark:bg-purple-900 dark:text-purple-200";
    default:
      return "bg-gray-100 text-gray-800 dark:bg-gray-900 dark:text-gray-200";
  }
}
