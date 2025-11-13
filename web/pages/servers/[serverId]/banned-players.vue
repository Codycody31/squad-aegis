<script setup lang="ts">
import { ref, onMounted, onUnmounted, computed } from "vue";
import { Button } from "~/components/ui/button";
import { Input } from "~/components/ui/input";
import {
    Card,
    CardContent,
    CardHeader,
    CardTitle,
    CardDescription,
} from "~/components/ui/card";
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
const serverRules = ref<any[]>([]);
const playerHistory = ref<any[]>([]);
const suggestedDuration = ref<number>(24);
const isLoadingHistory = ref(false);
const searchQuery = ref("");
const showAddBanDialog = ref(false);
const showEditBanDialog = ref(false);
const showBanListDialog = ref(false);
const addBanLoading = ref(false);
const editBanLoading = ref(false);
const selectedBanListId = ref("");
const subscribing = ref(false);
const unsubscribing = ref<string>("");
const editingBan = ref<BannedPlayer | null>(null);

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
    rule_id?: string;
    rule_name?: string;
    rule_number?: string;
    player_name?: string;
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
        rule_id: z.string().optional(),
    }),
);

// Form schema for editing a ban
const editFormSchema = toTypedSchema(
    z.object({
        reason: z.string().min(1, "Reason is required"),
        duration: z.number().min(0, "Duration must be at least 0"),
        ban_list_id: z.string().optional(),
        rule_id: z.string().optional(),
    }),
);

