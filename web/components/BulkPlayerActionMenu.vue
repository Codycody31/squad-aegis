<script setup lang="ts">
import { ref, computed } from "vue";
import {
    ContextMenu,
    ContextMenuContent,
    ContextMenuItem,
    ContextMenuTrigger,
} from "@/components/ui/context-menu";
import {
    Dialog,
    DialogContent,
    DialogDescription,
    DialogFooter,
    DialogHeader,
    DialogTitle,
} from "~/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Textarea } from "~/components/ui/textarea";
import { Input } from "~/components/ui/input";
import {
    Select,
    SelectContent,
    SelectItem,
    SelectTrigger,
    SelectValue,
} from "~/components/ui/select";
import { useToast } from "~/components/ui/toast";
import type { Player } from "~/types";

const { toast } = useToast();
const authStore = useAuthStore();

const props = defineProps<{
    selectedPlayers: Player[];
    serverId: string;
}>();

const emit = defineEmits<{
    (e: "action-completed"): void;
    (e: "clear-selection"): void;
}>();

// Action dialog state
const showActionDialog = ref(false);
const actionType = ref<"kick" | "ban" | "warn" | "move" | null>(null);
const actionReason = ref("");
const actionDuration = ref(0);
const selectedRuleId = ref<string>("__none__");
const serverRules = ref<
    Array<{
        id: string;
        title: string;
        displayNumber: string;
        description?: string;
    }>
>([]);
const isActionLoading = ref(false);
const rulesFetched = ref(false);

