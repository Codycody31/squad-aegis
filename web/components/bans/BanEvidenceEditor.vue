<script setup lang="ts">
import { ref, watch, nextTick } from "vue";
import { Badge } from "~/components/ui/badge";
import { Button } from "~/components/ui/button";
import { Input } from "~/components/ui/input";
import { Textarea } from "~/components/ui/textarea";
import {
    Select,
    SelectContent,
    SelectItem,
    SelectTrigger,
    SelectValue,
} from "~/components/ui/select";
import {
    Tabs,
    TabsList,
    TabsTrigger,
    TabsContent,
} from "~/components/ui/tabs";
import { toast } from "~/components/ui/toast";
import {
    cleanPlayerId,
    hasValidEvidencePlayerId,
    getClickhouseTableForType,
    formatEventDescription,
    getEvidenceDisplayPayload,
    getEvidenceDisplayTime,
    formatFileSize,
} from "~/utils/evidence";
import type {
    EventEvidenceDraft,
    FileEvidenceDraft,
    TextEvidenceDraft,
} from "~/types/bans";

const props = defineProps<{
    serverId: string;
    // Steam/EOS id used for /events/search
    playerId?: string | null;
    // Empty-state hint shown in the events tab before a search
    searchHint?: string;
}>();

const events = defineModel<EventEvidenceDraft[]>("events", { default: () => [] });
const files = defineModel<FileEvidenceDraft[]>("files", { default: () => [] });
const texts = defineModel<TextEvidenceDraft[]>("texts", { default: () => [] });

const runtimeConfig = useRuntimeConfig();

const evidenceTab = ref("events"); // 'events', 'files', 'text'
const evidenceSearchType = ref("chat_message");
const evidenceSearchResults = ref<any[]>([]);
const evidenceSearchSteamId = ref("");
const isSearchingEvidence = ref(false);
const evidenceText = ref("");
const isUploadingFile = ref(false);

// Function to search evidence inline (no longer opens a dialog)
async function searchEvidenceInline(steamId: string) {
    try {
        const playerIds = cleanPlayerId(steamId);
        const cleanId = playerIds.rconId;
        evidenceSearchSteamId.value = cleanId;
        isSearchingEvidence.value = true;
        evidenceSearchResults.value = [];

        const params = new URLSearchParams({
            event_type: evidenceSearchType.value,
        });
        if (playerIds.steamId) {
            params.set("steam_id", playerIds.steamId);
        }
        if (playerIds.eosId) {
            params.set("eos_id", playerIds.eosId);
        }

        const { data, error: fetchError } = await useAuthFetch(
            `${runtimeConfig.public.backendApi}/servers/${props.serverId}/events/search?${params.toString()}`,
            {
                method: "GET",
            },
        );

        if (fetchError.value) {
            throw new Error(
                extractApiErrorMessage(fetchError.value, "Failed to search for evidence"),
            );
        }

        if (data.value && (data.value as any).data) {
            evidenceSearchResults.value = (data.value as any).data.events || [];
        }
    } catch (err: any) {
        console.error("Failed to search evidence:", err);
        toast({
            title: "Error",
            description: extractApiErrorMessage(err, "Failed to search for evidence"),
            variant: "destructive",
        });
    } finally {
        isSearchingEvidence.value = false;
    }
}

// Watch for evidence search type changes and clear results
watch(evidenceSearchType, () => {
    evidenceSearchResults.value = [];
});

// Function to toggle evidence selection
function toggleEvidenceSelection(event: any) {
    // Use record_id if available (from fixed API), otherwise fall back to event_id/message_id
    const recordId = event.record_id || event.event_id || event.message_id;

    const index = events.value.findIndex(
        (e) => e.record_id === recordId,
    );

    if (index > -1) {
        events.value.splice(index, 1);
    } else {
        events.value.push({
            evidence_type: evidenceSearchType.value,
            clickhouse_table: getClickhouseTableForType(evidenceSearchType.value),
            record_id: recordId,
            event_time: event.event_time || event.sent_at,
            metadata: event,
        });
    }
}

// Function to check if evidence is selected
function isEvidenceSelected(event: any): boolean {
    // Use record_id if available (from fixed API), otherwise fall back to event_id/message_id
    const recordId = event.record_id || event.event_id || event.message_id;
    return events.value.some((e) => e.record_id === recordId);
}

