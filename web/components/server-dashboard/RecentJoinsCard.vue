<script setup lang="ts">
import {
    Card,
    CardContent,
    CardDescription,
    CardHeader,
    CardTitle,
} from "~/components/ui/card";
import {
    Table,
    TableBody,
    TableCell,
    TableHead,
    TableHeader,
    TableRow,
} from "~/components/ui/table";
import type { RecentJoin } from "~/components/server-dashboard/types";

defineProps<{
    joins: RecentJoin[];
    loading?: boolean;
    error?: string | null;
}>();
</script>

<template>
    <Card>
        <CardHeader>
            <CardTitle>Last 5 Server Joins</CardTitle>
            <CardDescription
                >Most recent players who joined the server</CardDescription
            >
        </CardHeader>
        <CardContent>
            <div v-if="loading && joins.length === 0" class="text-center py-4">
                <div
                    class="animate-spin h-6 w-6 border-4 border-primary border-t-transparent rounded-full mx-auto mb-2"
                ></div>
                <p class="text-sm">Loading recent joins...</p>
            </div>
            <div v-else-if="error" class="text-red-500 text-sm py-2">
                {{ error }}
            </div>
            <div v-else-if="joins.length === 0" class="text-center py-4">
                <p class="text-sm text-muted-foreground">
                    No recent joins recorded
                </p>
            </div>
            <Table v-else>
                <TableHeader>
                    <TableRow>
                        <TableHead>Player</TableHead>
                        <TableHead>Joined</TableHead>
                    </TableRow>
                </TableHeader>
                <TableBody>
                    <TableRow v-for="join in joins" :key="join.id">
                        <TableCell>
                            <div class="font-medium">
                                {{ join.player_name }}
                            </div>
                        </TableCell>
                        <TableCell>
                            <span class="text-sm text-muted-foreground">
                                {{
                                    new Date(
                                        join.joined_at,
                                    ).toLocaleTimeString()
                                }}
                            </span>
                        </TableCell>
                    </TableRow>
                </TableBody>
            </Table>
        </CardContent>
    </Card>
</template>
