<script setup lang="ts">
import { ref, onMounted, onUnmounted, computed } from "vue";
import { Button } from "~/components/ui/button";
import { Input } from "~/components/ui/input";
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
import {
    Dialog,
    DialogContent,
    DialogDescription,
    DialogFooter,
    DialogHeader,
    DialogTitle,
    DialogTrigger,
} from "~/components/ui/dialog";
import {
    Form,
    FormControl,
    FormDescription,
    FormField,
    FormItem,
    FormLabel,
    FormMessage,
} from "~/components/ui/form";
import { Textarea } from "~/components/ui/textarea";
import {
    Select,
    SelectContent,
    SelectItem,
    SelectTrigger,
    SelectValue,
} from "~/components/ui/select";
import { useForm } from "vee-validate";
import { toTypedSchema } from "@vee-validate/zod";
import * as z from "zod";

const authStore = useAuthStore();
const runtimeConfig = useRuntimeConfig();
const route = useRoute();
const serverId = route.params.serverId;

const loading = ref(true);
const error = ref<string | null>(null);
const bannedPlayers = ref<BannedPlayer[]>([]);
const refreshInterval = ref<NodeJS.Timeout | null>(null);
const searchQuery = ref("");
const showAddBanDialog = ref(false);
const addBanLoading = ref(false);

interface BannedPlayer {
    id: string;
    server_id: string;
    admin_id: string;
    admin_name: string;
    player_id: string;
    steam_id: string;
    name: string;
    reason: string;
    duration: number;
    permanent: boolean;
    expires_at: string;
    created_at: string;
    updated_at: string;
}

interface BannedPlayersResponse {
    data: {
        bans: BannedPlayer[];
    };
}

// Form schema for adding a ban
const formSchema = toTypedSchema(
    z.object({
        steam_id: z
            .string()
            .min(17, "Steam ID must be at least 17 characters")
            .max(17, "Steam ID must be exactly 17 characters")
            .regex(/^\d+$/, "Steam ID must contain only numbers"),
        reason: z.string().min(1, "Reason is required"),
        duration: z.number().min(0, "Duration must be at least 0"),
    }),
);

// Setup form
const form = useForm({
    validationSchema: formSchema,
    initialValues: {
        steam_id: "",
        reason: "",
        duration: 24,
    },
});

// Computed property for filtered banned players
const filteredBannedPlayers = computed(() => {
    if (!searchQuery.value.trim()) {
        return bannedPlayers.value;
    }

    const query = searchQuery.value.toLowerCase();
    return bannedPlayers.value.filter(
        (player) =>
            player.name.toLowerCase().includes(query) ||
            player.steam_id.includes(query) ||
            player.reason.toLowerCase().includes(query),
    );
});

// Function to fetch banned players data
async function fetchBannedPlayers() {
    loading.value = true;
    error.value = null;

    const runtimeConfig = useRuntimeConfig();
    const cookieToken = useCookie(
        runtimeConfig.public.sessionCookieName as string,
    );
    const token = cookieToken.value;

    if (!token) {
        error.value = "Authentication required";
        loading.value = false;
        return;
    }

    try {
        const { data, error: fetchError } =
            await useFetch<BannedPlayersResponse>(
                `${runtimeConfig.public.backendApi}/servers/${serverId}/bans`,
                {
                    method: "GET",
                    headers: {
                        Authorization: `Bearer ${token}`,
                    },
                },
            );

        if (fetchError.value) {
            throw new Error(
                fetchError.value.message ||
                    "Failed to fetch banned players data",
            );
        }

        if (data.value && data.value.data) {
            bannedPlayers.value = data.value.data.bans || [];

            // Sort by ban date (most recent first)
            bannedPlayers.value.sort((a, b) => {
                return (
                    new Date(b.bannedAt).getTime() -
                    new Date(a.bannedAt).getTime()
                );
            });
        }
    } catch (err: any) {
        error.value =
            err.message ||
            "An error occurred while fetching banned players data";
        console.error(err);
    } finally {
        loading.value = false;
    }
}

// Function to add a ban
async function addBan(values: any) {
    const { steam_id, reason, duration } = values;

    addBanLoading.value = true;
    error.value = null;

    const runtimeConfig = useRuntimeConfig();
    const cookieToken = useCookie(
        runtimeConfig.public.sessionCookieName as string,
    );
    const token = cookieToken.value;

    if (!token) {
        error.value = "Authentication required";
        addBanLoading.value = false;
        return;
    }

    try {
        const { data, error: fetchError } = await useFetch(
            `${runtimeConfig.public.backendApi}/servers/${serverId}/bans`,
            {
                method: "POST",
                body: {
                    steam_id,
                    reason,
                    duration,
                },
                headers: {
                    Authorization: `Bearer ${token}`,
                },
            },
        );

        if (fetchError.value) {
            throw new Error(fetchError.value.message || "Failed to add ban");
        }

        // Reset form and close dialog
        form.resetForm();
        showAddBanDialog.value = false;

        // Refresh the banned players list
        fetchBannedPlayers();
    } catch (err: any) {
        error.value = err.message || "An error occurred while adding the ban";
        console.error(err);
    } finally {
        addBanLoading.value = false;
    }
}

