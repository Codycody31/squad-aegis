<script setup lang="ts">
import { ref, onMounted, computed } from "vue";
import { useRoute, useRouter } from "vue-router";
import { Button } from "~/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "~/components/ui/card";
import {
    Table,
    TableBody,
    TableCell,
    TableHead,
    TableHeader,
    TableRow,
} from "~/components/ui/table";
import { Badge } from "~/components/ui/badge";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "~/components/ui/tabs";
import { useAuthStore } from "@/stores/auth";
import { ExternalLink } from "lucide-vue-next";

const authStore = useAuthStore();
const runtimeConfig = useRuntimeConfig();
const route = useRoute();
const router = useRouter();

const loading = ref(true);
const error = ref<string | null>(null);
const player = ref<PlayerProfile | null>(null);
const cblData = ref<CBLUser | null>(null);
const cblLoading = ref(false);

interface PlayerProfile {
    steam_id: string;
    eos_id: string;
    player_name: string;
    last_seen: string | null;
    first_seen: string | null;
    total_play_time: number;
    total_sessions: number;
    statistics: PlayerStatistics;
    recent_activity: PlayerActivity[];
    chat_history: ChatMessage[];
    violations: RuleViolation[];
    recent_servers: RecentServerInfo[];
}

interface PlayerStatistics {
    kills: number;
    deaths: number;
    teamkills: number;
    revives: number;
    times_revived: number;
    damage_dealt: number;
    damage_taken: number;
    kd_ratio: number;
}

interface PlayerActivity {
    event_time: string;
    event_type: string;
    description: string;
    server_id: string;
    server_name?: string;
}

interface ChatMessage {
    sent_at: string;
    message: string;
    chat_type: string;
    server_id: string;
    server_name?: string;
}

interface RuleViolation {
    violation_id: string;
    server_id: string;
    server_name?: string;
    rule_id: string | null;
    rule_name?: string | null;
    action_type: string;
    admin_user_id: string | null;
    admin_name?: string | null;
    created_at: string;
}

interface RecentServerInfo {
    server_id: string;
    server_name: string;
    last_seen: string;
    sessions: number;
}

interface PlayerResponse {
    data: {
        player: PlayerProfile;
    };
}

interface CBLUser {
    id: string;
    name: string;
    avatarFull: string;
    reputationPoints: number;
    riskRating: number;
    reputationRank: number;
    lastRefreshedInfo: string;
    lastRefreshedReputationPoints: string;
    lastRefreshedReputationRank: string;
    reputationPointsMonthChange: number;
}

interface CBLGraphQLResponse {
    data: {
        steamUser: CBLUser | null;
    };
    errors?: any[];
}

// Function to fetch player profile
async function fetchPlayerProfile() {
    loading.value = true;
    error.value = null;

    const playerId = route.params.playerId as string;

    const cookieToken = useCookie(
        runtimeConfig.public.sessionCookieName as string,
    );
    const token = cookieToken.value;

    if (!token) {
        error.value = "Authentication required";
        return;
    }

    try {
        const response = await fetch(
            `${runtimeConfig.public.backendApi}/players/${playerId}`,
            {
                method: "GET",
                headers: {
                    "Content-Type": "application/json",
                    Authorization: `Bearer ${token}`,
                },
                credentials: "include",
            },
        );

        if (!response.ok) {
            const errorData = await response.json();
            throw new Error(
                errorData.message || "Failed to fetch player profile",
            );
        }

        const data: PlayerResponse = await response.json();
        player.value = data.data.player;
    } catch (err: any) {
        error.value =
            err.message || "An error occurred while fetching player profile";
    } finally {
        loading.value = false;
    }
}

