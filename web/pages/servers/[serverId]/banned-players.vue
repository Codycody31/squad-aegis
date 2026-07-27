<script setup lang="ts">
import { ref, onMounted, onUnmounted, computed, watch } from "vue";
import { Button } from "~/components/ui/button";
import { Input } from "~/components/ui/input";
import {
    Card,
    CardContent,
    CardDescription,
    CardHeader,
    CardTitle,
} from "~/components/ui/card";
import {
    Dialog,
    DialogContent,
    DialogDescription,
    DialogHeader,
    DialogTitle,
} from "~/components/ui/dialog";
import {
    DropdownMenu,
    DropdownMenuContent,
    DropdownMenuItem,
    DropdownMenuTrigger,
} from "~/components/ui/dropdown-menu";
import {
    Popover,
    PopoverContent,
    PopoverTrigger,
} from "~/components/ui/popover";
import {
    Tabs,
    TabsContent,
    TabsList,
    TabsTrigger,
} from "~/components/ui/tabs";
import { toast } from "~/components/ui/toast";
import { UI_PERMISSIONS } from "~/constants/permissions";
import PageHeader from "~/components/layout/PageHeader.vue";
import BanFormDialog from "~/components/bans/BanFormDialog.vue";
import BanImportDialog from "~/components/bans/BanImportDialog.vue";
import BanListSubscriptionsCard from "~/components/bans/BanListSubscriptionsCard.vue";
import BanTable from "~/components/bans/BanTable.vue";
import BanViewDialog from "~/components/bans/BanViewDialog.vue";
import type {
    BannedPlayer,
    BannedPlayerDetailResponse,
    BannedPlayersResponse,
} from "~/types/bans";

definePageMeta({ middleware: ["auth"] });

const authStore = useAuthStore();
const runtimeConfig = useRuntimeConfig();
const route = useRoute();
const serverId = Array.isArray(route.params.serverId)
    ? route.params.serverId[0]
    : route.params.serverId;

const loading = ref(true);
const error = ref<string | null>(null);
const bannedPlayers = ref<BannedPlayer[]>([]);
const searchQuery = ref("");
const statusFilter = ref<"all" | "active" | "expired">("all");
const {
    limit,
    cursor,
    page,
    canGoPrevious,
    canGoNext,
    setPageInfo,
    reset: resetPagination,
    next: advancePage,
    previous: returnToPreviousPage,
} = useCursorPagination(50);
let searchTimer: ReturnType<typeof setTimeout> | null = null;

// Add/edit ban dialog (components/bans/BanFormDialog.vue)
const showBanFormDialog = ref(false);
const banFormMode = ref<"create" | "edit">("create");
const editingBan = ref<BannedPlayer | null>(null);

// Evidence view dialog (components/bans/BanViewDialog.vue)
const showEvidenceViewDialog = ref(false);
const viewingBanEvidence = ref<BannedPlayer | null>(null);
const detailLoadingId = ref<string | null>(null);
const banDetailCache = new Map<string, BannedPlayer>();

// Ban Import from Bans.cfg (components/bans/BanImportDialog.vue)
const showImportDialog = ref(false);

// Ban config URL manual-copy fallback
const showBanCfgDialog = ref(false);

// Function to fetch banned players data
async function fetchBannedPlayers() {
    loading.value = true;
    error.value = null;

    try {
        const params = new URLSearchParams({
            limit: limit.value.toString(),
            status: statusFilter.value,
        });
        if (cursor.value) {
            params.set("cursor", cursor.value);
        }
        if (searchQuery.value.trim()) {
            params.set("search", searchQuery.value.trim());
        }

        const response = await useAuthFetchImperative<BannedPlayersResponse>(
            `${runtimeConfig.public.backendApi}/servers/${serverId}/bans?${params.toString()}`,
            { method: "GET" },
        );

        bannedPlayers.value = response.data?.bans || [];
        setPageInfo(response.data?.page_info);
    } catch (err: any) {
        error.value = extractApiErrorMessage(err, "An error occurred while fetching banned players data");
        console.error(err);
    } finally {
        loading.value = false;
    }
}

async function fetchBanDetail(ban: BannedPlayer): Promise<BannedPlayer | null> {
    const cached = banDetailCache.get(ban.id);
    if (cached) {
        return cached;
    }

    if (detailLoadingId.value) {
        return null;
    }

    detailLoadingId.value = ban.id;
    try {
        const response =
            await useAuthFetchImperative<BannedPlayerDetailResponse>(
                `${runtimeConfig.public.backendApi}/servers/${serverId}/bans/${ban.id}`,
                { method: "GET" },
            );
        const detail = response.data?.ban;
        if (!detail) {
            throw new Error("Ban details were not returned");
        }
        const detailedBan = {
            ...ban,
            ...detail,
            name: ban.name || detail.name,
        };
        banDetailCache.set(ban.id, detailedBan);
        return detailedBan;
    } catch (err: any) {
        error.value = extractApiErrorMessage(
            err,
            "An error occurred while fetching ban evidence",
        );
        return null;
    } finally {
        detailLoadingId.value = null;
    }
}

