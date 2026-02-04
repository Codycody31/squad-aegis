<script setup lang="ts">
import { ref, watch, computed } from "vue";
import { Card, CardContent, CardHeader, CardTitle } from "~/components/ui/card";
import { Input } from "~/components/ui/input";
import { Button } from "~/components/ui/button";
import { Badge } from "~/components/ui/badge";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "~/components/ui/select";
import { Search, ChevronLeft, ChevronRight, Loader2 } from "lucide-vue-next";
import type { ChatMessage, PaginatedChatHistory } from "~/types/player";

const props = defineProps<{
  playerId: string;
  initialMessages: ChatMessage[];
}>();

const runtimeConfig = useRuntimeConfig();
const loading = ref(false);
const error = ref<string | null>(null);

const messages = ref<ChatMessage[]>(props.initialMessages);
const total = ref(props.initialMessages.length);
const page = ref(1);
const limit = ref(50);
const totalPages = ref(1);

const searchQuery = ref("");
const chatTypeFilter = ref("all");

async function fetchChat() {
  loading.value = true;
  error.value = null;

  const cookieToken = useCookie(
    runtimeConfig.public.sessionCookieName as string
  );
  const token = cookieToken.value;

  if (!token) {
    error.value = "Authentication required";
    loading.value = false;
    return;
  }

  try {
    const params = new URLSearchParams({
      page: page.value.toString(),
      limit: limit.value.toString(),
    });

    if (chatTypeFilter.value && chatTypeFilter.value !== "all") {
      params.set("type", chatTypeFilter.value);
    }
    if (searchQuery.value) {
      params.set("search", searchQuery.value);
    }

    const response = await fetch(
      `${runtimeConfig.public.backendApi}/players/${props.playerId}/chat?${params}`,
      {
        headers: {
          Authorization: `Bearer ${token}`,
        },
      }
    );

    if (!response.ok) throw new Error("Failed to fetch chat history");

    const data = await response.json();
    const chat: PaginatedChatHistory = data.data.chat;
    messages.value = chat.messages;
    total.value = chat.total;
    totalPages.value = chat.total_pages;
  } catch (err: any) {
    error.value = err.message;
  } finally {
    loading.value = false;
  }
}

function formatDate(dateString: string): string {
  return new Date(dateString).toLocaleString();
}

function getChatTypeColor(chatType: string): string {
  switch (chatType.toLowerCase()) {
    case "all":
      return "text-blue-500";
    case "team":
      return "text-green-500";
    case "squad":
      return "text-yellow-500";
    case "admin":
      return "text-red-500";
    default:
      return "text-muted-foreground";
  }
}

function getChatTypeBadgeVariant(
  chatType: string
): "default" | "secondary" | "destructive" | "outline" {
  switch (chatType.toLowerCase()) {
    case "admin":
      return "destructive";
    case "squad":
      return "secondary";
    default:
      return "outline";
  }
}

// Debounced search
let searchTimeout: any = null;
watch(searchQuery, () => {
  if (searchTimeout) clearTimeout(searchTimeout);
  searchTimeout = setTimeout(() => {
    page.value = 1;
    fetchChat();
  }, 500);
});

watch(chatTypeFilter, () => {
  page.value = 1;
  fetchChat();
});

function nextPage() {
  if (page.value < totalPages.value) {
    page.value++;
    fetchChat();
  }
}

function prevPage() {
  if (page.value > 1) {
    page.value--;
    fetchChat();
  }
}
</script>

<template>
  <Card>
    <CardHeader class="pb-3">
      <div
        class="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-3"
      >
        <CardTitle class="text-lg">Chat History</CardTitle>
        <div class="flex flex-col sm:flex-row gap-2">
          <div class="relative">
            <Search
              class="absolute left-2.5 top-2.5 h-4 w-4 text-muted-foreground"
            />
            <Input
              v-model="searchQuery"
              placeholder="Search messages..."
              class="pl-8 w-full sm:w-64"
            />
          </div>
          <Select v-model="chatTypeFilter">
            <SelectTrigger class="w-full sm:w-32">
              <SelectValue placeholder="Type" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="all">All</SelectItem>
              <SelectItem value="team">Team</SelectItem>
              <SelectItem value="squad">Squad</SelectItem>
              <SelectItem value="admin">Admin</SelectItem>
            </SelectContent>
          </Select>
        </div>
      </div>
    </CardHeader>
    <CardContent>
      <div v-if="loading" class="flex justify-center py-8">
        <Loader2 class="h-8 w-8 animate-spin text-muted-foreground" />
      </div>

      <div v-else-if="error" class="text-center py-8 text-destructive">
        {{ error }}
      </div>

      <div v-else-if="messages.length === 0" class="text-center py-8 text-muted-foreground">
        No chat messages found
      </div>

      <div v-else class="space-y-2">
        <div
          v-for="(msg, index) in messages"
          :key="index"
          class="p-3 rounded-md hover:bg-muted/50 transition-colors"
          :class="{ 'bg-destructive/10': msg.chat_type === 'admin' }"
        >
          <div class="flex flex-wrap items-center gap-2 mb-1">
            <Badge
              :variant="getChatTypeBadgeVariant(msg.chat_type)"
              :class="getChatTypeColor(msg.chat_type)"
              class="text-xs"
            >
              {{ msg.chat_type }}
            </Badge>
            <span class="text-xs text-muted-foreground">
              {{ formatDate(msg.sent_at) }}
            </span>
          </div>
          <div class="text-sm break-words">{{ msg.message }}</div>
        </div>
      </div>

      <!-- Pagination -->
      <div
        v-if="totalPages > 1"
        class="flex items-center justify-between mt-4 pt-4 border-t"
      >
        <div class="text-sm text-muted-foreground">
          Page {{ page }} of {{ totalPages }} ({{ total }} messages)
        </div>
        <div class="flex gap-2">
          <Button
            variant="outline"
            size="sm"
            :disabled="page <= 1"
            @click="prevPage"
          >
            <ChevronLeft class="h-4 w-4" />
          </Button>
          <Button
            variant="outline"
            size="sm"
            :disabled="page >= totalPages"
            @click="nextPage"
          >
            <ChevronRight class="h-4 w-4" />
          </Button>
        </div>
      </div>
    </CardContent>
  </Card>
</template>