// Function to fetch CBL reputation data
async function fetchCBLData(steamId: string) {
    cblLoading.value = true;

    const query = `
        query Search($id: String!) {
            steamUser(id: $id) {
                id
                name
                avatarFull
                reputationPoints
                riskRating
                reputationRank
                lastRefreshedInfo
                lastRefreshedReputationPoints
                lastRefreshedReputationRank
                reputationPointsMonthChange
            }
        }
    `;

    try {
        const response = await fetch("https://communitybanlist.com/graphql", {
            method: "POST",
            headers: {
                "Content-Type": "application/json",
            },
            body: JSON.stringify({
                query,
                variables: {
                    id: steamId,
                },
            }),
        });

        if (!response.ok) {
            throw new Error("Failed to fetch CBL data");
        }

        const result: CBLGraphQLResponse = await response.json();

        if (result.errors && result.errors.length > 0) {
            throw new Error("GraphQL errors occurred");
        }

        cblData.value = result.data.steamUser;
    } catch (err: any) {
        console.error("Failed to fetch CBL data:", err);
        cblData.value = null;
    } finally {
        cblLoading.value = false;
    }
}

// Helper function to get risk rating color and label
function getRiskRatingInfo(riskRating: number) {
    if (riskRating >= 8) {
        return {
            variant: "destructive" as const,
            label: "Very High Risk",
            colorClass: "text-destructive",
        };
    } else if (riskRating >= 6) {
        return {
            variant: "destructive" as const,
            label: "High Risk",
            colorClass: "text-orange-500",
        };
    } else if (riskRating >= 4) {
        return {
            variant: "secondary" as const,
            label: "Medium Risk",
            colorClass: "text-yellow-600",
        };
    } else if (riskRating >= 2) {
        return {
            variant: "secondary" as const,
            label: "Low Risk",
            colorClass: "text-blue-500",
        };
    } else {
        return {
            variant: "default" as const,
            label: "Very Low Risk",
            colorClass: "text-green-500",
        };
    }
}

// Function to format date
function formatDate(dateString: string | null): string {
    if (!dateString) return "N/A";
    const date = new Date(dateString);
    return date.toLocaleString();
}

// Function to get time ago
function getTimeAgo(dateString: string | null): string {
    if (!dateString) return "N/A";
    const date = new Date(dateString);
    const now = new Date();
    const diff = now.getTime() - date.getTime();

    const seconds = Math.floor(diff / 1000);
    const minutes = Math.floor(seconds / 60);
    const hours = Math.floor(minutes / 60);
    const days = Math.floor(hours / 24);
    const months = Math.floor(days / 30);
    const years = Math.floor(days / 365);

    if (years > 0) return `${years} year${years > 1 ? "s" : ""} ago`;
    if (months > 0) return `${months} month${months > 1 ? "s" : ""} ago`;
    if (days > 0) return `${days} day${days > 1 ? "s" : ""} ago`;
    if (hours > 0) return `${hours} hour${hours > 1 ? "s" : ""} ago`;
    if (minutes > 0) return `${minutes} minute${minutes > 1 ? "s" : ""} ago`;
    return "Just now";
}

// Function to format play time
function formatPlayTime(seconds: number): string {
    if (seconds === 0) return "N/A";

    const days = Math.floor(seconds / 86400);
    const hours = Math.floor((seconds % 86400) / 3600);
    const minutes = Math.floor((seconds % 3600) / 60);

    const parts = [];
    if (days > 0) parts.push(`${days}d`);
    if (hours > 0) parts.push(`${hours}h`);
    if (minutes > 0) parts.push(`${minutes}m`);

    return parts.join(" ") || "< 1m";
}

// Function to get badge variant for violation type
function getViolationBadgeVariant(
    actionType: string,
): "default" | "destructive" | "outline" | "secondary" {
    switch (actionType.toUpperCase()) {
        case "BAN":
            return "destructive";
        case "KICK":
            return "secondary";
        case "WARN":
            return "outline";
        default:
            return "default";
    }
}

// Function to get badge variant for event type
function getEventTypeBadgeVariant(
    eventType: string,
): "default" | "destructive" | "outline" | "secondary" {
    switch (eventType.toLowerCase()) {
        case "death":
            return "destructive";
        case "chat":
            return "outline";
        case "connection":
            return "secondary";
        default:
            return "default";
    }
}

// Function to get chat type color
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

