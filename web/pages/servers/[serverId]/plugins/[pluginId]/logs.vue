<script setup lang="ts">
import { ref, onMounted, onUnmounted, nextTick, watch } from "vue";
import { Button } from "~/components/ui/button";
import { Input } from "~/components/ui/input";
import {
    Select,
    SelectContent,
    SelectItem,
    SelectTrigger,
    SelectValue,
} from "~/components/ui/select";
import { Checkbox } from "~/components/ui/checkbox";
import { toast } from "~/components/ui/toast";
import {
    Dialog,
    DialogContent,
    DialogDescription,
    DialogFooter,
    DialogHeader,
    DialogTitle,
} from "~/components/ui/dialog";
import { useAuthStore } from "~/stores/auth";
import {
    ArrowLeft,
    FileText,
    Download,
    RefreshCw,
    ChevronLeft,
    ChevronRight,
    X,
    ChevronDown,
    ChevronRight as ChevronRightIcon,
    Clock,
    Hash,
    AlertCircle,
    Info,
    AlertTriangle,
    Bug,
    ArrowDown,
    Trash2,
} from "lucide-vue-next";

definePageMeta({
    middleware: ["auth"],
});

const route = useRoute();
const router = useRouter();
const serverId = route.params.serverId;
const pluginId = route.params.pluginId;
const authStore = useAuthStore();

// State variables
const loading = ref(true);
const plugin = ref<any>(null);
const logs = ref<any[]>([]);
const refreshing = ref(false);
const loadingOlder = ref(false);
const logsPerPage = ref(50);
const logLevelFilter = ref("");
const searchFilter = ref("");
const expandedLogs = ref<Set<string>>(new Set());
const showMetadata = ref(true);
const showFields = ref(true);
const hasMoreLogs = ref(true);
const oldestLogId = ref<string | null>(null);
const newestLogId = ref<string | null>(null);
const isAtBottom = ref(true);

// WebSocket state
const isConnected = ref(false);
const connecting = ref(false);
const error = ref<string | null>(null);
let websocket: WebSocket | null = null;

// Available log levels for filtering
const logLevels = ["debug", "info", "warn", "error"];

// Console-style log level colors and styling
const getLogLevelStyle = (level: string) => {
    switch (level?.toLowerCase()) {
        case "error":
            return "text-red-400";
        case "warn":
        case "warning":
            return "text-yellow-400";
        case "info":
            return "text-blue-400";
        case "debug":
            return "text-gray-400";
        default:
            return "text-gray-400";
    }
};

// Get log level icon
const getLogLevelIcon = (level: string) => {
    switch (level?.toLowerCase()) {
        case "error":
            return AlertCircle;
        case "warn":
        case "warning":
            return AlertTriangle;
        case "info":
            return Info;
        case "debug":
            return Bug;
        default:
            return Info;
    }
};

// Toggle log expansion
const toggleLogExpansion = (logId: string) => {
    if (expandedLogs.value.has(logId)) {
        expandedLogs.value.delete(logId);
    } else {
        expandedLogs.value.add(logId);
    }
};

// Format JSON fields for display
const formatFields = (fields: any) => {
    if (!fields || typeof fields !== "object") return null;
    return JSON.stringify(fields, null, 2);
};

// Check if log has additional data
const hasAdditionalData = (log: any) => {
    return log.fields && Object.keys(log.fields).length > 0;
};

// Expand all logs with additional data
const expandAllLogs = () => {
    logs.value.forEach((log) => {
        if (hasAdditionalData(log) || showMetadata.value) {
            expandedLogs.value.add(log.id);
        }
    });
};

// Collapse all logs
const collapseAllLogs = () => {
    expandedLogs.value.clear();
};

// Keyboard shortcuts
const handleKeydown = (event: KeyboardEvent) => {
    // Ctrl/Cmd + R to refresh
    if ((event.ctrlKey || event.metaKey) && event.key === "r") {
        event.preventDefault();
        refreshLogs();
    }

    // Ctrl/Cmd + E to expand all
    if ((event.ctrlKey || event.metaKey) && event.key === "e") {
        event.preventDefault();
        expandAllLogs();
    }

    // Ctrl/Cmd + Shift + E to collapse all
    if (
        (event.ctrlKey || event.metaKey) &&
        event.shiftKey &&
        event.key === "E"
    ) {
        event.preventDefault();
        collapseAllLogs();
    }
};

