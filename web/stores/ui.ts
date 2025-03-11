import type { Server } from "~/types";

export const useUiStore = defineStore("ui", {
  state: () => {
    const activeServer = ref<Server | null>(null);

    return {
      activeServer,
    };
  },
  actions: {
    setActiveServer(server: Server | null) {
      this.activeServer = server;
    },
  },
});
