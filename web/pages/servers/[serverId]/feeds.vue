<template>
  <div class="p-6 space-y-6">
    <div class="flex items-center justify-between">
      <div>
        <h1 class="text-3xl font-bold tracking-tight">Live Feeds</h1>
        <p class="text-muted-foreground">
          Real-time chat messages, player connections, and teamkills
        </p>
      </div>
      <div class="flex items-center space-x-2">
        <Button
          variant="outline"
          size="sm"
          @click="toggleConnection"
          :disabled="connecting"
        >
          <Icon
            :name="isConnected ? 'mdi:pause' : 'mdi:play'"
            class="h-4 w-4 mr-2"
          />
          {{ isConnected ? "Pause" : "Start" }}
        </Button>
        <Button variant="outline" size="sm" @click="clearAllFeeds">
          <Icon name="mdi:delete" class="h-4 w-4 mr-2" />
          Clear All
        </Button>
      </div>
    </div>

    <!-- Connection Status -->
    <div v-if="connecting || error" class="mb-4">
      <Card v-if="connecting" class="mb-4 border-blue-200 bg-blue-50">
        <CardContent class="p-4">
          <div class="flex items-center space-x-2">
            <Icon name="mdi:loading" class="h-4 w-4 animate-spin text-blue-600" />
            <div>
              <p class="font-medium text-blue-900">Connecting</p>
              <p class="text-sm text-blue-700">Establishing connection to live feeds...</p>
            </div>
          </div>
        </CardContent>
      </Card>
      <Card v-if="error" class="mb-4 border-red-200 bg-red-50">
        <CardContent class="p-4">
          <div class="flex items-center space-x-2">
            <Icon name="mdi:alert-circle" class="h-4 w-4 text-red-600" />
            <div>
              <p class="font-medium text-red-900">Connection Error</p>
              <p class="text-sm text-red-700">{{ error }}</p>
            </div>
          </div>
        </CardContent>
      </Card>
    </div>

    <!-- Feed Stats -->
    <div class="grid grid-cols-1 md:grid-cols-3 gap-4 mb-6">
      <Card>
        <CardContent class="p-4">
          <div class="flex items-center space-x-2">
            <Icon name="mdi:message-text" class="h-6 w-6 text-blue-500" />
            <div>
              <p class="text-sm text-muted-foreground">Chat Messages</p>
              <p class="text-2xl font-bold">{{ chatCount }}</p>
            </div>
          </div>
        </CardContent>
      </Card>
      <Card>
        <CardContent class="p-4">
          <div class="flex items-center space-x-2">
            <Icon name="mdi:account-multiple" class="h-6 w-6 text-green-500" />
            <div>
              <p class="text-sm text-muted-foreground">Connections</p>
              <p class="text-2xl font-bold">{{ connectionsCount }}</p>
            </div>
          </div>
        </CardContent>
      </Card>
      <Card>
        <CardContent class="p-4">
          <div class="flex items-center space-x-2">
            <Icon name="mdi:skull" class="h-6 w-6 text-red-500" />
            <div>
              <p class="text-sm text-muted-foreground">Teamkills</p>
              <p class="text-2xl font-bold">{{ teamkillsCount }}</p>
            </div>
          </div>
        </CardContent>
      </Card>
    </div>

    <!-- Tabs -->
    <Tabs v-model="activeTab" class="w-full">
      <TabsList class="grid w-full grid-cols-3">
        <TabsTrigger value="chat">
          <Icon name="mdi:message-text" class="h-4 w-4 mr-2" />
          Chat Messages
        </TabsTrigger>
        <TabsTrigger value="connections">
          <Icon name="mdi:account-multiple" class="h-4 w-4 mr-2" />
          Player Connections
        </TabsTrigger>
        <TabsTrigger value="teamkills">
          <Icon name="mdi:skull" class="h-4 w-4 mr-2" />
          Teamkills
        </TabsTrigger>
      </TabsList>

      <!-- Chat Feed -->
      <TabsContent value="chat" class="space-y-4">
        <div class="flex items-center justify-between">
          <h3 class="text-lg font-semibold">Chat Messages</h3>
          <Button variant="outline" size="sm" @click="clearFeed('chat')">
            <Icon name="mdi:delete" class="h-4 w-4 mr-2" />
            Clear
          </Button>
        </div>
        <Card>
          <CardContent class="p-0">
            <div
              ref="chatContainer"
              class="max-h-96 overflow-y-auto border rounded-lg"
            >
              <div v-if="chatMessages.length === 0" class="p-8 text-center text-muted-foreground">
                <Icon name="mdi:message-text-outline" class="h-12 w-12 mx-auto mb-4 opacity-50" />
                <p>No chat messages yet</p>
                <p class="text-sm">Messages will appear here when players chat</p>
              </div>
              <div v-else class="space-y-1 p-4">
                <div
                  v-for="message in chatMessages"
                  :key="message.id"
                  class="flex items-start space-x-3 p-2 rounded-lg hover:bg-muted/50"
                >
                  <div class="flex-shrink-0 mt-1">
                    <div
                      class="w-2 h-2 rounded-full"
                      :class="getChatTypeColor(message.data.chat_type)"
                    ></div>
                  </div>
                  <div class="flex-1 min-w-0">
                    <div class="flex items-center space-x-2">
                      <p class="font-medium text-sm">{{ message.data.player_name }}</p>
                      <Badge :variant="getChatTypeBadge(message.data.chat_type)" class="text-xs">
                        {{ message.data.chat_type }}
                      </Badge>
                      <span class="text-xs text-muted-foreground">
                        {{ formatTimestamp(message.timestamp) }}
                      </span>
                    </div>
                    <p class="text-sm text-foreground mt-1">{{ message.data.message }}</p>
                  </div>
                </div>
              </div>
            </div>
          </CardContent>
        </Card>
      </TabsContent>

      <!-- Connections Feed -->
      <TabsContent value="connections" class="space-y-4">
        <div class="flex items-center justify-between">
          <h3 class="text-lg font-semibold">Player Connections</h3>
          <Button variant="outline" size="sm" @click="clearFeed('connections')">
            <Icon name="mdi:delete" class="h-4 w-4 mr-2" />
            Clear
          </Button>
        </div>
        <Card>
          <CardContent class="p-0">
            <div
              ref="connectionsContainer"
              class="max-h-96 overflow-y-auto border rounded-lg"
            >
              <div v-if="connections.length === 0" class="p-8 text-center text-muted-foreground">
                <Icon name="mdi:account-multiple-outline" class="h-12 w-12 mx-auto mb-4 opacity-50" />
                <p>No connection events yet</p>
                <p class="text-sm">Player connections will appear here</p>
              </div>
              <div v-else class="space-y-1 p-4">
                <div
                  v-for="connection in connections"
                  :key="connection.id"
                  class="flex items-center space-x-3 p-2 rounded-lg hover:bg-muted/50"
                >
                  <div class="flex-shrink-0">
                    <Icon
                      :name="connection.data.action === 'connected' ? 'mdi:account-plus' : 'mdi:account-check'"
                      :class="connection.data.action === 'connected' ? 'text-green-500' : 'text-blue-500'"
                      class="h-5 w-5"
                    />
                  </div>
                  <div class="flex-1">
                    <div class="flex items-center space-x-2">
                      <p class="font-medium text-sm">
                        {{ connection.data.player_suffix || connection.data.player_controller }}
                      </p>
                      <Badge
                        :variant="connection.data.action === 'connected' ? 'default' : 'secondary'"
                        class="text-xs"
                      >
                        {{ connection.data.action }}
                      </Badge>
                      <span class="text-xs text-muted-foreground">
                        {{ formatTimestamp(connection.timestamp) }}
                      </span>
                    </div>
                    <p class="text-xs text-muted-foreground mt-1">
                      IP: {{ connection.data.ip_address || 'Unknown' }}
                    </p>
                  </div>
                </div>
              </div>
            </div>
          </CardContent>
        </Card>
      </TabsContent>

      <!-- Teamkills Feed -->
      <TabsContent value="teamkills" class="space-y-4">
        <div class="flex items-center justify-between">
          <h3 class="text-lg font-semibold">Teamkills</h3>
          <Button variant="outline" size="sm" @click="clearFeed('teamkills')">
            <Icon name="mdi:delete" class="h-4 w-4 mr-2" />
            Clear
          </Button>
        </div>
        <Card>
          <CardContent class="p-0">
            <div
              ref="teamkillsContainer"
              class="max-h-96 overflow-y-auto border rounded-lg"
            >
              <div v-if="teamkills.length === 0" class="p-8 text-center text-muted-foreground">
                <Icon name="mdi:skull-outline" class="h-12 w-12 mx-auto mb-4 opacity-50" />
                <p>No teamkills yet</p>
                <p class="text-sm">Teamkill events will appear here</p>
              </div>
              <div v-else class="space-y-1 p-4">
                <div
                  v-for="teamkill in teamkills"
                  :key="teamkill.id"
                  class="flex items-start space-x-3 p-2 rounded-lg hover:bg-muted/50 border-l-4 border-red-500"
                >
                  <div class="flex-shrink-0 mt-1">
                    <Icon name="mdi:skull" class="h-5 w-5 text-red-500" />
                  </div>
                  <div class="flex-1">
                    <div class="flex items-center space-x-2">
                      <p class="font-medium text-sm text-red-600">
                        {{ teamkill.data.attacker_name }} → {{ teamkill.data.victim_name }}
                      </p>
                      <span class="text-xs text-muted-foreground">
                        {{ formatTimestamp(teamkill.timestamp) }}
                      </span>
                    </div>
                    <div class="flex items-center space-x-4 mt-1 text-xs text-muted-foreground">
                      <span>Weapon: {{ teamkill.data.weapon }}</span>
                      <span>Damage: {{ teamkill.data.damage }}</span>
                    </div>
                  </div>
                </div>
              </div>
            </div>
          </CardContent>
        </Card>
      </TabsContent>
    </Tabs>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, onUnmounted, nextTick, computed } from "vue";
