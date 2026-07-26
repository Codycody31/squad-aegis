<script setup lang="ts">
import { computed, ref } from "vue";
import { Badge } from "~/components/ui/badge";
import { Button } from "~/components/ui/button";
import { Input } from "~/components/ui/input";
import {
    Select,
    SelectContent,
    SelectItem,
    SelectTrigger,
    SelectValue,
} from "~/components/ui/select";
import DataTable from "~/components/data-table/DataTable.vue";
import type { DataTableColumn } from "~/components/data-table/types";
import { UI_PERMISSIONS } from "~/constants/permissions";
import { formatDate, formatExpiresAt } from "~/utils/evidence";
import type { BannedPlayer } from "~/types/bans";

// Ban list table with its own search text and status filter
// (All | Active | Expired). Row actions are permission-gated.

const props = defineProps<{
    bans: BannedPlayer[];
    loading: boolean;
    serverId: string;
}>();

const emit = defineEmits<{
    (e: "edit", ban: BannedPlayer): void;
    (e: "remove", ban: BannedPlayer): void;
    (e: "view", ban: BannedPlayer): void;
}>();

const authStore = useAuthStore();

const searchQuery = ref("");
const statusFilter = ref<"all" | "active" | "expired">("all");

const canEdit = computed(() =>
    authStore.hasPermission(props.serverId, UI_PERMISSIONS.BANS_EDIT),
);
const canRemove = computed(() =>
    authStore.hasPermission(props.serverId, UI_PERMISSIONS.BANS_DELETE),
);

// Helper function to check if a ban is expired
function isBanExpired(ban: BannedPlayer): boolean {
    // Permanent bans never expire
    if (ban.permanent) {
        return false;
    }

    // Check if expires_at exists and is in the past
    if (ban.expires_at) {
        const expiresAt = new Date(ban.expires_at);
        const now = new Date();
        return expiresAt < now;
    }

    // If no expires_at and not permanent, consider it expired
    // (shouldn't happen in normal cases, but handle it)
    return false;
}

// Computed property for filtered banned players
const filteredBans = computed(() => {
    let filtered = props.bans;

    if (statusFilter.value === "active") {
        filtered = filtered.filter((player) => !isBanExpired(player));
    } else if (statusFilter.value === "expired") {
        filtered = filtered.filter((player) => isBanExpired(player));
    }

    // Apply search query filter
    if (searchQuery.value.trim()) {
        const query = searchQuery.value.toLowerCase();
        filtered = filtered.filter(
            (player) =>
                player.name.toLowerCase().includes(query) ||
                player.steam_id?.includes(query) ||
                player.eos_id?.toLowerCase().includes(query) ||
                player.reason.toLowerCase().includes(query),
        );
    }

    return filtered;
});

const emptyText = computed(() =>
    props.bans.length === 0
        ? "No banned players found"
        : "No players match your search",
);

const columns: DataTableColumn<BannedPlayer>[] = [
    { key: "player", label: "Player ID", hideOnMobile: true },
    { key: "reason", label: "Reason", accessor: (row) => row.reason },
    { key: "rule", label: "Rule" },
    { key: "evidence", label: "Evidence" },
    {
        key: "created_at",
        label: "Banned At",
        mobileLabel: "Banned",
        accessor: (row) => formatDate(row.created_at),
    },
    { key: "duration", label: "Duration" },
    { key: "source", label: "Source" },
];

function hasEvidence(ban: BannedPlayer): boolean {
    return Boolean(
        (ban.evidence && ban.evidence.length > 0) ||
            (ban.evidence_text && ban.evidence_text.trim()),
    );
}
</script>

