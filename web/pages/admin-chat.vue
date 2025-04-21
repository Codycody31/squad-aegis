<template>
  <div class="p-4">
    <h1 class="text-2xl font-bold mb-6">Admin Chat</h1>

    <Card class="mb-6">
      <CardHeader class="px-6 py-4 border-b border-border">
        <div
          class="flex flex-col space-y-4 md:flex-row md:space-y-0 md:space-x-4"
        >
          <div class="w-full md:w-1/3">
            <label
              for="chatType"
              class="text-sm font-medium text-muted-foreground mb-1 block"
              >Chat Type</label
            >
            <select
              id="chatType"
              v-model="chatType"
              class="w-full flex h-10 rounded-md border border-input bg-background px-3 py-2 text-sm ring-offset-background placeholder:text-muted-foreground focus:outline-none focus:ring-2 focus:ring-ring focus:ring-offset-2"
              @change="handleChatTypeChange"
            >
              <option value="global">Global Admin Chat</option>
              <option value="server">Server-Specific Chat</option>
            </select>
          </div>

          <div v-if="chatType === 'server'" class="w-full md:w-2/3">
            <label
              for="server"
              class="text-sm font-medium text-muted-foreground mb-1 block"
              >Server</label
            >
            <select
              id="server"
              v-model="selectedServerId"
              class="w-full flex h-10 rounded-md border border-input bg-background px-3 py-2 text-sm ring-offset-background placeholder:text-muted-foreground focus:outline-none focus:ring-2 focus:ring-ring focus:ring-offset-2"
              @change="loadMessages"
            >
              <option value="">Select a server</option>
              <option
                v-for="server in servers"
                :key="server.id"
                :value="server.id"
              >
                {{ server.name }}
              </option>
            </select>
          </div>
        </div>
      </CardHeader>

      <CardContent class="p-6">
        <div v-if="loading" class="flex justify-center p-8">
          <div
            class="animate-spin h-8 w-8 border-4 border-primary border-t-transparent rounded-full"
          ></div>
        </div>

        <div
          v-else-if="messages.length === 0"
          class="text-center p-8 text-muted-foreground"
        >
          No messages found. Start a conversation!
        </div>

        <div
          v-else
          ref="messagesContainer"
          class="space-y-4 max-h-[60vh] overflow-y-auto p-2 scroll-smooth relative"
        >
          <template v-for="message in sortedMessages" :key="message.id">
            <div
              class="p-3 rounded-lg transition-all duration-200 hover:bg-opacity-80"
              :class="
                message.user.id === currentUser.id
                  ? 'bg-primary/10 ml-12'
                  : 'bg-muted ml-0 mr-12'
              "
              v-if="
                message.server_id == selectedServerId ||
                (message.server_id == null && selectedServerId == '')
              "
            >
              <div class="flex items-start">
                <div class="flex-shrink-0 mr-3">
                  <div
                    class="h-10 w-10 rounded-full bg-primary flex items-center justify-center text-primary-foreground"
                  >
                    {{ getInitials(message.user.username) }}
                  </div>
                </div>
                <div class="flex-1 overflow-hidden">
                  <div class="flex items-center justify-between mb-1">
                    <span class="font-medium">{{ message.user.username }}</span>
                    <span class="text-xs text-muted-foreground">{{
                      formatDate(message.created_at)
                    }}</span>
                  </div>
                  <p class="text-sm whitespace-pre-wrap break-words">
                    {{ message.message }}
                  </p>
                  <p
                    v-if="
                      message.server_id && globalServerNames[message.server_id]
                    "
                    class="text-xs text-muted-foreground mt-1"
                  >
                    Server: {{ globalServerNames[message.server_id] }}
                  </p>
                </div>
              </div>
            </div>
          </template>

          <!-- New message indicator -->
          <div
            v-if="unreadMessages"
            @click="scrollToBottom"
            class="absolute bottom-4 right-4 bg-primary text-primary-foreground rounded-full p-2 shadow-lg cursor-pointer hover:bg-primary/90 transition-all flex items-center gap-2"
          >
            <span class="text-xs">New messages</span>
            <svg
              xmlns="http://www.w3.org/2000/svg"
              width="16"
              height="16"
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              stroke-width="2"
              stroke-linecap="round"
              stroke-linejoin="round"
            >
              <path d="M12 5v14M5 12l7 7 7-7" />
            </svg>
          </div>
        </div>
      </CardContent>
    </Card>

    <Card>
      <CardContent class="p-6">
        <form @submit.prevent="sendMessage" class="flex flex-col space-y-4">
          <div
            class="flex items-center gap-2 mb-1 text-xs text-muted-foreground"
            v-if="typingStatus"
          >
            <div class="flex gap-1">
              <span class="animate-bounce">•</span>
              <span class="animate-bounce delay-75">•</span>
              <span class="animate-bounce delay-150">•</span>
            </div>
            <span>Someone is typing...</span>
          </div>

          <div class="flex">
            <textarea
              v-model="newMessage"
              placeholder="Type your message here..."
              rows="3"
              class="w-full flex min-h-20 rounded-md border border-input bg-background px-3 py-2 text-sm ring-offset-background placeholder:text-muted-foreground focus:outline-none focus:ring-2 focus:ring-ring focus:ring-offset-2"
              :disabled="
                (chatType === 'server' && !selectedServerId) || sending
              "
              @keydown.enter.exact.prevent="sendMessage"
              @input="handleTyping"
            ></textarea>
          </div>

          <div class="flex justify-between items-center">
            <button
              type="submit"
              class="inline-flex items-center justify-center rounded-md text-sm font-medium transition-colors focus:outline-none focus:ring-2 focus:ring-ring focus:ring-offset-2 bg-primary text-primary-foreground hover:bg-primary/90 h-10 px-4 py-2"
              :disabled="
                !newMessage.trim() ||
                (chatType === 'server' && !selectedServerId) ||
                sending
              "
            >
              <span v-if="sending" class="flex items-center gap-2">
                <svg
                  class="animate-spin h-4 w-4"
                  xmlns="http://www.w3.org/2000/svg"
                  fill="none"
                  viewBox="0 0 24 24"
                >
                  <circle
                    class="opacity-25"
                    cx="12"
                    cy="12"
                    r="10"
                    stroke="currentColor"
                    stroke-width="4"
                  ></circle>
                  <path
                    class="opacity-75"
                    fill="currentColor"
                    d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
                  ></path>
                </svg>
                Sending...
              </span>
              <span v-else class="flex items-center gap-2">
                Send
                <svg
                  xmlns="http://www.w3.org/2000/svg"
                  width="16"
                  height="16"
                  viewBox="0 0 24 24"
                  fill="none"
                  stroke="currentColor"
                  stroke-width="2"
                  stroke-linecap="round"
                  stroke-linejoin="round"
                >
                  <line x1="22" y1="2" x2="11" y2="13"></line>
                  <polygon points="22 2 15 22 11 13 2 9 22 2"></polygon>
                </svg>
              </span>
            </button>
          </div>
        </form>
      </CardContent>
    </Card>
  </div>