import { Card, CardContent } from "~/components/ui/card";
import { Button } from "~/components/ui/button";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "~/components/ui/tabs";
import { Badge } from "~/components/ui/badge";
import { Icon } from "#components";

definePageMeta({
  middleware: "auth",
});

const route = useRoute();
const serverId = route.params.serverId as string;

// Reactive state
const activeTab = ref("chat");
const isConnected = ref(false);
const connecting = ref(false);
const error = ref<string | null>(null);

// Feed data
const chatMessages = ref<any[]>([]);
const connections = ref<any[]>([]);
const teamkills = ref<any[]>([]);

// Container refs for auto-scrolling
const chatContainer = ref<HTMLElement>();
const connectionsContainer = ref<HTMLElement>();
const teamkillsContainer = ref<HTMLElement>();

// Event source for SSE
let eventSource: EventSource | null = null;

// Computed counts
const chatCount = computed(() => chatMessages.value.length);
const connectionsCount = computed(() => connections.value.length);
const teamkillsCount = computed(() => teamkills.value.length);

// Connect to SSE endpoint
const connectToFeeds = async () => {
  if (isConnected.value || connecting.value) return;

  connecting.value = true;
  error.value = null;

  try {
    const runtimeConfig = useRuntimeConfig();
    const cookieToken = useCookie(
      runtimeConfig.public.sessionCookieName as string
    );
    const token = cookieToken.value;

    if (!token) {
      throw new Error("Authentication required");
    }

    const url = `${runtimeConfig.public.backendApi}/servers/${serverId}/feeds?types=chat&types=connections&types=teamkills&token=${token}`;
    
    eventSource = new EventSource(url);

    eventSource.onopen = () => {
      isConnected.value = true;
      connecting.value = false;
      error.value = null;
    };

    eventSource.onmessage = (event) => {
      try {
        const data = JSON.parse(event.data);
        handleFeedEvent(data);
      } catch (err) {
        console.error("Failed to parse event data:", err);
      }
    };

    eventSource.addEventListener("feed", (event) => {
      try {
        const data = JSON.parse((event as MessageEvent).data);
        handleFeedEvent(data);
      } catch (err) {
        console.error("Failed to parse feed event:", err);
      }
    });

    eventSource.addEventListener("connected", (event) => {
      console.log("Connected to feeds:", (event as MessageEvent).data);
    });

    eventSource.addEventListener("ping", (event) => {
      // Handle ping to keep connection alive
    });

    eventSource.onerror = (event) => {
      console.error("EventSource failed:", event);
      error.value = "Connection to live feeds failed";
      isConnected.value = false;
      connecting.value = false;
      
      // Retry connection after 5 seconds
      setTimeout(() => {
        if (!isConnected.value) {
          connectToFeeds();
        }
      }, 5000);
    };

  } catch (err: any) {
    error.value = err.message || "Failed to connect to live feeds";
    connecting.value = false;
  }
};

