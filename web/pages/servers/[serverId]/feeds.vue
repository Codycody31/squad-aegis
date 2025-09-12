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
          @click="refreshFeeds"
          :disabled="loading || connecting"
        >
          <Icon
            :name="loading ? 'mdi:loading' : 'mdi:refresh'"
            :class="['h-4 w-4 mr-2', { 'animate-spin': loading }]"
          />
          Refresh
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
const loading = ref(true);
const loadingOlder = ref(false);

// Feed data with infinite scroll support
const chatMessages = ref<any[]>([]);
const connections = ref<any[]>([]);
const teamkills = ref<any[]>([]);

const hasMoreChat = ref(true);
const hasMoreConnections = ref(true);
const hasMoreTeamkills = ref(true);

const oldestChatTime = ref<string | null>(null);
const oldestConnectionTime = ref<string | null>(null);
const oldestTeamkillTime = ref<string | null>(null);

// Container refs for auto-scrolling
const chatContainer = ref<HTMLElement>();
const connectionsContainer = ref<HTMLElement>();
const teamkillsContainer = ref<HTMLElement>();

// WebSocket connection
let websocket: WebSocket | null = null;

// Computed counts
const chatCount = computed(() => chatMessages.value.length);
const connectionsCount = computed(() => connections.value.length);
const teamkillsCount = computed(() => teamkills.value.length);

// Scroll tracking
const isAtBottom = ref(true);

// Load initial historical data
const loadInitialHistoricalData = async () => {
  await Promise.all([
    loadHistoricalData("chat", 50),
    loadHistoricalData("connections", 50),
    loadHistoricalData("teamkills", 50),
  ]);

  // Scroll to bottom initially
  await nextTick();
  setTimeout(() => {
    scrollToBottom("chat");
    scrollToBottom("connections");
    scrollToBottom("teamkills");
  }, 100);
};

// Load historical data for a specific feed type
const loadHistoricalData = async (type: string, limit: number = 50, before?: string) => {
  try {
    let url = `/api/servers/${serverId}/feeds/history?type=${type}&limit=${limit}`;
    if (before) {
      url += `&before=${before}`;
    }

    const response = await $fetch(url, {
      headers: {
        Authorization: `Bearer ${useCookie(useRuntimeConfig().public.sessionCookieName as string).value}`,
      },
    });

    const newEvents = (response as any).data.events || [];
    
    if (newEvents.length > 0) {
      if (type === "chat") {
        if (before) {
          // Prepend older events to the beginning
          chatMessages.value.unshift(...newEvents);
        } else {
          // Initial load or refresh
          chatMessages.value = newEvents;
        }
        oldestChatTime.value = newEvents[0]?.timestamp;
        hasMoreChat.value = newEvents.length === limit;
      } else if (type === "connections") {
        if (before) {
          connections.value.unshift(...newEvents);
        } else {
          connections.value = newEvents;
        }
        oldestConnectionTime.value = newEvents[0]?.timestamp;
        hasMoreConnections.value = newEvents.length === limit;
      } else if (type === "teamkills") {
        if (before) {
          teamkills.value.unshift(...newEvents);
        } else {
          teamkills.value = newEvents;
        }
        oldestTeamkillTime.value = newEvents[0]?.timestamp;
        hasMoreTeamkills.value = newEvents.length === limit;
      }
    } else {
      // No more data available
      if (type === "chat") hasMoreChat.value = false;
      else if (type === "connections") hasMoreConnections.value = false;
      else if (type === "teamkills") hasMoreTeamkills.value = false;
    }
  } catch (err: any) {
    console.error(`Failed to load ${type} history:`, err);
  }
};

// Handle scroll events for infinite loading
const handleScroll = async (event: Event, feedType: string) => {
  const container = event.target as HTMLElement;
  const scrollTop = container.scrollTop;
  const scrollHeight = container.scrollHeight;
  const clientHeight = container.clientHeight;

  // Check if user is at the bottom
  isAtBottom.value = scrollHeight - scrollTop - clientHeight < 100;

  // Load older data when scrolling near the top
  if (scrollTop < 200 && !loadingOlder.value) {
    let hasMore = false;
    let oldestTime = null;

    if (feedType === "chat") {
      hasMore = hasMoreChat.value;
      oldestTime = oldestChatTime.value;
    } else if (feedType === "connections") {
      hasMore = hasMoreConnections.value;
      oldestTime = oldestConnectionTime.value;
    } else if (feedType === "teamkills") {
      hasMore = hasMoreTeamkills.value;
      oldestTime = oldestTeamkillTime.value;
    }

    if (hasMore && oldestTime) {
      loadingOlder.value = true;
      const previousScrollHeight = container.scrollHeight;

      await loadHistoricalData(feedType, 50, oldestTime);

      // Maintain scroll position
      await nextTick();
      const newScrollHeight = container.scrollHeight;
      container.scrollTop = scrollTop + (newScrollHeight - previousScrollHeight);
      
      loadingOlder.value = false;
    }
  }
};

