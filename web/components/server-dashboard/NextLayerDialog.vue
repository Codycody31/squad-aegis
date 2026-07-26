<script setup lang="ts">
import { ref, watch } from "vue";
import { Button } from "~/components/ui/button";
import {
    Dialog,
    DialogContent,
    DialogDescription,
    DialogFooter,
    DialogHeader,
    DialogTitle,
} from "~/components/ui/dialog";
import {
    Command,
    CommandEmpty,
    CommandGroup,
    CommandInput,
    CommandItem,
    CommandList,
} from "~/components/ui/command";
import {
    Popover,
    PopoverContent,
    PopoverTrigger,
} from "~/components/ui/popover";
import { toast } from "~/components/ui/toast";

const props = defineProps<{
    serverId: string;
    currentNextLayer?: string;
}>();

const emit = defineEmits<{
    (e: "completed"): void;
}>();

const open = defineModel<boolean>("open", { default: false });

const availableLayers = ref<
    { name: string; mod: string; isVanilla: boolean }[]
>([]);
const selectedNextLayer = ref("");
const loadingLayers = ref(false);
const settingNextLayer = ref(false);
const nextLayerComboboxOpen = ref(false);
const layersFetched = ref(false);

// Fetch available layers lazily on first open
watch(open, (isOpen) => {
    if (isOpen && !layersFetched.value) {
        layersFetched.value = true;
        fetchAvailableLayers();
    }
});

// Fetch available layers
async function fetchAvailableLayers() {
    loadingLayers.value = true;

    const runtimeConfig = useRuntimeConfig();

    try {
        interface LayersResponse {
            data: {
                layers: { name: string; mod: string; isVanilla: boolean }[];
            };
        }

        const responseData = await useAuthFetchImperative<LayersResponse>(
            `${runtimeConfig.public.backendApi}/servers/${props.serverId}/rcon/available-layers`,
        );

        if (responseData && responseData.data) {
            availableLayers.value = responseData.data.layers || [];
        }
    } catch (err: any) {
        toast({
            title: "Error",
            description: extractApiErrorMessage(err, "Failed to fetch available layers"),
            variant: "destructive",
        });
        console.error(err);
    } finally {
        loadingLayers.value = false;
    }
}

// Set next layer function
async function setNextLayer() {
    if (!selectedNextLayer.value) {
        toast({
            title: "Error",
            description: "Please select a layer",
            variant: "destructive",
        });
        return;
    }

    settingNextLayer.value = true;

    const runtimeConfig = useRuntimeConfig();

    try {
        interface LayerChangeResponse {
            data: {
                success: boolean;
                message?: string;
            };
        }

        const { data: responseData, error: fetchError } =
            await useAuthFetch<LayerChangeResponse>(
                `${runtimeConfig.public.backendApi}/servers/${props.serverId}/rcon/set-next-layer`,
                {
                    method: "POST",
                    body: {
                        layer: selectedNextLayer.value,
                    },
                },
            );

        if (fetchError.value) {
            throw new Error(
                extractApiErrorMessage(fetchError.value, "Failed to set next layer"),
            );
        }

        toast({
            title: "Success",
            description: "Next layer set successfully",
        });

        // Close dialog and let the page refresh
        open.value = false;
        emit("completed");
    } catch (err: any) {
        toast({
            title: "Error",
            description: extractApiErrorMessage(err, "Failed to set next layer"),
            variant: "destructive",
        });
        console.error(err);
    } finally {
        settingNextLayer.value = false;
    }
}
</script>

<template>
    <Dialog v-model:open="open">
        <DialogContent class="sm:max-w-[425px]">
            <DialogHeader>
                <DialogTitle>Set Next Layer</DialogTitle>
                <DialogDescription>
                    Select the next layer to be loaded after the current
                    match ends.
                </DialogDescription>
            </DialogHeader>

            <div class="py-4">
                <div v-if="loadingLayers" class="text-center py-4">
                    <div
                        class="animate-spin h-6 w-6 border-4 border-primary border-t-transparent rounded-full mx-auto mb-2"
                    ></div>
                    <p class="text-sm">Loading available layers...</p>
                </div>
                <div
                    v-else-if="availableLayers.length === 0"
                    class="text-center py-4"
                >
                    <p class="text-sm text-muted-foreground">
                        No layers available
                    </p>
                </div>
                <div v-else>
                    <div class="space-y-4">
                        <div class="space-y-2">
                            <label class="text-sm font-medium">Select Next Layer</label>
                            <Popover v-model:open="nextLayerComboboxOpen">
                                <PopoverTrigger as-child>
                                    <Button
                                        variant="outline"
                                        role="combobox"
                                        :aria-expanded="nextLayerComboboxOpen"
                                        class="w-full justify-between"
                                    >
                                        {{ selectedNextLayer || "Search for a layer..." }}
                                        <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="ml-2 h-4 w-4 shrink-0 opacity-50"><path d="m7 15 5 5 5-5"/><path d="m7 9 5-5 5 5"/></svg>
                                    </Button>
                                </PopoverTrigger>
                                <PopoverContent class="w-[--reka-popover-trigger-width] p-0">
                                    <Command>
                                        <CommandInput placeholder="Search layers..." />
                                        <CommandEmpty>No layer found.</CommandEmpty>
                                        <CommandList>
                                            <CommandGroup>
                                                <CommandItem
                                                    v-for="layer in availableLayers"
                                                    :key="layer.name"
                                                    :value="layer.name"
                                                    @select="selectedNextLayer = layer.name; nextLayerComboboxOpen = false"
                                                >
                                                    <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="mr-2 h-4 w-4" :class="selectedNextLayer === layer.name ? 'opacity-100' : 'opacity-0'"><path d="M20 6 9 17l-5-5"/></svg>
                                                    {{ layer.name }}
                                                </CommandItem>
                                            </CommandGroup>
                                        </CommandList>
                                    </Command>
                                </PopoverContent>
                            </Popover>
                        </div>
                        <div
                            v-if="currentNextLayer"
                            class="p-3 bg-blue-50 rounded-md"
                        >
                            <p class="text-sm font-medium text-blue-900">
                                Current Next Layer:
                            </p>
                            <p class="text-sm text-blue-700">
                                {{ currentNextLayer }}
                            </p>
                        </div>
                    </div>
                </div>
            </div>

            <DialogFooter>
                <Button variant="outline" @click="open = false">Cancel</Button>
                <Button
                    @click="setNextLayer"
                    :disabled="
                        loadingLayers ||
                        settingNextLayer ||
                        !selectedNextLayer
                    "
                >
                    {{ settingNextLayer ? "Setting..." : "Set Next Layer" }}
                </Button>
            </DialogFooter>
        </DialogContent>
    </Dialog>
</template>