onMounted(async () => {
    if (!authStore.isLoggedIn) {
        navigateTo("/login");
        return;
    }
    await fetchPlayerProfile();

    // Fetch CBL data if steam_id is available
    if (player.value && player.value.steam_id) {
        fetchCBLData(player.value.steam_id);
    }
});
</script>

<template>
    <div class="container mx-auto p-6">
        <div class="flex items-center gap-4 mb-6">
            <Button variant="outline" @click="router.push('/players')">
                ← Back to Search
            </Button>
            <h1 class="text-3xl font-bold">Player Profile</h1>
        </div>

        <div v-if="loading" class="flex justify-center items-center py-12">
            <div class="text-center">
                <div
                    class="animate-spin rounded-full h-12 w-12 border-b-2 border-primary mx-auto mb-4"
                ></div>
                <p class="text-muted-foreground">Loading player profile...</p>
            </div>
        </div>

        <div
            v-else-if="error"
            class="p-4 bg-destructive/15 text-destructive rounded-md"
        >
            {{ error }}
        </div>

        <div v-else-if="player">
            <!-- Player Header -->
            <Card class="mb-6">
                <CardHeader>
                    <CardTitle class="text-2xl">{{
                        player.player_name
                    }}</CardTitle>
                </CardHeader>
                <CardContent>
                    <div
                        class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4 mb-4"
                    >
                        <div>
                            <div class="text-sm text-muted-foreground">
                                Steam ID
                            </div>
                            <code class="text-sm bg-muted px-2 py-1 rounded">
                                {{ player.steam_id || "N/A" }}
                            </code>
                        </div>
                        <div>
                            <div class="text-sm text-muted-foreground">
                                EOS ID
                            </div>
                            <code class="text-sm bg-muted px-2 py-1 rounded">
                                {{ player.eos_id || "N/A" }}
                            </code>
                        </div>
                        <div>
                            <div class="text-sm text-muted-foreground">
                                Last Seen
                            </div>
                            <div class="text-sm font-medium">
                                {{ getTimeAgo(player.last_seen) }}
                            </div>
                            <div class="text-xs text-muted-foreground">
                                {{ formatDate(player.last_seen) }}
                            </div>
                        </div>
                        <div>
                            <div class="text-sm text-muted-foreground">
                                First Seen
                            </div>
                            <div class="text-sm font-medium">
                                {{ formatDate(player.first_seen) }}
                            </div>
                        </div>
                    </div>

                    <!-- External Links -->
                    <div class="pt-4 border-t">
                        <div class="text-sm text-muted-foreground mb-3">
                            External Links
                        </div>
                        <div class="flex flex-wrap gap-2">
                            <a
                                v-if="player.steam_id"
                                :href="`https://steamcommunity.com/profiles/${player.steam_id}`"
                                target="_blank"
                                rel="noopener noreferrer"
                                class="inline-flex items-center gap-1.5 px-3 py-1.5 text-xs bg-[#171a21] hover:bg-[#1b2838] text-white rounded-md transition-colors"
                            >
                                <ExternalLink :size="14" />
                                <span>Steam</span>
                            </a>
                            <a
                                v-if="player.steam_id"
                                :href="`https://www.battlemetrics.com/players?filter[search]=${player.steam_id}`"
                                target="_blank"
                                rel="noopener noreferrer"
                                class="inline-flex items-center gap-1.5 px-3 py-1.5 text-xs bg-[#f26a21] hover:bg-[#d85a15] text-white rounded-md transition-colors"
                            >
                                <ExternalLink :size="14" />
                                <span>Battlemetrics</span>
                            </a>
                        </div>
                    </div>
                </CardContent>
            </Card>

            <!-- CBL Reputation Card -->
            <Card v-if="player.steam_id" class="mb-6">
                <CardHeader>
                    <div class="flex items-center justify-between">
                        <CardTitle class="text-lg"
                            >Community Ban List Reputation</CardTitle
                        >
                        <a
                            :href="`https://communitybanlist.com/search/${player.steam_id}`"
                            target="_blank"
                            rel="noopener noreferrer"
                            class="text-sm text-muted-foreground hover:text-foreground inline-flex items-center gap-1"
                        >
                            <span>View Full Profile</span>
                            <ExternalLink :size="14" />
                        </a>
                    </div>
                </CardHeader>
                <CardContent>
                    <div v-if="cblLoading" class="flex justify-center py-4">
                        <div
                            class="animate-spin rounded-full h-8 w-8 border-b-2 border-primary"
                        ></div>
                    </div>
                    <div
                        v-else-if="cblData"
                        class="grid grid-cols-1 md:grid-cols-3 gap-4"
                    >
                        <div class="text-center p-4 bg-muted/50 rounded-lg">
                            <div class="text-sm text-muted-foreground mb-1">
                                Reputation Points
                            </div>
                            <div
                                class="text-3xl font-bold"
                                :class="{
                                    'text-destructive':
                                        cblData.reputationPoints >= 6,
                                    'text-orange-500':
                                        cblData.reputationPoints >= 3 &&
                                        cblData.reputationPoints < 6,
                                    'text-green-500':
                                        cblData.reputationPoints < 3,
                                }"
                            >
                                {{ cblData.reputationPoints }}
                            </div>
                            <div
                                v-if="cblData.reputationPointsMonthChange !== 0"
                                class="text-xs mt-1"
                                :class="{
                                    'text-destructive':
                                        cblData.reputationPointsMonthChange > 0,
                                    'text-green-500':
                                        cblData.reputationPointsMonthChange < 0,
                                }"
                            >
                                {{
                                    cblData.reputationPointsMonthChange > 0
                                        ? "+"
                                        : ""
                                }}{{ cblData.reputationPointsMonthChange }} this
                                month
                            </div>
                        </div>
                        <div class="text-center p-4 bg-muted/50 rounded-lg">
                            <div class="text-sm text-muted-foreground mb-1">
                                Risk Rating
                            </div>
                            <div
                                class="text-3xl font-bold"
                                :class="
                                    getRiskRatingInfo(cblData.riskRating)
                                        .colorClass
                                "
                            >
                                {{ cblData.riskRating }}/10
                            </div>
                            <Badge
                                :variant="
                                    getRiskRatingInfo(cblData.riskRating)
                                        .variant
                                "
                                class="mt-2"
                            >
                                {{
                                    getRiskRatingInfo(cblData.riskRating).label
                                }}
                            </Badge>
                        </div>
                        <div class="text-center p-4 bg-muted/50 rounded-lg">
                            <div class="text-sm text-muted-foreground mb-1">
                                Reputation Rank
                            </div>
                            <div class="text-3xl font-bold">
                                #{{ cblData.reputationRank.toLocaleString() }}
                            </div>
                            <div class="text-xs text-muted-foreground mt-1">
                                Global ranking
                            </div>
                        </div>
                    </div>
                    <div v-else class="text-center py-4 text-muted-foreground">
                        <p>No Community Ban List data available</p>
                        <p class="text-xs mt-1">
                            Player may not be in the CBL database
                        </p>
                    </div>
                </CardContent>
            </Card>

            <!-- Statistics Overview -->
            <div
                class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4 mb-6"
            >
                <Card>
                    <CardHeader class="pb-2">
                        <CardTitle
                            class="text-sm font-medium text-muted-foreground"
                            >K/D Ratio</CardTitle
                        >
                    </CardHeader>
                    <CardContent>
                        <div class="text-2xl font-bold">
                            {{ player.statistics.kd_ratio.toFixed(2) }}
                        </div>
                        <div class="text-xs text-muted-foreground">
                            {{ player.statistics.kills }} kills /
                            {{ player.statistics.deaths }} deaths
                        </div>
                    </CardContent>
                </Card>

                <Card>
                    <CardHeader class="pb-2">
                        <CardTitle
                            class="text-sm font-medium text-muted-foreground"
                            >Teamkills</CardTitle
                        >
                    </CardHeader>
                    <CardContent>
                        <div class="text-2xl font-bold text-destructive">
                            {{ player.statistics.teamkills }}
                        </div>
                    </CardContent>
                </Card>

                <Card>
                    <CardHeader class="pb-2">
                        <CardTitle
                            class="text-sm font-medium text-muted-foreground"
                            >Revives</CardTitle
                        >
                    </CardHeader>
                    <CardContent>
                        <div class="text-2xl font-bold text-green-500">
                            {{ player.statistics.revives }}
                        </div>
                        <div class="text-xs text-muted-foreground">
                            Revived {{ player.statistics.times_revived }} times
                        </div>
                    </CardContent>
                </Card>

                <Card>
                    <CardHeader class="pb-2">
                        <CardTitle
                            class="text-sm font-medium text-muted-foreground"
                            >Total Sessions</CardTitle
                        >
                    </CardHeader>
                    <CardContent>
                        <div class="text-2xl font-bold">
                            {{ player.total_sessions }}
                        </div>
                        <div class="text-xs text-muted-foreground">
                            ~{{ formatPlayTime(player.total_play_time) }}
                            playtime
                        </div>
                    </CardContent>
                </Card>
            </div>

            <!-- Detailed Information Tabs -->
            <Tabs default-value="activity" class="space-y-4">
                <TabsList>
                    <TabsTrigger value="activity">Recent Activity</TabsTrigger>
                    <TabsTrigger value="chat">Chat History</TabsTrigger>
                    <TabsTrigger value="violations">Violations</TabsTrigger>
                    <TabsTrigger value="servers">Recent Servers</TabsTrigger>
                    <TabsTrigger value="statistics">Statistics</TabsTrigger>
                </TabsList>

                <!-- Recent Activity Tab -->
                <TabsContent value="activity">
                    <Card>
                        <CardHeader>
                            <CardTitle>Recent Activity</CardTitle>
                        </CardHeader>
                        <CardContent>
                            <div
                                v-if="player.recent_activity?.length > 0"
                                class="space-y-2"
                            >
                                <div
                                    v-for="(
                                        activity, index
                                    ) in player.recent_activity"
                                    :key="index"
                                    class="flex items-start gap-3 p-3 rounded-md hover:bg-muted/50 transition-colors"
                                >
                                    <Badge
                                        :variant="
                                            getEventTypeBadgeVariant(
                                                activity.event_type,
                                            )
                                        "
                                    >
                                        {{ activity.event_type }}
                                    </Badge>
                                    <div class="flex-1">
                                        <div class="text-sm">
                                            {{ activity.description }}
                                        </div>
                                        <div
                                            class="text-xs text-muted-foreground mt-1"
                                        >
                                            {{
                                                formatDate(activity.event_time)
                                            }}
                                            ({{
                                                getTimeAgo(activity.event_time)
                                            }})
                                        </div>
                                    </div>
                                </div>
                            </div>
                            <div
                                v-else
                                class="text-center py-8 text-muted-foreground"
                            >
                                No recent activity
                            </div>
                        </CardContent>
                    </Card>
                </TabsContent>

                <!-- Chat History Tab -->
                <TabsContent value="chat">
                    <Card>
                        <CardHeader>
                            <CardTitle
                                >Chat History (Last 50 messages)</CardTitle
                            >
                        </CardHeader>
                        <CardContent>
                            <div
                                v-if="player.chat_history?.length > 0"
                                class="space-y-2"
                            >
                                <div
                                    v-for="(chat, index) in player.chat_history"
                                    :key="index"
                                    class="p-3 rounded-md hover:bg-muted/50 transition-colors"
                                >
                                    <div class="flex items-center gap-2 mb-1">
                                        <Badge
                                            variant="outline"
                                            :class="
                                                getChatTypeColor(chat.chat_type)
                                            "
                                        >
                                            {{ chat.chat_type }}
                                        </Badge>
                                        <span
                                            class="text-xs text-muted-foreground"
                                        >
                                            {{ formatDate(chat.sent_at) }}
                                        </span>
                                    </div>
                                    <div class="text-sm">
                                        {{ chat.message }}
                                    </div>
                                </div>
                            </div>
                            <div
                                v-else
                                class="text-center py-8 text-muted-foreground"
                            >
                                No chat history
                            </div>
                        </CardContent>
                    </Card>
                </TabsContent>

                <!-- Violations Tab -->
                <TabsContent value="violations">
                    <Card>
                        <CardHeader>
                            <CardTitle>Rule Violations</CardTitle>
                        </CardHeader>
                        <CardContent>
                            <div v-if="player.violations?.length > 0">
                                <Table>
                                    <TableHeader>
                                        <TableRow>
                                            <TableHead>Date</TableHead>
                                            <TableHead>Action</TableHead>
                                            <TableHead>Rule</TableHead>
                                            <TableHead>Server</TableHead>
                                            <TableHead>Admin</TableHead>
                                        </TableRow>
                                    </TableHeader>
                                    <TableBody>
                                        <TableRow
                                            v-for="violation in player.violations"
                                            :key="violation.violation_id"
                                        >
                                            <TableCell>
                                                <div class="text-sm">
                                                    {{
                                                        formatDate(
                                                            violation.created_at,
                                                        )
                                                    }}
                                                </div>
                                                <div
                                                    class="text-xs text-muted-foreground"
                                                >
                                                    {{
                                                        getTimeAgo(
                                                            violation.created_at,
                                                        )
                                                    }}
                                                </div>
                                            </TableCell>
                                            <TableCell>
                                                <Badge
                                                    :variant="
                                                        getViolationBadgeVariant(
                                                            violation.action_type,
                                                        )
                                                    "
                                                >
                                                    {{ violation.action_type }}
                                                </Badge>
                                            </TableCell>
                                            <TableCell>
                                                <div
                                                    v-if="violation.rule_name"
                                                    class="text-sm"
                                                >
                                                    {{ violation.rule_name }}
                                                </div>
                                                <div
                                                    v-else
                                                    class="text-sm text-muted-foreground"
                                                >
                                                    No rule specified
                                                </div>
                                            </TableCell>
                                            <TableCell>
                                                <div class="text-sm">
                                                    {{
                                                        violation.server_name ||
                                                        "Unknown Server"
                                                    }}
                                                </div>
                                                <code
                                                    class="text-xs text-muted-foreground"
                                                    >{{
                                                        violation.server_id
                                                    }}</code
                                                >
                                            </TableCell>
                                            <TableCell>
                                                <div class="text-sm">
                                                    {{
                                                        violation.admin_name ||
                                                        "System"
                                                    }}
                                                </div>
                                                <div
                                                    v-if="
                                                        violation.admin_user_id
                                                    "
                                                    class="text-xs text-muted-foreground"
                                                >
                                                    ID:
                                                    {{
                                                        violation.admin_user_id
                                                    }}
                                                </div>
                                            </TableCell>
                                        </TableRow>
                                    </TableBody>
                                </Table>
                            </div>
                            <div
                                v-else
                                class="text-center py-8 text-muted-foreground"
                            >
                                No violations found
                            </div>
                        </CardContent>
                    </Card>
                </TabsContent>

                <!-- Recent Servers Tab -->
                <TabsContent value="servers">
                    <Card>
                        <CardHeader>
                            <CardTitle>Recent Servers</CardTitle>
                        </CardHeader>
                        <CardContent>
                            <div v-if="player.recent_servers?.length > 0">
                                <Table>
                                    <TableHeader>
                                        <TableRow>
                                            <TableHead>Server Name</TableHead>
                                            <TableHead>Last Seen</TableHead>
                                            <TableHead>Sessions</TableHead>
                                        </TableRow>
                                    </TableHeader>
                                    <TableBody>
                                        <TableRow
                                            v-for="server in player.recent_servers"
                                            :key="server.server_id"
                                        >
                                            <TableCell class="font-medium">{{
                                                server.server_name
                                            }}</TableCell>
                                            <TableCell>
                                                <div class="text-sm">
                                                    {{
                                                        getTimeAgo(
                                                            server.last_seen,
                                                        )
                                                    }}
                                                </div>
                                                <div
                                                    class="text-xs text-muted-foreground"
                                                >
                                                    {{
                                                        formatDate(
                                                            server.last_seen,
                                                        )
                                                    }}
                                                </div>
                                            </TableCell>
                                            <TableCell>{{
                                                server.sessions
                                            }}</TableCell>
                                        </TableRow>
                                    </TableBody>
                                </Table>
                            </div>
                            <div
                                v-else
                                class="text-center py-8 text-muted-foreground"
                            >
                                No server history
                            </div>
                        </CardContent>
                    </Card>
                </TabsContent>

                <!-- Statistics Tab -->
                <TabsContent value="statistics">
                    <Card>
                        <CardHeader>
                            <CardTitle>Combat Statistics</CardTitle>
                        </CardHeader>
                        <CardContent>
                            <div class="grid grid-cols-1 md:grid-cols-2 gap-6">
                                <div>
                                    <h3 class="text-lg font-semibold mb-4">
                                        Combat
                                    </h3>
                                    <div class="space-y-3">
                                        <div class="flex justify-between">
                                            <span class="text-muted-foreground"
                                                >Kills</span
                                            >
                                            <span class="font-medium">{{
                                                player.statistics.kills
                                            }}</span>
                                        </div>
                                        <div class="flex justify-between">
                                            <span class="text-muted-foreground"
                                                >Deaths</span
                                            >
                                            <span class="font-medium">{{
                                                player.statistics.deaths
                                            }}</span>
                                        </div>
                                        <div class="flex justify-between">
                                            <span class="text-muted-foreground"
                                                >K/D Ratio</span
                                            >
                                            <span class="font-medium">{{
                                                player.statistics.kd_ratio.toFixed(
                                                    2,
                                                )
                                            }}</span>
                                        </div>
                                        <div class="flex justify-between">
                                            <span class="text-destructive"
                                                >Teamkills</span
                                            >
                                            <span
                                                class="font-medium text-destructive"
                                                >{{
                                                    player.statistics.teamkills
                                                }}</span
                                            >
                                        </div>
                                    </div>
                                </div>

                                <div>
                                    <h3 class="text-lg font-semibold mb-4">
                                        Support
                                    </h3>
                                    <div class="space-y-3">
                                        <div class="flex justify-between">
                                            <span class="text-muted-foreground"
                                                >Revives Given</span
                                            >
                                            <span
                                                class="font-medium text-green-500"
                                                >{{
                                                    player.statistics.revives
                                                }}</span
                                            >
                                        </div>
                                        <div class="flex justify-between">
                                            <span class="text-muted-foreground"
                                                >Times Revived</span
                                            >
                                            <span class="font-medium">{{
                                                player.statistics.times_revived
                                            }}</span>
                                        </div>
                                    </div>
                                </div>

                                <div>
                                    <h3 class="text-lg font-semibold mb-4">
                                        Damage
                                    </h3>
                                    <div class="space-y-3">
                                        <div class="flex justify-between">
                                            <span class="text-muted-foreground"
                                                >Damage Dealt</span
                                            >
                                            <span class="font-medium">{{
                                                player.statistics.damage_dealt.toFixed(
                                                    0,
                                                )
                                            }}</span>
                                        </div>
                                        <div class="flex justify-between">
                                            <span class="text-muted-foreground"
                                                >Damage Taken</span
                                            >
                                            <span class="font-medium">{{
                                                player.statistics.damage_taken.toFixed(
                                                    0,
                                                )
                                            }}</span>
                                        </div>
                                    </div>
                                </div>

                                <div>
                                    <h3 class="text-lg font-semibold mb-4">
                                        Activity
                                    </h3>
                                    <div class="space-y-3">
                                        <div class="flex justify-between">
                                            <span class="text-muted-foreground"
                                                >Total Sessions</span
                                            >
                                            <span class="font-medium">{{
                                                player.total_sessions
                                            }}</span>
                                        </div>
                                        <div class="flex justify-between">
                                            <span class="text-muted-foreground"
                                                >Play Time</span
                                            >
                                            <span class="font-medium">{{
                                                formatPlayTime(
                                                    player.total_play_time,
                                                )
                                            }}</span>
                                        </div>
                                    </div>
                                </div>
                            </div>
                        </CardContent>
                    </Card>
                </TabsContent>
            </Tabs>
        </div>
    </div>
</template>
