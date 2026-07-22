<script setup lang="ts">
import { ref, watch, computed, onMounted, onBeforeUnmount, nextTick } from "vue";
import { Input } from "~/components/ui/input";
import { Button } from "~/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from "~/components/ui/dialog";
import { Card, CardContent, CardHeader, CardTitle } from "~/components/ui/card";
import PageHeader from "~/components/layout/PageHeader.vue";
import PermissionButton from "~/components/PermissionButton.vue";
import BroadcastDialog from "~/components/BroadcastDialog.vue";
import { UI_PERMISSIONS } from "~/constants/permissions";

definePageMeta({ middleware: ["auth"] });

const route = useRoute();
const serverId = route.params.serverId as string;

const loading = ref(false);
const command = ref("");
const output = ref("");
const suggestions = ref<CommandInfo[]>([]);
const activeFilter = ref("all"); // "all", "admin", or "public"
const selectedCommand = ref<CommandInfo | null>(null);
const showAllCommands = ref(false);
const allCommands = ref<CommandInfo[]>([]);
const commandsDialogFilter = ref("all");
const searchQuery = ref("");

// Command history tracking
interface CommandHistoryEntry {
  command: string;
  response: string;
  timestamp: Date;
}

const commandHistory = ref<CommandHistoryEntry[]>([]);
const showCommandHistory = ref(false);
const showBroadcastDialog = ref(false);

const HISTORY_LIMIT = 100;
const historyStorageKey = `aegis:console:${serverId}:history`;

function persistCommandHistory() {
  if (!import.meta.client) return;
  try {
    localStorage.setItem(
      historyStorageKey,
      JSON.stringify(commandHistory.value.slice(0, HISTORY_LIMIT))
    );
  } catch {
    // Storage unavailable or full — history just won't survive reloads.
  }
}

function hydrateCommandHistory() {
  if (!import.meta.client) return;
  try {
    const raw = localStorage.getItem(historyStorageKey);
    if (!raw) return;
    const parsed = JSON.parse(raw);
    if (!Array.isArray(parsed)) return;
    commandHistory.value = parsed
      .filter((entry) => entry && typeof entry.command === "string")
      .slice(0, HISTORY_LIMIT)
      .map((entry) => ({
        command: entry.command,
        response: typeof entry.response === "string" ? entry.response : "",
        timestamp: entry.timestamp ? new Date(entry.timestamp) : new Date(),
      }));
  } catch {
    // Corrupt stored history — start fresh.
  }
}

onMounted(hydrateCommandHistory);

// CommandType enum to match backend commands package
enum CommandType {
  PublicCommand = 0,
  AdminCommand = 1,
}

interface CommandResponse {
  data: {
    response: string;
  };
}

interface CommandInfo {
  SupportsRCON: boolean;
  Name: string;
  Category: string;
  Syntax: string;
  Description: string;
  CommandType: CommandType;
}

interface CommandsResponse {
  data: {
    commands: CommandInfo[];
  };
}

// Computed properties for filtered suggestions
const filteredSuggestions = computed(() => {
  if (activeFilter.value === "all") {
    return suggestions.value;
  } else if (activeFilter.value === "admin") {
    return suggestions.value.filter(
      (cmd) => cmd.CommandType === CommandType.AdminCommand
    );
  } else {
    return suggestions.value.filter(
      (cmd) => cmd.CommandType === CommandType.PublicCommand
    );
  }
});

// Computed property for filtered all commands
const filteredAllCommands = computed(() => {
  let filtered = allCommands.value;

  // Apply type filter
  if (commandsDialogFilter.value === "admin") {
    filtered = filtered.filter(
      (cmd) => cmd.CommandType === CommandType.AdminCommand
    );
  } else if (commandsDialogFilter.value === "public") {
    filtered = filtered.filter(
      (cmd) => cmd.CommandType === CommandType.PublicCommand
    );
  }

  // Apply search filter if there's a search query
  if (searchQuery.value.trim()) {
    const query = searchQuery.value.toLowerCase();
    filtered = filtered.filter(
      (cmd) =>
        cmd.Name.toLowerCase().includes(query) ||
        cmd.Description.toLowerCase().includes(query) ||
        cmd.Category.toLowerCase().includes(query)
    );
  }

  return filtered;
});