async function resetBanList() {
    banDetailCache.clear();
    resetPagination();
    await fetchBannedPlayers();
}

function nextPage() {
    if (advancePage()) {
        fetchBannedPlayers();
    }
}

function previousPage() {
    if (returnToPreviousPage()) {
        fetchBannedPlayers();
    }
}

function changePageSize(value: number) {
    limit.value = value;
    resetPagination();
    fetchBannedPlayers();
}

// Function to remove a ban
async function removeBan(banId: string) {
    if (!confirm("Are you sure you want to remove this ban?")) {
        return;
    }

    loading.value = true;
    error.value = null;

    const runtimeConfig = useRuntimeConfig();

    try {
        const { data, error: fetchError } = await useAuthFetch(
            `${runtimeConfig.public.backendApi}/servers/${serverId}/bans/${banId}`,
            {
                method: "DELETE",
            },
        );

        if (fetchError.value) {
            throw new Error(extractApiErrorMessage(fetchError.value, "Failed to remove ban"));
        }

        toast({
            title: "Success",
            description: "Ban removed successfully",
        });

        await resetBanList();
    } catch (err: any) {
        error.value = extractApiErrorMessage(err, "An error occurred while removing the ban");
    } finally {
        loading.value = false;
    }
}

// Function to open the add ban dialog
function openAddBanDialog() {
    banFormMode.value = "create";
    editingBan.value = null;
    showBanFormDialog.value = true;
}

// Function to open the edit ban dialog
async function openEditBanDialog(ban: BannedPlayer) {
    const detailedBan = await fetchBanDetail(ban);
    if (!detailedBan) {
        return;
    }
    banFormMode.value = "edit";
    editingBan.value = detailedBan;
    showBanFormDialog.value = true;
}

// Function to open evidence view dialog
async function openEvidenceViewDialog(ban: BannedPlayer) {
    const detailedBan = await fetchBanDetail(ban);
    if (!detailedBan) {
        return;
    }
    viewingBanEvidence.value = detailedBan;
    showEvidenceViewDialog.value = true;
}

watch(searchQuery, () => {
    resetPagination();
    if (searchTimer) {
        clearTimeout(searchTimer);
    }
    searchTimer = setTimeout(fetchBannedPlayers, 350);
});

watch(statusFilter, () => {
    if (searchTimer) {
        clearTimeout(searchTimer);
        searchTimer = null;
    }
    resetPagination();
    fetchBannedPlayers();
});

onMounted(() => {
    fetchBannedPlayers();
});

onUnmounted(() => {
    if (searchTimer) {
        clearTimeout(searchTimer);
    }
});

// Computed property for ban config URL
const banCfgUrl = computed(() => {
    var url = "";
    if (runtimeConfig.public.backendApi.startsWith("/")) {
        // Relative URL, construct full URL
        const origin = window.location.origin;
        url = `${origin}${runtimeConfig.public.backendApi}/servers/${serverId}/bans/cfg`;
    } else {
        url = `${runtimeConfig.public.backendApi}/servers/${serverId}/bans/cfg`;
    }
    return url;
});

async function copyBanCfgUrl() {
    const ok = await copyToClipboard(banCfgUrl.value);
    if (ok) {
        toast({
            title: "Success",
            description: "Ban configuration URL copied to clipboard",
        });
        return;
    }

    // Fallback: open a dialog showing the URL for manual copy
    showBanCfgDialog.value = true;
    toast({
        title: "Copy failed",
        description: "Clipboard unavailable. Copy the URL manually.",
        variant: "destructive",
    });
}

function selectBanCfgUrl(event: Event) {
    const input = event.target as HTMLInputElement;
    input.select();
}
</script>

