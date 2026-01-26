<script setup lang="ts">
import { computed, ref, watch, onMounted, onUnmounted } from "vue";
import { getMediaCategory, type MediaCategory } from "@/utils/mediaTypes";

interface EvidenceFile {
  file_path?: string | null;
  file_name?: string | null;
  file_size?: number | null;
  file_type?: string | null;
}

const props = defineProps<{
  open: boolean;
  file: EvidenceFile | null;
  files: EvidenceFile[];
  currentIndex: number;
  previewUrl: string;
}>();

const emit = defineEmits<{
  (e: "update:open", value: boolean): void;
  (e: "navigate", direction: "prev" | "next"): void;
  (e: "download"): void;
}>();

const textContent = ref<string>("");
const isLoadingText = ref(false);
const loadError = ref<string | null>(null);

const mediaCategory = computed<MediaCategory>(() => {
  return getMediaCategory(props.file?.file_type);
});

const hasPrevious = computed(() => props.currentIndex > 0);
const hasNext = computed(() => props.currentIndex < props.files.length - 1);

function formatFileSize(bytes: number): string {
  if (bytes === 0) return "0 B";
  const k = 1024;
  const sizes = ["B", "KB", "MB", "GB"];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + " " + sizes[i];
}

async function loadTextContent() {
  if (mediaCategory.value !== "text" || !props.previewUrl) return;

  isLoadingText.value = true;
  loadError.value = null;

  try {
    const response = await fetch(props.previewUrl, {
      credentials: "include",
    });
    if (!response.ok) throw new Error("Failed to load text content");
    textContent.value = await response.text();
  } catch (error) {
    loadError.value = "Failed to load file content";
    textContent.value = "";
  } finally {
    isLoadingText.value = false;
  }
}

function handleKeydown(event: KeyboardEvent) {
  if (!props.open) return;

  switch (event.key) {
    case "Escape":
      emit("update:open", false);
      break;
    case "ArrowLeft":
      if (hasPrevious.value) emit("navigate", "prev");
      break;
    case "ArrowRight":
      if (hasNext.value) emit("navigate", "next");
      break;
  }
}

function handleImageError() {
  loadError.value = "Failed to load image. File may be corrupted or inaccessible.";
}

function handleVideoError() {
  loadError.value = "Failed to load video. Format may not be supported by your browser.";
}

watch(
  () => [props.open, props.file],
  () => {
    if (props.open && mediaCategory.value === "text") {
      loadTextContent();
    }
    loadError.value = null;
  },
  { immediate: true }
);

onMounted(() => {
  window.addEventListener("keydown", handleKeydown);
});

onUnmounted(() => {
  window.removeEventListener("keydown", handleKeydown);
});
</script>

<template>
  <Dialog :open="open" @update:open="emit('update:open', $event)">
    <DialogScrollContent class="max-w-5xl max-h-[90vh]">
      <DialogHeader>
        <DialogTitle class="truncate pr-8">{{ file?.file_name || "Preview" }}</DialogTitle>
        <DialogDescription>
          {{ formatFileSize(file?.file_size || 0) }} &bull; {{ file?.file_type }}
        </DialogDescription>
      </DialogHeader>

      <div class="relative min-h-[300px] flex items-center justify-center bg-muted/30 rounded-lg">
        <!-- Error State -->
        <div v-if="loadError" class="text-center text-destructive p-8">
          <Icon name="lucide:alert-circle" class="w-12 h-12 mx-auto mb-4" />
          <p>{{ loadError }}</p>
          <Button variant="outline" class="mt-4" @click="emit('download')">
            <Icon name="lucide:download" class="w-4 h-4 mr-2" />
            Download Instead
          </Button>
        </div>

        <!-- Image Preview -->
        <img
          v-else-if="mediaCategory === 'image'"
          :src="previewUrl"
          :alt="file?.file_name || 'Image'"
          class="max-w-full max-h-[70vh] object-contain rounded"
          @error="handleImageError"
        />

        <!-- Video Preview -->
        <video
          v-else-if="mediaCategory === 'video'"
          :src="previewUrl"
          controls
          class="max-w-full max-h-[70vh] rounded"
          @error="handleVideoError"
        >
          Your browser does not support video playback.
        </video>

        <!-- PDF Preview -->
        <iframe
          v-else-if="mediaCategory === 'pdf'"
          :src="previewUrl"
          class="w-full h-[70vh] rounded border-0"
          title="PDF Preview"
        />

        <!-- Text Preview -->
        <div
          v-else-if="mediaCategory === 'text'"
          class="w-full h-[70vh] overflow-auto rounded"
        >
          <div v-if="isLoadingText" class="flex items-center justify-center h-full">
            <Icon name="lucide:loader-2" class="w-8 h-8 animate-spin text-muted-foreground" />
          </div>
          <pre
            v-else
            class="p-4 bg-muted text-sm font-mono whitespace-pre-wrap break-words"
          >{{ textContent }}</pre>
        </div>

        <!-- Unknown Type -->
        <div v-else class="text-center text-muted-foreground p-8">
          <Icon name="lucide:file" class="w-12 h-12 mx-auto mb-4" />
          <p>Preview not available for this file type</p>
          <Button variant="outline" class="mt-4" @click="emit('download')">
            <Icon name="lucide:download" class="w-4 h-4 mr-2" />
            Download File
          </Button>
        </div>

        <!-- Navigation Arrows -->
        <Button
          v-if="hasPrevious && !loadError"
          variant="secondary"
          size="icon"
          class="absolute left-2 top-1/2 -translate-y-1/2 opacity-80 hover:opacity-100"
          @click="emit('navigate', 'prev')"
        >
          <Icon name="lucide:chevron-left" class="w-6 h-6" />
        </Button>
        <Button
          v-if="hasNext && !loadError"
          variant="secondary"
          size="icon"
          class="absolute right-2 top-1/2 -translate-y-1/2 opacity-80 hover:opacity-100"
          @click="emit('navigate', 'next')"
        >
          <Icon name="lucide:chevron-right" class="w-6 h-6" />
        </Button>
      </div>

      <!-- Counter -->
      <div v-if="files.length > 1" class="text-center text-sm text-muted-foreground">
        {{ currentIndex + 1 }} of {{ files.length }}
      </div>

      <DialogFooter class="flex-row gap-2 sm:justify-end">
        <Button variant="outline" @click="emit('download')">
          <Icon name="lucide:download" class="w-4 h-4 mr-2" />
          Download
        </Button>
        <Button variant="outline" @click="emit('update:open', false)">
          Close
        </Button>
      </DialogFooter>
    </DialogScrollContent>
  </Dialog>
</template>