async function executeCommand() {
  if (!command.value.trim()) return;

  loading.value = true;

  const runtimeConfig = useRuntimeConfig();

  const { data, error } = await useAuthFetch<CommandResponse>(
    `${runtimeConfig.public.backendApi}/servers/${serverId}/rcon/execute`,
    {
      method: "POST",
      body: {
        command: command.value,
      },
    }
  );

  let responseText = "No response from server";

  if (error.value) {
    responseText = `Error: ${error.value.message || "Unknown error"}`;
  }

  if (data.value && data.value.data && data.value.data.response) {
    responseText = data.value.data.response;
  }

  output.value = responseText;

  // Add to command history
  commandHistory.value.unshift({
    command: command.value,
    response: responseText,
    timestamp: new Date(),
  });

  // Limit history size to prevent memory issues
  if (commandHistory.value.length > HISTORY_LIMIT) {
    commandHistory.value = commandHistory.value.slice(0, HISTORY_LIMIT);
  }

  persistCommandHistory();

  loading.value = false;
}

async function fetchSuggestions(query: string) {
  if (!query) {
    suggestions.value = [];
    selectedCommand.value = null;
    return;
  }

  const runtimeConfig = useRuntimeConfig();

  const { data, error } = await useAuthFetch<CommandsResponse>(
    `${runtimeConfig.public.backendApi}/servers/${serverId}/rcon/commands/autocomplete?q=${query}`,
    {
      method: "GET",
    }
  );

  if (data.value && data.value.data && data.value.data.commands) {
    const commandsList = data.value.data.commands;

    // Check if the first part of the input exactly matches a command
    // Split the input by space to get the first word
    const firstWord = query.split(" ")[0];

    // If the first word exactly matches any command, hide suggestions
    const exactMatch = commandsList.some(
      (cmd) => cmd.Name.toLowerCase() === firstWord.toLowerCase()
    );

    if (exactMatch) {
      // Find the exact match command to display its details
      const matchedCommand = commandsList.find(
        (cmd) => cmd.Name.toLowerCase() === firstWord.toLowerCase()
      );
      if (matchedCommand) {
        selectedCommand.value = matchedCommand;
      }
      suggestions.value = [];
    } else {
      suggestions.value = commandsList;
      selectedCommand.value = null;
    }
  }
}

// Function to select a command from suggestions
function selectCommand(cmd: CommandInfo) {
  command.value = cmd.Name;
  selectedCommand.value = cmd;
  suggestions.value = [];
}

watch(command, (newCommand) => {
  fetchSuggestions(newCommand);
});

// Function to fetch all commands
async function fetchAllCommands() {
  const runtimeConfig = useRuntimeConfig();

  const { data, error } = await useAuthFetch<CommandsResponse>(
    `${runtimeConfig.public.backendApi}/servers/${serverId}/rcon/commands`,
    {
      method: "GET",
    }
  );

  if (data.value && data.value.data && data.value.data.commands) {
    allCommands.value = data.value.data.commands;
  }
}

// Function to open the all commands dialog
function openAllCommandsDialog() {
  fetchAllCommands();
  showAllCommands.value = true;
}

// Function to select a command from the dialog
function selectCommandFromDialog(cmd: CommandInfo) {
  command.value = cmd.Name;
  selectedCommand.value = cmd;
  showAllCommands.value = false;
}

// Function to reuse a command from history
function reuseCommand(historyEntry: CommandHistoryEntry) {
  command.value = historyEntry.command;
  showCommandHistory.value = false;
}

// Function to clear command history
function clearCommandHistory() {
  commandHistory.value = [];
  showCommandHistory.value = false;
  if (import.meta.client) {
    try {
      localStorage.removeItem(historyStorageKey);
    } catch {
      // Storage unavailable — nothing to clear.
    }
  }
}

