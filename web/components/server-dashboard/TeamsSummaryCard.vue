<script setup lang="ts">
import { Button } from "~/components/ui/button";
import {
    Card,
    CardContent,
    CardDescription,
    CardFooter,
    CardHeader,
    CardTitle,
} from "~/components/ui/card";
import { Progress } from "~/components/ui/progress";
import type { Team } from "~/components/server-dashboard/types";

defineProps<{
    serverId: string;
    teams: Team[];
    serverInfo: any;
    loading?: boolean;
    error?: string | null;
}>();
</script>

<template>
    <Card>
        <CardHeader>
            <CardTitle>Teams & Squads</CardTitle>
            <CardDescription
                >Team balance and squad distribution</CardDescription
            >
        </CardHeader>
        <CardContent>
            <div v-if="loading && teams.length === 0" class="text-center py-4">
                <div
                    class="animate-spin h-6 w-6 border-4 border-primary border-t-transparent rounded-full mx-auto mb-2"
                ></div>
                <p class="text-sm">Loading teams data...</p>
            </div>
            <div v-else-if="error" class="text-red-500 text-sm py-2">
                {{ error }}
            </div>
            <div v-else-if="teams.length === 0" class="text-center py-4">
                <p class="text-sm text-muted-foreground">
                    No teams data available
                </p>
            </div>
            <div v-else class="space-y-4">
                <div>
                    <h3 class="text-sm font-medium mb-2">Team Balance</h3>
                    <div
                        class="flex h-4 mb-2 bg-gray-200 rounded-full overflow-hidden"
                    >
                        <template v-if="teams.length >= 2">
                            <div
                                class="h-full bg-blue-500 transition-all duration-300"
                                :style="`width: ${Math.max(
                                    5,
                                    ((serverInfo?.metrics?.players?.teams?.[
                                        teams[0]?.id
                                    ] || 0) /
                                        Math.max(
                                            1,
                                            serverInfo?.metrics?.players
                                                ?.max || 64,
                                        )) *
                                        100,
                                )}%`"
                            ></div>
                            <div
                                class="h-full bg-red-500 transition-all duration-300"
                                :style="`width: ${Math.max(
                                    5,
                                    ((serverInfo?.metrics?.players?.teams?.[
                                        teams[1]?.id
                                    ] || 0) /
                                        Math.max(
                                            1,
                                            serverInfo?.metrics?.players
                                                ?.max || 64,
                                        )) *
                                        100,
                                )}%`"
                            ></div>
                        </template>
                        <template v-else>
                            <div class="h-full bg-blue-500 w-1/2"></div>
                            <div class="h-full bg-red-500 w-1/2"></div>
                        </template>
                    </div>
                    <div class="flex justify-between text-xs">
                        <span v-for="team in teams" :key="team.id">
                            {{ team.name }}:
                            {{
                                serverInfo?.metrics?.players?.teams?.[
                                    team.id
                                ] || 0
                            }}
                        </span>
                    </div>
                </div>

                <div>
                    <h3 class="text-sm font-medium mb-2">Squad Distribution</h3>
                    <div class="space-y-2">
                        <div v-for="team in teams" :key="team.id">
                            <h4 class="text-xs font-medium mb-1">
                                {{ team.name }}
                            </h4>
                            <div
                                v-for="squad in team.squads"
                                :key="squad.id"
                                class="mb-2"
                            >
                                <div class="flex justify-between text-xs mb-1">
                                    <span>{{ squad.name }}</span>
                                    <span
                                        >{{ squad.players.length }} / 9</span
                                    >
                                </div>
                                <Progress
                                    :modelValue="
                                        Math.min(
                                            100,
                                            Math.round(
                                                (squad.players.length / 9) *
                                                    100,
                                            ),
                                        )
                                    "
                                    class="h-2"
                                />
                            </div>
                        </div>
                    </div>
                </div>
            </div>
        </CardContent>
        <CardFooter>
            <NuxtLink
                :to="`/servers/${serverId}/teams-and-squads`"
                class="w-full"
            >
                <Button variant="outline" size="sm" class="w-full">
                    Manage Teams & Squads
                </Button>
            </NuxtLink>
        </CardFooter>
    </Card>
</template>
