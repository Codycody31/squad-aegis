<script setup lang="ts">
import { Card, CardHeader, CardContent } from "@/components/ui/card";
import {
    Table,
    TableBody,
    TableCell,
    TableHead,
    TableHeader,
    TableRow,
} from "@/components/ui/table";
import type { Server } from "~/types";

useHead({
    title: "Dashboard",
});

definePageMeta({
    middleware: "auth",
});

const runtimeConfig = useRuntimeConfig();
type DashboardServerStatus = {
    gamePort: boolean;
    rcon: boolean;
};

const serverStatus = ref<Record<string, DashboardServerStatus>>({});
const servers = ref<Server[]>([]);
const loading = ref(true);
const stats = ref({
    totalServers: 0,
    totalPlayers: 0,
    totalBans: 0,
    activeServers: 0,
});

const statusList = computed(() => Object.values(serverStatus.value));
const allGamePortsOnline = computed(() =>
    statusList.value.every((status) => status.gamePort),
);
const allRconOnline = computed(() =>
    statusList.value.every((status) => status.rcon),
);
const systemHealthy = computed(
    () => allGamePortsOnline.value && allRconOnline.value,
);
const systemStatusMessage = computed(() => {
    if (!allGamePortsOnline.value) {
        return "Some game ports are offline";
    }
    if (!allRconOnline.value) {
        return "Some RCON connections are offline";
    }
    return "All systems operational";
});

// Fetch servers data
const fetchServers = async () => {
    loading.value = true;
    const { data, error } = await useAuthFetch(
        `${runtimeConfig.public.backendApi}/servers`
    );

    if (error.value) {
        console.error("Error fetching servers:", error.value);
    } else if (data.value?.data?.servers) {
        servers.value = data.value.data.servers;
        stats.value.totalServers = servers.value.length;

        // Fetch additional stats for each server
        fetchAllServerStats();
    }
    loading.value = false;
};

// Fetch stats for all servers
const fetchAllServerStats = async () => {
    let totalPlayers = 0;
    let totalBans = 0;

    // Create an array of promises for parallel fetching
    const promises = servers.value.map(async (server) => {
        // Fetch server metrics (including player count)
        const { data: metricsData } = await useAuthFetch(
            `${runtimeConfig.public.backendApi}/servers/${server.id}/metrics`
        );

        if (metricsData.value?.data?.metrics?.players?.total) {
            totalPlayers += metricsData.value.data.metrics.players.total;
        }

        // Fetch banned players count
        const { data: bansData } = await useAuthFetch(
            `${runtimeConfig.public.backendApi}/servers/${server.id}/bans`
        );

        if (bansData.value?.data?.bans) {
            totalBans += bansData.value.data.bans.length;
        }

        // Fetch server status
        const { data: statusData } = await useAuthFetch(
            `${runtimeConfig.public.backendApi}/servers/${server.id}/status`
        );

        if (statusData.value?.data?.status) {
            serverStatus.value[server.id] = statusData.value.data.status;
        }
    });

    // Wait for all promises to resolve
    await Promise.all(promises);

    // Update stats
    stats.value.activeServers = servers.value.filter(
        (server) => serverStatus.value[server.id]?.gamePort,
    ).length;
    stats.value.totalPlayers = totalPlayers;
    stats.value.totalBans = totalBans;
};

fetchServers();
</script>