</template>

<script setup>
import { ref, onMounted, onUnmounted, computed, nextTick, watch } from "vue";
import { useToast } from "@/components/ui/toast/use-toast";
import { dateTimeFormat } from "~/utils/formatters";
import { useAuthStore } from "~/stores/auth";
import { Card, CardHeader, CardContent } from "@/components/ui/card";

const authStore = useAuthStore();
const { toast } = useToast();
const currentUser = computed(() => authStore.user);

const chatType = ref("global");
const selectedServerId = ref("");
const servers = ref([]);
const messages = ref([]);
const newMessage = ref("");
const loading = ref(false);
const sending = ref(false);
const messagesContainer = ref(null);
const pollingInterval = ref(null);
const globalServerNames = ref({}); // Map of server IDs to server names
const unreadMessages = ref(false);
const isScrolledToBottom = ref(true);
const typingStatus = ref(false);
const typingTimeout = ref(null);

// Computed property to get the current server name
const currentServerName = computed(() => {
  if (!selectedServerId.value) return "";
  const server = servers.value.find((s) => s.id === selectedServerId.value);
  return server ? server.name : "";
});

// Computed property to sort messages chronologically (oldest to newest)
const sortedMessages = computed(() => {
  return [...messages.value].sort((a, b) => {
    return new Date(a.created_at) - new Date(b.created_at);
  });
});

// Function to handle chat type change
function handleChatTypeChange() {
  // Reset the server selection when switching to/from server mode
  if (chatType.value === "global") {
    selectedServerId.value = "";
  }
  loadMessages();
}

// Fetch the list of servers on component mount
onMounted(async () => {
  await fetchServers();

  // Start polling for new messages
  startPolling();

  // Load initial messages
  await loadMessages();

  // Add scroll event listener to check if user is at bottom
  if (messagesContainer.value) {
    messagesContainer.value.addEventListener("scroll", checkScrollPosition);
  }
});

// Clean up on component unmount
onUnmounted(() => {
  stopPolling();

  // Remove scroll event listener
  if (messagesContainer.value) {
    messagesContainer.value.removeEventListener("scroll", checkScrollPosition);
  }

  // Clear typing timeout
  if (typingTimeout.value) {
    clearTimeout(typingTimeout.value);
  }
});

// Fetch all servers and create a lookup table for server names
async function fetchServers() {
  try {
    const response = await fetch("/api/servers", {
      headers: {
        Authorization: `Bearer ${authStore.token}`,
      },
    });
    const data = await response.json();
    if (data.code === 200) {
      servers.value = data.data.servers;

      // Create a map of server IDs to server names for global chat display
      data.data.servers.forEach((server) => {
        globalServerNames.value[server.id] = server.name;
      });
    }
  } catch (error) {
    console.error("Error fetching servers:", error);
    toast.error("Failed to fetch servers");
  }
}

