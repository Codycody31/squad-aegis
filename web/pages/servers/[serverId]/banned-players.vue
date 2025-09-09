<script setup lang="ts">
import { ref, onMounted, onUnmounted, computed } from "vue";
import { Button } from "~/components/ui/button";
import { Input } from "~/components/ui/input";
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from "~/components/ui/card";
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
import { Plus, Trash2 } from "lucide-vue-next";
import { useForm } from "vee-validate";
import { toTypedSchema } from "@vee-validate/zod";
import * as z from "zod";
import { toast } from "~/components/ui/toast";

const authStore = useAuthStore();
const runtimeConfig = useRuntimeConfig();
const route = useRoute();
const serverId = Array.isArray(route.params.serverId) 
  ? route.params.serverId[0] 
  : route.params.serverId;

const loading = ref(true);
const error = ref<string | null>(null);
const bannedPlayers = ref<BannedPlayer[]>([]);
const banLists = ref<any[]>([]);
const subscribedBanLists = ref<any[]>([]);
const availableBanLists = ref<any[]>([]);
const searchQuery = ref("");
const showAddBanDialog = ref(false);
const showBanListDialog = ref(false);
const addBanLoading = ref(false);
const selectedBanListId = ref("");
const subscribing = ref(false);
const unsubscribing = ref<string>("");

interface BannedPlayer {
    id: string;
    server_id: string;
    admin_id: string;
    admin_name: string;
    steam_id: string;
    name: string;
    reason: string;
    duration: number;
    permanent: boolean;
    expires_at: string;
    created_at: string;
    updated_at: string;
    ban_list_id?: string;
    ban_list_name?: string;
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
        player_name: z.string().optional(),
        reason: z.string().min(1, "Reason is required"),
        duration: z.number().min(0, "Duration must be at least 0"),
        ban_list_id: z.string().optional(),
    }),
);

