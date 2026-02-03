<script setup lang="ts">
import { ref } from "vue";
import {
    DropdownMenu,
    DropdownMenuContent,
    DropdownMenuItem,
    DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { Button } from "@/components/ui/button";
import {
    Dialog,
    DialogContent,
    DialogDescription,
    DialogFooter,
    DialogHeader,
    DialogTitle,
} from "~/components/ui/dialog";
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
import { UI_PERMISSIONS } from "~/constants/permissions";

const { toast } = useToast();
const authStore = useAuthStore();

const props = defineProps<{
    player: Player;
    serverId: string;
}>();

const emit = defineEmits<{
    (e: "action-completed"): void;
}>();

// Action dialog state
const showActionDialog = ref(false);
const actionType = ref<
    "kick" | "ban" | "warn" | "move" | "remove-from-squad" | null
>(null);
const actionReason = ref("");
const actionDuration = ref(0); // For ban duration in minutes
const selectedRuleId = ref<string>("__none__");
const serverRules = ref<
    Array<{
        id: string;
        title: string;
        displayNumber: string;
        description?: string;
    }>
>([]);
const escalationSuggestion = ref<any>(null);
const loadingEscalation = ref(false);
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
            // Flatten rules hierarchy
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
        // Calculate rule number (e.g., "1", "1.2", "1.2.3")
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

// Fetch escalation suggestion when rule is selected
async function fetchEscalationSuggestion(steamId: string, ruleId: string) {
    if (!ruleId || !steamId || ruleId === "" || ruleId === "__none__") {
        escalationSuggestion.value = null;
        return;
    }

    loadingEscalation.value = true;
    const runtimeConfig = useRuntimeConfig();
    const cookieToken = useCookie(
        runtimeConfig.public.sessionCookieName as string,
    );
    const token = cookieToken.value;

    if (!token) {
        loadingEscalation.value = false;
        return;
    }

    try {
        const { data, error: fetchError } = await useFetch(
            `${runtimeConfig.public.backendApi}/servers/${props.serverId}/rcon/player/escalation-suggestion?steam_id=${steamId}&rule_id=${ruleId}`,
            {
                method: "GET",
                headers: {
                    Authorization: `Bearer ${token}`,
                },
            },
        );

        if (fetchError.value) {
            console.error(
                "Failed to fetch escalation suggestion:",
                fetchError.value,
            );
            escalationSuggestion.value = null;
        } else if (
            data.value &&
            (data.value as any).data &&
            (data.value as any).data.suggestion
        ) {
            const suggestion = (data.value as any).data.suggestion;
            escalationSuggestion.value = suggestion;

            // Generate reason from rule number and title if no custom message is provided
            if (
                !actionReason.value &&
                suggestion.rule_id &&
                !suggestion.suggested_message
            ) {
                const rule = serverRules.value.find(
                    (r) => r.id === suggestion.rule_id,
                );
                if (rule) {
                    actionReason.value = `${rule.displayNumber} | ${rule.title}`;
                }
            } else if (!actionReason.value && suggestion.suggested_message) {
                // Use custom message from rule action if available
                actionReason.value = suggestion.suggested_message;
            }

            // Prefill duration if suggestion provides one for ban
            if (
                actionType.value === "ban" &&
                suggestion.suggested_duration &&
                actionDuration.value === 0
            ) {
                actionDuration.value = suggestion.suggested_duration;
            }
        } else {
            escalationSuggestion.value = null;
        }
    } catch (err: any) {
        console.error("Failed to fetch escalation suggestion:", err);
        escalationSuggestion.value = null;
    } finally {
        loadingEscalation.value = false;
    }
}

// Handle rule selection
function onRuleSelected(ruleId: string) {
    if (props.player && ruleId && ruleId !== "" && ruleId !== "__none__") {
        fetchEscalationSuggestion(props.player.steam_id, ruleId);
    } else {
        escalationSuggestion.value = null;
    }
}

// Switch to suggested action
function switchToSuggestedAction() {
    if (
        !escalationSuggestion.value ||
        !escalationSuggestion.value.suggested_action
    ) {
        return;
    }

    const suggestedAction =
        escalationSuggestion.value.suggested_action.toLowerCase();

    // Map suggested action to our action type
    if (
        suggestedAction === "kick" ||
        suggestedAction === "ban" ||
        suggestedAction === "warn"
    ) {
        actionType.value = suggestedAction as "kick" | "ban" | "warn";

        // Set duration if switching to ban and suggestion provides one
        if (
            suggestedAction === "ban" &&
            escalationSuggestion.value.suggested_duration
        ) {
            actionDuration.value =
                escalationSuggestion.value.suggested_duration;
        } else if (suggestedAction !== "ban") {
            actionDuration.value = 0;
        }

        // Set reason/message if not already set
        if (!actionReason.value) {
            if (escalationSuggestion.value.suggested_message) {
                actionReason.value =
                    escalationSuggestion.value.suggested_message;
            } else if (escalationSuggestion.value.rule_id) {
                const rule = serverRules.value.find(
                    (r) => r.id === escalationSuggestion.value.rule_id,
                );
                if (rule) {
                    actionReason.value = `Rule #${rule.displayNumber}: ${rule.title}`;
                }
            }
        }
    }
}

// Open action dialog
async function openActionDialog(
    action: "kick" | "ban" | "warn" | "move" | "remove-from-squad",
) {
    actionType.value = action;
    actionReason.value = "";
    actionDuration.value = action === "ban" ? 0 : 0;
    selectedRuleId.value = "__none__";
    escalationSuggestion.value = null;
    showActionDialog.value = true;

    // Only fetch rules if needed for this action and haven't fetched yet
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
    escalationSuggestion.value = null;
}

function getActionTitle() {
    if (!actionType.value) return "";

    const actionMap = {
        kick: "Kick",
        ban: "Ban",
        warn: "Warn",
        move: "Move",
        "remove-from-squad": "Remove from Squad",
    } as const;
    return `${actionMap[actionType.value]} ${props.player.name}`;
}

async function executePlayerAction() {
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
        let endpoint = "";
        let payload: any = {};

        switch (actionType.value) {
            case "kick":
                endpoint = `${runtimeConfig.public.backendApi}/servers/${props.serverId}/rcon/player/kick`;
                payload = {
                    steam_id: props.player.steam_id,
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
                    steam_id: props.player.steam_id,
                    reason: actionReason.value,
                    duration: actionDuration.value, // Duration in days
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
                    steam_id: props.player.steam_id,
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
                    steam_id: props.player.steam_id,
                };
                break;
            case "remove-from-squad":
                endpoint = `${runtimeConfig.public.backendApi}/servers/${props.serverId}/rcon/execute`;
                payload = {
                    command: `AdminRemovePlayerFromSquadById ${props.player.playerId}`,
                };
                break;
        }

        const { data, error: fetchError } = await useFetch(endpoint, {
            method: "POST",
            headers: {
                Authorization: `Bearer ${token}`,
                "Content-Type": "application/json",
            },
            body: JSON.stringify(payload),
        });

        if (fetchError.value) {
            throw new Error(
                fetchError.value.message || `Failed to ${actionType.value}`,
            );
        }

        let successMessage = `Player ${props.player.name} has been `;
        if (actionType.value === "move") {
            successMessage += "moved";
        } else if (actionType.value === "ban") {
            successMessage += "banned";
            if (actionDuration.value) {
                const days = actionDuration.value;
                if (days >= 1) {
                    successMessage += ` for ${days} ${days === 1 ? "day" : "days"}`;
                } else {
                    successMessage += " permanently";
                }
            }
        } else if (actionType.value === "remove-from-squad") {
            successMessage += "removed from squad";
        } else {
            successMessage += actionType.value + "ed";
        }

        toast({
            title: "Success",
            description: successMessage,
            variant: "default",
        });

        emit("action-completed");
    } catch (err: any) {
        console.error(err);
        toast({
            title: "Error",
            description: err.message || `Failed to ${actionType.value}`,
            variant: "destructive",
        });
    } finally {
        isActionLoading.value = false;
        closeActionDialog();
    }
}

