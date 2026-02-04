<template>
    <div class="space-y-4">
        <!-- Summary Statistics -->
        <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-3 sm:gap-4">
            <Card>
                <CardHeader class="flex flex-row items-center justify-between space-y-0 pb-2">
                    <CardTitle class="text-xs sm:text-sm font-medium">Total Executions</CardTitle>
                    <Activity class="w-3 h-3 sm:w-4 sm:h-4 text-muted-foreground" />
                </CardHeader>
                <CardContent>
                    <div class="text-xl sm:text-2xl font-bold">{{ totalExecutions }}</div>
                </CardContent>
            </Card>

            <Card>
                <CardHeader class="flex flex-row items-center justify-between space-y-0 pb-2">
                    <CardTitle class="text-xs sm:text-sm font-medium">Success Rate</CardTitle>
                    <TrendingUp class="w-3 h-3 sm:w-4 sm:h-4 text-green-600" />
                </CardHeader>
                <CardContent>
                    <div class="text-xl sm:text-2xl font-bold text-green-600">{{ successRate }}%</div>
                </CardContent>
            </Card>

            <Card>
                <CardHeader class="flex flex-row items-center justify-between space-y-0 pb-2">
                    <CardTitle class="text-xs sm:text-sm font-medium">Avg Duration</CardTitle>
                    <Clock class="w-3 h-3 sm:w-4 sm:h-4 text-muted-foreground" />
                </CardHeader>
                <CardContent>
                    <div class="text-xl sm:text-2xl font-bold">{{ avgDuration }}</div>
                </CardContent>
            </Card>

            <Card>
                <CardHeader class="flex flex-row items-center justify-between space-y-0 pb-2">
                    <CardTitle class="text-xs sm:text-sm font-medium">Running Now</CardTitle>
                    <PlayCircle class="w-3 h-3 sm:w-4 sm:h-4 text-blue-600" />
                </CardHeader>
                <CardContent>
                    <div class="text-xl sm:text-2xl font-bold text-blue-600">{{ runningCount }}</div>
                </CardContent>
            </Card>
        </div>

        <!-- Filters -->
        <div class="flex flex-col sm:flex-row gap-2 sm:gap-4 items-stretch sm:items-center">
            <div class="flex-1">
                <Input
                    v-model="searchQuery"
                    placeholder="Search executions..."
                    class="w-full"
                />
            </div>
            <Select v-model="statusFilter">
                <SelectTrigger class="w-full sm:w-[180px]">
                    <SelectValue placeholder="Filter by status" />
                </SelectTrigger>
                <SelectContent>
                    <SelectItem value="all">All Statuses</SelectItem>
                    <SelectItem value="running">Running</SelectItem>
                    <SelectItem value="completed">Completed</SelectItem>
                    <SelectItem value="failed">Failed</SelectItem>
                </SelectContent>
            </Select>
            <Button @click="() => { loadStats(); loadExecutions(false); }" :disabled="loading" variant="outline" class="w-full sm:w-auto">
                <RefreshCw class="h-4 w-4 mr-2" :class="{ 'animate-spin': loading }" />
                Refresh
            </Button>
        </div>

        <!-- Loading State -->
        <div v-if="loading && executions.length === 0" class="flex items-center justify-center py-12">
            <Loader2 class="h-8 w-8 animate-spin" />
        </div>

        <!-- Error State -->
        <div v-else-if="error" class="border border-red-200 bg-red-50 p-4 rounded-lg">
            <div class="flex items-center gap-2 text-red-800">
                <AlertCircle class="h-4 w-4" />
                <h3 class="font-medium">Error</h3>
            </div>
            <p class="text-red-700 mt-1">{{ error }}</p>
        </div>

        <!-- Timeline -->
        <div v-else-if="filteredExecutions.length > 0" class="space-y-6">
            <div v-for="(group, date) in groupedExecutions" :key="date" class="space-y-3">
                <!-- Date Header -->
                <div class="flex items-center gap-3">
                    <div class="text-sm font-medium text-muted-foreground">{{ date }}</div>
                    <div class="flex-1 h-px bg-border"></div>
                </div>

                <!-- Execution Cards -->
                <div class="space-y-3 relative">
                    <!-- Timeline Line -->
                    <div class="absolute left-6 top-0 bottom-0 w-px bg-border hidden sm:block"></div>

                    <div
                        v-for="execution in group"
                        :key="execution.id"
                        class="relative pl-0 sm:pl-12"
                    >
                        <!-- Timeline Dot -->
                        <div
                            class="absolute left-4 top-4 w-4 h-4 rounded-full border-4 border-background hidden sm:block"
                            :class="getStatusBgColor(execution.status)"
                        ></div>

                        <!-- Execution Card -->
                        <Card
                            class="cursor-pointer hover:shadow-md transition-all"
                            :class="[
                                'border-l-4',
                                getStatusBorderColor(execution.status)
                            ]"
                            @click="toggleExpand(execution.id)"
                        >
                            <CardContent class="p-3 sm:p-4">
                                <div class="flex items-start justify-between gap-2">
                                    <div class="flex-1 min-w-0 space-y-2">
                                        <!-- Header Row -->
                                        <div class="flex items-center gap-2 flex-wrap">
                                            <component
                                                :is="getStatusIcon(execution.status)"
                                                :class="getStatusColor(execution.status)"
                                                class="h-4 w-4 shrink-0"
                                            />
                                            <Badge
                                                :variant="getStatusVariant(execution.status)"
                                                class="text-xs"
                                            >
                                                {{ execution.status }}
                                            </Badge>
                                            <Badge variant="outline" class="text-xs font-mono">
                                                {{ execution.execution_id?.substring(0, 8) || execution.id.substring(0, 8) }}
                                            </Badge>
                                            <span class="text-xs text-muted-foreground">
                                                {{ formatTime(execution.started_at) }}
                                            </span>
                                        </div>

                                        <!-- Details Row -->
                                        <div class="flex flex-wrap items-center gap-x-4 gap-y-1 text-xs sm:text-sm text-muted-foreground">
                                            <div class="flex items-center gap-1">
                                                <Clock class="h-3 w-3" />
                                                <span>{{ formatDuration(execution.started_at, execution.completed_at) }}</span>
                                            </div>
                                            <div v-if="execution.trigger_data" class="flex items-center gap-1">
                                                <Zap class="h-3 w-3" />
                                                <span>{{ getTriggerDescription(execution.trigger_data) }}</span>
                                            </div>
                                        </div>

                                        <!-- Step Counts -->
                                        <div
                                            v-if="execution.completed_steps || execution.failed_steps"
                                            class="flex gap-3 text-xs"
                                        >
                                            <span v-if="execution.completed_steps" class="flex items-center gap-1 text-green-600">
                                                <CheckCircle class="h-3 w-3" />
                                                {{ execution.completed_steps }} completed
                                            </span>
                                            <span v-if="execution.failed_steps" class="flex items-center gap-1 text-red-600">
                                                <XCircle class="h-3 w-3" />
                                                {{ execution.failed_steps }} failed
                                            </span>
                                            <span v-if="execution.skipped_steps" class="flex items-center gap-1 text-gray-600">
                                                <SkipForward class="h-3 w-3" />
                                                {{ execution.skipped_steps }} skipped
                                            </span>
                                        </div>

                                        <!-- Error Message -->
                                        <div v-if="execution.error" class="mt-2">
                                            <div class="border border-red-200 bg-red-50 p-2 rounded text-xs">
                                                <div class="flex items-center gap-1 text-red-800">
                                                    <AlertCircle class="h-3 w-3" />
                                                    <span class="font-medium">Error:</span>
                                                </div>
                                                <p class="mt-1 text-red-700 break-words">{{ execution.error }}</p>
                                            </div>
                                        </div>

                                        <!-- Expanded Details -->
                                        <div v-if="expandedIds.has(execution.id)" class="mt-3 pt-3 border-t space-y-2">
                                            <div v-if="execution.trigger_data" class="text-xs">
                                                <Label class="text-xs font-medium">Trigger Data</Label>
                                                <div class="bg-muted p-2 rounded mt-1 overflow-auto max-h-32">
                                                    <pre class="text-xs">{{ JSON.stringify(execution.trigger_data, null, 2) }}</pre>
                                                </div>
                                            </div>
                                        </div>
                                    </div>

                                    <!-- Action Buttons -->
                                    <div class="flex items-center gap-1 shrink-0">
                                        <Button
                                            size="sm"
                                            variant="ghost"
                                            @click.stop="toggleExpand(execution.id)"
                                        >
                                            <component
                                                :is="expandedIds.has(execution.id) ? ChevronUp : ChevronDown"
                                                class="h-4 w-4"
                                            />
                                        </Button>
                                        <RouterLink
                                            :to="`/servers/${props.serverId}/workflows/${props.workflowId}/executions/${execution.execution_id || execution.id}`"
                                            @click.stop
                                        >
                                            <Button size="sm" variant="outline">
                                                <ExternalLink class="h-4 w-4" />
                                            </Button>
                                        </RouterLink>
                                    </div>
                                </div>
                            </CardContent>
                        </Card>
                    </div>
                </div>
            </div>

            <!-- Pagination -->
            <div v-if="hasMore" class="flex justify-center pt-4">
                <Button variant="outline" @click="loadMore" :disabled="loading" class="w-full sm:w-auto">
                    <ChevronDown class="h-4 w-4 mr-2" />
                    Load More Executions
                </Button>
            </div>
        </div>

        <!-- No Executions -->
        <div v-else class="text-center py-12 text-muted-foreground">
            <PlayCircle class="h-16 w-16 mx-auto mb-4 opacity-25" />
            <p class="text-base">No executions found</p>
            <p class="text-sm mt-1">
                {{ statusFilter !== 'all' ? 'Try changing the filter' : 'This workflow hasn\'t been executed yet' }}
            </p>
        </div>
    </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from "vue";