// Setup form
const form = useForm({
    validationSchema: formSchema,
    initialValues: {
        steam_id: "",
        player_name: "",
        reason: "",
        duration: 24,
        ban_list_id: "",
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
                    new Date(b.created_at).getTime() -
                    new Date(a.created_at).getTime()
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
    const { steam_id, player_name, reason, duration, ban_list_id } = values;

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
        // Enhance the reason with player name if provided
        let enhancedReason = reason;
        if (player_name && player_name.trim()) {
            enhancedReason = `${player_name}: ${reason}`;
        }

        const requestBody: any = {
            steam_id,
            reason: enhancedReason,
            duration,
        };

        // Add ban_list_id if selected
        if (ban_list_id && ban_list_id.trim()) {
            requestBody.ban_list_id = ban_list_id;
        }

        const { data, error: fetchError } = await useFetch(
            `${runtimeConfig.public.backendApi}/servers/${serverId}/bans`,
            {
                method: "POST",
                body: requestBody,
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

        toast({
            title: "Success",
            description: "Ban added successfully",
        });

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

// Function to fetch ban lists
async function fetchBanLists() {
    const runtimeConfig = useRuntimeConfig();
    const cookieToken = useCookie(runtimeConfig.public.sessionCookieName as string);
    const token = cookieToken.value;

    if (!token) return;

    try {
        const response = await $fetch(`${runtimeConfig.public.backendApi}/ban-lists`, {
            headers: {
                Authorization: `Bearer ${token}`,
            },
        }) as any;

        if (response?.data?.ban_lists) {
            banLists.value = response.data.ban_lists;
        }
    } catch (err) {
        console.error("Failed to fetch ban lists:", err);
    }
}

// Function to fetch server's ban list subscriptions
async function fetchServerBanListSubscriptions() {
    const runtimeConfig = useRuntimeConfig();
    const cookieToken = useCookie(runtimeConfig.public.sessionCookieName as string);
    const token = cookieToken.value;

    if (!token) return;

    try {
        const response = await $fetch(
            `${runtimeConfig.public.backendApi}/servers/${serverId}/ban-list-subscriptions`,
            {
                headers: {
                    Authorization: `Bearer ${token}`,
                },
            }
        ) as any;

        if (response?.data?.subscriptions) {
            subscribedBanLists.value = response.data.subscriptions;
            // Calculate available ban lists (not subscribed)
            availableBanLists.value = banLists.value.filter(
                banList => !subscribedBanLists.value.some(sub => sub.ban_list_id === banList.id)
            );
        } else {
            subscribedBanLists.value = [];
            availableBanLists.value = banLists.value;
        }
    } catch (err) {
        console.error("Failed to fetch ban list subscriptions:", err);
    }
}

// Function to subscribe to a ban list
async function subscribeToBanList() {
    if (!selectedBanListId.value) return;

    const runtimeConfig = useRuntimeConfig();
    const cookieToken = useCookie(runtimeConfig.public.sessionCookieName as string);
    const token = cookieToken.value;

    if (!token) return;

    subscribing.value = true;
    
    try {
        await $fetch(
            `${runtimeConfig.public.backendApi}/servers/${serverId}/ban-list-subscriptions`,
            {
                method: "POST",
                headers: {
                    Authorization: `Bearer ${token}`,
                },
                body: {
                    ban_list_id: selectedBanListId.value,
                },
            }
        );

        selectedBanListId.value = "";
        await fetchServerBanListSubscriptions();
        await fetchBannedPlayers();
        
        toast({
            title: "Success",
            description: "Successfully subscribed to ban list. The server's ban configuration has been updated.",
        });
    } catch (err: any) {
        error.value = err.data?.message || "Failed to subscribe to ban list";
        console.error(err);
    } finally {
        subscribing.value = false;
    }
}

// Function to unsubscribe from a ban list
async function unsubscribeFromBanList(banListId: string) {
    if (!confirm("Are you sure you want to unsubscribe from this ban list?")) return;

    const runtimeConfig = useRuntimeConfig();
    const cookieToken = useCookie(runtimeConfig.public.sessionCookieName as string);
    const token = cookieToken.value;

    if (!token) return;

    unsubscribing.value = banListId;
    
    try {
        await $fetch(
            `${runtimeConfig.public.backendApi}/servers/${serverId}/ban-list-subscriptions/${banListId}`,
            {
                method: "DELETE",
                headers: {
                    Authorization: `Bearer ${token}`,
                },
            }
        );

        await fetchServerBanListSubscriptions();
        await fetchBannedPlayers();
        
        toast({
            title: "Success",
            description: "Ban list subscription removed. The server's ban configuration has been updated and will take effect on the next ban list refresh.",
        });
    } catch (err: any) {
        error.value = err.data?.message || "Failed to unsubscribe from ban list";
        console.error(err);
    } finally {
        unsubscribing.value = "";
    }
}

// Format date
function formatDate(dateString: string): string {
    return new Date(dateString).toLocaleString();
}

// Setup auto-refresh
onMounted(async () => {
    await fetchBanLists();
    await fetchServerBanListSubscriptions();
    await fetchBannedPlayers();
});

// Manual refresh function
async function refreshData() {
    await fetchBanLists();
    await fetchServerBanListSubscriptions();
    await fetchBannedPlayers();
}

function copyBanCfgUrl() {
    const url = `${runtimeConfig.public.backendApi}/servers/${serverId}/bans/cfg`;
    navigator.clipboard.writeText(url);
    
    toast({
        title: "Success",
        description: "Ban configuration URL copied to clipboard",
    });
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
                        player_name: '',
                        reason: '',
                        duration: 1,
                        ban_list_id: '',
                    }"
                >
                    <Dialog v-model:open="showAddBanDialog">
                        <DialogTrigger asChild>
                            <Button v-if="authStore.getServerPermission(serverId as string, 'ban')">Add Ban Manually</Button>
                        </DialogTrigger>
                        <DialogContent class="sm:max-w-[500px] max-h-[90vh] overflow-y-auto">
                            <DialogHeader>
                                <DialogTitle>Add New Ban</DialogTitle>
                                <DialogDescription>
                                    Enter the details of the player you want to ban. 
                                    You can optionally assign the ban to a shared ban list.
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
                                        name="player_name"
                                        v-slot="{ componentField }"
                                    >
                                        <FormItem>
                                            <FormLabel>Player Name (Optional)</FormLabel>
                                            <FormControl>
                                                <Input
                                                    placeholder="Player display name"
                                                    v-bind="componentField"
                                                />
                                            </FormControl>
                                            <FormDescription>
                                                If provided, will be included in the ban reason
                                            </FormDescription>
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
                                    <FormField
                                        name="ban_list_id"
                                        v-slot="{ componentField }"
                                    >
                                        <FormItem>
                                            <FormLabel>Ban List (Optional)</FormLabel>
                                            <FormControl>
                                                <Select v-bind="componentField">
                                                    <SelectTrigger>
                                                        <SelectValue placeholder="Select a ban list (optional)" />
                                                    </SelectTrigger>
                                                    <SelectContent>
                                                        <SelectItem
                                                            v-for="banList in banLists.filter(bl => !bl.is_remote)"
                                                            :key="banList.id"
                                                            :value="banList.id.toString()"
                                                        >
                                                            {{ banList.name }}
                                                        </SelectItem>
                                                    </SelectContent>
                                                </Select>
                                            </FormControl>
                                            <FormDescription>
                                                Select a ban list to add this ban to for sharing across servers
                                            </FormDescription>
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
                                <TableHead>Source</TableHead>
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
                                <TableCell>
                                    <Badge variant="secondary">
                                        {{ player.ban_list_name || "Manual" }}
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

        <!-- Ban List Subscriptions -->
        <Card class="mb-4">
            <CardHeader>
                <CardTitle>Ban List Subscriptions</CardTitle>
                <CardDescription>
                    Manage which ban lists this server subscribes to for automatic ban synchronization.
                </CardDescription>
            </CardHeader>
            <CardContent>
                <div class="space-y-4">
                    <!-- Add Ban List Subscription -->
                    <div class="flex gap-2 items-center">
                        <Select v-model="selectedBanListId">
                            <SelectTrigger class="w-64">
                                <SelectValue placeholder="Select a ban list to subscribe to" />
                            </SelectTrigger>
                            <SelectContent>
                                <SelectItem
                                    v-for="banList in availableBanLists"
                                    :key="banList.id"
                                    :value="banList.id.toString()"
                                >
                                    {{ banList.name }}
                                </SelectItem>
                            </SelectContent>
                        </Select>
                        <Button
                            @click="subscribeToBanList"
                            :disabled="!selectedBanListId || subscribing"
                        >
                            <Plus class="h-4 w-4 mr-2" />
                            {{ subscribing ? "Subscribing..." : "Subscribe" }}
                        </Button>
                    </div>

                    <!-- Current Subscriptions -->
                    <div v-if="subscribedBanLists.length > 0">
                        <h4 class="text-sm font-medium mb-2">Current Subscriptions</h4>
                        <div class="space-y-2">
                            <div
                                v-for="subscription in subscribedBanLists"
                                :key="subscription.ban_list_id"
                                class="flex items-center justify-between p-3 border rounded-lg"
                            >
                                <div>
                                    <div class="font-medium">{{ subscription.ban_list_name || 'Unknown Ban List' }}</div>
                                    <div class="text-sm text-gray-500">
                                        Subscribed on {{ new Date(subscription.created_at).toLocaleDateString() }}
                                    </div>
                                </div>
                                <Button
                                    variant="destructive"
                                    size="sm"
                                    @click="unsubscribeFromBanList(subscription.ban_list_id.toString())"
                                    :disabled="unsubscribing === subscription.ban_list_id.toString()"
                                >
                                    <Trash2 class="h-4 w-4 mr-2" />
                                    {{ unsubscribing === subscription.ban_list_id.toString() ? "Unsubscribing..." : "Unsubscribe" }}
                                </Button>
                            </div>
                        </div>
                    </div>

                    <div v-else class="text-center text-gray-500 py-4">
                        No ban list subscriptions configured
                    </div>
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
                <p class="text-sm text-muted-foreground mt-2">
                    Ban list subscriptions allow this server to automatically include bans from other shared ban lists.
                    Players banned on subscribed lists will be automatically banned on this server as well.
                </p>
                <p class="text-sm text-muted-foreground mt-2">
                    <strong>Note:</strong> Squad servers typically cache ban configurations and refresh them periodically. 
                    Changes to ban list subscriptions will be reflected in the ban configuration immediately, but may take 
                    some time to take effect in-game depending on your server's ban list refresh interval.
                </p>
            </CardContent>
        </Card>
    </div>
</template>

<style scoped>
/* Add any page-specific styles here */
</style>