<template>
    <div>
        <div class="flex flex-col sm:flex-row items-stretch sm:items-center gap-3 mb-3 sm:mb-4">
            <Input
                v-model="searchQuery"
                placeholder="Search by Steam ID, EOS ID, or reason..."
                class="flex-grow text-sm sm:text-base"
            />
            <Select v-model="statusFilter">
                <SelectTrigger class="w-full sm:w-36">
                    <SelectValue />
                </SelectTrigger>
                <SelectContent>
                    <SelectItem value="all">All</SelectItem>
                    <SelectItem value="active">Active</SelectItem>
                    <SelectItem value="expired">Expired</SelectItem>
                </SelectContent>
            </Select>
        </div>

        <div class="text-xs sm:text-sm text-muted-foreground mb-2">
            Showing {{ filteredBans.length }} of {{ bans.length }} bans
        </div>

        <DataTable
            :columns="columns"
            :rows="filteredBans"
            row-key="id"
            :loading="loading && bans.length === 0"
            :empty-text="emptyText"
        >
            <template #cell-player="{ row: player }">
                <RouterLink
                    v-if="player.steam_id || player.eos_id"
                    :to="`/players/${player.steam_id || player.eos_id}`"
                    class="hover:underline"
                >
                    <div class="font-medium text-sm sm:text-base text-primary">
                        {{ player.name && player.name !== player.steam_id && player.name !== player.eos_id ? player.name : (player.steam_id || player.eos_id) }}
                    </div>
                    <div
                        v-if="player.name && player.name !== player.steam_id && player.name !== player.eos_id"
                        class="text-xs text-muted-foreground"
                    >
                        {{ player.steam_id }}
                    </div>
                    <div
                        v-if="player.eos_id"
                        class="text-xs text-muted-foreground"
                    >
                        EOS: {{ player.eos_id }}
                    </div>
                </RouterLink>
                <div v-else>
                    <div class="font-medium text-sm sm:text-base">
                        {{ player.name && player.name !== player.eos_id ? player.name : player.eos_id }}
                    </div>
                    <div
                        v-if="player.eos_id && player.name && player.name !== player.eos_id"
                        class="text-xs text-muted-foreground"
                    >
                        EOS: {{ player.eos_id }}
                    </div>
                </div>
            </template>

            <template #mobile-title="{ row: player }">
                <RouterLink
                    v-if="player.steam_id || player.eos_id"
                    :to="`/players/${player.steam_id || player.eos_id}`"
                    class="hover:underline"
                >
                    <div class="font-semibold text-sm sm:text-base text-primary">
                        {{ player.name && player.name !== player.steam_id && player.name !== player.eos_id ? player.name : (player.steam_id || player.eos_id) }}
                    </div>
                    <div
                        v-if="player.name && player.name !== player.steam_id && player.name !== player.eos_id"
                        class="text-xs text-muted-foreground"
                    >
                        {{ player.steam_id }}
                    </div>
                </RouterLink>
                <div v-else class="font-semibold text-sm sm:text-base">
                    {{ player.name && player.name !== player.eos_id ? player.name : player.eos_id }}
                </div>
            </template>

            <template #cell-reason="{ row: player }">
                <span class="text-xs sm:text-sm break-words">{{ player.reason }}</span>
            </template>

            <template #cell-rule="{ row: player }">
                <span
                    v-if="player.rule_name"
                    class="font-medium text-xs sm:text-sm"
                    >{{ player.rule_name }}</span
                >
                <span v-else class="text-muted-foreground text-xs"
                    >No rule specified</span
                >
            </template>

            <template #cell-evidence="{ row: player }">
                <Button
                    v-if="hasEvidence(player)"
                    variant="ghost"
                    size="sm"
                    @click="emit('view', player)"
                    class="h-auto p-1 text-xs"
                >
                    <div class="flex items-center gap-1">
                        <Icon
                            name="lucide:file-check"
                            class="h-3 w-3 sm:h-4 sm:w-4 text-green-500"
                        />
                        <span class="text-xs sm:text-sm">
                            {{ player.evidence ? player.evidence.length : 0 }} item(s)
                        </span>
                    </div>
                </Button>
                <span v-else class="text-muted-foreground text-xs">None</span>
            </template>

            <template #cell-duration="{ row: player }">
                <Badge
                    :variant="player.permanent ? 'destructive' : 'outline'"
                    class="text-xs"
                >
                    {{ formatExpiresAt(player) }}
                </Badge>
            </template>

            <template #cell-source="{ row: player }">
                <Badge variant="secondary" class="text-xs">
                    {{ player.ban_list_name || "Manual" }}
                </Badge>
            </template>

            <template #actions="{ row: player }">
                <div class="flex gap-2 justify-end">
                    <Button
                        v-if="canEdit"
                        variant="outline"
                        size="sm"
                        :disabled="loading"
                        class="text-xs"
                        @click="emit('edit', player)"
                    >
                        Edit
                    </Button>
                    <Button
                        v-if="canRemove"
                        variant="destructive"
                        size="sm"
                        :disabled="loading"
                        class="text-xs"
                        @click="emit('remove', player)"
                    >
                        Unban
                    </Button>
                </div>
            </template>
        </DataTable>
    </div>
</template>