// ---- Live server events (SSE) ----

interface LiveEventLine {
  id: number;
  time: string;
  type: string;
  body: string;
}

const runtimeConfig = useRuntimeConfig();
const sessionToken = useCookie<string | null>(
  runtimeConfig.public.sessionCookieName
);

const EVENT_BUFFER_LIMIT = 500;
const eventsEnabled = ref(false);
const eventsStatus = ref<"off" | "live" | "reconnecting">("off");
const liveEvents = ref<LiveEventLine[]>([]);
const eventsContainer = ref<HTMLElement | null>(null);
let eventSource: EventSource | null = null;
let eventSeq = 0;

const EVENT_TYPE_LABELS: Record<string, string> = {
  CHAT_MESSAGE: "CHAT",
  CHAT_COMMAND: "CMD",
  PLAYER_WARNED: "WARN",
  PLAYER_KICKED: "KICK",
  POSSESSED_ADMIN_CAMERA: "CAM+",
  UNPOSSESSED_ADMIN_CAMERA: "CAM-",
  SQUAD_CREATED: "SQUAD",
  CONNECTION_CLOSED: "CONN",
  CONNECTION_ERROR: "ERROR",
};

const EVENT_TYPE_STYLES: Record<string, string> = {
  CHAT_MESSAGE: "event-type-blue",
  CHAT_COMMAND: "event-type-blue",
  PLAYER_WARNED: "event-type-orange",
  PLAYER_KICKED: "event-type-red",
  POSSESSED_ADMIN_CAMERA: "event-type-purple",
  UNPOSSESSED_ADMIN_CAMERA: "event-type-purple",
  SQUAD_CREATED: "event-type-green",
  CONNECTION_CLOSED: "event-type-red",
  CONNECTION_ERROR: "event-type-red",
};

function eventTypeLabel(type: string): string {
  return EVENT_TYPE_LABELS[type] ?? type;
}

function eventTypeClass(type: string): string {
  return EVENT_TYPE_STYLES[type] ?? "event-type-gray";
}

function formatEventTime(value: unknown): string {
  let d = new Date();
  if (typeof value === "string" || typeof value === "number") {
    const parsed = new Date(value);
    if (!Number.isNaN(parsed.getTime())) d = parsed;
  }
  const pad = (n: number) => String(n).padStart(2, "0");
  return `${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`;
}

function eventField(
  data: Record<string, unknown>,
  key: string
): string | null {
  const value = data[key];
  return typeof value === "string" && value.length > 0 ? value : null;
}

function formatEventBody(type: string, data: unknown): string {
  const fallback = () => {
    if (data === undefined || data === null) return "";
    try {
      return typeof data === "string" ? data : JSON.stringify(data);
    } catch {
      return String(data);
    }
  };

  // Connection events carry arbitrary payloads (error string, object, or nothing).
  if (type === "CONNECTION_CLOSED" || type === "CONNECTION_ERROR") {
    const label =
      type === "CONNECTION_CLOSED"
        ? "RCON event connection closed"
        : "RCON event connection error";
    const detail = fallback();
    return detail && detail !== "{}" && detail !== "null"
      ? `${label} — ${detail}`
      : label;
  }

  if (typeof data !== "object" || data === null || Array.isArray(data)) {
    return fallback();
  }
  const d = data as Record<string, unknown>;

  switch (type) {
    case "CHAT_MESSAGE":
    case "CHAT_COMMAND": {
      const player = eventField(d, "PlayerName");
      const message = eventField(d, "Message");
      if (!player || !message) return fallback();
      const chat = eventField(d, "ChatType");
      return `${chat ? `[${chat}] ` : ""}${player}: ${message}`;
    }
    case "PLAYER_WARNED": {
      const player = eventField(d, "PlayerName");
      if (!player) return fallback();
      const message = eventField(d, "Message");
      return message ? `${player} warned: ${message}` : `${player} warned`;
    }
    case "PLAYER_KICKED": {
      const player = eventField(d, "PlayerName");
      if (!player) return fallback();
      const steamId = eventField(d, "SteamID");
      return `${player} kicked${steamId ? ` (${steamId})` : ""}`;
    }
    case "POSSESSED_ADMIN_CAMERA":
    case "UNPOSSESSED_ADMIN_CAMERA": {
      const admin = eventField(d, "AdminName");
      if (!admin) return fallback();
      return type === "POSSESSED_ADMIN_CAMERA"
        ? `${admin} entered admin camera`
        : `${admin} left admin camera`;
    }
    case "SQUAD_CREATED": {
      const player = eventField(d, "PlayerName");
      const squadName = eventField(d, "SquadName");
      if (!player || !squadName) return fallback();
      const squadId = eventField(d, "SquadID");
      const team = eventField(d, "TeamName");
      return `${player} created squad ${
        squadId ? `${squadId} ` : ""
      }"${squadName}"${team ? ` (${team})` : ""}`;
    }
    default:
      return fallback();
  }
}

