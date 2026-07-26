<script setup lang="ts">
import { ref, watch, onBeforeUnmount } from "vue";
import { Badge } from "~/components/ui/badge";
import { Button } from "~/components/ui/button";
import { Input } from "~/components/ui/input";
import { toast } from "~/components/ui/toast";
import { cleanPlayerId } from "~/utils/evidence";

// Player picker for the add-ban form: debounced name search, raw Steam/EOS ID
// entry, a selected-player chip, and the previous-ban-history hint (which is
// coupled to the selected/typed id, so it lives here). The default model holds
// the Steam/EOS id and is bound to the parent form's steam_id field.

const emit = defineEmits<{
    (
        e: "suggestion",
        payload: { historyCount: number; suggestedDuration: number },
    ): void;
}>();

const model = defineModel<string>({ default: "" });

const runtimeConfig = useRuntimeConfig();

const playerSearchQuery = ref("");
const playerSearchResults = ref<any[]>([]);
const selectedPlayer = ref<any | null>(null);
const isSearchingPlayers = ref(false);
const showPlayerDropdown = ref(false);
const playerHistory = ref<any[]>([]);
const suggestedDuration = ref<number>(24);
const isLoadingHistory = ref(false);

// Search for players
async function searchPlayers(query: string) {
    if (!query || query.length < 2) {
        playerSearchResults.value = [];
        showPlayerDropdown.value = false;
        return;
    }

    isSearchingPlayers.value = true;

    try {
        const data = await useAuthFetchImperative<any>(
            `${runtimeConfig.public.backendApi}/players?search=${encodeURIComponent(query)}&limit=10`,
        );

        if (data && data.data) {
            playerSearchResults.value = data.data.players || [];
            showPlayerDropdown.value = playerSearchResults.value.length > 0;
        }
    } catch (err: any) {
        toast({ title: "Player search failed", description: extractApiErrorMessage(err, "An error occurred"), variant: "destructive" });
        playerSearchResults.value = [];
    } finally {
        isSearchingPlayers.value = false;
    }
}

