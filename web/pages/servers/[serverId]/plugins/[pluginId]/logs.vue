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

// Format relative time
const getRelativeTime = (timestamp: string) => {
    const now = new Date();
    const logTime = new Date(timestamp);
    const diff = now.getTime() - logTime.getTime();

    const seconds = Math.floor(diff / 1000);
    const minutes = Math.floor(seconds / 60);
    const hours = Math.floor(minutes / 60);
    const days = Math.floor(hours / 24);

    if (days > 0) return `${days}d ago`;
    if (hours > 0) return `${hours}h ago`;
    if (minutes > 0) return `${minutes}m ago`;
    return `${seconds}s ago`;
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

        // Add keyboard event listeners
        document.addEventListener("keydown", handleKeydown);

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
            <div class="p-4">
                <div
                    class="flex flex-col lg:flex-row lg:items-center justify-between gap-4"
                >
                    <div
                        class="flex flex-col sm:flex-row sm:items-center gap-4"
                    >
                        <Button variant="outline" @click="goBack" size="sm">
                            <ArrowLeft class="w-4 h-4 mr-2" />
                            Back
                        </Button>
                        <div>
                            <h1 class="text-xl font-bold">
                                {{ plugin?.plugin_name || "Plugin" }} Plugin
                                Logs
                            </h1>
                            <p class="text-sm text-muted-foreground">
                                Live log stream • {{ logs.length }} entries
                            </p>
                        </div>
                    </div>

                    <div class="flex items-center gap-2">
                        <Button
                            variant="outline"
                            size="sm"
                            @click="expandAllLogs"
                            :disabled="logs.length === 0"
                            title="Ctrl+E"
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
                        >
                            <RefreshCw
                                class="w-4 h-4 mr-2"
                                :class="{ 'animate-spin': refreshing }"
                            />
                            Refresh
                        </Button>
                        <Button
                            variant="outline"
                            size="sm"
                            @click="scrollToBottomAndRefresh"
                        >
                            <ArrowDown class="w-4 h-4 mr-2" />
                            Latest
                        </Button>

                        <Button
                            variant="outline"
                            size="sm"
                            @click="exportLogs"
                            :disabled="logs.length === 0"
                        >
                            <Download class="w-4 h-4 mr-2" />
                            Export
                        </Button>
                    </div>
                </div>

                <!-- Compact Filters -->
                <div class="flex flex-col sm:flex-row gap-2 mt-4">
                    <div class="flex-1">
                        <Input
                            v-model="searchFilter"
                            placeholder="Search messages..."
                            size="sm"
                            @keyup.enter="applyFilters"
                            class="text-sm"
                        />
                    </div>
                    <Select v-model="logLevelFilter">
                        <SelectTrigger class="w-32">
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
                    <Button @click="applyFilters" size="sm" variant="outline">
                        Apply
                    </Button>
                    <Button @click="clearFilters" size="sm" variant="outline">
                        <X class="w-4 h-4" />
                    </Button>
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
                class="h-full bg-black text-green-400 font-mono text-sm overflow-auto p-4"
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
                            class="flex items-start text-xs leading-relaxed hover:bg-gray-900/50 px-3 py-2 cursor-pointer"
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
                                class="flex-shrink-0 w-4 mr-2 flex items-center"
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
                                class="text-gray-500 mr-3 flex-shrink-0 w-20 text-xs"
                            >
                                <div>
                                    {{ formatConsoleTimestamp(log.timestamp) }}
                                </div>
                                <div class="text-gray-600 text-xs">
                                    {{ getRelativeTime(log.timestamp) }}
                                </div>
                            </div>

                            <!-- Level Badge with Icon -->
                            <div
                                class="flex items-center mr-3 flex-shrink-0 w-16"
                            >
                                <component
                                    :is="getLogLevelIcon(log.level)"
                                    class="w-3 h-3 mr-1"
                                    :class="getLogLevelStyle(log.level)"
                                />
                                <span
                                    :class="getLogLevelStyle(log.level)"
                                    class="text-right font-bold text-xs"
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
                                class="flex items-center gap-1 ml-2 flex-shrink-0"
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
            <div class="p-3">
                <div class="flex items-center justify-between text-sm">
                    <div class="text-muted-foreground flex items-center gap-4">
                        <span>{{ logs.length }} entries loaded</span>
                        <span class="flex items-center gap-1">
                            <Clock class="w-3 h-3" />
                            {{ expandedLogs.size }} expanded
                        </span>
                        <span v-if="logs.length > 0" class="text-xs">
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
                        <span v-if="!hasMoreLogs" class="text-gray-500 text-xs">
                            No more logs to load
                        </span>
                    </div>
                    <div class="flex items-center space-x-2">
                        <Button
                            variant="outline"
                            size="sm"
                            @click="prevPage"
                            :disabled="!hasMoreLogs || loadingOlder"
                            title="Scroll to top to load older logs"
                        >
                            <ChevronLeft class="w-4 h-4" />
                            Older
                        </Button>
                        <Button
                            variant="outline"
                            size="sm"
                            @click="scrollToBottomAndRefresh"
                            :disabled="refreshing"
                            title="Jump to latest logs"
                        >
                            <ArrowDown class="w-4 h-4" />
                            Latest
                        </Button>

                        <!-- Status indicators -->
                        <div class="ml-4 flex items-center gap-2">
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
            class="fixed bottom-20 right-6 z-10"
        >
            <Button
                @click="scrollToBottom"
                class="shadow-lg bg-blue-600 hover:bg-blue-700 text-white"
                size="sm"
            >
                <ArrowDown class="w-4 h-4 mr-2" />
                New logs available
            </Button>
        </div>
    </div>
</template>
