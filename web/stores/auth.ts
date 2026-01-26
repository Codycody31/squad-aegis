import { defineStore } from "pinia";
import type { User } from "@/types";
import {
  PERMISSION_WILDCARD,
  type Permission,
} from "@/constants/permissions";

export const useAuthStore = defineStore("auth", {
  state: () => {
    const user = ref<User | null>(null);
    const token = ref<string | null>(null);
    const serverPermissions = ref<Record<string, string[]>>({});

    return {
      user,
      token,
      serverPermissions,
    };
  },
  getters: {
    isLoggedIn: (state) => !!state.user,
    isSuperAdmin: (state) => state.user?.super_admin ?? false,
  },
  actions: {
    async fetch() {
      const runtimeConfig = useRuntimeConfig();
      const cookieToken = useCookie(
        runtimeConfig.public.sessionCookieName as string
      );
      const token = cookieToken.value;

      if (!token) {
        return;
      }

      const { data, error } = await useFetch(
        `${runtimeConfig.public.backendApi}/auth/initial`,
        {
          headers: {
            Authorization: `Bearer ${token}`,
          },
        }
      );

      if (error.value && error.value.statusCode === 401) {
        this.user = null;
        this.token = null;
        document.cookie = `${runtimeConfig.public.sessionCookieName}=; Path=/; Expires=Thu, 01 Jan 1970 00:00:01 GMT;`;
        navigateTo("/login");
        return;
      }

      this.user = data.value?.data.user as User;
      this.serverPermissions = data.value?.data
        .serverPermissions as Record<string, string[]>;
      this.token = token;
    },

    // Get raw permissions array for a server
    getServerPermissions(serverId: string): string[] {
      return this.serverPermissions[serverId] ?? [];
    },

    // Check if user has a specific permission for a server
    hasPermission(serverId: string, permission: Permission | string): boolean {
      // Super admins have all permissions
      if (this.isSuperAdmin) return true;

      const perms = this.serverPermissions[serverId];
      if (!perms || !Array.isArray(perms)) return false;

      return this.evaluatePermission(perms, permission);
    },

    // Check if user has any of the specified permissions
    hasAnyPermission(
      serverId: string,
      ...permissions: (Permission | string)[]
    ): boolean {
      if (this.isSuperAdmin) return true;

      const userPerms = this.serverPermissions[serverId];
      if (!userPerms || !Array.isArray(userPerms)) return false;

      return permissions.some((p) => this.evaluatePermission(userPerms, p));
    },

    // Check if user has all specified permissions
    hasAllPermissions(
      serverId: string,
      ...permissions: (Permission | string)[]
    ): boolean {
      if (this.isSuperAdmin) return true;

      const userPerms = this.serverPermissions[serverId];
      if (!userPerms || !Array.isArray(userPerms)) return false;

      return permissions.every((p) => this.evaluatePermission(userPerms, p));
    },

    // Evaluate if a permission is granted based on user's permissions
    evaluatePermission(
      userPermissions: string[],
      required: Permission | string
    ): boolean {
      for (const p of userPermissions) {
        // Wildcard grants all
        if (p === PERMISSION_WILDCARD) return true;
        // Exact match
        if (p === required) return true;
        // Category wildcard (e.g., "ui:*" grants all UI permissions)
        if (p.endsWith(":*")) {
          const prefix = p.slice(0, -1);
          if (required.startsWith(prefix)) return true;
        }
      }
      return false;
    },

    // DEPRECATED: Use hasPermission instead
    // Kept for backward compatibility
    getServerPermission(serverId: string, permission: string): boolean {
      return this.hasPermission(serverId, permission);
    },

    setUser(user: User) {
      this.user = user;
    },

    setToken(token: string) {
      this.token = token;
    },

    logout() {
      this.user = null;
      this.token = null;
      this.serverPermissions = {};
    },
  },
});