// Fetch server rules
async function fetchServerRules() {
    const runtimeConfig = useRuntimeConfig();
    const cookieToken = useCookie(
        runtimeConfig.public.sessionCookieName as string,
    );
    const token = cookieToken.value;

    if (!token) return;

    try {
        const { data, error: fetchError } = await useFetch(
            `${runtimeConfig.public.backendApi}/servers/${props.serverId}/rules`,
            {
                method: "GET",
                headers: {
                    Authorization: `Bearer ${token}`,
                },
            },
        );

        if (fetchError.value) {
            console.error("Failed to fetch server rules:", fetchError.value);
            return;
        }

        if (data.value) {
            serverRules.value = flattenRulesForDropdown(
                Array.isArray(data.value) ? data.value : [],
            );
            rulesFetched.value = true;
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
): any[] {
    rules.forEach((rule, idx) => {
        const ruleNumber = parentNumber
            ? `${parentNumber}.${idx + 1}`
            : `${idx + 1}`;
        const formattedTitle = parentTitle
            ? `${parentTitle} > ${rule.title}`
            : rule.title;

        result.push({
            id: rule.id,
            title: formattedTitle,
            displayNumber: ruleNumber,
            description: rule.description,
        });

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

// Open action dialog
async function openActionDialog(action: "kick" | "ban" | "warn" | "move") {
    actionType.value = action;
    actionReason.value = "";
    actionDuration.value = action === "ban" ? 0 : 0;
    selectedRuleId.value = "__none__";
    showActionDialog.value = true;

    if (
        (action === "kick" || action === "ban" || action === "warn") &&
        !rulesFetched.value
    ) {
        await fetchServerRules();
        rulesFetched.value = true;
    }
}

// Close action dialog
function closeActionDialog() {
    showActionDialog.value = false;
    actionType.value = null;
    actionReason.value = "";
    actionDuration.value = 0;
    selectedRuleId.value = "__none__";
}

function getActionTitle() {
    if (!actionType.value) return "";

    const actionMap = {
        kick: "Kick",
        ban: "Ban",
        warn: "Warn",
        move: "Move",
    } as const;
    return `${actionMap[actionType.value]} ${props.selectedPlayers.length} Player${props.selectedPlayers.length > 1 ? "s" : ""}`;
}

async function executeBulkAction() {
    if (!actionType.value) return;

    isActionLoading.value = true;
    const runtimeConfig = useRuntimeConfig();
    const cookieToken = useCookie(
        runtimeConfig.public.sessionCookieName as string,
    );
    const token = cookieToken.value;

    if (!token) {
        toast({
            title: "Authentication Error",
            description: "You must be logged in to perform this action",
            variant: "destructive",
        });
        isActionLoading.value = false;
        closeActionDialog();
        return;
    }

    try {
        let successCount = 0;
        let failCount = 0;

        for (const player of props.selectedPlayers) {
            try {
                let endpoint = "";
                let payload: any = {};

                switch (actionType.value) {
                    case "kick":
                        endpoint = `${runtimeConfig.public.backendApi}/servers/${props.serverId}/rcon/player/kick`;
                        payload = {
                            steam_id: player.steam_id,
                            reason: actionReason.value,
                        };
                        if (
                            selectedRuleId.value &&
                            selectedRuleId.value !== "" &&
                            selectedRuleId.value !== "__none__"
                        ) {
                            payload.rule_id = selectedRuleId.value;
                        }
                        break;
                    case "ban":
                        endpoint = `${runtimeConfig.public.backendApi}/servers/${props.serverId}/rcon/player/ban`;
                        payload = {
                            steam_id: player.steam_id,
                            reason: actionReason.value,
                            duration: actionDuration.value,
                        };
                        if (
                            selectedRuleId.value &&
                            selectedRuleId.value !== "" &&
                            selectedRuleId.value !== "__none__"
                        ) {
                            payload.rule_id = selectedRuleId.value;
                        }
                        break;
                    case "warn":
                        endpoint = `${runtimeConfig.public.backendApi}/servers/${props.serverId}/rcon/player/warn`;
                        payload = {
                            steam_id: player.steam_id,
                            message: actionReason.value,
                        };
                        if (
                            selectedRuleId.value &&
                            selectedRuleId.value !== "" &&
                            selectedRuleId.value !== "__none__"
                        ) {
                            payload.rule_id = selectedRuleId.value;
                        }
                        break;
                    case "move":
                        endpoint = `${runtimeConfig.public.backendApi}/servers/${props.serverId}/rcon/move-player`;
                        payload = {
                            steam_id: player.steam_id,
                        };
                        break;
                }

                const { error: fetchError } = await useFetch(endpoint, {
                    method: "POST",
                    headers: {
                        Authorization: `Bearer ${token}`,
                        "Content-Type": "application/json",
                    },
                    body: JSON.stringify(payload),
                });

                if (fetchError.value) {
                    console.error(`Failed to ${actionType.value} ${player.name}:`, fetchError.value);
                    failCount++;
                } else {
                    successCount++;
                }
            } catch (err: any) {
                console.error(`Failed to ${actionType.value} ${player.name}:`, err);
                failCount++;
            }
        }

        let successMessage = `${successCount} player${successCount !== 1 ? "s" : ""} ${actionType.value === "move" ? "moved" : actionType.value + "ed"}`;
        if (failCount > 0) {
            successMessage += `, ${failCount} failed`;
        }

        toast({
            title: failCount === 0 ? "Success" : "Partial Success",
            description: successMessage,
            variant: failCount === 0 ? "default" : "destructive",
        });

        emit("action-completed");
        emit("clear-selection");
    } catch (err: any) {
        console.error(err);
        toast({
            title: "Error",
            description: err.message || `Failed to ${actionType.value} players`,
            variant: "destructive",
        });
    } finally {
        isActionLoading.value = false;
        closeActionDialog();
    }
}

const hasKickPermission = computed(() =>
    authStore.getServerPermission(props.serverId as string, "kick")
);

const hasBanPermission = computed(() =>
    authStore.getServerPermission(props.serverId as string, "ban")
);

const hasWarnPermission = computed(() =>
    authStore.getServerPermission(props.serverId as string, "warn")
);

const hasMovePermission = computed(() =>
    authStore.getServerPermission(props.serverId as string, "forceteamchange")
);
</script>

<template>
    <ContextMenu>
        <ContextMenuTrigger>
            <slot />
        </ContextMenuTrigger>
        <ContextMenuContent v-if="selectedPlayers.length > 0">
            <div class="px-2 py-1.5 text-sm font-semibold">
                {{ selectedPlayers.length }} player{{
                    selectedPlayers.length > 1 ? "s" : ""
                }}
                selected
            </div>
            <ContextMenuItem
                @click="openActionDialog('warn')"
                v-if="hasWarnPermission"
            >
                <Icon
                    name="lucide:alert-triangle"
                    class="mr-2 h-4 w-4 text-yellow-500"
                />
                <span>Warn Players</span>
            </ContextMenuItem>
            <ContextMenuItem
                @click="openActionDialog('move')"
                v-if="hasMovePermission"
            >
                <Icon name="lucide:move" class="mr-2 h-4 w-4 text-blue-500" />
                <span>Move to Other Team</span>
            </ContextMenuItem>
            <ContextMenuItem
                @click="openActionDialog('kick')"
                v-if="hasKickPermission"
            >
                <Icon
                    name="lucide:log-out"
                    class="mr-2 h-4 w-4 text-orange-500"
                />
                <span>Kick Players</span>
            </ContextMenuItem>
            <ContextMenuItem @click="openActionDialog('ban')" v-if="hasBanPermission">
                <Icon name="lucide:ban" class="mr-2 h-4 w-4 text-red-500" />
                <span>Ban Players</span>
            </ContextMenuItem>
            <ContextMenuItem @click="emit('clear-selection')">
                <Icon name="lucide:x" class="mr-2 h-4 w-4" />
                <span>Clear Selection</span>
            </ContextMenuItem>
        </ContextMenuContent>
    </ContextMenu>

    <!-- Action Dialog -->
    <Dialog v-model:open="showActionDialog">
        <DialogContent class="sm:max-w-[425px]">
            <DialogHeader>
                <DialogTitle>{{ getActionTitle() }}</DialogTitle>
                <DialogDescription>
                    <template v-if="actionType === 'kick'">
                        Kick {{ selectedPlayers.length }} player{{
                            selectedPlayers.length > 1 ? "s" : ""
                        }}
                        from the server. They will be able to rejoin.
                    </template>
                    <template v-else-if="actionType === 'ban'">
                        Ban {{ selectedPlayers.length }} player{{
                            selectedPlayers.length > 1 ? "s" : ""
                        }}
                        from the server for a specified duration.
                    </template>
                    <template v-else-if="actionType === 'warn'">
                        Send a warning message to {{ selectedPlayers.length }}
                        player{{ selectedPlayers.length > 1 ? "s" : "" }}.
                    </template>
                    <template v-else-if="actionType === 'move'">
                        Force {{ selectedPlayers.length }} player{{
                            selectedPlayers.length > 1 ? "s" : ""
                        }}
                        to switch to another team.
                    </template>
                </DialogDescription>
            </DialogHeader>

            <div class="grid gap-4 py-4">
                <!-- Rule Selection (for kick, ban, warn) -->
                <div
                    v-if="
                        actionType === 'kick' ||
                        actionType === 'ban' ||
                        actionType === 'warn'
                    "
                    class="grid grid-cols-4 items-center gap-4"
                >
                    <label for="rule" class="text-right col-span-1"
                        >Rule Violation</label
                    >
                    <Select v-model="selectedRuleId">
                        <SelectTrigger class="col-span-3">
                            <SelectValue
                                placeholder="Optional: Select a violated rule"
                            />
                        </SelectTrigger>
                        <SelectContent>
                            <SelectItem value="__none__">
                                No rule violation
                            </SelectItem>
                            <SelectItem
                                v-for="rule in serverRules"
                                :key="rule.id"
                                :value="rule.id"
                            >
                                {{ rule.displayNumber }}: {{ rule.title }}
                            </SelectItem>
                        </SelectContent>
                    </Select>
                </div>

                <div
                    v-if="actionType === 'ban'"
                    class="grid grid-cols-4 items-center gap-4"
                >
                    <label for="duration" class="text-right col-span-1"
                        >Duration</label
                    >
                    <Input
                        id="duration"
                        v-model="actionDuration"
                        placeholder="0"
                        class="col-span-3"
                        type="number"
                    />
                    <div class="col-span-1"></div>
                    <div class="text-xs text-muted-foreground col-span-3">
                        Ban duration in days. Use 0 for a permanent ban.
                    </div>
                </div>

                <div v-if="actionType !== 'move'" class="grid grid-cols-4 items-center gap-4">
                    <label for="reason" class="text-right col-span-1">
                        {{ actionType === "warn" ? "Message" : "Reason" }}
                    </label>
                    <Textarea
                        id="reason"
                        v-model="actionReason"
                        :placeholder="
                            actionType === 'warn'
                                ? 'Warning message'
                                : 'Reason for action'
                        "
                        class="col-span-3"
                        rows="3"
                    />
                </div>

                <!-- Selected Players List -->
                <div class="grid grid-cols-4 items-start gap-4">
                    <label class="text-right col-span-1">Players</label>
                    <div class="col-span-3 max-h-32 overflow-y-auto">
                        <div
                            v-for="player in selectedPlayers"
                            :key="player.steam_id"
                            class="text-sm py-1"
                        >
                            {{ player.name }}
                        </div>
                    </div>
                </div>
            </div>

            <DialogFooter>
                <Button variant="outline" @click="closeActionDialog"
                    >Cancel</Button
                >
                <Button
                    :variant="
                        actionType === 'warn' || actionType === 'move'
                            ? 'default'
                            : 'destructive'
                    "
                    @click="executeBulkAction"
                    :disabled="isActionLoading"
                >
                    <span v-if="isActionLoading" class="mr-2">
                        <Icon
                            name="lucide:loader-2"
                            class="h-4 w-4 animate-spin"
                        />
                    </span>
                    <template v-if="actionType === 'kick'">Kick Players</template>
                    <template v-else-if="actionType === 'ban'">Ban Players</template>
                    <template v-else-if="actionType === 'warn'"
                        >Send Warning</template
                    >
                    <template v-else-if="actionType === 'move'"
                        >Move Players</template
                    >
                </Button>
            </DialogFooter>
        </DialogContent>
    </Dialog>
</template>

