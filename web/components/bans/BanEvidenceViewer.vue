<script setup lang="ts">
import { ref, computed } from "vue";
import { Badge } from "~/components/ui/badge";
import { Button } from "~/components/ui/button";
import {
    Tabs,
    TabsList,
    TabsTrigger,
    TabsContent,
} from "~/components/ui/tabs";
import { toast } from "~/components/ui/toast";
import { canPreview, getMediaCategory } from "~/utils/mediaTypes";
import {
    formatEventDescription,
    getEvidenceDisplayPayload,
    getEvidenceDisplayTime,
    formatFileSize,
} from "~/utils/evidence";
import EvidenceMediaPreviewModal from "~/components/evidence/MediaPreviewModal.vue";
import type { BanEvidence, BannedPlayer } from "~/types/bans";

const props = defineProps<{
    ban: BannedPlayer;
}>();

// Evidence files are served under the server the page is scoped to,
// matching the original page behavior.
const route = useRoute();
const runtimeConfig = useRuntimeConfig();
const serverId = Array.isArray(route.params.serverId)
    ? route.params.serverId[0]
    : route.params.serverId;

// Media preview state
const showMediaPreviewModal = ref(false);
const previewingFile = ref<BanEvidence | null>(null);
const previewFileIndex = ref(0);

// Function to get evidence count by type
function getEvidenceCounts(ban: BannedPlayer | null) {
    if (!ban || !ban.evidence) {
        return { events: 0, files: 0, text: 0 };
    }
    const events = ban.evidence.filter((e) =>
        e.evidence_type !== "file_upload" && e.evidence_type !== "text_paste"
    ).length;
    const files = ban.evidence.filter((e) => e.evidence_type === "file_upload").length;
    const text = ban.evidence.filter((e) => e.evidence_type === "text_paste").length;
    return { events, files, text };
}

// Function to download evidence file
async function downloadEvidenceFile(filePath: string, fileName: string) {
    try {
        // Extract file ID from path (last part before extension)
        const fileId = filePath.split('/').pop()?.split('.')[0];
        if (!fileId) {
            throw new Error("Invalid file path");
        }

        const url = `${runtimeConfig.public.backendApi}/servers/${serverId}/evidence/files/${fileId}`;
        const blob = await useAuthFetchImperative<Blob>(url, {
            responseType: "blob",
        });
        const downloadUrl = window.URL.createObjectURL(blob);
        const link = document.createElement('a');
        link.href = downloadUrl;
        link.download = fileName;
        document.body.appendChild(link);
        link.click();
        document.body.removeChild(link);
        window.URL.revokeObjectURL(downloadUrl);
    } catch (err: any) {
        console.error("File download error:", err);
        toast({
            title: "Error",
            description: extractApiErrorMessage(err, "Failed to download file"),
            variant: "destructive",
        });
    }
}

// Function to get the previewable files from the current ban evidence
const previewableFiles = computed(() => {
    if (!props.ban?.evidence) return [];
    return props.ban.evidence.filter(
        (e) => e.evidence_type === "file_upload"
    );
});

// Function to get preview URL for a file
function getPreviewUrl(filePath: string): string {
    const fileId = filePath.split("/").pop()?.split(".")[0];
    return `${runtimeConfig.public.backendApi}/servers/${serverId}/evidence/files/${fileId}?inline=true`;
}

// Function to open media preview modal
function openMediaPreviewModal(evidence: BanEvidence, index: number) {
    previewingFile.value = evidence;
    previewFileIndex.value = index;
    showMediaPreviewModal.value = true;
}

// Function to navigate between files in preview
function navigatePreview(direction: "prev" | "next") {
    const files = previewableFiles.value;
    if (direction === "next" && previewFileIndex.value < files.length - 1) {
        previewFileIndex.value++;
        previewingFile.value = files[previewFileIndex.value];
    } else if (direction === "prev" && previewFileIndex.value > 0) {
        previewFileIndex.value--;
        previewingFile.value = files[previewFileIndex.value];
    }
}

// Function to download the currently previewed file
function downloadPreviewedFile() {
    if (previewingFile.value?.file_path && previewingFile.value?.file_name) {
        downloadEvidenceFile(previewingFile.value.file_path, previewingFile.value.file_name);
    }
}
</script>