function appendLiveEvent(raw: unknown) {
  if (typeof raw !== "object" || raw === null) return;
  const ev = raw as Record<string, unknown>;
  const type = typeof ev.Type === "string" ? ev.Type : "UNKNOWN";

  const el = eventsContainer.value;
  const wasAtBottom =
    !el || el.scrollHeight - el.scrollTop - el.clientHeight < 24;

  liveEvents.value.push({
    id: eventSeq++,
    time: formatEventTime(ev.Time),
    type,
    body: formatEventBody(type, ev.Data),
  });
  if (liveEvents.value.length > EVENT_BUFFER_LIMIT) {
    liveEvents.value.splice(0, liveEvents.value.length - EVENT_BUFFER_LIMIT);
  }

  if (wasAtBottom) {
    nextTick(() => {
      const node = eventsContainer.value;
      if (node) node.scrollTop = node.scrollHeight;
    });
  }
}

function connectEvents() {
  if (eventSource) return;
  const token = sessionToken.value;
  if (!token) {
    eventsEnabled.value = false;
    eventsStatus.value = "off";
    return;
  }

  eventsEnabled.value = true;
  eventsStatus.value = "reconnecting";

  const url = `${
    runtimeConfig.public.backendApi
  }/servers/${serverId}/rcon/events?token=${encodeURIComponent(token)}`;
  eventSource = new EventSource(url);
  eventSource.onopen = () => {
    eventsStatus.value = "live";
  };
  eventSource.onmessage = (message) => {
    eventsStatus.value = "live";
    try {
      appendLiveEvent(JSON.parse(message.data));
    } catch {
      // Not JSON — skip the line.
    }
  };
  eventSource.onerror = () => {
    // EventSource reconnects on its own; just surface the state.
    eventsStatus.value = "reconnecting";
  };
}

function disconnectEvents() {
  if (eventSource) {
    eventSource.close();
    eventSource = null;
  }
  eventsEnabled.value = false;
  eventsStatus.value = "off";
}

function toggleEvents() {
  if (eventsEnabled.value) {
    disconnectEvents();
  } else {
    connectEvents();
  }
}

onBeforeUnmount(disconnectEvents);
</script>

