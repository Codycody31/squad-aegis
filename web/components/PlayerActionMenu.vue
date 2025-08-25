<script setup lang="ts">
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from "@/components/ui/dropdown-menu";
import { Button } from "@/components/ui/button";
import type { Player } from "~/types";
import { useToast } from "~/components/ui/toast";

const { toast } = useToast();
const authStore = useAuthStore();

const props = defineProps<{
    player: Player;
    serverId: string;
}>();

const emit = defineEmits<{
    (e: "warn"): void;
    (e: "move"): void;
    (e: "kick"): void;
    (e: "ban"): void;
    (e: "remove-from-squad"): void;
}>();


// Function to copy text to clipboard
function copyToClipboard(text: string) {
    if (process.client) {
        navigator.clipboard.writeText(text)
            .then(() => {
                toast({
                    title: "Copied to clipboard",
                    description: "The text has been copied to your clipboard.",
                })
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
            <DropdownMenuItem @click="copyToClipboard(player.steam_id)">
                <Icon name="lucide:copy" class="mr-2 h-4 w-4 text-yellow-500" />
                <span>Copy Steam ID</span>
            </DropdownMenuItem>
            <DropdownMenuItem @click="copyToClipboard(player.eosId)">
                <Icon name="lucide:copy" class="mr-2 h-4 w-4 text-yellow-500" />
                <span>Copy EOS ID</span>
            </DropdownMenuItem>
            <template v-if="player.sinceDisconnect == ''">
                <DropdownMenuItem @click="emit('warn')" v-if="authStore.getServerPermission(serverId as string, 'warn')">
                <Icon name="lucide:alert-triangle" class="mr-2 h-4 w-4 text-yellow-500" />
                <span>Warn Player</span>
            </DropdownMenuItem>
            <DropdownMenuItem @click="emit('move')"
                v-if="authStore.getServerPermission(serverId as string, 'forceteamchange')">
                <Icon name="lucide:move" class="mr-2 h-4 w-4 text-blue-500" />
                <span>Move to Other Team</span>
            </DropdownMenuItem>
            <DropdownMenuItem @click="emit('remove-from-squad')" v-if="authStore.getServerPermission(serverId as string, 'kick') && player.squadId != 0">
                <Icon name="lucide:user-minus" class="mr-2 h-4 w-4 text-red-500" />
                <span>Remove from Squad</span>
            </DropdownMenuItem>
            <DropdownMenuItem @click="emit('kick')" v-if="authStore.getServerPermission(serverId as string, 'kick')">
                <Icon name="lucide:log-out" class="mr-2 h-4 w-4 text-orange-500" />
                <span>Kick Player</span>
            </DropdownMenuItem>
            </template>
            <DropdownMenuItem @click="emit('ban')" v-if="authStore.getServerPermission(serverId as string, 'ban')">
                <Icon name="lucide:ban" class="mr-2 h-4 w-4 text-red-500" />
                <span>Ban Player</span>
            </DropdownMenuItem>
        </DropdownMenuContent>
    </DropdownMenu>
</template>