<template>
    <div class="p-3 sm:p-4">
        <h1 class="text-xl sm:text-2xl font-bold mb-4 sm:mb-6">Dashboard</h1>

        <!-- Stats Overview -->
        <div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-3 sm:gap-4 mb-4 sm:mb-6">
            <!-- Total Servers Card -->
            <Card>
                <CardContent class="p-4 sm:p-6">
                    <div class="flex items-center justify-between">
                        <div>
                            <p
                                class="text-xs sm:text-sm font-medium text-muted-foreground"
                            >
                                Total Servers
                            </p>
                            <h3 class="text-2xl sm:text-3xl font-bold mt-1">
                                {{ stats.totalServers }}
                            </h3>
                        </div>
                        <div
                            class="h-10 w-10 sm:h-12 sm:w-12 rounded-full bg-primary/10 flex items-center justify-center"
                        >
                            <svg
                                xmlns="http://www.w3.org/2000/svg"
                                class="h-5 w-5 sm:h-6 sm:w-6 text-primary"
                                fill="none"
                                viewBox="0 0 24 24"
                                stroke="currentColor"
                            >
                                <path
                                    stroke-linecap="round"
                                    stroke-linejoin="round"
                                    stroke-width="2"
                                    d="M5 12h14M5 12a2 2 0 01-2-2V6a2 2 0 012-2h14a2 2 0 012 2v4a2 2 0 01-2 2M5 12a2 2 0 00-2 2v4a2 2 0 002 2h14a2 2 0 002-2v-4a2 2 0 00-2-2H5z"
                                />
                            </svg>
                        </div>
                    </div>
                    <div class="mt-3 sm:mt-4">
                        <p class="text-xs sm:text-sm text-muted-foreground">
                            <span class="text-green-500">{{
                                stats.activeServers
                            }}</span>
                            servers online
                        </p>
                    </div>
                </CardContent>
            </Card>

            <!-- Total Players Card -->
            <Card>
                <CardContent class="p-4 sm:p-6">
                    <div class="flex items-center justify-between">
                        <div>
                            <p
                                class="text-xs sm:text-sm font-medium text-muted-foreground"
                            >
                                Total Players
                            </p>
                            <h3 class="text-2xl sm:text-3xl font-bold mt-1">
                                {{ stats.totalPlayers }}
                            </h3>
                        </div>
                        <div
                            class="h-10 w-10 sm:h-12 sm:w-12 rounded-full bg-blue-500/10 flex items-center justify-center"
                        >
                            <svg
                                xmlns="http://www.w3.org/2000/svg"
                                class="h-5 w-5 sm:h-6 sm:w-6 text-blue-500"
                                fill="none"
                                viewBox="0 0 24 24"
                                stroke="currentColor"
                            >
                                <path
                                    stroke-linecap="round"
                                    stroke-linejoin="round"
                                    stroke-width="2"
                                    d="M12 4.354a4 4 0 110 5.292M15 21H3v-1a6 6 0 0112 0v1zm0 0h6v-1a6 6 0 00-9-5.197M13 7a4 4 0 11-8 0 4 4 0 018 0z"
                                />
                            </svg>
                        </div>
                    </div>
                    <div class="mt-3 sm:mt-4">
                        <p class="text-xs sm:text-sm text-muted-foreground">
                            Currently connected across all servers
                        </p>
                    </div>
                </CardContent>
            </Card>

            <!-- Total Bans Card -->
            <Card>
                <CardContent class="p-4 sm:p-6">
                    <div class="flex items-center justify-between">
                        <div>
                            <p
                                class="text-xs sm:text-sm font-medium text-muted-foreground"
                            >
                                Total Bans
                            </p>
                            <h3 class="text-2xl sm:text-3xl font-bold mt-1">
                                {{ stats.totalBans }}
                            </h3>
                        </div>
                        <div
                            class="h-10 w-10 sm:h-12 sm:w-12 rounded-full bg-red-500/10 flex items-center justify-center"
                        >
                            <svg
                                xmlns="http://www.w3.org/2000/svg"
                                class="h-5 w-5 sm:h-6 sm:w-6 text-red-500"
                                fill="none"
                                viewBox="0 0 24 24"
                                stroke="currentColor"
                            >
                                <path
                                    stroke-linecap="round"
                                    stroke-linejoin="round"
                                    stroke-width="2"
                                    d="M18.364 18.364A9 9 0 005.636 5.636m12.728 12.728A9 9 0 015.636 5.636m12.728 12.728L5.636 5.636"
                                />
                            </svg>
                        </div>
                    </div>
                    <div class="mt-3 sm:mt-4">
                        <p class="text-xs sm:text-sm text-muted-foreground">
                            Banned players across all servers
                        </p>
                    </div>
                </CardContent>
            </Card>

            <!-- System Status Card -->
            <Card>
                <CardContent class="p-4 sm:p-6">
                    <div class="flex items-center justify-between">
                        <div>
                            <p
                                class="text-xs sm:text-sm font-medium text-muted-foreground"
                            >
                                System Status
                            </p>
                            <h3 class="text-2xl sm:text-3xl font-bold mt-1 text-green-500">
                                {{
                                    systemHealthy ? "Healthy" : "Degraded"
                                }}
                            </h3>
                        </div>
                        <div
                            class="h-10 w-10 sm:h-12 sm:w-12 rounded-full bg-green-500/10 flex items-center justify-center"
                        >
                            <svg
                                xmlns="http://www.w3.org/2000/svg"
                                class="h-5 w-5 sm:h-6 sm:w-6 text-green-500"
                                fill="none"
                                viewBox="0 0 24 24"
                                stroke="currentColor"
                                v-if="systemHealthy"
                            >
                                <path
                                    stroke-linecap="round"
                                    stroke-linejoin="round"
                                    stroke-width="2"
                                    d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z"
                                />
                            </svg>
                            <svg
                                xmlns="http://www.w3.org/2000/svg"
                                class="h-5 w-5 sm:h-6 sm:w-6 text-red-500"
                                fill="none"
                                viewBox="0 0 24 24"
                                stroke="currentColor"
                                v-else
                            >
                                <path
                                    stroke-linecap="round"
                                    stroke-linejoin="round"
                                    stroke-width="2"
                                    d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z"
                                />
                            </svg>
                        </div>
                    </div>
                    <div class="mt-3 sm:mt-4">
                        <p class="text-xs sm:text-sm text-muted-foreground">
                            {{ systemStatusMessage }}
                        </p>
                    </div>
                </CardContent>
            </Card>
        </div>

        <!-- Servers Table -->
        <Card>
            <CardHeader class="pb-2 sm:pb-3">
                <h2 class="text-base sm:text-lg font-semibold">Servers</h2>
            </CardHeader>
            <CardContent>
                <div v-if="loading" class="py-6 sm:py-8 text-center">
                    <div
                        class="animate-spin h-8 w-8 border-4 border-primary border-t-transparent rounded-full mx-auto mb-4"
                    ></div>
                    <p class="text-sm sm:text-base">Loading server information...</p>
                </div>
                <div v-else-if="servers.length === 0" class="py-6 sm:py-8 text-center">
                    <p class="text-sm sm:text-base text-muted-foreground">No servers available</p>
                </div>
                <template v-else>
                    <!-- Desktop Table View -->
                    <div class="hidden md:block w-full overflow-x-auto">
                        <Table>
                            <TableHeader>
                                <TableRow>
                                    <TableHead class="text-xs sm:text-sm">Server Name</TableHead>
                                    <TableHead class="text-xs sm:text-sm">IP Address</TableHead>
                                    <TableHead class="text-xs sm:text-sm">Game Port</TableHead>
                                    <TableHead class="text-xs sm:text-sm">RCON IP Address</TableHead>
                                    <TableHead class="text-xs sm:text-sm">RCON Port</TableHead>
                                    <TableHead class="text-xs sm:text-sm">Status</TableHead>
                                    <TableHead class="text-right text-xs sm:text-sm">Actions</TableHead>
                                </TableRow>
                            </TableHeader>
                            <TableBody>
                                <TableRow v-for="server in servers" :key="server.id" class="hover:bg-muted/50">
                                    <TableCell class="font-medium text-sm sm:text-base">{{
                                        server.name
                                    }}</TableCell>
                                    <TableCell class="text-xs sm:text-sm">{{ server.ip_address }}</TableCell>
                                    <TableCell class="text-xs sm:text-sm">{{ server.game_port }}</TableCell>
                                    <TableCell class="text-xs sm:text-sm">{{ server.rcon_ip_address || "Unknown" }}</TableCell>
                                    <TableCell class="text-xs sm:text-sm">{{ server.rcon_port }}</TableCell>
                                    <TableCell class="align-middle">
                                        <div class="flex items-center gap-1.5 flex-wrap">
                                            <span
                                                class="px-2 py-1 rounded-full text-xs font-medium"
                                                :class="
                                                    serverStatus[server.id]?.gamePort
                                                        ? 'bg-green-100 text-green-800'
                                                        : 'bg-red-100 text-red-800'
                                                "
                                            >
                                                {{
                                                    serverStatus[server.id]?.gamePort
                                                        ? "Game Online"
                                                        : "Game Offline"
                                                }}
                                            </span>
                                            <span
                                                class="px-2 py-1 rounded-full text-xs font-medium"
                                                :class="
                                                    serverStatus[server.id]?.rcon
                                                        ? 'bg-green-100 text-green-800'
                                                        : 'bg-red-100 text-red-800'
                                                "
                                            >
                                                {{
                                                    serverStatus[server.id]?.rcon
                                                        ? "RCON Online"
                                                        : "RCON Offline"
                                                }}
                                            </span>
                                        </div>
                                    </TableCell>
                                    <TableCell class="text-right align-middle">
                                        <div class="flex items-center justify-end">
                                            <NuxtLink :to="`/servers/${server.id}`" class="inline-flex">
                                                <button
                                                    class="inline-flex items-center px-3 py-1 bg-primary text-primary-foreground rounded-md text-xs sm:text-sm"
                                                >
                                                    Manage
                                                </button>
                                            </NuxtLink>
                                        </div>
                                    </TableCell>
                                </TableRow>
                            </TableBody>
                        </Table>
                    </div>

                    <!-- Mobile Card View -->
                    <div class="md:hidden space-y-3">
                        <div
                            v-for="server in servers"
                            :key="server.id"
                            class="border rounded-lg p-3 sm:p-4 hover:bg-muted/30 transition-colors"
                        >
                            <div class="flex items-start justify-between gap-2 mb-2">
                                <div class="flex-1 min-w-0">
                                    <div class="font-semibold text-sm sm:text-base mb-1">
                                        {{ server.name }}
                                    </div>
                                    <div class="space-y-1.5">
                                        <div>
                                            <span class="text-xs text-muted-foreground">IP: </span>
                                            <span class="text-xs sm:text-sm">{{ server.ip_address }}</span>
                                        </div>
                                        <div>
                                            <span class="text-xs text-muted-foreground">Game Port: </span>
                                            <span class="text-xs sm:text-sm">{{ server.game_port }}</span>
                                        </div>
                                        <div>
                                            <span class="text-xs text-muted-foreground">RCON IP: </span>
                                            <span class="text-xs sm:text-sm">{{ server.rcon_ip_address || "Unknown" }}</span>
                                        </div>
                                        <div>
                                            <span class="text-xs text-muted-foreground">RCON Port: </span>
                                            <span class="text-xs sm:text-sm">{{ server.rcon_port }}</span>
                                        </div>
                                        <div class="flex items-center gap-2 mt-2">
                                            <span
                                                class="px-2 py-1 rounded-full text-xs font-medium"
                                                :class="
                                                    serverStatus[server.id]?.gamePort
                                                        ? 'bg-green-100 text-green-800'
                                                        : 'bg-red-100 text-red-800'
                                                "
                                            >
                                                {{
                                                    serverStatus[server.id]?.gamePort
                                                        ? "Game Online"
                                                        : "Game Offline"
                                                }}
                                            </span>
                                            <span
                                                class="px-2 py-1 rounded-full text-xs font-medium"
                                                :class="
                                                    serverStatus[server.id]?.rcon
                                                        ? 'bg-green-100 text-green-800'
                                                        : 'bg-red-100 text-red-800'
                                                "
                                            >
                                                {{
                                                    serverStatus[server.id]?.rcon
                                                        ? "RCON Online"
                                                        : "RCON Offline"
                                                }}
                                            </span>
                                        </div>
                                    </div>
                                </div>
                            </div>
                            <div class="flex items-center justify-end gap-2 pt-2 border-t">
                                <NuxtLink :to="`/servers/${server.id}`" class="w-full">
                                    <button
                                        class="w-full px-3 py-1.5 bg-primary text-primary-foreground rounded-md text-xs sm:text-sm"
                                    >
                                        Manage
                                    </button>
                                </NuxtLink>
                            </div>
                        </div>
                    </div>
                </template>
            </CardContent>
        </Card>
    </div>
</template>
