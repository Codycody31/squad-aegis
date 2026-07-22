<script setup lang="ts">
import {
    Card,
    CardContent,
    CardHeader,
    CardTitle,
} from "~/components/ui/card";
import { Badge } from "~/components/ui/badge";

defineProps<{
    server: any;
    version?: string;
    licensed?: boolean;
}>();
</script>

<template>
    <Card>
        <CardHeader>
            <CardTitle>Server Information</CardTitle>
        </CardHeader>
        <CardContent>
            <div class="space-y-2">
                <div class="flex justify-between">
                    <span class="text-sm font-medium">Name:</span>
                    <span class="text-sm">{{ server?.name || "Unknown" }}</span>
                </div>
                <div class="flex justify-between">
                    <span class="text-sm font-medium">IP Address:</span>
                    <span class="text-sm">{{
                        server?.ip_address || "Unknown"
                    }}</span>
                </div>
                <div class="flex justify-between">
                    <span class="text-sm font-medium">Game Port:</span>
                    <span class="text-sm">{{
                        server?.game_port || "Unknown"
                    }}</span>
                </div>
                <div class="flex justify-between">
                    <span class="text-sm font-medium">RCON IP Address:</span>
                    <span class="text-sm">{{
                        server?.rcon_ip_address || "Unknown"
                    }}</span>
                </div>
                <div class="flex justify-between">
                    <span class="text-sm font-medium">RCON Port:</span>
                    <span class="text-sm">{{
                        server?.rcon_port || "Unknown"
                    }}</span>
                </div>
                <div class="flex justify-between">
                    <span class="text-sm font-medium">Server Version:</span>
                    <span class="text-sm">{{ version || "Unknown" }}</span>
                </div>
                <div class="flex justify-between">
                    <span class="text-sm font-medium">License Status:</span>
                    <span class="text-sm">
                        <Badge
                            v-if="licensed !== undefined"
                            :variant="licensed ? 'default' : 'destructive'"
                        >
                            {{ licensed ? "Licensed" : "Unlicensed" }}
                        </Badge>
                        <span v-else>Unknown</span>
                    </span>
                </div>
            </div>
        </CardContent>
    </Card>
</template>
