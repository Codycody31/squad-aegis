<script setup lang="ts">
import { ref, onMounted } from "vue";
import { Button } from "~/components/ui/button";
import {
    Card,
    CardContent,
    CardDescription,
    CardHeader,
    CardTitle,
} from "~/components/ui/card";
import {
    Select,
    SelectContent,
    SelectItem,
    SelectTrigger,
    SelectValue,
} from "~/components/ui/select";
import { Plus, Trash2 } from "lucide-vue-next";
import { toast } from "~/components/ui/toast";
import type { BanList, BanListSubscription } from "~/types/bans";

// Self-contained ban list subscriptions management: fetches the available ban
// lists and this server's subscriptions, and handles subscribe/unsubscribe.
// Emits `changed` after either so the page can refresh the ban list.

const props = defineProps<{
    serverId: string;
}>();

const emit = defineEmits<{
    (e: "changed"): void;
}>();

const banLists = ref<BanList[]>([]);
const subscribedBanLists = ref<BanListSubscription[]>([]);
const availableBanLists = ref<BanList[]>([]);
const selectedBanListId = ref("");
const subscribing = ref(false);
const unsubscribing = ref<string>("");
const error = ref<string | null>(null);

// Function to fetch ban lists
async function fetchBanLists() {
    const runtimeConfig = useRuntimeConfig();

    try {
        const response = (await useAuthFetchImperative(
            `${runtimeConfig.public.backendApi}/ban-lists`,
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

    try {
        const response = (await useAuthFetchImperative(
            `${runtimeConfig.public.backendApi}/servers/${props.serverId}/ban-list-subscriptions`,
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

    subscribing.value = true;

    try {
        await useAuthFetchImperative(
            `${runtimeConfig.public.backendApi}/servers/${props.serverId}/ban-list-subscriptions`,
            {
                method: "POST",
                body: {
                    ban_list_id: selectedBanListId.value,
                },
            },
        );

        selectedBanListId.value = "";
        await fetchServerBanListSubscriptions();
        emit("changed");

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

    unsubscribing.value = banListId;

    try {
        await useAuthFetchImperative(
            `${runtimeConfig.public.backendApi}/servers/${props.serverId}/ban-list-subscriptions/${banListId}`,
            {
                method: "DELETE",
            },
        );

        await fetchServerBanListSubscriptions();
        emit("changed");

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

onMounted(async () => {
    await fetchBanLists();
    await fetchServerBanListSubscriptions();
});
</script>

<template>
    <Card>
        <CardHeader>
            <CardTitle class="text-base sm:text-lg">Ban List Subscriptions</CardTitle>
            <CardDescription class="text-xs sm:text-sm">
                Manage which ban lists this server subscribes to for
                automatic ban synchronization.
            </CardDescription>
        </CardHeader>
        <CardContent>
            <div
                v-if="error"
                class="bg-red-500 text-white p-3 rounded mb-3 sm:mb-4 text-sm"
            >
                {{ error }}
            </div>

            <div class="space-y-3 sm:space-y-4">
                <!-- Add Ban List Subscription -->
                <div class="flex flex-col sm:flex-row gap-2 items-stretch sm:items-center">
                    <Select v-model="selectedBanListId">
                        <SelectTrigger class="w-full sm:w-64 text-sm sm:text-base">
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
                        class="w-full sm:w-auto text-sm sm:text-base"
                    >
                        <Plus class="h-4 w-4 mr-2" />
                        {{ subscribing ? "Subscribing..." : "Subscribe" }}
                    </Button>
                </div>

                <!-- Current Subscriptions -->
                <div v-if="subscribedBanLists.length > 0">
                    <h4 class="text-xs sm:text-sm font-medium mb-2">
                        Current Subscriptions
                    </h4>
                    <div class="space-y-2">
                        <div
                            v-for="subscription in subscribedBanLists"
                            :key="subscription.ban_list_id"
                            class="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-2 p-3 border rounded-lg"
                        >
                            <div class="flex-1 min-w-0">
                                <div class="font-medium text-sm sm:text-base">
                                    {{
                                        subscription.ban_list_name ||
                                        "Unknown Ban List"
                                    }}
                                </div>
                                <div class="text-xs sm:text-sm text-gray-500">
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
                                class="w-full sm:w-auto text-xs sm:text-sm"
                            >
                                <Trash2 class="h-3 w-3 sm:h-4 sm:w-4 mr-1 sm:mr-2" />
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

                <div v-else class="text-center text-gray-500 py-4 text-xs sm:text-sm">
                    No ban list subscriptions configured
                </div>
            </div>
        </CardContent>
    </Card>
</template>