// Format timestamp for console view
const formatConsoleTimestamp = (timestamp: string) => {
    return new Date(timestamp).toLocaleString("en-US", {
        month: "2-digit",
        day: "2-digit",
        hour: "2-digit",
        minute: "2-digit",
        second: "2-digit",
        hour12: false,
    });
};

// Load plugin details
const loadPlugin = async () => {
    try {
        const response = await $fetch(
            `/api/servers/${serverId}/plugins/${pluginId}`,
            {
                headers: {
                    Authorization: `Bearer ${authStore.token}`,
                },
            },
        );
        plugin.value = (response as any).data.plugin;
    } catch (error: any) {
        console.error("Failed to load plugin:", error);
        toast({
            title: "Error",
            description: "Failed to load plugin details",
            variant: "destructive",
        });
    }
};

// Scroll detection and infinite scroll
const handleScroll = async (event: Event) => {
    const container = event.target as HTMLElement;
    const scrollTop = container.scrollTop;
    const scrollHeight = container.scrollHeight;
    const clientHeight = container.clientHeight;

    // Check if user is at the bottom (within 100px threshold)
    isAtBottom.value = scrollHeight - scrollTop - clientHeight < 100;

    // Load older logs when scrolling near the top (within 200px)
    if (scrollTop < 200 && !loadingOlder.value && hasMoreLogs.value) {
        await loadOlderLogs();
    }
};

// Load initial logs (latest first)
const loadInitialLogs = async () => {
    try {
        let url = `/api/servers/${serverId}/plugins/${pluginId}/logs?limit=${logsPerPage.value}&order=desc`;

        // Add filters if set
        const params = new URLSearchParams();
        if (logLevelFilter.value) params.append("level", logLevelFilter.value);
        if (searchFilter.value) params.append("search", searchFilter.value);

        if (params.toString()) {
            url += "&" + params.toString();
        }

        const response = await $fetch(url, {
            headers: {
                Authorization: `Bearer ${authStore.token}`,
            },
        });

        const newLogs = (response as any).data.logs || [];
        const reversedLogs = newLogs.slice().reverse();
        logs.value = reversedLogs;

        if (reversedLogs.length > 0) {
            newestLogId.value = reversedLogs[reversedLogs.length - 1].id;
            oldestLogId.value = reversedLogs[0].id;
        }

        hasMoreLogs.value = newLogs.length === logsPerPage.value;

        // Always scroll to bottom after initial load with a slight delay
        await nextTick();
        setTimeout(() => {
            scrollToBottom();
        }, 100);
    } catch (error: any) {
        console.error("Failed to load logs:", error);
        toast({
            title: "Error",
            description: "Failed to load plugin logs",
            variant: "destructive",
        });
    }
};

// Load older logs (for infinite scroll)
const loadOlderLogs = async () => {
    if (!hasMoreLogs.value || loadingOlder.value || !oldestLogId.value) return;

    loadingOlder.value = true;
    const container = document.getElementById("console-container");
    const previousScrollHeight = container?.scrollHeight || 0;

    try {
        let url = `/api/servers/${serverId}/plugins/${pluginId}/logs?limit=${logsPerPage.value}&order=desc&before=${oldestLogId.value}`;

        // Add filters if set
        const params = new URLSearchParams();
        if (logLevelFilter.value) params.append("level", logLevelFilter.value);
        if (searchFilter.value) params.append("search", searchFilter.value);

        if (params.toString()) {
            url += "&" + params.toString();
        }

        const response = await $fetch(url, {
            headers: {
                Authorization: `Bearer ${authStore.token}`,
            },
        });

        const olderLogs = (response as any).data.logs || [];
        const reversedOlderLogs = olderLogs.slice().reverse();

        if (reversedOlderLogs.length > 0) {
            // Prepend older logs to the beginning of the array
            logs.value = [...reversedOlderLogs, ...logs.value];
            oldestLogId.value = reversedOlderLogs[0].id;

            // Maintain scroll position
            await nextTick();
            if (container) {
                const newScrollHeight = container.scrollHeight;
                container.scrollTop = newScrollHeight - previousScrollHeight;
            }
        }

        hasMoreLogs.value = olderLogs.length === logsPerPage.value;
    } catch (error: any) {
        console.error("Failed to load older logs:", error);
        toast({
            title: "Error",
            description: "Failed to load older logs",
            variant: "destructive",
        });
    } finally {
        loadingOlder.value = false;
    }
};

