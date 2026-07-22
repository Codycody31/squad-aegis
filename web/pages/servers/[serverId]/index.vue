<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from "vue";
import PageHeader from "~/components/layout/PageHeader.vue";
import BroadcastDialog from "~/components/BroadcastDialog.vue";
import ServerStatusStrip from "~/components/server-dashboard/ServerStatusStrip.vue";
import CurrentMapCard from "~/components/server-dashboard/CurrentMapCard.vue";
import MapChangeDialog from "~/components/server-dashboard/MapChangeDialog.vue";
import NextLayerDialog from "~/components/server-dashboard/NextLayerDialog.vue";
import EndMatchDialog from "~/components/server-dashboard/EndMatchDialog.vue";
import QuickActions from "~/components/server-dashboard/QuickActions.vue";
import RecentJoinsCard from "~/components/server-dashboard/RecentJoinsCard.vue";
import TeamsSummaryCard from "~/components/server-dashboard/TeamsSummaryCard.vue";
import ServerInfoCard from "~/components/server-dashboard/ServerInfoCard.vue";
import { Button } from "~/components/ui/button";
import type { RecentJoin, Team } from "~/components/server-dashboard/types";

definePageMeta({ middleware: ["auth"] });

const route = useRoute();
const serverId = route.params.serverId as string;

// State variables
const loading = ref(true);
const error = ref<string | null>(null);
const serverInfo = ref<any>(null);

const rconServerInfo = ref<any>(null);
const rconServerInfoLoading = ref(false);
const rconServerInfoError = ref<string | null>(null);

const teamsData = ref<Team[]>([]);
const loadingTeams = ref(false);
const errorTeams = ref<string | null>(null);

const recentJoins = ref<RecentJoin[]>([]);
const loadingRecentJoins = ref(false);
const errorRecentJoins = ref<string | null>(null);

// Dialog state
const showMapChangeDialog = ref(false);
const showNextLayerDialog = ref(false);
const showEndMatchDialog = ref(false);
const showBroadcastDialog = ref(false);

interface TeamsResponse {
    data: {
        teams: Team[];
    };
}

interface RecentJoinsResponse {
    data: {
        joins: RecentJoin[];
        count: number;
    };
}

// Fetch server information
async function fetchServerInfo(background = false) {
    if (!background) loading.value = true;
    error.value = null;

    const runtimeConfig = useRuntimeConfig();

    try {
        const { data: responseData, error: fetchError } = await useAuthFetch(
            `${runtimeConfig.public.backendApi}/servers/${serverId}`,
            {
                method: "GET",
            },
        );

        if (fetchError.value) {
            throw new Error(
                extractApiErrorMessage(fetchError.value, "Failed to fetch server information"),
            );
        }

        if (responseData.value && responseData.value.data) {
            serverInfo.value = responseData.value.data;
        }
    } catch (err: any) {
        error.value = extractApiErrorMessage(err, "An error occurred while fetching server information");
        console.error(err);
    } finally {
        loading.value = false;
    }
}

// Fetch server metrics
async function fetchServerMetrics() {
    const runtimeConfig = useRuntimeConfig();

    const { data: responseData, error: fetchError } = await useAuthFetch(
        `${runtimeConfig.public.backendApi}/servers/${serverId}/metrics`,
        {
            method: "GET",
        },
    );

    if (fetchError.value) {
        throw new Error(
            extractApiErrorMessage(fetchError.value, "Failed to fetch server metrics"),
        );
    }

    if (responseData.value && responseData.value.data) {
        serverInfo.value.metrics = responseData.value.data.metrics;
    }
}

async function fetchRconServerInfo(background = false) {
    if (!background) rconServerInfoLoading.value = true;
    rconServerInfoError.value = null;

    const runtimeConfig = useRuntimeConfig();

    try {
        const { data: responseData, error: fetchError } = await useAuthFetch(
            `${runtimeConfig.public.backendApi}/servers/${serverId}/rcon/server-info`,
            {
                method: "GET",
            },
        );

        if (fetchError.value) {
            throw new Error(
                extractApiErrorMessage(fetchError.value, "Failed to fetch rcon server information"),
            );
        }

        if (responseData.value && responseData.value.data) {
            rconServerInfo.value = responseData.value.data.serverInfo;
        }
    } catch (err: any) {
        rconServerInfoError.value = extractApiErrorMessage(err, "An error occurred while fetching rcon server information");
        console.error(err);
    } finally {
        rconServerInfoLoading.value = false;
    }
}

// Fetch teams and squads data
async function fetchTeamsData(background = false) {
    if (!background) loadingTeams.value = true;
    errorTeams.value = null;

    const runtimeConfig = useRuntimeConfig();

    try {
        const { data, error: fetchError } = await useAuthFetch<TeamsResponse>(
            `${runtimeConfig.public.backendApi}/servers/${serverId}/rcon/server-population`,
            {
                method: "GET",
            },
        );

        if (fetchError.value) {
            throw new Error(
                extractApiErrorMessage(fetchError.value, "Failed to fetch teams data"),
            );
        }

        if (data.value && data.value.data) {
            teamsData.value = data.value.data.teams || [];
        }
    } catch (err: any) {
        errorTeams.value =
            extractApiErrorMessage(err, "An error occurred while fetching teams data");
        console.error(err);
    } finally {
        loadingTeams.value = false;
    }
}

