<script setup lang="ts">
import { ref } from "vue";
import { Button } from "~/components/ui/button";
import {
    Dialog,
    DialogContent,
    DialogDescription,
    DialogFooter,
    DialogHeader,
    DialogTitle,
} from "~/components/ui/dialog";
import { toast } from "~/components/ui/toast";

const props = defineProps<{
    serverId: string;
}>();

const emit = defineEmits<{
    (e: "completed"): void;
}>();

const open = defineModel<boolean>("open", { default: false });

const endingMatch = ref(false);

// End match function
async function endMatch() {
    endingMatch.value = true;

    const runtimeConfig = useRuntimeConfig();

    try {
        interface EndMatchResponse {
            data: {
                success: boolean;
                message?: string;
            };
        }

        const { data: responseData, error: fetchError } =
            await useAuthFetch<EndMatchResponse>(
                `${runtimeConfig.public.backendApi}/servers/${props.serverId}/rcon/execute`,
                {
                    method: "POST",
                    body: {
                        command: `AdminEndMatch`,
                    },
                },
            );

        if (fetchError.value) {
            throw new Error(extractApiErrorMessage(fetchError.value, "Failed to end match"));
        }

        toast({
            title: "Success",
            description: "Match ended successfully",
        });

        // Close dialog and let the page refresh
        open.value = false;
        emit("completed");
    } catch (err: any) {
        toast({
            title: "Error",
            description: extractApiErrorMessage(err, "Failed to end match"),
            variant: "destructive",
        });
        console.error(err);
    } finally {
        endingMatch.value = false;
    }
}
</script>

<template>
    <Dialog v-model:open="open">
        <DialogContent class="sm:max-w-[425px]">
            <DialogHeader>
                <DialogTitle>End Current Match</DialogTitle>
                <DialogDescription>
                    Are you sure you want to end the current match? This
                    will immediately end the match and transition to the
                    next layer.
                </DialogDescription>
            </DialogHeader>

            <div class="py-4">
                <div
                    class="p-4 bg-yellow-50 border border-yellow-200 rounded-md"
                >
                    <p class="text-sm text-yellow-800">
                        <strong>Warning:</strong> This action cannot be
                        undone. All players will be moved to the next layer
                        immediately.
                    </p>
                </div>
            </div>

            <DialogFooter>
                <Button variant="outline" @click="open = false">Cancel</Button>
                <Button
                    variant="destructive"
                    @click="endMatch"
                    :disabled="endingMatch"
                >
                    {{ endingMatch ? "Ending..." : "End Match" }}
                </Button>
            </DialogFooter>
        </DialogContent>
    </Dialog>
</template>
