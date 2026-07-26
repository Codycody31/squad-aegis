<script setup lang="ts">
import { computed } from "vue";
import StatCard from "~/components/StatCard.vue";
import { Badge } from "~/components/ui/badge";

const props = defineProps<{
    serverInfo: any;
    rconServerInfo: any;
    rconLoading?: boolean;
    rconError?: string | null;
}>();

const playersValue = computed(() => {
    const info = props.rconServerInfo;
    if (info) {
        const reserve = info.player_reserve_count || 0;
        const max = (info.max_players || 0) - reserve;
        return `${info.player_count || 0}/${max}${reserve !== 0 ? `(+${reserve})` : ""}`;
    }
    const players = props.serverInfo?.metrics?.players;
    if (players) return `${players.total} / ${players.max}`;
    return null;
});

const queueSubline = computed(() => {
    const queue = props.rconServerInfo?.public_queue || 0;
    return queue > 0 ? `${queue} in queue` : undefined;
});

const currentLayer = computed(
    () =>
        props.serverInfo?.metrics?.current?.layer ||
        props.serverInfo?.metrics?.current?.map ||
        null,
);

const layerSubline = computed(() => {
    const next =
        props.serverInfo?.metrics?.next?.map ||
        props.serverInfo?.metrics?.next?.layer;
    if (next) return `Next: ${next}`;
    const gamePort = props.serverInfo?.server?.game_port;
    return gamePort ? `Game port ${gamePort}` : undefined;
});

const rconConnected = computed(
    () => !props.rconError && !!props.rconServerInfo,
);
</script>

<template>
    <div class="grid grid-cols-2 lg:grid-cols-4 gap-4">
        <StatCard
            label="Players"
            :value="playersValue"
            :subline="queueSubline"
            icon="mdi:account-group"
            :loading="rconLoading && !rconServerInfo"
        />
        <StatCard
            label="Current Layer"
            :value="currentLayer"
            :subline="layerSubline"
            icon="mdi:map"
        />
        <StatCard
            label="RCON"
            icon="mdi:console-network"
            :icon-class="
                rconConnected
                    ? 'bg-green-500/10 text-green-600'
                    : 'bg-red-500/10 text-red-600'
            "
            :loading="rconLoading && !rconServerInfo"
        >
            <template #value>
                {{ rconError ? "Error" : rconConnected ? "Connected" : "Unknown" }}
            </template>
        </StatCard>
        <StatCard label="License" icon="mdi:license">
            <template #value>
                <Badge
                    v-if="rconServerInfo?.licensed_server !== undefined"
                    :variant="
                        rconServerInfo?.licensed_server
                            ? 'default'
                            : 'destructive'
                    "
                >
                    {{
                        rconServerInfo?.licensed_server
                            ? "Licensed"
                            : "Unlicensed"
                    }}
                </Badge>
                <span v-else>—</span>
            </template>
        </StatCard>
    </div>
</template>