// Disconnect from SSE
const disconnectFromFeeds = () => {
  if (eventSource) {
    eventSource.close();
    eventSource = null;
  }
  isConnected.value = false;
  connecting.value = false;
  error.value = null;
};

// Toggle connection
const toggleConnection = () => {
  if (isConnected.value) {
    disconnectFromFeeds();
  } else {
    connectToFeeds();
  }
};

// Handle incoming feed events
const handleFeedEvent = (event: any) => {
  const maxEvents = 500; // Limit to prevent memory issues

  switch (event.type) {
    case "chat":
      chatMessages.value.unshift(event);
      if (chatMessages.value.length > maxEvents) {
        chatMessages.value = chatMessages.value.slice(0, maxEvents);
      }
      nextTick(() => scrollToTop(chatContainer.value));
      break;

    case "connection":
      connections.value.unshift(event);
      if (connections.value.length > maxEvents) {
        connections.value = connections.value.slice(0, maxEvents);
      }
      nextTick(() => scrollToTop(connectionsContainer.value));
      break;

    case "teamkill":
      teamkills.value.unshift(event);
      if (teamkills.value.length > maxEvents) {
        teamkills.value = teamkills.value.slice(0, maxEvents);
      }
      nextTick(() => scrollToTop(teamkillsContainer.value));
      break;
  }
};

