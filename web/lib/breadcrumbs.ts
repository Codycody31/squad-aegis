import { computed } from "vue";
import { useRoute } from "vue-router";
import { useServersStore } from "~/stores/servers";

const SEGMENT_LABELS: Record<string, string> = {
  dashboard: "Dashboard",
  servers: "Servers",
  players: "Players",
  "ban-lists": "Ban Lists",
  connectors: "Connectors",
  users: "Users",
  settings: "Settings",
  sudo: "Super Admin",
  "banned-players": "Banned Players",
  "teams-and-squads": "Teams & Squads",
  "disconnected-players": "Disconnected Players",
  "users-and-roles": "Users & Roles",
  "audit-logs": "Audit Logs",
  console: "Console",
  feeds: "Feeds",
  metrics: "Metrics",
  rules: "Rules",
  motd: "MOTD",
  plugins: "Plugins",
  workflows: "Workflows",
  executions: "Executions",
  alts: "Suspected Alts",
  logs: "Logs",
  storage: "Storage",
  system: "System Health",
  config: "Configuration",
  audit: "Audit Logs",
  sessions: "Sessions",
  database: "Database",
};

const UUID_RE =
  /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;

export function useBreadcrumbs() {
  const route = useRoute();
  const serversStore = useServersStore();

  return computed(() => {
    const segments = route.path.split("/").filter(Boolean);
    return segments.map((segment, index) => {
      const link = `/${segments.slice(0, index + 1).join("/")}`;
      const prev = segments[index - 1];

      let title: string;
      if (prev === "servers" && UUID_RE.test(segment)) {
        title =
          serversStore.servers.find((s) => s.id === segment)?.name ?? "Server";
      } else if (prev === "workflows" && UUID_RE.test(segment)) {
        title = "Workflow";
      } else if (prev === "executions" && UUID_RE.test(segment)) {
        title = "Execution";
      } else if (prev === "plugins" && !SEGMENT_LABELS[segment]) {
        title = "Plugin";
      } else if (prev === "players" && !SEGMENT_LABELS[segment]) {
        title = "Player";
      } else if (UUID_RE.test(segment)) {
        title = "Details";
      } else {
        title =
          SEGMENT_LABELS[segment] ??
          segment.charAt(0).toUpperCase() + segment.slice(1);
      }
      return { title, link };
    });
  });
}