<template>
  <div class="console-page p-4 md:p-6">
    <PageHeader title="Console" />

    <!-- Replace the command input section with a responsive layout -->
    <div class="command-input-container mb-4">
      <Input
        v-model="command"
        placeholder="Enter RCON command"
        class="w-full mb-2"
      />
      <div class="command-buttons-container">
        <Button @click="executeCommand" :disabled="loading" class="command-button">
          {{ loading ? "Executing..." : "Execute" }}
        </Button>
        <Button @click="openAllCommandsDialog" variant="outline" class="command-button">
          List All Commands
        </Button>
        <Button
          @click="showCommandHistory = true"
          variant="outline"
          :disabled="commandHistory.length === 0"
          class="command-button"
        >
          History
        </Button>
        <PermissionButton
          :permission="UI_PERMISSIONS.CONSOLE_EXECUTE"
          :server-id="serverId"
          variant="outline"
          class="command-button"
          @click="showBroadcastDialog = true"
        >
          <Icon name="mdi:bullhorn" class="mr-2 h-4 w-4" />
          Broadcast
        </PermissionButton>
      </div>
    </div>

    <div v-if="selectedCommand" class="selected-command-info mb-4">
      <div class="command-name font-bold">{{ selectedCommand.Name }}</div>
      <div class="command-syntax text-sm">
        Syntax: {{ selectedCommand.Syntax }}
      </div>
      <div class="command-description">{{ selectedCommand.Description }}</div>
      <div class="command-type text-xs mt-1">
        <span
          :class="{
            'text-red-500': selectedCommand.CommandType === 1,
            'text-green-500': selectedCommand.CommandType === 0,
          }"
        >
          {{
            selectedCommand.CommandType === 1
              ? "Admin Command"
              : "Public Command"
          }}
        </span>
      </div>
    </div>

    <div v-if="suggestions.length" class="filter-buttons mb-4">
      <Button
        @click="activeFilter = 'all'"
        :class="{ active: activeFilter === 'all' }"
        variant="outline"
        class="mr-2"
      >
        All
      </Button>
      <Button
        @click="activeFilter = 'admin'"
        :class="{ active: activeFilter === 'admin' }"
        variant="outline"
        class="mr-2"
      >
        Admin
      </Button>
      <Button
        @click="activeFilter = 'public'"
        :class="{ active: activeFilter === 'public' }"
        variant="outline"
      >
        Public
      </Button>
    </div>

    <ul v-if="filteredSuggestions.length" class="suggestions-list">
      <li
        v-for="suggestion in filteredSuggestions"
        :key="suggestion.Name"
        @click="selectCommand(suggestion)"
        :class="{
          'admin-command': suggestion.CommandType === 1,
          'public-command': suggestion.CommandType === 0,
        }"
      >
        <div class="command-name">{{ suggestion.Name }}</div>
        <div class="command-description">{{ suggestion.Description }}</div>
        <div class="command-syntax text-xs opacity-70">
          {{ suggestion.Syntax }}
        </div>
      </li>
    </ul>

    <div class="terminal-output mt-4">
      <pre class="console-pre">{{ output }}</pre>
    </div>

    <!-- Live server events -->
    <Card class="mt-4">
      <CardHeader class="pb-2">
        <div class="flex flex-wrap items-center justify-between gap-2">
          <CardTitle class="flex items-center gap-3 text-base">
            Live server events
            <span
              class="flex items-center gap-1.5 text-xs font-normal text-muted-foreground"
            >
              <span
                class="live-status-dot"
                :class="{
                  'live-status-live': eventsStatus === 'live',
                  'live-status-reconnecting': eventsStatus === 'reconnecting',
                  'live-status-off': eventsStatus === 'off',
                }"
              />
              {{
                eventsStatus === "live"
                  ? "Live"
                  : eventsStatus === "reconnecting"
                    ? "Reconnecting…"
                    : "Off"
              }}
            </span>
          </CardTitle>
          <Button variant="outline" size="sm" @click="toggleEvents">
            {{ eventsEnabled ? "Disconnect" : "Connect" }}
          </Button>
        </div>
        <p class="text-xs text-muted-foreground">
          Server events (chat, admin actions) — executed commands are not
          echoed here.
        </p>
      </CardHeader>
      <CardContent>
        <div ref="eventsContainer" class="live-events-container">
          <div
            v-if="liveEvents.length === 0"
            class="py-4 text-center text-sm opacity-70"
          >
            {{
              eventsEnabled
                ? "Waiting for events…"
                : "Connect to stream live server events."
            }}
          </div>
          <div v-for="ev in liveEvents" :key="ev.id" class="live-event-line">
            <span class="live-event-time">{{ ev.time }}</span>
            <span class="live-event-type" :class="eventTypeClass(ev.type)">{{
              eventTypeLabel(ev.type)
            }}</span>
            <span class="live-event-body">{{ ev.body }}</span>
          </div>
        </div>
      </CardContent>
    </Card>

    <BroadcastDialog
      v-model:open="showBroadcastDialog"
      :server-id="serverId"
    />

    <!-- All Commands Dialog -->
    <Dialog :open="showAllCommands" @update:open="showAllCommands = $event">
      <DialogContent class="max-w-4xl max-h-[80vh] overflow-hidden">
        <DialogHeader>
          <DialogTitle>All Available Commands</DialogTitle>
        </DialogHeader>

        <div class="mt-4 flex flex-col sm:flex-row sm:items-center space-y-2 sm:space-y-0 sm:space-x-2">
          <Input
            v-model="searchQuery"
            placeholder="Search commands..."
            class="w-full sm:flex-grow"
          />

          <div class="filter-buttons flex flex-wrap gap-2">
            <Button
              @click="commandsDialogFilter = 'all'"
              :class="{ active: commandsDialogFilter === 'all' }"
              variant="outline"
              size="sm"
            >
              All
            </Button>
            <Button
              @click="commandsDialogFilter = 'admin'"
              :class="{ active: commandsDialogFilter === 'admin' }"
              variant="outline"
              size="sm"
            >
              Admin
            </Button>
            <Button
              @click="commandsDialogFilter = 'public'"
              :class="{ active: commandsDialogFilter === 'public' }"
              variant="outline"
              size="sm"
            >
              Public
            </Button>
          </div>
        </div>

        <div class="commands-list-container mt-4">
          <div class="table-responsive">
            <table class="commands-table w-full">
              <thead>
                <tr>
                  <th class="text-left p-2">Command</th>
                  <th class="text-left p-2 hidden sm:table-cell">Category</th>
                  <th class="text-left p-2">Description</th>
                  <th class="text-left p-2 hidden sm:table-cell">Type</th>
                </tr>
              </thead>
              <tbody>
                <tr
                  v-for="cmd in filteredAllCommands"
                  :key="cmd.Name"
                  @click="selectCommandFromDialog(cmd)"
                  class="command-row"
                  :class="{
                    'admin-row': cmd.CommandType === 1,
                    'public-row': cmd.CommandType === 0,
                  }"
                >
                  <td class="p-2 font-mono">{{ cmd.Name }}</td>
                  <td class="p-2 hidden sm:table-cell">{{ cmd.Category }}</td>
                  <td class="p-2">
                    <div>{{ cmd.Description }}</div>
                    <div class="sm:hidden text-xs mt-1">
                      <div>Category: {{ cmd.Category }}</div>
                      <span
                        :class="{
                          'text-red-500': cmd.CommandType === 1,
                          'text-green-500': cmd.CommandType === 0,
                        }"
                      >
                        {{ cmd.CommandType === 1 ? "Admin" : "Public" }}
                      </span>
                    </div>
                  </td>
                  <td class="p-2 hidden sm:table-cell">
                    <span
                      :class="{
                        'text-red-500': cmd.CommandType === 1,
                        'text-green-500': cmd.CommandType === 0,
                      }"
                    >
                      {{ cmd.CommandType === 1 ? "Admin" : "Public" }}
                    </span>
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
        </div>
      </DialogContent>
    </Dialog>

    <!-- Command History Dialog -->
    <Dialog
      :open="showCommandHistory"
      @update:open="showCommandHistory = $event"
    >
      <DialogContent class="max-w-4xl max-h-[80vh]">
        <DialogHeader>
          <DialogTitle>Command History</DialogTitle>
        </DialogHeader>

        <div class="flex justify-end mb-2">
          <Button @click="clearCommandHistory" variant="destructive" size="sm">
            Clear History
          </Button>
        </div>

        <div class="command-history-container">
          <div
            v-for="(entry, index) in commandHistory"
            :key="index"
            class="history-entry"
          >
            <div class="history-command">
              <div class="flex justify-between items-center">
                <div class="font-mono font-bold">{{ entry.command }}</div>
                <div class="flex space-x-2">
                  <span class="text-xs opacity-70">{{
                    new Date(entry.timestamp).toLocaleString()
                  }}</span>
                  <Button
                    @click="reuseCommand(entry)"
                    variant="outline"
                    size="sm"
                  >
                    Reuse
                  </Button>
                </div>
              </div>
              <div class="history-response">
                <pre class="console-pre">{{ entry.response }}</pre>
              </div>
            </div>
          </div>

          <div
            v-if="commandHistory.length === 0"
            class="text-center py-4 opacity-70"
          >
            No command history available
          </div>
        </div>
      </DialogContent>
    </Dialog>
  </div>
