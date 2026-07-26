<script setup lang="ts">
import PermissionButton from "@/components/PermissionButton.vue";
import { UI_PERMISSIONS } from "@/constants/permissions";
import { Button } from "~/components/ui/button";
import {
    Card,
    CardContent,
    CardDescription,
    CardHeader,
    CardTitle,
} from "~/components/ui/card";

defineProps<{
    serverId: string;
}>();

defineEmits<{
    (e: "broadcast"): void;
}>();

const links = [
    { label: "Console", icon: "mdi:console", to: "console" },
    { label: "Bans", icon: "mdi:account-remove", to: "banned-players" },
    { label: "Teams", icon: "mdi:flag-variant", to: "teams-and-squads" },
    {
        label: "Audit Logs",
        icon: "mdi:clipboard-text-clock",
        to: "audit-logs",
    },
];
</script>

<template>
    <Card>
        <CardHeader>
            <CardTitle>Quick Actions</CardTitle>
            <CardDescription>Common server management tasks</CardDescription>
        </CardHeader>
        <CardContent>
            <div class="grid grid-cols-2 sm:grid-cols-3 gap-4">
                <NuxtLink
                    v-for="link in links"
                    :key="link.to"
                    :to="`/servers/${serverId}/${link.to}`"
                    class="w-full"
                >
                    <Button
                        variant="outline"
                        class="w-full h-full flex flex-col items-center justify-center gap-2 p-4"
                    >
                        <Icon :name="link.icon" class="h-6 w-6" />
                        <div class="text-sm">{{ link.label }}</div>
                    </Button>
                </NuxtLink>
                <PermissionButton
                    :permission="UI_PERMISSIONS.CONSOLE_EXECUTE"
                    :server-id="serverId"
                    variant="outline"
                    class="w-full h-full flex flex-col items-center justify-center gap-2 p-4"
                    @click="$emit('broadcast')"
                >
                    <Icon name="mdi:bullhorn" class="h-6 w-6" />
                    <div class="text-sm">Broadcast</div>
                </PermissionButton>
            </div>
        </CardContent>
    </Card>
</template>