<template>
    <div class="p-4 md:p-6">
        <PageHeader title="Banned Players">
            <template #title-extra>
                <Popover>
                    <PopoverTrigger as-child>
                        <Button variant="ghost" size="icon" class="h-7 w-7">
                            <Icon name="lucide:info" class="h-4 w-4" />
                            <span class="sr-only">About bans</span>
                        </Button>
                    </PopoverTrigger>
                    <PopoverContent class="w-96 space-y-2 text-sm text-muted-foreground">
                        <p>
                            This page shows players who have been banned from
                            the server. You can add new bans manually or remove
                            existing bans.
                        </p>
                        <p>
                            Permanent bans will remain in effect until manually
                            removed. Temporary bans will expire after the
                            specified duration.
                        </p>
                        <p>
                            <strong>Note:</strong> Squad servers typically
                            cache ban configurations and refresh them
                            periodically. Changes to ban list subscriptions
                            will be reflected in the ban configuration
                            immediately, but may take some time to take effect
                            in-game depending on your server's ban list refresh
                            interval.
                        </p>
                    </PopoverContent>
                </Popover>
            </template>
            <template #actions>
                <Button
                    v-if="authStore.hasPermission(serverId as string, UI_PERMISSIONS.BANS_CREATE)"
                    @click="openAddBanDialog"
                >
                    <Icon name="lucide:plus" class="h-4 w-4 mr-1" />
                    Add Ban
                </Button>
                <DropdownMenu>
                    <DropdownMenuTrigger as-child>
                        <Button variant="outline" size="icon">
                            <Icon name="lucide:more-horizontal" class="h-4 w-4" />
                            <span class="sr-only">More actions</span>
                        </Button>
                    </DropdownMenuTrigger>
                    <DropdownMenuContent align="end">
                        <DropdownMenuItem
                            v-if="authStore.hasPermission(serverId as string, UI_PERMISSIONS.BANS_CREATE)"
                            @click="showImportDialog = true"
                        >
                            Import from Bans.cfg
                        </DropdownMenuItem>
                        <DropdownMenuItem @click="copyBanCfgUrl">
                            Copy Ban Config URL
                        </DropdownMenuItem>
                        <DropdownMenuItem
                            :disabled="loading"
                            @click="fetchBannedPlayers"
                        >
                            Refresh
                        </DropdownMenuItem>
                    </DropdownMenuContent>
                </DropdownMenu>
            </template>
        </PageHeader>

        <div
            v-if="error"
            class="bg-red-500 text-white p-3 sm:p-4 rounded mb-3 sm:mb-4 text-sm sm:text-base"
        >
            {{ error }}
        </div>

        <Tabs default-value="bans">
            <TabsList class="mb-3 sm:mb-4">
                <TabsTrigger value="bans">Bans</TabsTrigger>
                <TabsTrigger value="subscriptions">Subscriptions</TabsTrigger>
            </TabsList>

            <TabsContent value="bans">
                <Card>
                    <CardHeader class="pb-2 sm:pb-3">
                        <CardTitle class="text-base sm:text-lg">Ban List</CardTitle>
                        <CardDescription class="text-xs sm:text-sm">
                            View and manage banned players.
                        </CardDescription>
                    </CardHeader>
                    <CardContent>
                        <BanTable
                            :bans="bannedPlayers"
                            :loading="loading"
                            :server-id="serverId as string"
                            :search="searchQuery"
                            :status="statusFilter"
                            :page-size="limit"
                            @update:search="searchQuery = $event"
                            @update:status="statusFilter = $event"
                            @update:page-size="changePageSize"
                            @edit="openEditBanDialog"
                            @remove="(ban) => removeBan(ban.id)"
                            @view="openEvidenceViewDialog"
                        />
                        <div
                            v-if="canGoPrevious || canGoNext"
                            class="mt-4 flex items-center justify-between border-t pt-4"
                        >
                            <span class="text-xs text-muted-foreground sm:text-sm">
                                Page {{ page }} · {{ bannedPlayers.length }} bans
                            </span>
                            <div class="flex gap-2">
                                <Button
                                    variant="outline"
                                    size="sm"
                                    :disabled="!canGoPrevious || loading"
                                    @click="previousPage"
                                >
                                    Previous
                                </Button>
                                <Button
                                    variant="outline"
                                    size="sm"
                                    :disabled="!canGoNext || loading"
                                    @click="nextPage"
                                >
                                    Next
                                </Button>
                            </div>
                        </div>
                    </CardContent>
                </Card>
            </TabsContent>

            <TabsContent value="subscriptions" class="space-y-3">
                <BanListSubscriptionsCard
                    :server-id="serverId as string"
                    @changed="resetBanList"
                />
                <p class="text-xs sm:text-sm text-muted-foreground">
                    Ban list subscriptions allow this server to automatically
                    include bans from other shared ban lists. Players banned on
                    subscribed lists will be automatically banned on this server
                    as well.
                </p>
            </TabsContent>
        </Tabs>

        <!-- Add/Edit Ban Dialog -->
        <BanFormDialog
            v-model:open="showBanFormDialog"
            :mode="banFormMode"
            :server-id="serverId as string"
            :ban="editingBan"
            @saved="resetBanList"
        />

        <!-- Import from Bans.cfg Dialog -->
        <BanImportDialog
            v-model:open="showImportDialog"
            :server-id="serverId as string"
            @imported="resetBanList"
        />

        <!-- Evidence View Dialog -->
        <BanViewDialog
            v-model:open="showEvidenceViewDialog"
            :ban="viewingBanEvidence"
        />

        <!-- Ban Config URL manual-copy fallback -->
        <Dialog v-model:open="showBanCfgDialog">
            <DialogContent class="sm:max-w-md">
                <DialogHeader>
                    <DialogTitle class="text-base">Ban Config URL</DialogTitle>
                    <DialogDescription class="text-xs sm:text-sm">
                        Automatic copying requires HTTPS or localhost. Please
                        copy the URL manually:
                    </DialogDescription>
                </DialogHeader>
                <Input
                    :value="banCfgUrl"
                    readonly
                    @focus="selectBanCfgUrl"
                    class="text-xs"
                />
            </DialogContent>
        </Dialog>
    </div>
</template>
