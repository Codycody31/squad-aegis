import { defineStore } from "pinia";
import type { User } from "@/types";

export const useAuthStore = defineStore("auth", {
  state: () => {
    const user = ref<User | null>(null);
    const token = ref<string | null>(null);

    return {
      user,
      token,
    };
  },
  getters: {
    isLoggedIn: (state) => !!state.user,
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
      this.token = token;
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
    },
  },
});
