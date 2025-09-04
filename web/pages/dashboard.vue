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
const serverStatus = ref<Record<string, { rcon: boolean }>>({});
const servers = ref<Server[]>([]);
const loading = ref(true);
const stats = ref({
    totalServers: 0,
    totalPlayers: 0,
    totalBans: 0,
    activeServers: 0,
});

// Fetch servers data
const fetchServers = async () => {
    loading.value = true;
    const { data, error } = await useFetch(
        `${runtimeConfig.public.backendApi}/servers`,
        {
            headers: {
                Authorization: `Bearer ${
                    useCookie(runtimeConfig.public.sessionCookieName).value
                }`,
            },
        },
    );

    if (error.value) {
        console.error("Error fetching servers:", error.value);
    } else if (data.value?.data?.servers) {
        servers.value = data.value.data.servers;
        stats.value.totalServers = servers.value.length;

        // Fetch additional stats for each server
        await fetchAllServerStats();
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
        const { data: metricsData } = await useFetch(
            `${runtimeConfig.public.backendApi}/servers/${server.id}/metrics`,
            {
                headers: {
                    Authorization: `Bearer ${
                        useCookie(runtimeConfig.public.sessionCookieName).value
                    }`,
                },
            },
        );

        if (metricsData.value?.data?.metrics?.players?.total) {
            totalPlayers += metricsData.value.data.metrics.players.total;
        }

        // Fetch banned players count
        const { data: bansData } = await useFetch(
            `${runtimeConfig.public.backendApi}/servers/${server.id}/bans`,
            {
                headers: {
                    Authorization: `Bearer ${
                        useCookie(runtimeConfig.public.sessionCookieName).value
                    }`,
                },
            },
        );

        if (bansData.value?.data?.bannedPlayers) {
            totalBans += bansData.value.data.bannedPlayers.length;
        }

        // Fetch server status
        const { data: statusData } = await useFetch(
            `${runtimeConfig.public.backendApi}/servers/${server.id}/status`,
            {
                headers: {
                    Authorization: `Bearer ${
                        useCookie(runtimeConfig.public.sessionCookieName).value
                    }`,
                },
            },
        );

        if (statusData.value?.data?.status) {
            serverStatus.value[server.id] = statusData.value.data.status;
        }
    });

    // Wait for all promises to resolve
    await Promise.all(promises);

    // Update stats
    stats.value.activeServers = servers.value.filter(
        (server) => serverStatus.value[server.id]?.rcon,
    ).length;
    stats.value.totalPlayers = totalPlayers;
    stats.value.totalBans = totalBans;
};

await fetchServers();
</script>