// Load newer logs (for auto-refresh)
const loadNewerLogs = async () => {
    if (!newestLogId.value) return;

    try {
        let url = `/api/servers/${serverId}/plugins/${pluginId}/logs?limit=${logsPerPage.value}&order=desc&after=${newestLogId.value}`;

        // Add filters if set
        const params = new URLSearchParams();
        if (logLevelFilter.value) params.append("level", logLevelFilter.value);
        if (searchFilter.value) params.append("search", searchFilter.value);

        if (params.toString()) {
            url += "&" + params.toString();
        }

        const response = await $fetch(url, {
            headers: {
                Authorization: `Bearer ${authStore.token}`,
            },
        });

        const newerLogs = (response as any).data.logs || [];
        const reversedNewerLogs = newerLogs.slice().reverse();

        if (reversedNewerLogs.length > 0) {
            // Append newer logs to the end of the array
            logs.value = [...logs.value, ...reversedNewerLogs];
            newestLogId.value =
                reversedNewerLogs[reversedNewerLogs.length - 1].id;

            // Auto-scroll to bottom if user was already at bottom
            if (isAtBottom.value) {
                await nextTick();
                scrollToBottom();
            } else {
                toast({
                    title: "New logs available",
                    description: `${reversedNewerLogs.length} new log entries`,
                    action: {
                        label: "Scroll to bottom",
                        onClick: scrollToBottom,
                    },
                });
            }
        }
    } catch (error: any) {
        console.error("Failed to load newer logs:", error);
    }
};

// Scroll to bottom of console
const scrollToBottom = () => {
    const container = document.getElementById("console-container");
    if (container) {
        container.scrollTop = container.scrollHeight;
        // Ensure we're marked as at bottom
        isAtBottom.value = true;
    }
};