// Fetch recent server joins
async function fetchRecentJoins(background = false) {
    if (!background) loadingRecentJoins.value = true;
    errorRecentJoins.value = null;

    const runtimeConfig = useRuntimeConfig();

    try {
        const { data, error: fetchError } =
            await useAuthFetch<RecentJoinsResponse>(
                `${runtimeConfig.public.backendApi}/servers/${serverId}/feeds/recent-joins?limit=5`,
                {
                    method: "GET",
                },
            );

        if (fetchError.value) {
            throw new Error(
                extractApiErrorMessage(fetchError.value, "Failed to fetch recent joins"),
            );
        }

        if (data.value && data.value.data) {
            recentJoins.value = data.value.data.joins || [];
        }
    } catch (err: any) {
        errorRecentJoins.value =
            extractApiErrorMessage(err, "An error occurred while fetching recent joins");
        console.error(err);
    } finally {
        loadingRecentJoins.value = false;
    }
}

function refresh(background = false) {
    fetchServerInfo(background);
    fetchTeamsData(background);
    fetchServerMetrics();
    fetchRconServerInfo(background);
    fetchRecentJoins(background);
}

// Visibility-gated auto-refresh
const AUTO_REFRESH_INTERVAL_MS = 30_000;
let refreshTimer: ReturnType<typeof setInterval> | null = null;

function startAutoRefresh() {
    stopAutoRefresh();
    refreshTimer = setInterval(() => refresh(true), AUTO_REFRESH_INTERVAL_MS);
}

function stopAutoRefresh() {
    if (refreshTimer) {
        clearInterval(refreshTimer);
        refreshTimer = null;
    }
}

function handleVisibilityChange() {
    if (document.hidden) {
        stopAutoRefresh();
    } else {
        refresh(true);
        startAutoRefresh();
    }
}

onMounted(() => {
    startAutoRefresh();
    document.addEventListener("visibilitychange", handleVisibilityChange);
});

onBeforeUnmount(() => {
    stopAutoRefresh();
    document.removeEventListener("visibilitychange", handleVisibilityChange);
});

refresh();
</script>

<template>
    <div class="p-4 md:p-6">
        <PageHeader title="Server Dashboard">
            <template #actions>
                <Button :disabled="loading" @click="refresh()">
                    {{ loading ? "Refreshing..." : "Refresh" }}
                </Button>
            </template>
        </PageHeader>

        <div v-if="error" class="bg-red-500 text-white p-4 rounded mb-4">
            {{ error }}
        </div>

        <div v-if="loading && !serverInfo" class="text-center py-8">
            <div
                class="animate-spin h-8 w-8 border-4 border-primary border-t-transparent rounded-full mx-auto mb-4"
            ></div>
            <p>Loading server information...</p>
        </div>

        <div v-else class="space-y-4">
            <ServerStatusStrip
                :server-info="serverInfo"
                :rcon-server-info="rconServerInfo"
                :rcon-loading="rconServerInfoLoading"
                :rcon-error="rconServerInfoError"
            />

            <div class="grid grid-cols-1 lg:grid-cols-2 gap-4">
                <CurrentMapCard
                    :server-id="serverId"
                    :server-info="serverInfo"
                    :rcon-server-info="rconServerInfo"
                    @change-map="showMapChangeDialog = true"
                    @set-next="showNextLayerDialog = true"
                    @end-match="showEndMatchDialog = true"
                />
                <QuickActions
                    :server-id="serverId"
                    @broadcast="showBroadcastDialog = true"
                />
            </div>

            <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
                <RecentJoinsCard
                    :joins="recentJoins"
                    :loading="loadingRecentJoins"
                    :error="errorRecentJoins"
                />
                <TeamsSummaryCard
                    :server-id="serverId"
                    :teams="teamsData"
                    :server-info="serverInfo"
                    :loading="loadingTeams"
                    :error="errorTeams"
                />
            </div>

            <ServerInfoCard
                :server="serverInfo?.server"
                :version="rconServerInfo?.version || rconServerInfo?.game_version"
                :licensed="rconServerInfo?.licensed_server"
            />
        </div>

        <MapChangeDialog
            v-model:open="showMapChangeDialog"
            :server-id="serverId"
            :current-layer="serverInfo?.metrics?.current?.map"
            @completed="refresh(true)"
        />
        <NextLayerDialog
            v-model:open="showNextLayerDialog"
            :server-id="serverId"
            :current-next-layer="serverInfo?.metrics?.next?.map"
            @completed="refresh(true)"
        />
        <EndMatchDialog
            v-model:open="showEndMatchDialog"
            :server-id="serverId"
            @completed="refresh(true)"
        />
        <BroadcastDialog
            v-model:open="showBroadcastDialog"
            :server-id="serverId"
        />
    </div>
</template>