import {
    RefreshCw,
    Loader2,
    AlertCircle,
    Clock,
    ChevronDown,
    ChevronUp,
    PlayCircle,
    ExternalLink,
    CheckCircle,
    XCircle,
    Activity,
    TrendingUp,
    Zap,
    SkipForward,
} from "lucide-vue-next";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
    Select,
    SelectContent,
    SelectItem,
    SelectTrigger,
    SelectValue,
} from "@/components/ui/select";

interface WorkflowExecution {
    id: string;
    workflow_id: string;
    execution_id: string;
    status: string;
    started_at: string;
    completed_at?: string;
    trigger_data?: any;
    error?: string;
    completed_steps?: number;
    failed_steps?: number;
    skipped_steps?: number;
    total_steps?: number;
}

interface ApiResponse<T = any> {
    success: boolean;
    message?: string;
    data: T;
    code?: number;
}

interface Props {
    workflowId: string;
    serverId: string;
}

const props = defineProps<Props>();

// Stats interface
interface WorkflowStats {
    total_executions: number;
    success_rate: number;
    avg_duration_ms: number | null;
    running_count: number;
}

// State
const loading = ref(false);
const error = ref<string | null>(null);
const executions = ref<WorkflowExecution[]>([]);
const stats = ref<WorkflowStats | null>(null);
const offset = ref(0);
const limit = ref(20);
const hasMore = ref(true);
const expandedIds = ref<Set<string>>(new Set());
const searchQuery = ref("");
const statusFilter = ref("all");

