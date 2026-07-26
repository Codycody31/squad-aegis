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
import { Input } from "~/components/ui/input";
import { Label } from "~/components/ui/label";
import { Textarea } from "~/components/ui/textarea";
import { toast } from "~/components/ui/toast";
import type { WorkflowFormValues } from "~/components/workflows/types";

const emit = defineEmits<{
  /** Fired with the parsed form values; the parent opens the create dialog. */
  (e: "imported", values: WorkflowFormValues): void;
}>();

const open = defineModel<boolean>("open", { default: false });

const isImporting = ref(false);
const importJsonText = ref("");
const importError = ref("");
const importFile = ref<File | null>(null);

// Reset state whenever the dialog opens
watch(open, (isOpen) => {
  if (!isOpen) return;
  importJsonText.value = "";
  importError.value = "";
  importFile.value = null;
});

function closeImportDialog() {
  open.value = false;
  importJsonText.value = "";
  importError.value = "";
  importFile.value = null;
}

function handleFileUpload(event: Event) {
  const target = event.target as HTMLInputElement;
  const file = target.files?.[0];

  if (!file) return;

  importFile.value = file;

  const reader = new FileReader();
  reader.onload = (e) => {
    const content = e.target?.result as string;
    importJsonText.value = content;
  };
  reader.readAsText(file);
}

async function importWorkflow() {
  importError.value = "";
  isImporting.value = true;

  if (!importJsonText.value.trim()) {
    importError.value = "Please provide JSON content";
    isImporting.value = false;
    return;
  }

  try {
    const parsed = JSON.parse(importJsonText.value);

    // Validate required structure
    if (!parsed.version) {
      importError.value = "Missing required field: version";
      isImporting.value = false;
      return;
    }

    if (!Array.isArray(parsed.triggers)) {
      importError.value = "Missing or invalid triggers array";
      isImporting.value = false;
      return;
    }

    if (!Array.isArray(parsed.steps)) {
      importError.value = "Missing or invalid steps array";
      isImporting.value = false;
      return;
    }

    const values: WorkflowFormValues = {
      name: parsed.name || "",
      description: parsed.description || "",
      enabled: typeof parsed.enabled === "boolean" ? parsed.enabled : true,
      definition: {
        version: parsed.version,
        triggers: parsed.triggers || [],
        variables: parsed.variables || {},
        steps: parsed.steps || [],
        error_handling: parsed.error_handling || {
          default_action: "stop",
          max_retries: 3,
          retry_delay_ms: 1000,
        },
      },
    };

    closeImportDialog();
    emit("imported", values);

    toast({
      title: "Workflow Imported",
      description:
        "Workflow has been imported successfully. Review and create it.",
    });
  } catch (error) {
    if (error instanceof SyntaxError) {
      importError.value = `Invalid JSON: ${error.message}`;
    } else {
      importError.value = `Import error: ${
        error instanceof Error ? error.message : "Unknown error"
      }`;
    }
  } finally {
    isImporting.value = false;
  }
}
</script>

<template>
  <Dialog v-model:open="open">
    <DialogContent
      class="max-h-[80vh] w-[95vw] overflow-y-auto p-4 sm:max-w-3xl sm:p-6"
    >
      <DialogHeader>
        <DialogTitle>Import Workflow</DialogTitle>
        <DialogDescription>
          Import a workflow from a JSON file or paste JSON directly
        </DialogDescription>
      </DialogHeader>

      <div class="space-y-4">
        <!-- File Upload Section -->
        <div class="space-y-2">
          <Label for="workflow-file">Upload JSON File</Label>
          <div class="flex items-center gap-2">
            <Input
              id="workflow-file"
              type="file"
              accept=".json,application/json"
              class="cursor-pointer"
              @change="handleFileUpload"
            />
          </div>
          <p class="text-xs text-muted-foreground">
            Select a JSON file containing workflow configuration
          </p>
        </div>

        <div class="relative">
          <div class="absolute inset-0 flex items-center">
            <span class="w-full border-t" />
          </div>
          <div class="relative flex justify-center text-xs uppercase">
            <span class="bg-background px-2 text-muted-foreground">
              Or paste JSON
            </span>
          </div>
        </div>

        <!-- Text Input Section -->
        <div class="space-y-2">
          <Label>Workflow JSON</Label>
          <Textarea
            v-model="importJsonText"
            placeholder="Paste your workflow JSON here..."
            rows="15"
            class="font-mono text-sm"
          />
          <p class="text-xs text-muted-foreground">
            Ensure the JSON includes version, triggers, steps, and variables
            fields
          </p>
        </div>

        <!-- Error Display -->
        <div
          v-if="importError"
          class="rounded-md border border-destructive/20 bg-destructive/10 p-3"
        >
          <p class="text-sm font-medium text-destructive">Import Error:</p>
          <p class="text-sm text-destructive">{{ importError }}</p>
        </div>
      </div>

      <DialogFooter>
        <Button variant="outline" @click="closeImportDialog">Cancel</Button>
        <Button
          :disabled="!importJsonText.trim() || isImporting"
          @click="importWorkflow"
        >
          {{ isImporting ? "Importing..." : "Import Workflow" }}
        </Button>
      </DialogFooter>
    </DialogContent>
  </Dialog>
</template>