// Clear specific feed
const clearFeed = (feedType: string) => {
  switch (feedType) {
    case "chat":
      chatMessages.value = [];
      break;
    case "connections":
      connections.value = [];
      break;
    case "teamkills":
      teamkills.value = [];
      break;
  }
};

// Clear all feeds
const clearAllFeeds = () => {
  chatMessages.value = [];
  connections.value = [];
  teamkills.value = [];
};

// Helper functions
const scrollToTop = (container: HTMLElement | undefined) => {
  if (container) {
    container.scrollTop = 0;
  }
};

const formatTimestamp = (timestamp: string) => {
  return new Date(timestamp).toLocaleTimeString();
};

const getChatTypeColor = (chatType: string) => {
  switch (chatType?.toLowerCase()) {
    case "chatall":
      return "bg-blue-500";
    case "chatteam":
      return "bg-green-500";
    case "chatsquad":
      return "bg-yellow-500";
    case "chatadmin":
      return "bg-red-500";
    default:
      return "bg-gray-500";
  }
};

const getChatTypeBadge = (chatType: string) => {
  switch (chatType?.toLowerCase()) {
    case "chatall":
      return "default";
    case "chatteam":
      return "secondary";
    case "chatsquad":
      return "outline";
    case "chatadmin":
      return "destructive";
    default:
      return "secondary";
  }
};

// Lifecycle
onMounted(() => {
  connectToFeeds();
});

onUnmounted(() => {
  disconnectFromFeeds();
});
</script>
