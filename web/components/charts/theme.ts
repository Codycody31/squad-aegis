import { onUnmounted } from "vue";

const FALLBACK_COLOR = "hsl(0 0% 50%)";

const isDark = () =>
  typeof window !== "undefined" &&
  document.documentElement.classList.contains("dark");

/**
 * Reads a CSS custom property (storing space-separated HSL values, e.g.
 * `12 76% 61%`) from the document root and returns it wrapped as an
 * `hsl(...)` color string.
 */
export const readToken = (name: string): string => {
  if (typeof window === "undefined") {
    return FALLBACK_COLOR;
  }

  const value = getComputedStyle(document.documentElement)
    .getPropertyValue(name)
    .trim();

  if (!value) {
    return FALLBACK_COLOR;
  }

  return `hsl(${value})`;
};

export const chartPalette = (): string[] =>
  ["--chart-1", "--chart-2", "--chart-3", "--chart-4", "--chart-5"].map(
    (name) => readToken(name)
  );

export const chartAxisTextColor = (): string => readToken("--muted-foreground");

export const chartGridColor = (): string => readToken("--border");

export const chartTooltipDomStyles = () => ({
  "g2-tooltip": {
    background: readToken("--popover"),
    color: readToken("--popover-foreground"),
    border: `1px solid ${readToken("--border")}`,
    borderRadius: "8px",
    boxShadow: "0 4px 12px rgb(0 0 0 / 0.15)",
  },
});

/**
 * Semantic status colors (no dedicated CSS tokens exist for success/warning,
 * so these are theme-aware literals).
 */
export const statusColors = (): {
  good: string;
  warn: string;
  danger: string;
} =>
  isDark()
    ? { good: "#22c55e", warn: "#f59e0b", danger: "#ef4444" }
    : { good: "#16a34a", warn: "#d97706", danger: "#dc2626" };

/**
 * Calls `rebuild` whenever the `dark` class on <html> is toggled so charts
 * re-render with colors re-read from the active theme.
 */
export const useChartTheme = (rebuild: () => void) => {
  if (typeof window === "undefined") {
    return;
  }

  let wasDark = isDark();

  const observer = new MutationObserver(() => {
    const nowDark = isDark();

    if (nowDark !== wasDark) {
      wasDark = nowDark;
      rebuild();
    }
  });

  observer.observe(document.documentElement, {
    attributes: true,
    attributeFilter: ["class"],
  });

  onUnmounted(() => {
    observer.disconnect();
  });
};