// Function to handle file upload
async function handleFileUpload(event: Event) {
    const target = event.target as HTMLInputElement;
    const fileList = target.files;
    if (!fileList || fileList.length === 0) return;

    isUploadingFile.value = true;

    try {
        for (const file of Array.from(fileList)) {
            const formData = new FormData();
            formData.append("file", file);

            const response = await useAuthFetchImperative<any>(
                `${runtimeConfig.public.backendApi}/servers/${props.serverId}/evidence/upload`,
                {
                    method: "POST",
                    body: formData,
                },
            );

            if (response && response.data) {
                const fileData = response.data;
                files.value.push({
                    evidence_type: "file_upload",
                    file_path: fileData.file_path,
                    file_name: fileData.file_name,
                    file_size: fileData.file_size,
                    file_type: fileData.file_type,
                    file_id: fileData.file_id,
                });
            }
        }

        toast({
            title: "Success",
            description: `Uploaded ${fileList.length} file(s) successfully`,
        });
    } catch (err: any) {
        console.error("File upload error:", err);
        toast({
            title: "Error",
            description: extractApiErrorMessage(err, "Failed to upload file"),
            variant: "destructive",
        });
    } finally {
        isUploadingFile.value = false;
        // Reset file input safely using nextTick to avoid DOM issues
        // Wait for Vue to finish its update cycle before resetting
        nextTick(() => {
            try {
                if (target && target.value !== undefined) {
                    target.value = "";
                }
            } catch (resetErr) {
                // Ignore errors when resetting - element might have been removed or is no longer usable
                // This is safe to ignore as the input will reset naturally on next interaction
                console.debug("File input reset skipped (safe to ignore):", resetErr);
            }
        });
    }
}

// Function to remove uploaded file
function removeUploadedFile(index: number) {
    files.value.splice(index, 1);
}

// Function to add text evidence
function addTextEvidence() {
    if (!evidenceText.value.trim()) {
        toast({
            title: "Error",
            description: "Please enter some text",
            variant: "destructive",
        });
        return;
    }

    texts.value.push({
        evidence_type: "text_paste",
        text_content: evidenceText.value.trim(),
    });

    evidenceText.value = "";

    toast({
        title: "Success",
        description: "Text evidence added",
    });
}

// Function to remove text evidence
function removeTextEvidence(index: number) {
    texts.value.splice(index, 1);
}
</script>