// Function to remove a ban
async function removeBan(banId: string) {
    if (!confirm("Are you sure you want to remove this ban?")) {
        return;
    }

    loading.value = true;
    error.value = null;

    const runtimeConfig = useRuntimeConfig();
    const cookieToken = useCookie(
        runtimeConfig.public.sessionCookieName as string,
    );
    const token = cookieToken.value;

    if (!token) {
        error.value = "Authentication required";
        loading.value = false;
        return;
    }

    try {
        const { data, error: fetchError } = await useFetch(
            `${runtimeConfig.public.backendApi}/servers/${serverId}/bans/${banId}`,
            {
                method: "DELETE",
                headers: {
                    Authorization: `Bearer ${token}`,
                },
            },
        );

        if (fetchError.value) {
            throw new Error(fetchError.value.message || "Failed to remove ban");
        }

        // Refresh the banned players list
        fetchBannedPlayers();
    } catch (err: any) {
        error.value = err.message || "An error occurred while removing the ban";
        console.error(err);
    } finally {
        loading.value = false;
    }
}

// Format date
function formatDate(dateString: string): string {
    return new Date(dateString).toLocaleString();
}

// Setup auto-refresh
onMounted(() => {
    fetchBannedPlayers();

    // Refresh data every 60 seconds
    refreshInterval.value = setInterval(() => {
        fetchBannedPlayers();
    }, 60000);
});

// Clear interval on component unmount
onUnmounted(() => {
    if (refreshInterval.value) {
        clearInterval(refreshInterval.value);
    }
});

// Manual refresh function
function refreshData() {
    fetchBannedPlayers();
}

function copyBanCfgUrl() {
    navigator.clipboard.writeText(
        `${runtimeConfig.public.backendApi}/servers/${serverId}/bans/cfg`,
    );
}
</script>