// Computed
const filteredExecutions = computed(() => {
    let filtered = executions.value;

    // Filter by status
    if (statusFilter.value !== "all") {
        filtered = filtered.filter(
            (ex) => ex.status.toLowerCase() === statusFilter.value.toLowerCase()
        );
    }

    // Filter by search query
    if (searchQuery.value.trim()) {
        const query = searchQuery.value.toLowerCase();
        filtered = filtered.filter(
            (ex) =>
                ex.id.toLowerCase().includes(query) ||
                ex.execution_id?.toLowerCase().includes(query) ||
                ex.status.toLowerCase().includes(query) ||
                JSON.stringify(ex.trigger_data).toLowerCase().includes(query)
        );
    }

    return filtered;
});

const groupedExecutions = computed(() => {
    const groups: Record<string, WorkflowExecution[]> = {};

    filteredExecutions.value.forEach((execution) => {
        const date = formatDate(execution.started_at);
        if (!groups[date]) {
            groups[date] = [];
        }
        groups[date].push(execution);
    });

    return groups;
});

const totalExecutions = computed(() => stats.value?.total_executions ?? executions.value.length);

const successRate = computed(() => {
    if (stats.value) {
        return Math.round(stats.value.success_rate);
    }
    // Fallback to client-side calculation
    if (executions.value.length === 0) return 0;
    const completed = executions.value.filter(
        (ex) => ex.status.toLowerCase() === "completed"
    ).length;
    return Math.round((completed / executions.value.length) * 100);
});