// Function to copy text to clipboard
function copyToClipboard(text: string) {
    if (process.client) {
        navigator.clipboard
            .writeText(text)
            .then(() => {
                toast({
                    title: "Copied to clipboard",
                    description: "The text has been copied to your clipboard.",
                });
            })
            .catch((err) => {
                console.error("Failed to copy text: ", err);
            });
    }
}
</script>

<template>
    <DropdownMenu>
        <DropdownMenuTrigger asChild>
            <Button variant="outline" size="sm" class="h-8 w-8 p-0">
                <span class="sr-only">Open menu</span>
                <Icon name="lucide:more-vertical" class="h-4 w-4" />
            </Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="end">
            <RouterLink
                :to="'/players/' + player.steam_id || player.eosId"
                as-child
            >
                <DropdownMenuItem>
                    <Icon
                        name="lucide:user"
                        class="mr-2 h-4 w-4 text-blue-500"
                    />
                    <span>View Player Profile</span>
                </DropdownMenuItem>
            </RouterLink>
            <DropdownMenuItem @click="copyToClipboard(player.steam_id)">
                <Icon name="lucide:copy" class="mr-2 h-4 w-4 text-yellow-500" />
                <span>Copy Steam ID</span>
            </DropdownMenuItem>
            <DropdownMenuItem @click="copyToClipboard(player.eosId)">
                <Icon name="lucide:copy" class="mr-2 h-4 w-4 text-yellow-500" />
                <span>Copy EOS ID</span>
            </DropdownMenuItem>
            <template v-if="player.sinceDisconnect == ''">
                <DropdownMenuItem
                    @click="openActionDialog('warn')"
                    v-if="
                        authStore.hasPermission(
                            serverId as string,
                            UI_PERMISSIONS.PLAYERS_WARN,
                        )
                    "
                >
                    <Icon
                        name="lucide:alert-triangle"
                        class="mr-2 h-4 w-4 text-yellow-500"
                    />
                    <span>Warn Player</span>
                </DropdownMenuItem>
                <DropdownMenuItem
                    @click="openActionDialog('move')"
                    v-if="
                        authStore.hasPermission(
                            serverId as string,
                            UI_PERMISSIONS.PLAYERS_MOVE,
                        )
                    "
                >
                    <Icon
                        name="lucide:move"
                        class="mr-2 h-4 w-4 text-blue-500"
                    />
                    <span>Move to Other Team</span>
                </DropdownMenuItem>
                <DropdownMenuItem
                    @click="openActionDialog('remove-from-squad')"
                    v-if="
                        authStore.hasPermission(
                            serverId as string,
                            UI_PERMISSIONS.PLAYERS_KICK,
                        ) && player.squadId != 0
                    "
                >
                    <Icon
                        name="lucide:user-minus"
                        class="mr-2 h-4 w-4 text-red-500"
                    />
                    <span>Remove from Squad</span>
                </DropdownMenuItem>
                <DropdownMenuItem
                    @click="openActionDialog('kick')"
                    v-if="
                        authStore.hasPermission(
                            serverId as string,
                            UI_PERMISSIONS.PLAYERS_KICK,
                        )
                    "
                >
                    <Icon
                        name="lucide:log-out"
                        class="mr-2 h-4 w-4 text-orange-500"
                    />
                    <span>Kick Player</span>
                </DropdownMenuItem>
            </template>
            <DropdownMenuItem
                @click="openActionDialog('ban')"
                v-if="authStore.hasPermission(serverId as string, UI_PERMISSIONS.BANS_CREATE)"
            >
                <Icon name="lucide:ban" class="mr-2 h-4 w-4 text-red-500" />
                <span>Ban Player</span>
            </DropdownMenuItem>
        </DropdownMenuContent>
    </DropdownMenu>

    <!-- Action Dialog -->
    <Dialog v-model:open="showActionDialog">
        <DialogContent class="sm:max-w-[425px]">
            <DialogHeader>
                <DialogTitle>{{ getActionTitle() }}</DialogTitle>
                <DialogDescription>
                    <template v-if="actionType === 'kick'">
                        Kick this player from the server. They will be able to
                        rejoin.
                    </template>
                    <template v-else-if="actionType === 'ban'">
                        Ban this player from the server for a specified
                        duration.
                    </template>
                    <template v-else-if="actionType === 'warn'">
                        Send a warning message to this player.
                    </template>
                    <template v-else-if="actionType === 'move'">
                        Force this player to switch to another team.
                    </template>
                    <template v-else-if="actionType === 'remove-from-squad'">
                        Remove this player from their squad.
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
                    <Select
                        v-model="selectedRuleId"
                        @update:model-value="
                            (value) => onRuleSelected(value as string)
                        "
                    >
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

                <div
                    v-if="
                        actionType !== 'move' &&
                        actionType !== 'remove-from-squad'
                    "
                    class="grid grid-cols-4 items-center gap-4"
                >
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

                <div
                    v-if="actionType === 'remove-from-squad'"
                    class="text-sm text-muted-foreground"
                >
                    Are you sure you want to remove {{ player.name }} from their
                    squad?
                </div>
            </div>

            <!-- Escalation Suggestion -->
            <div
                v-if="
                    escalationSuggestion &&
                    (actionType === 'kick' ||
                        actionType === 'ban' ||
                        actionType === 'warn')
                "
                class="gap-4 p-3 bg-muted border rounded-lg"
            >
                <div class="col-span-4">
                    <div class="flex items-start space-x-2">
                        <Icon
                            name="mdi:information"
                            class="h-5 w-5 text-primary mt-0.5 flex-shrink-0"
                        />
                        <div class="flex-1">
                            <div class="flex items-center justify-between">
                                <p class="text-sm font-medium">
                                    Escalation Suggestion
                                </p>
                                <Button
                                    v-if="
                                        escalationSuggestion.suggested_action &&
                                        escalationSuggestion.suggested_action.toLowerCase() !==
                                            actionType
                                    "
                                    @click="switchToSuggestedAction"
                                    variant="default"
                                    size="sm"
                                    class="h-7 text-xs"
                                >
                                    Switch to
                                    {{ escalationSuggestion.suggested_action }}
                                </Button>
                            </div>
                            <p class="text-xs text-muted-foreground mt-1">
                                This player has violated this rule
                                <strong>{{
                                    escalationSuggestion.violation_count
                                }}</strong>
                                time(s).
                                <template
                                    v-if="escalationSuggestion.suggested_action"
                                >
                                    Suggested action:
                                    <strong>{{
                                        escalationSuggestion.suggested_action
                                    }}</strong>
                                    <template
                                        v-if="
                                            escalationSuggestion.suggested_duration
                                        "
                                    >
                                        for
                                        <strong
                                            >{{
                                                escalationSuggestion.suggested_duration
                                            }}
                                            days</strong
                                        >
                                    </template>
                                </template>
                            </p>
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
                    @click="executePlayerAction"
                    :disabled="isActionLoading"
                >
                    <span v-if="isActionLoading" class="mr-2">
                        <Icon name="mdi:loading" class="h-4 w-4 animate-spin" />
                    </span>
                    <template v-if="actionType === 'kick'"
                        >Kick Player</template
                    >
                    <template v-else-if="actionType === 'ban'"
                        >Ban Player</template
                    >
                    <template v-else-if="actionType === 'warn'"
                        >Send Warning</template
                    >
                    <template v-else-if="actionType === 'move'"
                        >Move Player</template
                    >
                    <template v-else-if="actionType === 'remove-from-squad'"
                        >Remove from Squad</template
                    >
                </Button>
            </DialogFooter>
        </DialogContent>
    </Dialog>
</template>
