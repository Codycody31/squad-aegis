<script setup lang="ts">
import { ref, watch, computed } from "vue";
import { Input } from "~/components/ui/input";
import { Button } from "~/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from "~/components/ui/dialog";

const route = useRoute();
const serverId = route.params.serverId;

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
  const cookieToken = useCookie(
    runtimeConfig.public.sessionCookieName as string
  );
  const token = cookieToken.value;

  if (!token) {
    return;
  }

  const { data, error } = await useFetch<CommandResponse>(
    `${runtimeConfig.public.backendApi}/servers/${serverId}/rcon/execute`,
    {
      method: "POST",
      body: {
        command: command.value,
      },
      headers: {
        Authorization: `Bearer ${token}`,
      },
    }
  );

  let responseText = "No response from server";

  if (error.value) {
    console.error(error.value);
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
  if (commandHistory.value.length > 100) {
    commandHistory.value = commandHistory.value.slice(0, 100);
  }

  loading.value = false;
}

async function fetchSuggestions(query: string) {
  if (!query) {
    suggestions.value = [];
    selectedCommand.value = null;
    return;
  }

  const runtimeConfig = useRuntimeConfig();
  const cookieToken = useCookie(
    runtimeConfig.public.sessionCookieName as string
  );
  const token = cookieToken.value;

  if (!token) {
    return;
  }

  const { data, error } = await useFetch<CommandsResponse>(
    `${runtimeConfig.public.backendApi}/servers/${serverId}/rcon/commands/autocomplete?q=${query}`,
    {
      method: "GET",
      headers: {
        Authorization: `Bearer ${token}`,
      },
    }
  );

  if (error.value) {
    console.error(error.value);
  }

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
  const cookieToken = useCookie(
    runtimeConfig.public.sessionCookieName as string
  );
  const token = cookieToken.value;

  const { data, error } = await useFetch<CommandsResponse>(
    `${runtimeConfig.public.backendApi}/servers/${serverId}/rcon/commands`,
    {
      method: "GET",
      headers: {
        Authorization: `Bearer ${token}`,
      },
    }
  );

  if (error.value) {
    console.error(error.value);
    return;
  }

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
}
</script>

<template>
  <div class="p-4">
    <h1 class="text-2xl font-bold mb-4">Console</h1>

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
      <pre>{{ output }}</pre>
    </div>

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
                <pre>{{ entry.response }}</pre>
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
  overflow-y: auto;
  overflow-x: auto;
  border-radius: 4px;
  border: 1px solid #333;
}

.terminal-output pre {
  white-space: pre-wrap;
  word-wrap: break-word;
  max-width: 100%;
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
  max-height: 200px;
  overflow-y: auto;
}

.history-response pre {
  white-space: pre-wrap;
  word-wrap: break-word;
  max-width: 100%;
  font-family: monospace;
  font-size: 0.9rem;
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