// Connect to WebSocket endpoint
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

    // Convert HTTP/HTTPS URL to WebSocket URL
    const backendUrl = runtimeConfig.public.backendApi;
    const wsProtocol = backendUrl.startsWith('https') ? 'wss' : 'ws';
    const baseUrl = backendUrl.replace(/^https?:\/\//, '');
    const url = `${wsProtocol}://${baseUrl}/servers/${serverId}/feeds?types=chat&types=connections&types=teamkills&token=${token}`;
    
    websocket = new WebSocket(url);

    websocket.onopen = () => {
      isConnected.value = true;
      connecting.value = false;
      error.value = null;
    };

    websocket.onmessage = (event) => {
      try {
        const data = JSON.parse(event.data);
        handleFeedEvent(data);
      } catch (err) {
        console.error("Failed to parse WebSocket message:", err);
      }
    };

    websocket.onclose = (event) => {
      console.log("WebSocket connection closed:", event);
      isConnected.value = false;
      connecting.value = false;
      
      // Retry connection after 5 seconds if not manually disconnected
      if (event.code !== 1000) { // 1000 is normal closure
        error.value = "Connection to live feeds was lost";
        setTimeout(() => {
          if (!isConnected.value) {
            connectToFeeds();
          }
        }, 5000);
      }
    };

    websocket.onerror = (event) => {
      console.error("WebSocket error:", event);
      error.value = "Connection to live feeds failed";
      isConnected.value = false;
      connecting.value = false;
    };

  } catch (err: any) {
    error.value = err.message || "Failed to connect to live feeds";
    connecting.value = false;
  }
};

// Disconnect from WebSocket
const disconnectFromFeeds = () => {
  if (websocket) {
    websocket.close(1000, "User disconnect"); // Normal closure
    websocket = null;
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

// Handle incoming feed events (real-time)
const handleFeedEvent = (event: any) => {
  // Handle connection message
  if (event.type === "connected") {
    console.log("Connected to feeds:", event.message, event.types);
    return;
  }

  const maxEvents = 1000; // Increased limit for better history

  switch (event.type) {
    case "chat":
      chatMessages.value.push(event);
      if (chatMessages.value.length > maxEvents) {
        chatMessages.value = chatMessages.value.slice(-maxEvents);
      }
      if (isAtBottom.value) {
        nextTick(() => scrollToBottom("chat"));
      }
      break;

    case "connection":
      connections.value.push(event);
      if (connections.value.length > maxEvents) {
        connections.value = connections.value.slice(-maxEvents);
      }
      if (isAtBottom.value) {
        nextTick(() => scrollToBottom("connections"));
      }
      break;

    case "teamkill":
      teamkills.value.push(event);
      if (teamkills.value.length > maxEvents) {
        teamkills.value = teamkills.value.slice(-maxEvents);
      }
      if (isAtBottom.value) {
        nextTick(() => scrollToBottom("teamkills"));
      }
      break;
  }
};

// Clear specific feed
const clearFeed = (feedType: string) => {
  switch (feedType) {
    case "chat":
      chatMessages.value = [];
      oldestChatTime.value = null;
      hasMoreChat.value = true;
      break;
    case "connections":
      connections.value = [];
      oldestConnectionTime.value = null;
      hasMoreConnections.value = true;
      break;
    case "teamkills":
      teamkills.value = [];
      oldestTeamkillTime.value = null;
      hasMoreTeamkills.value = true;
      break;
  }
};

// Clear all feeds
const clearAllFeeds = () => {
  clearFeed("chat");
  clearFeed("connections");
  clearFeed("teamkills");
};

// Refresh feeds (reload historical and reconnect)
const refreshFeeds = async () => {
  loading.value = true;
  try {
    // Clear existing data
    clearAllFeeds();
    
    // Disconnect and reconnect websocket
    disconnectFromFeeds();
    
    // Reload historical data
    await loadInitialHistoricalData();
    
    // Reconnect websocket
    await connectToFeeds();
  } finally {
    loading.value = false;
  }
};

// Helper functions
const scrollToBottom = (feedType: string) => {
  let container: HTMLElement | undefined;
  if (feedType === "chat") container = chatContainer.value;
  else if (feedType === "connections") container = connectionsContainer.value;
  else if (feedType === "teamkills") container = teamkillsContainer.value;

  if (container) {
    container.scrollTop = container.scrollHeight;
    isAtBottom.value = true;
  }
};

const formatTimestamp = (timestamp: string) => {
  return new Date(timestamp).toLocaleString();
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
onMounted(async () => {
  loading.value = true;
  try {
    // Load historical data first
    await loadInitialHistoricalData();
    
    // Then connect to live feeds
    await connectToFeeds();
  } finally {
    loading.value = false;
  }
});

onUnmounted(() => {
  disconnectFromFeeds();
});
</script>