// Setup forms
const form = useForm({
    validationSchema: formSchema,
    initialValues: {
        steam_id: "",
        player_name: "",
        reason: "",
        duration: 24,
        ban_list_id: "",
        rule_id: "",
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
    const { steam_id, player_name, reason, duration, ban_list_id, rule_id } =
        values;

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

        // Add rule_id if selected
        if (rule_id && rule_id.trim()) {
            requestBody.rule_id = rule_id;
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

// Function to edit a ban
async function editBan(values: any) {
    const { reason, duration, ban_list_id, rule_id } = values;

    if (!editingBan.value) {
        error.value = "No ban selected for editing";
        return;
    }

    editBanLoading.value = true;
    error.value = null;

    const runtimeConfig = useRuntimeConfig();
    const cookieToken = useCookie(
        runtimeConfig.public.sessionCookieName as string,
    );
    const token = cookieToken.value;

    if (!token) {
        error.value = "Authentication required";
        editBanLoading.value = false;
        return;
    }

    try {
        const requestBody: any = {};

        // Only include fields that were actually changed
        if (reason !== editingBan.value.reason) {
            requestBody.reason = reason;
        }

        if (duration !== editingBan.value.duration) {
            requestBody.duration = duration;
        }

        // Handle ban list changes
        const currentBanListId = editingBan.value.ban_list_id || "";
        const newBanListId = ban_list_id || "";

        if (currentBanListId !== newBanListId) {
            requestBody.ban_list_id = newBanListId || null;
        }

        // Handle rule changes
        const currentRuleId = editingBan.value.rule_id || "";
        const newRuleId = rule_id || "";

        if (currentRuleId !== newRuleId) {
            requestBody.rule_id = newRuleId || null;
        }

        // If no changes were made, just close the dialog
        if (Object.keys(requestBody).length === 0) {
            closeEditBanDialog();
            return;
        }

        const { data, error: fetchError } = await useFetch(
            `${runtimeConfig.public.backendApi}/servers/${serverId}/bans/${editingBan.value.id}`,
            {
                method: "PUT",
                body: requestBody,
                headers: {
                    Authorization: `Bearer ${token}`,
                },
            },
        );

        if (fetchError.value) {
            throw new Error(fetchError.value.message || "Failed to update ban");
        }

        // Reset form and close dialog
        closeEditBanDialog();

        toast({
            title: "Success",
            description: "Ban updated successfully",
        });

        // Refresh the banned players list
        fetchBannedPlayers();
    } catch (err: any) {
        error.value = err.message || "An error occurred while updating the ban";
        console.error(err);
    } finally {
        editBanLoading.value = false;
    }
}

// Function to get rule details by ID
function getRuleDetails(ruleId: string) {
    const rule = serverRules.value.find((r) => r.id === ruleId);
    return rule || null;
}

// Function to open edit ban dialog
async function openEditBanDialog(ban: BannedPlayer) {
    editingBan.value = ban;

    // Ensure rules are loaded
    if (serverRules.value.length === 0) {
        await fetchServerRules();
    }

    // If the ban has a rule_id but no rule_name/number, try to fetch the details
    if (ban.rule_id && (!ban.rule_name || !ban.rule_number)) {
        const ruleDetails = getRuleDetails(ban.rule_id);
        if (ruleDetails) {
            // Update the ban object with rule details
            editingBan.value = {
                ...ban,
                rule_name: ruleDetails.title,
                rule_number: ruleDetails.number,
            };
        }
    }

    showEditBanDialog.value = true;
}

// Function to close edit ban dialog and reset state
function closeEditBanDialog() {
    showEditBanDialog.value = false;
    editingBan.value = null;
}

// Function to fetch ban lists
async function fetchBanLists() {
    const runtimeConfig = useRuntimeConfig();
    const cookieToken = useCookie(
        runtimeConfig.public.sessionCookieName as string,
    );
    const token = cookieToken.value;

    if (!token) return;

    try {
        const response = (await $fetch(
            `${runtimeConfig.public.backendApi}/ban-lists`,
            {
                headers: {
                    Authorization: `Bearer ${token}`,
                },
            },
        )) as any;

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
    const cookieToken = useCookie(
        runtimeConfig.public.sessionCookieName as string,
    );
    const token = cookieToken.value;

    if (!token) return;

    try {
        const response = (await $fetch(
            `${runtimeConfig.public.backendApi}/servers/${serverId}/ban-list-subscriptions`,
            {
                headers: {
                    Authorization: `Bearer ${token}`,
                },
            },
        )) as any;

        if (response?.data?.subscriptions) {
            subscribedBanLists.value = response.data.subscriptions;
            // Calculate available ban lists (not subscribed)
            availableBanLists.value = banLists.value.filter(
                (banList) =>
                    !subscribedBanLists.value.some(
                        (sub) => sub.ban_list_id === banList.id,
                    ),
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
    const cookieToken = useCookie(
        runtimeConfig.public.sessionCookieName as string,
    );
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
            },
        );

        selectedBanListId.value = "";
        await fetchServerBanListSubscriptions();
        await fetchBannedPlayers();

        toast({
            title: "Success",
            description:
                "Successfully subscribed to ban list. The server's ban configuration has been updated.",
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
    if (!confirm("Are you sure you want to unsubscribe from this ban list?"))
        return;

    const runtimeConfig = useRuntimeConfig();
    const cookieToken = useCookie(
        runtimeConfig.public.sessionCookieName as string,
    );
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
            },
        );

        await fetchServerBanListSubscriptions();
        await fetchBannedPlayers();

        toast({
            title: "Success",
            description:
                "Ban list subscription removed. The server's ban configuration has been updated and will take effect on the next ban list refresh.",
        });
    } catch (err: any) {
        error.value =
            err.data?.message || "Failed to unsubscribe from ban list";
        console.error(err);
    } finally {
        unsubscribing.value = "";
    }
}

// Format date
function formatDate(dateString: string): string {
    return new Date(dateString).toLocaleString();
}

// Function to fetch server rules
async function fetchServerRules() {
    const runtimeConfig = useRuntimeConfig();
    const cookieToken = useCookie(
        runtimeConfig.public.sessionCookieName as string,
    );
    const token = cookieToken.value;

    if (!token) return;

    try {
        const { data, error: fetchError } = await useFetch(
            `${runtimeConfig.public.backendApi}/servers/${serverId}/rules`,
            {
                method: "GET",
                headers: {
                    Authorization: `Bearer ${token}`,
                },
            },
        );

        if (fetchError.value) {
            throw new Error(
                fetchError.value.message || "Failed to fetch server rules",
            );
        }

        if (data.value) {
            // Flatten the rules hierarchy for easier selection in dropdown
            serverRules.value = flattenRulesForDropdown(
                Array.isArray(data.value) ? data.value : [],
            );
        }
    } catch (err: any) {
        console.error("Failed to fetch server rules:", err);
    }
}

// Helper function to flatten rules hierarchy for dropdown
function flattenRulesForDropdown(
    rules: any[],
    parentTitle = "",
    parentNumber = "",
    result: any[] = [],
) {
    rules.forEach((rule, index) => {
        // Create a formatted title that shows the hierarchy
        const formattedTitle = parentTitle
            ? `${parentTitle} > ${rule.title}`
            : rule.title;

        // Create a rule number (e.g., "1.2.3")
        const ruleNumber = parentNumber
            ? `${parentNumber}.${index + 1}`
            : `${index + 1}`;

        result.push({
            id: rule.id,
            title: formattedTitle,
            description: rule.description,
            number: ruleNumber,
        });

        // Process sub-rules if they exist
        if (rule.sub_rules && rule.sub_rules.length > 0) {
            flattenRulesForDropdown(
                rule.sub_rules,
                formattedTitle,
                ruleNumber,
                result,
            );
        }
    });

    return result;
}

// Function to fetch player ban history
async function fetchPlayerBanHistory(steamId: string) {
    if (!steamId || steamId.length !== 17) {
        playerHistory.value = [];
        return;
    }

    isLoadingHistory.value = true;

    const runtimeConfig = useRuntimeConfig();
    const cookieToken = useCookie(
        runtimeConfig.public.sessionCookieName as string,
    );
    const token = cookieToken.value;

    if (!token) {
        isLoadingHistory.value = false;
        return;
    }

    try {
        const { data, error: fetchError } = await useFetch<any>(
            `${runtimeConfig.public.backendApi}/players/${steamId}/ban-history`,
            {
                method: "GET",
                headers: {
                    Authorization: `Bearer ${token}`,
                },
            },
        );

        if (fetchError.value) {
            throw new Error(
                fetchError.value.message || "Failed to fetch player history",
            );
        }

        if (data.value && data.value.data) {
            playerHistory.value = data.value.data.history || [];
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

// Setup auto-refresh
onMounted(async () => {
    await fetchBanLists();
    await fetchServerBanListSubscriptions();
    await fetchBannedPlayers();
    await fetchServerRules();
});

// Manual refresh function
async function refreshData() {
    await fetchBanLists();
    await fetchServerBanListSubscriptions();
    await fetchBannedPlayers();
    await fetchServerRules();
}

function copyBanCfgUrl() {
    var url = "";
    if (runtimeConfig.public.backendApi.startsWith("/")) {
        // Relative URL, construct full URL
        const origin = window.location.origin;
        url = `${origin}${runtimeConfig.public.backendApi}/servers/${serverId}/bans/cfg`;
    } else {
        url = `${runtimeConfig.public.backendApi}/servers/${serverId}/bans/cfg`;
    }
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
                        rule_id: '',
                    }"
                >
                    <Dialog v-model:open="showAddBanDialog">
                        <DialogTrigger asChild>
                            <Button
                                v-if="
                                    authStore.getServerPermission(
                                        serverId as string,
                                        'ban',
                                    )
                                "
                                >Add Ban Manually</Button
                            >
                        </DialogTrigger>
                        <DialogContent
                            class="sm:max-w-[500px] max-h-[90vh] overflow-y-auto"
                        >
                            <DialogHeader>
                                <DialogTitle>Add New Ban</DialogTitle>
                                <DialogDescription>
                                    Enter the details of the player you want to
                                    ban. You can optionally assign the ban to a
                                    shared ban list.
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
                                                    @input="
                                                        (e: Event) => {
                                                            const target =
                                                                e.target as HTMLInputElement;
                                                            if (
                                                                target.value
                                                                    .length ===
                                                                17
                                                            ) {
                                                                fetchPlayerBanHistory(
                                                                    target.value,
                                                                );
                                                            }
                                                        }
                                                    "
                                                />
                                            </FormControl>
                                            <FormMessage />
                                        </FormItem>
                                    </FormField>

                                    <!-- Player History Loading Indicator -->
                                    <div
                                        v-if="isLoadingHistory"
                                        class="border rounded-md p-3 bg-muted/50 flex items-center"
                                    >
                                        <div
                                            class="animate-spin h-5 w-5 border-2 border-primary border-t-transparent rounded-full mr-2"
                                        ></div>
                                        <span class="text-sm"
                                            >Looking up player history...</span
                                        >
                                    </div>

                                    <!-- Player Ban History Card -->
                                    <div
                                        v-else-if="playerHistory.length > 0"
                                        class="border rounded-md p-3 bg-muted/50"
                                    >
                                        <h4
                                            class="font-medium mb-2 flex items-center"
                                        >
                                            <span class="text-orange-500 mr-1"
                                                >⚠️</span
                                            >
                                            Previous Ban History
                                        </h4>
                                        <p
                                            class="text-sm text-muted-foreground mb-2"
                                        >
                                            This player has been banned
                                            {{ playerHistory.length }} time{{
                                                playerHistory.length > 1
                                                    ? "s"
                                                    : ""
                                            }}
                                            before.
                                        </p>
                                        <ul class="text-sm space-y-1 mb-2">
                                            <li
                                                v-for="(
                                                    ban, index
                                                ) in playerHistory.slice(0, 3)"
                                                :key="index"
                                            >
                                                <span
                                                    class="text-muted-foreground"
                                                    >{{
                                                        new Date(
                                                            ban.created_at,
                                                        ).toLocaleDateString()
                                                    }}:</span
                                                >
                                                {{ ban.reason }}
                                            </li>
                                        </ul>
                                        <div
                                            class="flex items-center border-t pt-2 mt-2"
                                        >
                                            <span class="mr-2 text-sm"
                                                >Suggested ban:</span
                                            >
                                            <Badge
                                                variant="destructive"
                                                v-if="suggestedDuration === 0"
                                            >
                                                Permanent
                                            </Badge>
                                            <Badge variant="default" v-else>
                                                {{ suggestedDuration }}
                                                {{
                                                    suggestedDuration === 1
                                                        ? "day"
                                                        : "days"
                                                }}
                                            </Badge>
                                        </div>
                                    </div>

                                    <FormField
                                        name="player_name"
                                        v-slot="{ componentField }"
                                    >
                                        <FormItem>
                                            <FormLabel
                                                >Player Name
                                                (Optional)</FormLabel
                                            >
                                            <FormControl>
                                                <Input
                                                    placeholder="Player display name"
                                                    v-bind="componentField"
                                                />
                                            </FormControl>
                                            <FormDescription>
                                                If provided, will be included in
                                                the ban reason
                                            </FormDescription>
                                            <FormMessage />
                                        </FormItem>
                                    </FormField>

                                    <FormField
                                        name="rule_id"
                                        v-slot="{ componentField }"
                                    >
                                        <FormItem>
                                            <FormLabel
                                                >Rule Violated
                                                (Optional)</FormLabel
                                            >
                                            <FormControl>
                                                <Select v-bind="componentField">
                                                    <SelectTrigger>
                                                        <SelectValue
                                                            placeholder="Select the rule that was violated"
                                                        />
                                                    </SelectTrigger>
                                                    <SelectContent>
                                                        <SelectItem
                                                            v-for="rule in serverRules"
                                                            :key="rule.id"
                                                            :value="rule.id"
                                                        >
                                                            {{ rule.title }}
                                                        </SelectItem>
                                                    </SelectContent>
                                                </Select>
                                            </FormControl>
                                            <FormDescription>
                                                Select which server rule was
                                                violated
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
                                        v-slot="{ componentField, setValue }"
                                    >
                                        <FormItem>
                                            <FormLabel>Days</FormLabel>
                                            <div
                                                class="flex items-center space-x-2"
                                            >
                                                <FormControl>
                                                    <Input
                                                        type="number"
                                                        min="0"
                                                        v-bind="componentField"
                                                    />
                                                </FormControl>
                                                <Button
                                                    type="button"
                                                    variant="outline"
                                                    size="sm"
                                                    @click="
                                                        setValue(
                                                            suggestedDuration,
                                                        )
                                                    "
                                                    v-if="
                                                        playerHistory.length > 0
                                                    "
                                                >
                                                    Use Suggested
                                                </Button>
                                            </div>
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
                                            <FormLabel
                                                >Ban List (Optional)</FormLabel
                                            >
                                            <FormControl>
                                                <Select v-bind="componentField">
                                                    <SelectTrigger>
                                                        <SelectValue
                                                            placeholder="Select a ban list (optional)"
                                                        />
                                                    </SelectTrigger>
                                                    <SelectContent>
                                                        <SelectItem
                                                            v-for="banList in banLists.filter(
                                                                (bl) =>
                                                                    !bl.is_remote,
                                                            )"
                                                            :key="banList.id"
                                                            :value="
                                                                banList.id.toString()
                                                            "
                                                        >
                                                            {{ banList.name }}
                                                        </SelectItem>
                                                    </SelectContent>
                                                </Select>
                                            </FormControl>
                                            <FormDescription>
                                                Select a ban list to add this
                                                ban to for sharing across
                                                servers
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

                <!-- Edit Ban Dialog -->
                <Form
                    :key="editingBan?.id"
                    v-slot="{ handleSubmit }"
                    as=""
                    keep-values
                    :validation-schema="editFormSchema"
                    :initial-values="{
                        reason: editingBan?.reason || '',
                        duration: editingBan?.duration,
                        ban_list_id: editingBan?.ban_list_id || '',
                        rule_id: editingBan?.rule_id || '',
                    }"
                >
                    <Dialog v-model:open="showEditBanDialog">
                        <DialogContent
                            class="sm:max-w-[500px] max-h-[90vh] overflow-y-auto"
                        >
                            <DialogHeader>
                                <DialogTitle>Edit Ban</DialogTitle>
                                <DialogDescription>
                                    Update the ban details for player
                                    {{ editingBan?.steam_id }}.
                                </DialogDescription>
                            </DialogHeader>
                            <form
                                id="editDialogForm"
                                @submit="handleSubmit($event, editBan)"
                            >
                                <div class="grid gap-4 py-4">
                                    <!-- Display player information if available -->
                                    <div
                                        v-if="editingBan?.player_name"
                                        class="border rounded-md p-3 bg-muted/50"
                                    >
                                        <h4 class="font-medium mb-2">
                                            Player Information
                                        </h4>
                                        <div class="text-sm">
                                            <p>
                                                <span
                                                    class="text-muted-foreground"
                                                    >Name:</span
                                                >
                                                {{ editingBan.player_name }}
                                            </p>
                                            <p>
                                                <span
                                                    class="text-muted-foreground"
                                                    >Steam ID:</span
                                                >
                                                {{ editingBan.steam_id }}
                                            </p>
                                        </div>
                                    </div>

                                    <!-- Rule selector -->
                                    <FormField
                                        name="rule_id"
                                        v-slot="{ componentField }"
                                    >
                                        <FormItem>
                                            <FormLabel
                                                >Rule Violated
                                                (Optional)</FormLabel
                                            >
                                            <FormControl>
                                                <Select v-bind="componentField">
                                                    <SelectTrigger>
                                                        <SelectValue
                                                            placeholder="Select the rule that was violated"
                                                        />
                                                    </SelectTrigger>
                                                    <SelectContent>
                                                        <SelectItem
                                                            v-for="rule in serverRules"
                                                            :key="rule.id"
                                                            :value="rule.id"
                                                        >
                                                            {{ rule.title }}
                                                        </SelectItem>
                                                    </SelectContent>
                                                </Select>
                                            </FormControl>
                                            <FormDescription>
                                                Select which server rule was
                                                violated
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
                                            <FormDescription>
                                                Duration in days. 0 is permanent
                                            </FormDescription>
                                            <FormMessage />
                                        </FormItem>
                                    </FormField>
                                    <FormField
                                        name="ban_list_id"
                                        v-slot="{ componentField }"
                                    >
                                        <FormItem>
                                            <FormLabel
                                                >Ban List (Optional)</FormLabel
                                            >
                                            <FormControl>
                                                <Select v-bind="componentField">
                                                    <SelectTrigger>
                                                        <SelectValue
                                                            placeholder="Select a ban list (optional)"
                                                        />
                                                    </SelectTrigger>
                                                    <SelectContent>
                                                        <SelectItem
                                                            v-for="banList in banLists.filter(
                                                                (bl) =>
                                                                    !bl.is_remote,
                                                            )"
                                                            :key="banList.id"
                                                            :value="
                                                                banList.id.toString()
                                                            "
                                                        >
                                                            {{ banList.name }}
                                                        </SelectItem>
                                                    </SelectContent>
                                                </Select>
                                            </FormControl>
                                            <FormDescription>
                                                Select a ban list to add this
                                                ban to for sharing across
                                                servers
                                            </FormDescription>
                                            <FormMessage />
                                        </FormItem>
                                    </FormField>
                                </div>
                                <DialogFooter>
                                    <Button
                                        type="button"
                                        variant="outline"
                                        @click="closeEditBanDialog"
                                    >
                                        Cancel
                                    </Button>
                                    <Button
                                        type="submit"
                                        :disabled="editBanLoading"
                                    >
                                        {{
                                            editBanLoading
                                                ? "Updating..."
                                                : "Update Ban"
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
                                <TableHead>Rule</TableHead>
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
                                    <div
                                        v-if="player.player_name"
                                        class="text-xs text-muted-foreground"
                                    >
                                        {{ player.player_name }}
                                    </div>
                                </TableCell>
                                <TableCell>{{ player.reason }}</TableCell>
                                <TableCell>
                                    <div
                                        v-if="player.rule_name"
                                        class="flex flex-col"
                                    >
                                        <span class="font-medium">{{
                                            player.rule_name
                                        }}</span>
                                    </div>
                                    <span
                                        v-else
                                        class="text-muted-foreground text-xs"
                                        >No rule specified</span
                                    >
                                </TableCell>
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
                                    <div class="flex gap-2 justify-end">
                                        <Button
                                            variant="outline"
                                            size="sm"
                                            @click="openEditBanDialog(player)"
                                            :disabled="loading"
                                            v-if="
                                                authStore.getServerPermission(
                                                    serverId,
                                                    'ban',
                                                )
                                            "
                                        >
                                            Edit
                                        </Button>
                                        <Button
                                            variant="destructive"
                                            size="sm"
                                            @click="removeBan(player.id)"
                                            :disabled="loading"
                                            v-if="
                                                authStore.getServerPermission(
                                                    serverId,
                                                    'ban',
                                                )
                                            "
                                        >
                                            Unban
                                        </Button>
                                    </div>
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
                    Manage which ban lists this server subscribes to for
                    automatic ban synchronization.
                </CardDescription>
            </CardHeader>
            <CardContent>
                <div class="space-y-4">
                    <!-- Add Ban List Subscription -->
                    <div class="flex gap-2 items-center">
                        <Select v-model="selectedBanListId">
                            <SelectTrigger class="w-64">
                                <SelectValue
                                    placeholder="Select a ban list to subscribe to"
                                />
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
                        <h4 class="text-sm font-medium mb-2">
                            Current Subscriptions
                        </h4>
                        <div class="space-y-2">
                            <div
                                v-for="subscription in subscribedBanLists"
                                :key="subscription.ban_list_id"
                                class="flex items-center justify-between p-3 border rounded-lg"
                            >
                                <div>
                                    <div class="font-medium">
                                        {{
                                            subscription.ban_list_name ||
                                            "Unknown Ban List"
                                        }}
                                    </div>
                                    <div class="text-sm text-gray-500">
                                        Subscribed on
                                        {{
                                            new Date(
                                                subscription.created_at,
                                            ).toLocaleDateString()
                                        }}
                                    </div>
                                </div>
                                <Button
                                    variant="destructive"
                                    size="sm"
                                    @click="
                                        unsubscribeFromBanList(
                                            subscription.ban_list_id.toString(),
                                        )
                                    "
                                    :disabled="
                                        unsubscribing ===
                                        subscription.ban_list_id.toString()
                                    "
                                >
                                    <Trash2 class="h-4 w-4 mr-2" />
                                    {{
                                        unsubscribing ===
                                        subscription.ban_list_id.toString()
                                            ? "Unsubscribing..."
                                            : "Unsubscribe"
                                    }}
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
                    Ban list subscriptions allow this server to automatically
                    include bans from other shared ban lists. Players banned on
                    subscribed lists will be automatically banned on this server
                    as well.
                </p>
                <p class="text-sm text-muted-foreground mt-2">
                    <strong>Note:</strong> Squad servers typically cache ban
                    configurations and refresh them periodically. Changes to ban
                    list subscriptions will be reflected in the ban
                    configuration immediately, but may take some time to take
                    effect in-game depending on your server's ban list refresh
                    interval.
                </p>
            </CardContent>
        </Card>
    </div>
</template>

<style scoped>
/* Add any page-specific styles here */
</style>