</template>

<style scoped>
.console-page {
  min-width: 0;
}

.terminal-input {
  font-family: monospace;
  background-color: #1e1e1e;
  color: #d4d4d4;
  border: none;
  padding: 10px;
  width: 100%;
  border-radius: 4px;
}

.terminal-output {
  font-family: monospace;
  background-color: #1e1e1e;
  color: #d4d4d4;
  padding: 10px;
  height: 200px;
  width: 100%;
  max-width: 100%;
  min-width: 0;
  box-sizing: border-box;
  overflow-y: auto;
  overflow-x: auto;
  border-radius: 4px;
  border: 1px solid #333;
}

.console-pre {
  margin: 0;
  width: 100%;
  max-width: 100%;
  min-width: 0;
  white-space: pre-wrap;
  overflow-wrap: anywhere;
  word-break: break-word;
  word-wrap: break-word;
}

.suggestions-list {
  background-color: #1e1e1e;
  color: #d4d4d4;
  border: 1px solid #333;
  border-radius: 4px;
  margin-top: 0.5rem;
  padding: 0.5rem;
  list-style: none;
  max-height: 300px;
  overflow-y: auto;
}

.suggestions-list li {
  cursor: pointer;
  padding: 0.5rem;
  border-radius: 4px;
  margin-bottom: 0.25rem;
}

