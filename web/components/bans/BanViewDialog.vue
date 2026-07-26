<script setup lang="ts">
import { Button } from "~/components/ui/button";
import {
    Dialog,
    DialogContent,
    DialogDescription,
    DialogFooter,
    DialogHeader,
    DialogTitle,
} from "~/components/ui/dialog";
import BanEvidenceViewer from "~/components/bans/BanEvidenceViewer.vue";
import { formatDate, formatExpiresAt } from "~/utils/evidence";
import type { BannedPlayer } from "~/types/bans";

defineProps<{
    ban: BannedPlayer | null;
}>();

const open = defineModel<boolean>("open", { default: false });
</script>

<template>
    <!-- Evidence View Dialog -->
    <Dialog v-model:open="open">
        <DialogContent class="w-[95vw] sm:max-w-[900px] max-h-[90vh] overflow-y-auto p-4 sm:p-6">
            <DialogHeader>
                <DialogTitle class="text-base sm:text-lg">Ban Evidence</DialogTitle>
                <DialogDescription class="text-xs sm:text-sm">
                    View all evidence linked to this ban
                </DialogDescription>
            </DialogHeader>
            <div v-if="ban" class="space-y-4">
                <!-- Ban Info -->
                <div class="border rounded-md p-3 bg-muted/50">
                    <div class="grid grid-cols-2 gap-2 text-sm">
                        <div v-if="ban.steam_id">
                            <span class="text-muted-foreground">Steam ID:</span>
                            <span class="ml-2 font-medium">{{ ban.steam_id }}</span>
                        </div>
                        <div v-if="ban.eos_id">
                            <span class="text-muted-foreground">EOS ID:</span>
                            <span class="ml-2 font-medium">{{ ban.eos_id }}</span>
                        </div>
                        <div>
                            <span class="text-muted-foreground">Reason:</span>
                            <span class="ml-2 font-medium">{{ ban.reason }}</span>
                        </div>
                        <div>
                            <span class="text-muted-foreground">Banned At:</span>
                            <span class="ml-2">{{ formatDate(ban.created_at) }}</span>
                        </div>
                        <div>
                            <span class="text-muted-foreground">Duration:</span>
                            <span class="ml-2">
                                {{ formatExpiresAt(ban) }}
                            </span>
                        </div>
                    </div>
                </div>

                <!-- Evidence Description -->
                <div v-if="ban.evidence_text" class="border rounded-md p-3">
                    <h4 class="font-medium text-sm mb-2">Evidence Description</h4>
                    <p class="text-sm text-muted-foreground whitespace-pre-wrap">{{ ban.evidence_text }}</p>
                </div>

                <BanEvidenceViewer :ban="ban" />
            </div>
            <DialogFooter>
                <Button variant="outline" @click="open = false">
                    Close
                </Button>
            </DialogFooter>
        </DialogContent>
    </Dialog>
</template>