// WebSocket connection management
const connectToLogs = async () => {
    if (isConnected.value || connecting.value) return;

    connecting.value = true;
    error.value = null;

    try {
        const runtimeConfig = useRuntimeConfig();
        const cookieToken = useCookie(
            runtimeConfig.public.sessionCookieName as string,
        );
        const token = cookieToken.value;

        if (!token) {
            throw new Error("Authentication required");
        }

        // Convert HTTP/HTTPS URL to WebSocket URL
        const backendUrl = window.location.origin;
        const wsProtocol = backendUrl.startsWith("https") ? "wss" : "ws";
        const baseUrl = backendUrl.replace(/^https?:\/\//, "");
        const url = `${wsProtocol}://${baseUrl}/api/servers/${serverId}/plugins/${pluginId}/logs/ws?token=${token}`;

        websocket = new WebSocket(url);

        websocket.onopen = () => {
            isConnected.value = true;
            connecting.value = false;
            error.value = null;
        };

        websocket.onmessage = (event) => {
            try {
                const data = JSON.parse(event.data);
                handleLogEvent(data);
            } catch (err) {
                console.error("Failed to parse WebSocket message:", err);
            }
        };

        websocket.onclose = (event) => {
            console.log("WebSocket connection closed:", event);
            isConnected.value = false;
            connecting.value = false;

            // Retry connection after 5 seconds if not manually disconnected
            if (event.code !== 1000) {
                error.value = "Connection to live logs was lost";
                setTimeout(() => {
                    if (!isConnected.value) {
                        connectToLogs();
                    }
                }, 5000);
            }
        };

        websocket.onerror = (event) => {
            console.error("WebSocket error:", event);
            error.value = "Connection to live logs failed";
            isConnected.value = false;
            connecting.value = false;
        };
    } catch (err: any) {
        error.value = err.message || "Failed to connect to live logs";
        connecting.value = false;
    }
};

// Disconnect from WebSocket
const disconnectFromLogs = () => {
    if (websocket) {
        websocket.close(1000, "User disconnect");
        websocket = null;
    }
    isConnected.value = false;
    connecting.value = false;
    error.value = null;
};

// Toggle connection
const toggleConnection = () => {
    if (isConnected.value) {
        disconnectFromLogs();
    } else {
        connectToLogs();
    }
};

// Handle incoming log events (real-time)
const handleLogEvent = (event: any) => {
    // Handle connection message
    if (event.type === "connected") {
        console.log("Connected to logs:", event.message);
        return;
    }

    if (event.type === "log") {
        const maxLogs = 1000; // Limit stored logs

        // Apply filters if set
        if (logLevelFilter.value && event.level !== logLevelFilter.value) {
            return;
        }
        if (
            searchFilter.value &&
            !event.message
                .toLowerCase()
                .includes(searchFilter.value.toLowerCase())
        ) {
            return;
        }

        // Add log to the end
        logs.value.push(event);
        if (logs.value.length > maxLogs) {
            logs.value = logs.value.slice(-maxLogs);
        }

        // Update newest log ID
        newestLogId.value = event.id;

        // Auto-scroll to bottom if user was already at bottom
        if (isAtBottom.value) {
            nextTick(() => scrollToBottom());
        }
    }
};

// Refresh logs
const refreshLogs = async () => {
    refreshing.value = true;
    try {
        // Always reload from scratch for manual refresh
        await loadInitialLogs();

        toast({
            title: "Success",
            description: "Logs refreshed successfully",
        });
    } finally {
        refreshing.value = false;
    }
};

// Scroll to bottom and load newer logs
const scrollToBottomAndRefresh = async () => {
    await loadNewerLogs();
    scrollToBottom();
};

// Handle pagination (simplified for infinite scroll)
const nextPage = () => {
    // In infinite scroll, "next page" means scroll to bottom
    scrollToBottom();
};

const prevPage = () => {
    // In infinite scroll, "prev page" means scroll to top to load older logs
    const container = document.getElementById("console-container");
    if (container) {
        container.scrollTop = 0;
    }
};

// Handle filtering
const applyFilters = async () => {
    logs.value = [];
    oldestLogId.value = null;
    newestLogId.value = null;
    hasMoreLogs.value = true;
    await loadInitialLogs();
};

const clearFilters = async () => {
    logLevelFilter.value = "";
    searchFilter.value = "";
    logs.value = [];
    oldestLogId.value = null;
    newestLogId.value = null;
    hasMoreLogs.value = true;
    await loadInitialLogs();
};

// Export logs
const exportLogs = () => {
    const logData = logs.value.map((log) => ({
        id: log.id,
        timestamp: log.timestamp,
        level: log.level,
        message: log.message,
        error_message: log.error_message,
        fields: log.fields,
        ingested_at: log.ingested_at,
    }));

    const blob = new Blob([JSON.stringify(logData, null, 2)], {
        type: "application/json",
    });
    const url = URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url;
    a.download = `plugin-${plugin.value?.name || pluginId}-logs-${new Date().toISOString().split("T")[0]}.json`;
    document.body.appendChild(a);
    a.click();
    document.body.removeChild(a);
    URL.revokeObjectURL(url);
};

// Go back to plugins list
const goBack = () => {
    router.push(`/servers/${serverId}/plugins`);
};

// Load plugin data

onMounted(async () => {
    loading.value = true;
    try {
        await Promise.all([loadPlugin(), loadInitialLogs()]);

        // Connect to WebSocket for live updates
        connectToLogs();

        // Add keyboard event listeners
        document.addEventListener("keydown", handleKeydown);

        // Add scroll listener to console container</parameter>
        // Add scroll listener to console container
        const container = document.getElementById("console-container");
        if (container) {
            container.addEventListener("scroll", handleScroll);
        }
    } finally {
        loading.value = false;
    }
});

// Cleanup on unmount
onUnmounted(() => {
    // Disconnect from WebSocket
    disconnectFromLogs();

    document.removeEventListener("keydown", handleKeydown);

    const container = document.getElementById("console-container");
    if (container) {
        container.removeEventListener("scroll", handleScroll);
    }
});
</script>

<template>
    <div class="flex flex-col h-screen overflow-hidden">
        <!-- Header - Fixed at top -->
        <div
            class="flex-shrink-0 border-b bg-background/95 backdrop-blur supports-[backdrop-filter]:bg-background/60"
        >
            <div class="p-3 sm:p-4">
                <div
                    class="flex flex-col lg:flex-row lg:items-center justify-between gap-3 sm:gap-4"
                >
                    <div
                        class="flex flex-col sm:flex-row sm:items-center gap-2 sm:gap-4"
                    >
                        <Button variant="outline" @click="goBack" size="sm" class="w-fit">
                            <ArrowLeft class="w-4 h-4 sm:mr-2" />
                            <span class="hidden sm:inline">Back</span>
                        </Button>
                        <div>
                            <h1 class="text-lg sm:text-xl font-bold">
                                {{ plugin?.plugin_name || "Plugin" }} Plugin
                                Logs
                            </h1>
                            <p class="text-xs sm:text-sm text-muted-foreground">
                                Live log stream • {{ logs.length }} entries
                            </p>
                        </div>
                    </div>

                    <div class="flex items-center gap-1 sm:gap-2 flex-wrap">
                        <!-- WebSocket Connection Toggle -->
                        <Button
                            :variant="isConnected ? 'default' : 'outline'"
                            size="sm"
                            @click="toggleConnection"
                            :disabled="connecting"
                            class="flex-shrink-0"
                        >
                            <div
                                class="w-2 h-2 rounded-full sm:mr-2"
                                :class="{
                                    'bg-green-500 animate-pulse': isConnected,
                                    'bg-yellow-500 animate-pulse': connecting,
                                    'bg-red-500': !isConnected && !connecting,
                                }"
                            ></div>
                            <span class="hidden sm:inline">{{
                                isConnected
                                    ? "Live"
                                    : connecting
                                      ? "Connecting..."
                                      : "Connect"
                            }}</span>
                        </Button>

                        <Button
                            variant="outline"
                            size="sm"
                            @click="expandAllLogs"
                            :disabled="logs.length === 0"
                            title="Ctrl+E"
                            class="hidden md:inline-flex"
                        >
                            <ChevronDown class="w-4 h-4 mr-2" />
                            Expand All
                        </Button>
                        <Button
                            variant="outline"
                            size="sm"
                            @click="collapseAllLogs"
                            :disabled="expandedLogs.size === 0"
                            title="Ctrl+Shift+E"
                            class="hidden md:inline-flex"
                        >
                            <ChevronRightIcon class="w-4 h-4 mr-2" />
                            Collapse All
                        </Button>
                        <Button
                            variant="outline"
                            size="sm"
                            @click="refreshLogs"
                            :disabled="refreshing"
                            title="Ctrl+R"
                            class="flex-shrink-0"
                        >
                            <RefreshCw
                                class="w-4 h-4 sm:mr-2"
                                :class="{ 'animate-spin': refreshing }"
                            />
                            <span class="hidden sm:inline">Refresh</span>
                        </Button>
                        <Button
                            variant="outline"
                            size="sm"
                            @click="scrollToBottomAndRefresh"
                            class="hidden sm:inline-flex"
                        >
                            <ArrowDown class="w-4 h-4 mr-2" />
                            Latest
                        </Button>

                        <Button
                            variant="outline"
                            size="sm"
                            @click="exportLogs"
                            :disabled="logs.length === 0"
                            class="flex-shrink-0"
                        >
                            <Download class="w-4 h-4 sm:mr-2" />
                            <span class="hidden sm:inline">Export</span>
                        </Button>
                    </div>
                </div>

                <!-- Compact Filters -->
                <div class="flex flex-col sm:flex-row gap-2 mt-3 sm:mt-4">
                    <div class="flex-1 min-w-0">
                        <Input
                            v-model="searchFilter"
                            placeholder="Search messages..."
                            size="sm"
                            @keyup.enter="applyFilters"
                            class="text-sm w-full"
                        />
                    </div>
                    <div class="flex gap-2">
                        <Select v-model="logLevelFilter">
                            <SelectTrigger class="w-full sm:w-32">
                                <SelectValue placeholder="All" />
                            </SelectTrigger>
                            <SelectContent>
                                <SelectItem value="all">All</SelectItem>
                                <SelectItem
                                    v-for="level in logLevels"
                                    :key="level"
                                    :value="level"
                                >
                                    {{ level.toUpperCase() }}
                                </SelectItem>
                            </SelectContent>
                        </Select>
                        <Button @click="applyFilters" size="sm" variant="outline" class="flex-shrink-0">
                            <span class="hidden sm:inline">Apply</span>
                            <span class="sm:hidden">Filter</span>
                        </Button>
                        <Button @click="clearFilters" size="sm" variant="outline" class="flex-shrink-0">
                            <X class="w-4 h-4" />
                        </Button>
                    </div>
                </div>

                <!-- Display Options -->
                <div
                    class="flex flex-wrap gap-2 mt-3 pt-3 border-t border-border/50"
                >
                    <div class="flex items-center space-x-2">
                        <Checkbox id="show-metadata" v-model="showMetadata" />
                        <label
                            for="show-metadata"
                            class="text-sm text-muted-foreground"
                            >Show Metadata</label
                        >
                    </div>
                    <div class="flex items-center space-x-2">
                        <Checkbox id="show-fields" v-model="showFields" />
                        <label
                            for="show-fields"
                            class="text-sm text-muted-foreground"
                            >Show Fields</label
                        >
                    </div>
                </div>
            </div>
        </div>

        <div v-if="loading" class="flex-1 flex items-center justify-center">
            <div
                class="animate-spin rounded-full h-8 w-8 border-b-2 border-primary"
            ></div>
        </div>

        <!-- Console View - Scrollable content area -->
        <div v-else class="flex-1 overflow-hidden">
            <!-- Loading indicator for older logs -->
            <div
                v-if="loadingOlder"
                class="bg-gray-900 text-center py-2 text-sm text-gray-400 border-b border-gray-800"
            >
                <div class="flex items-center justify-center gap-2">
                    <div
                        class="animate-spin rounded-full h-4 w-4 border-b-2 border-blue-400"
                    ></div>
                    Loading older logs...
                </div>
            </div>

            <!-- Console Container -->
            <div
                class="h-full bg-black text-green-400 font-mono text-xs sm:text-sm overflow-auto p-2 sm:p-4"
                id="console-container"
                @scroll="handleScroll"
            >
                <div
                    v-if="logs.length === 0"
                    class="text-center py-8 text-gray-500"
                >
                    <FileText class="w-16 h-16 mx-auto mb-4" />
                    <p>No log entries found</p>
                </div>

                <!-- Log entries -->
                <div v-else class="space-y-1">
                    <div
                        v-for="log in logs"
                        :key="log.id"
                        class="border border-gray-800 rounded-lg hover:border-gray-700 transition-colors"
                        :class="{ 'bg-gray-900/30': expandedLogs.has(log.id) }"
                    >
                        <!-- Main log line -->
                        <div
                            class="flex items-start text-xs leading-relaxed hover:bg-gray-900/50 px-2 sm:px-3 py-2 cursor-pointer"
                            @click="
                                hasAdditionalData(log) || showMetadata
                                    ? toggleLogExpansion(log.id)
                                    : null
                            "
                            :class="{
                                'cursor-default':
                                    !hasAdditionalData(log) && !showMetadata,
                            }"
                        >
                            <!-- Expand/Collapse indicator -->
                            <div
                                class="flex-shrink-0 w-3 sm:w-4 mr-1 sm:mr-2 flex items-center"
                            >
                                <component
                                    v-if="
                                        hasAdditionalData(log) || showMetadata
                                    "
                                    :is="
                                        expandedLogs.has(log.id)
                                            ? ChevronDown
                                            : ChevronRightIcon
                                    "
                                    class="w-3 h-3 text-gray-500"
                                />
                            </div>

                            <!-- Timestamp -->
                            <div
                                class="text-gray-500 mr-1.5 sm:mr-3 flex-shrink-0 text-[10px] sm:text-xs"
                            >
                                <div class="whitespace-nowrap">
                                    {{ formatConsoleTimestamp(log.timestamp) }}
                                </div>
                            </div>

                            <!-- Level Badge with Icon -->
                            <div
                                class="flex items-center mr-1.5 sm:mr-3 flex-shrink-0"
                            >
                                <component
                                    :is="getLogLevelIcon(log.level)"
                                    class="w-3 h-3 sm:mr-1"
                                    :class="getLogLevelStyle(log.level)"
                                />
                                <span
                                    :class="getLogLevelStyle(log.level)"
                                    class="font-bold text-[10px] sm:text-xs hidden sm:inline"
                                >
                                    {{
                                        log.level?.toUpperCase().substring(0, 4)
                                    }}
                                </span>
                            </div>

                            <!-- Message -->
                            <span class="flex-1 break-words">
                                {{ log.message }}
                                <span
                                    v-if="log.error_message"
                                    class="text-red-400 ml-2"
                                >
                                    [ERROR: {{ log.error_message }}]
                                </span>
                            </span>

                            <!-- Data indicators -->
                            <div
                                class="flex items-center gap-1 ml-1 sm:ml-2 flex-shrink-0"
                            >
                                <Hash
                                    v-if="showMetadata"
                                    class="w-3 h-3 text-gray-600"
                                    title="Has ID"
                                />
                                <Database
                                    v-if="hasAdditionalData(log)"
                                    class="w-3 h-3 text-blue-500"
                                    title="Has structured data"
                                />
                            </div>
                        </div>

                        <!-- Expanded content -->
                        <div
                            v-if="expandedLogs.has(log.id)"
                            class="border-t border-gray-800 bg-gray-950/50 px-6 py-3 text-xs"
                        >
                            <!-- Metadata section -->
                            <div v-if="showMetadata" class="mb-4">
                                <div
                                    class="text-gray-400 font-semibold mb-2 flex items-center"
                                >
                                    <Hash class="w-3 h-3 mr-1" />
                                    Metadata
                                </div>
                                <div
                                    class="grid grid-cols-1 md:grid-cols-2 gap-3 text-gray-300"
                                >
                                    <div class="flex">
                                        <span
                                            class="text-gray-500 w-20 flex-shrink-0"
                                            >ID:</span
                                        >
                                        <span
                                            class="font-mono text-yellow-400"
                                            >{{ log.id }}</span
                                        >
                                    </div>
                                    <div class="flex">
                                        <span
                                            class="text-gray-500 w-20 flex-shrink-0"
                                            >Ingested:</span
                                        >
                                        <span class="font-mono text-blue-400">{{
                                            formatConsoleTimestamp(
                                                log.ingested_at ||
                                                    log.timestamp,
                                            )
                                        }}</span>
                                    </div>
                                    <div class="flex md:col-span-2">
                                        <span
                                            class="text-gray-500 w-20 flex-shrink-0"
                                            >Full Time:</span
                                        >
                                        <span
                                            class="font-mono text-green-400"
                                            >{{
                                                new Date(
                                                    log.timestamp,
                                                ).toISOString()
                                            }}</span
                                        >
                                    </div>
                                </div>
                            </div>

                            <!-- Error message (if different from main message) -->
                            <div
                                v-if="
                                    log.error_message &&
                                    log.error_message !== log.message
                                "
                                class="mb-4"
                            >
                                <div
                                    class="text-red-400 font-semibold mb-2 flex items-center"
                                >
                                    <AlertCircle class="w-3 h-3 mr-1" />
                                    Error Details
                                </div>
                                <div
                                    class="bg-red-950/30 border border-red-800/50 rounded p-3 text-red-300 font-mono whitespace-pre-wrap"
                                >
                                    {{ log.error_message }}
                                </div>
                            </div>

                            <!-- Structured fields -->
                            <div
                                v-if="showFields && hasAdditionalData(log)"
                                class="mb-2"
                            >
                                <div
                                    class="text-blue-400 font-semibold mb-2 flex items-center"
                                >
                                    <Database class="w-3 h-3 mr-1" />
                                    Structured Data
                                </div>
                                <div
                                    class="bg-blue-950/20 border border-blue-800/30 rounded p-3"
                                >
                                    <pre
                                        class="text-blue-200 text-xs whitespace-pre-wrap overflow-x-auto"
                                        >{{ formatFields(log.fields) }}</pre
                                    >
                                </div>
                            </div>

                            <!-- Raw log data toggle -->
                            <details class="mt-3">
                                <summary
                                    class="text-gray-400 hover:text-gray-300 cursor-pointer text-xs"
                                >
                                    View Raw JSON
                                </summary>
                                <div
                                    class="mt-2 bg-gray-900/50 border border-gray-700 rounded p-3"
                                >
                                    <pre
                                        class="text-gray-300 text-xs whitespace-pre-wrap overflow-x-auto"
                                        >{{ JSON.stringify(log, null, 2) }}</pre
                                    >
                                </div>
                            </details>
                        </div>
                    </div>
                </div>
            </div>
        </div>

        <!-- Footer with status - Fixed at bottom -->
        <div
            v-if="logs.length > 0"
            class="flex-shrink-0 border-t bg-background/95 backdrop-blur supports-[backdrop-filter]:bg-background/60"
        >
            <div class="p-2 sm:p-3">
                <div class="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-2 text-xs sm:text-sm">
                    <div class="text-muted-foreground flex flex-wrap items-center gap-2 sm:gap-4">
                        <span>{{ logs.length }} entries loaded</span>
                        <span class="flex items-center gap-1">
                            <Clock class="w-3 h-3" />
                            {{ expandedLogs.size }} expanded
                        </span>
                        <span v-if="logs.length > 0" class="hidden md:inline text-xs">
                            Latest:
                            {{
                                formatConsoleTimestamp(
                                    logs[logs.length - 1]?.timestamp,
                                )
                            }}
                        </span>
                        <span v-if="loadingOlder" class="text-blue-400 text-xs">
                            Loading older logs...
                        </span>
                        <span v-if="!hasMoreLogs" class="text-gray-500 text-xs hidden sm:inline">
                            No more logs to load
                        </span>
                    </div>
                    <div class="flex items-center gap-1 sm:gap-2 w-full sm:w-auto">
                        <Button
                            variant="outline"
                            size="sm"
                            @click="prevPage"
                            :disabled="!hasMoreLogs || loadingOlder"
                            title="Scroll to top to load older logs"
                            class="flex-1 sm:flex-initial"
                        >
                            <ChevronLeft class="w-4 h-4 sm:mr-1" />
                            <span class="hidden sm:inline">Older</span>
                        </Button>
                        <Button
                            variant="outline"
                            size="sm"
                            @click="scrollToBottomAndRefresh"
                            :disabled="refreshing"
                            title="Jump to latest logs"
                            class="flex-1 sm:flex-initial"
                        >
                            <ArrowDown class="w-4 h-4 sm:mr-1" />
                            <span class="hidden sm:inline">Latest</span>
                        </Button>

                        <!-- Status indicators -->
                        <div class="hidden sm:flex items-center gap-2 ml-2">
                            <div
                                v-if="!isAtBottom"
                                class="flex items-center gap-1 text-orange-400 text-xs"
                            >
                                <ArrowDown class="w-3 h-3" />
                                New logs available
                            </div>
                            <div
                                class="text-xs text-muted-foreground hidden lg:block"
                            >
                                Scroll up for older logs • Shortcuts: Ctrl+R
                                (refresh) • Ctrl+E (expand)
                            </div>
                        </div>
                    </div>
                </div>
            </div>
        </div>

        <!-- Floating "new logs" indicator -->
        <div
            v-if="!isAtBottom && logs.length > 0"
            class="fixed bottom-16 sm:bottom-20 right-3 sm:right-6 z-10"
        >
            <Button
                @click="scrollToBottom"
                class="shadow-lg bg-blue-600 hover:bg-blue-700 text-white"
                size="sm"
            >
                <ArrowDown class="w-4 h-4 sm:mr-2" />
                <span class="hidden sm:inline">New logs available</span>
            </Button>
        </div>
    </div>
</template>