// Select a player from search results
function selectPlayer(player: any) {
    selectedPlayer.value = player;
    playerSearchQuery.value = player.player_name || "";

    // Use the best available player identifier from search results.
    const playerId = String(player.steam_id || player.eos_id || "").replace(/"/g, "");
    model.value = playerId;
    showPlayerDropdown.value = false;
}

// Clear selected player
function clearSelectedPlayer() {
    selectedPlayer.value = null;
    playerSearchQuery.value = "";
    model.value = "";
}

// Debounced player search
let playerSearchTimeout: ReturnType<typeof setTimeout> | null = null;
function debouncedPlayerSearch(query: string) {
    if (playerSearchTimeout) {
        clearTimeout(playerSearchTimeout);
    }
    playerSearchTimeout = setTimeout(() => {
        searchPlayers(query);
    }, 300);
}

onBeforeUnmount(() => {
    if (playerSearchTimeout) {
        clearTimeout(playerSearchTimeout);
    }
});

// Function to fetch player ban history
async function fetchPlayerBanHistory(playerId: string) {
    if (!playerId) {
        playerHistory.value = [];
        return;
    }

    isLoadingHistory.value = true;

    try {
        const playerIds = cleanPlayerId(playerId);
        const historyId = playerIds.steamId || playerIds.eosId || "";
        if (!historyId) {
            playerHistory.value = [];
            return;
        }

        const data = await useAuthFetchImperative<any>(
            `${runtimeConfig.public.backendApi}/players/${historyId}/ban-history`,
        );

        if (data && data.data) {
            playerHistory.value = data.data.history || [];
            // Calculate suggested ban duration based on history
            calculateSuggestedDuration();
        } else {
            playerHistory.value = [];
            suggestedDuration.value = 24; // Default for first offenders
        }
    } catch (err: any) {
        console.error("Failed to fetch player history:", err);
        playerHistory.value = [];
    } finally {
        isLoadingHistory.value = false;
    }
}

// Calculate suggested ban duration based on player history
function calculateSuggestedDuration() {
    if (playerHistory.value.length === 0) {
        // First offense - suggest 24 hours
        suggestedDuration.value = 1;
        return;
    }

    // Sort history by date (newest first)
    const sortedHistory = [...playerHistory.value].sort(
        (a, b) =>
            new Date(b.created_at).getTime() - new Date(a.created_at).getTime(),
    );

    // Count previous offenses
    const offenseCount = sortedHistory.length;

    // Progressive ban duration based on number of previous offenses
    if (offenseCount === 1) {
        suggestedDuration.value = 3; // 3 days for second offense
    } else if (offenseCount === 2) {
        suggestedDuration.value = 7; // 7 days for third offense
    } else if (offenseCount === 3) {
        suggestedDuration.value = 14; // 14 days for fourth offense
    } else {
        suggestedDuration.value = 0; // Permanent ban for repeat offenders
    }
}

// Fetch history whenever the bound id becomes a valid Steam/EOS id. This
// covers both picking a search result and typing an id manually.
watch(model, (value) => {
    if (!value) {
        playerHistory.value = [];
        return;
    }
    if (/^\d{17}$/.test(value) || /^[0-9a-fA-F]{32}$/.test(value)) {
        fetchPlayerBanHistory(value);
    }
});

// Report the history-derived suggestion so the parent's duration field can
// offer a "Use Suggested" shortcut.
watch([() => playerHistory.value.length, suggestedDuration], () => {
    emit("suggestion", {
        historyCount: playerHistory.value.length,
        suggestedDuration: suggestedDuration.value,
    });
});
</script>

<template>
    <div class="space-y-2">
        <!-- Selected Player Display -->
        <div v-if="selectedPlayer" class="flex items-center gap-2 p-2 border rounded-md bg-muted/50">
            <div class="flex-1">
                <p class="font-medium text-sm">{{ selectedPlayer.player_name }}</p>
                <p class="text-xs text-muted-foreground">{{ selectedPlayer.steam_id }}</p>
            </div>
            <Button
                type="button"
                variant="ghost"
                size="sm"
                @click="clearSelectedPlayer"
            >
                <Icon name="lucide:x" class="h-4 w-4" />
            </Button>
        </div>

        <!-- Search Input (shown when no player selected) -->
        <div v-if="!selectedPlayer" class="relative">
            <Input
                v-model="playerSearchQuery"
                placeholder="Search by player name..."
                @input="(e: Event) => debouncedPlayerSearch((e.target as HTMLInputElement).value)"
                @focus="showPlayerDropdown = playerSearchResults.length > 0"
            />
            <div v-if="isSearchingPlayers" class="absolute right-3 top-1/2 -translate-y-1/2">
                <Icon name="mdi:loading" class="h-4 w-4 animate-spin" />
            </div>

            <!-- Search Results Dropdown -->
            <div
                v-if="showPlayerDropdown && playerSearchResults.length > 0"
                class="absolute z-50 w-full mt-1 bg-popover border rounded-md shadow-md max-h-60 overflow-auto"
            >
                <div
                    v-for="player in playerSearchResults"
                    :key="player.steam_id || player.eos_id"
                    class="p-2 hover:bg-muted cursor-pointer border-b last:border-b-0"
                    @click="selectPlayer(player)"
                >
                    <p class="font-medium text-sm">{{ player.player_name }}</p>
                    <p class="text-xs text-muted-foreground">{{ player.steam_id || player.eos_id || "Unknown ID" }}</p>
                    <p class="text-xs text-muted-foreground">Last seen: {{ new Date(player.last_seen).toLocaleDateString() }}</p>
                </div>
            </div>
        </div>

        <!-- Player ID Input - always present for form binding, hidden when player selected -->
        <Input
            v-model="model"
            :class="{ hidden: selectedPlayer }"
            placeholder="Steam ID (76561198012345678) or EOS ID (32-char hex)"
        />
        <p v-if="!selectedPlayer" class="text-sm text-muted-foreground">
            Search for a player by name, or enter their Steam ID or EOS ID directly
        </p>

        <!-- Player History Loading Indicator -->
        <div
            v-if="isLoadingHistory"
            class="border rounded-md p-3 bg-muted/50 flex items-center"
        >
            <div
                class="animate-spin h-5 w-5 border-2 border-primary border-t-transparent rounded-full mr-2"
            ></div>
            <span class="text-sm">Looking up player history...</span>
        </div>

        <!-- Player Ban History Card -->
        <div
            v-else-if="playerHistory.length > 0"
            class="border rounded-md p-3 bg-muted/50"
        >
            <h4 class="font-medium mb-2 flex items-center">
                <span class="text-orange-500 mr-1">⚠️</span>
                Previous Ban History
            </h4>
            <p class="text-sm text-muted-foreground mb-2">
                This player has been banned
                {{ playerHistory.length }} time{{
                    playerHistory.length > 1 ? "s" : ""
                }}
                before.
            </p>
            <ul class="text-sm space-y-1 mb-2">
                <li
                    v-for="(ban, index) in playerHistory.slice(0, 3)"
                    :key="index"
                >
                    <span class="text-muted-foreground">{{
                        new Date(ban.created_at).toLocaleDateString()
                    }}:</span>
                    {{ ban.reason }}
                </li>
            </ul>
            <div class="flex items-center border-t pt-2 mt-2">
                <span class="mr-2 text-sm">Suggested ban:</span>
                <Badge variant="destructive" v-if="suggestedDuration === 0">
                    Permanent
                </Badge>
                <Badge variant="default" v-else>
                    {{ suggestedDuration }}
                    {{ suggestedDuration === 1 ? "day" : "days" }}
                </Badge>
            </div>
        </div>
    </div>
</template>