const avgDuration = computed(() => {
    if (stats.value?.avg_duration_ms != null) {
        return formatDurationMs(stats.value.avg_duration_ms);
    }
    // Fallback to client-side calculation
    const completedExecutions = executions.value.filter(
        (ex) => ex.completed_at && ex.started_at
    );
    if (completedExecutions.length === 0) return "N/A";

    const totalMs = completedExecutions.reduce((sum, ex) => {
        const start = new Date(ex.started_at).getTime();
        const end = new Date(ex.completed_at!).getTime();
        return sum + (end - start);
    }, 0);

    const avgMs = totalMs / completedExecutions.length;
    return formatDurationMs(avgMs);
});

const runningCount = computed(() => {
    if (stats.value) {
        return stats.value.running_count;
    }
    // Fallback to client-side calculation
    return executions.value.filter(
        (ex) => ex.status.toLowerCase() === "running" || ex.status.toLowerCase() === "executing"
    ).length;
});

// Methods
const getStatusIcon = (status: string) => {
    switch (status.toLowerCase()) {
        case "completed":
            return CheckCircle;
        case "failed":
        case "error":
            return XCircle;
        case "running":
        case "executing":
            return PlayCircle;
        default:
            return Clock;
    }
};

const getStatusColor = (status: string) => {
    switch (status.toLowerCase()) {
        case "completed":
            return "text-green-600";
        case "failed":
        case "error":
            return "text-red-600";
        case "running":
        case "executing":
            return "text-blue-600";
        default:
            return "text-gray-600";
    }
};

const getStatusBgColor = (status: string) => {
    switch (status.toLowerCase()) {
        case "completed":
            return "bg-green-600";
        case "failed":
        case "error":
            return "bg-red-600";
        case "running":
        case "executing":
            return "bg-blue-600";
        default:
            return "bg-gray-600";
    }
};

const getStatusBorderColor = (status: string) => {
    switch (status.toLowerCase()) {
        case "completed":
            return "border-l-green-600";
        case "failed":
        case "error":
            return "border-l-red-600";
        case "running":
        case "executing":
            return "border-l-blue-600";
        default:
            return "border-l-gray-600";
    }
};

const getStatusVariant = (status: string): "default" | "secondary" | "destructive" | "outline" => {
    switch (status.toLowerCase()) {
        case "completed":
            return "default";
        case "failed":
        case "error":
            return "destructive";
        case "running":
        case "executing":
            return "secondary";
        default:
            return "outline";
    }
};

