import type { Server } from "~/types";

export const useServersStore = defineStore("servers", {
  state: () => ({
    servers: [] as Server[],
    loaded: false,
    loading: false,
    activeServerId: null as string | null,
  }),
  getters: {
    activeServer(state): Server | null {
      return state.servers.find((s) => s.id === state.activeServerId) ?? null;
    },
  },
  actions: {
    async fetchServers(force = false) {
      if (this.loading || (this.loaded && !force)) return;
      this.loading = true;
      try {
        const runtimeConfig = useRuntimeConfig();
        const response = await useAuthFetchImperative<{
          data?: { servers: Server[] };
        }>(`${runtimeConfig.public.backendApi}/servers`);
        this.servers = response.data?.servers ?? [];
        this.loaded = true;
      } catch (error) {
        console.error("Error fetching servers:", error);
      } finally {
        this.loading = false;
      }
    },
    setActiveServerId(id: string | null) {
      this.activeServerId = id;
    },
  },
});