.suggestions-list li:hover {
  background-color: #333;
}

.admin-command {
  border-left: 3px solid #ff5555;
}

.public-command {
  border-left: 3px solid #55ff55;
}

.command-name {
  font-weight: bold;
}

.command-description {
  font-size: 0.9rem;
  opacity: 0.9;
}

.filter-buttons .active {
  background-color: #333;
  color: white;
}

.flex {
  display: flex;
}

.flex-grow {
  flex-grow: 1;
}

.items-center {
  align-items: center;
}

.space-x-2 > :not([hidden]) ~ :not([hidden]) {
  --tw-space-x-reverse: 0;
  margin-right: calc(0.5rem * var(--tw-space-x-reverse));
  margin-left: calc(0.5rem * calc(1 - var(--tw-space-x-reverse)));
}

.mb-4 {
  margin-bottom: 1rem;
}

.mt-4 {
  margin-top: 1rem;
}

.mr-2 {
  margin-right: 0.5rem;
}

.selected-command-info {
  background-color: #1e1e1e;
  color: #d4d4d4;
  border: 1px solid #333;
  border-radius: 4px;
  padding: 0.75rem;
  margin-top: 0.5rem;
}

.commands-list-container {
  overflow-y: auto;
  max-height: 50vh;
  margin-right: -1rem;
  margin-left: -1rem;
  padding: 0 1rem;
}

.table-responsive {
  width: 100%;
  overflow-x: auto;
}

.commands-table {
  border-collapse: collapse;
  width: 100%;
  min-width: 100%;
  table-layout: fixed;
}

.commands-table th {
  background-color: #1e1e1e;
  position: sticky;
  top: 0;
  z-index: 10;
}

.command-row {
  cursor: pointer;
  border-bottom: 1px solid #333;
}

.command-row:hover {
  background-color: #333;
}

.admin-row {
  border-left: 3px solid #ff5555;
}

.public-row {
  border-left: 3px solid #55ff55;
}

.command-history-container {
  overflow-y: auto;
  max-height: 60vh;
}

.history-entry {
  margin-bottom: 1rem;
  border-bottom: 1px solid #333;
  padding-bottom: 1rem;
}