const formatDate = (dateStr: string) => {
    if (!dateStr) return "Unknown";
    try {
        const date = new Date(dateStr);
        const today = new Date();
        const yesterday = new Date(today);
        yesterday.setDate(yesterday.getDate() - 1);

        if (date.toDateString() === today.toDateString()) {
            return "Today";
        } else if (date.toDateString() === yesterday.toDateString()) {
            return "Yesterday";
        } else {
            return date.toLocaleDateString(undefined, {
                weekday: "long",
                year: "numeric",
                month: "long",
                day: "numeric",
            });
        }
    } catch {
        return dateStr;
    }
};

const formatTime = (dateStr: string) => {
    if (!dateStr) return "N/A";
    try {
        return new Date(dateStr).toLocaleTimeString();
    } catch {
        return dateStr;
    }
};

const formatDuration = (startStr: string, endStr?: string) => {
    if (!startStr) return "N/A";
    if (!endStr) return "In progress...";

    try {
        const start = new Date(startStr);
        const end = new Date(endStr);
        const diffMs = end.getTime() - start.getTime();
        return formatDurationMs(diffMs);
    } catch {
        return "N/A";
    }
};

const formatDurationMs = (durationMs: number) => {
    if (durationMs < 1000) return `${durationMs.toFixed(3)}ms`;
    if (durationMs < 60000) return `${(durationMs / 1000).toFixed(1)}s`;
    if (durationMs < 3600000) return `${(durationMs / 60000).toFixed(1)}m`;
    return `${(durationMs / 3600000).toFixed(1)}h`;
};

const getTriggerDescription = (triggerData: any) => {
    if (!triggerData) return "Unknown";

    if (triggerData.trigger) {
        return triggerData.trigger;
    }

    if (triggerData.event_type) {
        return triggerData.event_type;
    }

    return "Manual";
};

const toggleExpand = (id: string) => {
    if (expandedIds.value.has(id)) {
        expandedIds.value.delete(id);
    } else {
        expandedIds.value.add(id);
    }
};

const loadStats = async () => {
    try {
        const runtimeConfig = useRuntimeConfig();
        const cookieToken = useCookie(
            runtimeConfig.public.sessionCookieName as string
        );
        const token = cookieToken.value;

        const response = await $fetch<ApiResponse<{ stats: WorkflowStats }>>(
            `/api/servers/${props.serverId}/workflows/${props.workflowId}/executions/stats`,
            {
                headers: {
                    Authorization: `Bearer ${token}`,
                },
            }
        );

        if (response.code === 200) {
            stats.value = response.data.stats;
        }
    } catch (err: any) {
        console.error("Failed to load stats:", err);
    }
};

const loadExecutions = async (append = false) => {
    try {
        const runtimeConfig = useRuntimeConfig();
        const cookieToken = useCookie(
            runtimeConfig.public.sessionCookieName as string
        );
        const token = cookieToken.value;

        loading.value = true;
        error.value = null;

        if (!append) {
            executions.value = [];
            offset.value = 0;
        }

        const response = await $fetch<
            ApiResponse<{
                executions: WorkflowExecution[];
                limit: number;
                offset: number;
            }>
        >(
            `/api/servers/${props.serverId}/workflows/${props.workflowId}/executions`,
            {
                query: {
                    limit: limit.value,
                    offset: offset.value,
                },
                headers: {
                    Authorization: `Bearer ${token}`,
                },
            }
        );

        if (response.code === 200) {
            const newExecutions = response.data.executions || [];

            if (append) {
                executions.value.push(...newExecutions);
            } else {
                executions.value = newExecutions;
            }

            hasMore.value = newExecutions.length === limit.value;
        } else {
            error.value = response.message || "Failed to load executions";
        }
    } catch (err: any) {
        error.value = err.message || "Failed to load executions";
    } finally {
        loading.value = false;
    }
};

const loadMore = () => {
    offset.value += limit.value;
    loadExecutions(true);
};

// Lifecycle
onMounted(() => {
    loadStats();
    loadExecutions();
});
</script>