<template>
    <div class="p-4">
        <h1 class="text-2xl font-bold mb-6">Dashboard</h1>

        <!-- Stats Overview -->
        <div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4 mb-6">
            <!-- Total Servers Card -->
            <Card>
                <CardContent class="p-6">
                    <div class="flex items-center justify-between">
                        <div>
                            <p
                                class="text-sm font-medium text-muted-foreground"
                            >
                                Total Servers
                            </p>
                            <h3 class="text-3xl font-bold mt-1">
                                {{ stats.totalServers }}
                            </h3>
                        </div>
                        <div
                            class="h-12 w-12 rounded-full bg-primary/10 flex items-center justify-center"
                        >
                            <svg
                                xmlns="http://www.w3.org/2000/svg"
                                class="h-6 w-6 text-primary"
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
                    <div class="mt-4">
                        <p class="text-sm text-muted-foreground">
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
                <CardContent class="p-6">
                    <div class="flex items-center justify-between">
                        <div>
                            <p
                                class="text-sm font-medium text-muted-foreground"
                            >
                                Total Players
                            </p>
                            <h3 class="text-3xl font-bold mt-1">
                                {{ stats.totalPlayers }}
                            </h3>
                        </div>
                        <div
                            class="h-12 w-12 rounded-full bg-blue-500/10 flex items-center justify-center"
                        >
                            <svg
                                xmlns="http://www.w3.org/2000/svg"
                                class="h-6 w-6 text-blue-500"
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
                    <div class="mt-4">
                        <p class="text-sm text-muted-foreground">
                            Currently connected across all servers
                        </p>
                    </div>
                </CardContent>
            </Card>

            <!-- Total Bans Card -->
            <Card>
                <CardContent class="p-6">
                    <div class="flex items-center justify-between">
                        <div>
                            <p
                                class="text-sm font-medium text-muted-foreground"
                            >
                                Total Bans
                            </p>
                            <h3 class="text-3xl font-bold mt-1">
                                {{ stats.totalBans }}
                            </h3>
                        </div>
                        <div
                            class="h-12 w-12 rounded-full bg-red-500/10 flex items-center justify-center"
                        >
                            <svg
                                xmlns="http://www.w3.org/2000/svg"
                                class="h-6 w-6 text-red-500"
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
                    <div class="mt-4">
                        <p class="text-sm text-muted-foreground">
                            Banned players across all servers
                        </p>
                    </div>
                </CardContent>
            </Card>

            <!-- System Status Card -->
            <Card>
                <CardContent class="p-6">
                    <div class="flex items-center justify-between">
                        <div>
                            <p
                                class="text-sm font-medium text-muted-foreground"
                            >
                                System Status
                            </p>
                            <h3 class="text-3xl font-bold mt-1 text-green-500">
                                {{
                                    Object.values(serverStatus).every(
                                        (status) => status.rcon,
                                    )
                                        ? "Healthy"
                                        : "Degraded"
                                }}
                            </h3>
                        </div>
                        <div
                            class="h-12 w-12 rounded-full bg-green-500/10 flex items-center justify-center"
                        >
                            <svg
                                xmlns="http://www.w3.org/2000/svg"
                                class="h-6 w-6 text-green-500"
                                fill="none"
                                viewBox="0 0 24 24"
                                stroke="currentColor"
                                v-if="
                                    Object.values(serverStatus).every(
                                        (status) => status.rcon,
                                    )
                                "
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
                                class="h-6 w-6 text-red-500"
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
                    <div class="mt-4">
                        <p class="text-sm text-muted-foreground">
                            {{
                                Object.values(serverStatus).some(
                                    (status) => !status.rcon,
                                )
                                    ? "Some servers are offline"
                                    : "All systems operational"
                            }}
                        </p>
                    </div>
                </CardContent>
            </Card>
        </div>

        <!-- Servers Table -->
        <Card>
            <CardHeader>
                <h2 class="text-xl font-semibold">Servers</h2>
            </CardHeader>
            <CardContent>
                <div v-if="loading" class="py-8 text-center">
                    <div
                        class="animate-spin h-8 w-8 border-4 border-primary border-t-transparent rounded-full mx-auto mb-4"
                    ></div>
                    <p>Loading server information...</p>
                </div>
                <div v-else-if="servers.length === 0" class="py-8 text-center">
                    <p class="text-muted-foreground">No servers available</p>
                </div>
                <Table v-else>
                    <TableHeader>
                        <TableRow>
                            <TableHead>Server Name</TableHead>
                            <TableHead>IP Address</TableHead>
                            <TableHead>Game Port</TableHead>
                            <TableHead>RCON IP Address</TableHead>
                            <TableHead>RCON Port</TableHead>
                            <TableHead>Status</TableHead>
                            <TableHead class="text-right">Actions</TableHead>
                        </TableRow>
                    </TableHeader>
                    <TableBody>
                        <TableRow v-for="server in servers" :key="server.id">
                            <TableCell class="font-medium">{{
                                server.name
                            }}</TableCell>
                            <TableCell>{{ server.ip_address }}</TableCell>
                            <TableCell>{{ server.game_port }}</TableCell>
                            <TableCell>{{ server.rcon_ip_address || "Unknown" }}</TableCell>
                            <TableCell>{{ server.rcon_port }}</TableCell>
                            <TableCell>
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
                                            ? "Online"
                                            : "Offline"
                                    }}
                                </span>
                            </TableCell>
                            <TableCell class="text-right">
                                <NuxtLink :to="`/servers/${server.id}`">
                                    <button
                                        class="px-3 py-1 bg-primary text-primary-foreground rounded-md text-xs"
                                    >
                                        Manage
                                    </button>
                                </NuxtLink>
                            </TableCell>
                        </TableRow>
                    </TableBody>
                </Table>
            </CardContent>
        </Card>
    </div>
</template>
