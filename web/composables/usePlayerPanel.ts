import { ref } from "vue";
import type { Player } from "~/types";

/** Per-page state for the slide-over player context panel. */
export function usePlayerPanel() {
  const isOpen = ref(false);
  const selected = ref<Player | null>(null);

  function openPanel(player: Player) {
    selected.value = player;
    isOpen.value = true;
  }

  return { isOpen, selected, openPanel };
}