<template>
    <div class="p-4">
        <div class="flex justify-between items-center mb-4">
            <h1 class="text-2xl font-bold">Banned Players</h1>
            <div class="flex gap-2">
                <Form
                    v-slot="{ handleSubmit }"
                    as=""
                    keep-values
                    :validation-schema="formSchema"
                    :initial-values="{
                        steam_id: '',
                        reason: '',
                        duration: 1,
                    }"
                >
                    <Dialog v-model:open="showAddBanDialog">
                        <DialogTrigger asChild>
                            <Button v-if="authStore.getServerPermissions(serverId as string).includes('ban')">Add Ban Manually</Button>
                        </DialogTrigger>
                        <DialogContent class="sm:max-w-[425px]">
                            <DialogHeader>
                                <DialogTitle>Add New Ban</DialogTitle>
                                <DialogDescription>
                                    Enter the details of the player you want to
                                    ban.
                                </DialogDescription>
                            </DialogHeader>
                            <form
                                id="dialogForm"
                                @submit="handleSubmit($event, addBan)"
                            >
                                <div class="grid gap-4 py-4">
                                    <FormField
                                        name="steam_id"
                                        v-slot="{ componentField }"
                                    >
                                        <FormItem>
                                            <FormLabel>Steam ID</FormLabel>
                                            <FormControl>
                                                <Input
                                                    placeholder="76561198012345678"
                                                    v-bind="componentField"
                                                />
                                            </FormControl>
                                            <FormMessage />
                                        </FormItem>
                                    </FormField>
                                    <FormField
                                        name="reason"
                                        v-slot="{ componentField }"
                                    >
                                        <FormItem>
                                            <FormLabel>Reason</FormLabel>
                                            <FormControl>
                                                <Textarea
                                                    placeholder="Reason for ban"
                                                    v-bind="componentField"
                                                />
                                            </FormControl>
                                            <FormMessage />
                                        </FormItem>
                                    </FormField>
                                    <FormField
                                        name="duration"
                                        v-slot="{ componentField }"
                                    >
                                        <FormItem>
                                            <FormLabel>Days</FormLabel>
                                            <FormControl>
                                                <Input
                                                    type="number"
                                                    min="0"
                                                    v-bind="componentField"
                                                />
                                            </FormControl>
                                            <FormDescription
                                                >Duration in days. 0 is
                                                permanent</FormDescription
                                            >
                                            <FormMessage />
                                        </FormItem>
                                    </FormField>
                                </div>
                                <DialogFooter>
                                    <Button
                                        type="button"
                                        variant="outline"
                                        @click="showAddBanDialog = false"
                                    >
                                        Cancel
                                    </Button>
                                    <Button
                                        type="submit"
                                        :disabled="addBanLoading"
                                    >
                                        {{
                                            addBanLoading
                                                ? "Adding..."
                                                : "Add Ban"
                                        }}
                                    </Button>
                                </DialogFooter>
                            </form>
                        </DialogContent>
                    </Dialog>
                </Form>
                <Button
                    @click="refreshData"
                    :disabled="loading"
                    variant="outline"
                >
                    {{ loading ? "Refreshing..." : "Refresh" }}
                </Button>
                <Button @click="copyBanCfgUrl">Copy Ban Config URL</Button>
            </div>
        </div>

        <div v-if="error" class="bg-red-500 text-white p-4 rounded mb-4">
            {{ error }}
        </div>

        <Card class="mb-4">
            <CardHeader class="pb-2">
                <CardTitle>Ban List</CardTitle>
                <p class="text-sm text-muted-foreground">
                    View and manage banned players. Data refreshes automatically
                    every 60 seconds.
                </p>
            </CardHeader>
            <CardContent>
                <div class="flex items-center space-x-2 mb-4">
                    <Input
                        v-model="searchQuery"
                        placeholder="Search by Steam ID, or reason..."
                        class="flex-grow"
                    />
                </div>

                <div class="text-sm text-muted-foreground mb-2">
                    Showing {{ filteredBannedPlayers.length }} of
                    {{ bannedPlayers.length }} bans
                </div>

                <div
                    v-if="loading && bannedPlayers.length === 0"
                    class="text-center py-8"
                >
                    <div
                        class="animate-spin h-8 w-8 border-4 border-primary border-t-transparent rounded-full mx-auto mb-4"
                    ></div>
                    <p>Loading banned players...</p>
                </div>

                <div
                    v-else-if="bannedPlayers.length === 0"
                    class="text-center py-8"
                >
                    <p>No banned players found</p>
                </div>

                <div
                    v-else-if="filteredBannedPlayers.length === 0"
                    class="text-center py-8"
                >
                    <p>No players match your search</p>
                </div>

                <div v-else class="overflow-x-auto">
                    <Table>
                        <TableHeader>
                            <TableRow>
                                <TableHead>Steam ID</TableHead>
                                <TableHead>Reason</TableHead>
                                <TableHead>Banned At</TableHead>
                                <TableHead>Duration</TableHead>
                                <TableHead class="text-right"
                                    >Actions</TableHead
                                >
                            </TableRow>
                        </TableHeader>
                        <TableBody>
                            <TableRow
                                v-for="player in filteredBannedPlayers"
                                :key="player.id"
                                class="hover:bg-muted/50"
                            >
                                <TableCell>
                                    <div class="font-medium">
                                        {{ player.steam_id }}
                                    </div>
                                </TableCell>
                                <TableCell>{{ player.reason }}</TableCell>
                                <TableCell>{{
                                    formatDate(player.created_at)
                                }}</TableCell>
                                <TableCell>
                                    <Badge
                                        :variant="
                                            player.duration == 0
                                                ? 'destructive'
                                                : 'outline'
                                        "
                                    >
                                        {{
                                            player.duration == 0
                                                ? "Permanent"
                                                : player.duration + " days"
                                        }}
                                    </Badge>
                                </TableCell>
                                <TableCell class="text-right">
                                    <Button
                                        variant="destructive"
                                        size="sm"
                                        @click="removeBan(player.id)"
                                        :disabled="loading"
                                        v-if="authStore.getServerPermission(serverId, 'ban')"
                                    >
                                        Unban
                                    </Button>
                                </TableCell>
                            </TableRow>
                        </TableBody>
                    </Table>
                </div>
            </CardContent>
        </Card>

        <Card>
            <CardHeader>
                <CardTitle>About Bans</CardTitle>
            </CardHeader>
            <CardContent>
                <p class="text-sm text-muted-foreground">
                    This page shows players who have been banned from the
                    server. You can add new bans manually or remove existing
                    bans.
                </p>
                <p class="text-sm text-muted-foreground mt-2">
                    Permanent bans will remain in effect until manually removed.
                    Temporary bans will expire after the specified duration.
                </p>
            </CardContent>
        </Card>
    </div>
</template>

<style scoped>
/* Add any page-specific styles here */
</style>