.history-entry:last-child {
  border-bottom: none;
}

.history-command {
  background-color: #1e1e1e;
  border-radius: 4px;
  padding: 0.5rem;
}

.history-response {
  background-color: #2a2a2a;
  border-radius: 4px;
  padding: 0.5rem;
  margin-top: 0.5rem;
  width: 100%;
  max-width: 100%;
  min-width: 0;
  box-sizing: border-box;
  max-height: 200px;
  overflow-y: auto;
}

.history-response pre {
  font-family: monospace;
  font-size: 0.9rem;
}

/* Live server events pane */
.live-events-container {
  font-family: monospace;
  font-size: 0.8rem;
  background-color: #1e1e1e;
  color: #d4d4d4;
  border: 1px solid #333;
  border-radius: 4px;
  padding: 10px;
  height: 260px;
  width: 100%;
  max-width: 100%;
  min-width: 0;
  box-sizing: border-box;
  overflow-y: auto;
}

.live-event-line {
  display: flex;
  align-items: baseline;
  gap: 0.5rem;
  padding: 1px 0;
}

.live-event-time {
  opacity: 0.6;
  flex-shrink: 0;
}

.live-event-type {
  flex-shrink: 0;
  border-radius: 3px;
  padding: 0 0.35rem;
  font-size: 0.7rem;
  line-height: 1.5;
  min-width: 3.4rem;
  text-align: center;
}

.live-event-body {
  min-width: 0;
  overflow-wrap: anywhere;
  word-break: break-word;
}

.event-type-blue {
  background-color: rgba(59, 130, 246, 0.2);
  color: #60a5fa;
}

.event-type-orange {
  background-color: rgba(249, 115, 22, 0.2);
  color: #fb923c;
}

.event-type-red {
  background-color: rgba(239, 68, 68, 0.2);
  color: #f87171;
}

.event-type-green {
  background-color: rgba(34, 197, 94, 0.2);
  color: #4ade80;
}

.event-type-purple {
  background-color: rgba(168, 85, 247, 0.2);
  color: #c084fc;
}

.event-type-gray {
  background-color: rgba(113, 113, 122, 0.2);
  color: #a1a1aa;
}

.live-status-dot {
  display: inline-block;
  width: 0.5rem;
  height: 0.5rem;
  border-radius: 9999px;
}

.live-status-live {
  background-color: #22c55e;
}

.live-status-reconnecting {
  background-color: #f59e0b;
}

.live-status-off {
  background-color: #71717a;
}

/* Add responsive styles for the command input container */
.command-input-container {
  display: flex;
  flex-direction: column;
  width: 100%;
}

.command-buttons-container {
  display: flex;
  flex-wrap: wrap;
  gap: 0.5rem;
}

.command-button {
  flex-grow: 1;
  min-width: max-content;
}

/* Media query for larger screens */
@media (min-width: 768px) {
  .command-input-container {
    flex-direction: row;
    align-items: center;
    gap: 0.5rem;
  }
  
  .command-buttons-container {
    display: flex;
    flex-wrap: nowrap;
  }
  
  .command-button {
    flex-grow: 0;
  }
  
  input.w-full {
    width: auto;
    flex-grow: 1;
    margin-bottom: 0;
  }
}

@media (max-width: 640px) {
  .commands-table th:first-child,
  .commands-table td:first-child {
    width: 40%;
  }
  
  .commands-table th:nth-child(3),
  .commands-table td:nth-child(3) {
    width: 60%;
  }
}

@media (min-width: 641px) {
  .commands-table th:first-child,
  .commands-table td:first-child {
    width: 20%;
  }
  
  .commands-table th:nth-child(2),
  .commands-table td:nth-child(2) {
    width: 15%;
  }
  
  .commands-table th:nth-child(3),
  .commands-table td:nth-child(3) {
    width: 55%;
  }
  
  .commands-table th:nth-child(4),
  .commands-table td:nth-child(4) {
    width: 10%;
  }
}
</style>