<template>
    <!-- Evidence Tabs -->
    <Tabs value="events" class="w-full">
        <TabsList class="grid w-full grid-cols-3">
            <TabsTrigger value="events" class="text-xs sm:text-sm">
                <span class="hidden sm:inline">Events</span>
                <span class="sm:hidden">Events</span>
                <Badge v-if="getEvidenceCounts(ban).events > 0" variant="secondary" class="ml-1 sm:ml-2 text-xs">
                    {{ getEvidenceCounts(ban).events }}
                </Badge>
            </TabsTrigger>
            <TabsTrigger value="files" class="text-xs sm:text-sm">
                <span class="hidden sm:inline">Files</span>
                <span class="sm:hidden">Files</span>
                <Badge v-if="getEvidenceCounts(ban).files > 0" variant="secondary" class="ml-1 sm:ml-2 text-xs">
                    {{ getEvidenceCounts(ban).files }}
                </Badge>
            </TabsTrigger>
            <TabsTrigger value="text" class="text-xs sm:text-sm">
                <span class="hidden sm:inline">Text</span>
                <span class="sm:hidden">Text</span>
                <Badge v-if="getEvidenceCounts(ban).text > 0" variant="secondary" class="ml-1 sm:ml-2 text-xs">
                    {{ getEvidenceCounts(ban).text }}
                </Badge>
            </TabsTrigger>
        </TabsList>

        <!-- Events Tab -->
        <TabsContent value="events" class="mt-4">
            <div v-if="ban.evidence && ban.evidence.filter(e => e.evidence_type !== 'file_upload' && e.evidence_type !== 'text_paste').length > 0" class="space-y-2">
                <div
                    v-for="(evidence, idx) in ban.evidence.filter(e => e.evidence_type !== 'file_upload' && e.evidence_type !== 'text_paste')"
                    :key="`event-${idx}`"
                    class="border rounded-md p-3"
                >
                    <div class="flex items-start justify-between">
                        <div class="flex-1">
                            <div class="font-medium text-sm mb-1">
                                {{ formatEventDescription(getEvidenceDisplayPayload(evidence), evidence.evidence_type) }}
                            </div>
                            <div class="text-xs text-muted-foreground space-y-1">
                                <div>Type: {{ evidence.evidence_type }}</div>
                                <div v-if="getEvidenceDisplayTime(evidence)">
                                    Time: {{ new Date(getEvidenceDisplayTime(evidence) || "").toLocaleString() }}
                                </div>
                                <div v-if="getEvidenceDisplayPayload(evidence).teamkill">
                                    <Badge variant="destructive" class="text-xs">TEAMKILL</Badge>
                                </div>
                            </div>
                        </div>
                    </div>
                </div>
            </div>
            <div v-else class="text-sm text-muted-foreground text-center py-8">
                No event evidence attached to this ban.
            </div>
        </TabsContent>

        <!-- Files Tab -->
        <TabsContent value="files" class="mt-4">
            <div v-if="ban.evidence && ban.evidence.filter(e => e.evidence_type === 'file_upload').length > 0" class="space-y-2">
                <div
                    v-for="(evidence, idx) in ban.evidence.filter(e => e.evidence_type === 'file_upload')"
                    :key="`file-${idx}`"
                    class="border rounded-md p-3"
                >
                    <div class="flex items-start gap-3">
                        <!-- Thumbnail Preview -->
                        <div
                            v-if="canPreview(evidence.file_type)"
                            class="w-16 h-16 rounded cursor-pointer overflow-hidden bg-muted flex-shrink-0 flex items-center justify-center"
                            @click="openMediaPreviewModal(evidence, idx)"
                        >
                            <img
                                v-if="getMediaCategory(evidence.file_type) === 'image'"
                                :src="getPreviewUrl(evidence.file_path ?? '')"
                                :alt="evidence.file_name ?? 'Image'"
                                class="w-full h-full object-cover"
                            />
                            <Icon
                                v-else-if="getMediaCategory(evidence.file_type) === 'video'"
                                name="lucide:play-circle"
                                class="w-8 h-8 text-muted-foreground"
                            />
                            <Icon
                                v-else-if="getMediaCategory(evidence.file_type) === 'pdf'"
                                name="lucide:file-text"
                                class="w-8 h-8 text-red-500"
                            />
                            <Icon
                                v-else-if="getMediaCategory(evidence.file_type) === 'text'"
                                name="lucide:file-code"
                                class="w-8 h-8 text-blue-500"
                            />
                        </div>
                        <div v-else class="w-16 h-16 rounded bg-muted flex items-center justify-center flex-shrink-0">
                            <Icon name="lucide:file" class="w-8 h-8 text-muted-foreground" />
                        </div>

                        <!-- File Info -->
                        <div class="flex-1 min-w-0">
                            <div class="font-medium text-sm truncate">{{ evidence.file_name }}</div>
                            <div class="text-xs text-muted-foreground mt-1">
                                {{ formatFileSize(evidence.file_size || 0) }} &bull; {{ evidence.file_type }}
                            </div>
                        </div>

                        <!-- Actions -->
                        <div class="flex gap-2 flex-shrink-0">
                            <Button
                                v-if="canPreview(evidence.file_type)"
                                type="button"
                                variant="ghost"
                                size="sm"
                                @click="openMediaPreviewModal(evidence, idx)"
                                title="Preview"
                            >
                                <Icon name="lucide:eye" class="h-4 w-4" />
                            </Button>
                            <Button
                                type="button"
                                variant="outline"
                                size="sm"
                                @click="downloadEvidenceFile(evidence.file_path ?? '', evidence.file_name ?? 'file')"
                                title="Download"
                            >
                                <Icon name="lucide:download" class="h-4 w-4" />
                            </Button>
                        </div>
                    </div>
                </div>
            </div>
            <div v-else class="text-sm text-muted-foreground text-center py-8">
                No file evidence attached to this ban.
            </div>
        </TabsContent>

        <!-- Text Tab -->
        <TabsContent value="text" class="mt-4">
            <div v-if="ban.evidence && ban.evidence.filter(e => e.evidence_type === 'text_paste').length > 0" class="space-y-2">
                <div
                    v-for="(evidence, idx) in ban.evidence.filter(e => e.evidence_type === 'text_paste')"
                    :key="`text-${idx}`"
                    class="border rounded-md p-3"
                >
                    <div class="font-medium text-sm mb-2">Text Evidence</div>
                    <div class="text-sm text-muted-foreground whitespace-pre-wrap bg-muted/50 p-2 rounded">
                        {{ evidence.text_content }}
                    </div>
                </div>
            </div>
            <div v-else class="text-sm text-muted-foreground text-center py-8">
                No text evidence attached to this ban.
            </div>
        </TabsContent>
    </Tabs>

    <!-- Media Preview Modal -->
    <EvidenceMediaPreviewModal
        :open="showMediaPreviewModal"
        :file="previewingFile"
        :files="previewableFiles"
        :current-index="previewFileIndex"
        :preview-url="previewingFile?.file_path ? getPreviewUrl(previewingFile.file_path) : ''"
        @update:open="showMediaPreviewModal = $event"
        @navigate="navigatePreview"
        @download="downloadPreviewedFile"
    />
</template>