// Start polling for new messages
function startPolling() {
  stopPolling(); // Clear any existing interval
  pollingInterval.value = setInterval(() => {
    refreshMessages();
  }, 10000); // Poll every 10 seconds
}

// Stop polling
function stopPolling() {
  if (pollingInterval.value) {
    clearInterval(pollingInterval.value);
    pollingInterval.value = null;
  }
}

// Scroll to the bottom of the messages container
function scrollToBottom() {
  if (messagesContainer.value) {
    messagesContainer.value.scrollTop = messagesContainer.value.scrollHeight;
    unreadMessages.value = false;
    isScrolledToBottom.value = true;
  }
}

// Check if user is scrolled to bottom
function checkScrollPosition() {
  if (!messagesContainer.value) return;

  const { scrollTop, scrollHeight, clientHeight } = messagesContainer.value;
  const isAtBottom = scrollHeight - scrollTop - clientHeight < 50;
  isScrolledToBottom.value = isAtBottom;
}

// Handle typing status
function handleTyping() {
  // In a real app, this would emit to other users
  // For now we'll just clear any existing timeout
  if (typingTimeout.value) {
    clearTimeout(typingTimeout.value);
  }

  // In a real app, this would set your typing status for others
  // Then clear it after a delay
  typingTimeout.value = setTimeout(() => {
    // This would clear your typing status
  }, 2000);
}

// Refresh messages without showing loading indicator
async function refreshMessages() {
  try {
    let url = "/api/admin-chat";
    if (chatType.value === "server" && selectedServerId.value) {
      // Using server-specific endpoint
      url = `/api/servers/${selectedServerId.value}/admin-chat`;
    }
    // Global chat should always use the base endpoint without server_id

    const response = await fetch(url, {
      headers: {
        Authorization: `Bearer ${authStore.token}`,
      },
    });
    const data = await response.json();

    if (data.code === 200) {
      // Check if we have new messages
      if (data.data.messages.length !== messages.value.length) {
        const previousLength = messages.value.length;
        messages.value = data.data.messages;

        // If we have new messages and user is not scrolled to bottom,
        // show the unread messages indicator
        if (data.data.messages.length > previousLength) {
          if (isScrolledToBottom.value) {
            await nextTick();
            scrollToBottom();
          } else {
            unreadMessages.value = true;
          }
        }
      }
    }
  } catch (error) {
    console.error("Error refreshing messages:", error);
  }
}

// Load messages with loading indicator
async function loadMessages() {
  loading.value = true;
  messages.value = [];

  try {
    let url = "/api/admin-chat";
    if (chatType.value === "server" && selectedServerId.value) {
      // Using server-specific endpoint
      url = `/api/servers/${selectedServerId.value}/admin-chat`;
    }
    // Global chat should always use the base endpoint without server_id

    const response = await fetch(url, {
      headers: {
        Authorization: `Bearer ${authStore.token}`,
      },
    });
    const data = await response.json();

    if (data.code === 200) {
      messages.value = data.data.messages;

      // Scroll to bottom after messages load
      await nextTick();
      scrollToBottom();
    } else {
      toast.error(data.message || "Failed to load messages");
    }
  } catch (error) {
    console.error("Error loading messages:", error);
    toast.error("Failed to load messages");
  } finally {
    loading.value = false;
  }
}

// Send a new message
async function sendMessage() {
  if (!newMessage.value.trim()) return;

  sending.value = true;

  try {
    let url = "/api/admin-chat";
    if (chatType.value === "server" && selectedServerId.value) {
      // Using server-specific endpoint
      url = `/api/servers/${selectedServerId.value}/admin-chat`;
    }
    // Global chat should always use the base endpoint without server_id

    const response = await fetch(url, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        Authorization: `Bearer ${authStore.token}`,
      },
      body: JSON.stringify({
        message: newMessage.value,
      }),
    });

    const data = await response.json();

    if (data.code === 200) {
      newMessage.value = "";
      await loadMessages(); // Reload messages to see the new one
    } else {
      toast.error(data.message || "Failed to send message");
    }
  } catch (error) {
    console.error("Error sending message:", error);
    toast.error("Failed to send message");
  } finally {
    sending.value = false;
  }
}

// Format date for display
function formatDate(dateString) {
  return dateTimeFormat(new Date(dateString));
}

// Get initials from username for avatar
function getInitials(username) {
  if (!username) return "?";

  return username
    .split(" ")
    .map((part) => part[0])
    .join("")
    .toUpperCase()
    .substring(0, 2);
}

// Watch for changes in server selection
watch(selectedServerId, (newId) => {
  if (newId) {
    loadMessages();
  }
});
</script>

<script>
// Options API for page meta
export default {
  middleware: "auth",
  meta: {
    title: "Admin Chat",
  },
};
</script>
