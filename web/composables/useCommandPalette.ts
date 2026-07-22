import { ref } from "vue";

const isOpen = ref(false);

/** Shared open state for the global command palette (Ctrl/Cmd+K). */
export function useCommandPalette() {
  return {
    isOpen,
    open: () => {
      isOpen.value = true;
    },
    toggle: () => {
      isOpen.value = !isOpen.value;
    },
  };
}
