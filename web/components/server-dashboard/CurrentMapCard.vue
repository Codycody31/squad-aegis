<script setup lang="ts">
import { computed, ref, watch } from "vue";
import PermissionButton from "@/components/PermissionButton.vue";
import { UI_PERMISSIONS } from "@/constants/permissions";
import { Button } from "~/components/ui/button";
import {
    Card,
    CardContent,
    CardFooter,
    CardHeader,
    CardTitle,
} from "~/components/ui/card";
import { Badge } from "~/components/ui/badge";

const props = defineProps<{
    serverId: string;
    serverInfo: any;
    rconServerInfo: any;
}>();

defineEmits<{
    (e: "change-map"): void;
    (e: "set-next"): void;
    (e: "end-match"): void;
}>();

const imageFailed = ref(false);
const currentLayer = computed(
    () => props.serverInfo?.metrics?.current?.layer,
);
watch(currentLayer, () => {
    imageFailed.value = false;
});

const timeRemainingMinutes = computed(() => {
    const info = props.rconServerInfo;
    if (!info) return 0;
    const seconds = info.match_timeout * 60 - info.playtime;
    return seconds > 0 ? Math.floor(seconds / 60) : 0;
});
</script>

<template>
    <Card>
        <CardHeader>
            <CardTitle>Current Map</CardTitle>
        </CardHeader>
        <CardContent>
            <div class="text-center">
                <div
                    class="relative w-full h-32 bg-gray-200 rounded-md mb-2 overflow-hidden"
                >
                    <div
                        class="absolute inset-0 flex items-center justify-center text-gray-500"
                    >
                        Map Preview
                    </div>
                    <img
                        v-if="currentLayer && !imageFailed"
                        :src="`https://raw.githubusercontent.com/mahtoid/SquadMaps/refs/heads/master/img/maps/thumbnails/${currentLayer}.jpg`"
                        class="absolute inset-0 w-full h-full object-cover"
                        @error="imageFailed = true"
                    />
                </div>
                <h3 class="text-lg font-medium">
                    {{ serverInfo?.metrics?.current?.map }}
                    {{
                        serverInfo?.metrics?.next
                            ? `-> ${serverInfo?.metrics?.next?.map}`
                            : ""
                    }}
                </h3>
                <div
                    v-if="serverInfo?.metrics?.current?.factions"
                    class="text-sm text-muted-foreground mt-2 space-y-1"
                >
                    <div
                        v-if="serverInfo?.metrics?.current?.factions[0]"
                        class="flex items-center justify-center gap-2"
                    >
                        <Badge variant="outline">
                            {{ serverInfo.metrics.current.factions[0] }}
                        </Badge>
                        <span class="text-xs">vs</span>
                        <Badge
                            v-if="serverInfo?.metrics?.current?.factions[1]"
                            variant="outline"
                        >
                            {{ serverInfo.metrics.current.factions[1] }}
                        </Badge>
                    </div>
                </div>
                <p class="text-sm text-muted-foreground mt-2">
                    Time Remaining: {{ timeRemainingMinutes }} minutes
                </p>
            </div>
        </CardContent>
        <CardFooter>
            <div class="w-full grid grid-cols-3 gap-2">
                <PermissionButton
                    :permission="UI_PERMISSIONS.MAPS_CHANGE"
                    :server-id="serverId"
                    variant="outline"
                    size="sm"
                    @click="$emit('change-map')"
                >
                    Change
                </PermissionButton>
                <PermissionButton
                    :permission="UI_PERMISSIONS.MAPS_CHANGE"
                    :server-id="serverId"
                    variant="outline"
                    size="sm"
                    @click="$emit('set-next')"
                >
                    Set Next
                </PermissionButton>
                <Button
                    variant="destructive"
                    size="sm"
                    @click="$emit('end-match')"
                >
                    End Match
                </Button>
            </div>
        </CardFooter>
    </Card>
</template>
