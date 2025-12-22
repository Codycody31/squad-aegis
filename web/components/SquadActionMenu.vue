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
import { useToast } from "~/components/ui/toast";

const { toast } = useToast();
const authStore = useAuthStore();

interface Squad {
    id: number;
    name: string;
    size: number;
    locked: boolean;
    leader: any | null;
    players: any[];
    teamId?: number;
}

const props = defineProps<{
    squad: Squad;
    teamId: number;
    serverId: string;
}>();

const emit = defineEmits<{
    (e: "action-completed"): void;
}>();

// Action dialog state
const showActionDialog = ref(false);
const actionType = ref<"disband" | "swap-team" | "demote-commander" | null>(null);
const isActionLoading = ref(false);

// Open action dialog
function openActionDialog(action: "disband" | "swap-team" | "demote-commander") {
    actionType.value = action;
    showActionDialog.value = true;
}

// Close action dialog
function closeActionDialog() {
    showActionDialog.value = false;
    actionType.value = null;
}

function getActionTitle() {
    if (!actionType.value) return "";

    const actionMap = {
        disband: "Disband Squad",
        "swap-team": "Swap Squad to Other Team",
        "demote-commander": "Demote Commander",
    } as const;
    return `${actionMap[actionType.value]} ${props.squad.id}: ${props.squad.name}`;
}

function getSquadLeaderName(): string {
    if (props.squad.leader) {
        return props.squad.leader.name;
    }

    const leader = props.squad.players.find((player) => player.isSquadLeader);
    return leader ? leader.name : "No Leader";
}

async function executeSquadAction() {
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
        const endpoint = `${runtimeConfig.public.backendApi}/servers/${props.serverId}/rcon/execute`;
        let command = "";

        switch (actionType.value) {
            case "disband":
                command = `AdminDisbandSquad ${props.teamId} ${props.squad.id}`;
                break;
            case "swap-team":
                // Move all players in the squad to the other team
                for (const player of props.squad.players) {
                    const moveEndpoint = `${runtimeConfig.public.backendApi}/servers/${props.serverId}/rcon/move-player`;
                    const movePayload = {
                        steam_id: player.steam_id,
                    };

                    const { error: moveError } = await useFetch(moveEndpoint, {
                        method: "POST",
                        headers: {
                            Authorization: `Bearer ${token}`,
                            "Content-Type": "application/json",
                        },
                        body: JSON.stringify(movePayload),
                    });

                    if (moveError.value) {
                        console.error(`Failed to move player ${player.name}:`, moveError.value);
                    }
                }

                toast({
                    title: "Success",
                    description: `Squad ${props.squad.id}: ${props.squad.name} has been moved to the other team`,
                    variant: "default",
                });

                emit("action-completed");
                isActionLoading.value = false;
                closeActionDialog();
                return;
            case "demote-commander":
                const leaderName = getSquadLeaderName();
                if (!leaderName || leaderName === "No Leader") {
                    toast({
                        title: "Error",
                        description: "No squad leader found to demote",
                        variant: "destructive",
                    });
                    closeActionDialog();
                    isActionLoading.value = false;
                    return;
                }
                command = `AdminDemoteCommander ${leaderName}`;
                break;
        }

        const payload = {
            command: command,
        };

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

        let successMessage = "";
        if (actionType.value === "disband") {
            successMessage = `Squad ${props.squad.id}: ${props.squad.name} has been disbanded`;
        } else if (actionType.value === "demote-commander") {
            successMessage = `Commander ${getSquadLeaderName()} has been demoted`;
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
            <DropdownMenuItem
                @click="openActionDialog('swap-team')"
                v-if="
                    authStore.getServerPermission(
                        serverId as string,
                        'forceteamchange',
                    )
                "
            >
                <Icon
                    name="lucide:repeat"
                    class="mr-2 h-4 w-4 text-blue-500"
                />
                <span>Swap to Other Team</span>
            </DropdownMenuItem>
            <DropdownMenuItem
                v-if="squad.name === 'COMMAND SQUAD'"
                @click="openActionDialog('demote-commander')"
                v-show="
                    authStore.getServerPermission(
                        serverId as string,
                        'kick',
                    )
                "
            >
                <Icon
                    name="lucide:user-minus"
                    class="mr-2 h-4 w-4 text-orange-500"
                />
                <span>Demote Commander</span>
            </DropdownMenuItem>
            <DropdownMenuItem
                @click="openActionDialog('disband')"
                v-if="
                    authStore.getServerPermission(
                        serverId as string,
                        'kick',
                    )
                "
            >
                <Icon
                    name="lucide:x-circle"
                    class="mr-2 h-4 w-4 text-red-500"
                />
                <span>Disband Squad</span>
            </DropdownMenuItem>
        </DropdownMenuContent>
    </DropdownMenu>

    <!-- Action Dialog -->
    <Dialog v-model:open="showActionDialog">
        <DialogContent class="sm:max-w-[425px]">
            <DialogHeader>
                <DialogTitle>{{ getActionTitle() }}</DialogTitle>
                <DialogDescription>
                    <template v-if="actionType === 'disband'">
                        Disband this squad and remove all players from it. This
                        action cannot be undone.
                    </template>
                    <template v-else-if="actionType === 'swap-team'">
                        Move all players in this squad to the other team.
                    </template>
                    <template v-else-if="actionType === 'demote-commander'">
                        Demote the current commander from this squad.
                    </template>
                </DialogDescription>
            </DialogHeader>

            <div class="grid gap-4 py-4">
                <div class="text-sm text-muted-foreground">
                    <template v-if="actionType === 'disband'">
                        Are you sure you want to disband Squad {{ squad.id }}:
                        {{ squad.name }}? All {{ squad.players.length }} players
                        will be removed from the squad.
                    </template>
                    <template v-else-if="actionType === 'swap-team'">
                        Are you sure you want to move all {{ squad.players.length }}
                        players in Squad {{ squad.id }}: {{ squad.name }} to the
                        other team?
                    </template>
                    <template v-else-if="actionType === 'demote-commander'">
                        Are you sure you want to demote
                        <strong>{{ getSquadLeaderName() }}</strong> from commander?
                    </template>
                </div>
            </div>

            <DialogFooter>
                <Button variant="outline" @click="closeActionDialog"
                    >Cancel</Button
                >
                <Button
                    :variant="actionType === 'swap-team' ? 'default' : 'destructive'"
                    @click="executeSquadAction"
                    :disabled="isActionLoading"
                >
                    <span v-if="isActionLoading" class="mr-2">
                        <Icon
                            name="lucide:loader-2"
                            class="h-4 w-4 animate-spin"
                        />
                    </span>
                    <template v-if="actionType === 'disband'"
                        >Disband Squad</template
                    >
                    <template v-else-if="actionType === 'swap-team'"
                        >Swap Team</template
                    >
                    <template v-else-if="actionType === 'demote-commander'"
                        >Demote Commander</template
                    >
                </Button>
            </DialogFooter>
        </DialogContent>
    </Dialog>
</template>