<template>
    <!-- Evidence Section with Tabs -->
    <div class="border rounded-md p-3 bg-muted/50">
        <h4 class="font-medium text-sm mb-3">Attached Evidence</h4>
        <Tabs v-model="evidenceTab" class="w-full">
            <TabsList class="grid w-full grid-cols-3">
                <TabsTrigger value="events">
                    Events
                    <Badge v-if="events.length > 0" variant="secondary" class="ml-2">
                        {{ events.length }}
                    </Badge>
                </TabsTrigger>
                <TabsTrigger value="files">
                    Files
                    <Badge v-if="files.length > 0" variant="secondary" class="ml-2">
                        {{ files.length }}
                    </Badge>
                </TabsTrigger>
                <TabsTrigger value="text">
                    Text
                    <Badge v-if="texts.length > 0" variant="secondary" class="ml-2">
                        {{ texts.length }}
                    </Badge>
                </TabsTrigger>
            </TabsList>

            <!-- Events Tab -->
            <TabsContent value="events" class="mt-4">
                <div class="space-y-3">
                    <!-- Search Controls -->
                    <div class="flex flex-col sm:flex-row gap-2">
                        <Select v-model="evidenceSearchType" class="w-full sm:w-[180px]">
                            <SelectTrigger class="text-xs sm:text-sm">
                                <SelectValue placeholder="Event Type" />
                            </SelectTrigger>
                            <SelectContent>
                                <SelectItem value="chat_message" class="text-xs sm:text-sm">Chat Messages</SelectItem>
                                <SelectItem value="player_connected" class="text-xs sm:text-sm">Connections</SelectItem>
                            </SelectContent>
                        </Select>
                        <Button
                            type="button"
                            @click="searchEvidenceInline(playerId || '')"
                            :disabled="!hasValidEvidencePlayerId(playerId) || isSearchingEvidence"
                            class="flex-1 text-xs sm:text-sm"
                        >
                            <Icon v-if="isSearchingEvidence" name="mdi:loading" class="h-3 w-3 sm:h-4 sm:w-4 sm:mr-2 animate-spin" />
                            <Icon v-else name="lucide:search" class="h-3 w-3 sm:h-4 sm:w-4 sm:mr-2" />
                            <span class="hidden sm:inline">{{ isSearchingEvidence ? "Searching..." : "Search Events" }}</span>
                            <span class="sm:hidden">{{ isSearchingEvidence ? "Searching..." : "Search" }}</span>
                        </Button>
                    </div>

                    <!-- Search Results -->
                    <div v-if="evidenceSearchResults.length > 0" class="border rounded-md max-h-[300px] overflow-y-auto">
                        <div class="divide-y">
                            <div
                                v-for="event in evidenceSearchResults"
                                :key="event.record_id || event.event_id || event.message_id"
                                class="p-3 hover:bg-muted/50 cursor-pointer transition-colors"
                                :class="{ 'bg-primary/10': isEvidenceSelected(event) }"
                                @click="toggleEvidenceSelection(event)"
                            >
                                <div class="flex items-start justify-between">
                                    <div class="flex-1">
                                        <div class="font-medium text-sm">
                                            {{ formatEventDescription(event, evidenceSearchType) }}
                                        </div>
                                        <div class="text-xs text-muted-foreground mt-1">
                                            {{ new Date(event.event_time || event.sent_at).toLocaleString() }}
                                        </div>
                                        <div v-if="event.teamkill" class="mt-1">
                                            <Badge variant="destructive" class="text-xs">TEAMKILL</Badge>
                                        </div>
                                    </div>
                                    <div v-if="isEvidenceSelected(event)" class="ml-2">
                                        <Icon name="lucide:check-circle" class="h-5 w-5 text-primary" />
                                    </div>
                                </div>
                            </div>
                        </div>
                    </div>

                    <!-- Selected Events -->
                    <div v-if="events.length > 0" class="space-y-2">
                        <div class="text-sm font-medium">Selected Events ({{ events.length }})</div>
                        <div class="space-y-2">
                            <div
                                v-for="(evidence, idx) in events"
                                :key="`event-${idx}`"
                                class="flex items-center justify-between text-sm p-2 bg-background rounded border"
                            >
                                <div class="flex-1">
                                    <div class="font-medium">{{ formatEventDescription(getEvidenceDisplayPayload(evidence), evidence.evidence_type) }}</div>
                                    <div v-if="getEvidenceDisplayTime(evidence)" class="text-xs text-muted-foreground">
                                        {{ new Date(getEvidenceDisplayTime(evidence) || "").toLocaleString() }}
                                    </div>
                                </div>
                                <Button
                                    type="button"
                                    variant="ghost"
                                    size="sm"
                                    @click="events.splice(idx, 1)"
                                >
                                    <Icon name="lucide:x" class="h-4 w-4" />
                                </Button>
                            </div>
                        </div>
                    </div>

                    <!-- Empty State -->
                    <div v-if="evidenceSearchResults.length === 0 && events.length === 0 && !isSearchingEvidence" class="text-sm text-muted-foreground text-center py-4">
                        {{ searchHint || 'Click "Search Events" to find game events for this player' }}
                    </div>
                </div>
            </TabsContent>

            <!-- Files Tab -->
            <TabsContent value="files" class="mt-4">
                <div class="space-y-3">
                    <div class="flex items-center gap-2">
                        <Input
                            type="file"
                            accept="image/*,video/*,.pdf,.txt"
                            @change="handleFileUpload"
                            :disabled="isUploadingFile"
                            class="flex-1"
                            multiple
                        />
                    </div>
                    <div v-if="files.length > 0" class="space-y-2">
                        <div class="text-sm font-medium">Uploaded Files ({{ files.length }})</div>
                        <div class="space-y-1">
                            <div
                                v-for="(file, idx) in files"
                                :key="`file-${idx}`"
                                class="flex items-center justify-between text-sm p-2 bg-background rounded border"
                            >
                                <div class="flex-1">
                                    <div class="font-medium">{{ file.file_name }}</div>
                                    <div class="text-xs text-muted-foreground">
                                        {{ formatFileSize(file.file_size) }} • {{ file.file_type }}
                                    </div>
                                </div>
                                <Button
                                    type="button"
                                    variant="ghost"
                                    size="sm"
                                    @click="removeUploadedFile(idx)"
                                >
                                    <Icon name="lucide:x" class="h-4 w-4" />
                                </Button>
                            </div>
                        </div>
                    </div>
                    <div v-else class="text-sm text-muted-foreground text-center py-4">
                        No files uploaded. Select files above to upload.
                    </div>
                </div>
            </TabsContent>

            <!-- Text Tab -->
            <TabsContent value="text" class="mt-4">
                <div class="space-y-3">
                    <div class="flex items-center gap-2">
                        <Textarea
                            v-model="evidenceText"
                            placeholder="Paste text evidence here..."
                            rows="4"
                            class="flex-1"
                        />
                        <Button
                            type="button"
                            variant="outline"
                            @click="addTextEvidence"
                            :disabled="!evidenceText.trim()"
                        >
                            <Icon name="lucide:plus" class="h-4 w-4 mr-1" />
                            Add
                        </Button>
                    </div>
                    <div v-if="texts.length > 0" class="space-y-2">
                        <div class="text-sm font-medium">Text Evidence ({{ texts.length }})</div>
                        <div class="space-y-1">
                            <div
                                v-for="(text, idx) in texts"
                                :key="`text-${idx}`"
                                class="flex items-start justify-between text-sm p-2 bg-background rounded border"
                            >
                                <div class="flex-1">
                                    <div class="font-medium">Text Evidence</div>
                                    <div class="text-xs text-muted-foreground mt-1">
                                        {{ text.text_content.length > 100 ? text.text_content.substring(0, 100) + "..." : text.text_content }}
                                    </div>
                                </div>
                                <Button
                                    type="button"
                                    variant="ghost"
                                    size="sm"
                                    @click="removeTextEvidence(idx)"
                                >
                                    <Icon name="lucide:x" class="h-4 w-4" />
                                </Button>
                            </div>
                        </div>
                    </div>
                    <div v-else class="text-sm text-muted-foreground text-center py-4">
                        No text evidence added. Paste text above and click "Add".
                    </div>
                </div>
            </TabsContent>
        </Tabs>
    </div>
</template>
