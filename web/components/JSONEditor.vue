<script setup lang="ts">
import {
    ref,
    watch,
    onMounted,
    onBeforeUnmount,
    computed,
    nextTick,
} from "vue";
import { Label } from "~/components/ui/label";
import { Button } from "~/components/ui/button";
import { Badge } from "~/components/ui/badge";
import { Textarea } from "~/components/ui/textarea";
import { Code, Type, Check, X } from "lucide-vue-next";

interface Props {
    modelValue: string;
    label?: string;
    placeholder?: string;
    rows?: number;
    autoDetect?: boolean;
}

const props = withDefaults(defineProps<Props>(), {
    label: "Value",
    placeholder: "Enter value...",
    rows: 8,
    autoDetect: true,
});

const emit = defineEmits<{
    (e: "update:modelValue", value: string): void;
}>();

const editorContainer = ref<HTMLElement | null>(null);
const editor = ref<any>(null);
const isMonacoLoaded = ref(false);
const useJsonEditor = ref(false);
const isValidJson = ref(true);
const validationMessage = ref("");
const isClient = ref(false);

// Check if value is JSON
const isJsonValue = (value: string): boolean => {
    if (!value || value.trim() === "") return false;
    const trimmed = value.trim();
    if (!trimmed.startsWith("{") && !trimmed.startsWith("[")) return false;

    try {
        JSON.parse(trimmed);
        return true;
    } catch {
        return false;
    }
};

// Format JSON
const formatJson = (value: string): string => {
    try {
        const parsed = JSON.parse(value);
        return JSON.stringify(parsed, null, 2);
    } catch {
        return value;
    }
};

// Validate current value
const validateValue = () => {
    if (!useJsonEditor.value) {
        isValidJson.value = true;
        validationMessage.value = "";
        return;
    }

    const value = props.modelValue.trim();
    if (!value) {
        isValidJson.value = true;
        validationMessage.value = "";
        return;
    }

    try {
        JSON.parse(value);
        isValidJson.value = true;
        validationMessage.value = "Valid JSON";
    } catch (e: any) {
        isValidJson.value = false;
        validationMessage.value = e.message || "Invalid JSON";
    }
};

// Initialize Monaco Editor
const initMonaco = async () => {
    if (!isClient.value || !editorContainer.value || editor.value) return;

    try {
        // Dynamic import of Monaco Editor - only on client
        const monaco = await import("monaco-editor");

        // Configure Monaco
        monaco.editor.defineTheme("custom-theme", {
            base: "vs-dark",
            inherit: true,
            rules: [],
            colors: {
                "editor.background": "#0f172a",
            },
        });

        editor.value = monaco.editor.create(editorContainer.value, {
            value: props.modelValue,
            language: "json",
            theme: "custom-theme",
            minimap: { enabled: false },
            lineNumbers: "on",
            roundedSelection: true,
            scrollBeyondLastLine: false,
            automaticLayout: true,
            fontSize: 13,
            tabSize: 2,
            formatOnPaste: true,
            formatOnType: true,
        });

        // Listen for changes
        editor.value.onDidChangeModelContent(() => {
            const value = editor.value.getValue();
            emit("update:modelValue", value);
            validateValue();
        });

        isMonacoLoaded.value = true;

        // Format if valid JSON
        if (isJsonValue(props.modelValue)) {
            const formatted = formatJson(props.modelValue);
            editor.value.setValue(formatted);

            // Use setTimeout to ensure editor is ready
            setTimeout(() => {
                editor.value?.getAction("editor.action.formatDocument")?.run();
            }, 100);
        }
    } catch (error) {
        console.error("Failed to initialize Monaco Editor:", error);
        // Fallback to textarea
        useJsonEditor.value = false;
    }
};

// Toggle between textarea and JSON editor
const toggleEditorMode = async () => {
    if (!isClient.value) return;

    useJsonEditor.value = !useJsonEditor.value;

    if (useJsonEditor.value) {
        // Switching to JSON editor
        await nextTick();
        await initMonaco();
    } else {
        // Switching to textarea - cleanup Monaco
        if (editor.value) {
            editor.value.dispose();
            editor.value = null;
            isMonacoLoaded.value = false;
        }
    }

    validateValue();
};

// Format JSON in editor
const formatInEditor = () => {
    if (editor.value) {
        editor.value.getAction("editor.action.formatDocument")?.run();
    }
};

// Watch for external changes
watch(
    () => props.modelValue,
    (newValue) => {
        if (editor.value && editor.value.getValue() !== newValue) {
            const position = editor.value.getPosition();
            editor.value.setValue(newValue);
            if (position) {
                editor.value.setPosition(position);
            }
        }
        validateValue();
    },
);

// Auto-detect JSON on mount
onMounted(() => {
    // Set client flag
    isClient.value = true;

    if (props.autoDetect && isJsonValue(props.modelValue)) {
        useJsonEditor.value = true;
        nextTick(() => initMonaco());
    }
    validateValue();
});

// Cleanup
onBeforeUnmount(() => {
    if (editor.value) {
        editor.value.dispose();
        editor.value = null;
    }
});

const localValue = computed({
    get: () => props.modelValue,
    set: (value: string) => {
        emit("update:modelValue", value);
        validateValue();
    },
});
</script>

<template>
    <div class="space-y-2">
        <div class="flex items-center justify-between">
            <Label>{{ label }}</Label>
            <div class="flex items-center gap-2">
                <Badge
                    v-if="useJsonEditor"
                    :variant="isValidJson ? 'default' : 'destructive'"
                    class="text-xs"
                >
                    <Check v-if="isValidJson" class="h-3 w-3 mr-1" />
                    <X v-else class="h-3 w-3 mr-1" />
                    {{ isValidJson ? "Valid" : "Invalid" }}
                </Badge>
                <Button
                    v-if="isClient"
                    type="button"
                    variant="outline"
                    size="sm"
                    @click="toggleEditorMode"
                >
                    <Code v-if="!useJsonEditor" class="h-4 w-4 mr-2" />
                    <Type v-else class="h-4 w-4 mr-2" />
                    {{ useJsonEditor ? "Text Mode" : "JSON Mode" }}
                </Button>
                <Button
                    v-if="useJsonEditor && editor && isClient"
                    type="button"
                    variant="outline"
                    size="sm"
                    @click="formatInEditor"
                    :disabled="!isValidJson"
                >
                    Format
                </Button>
            </div>
        </div>

        <!-- Monaco Editor -->
        <ClientOnly>
            <div
                v-show="useJsonEditor"
                ref="editorContainer"
                class="border rounded-lg overflow-hidden"
                :style="{ height: `${rows * 24}px` }"
            ></div>
        </ClientOnly>

        <!-- Textarea Fallback -->
        <Textarea
            v-show="!useJsonEditor"
            v-model="localValue"
            :placeholder="placeholder"
            :rows="rows"
            class="font-mono text-sm"
        />

        <div class="flex items-start justify-between">
            <p class="text-xs text-muted-foreground">
                {{
                    useJsonEditor
                        ? "JSON editor with syntax highlighting and validation"
                        : "Enter a string or valid JSON (object, array, number, boolean)"
                }}
            </p>
            <p
                v-if="useJsonEditor && !isValidJson"
                class="text-xs text-destructive font-medium"
            >
                {{ validationMessage }}
            </p>
        </div>
    </div>
</template>
