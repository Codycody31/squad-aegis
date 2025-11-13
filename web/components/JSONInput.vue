<script setup lang="ts">
import { ref, computed, watch } from "vue";
import { Label } from "~/components/ui/label";
import { Button } from "~/components/ui/button";
import { Badge } from "~/components/ui/badge";
import { Textarea } from "~/components/ui/textarea";
import { Code, Type, Check, X, Sparkles } from "lucide-vue-next";

interface Props {
    modelValue: string;
    label?: string;
    placeholder?: string;
    rows?: number;
}

const props = withDefaults(defineProps<Props>(), {
    label: "Value",
    placeholder: "Enter value...",
    rows: 6,
});

const emit = defineEmits<{
    (e: "update:modelValue", value: string): void;
}>();

const useJsonMode = ref(false);
const isValidJson = ref(true);
const validationMessage = ref("");

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

// Validate current value
const validateValue = (value: string) => {
    if (!useJsonMode.value) {
        isValidJson.value = true;
        validationMessage.value = "";
        return;
    }

    const trimmed = value.trim();
    if (!trimmed) {
        isValidJson.value = true;
        validationMessage.value = "";
        return;
    }

    try {
        JSON.parse(trimmed);
        isValidJson.value = true;
        validationMessage.value = "Valid JSON";
    } catch (e: any) {
        isValidJson.value = false;
        validationMessage.value = e.message || "Invalid JSON";
    }
};

// Format JSON
const formatJson = () => {
    try {
        const parsed = JSON.parse(props.modelValue);
        const formatted = JSON.stringify(parsed, null, 2);
        emit("update:modelValue", formatted);
        isValidJson.value = true;
        validationMessage.value = "Valid JSON";
    } catch (e: any) {
        isValidJson.value = false;
        validationMessage.value = e.message || "Invalid JSON";
    }
};

// Toggle JSON mode
const toggleJsonMode = () => {
    useJsonMode.value = !useJsonMode.value;
    validateValue(props.modelValue);
};

// Computed value for v-model
const localValue = computed({
    get: () => props.modelValue,
    set: (value: string) => {
        emit("update:modelValue", value);
        validateValue(value);
    },
});

// Watch for value changes
watch(
    () => props.modelValue,
    (newValue) => {
        validateValue(newValue);

        // Auto-detect JSON
        if (!useJsonMode.value && isJsonValue(newValue)) {
            useJsonMode.value = true;
        }
    },
    { immediate: true }
);
</script>

<template>
    <div class="space-y-2">
        <div class="flex items-center justify-between">
            <Label>{{ label }}</Label>
            <div class="flex items-center gap-2">
                <Badge
                    v-if="useJsonMode"
                    :variant="isValidJson ? 'default' : 'destructive'"
                    class="text-xs"
                >
                    <Check v-if="isValidJson" class="h-3 w-3 mr-1" />
                    <X v-else class="h-3 w-3 mr-1" />
                    {{ isValidJson ? "Valid" : "Invalid" }}
                </Badge>
                <Button
                    type="button"
                    variant="outline"
                    size="sm"
                    @click="toggleJsonMode"
                >
                    <Code v-if="!useJsonMode" class="h-4 w-4 mr-2" />
                    <Type v-else class="h-4 w-4 mr-2" />
                    {{ useJsonMode ? "Text Mode" : "JSON Mode" }}
                </Button>
                <Button
                    v-if="useJsonMode"
                    type="button"
                    variant="outline"
                    size="sm"
                    @click="formatJson"
                    :disabled="!isValidJson && modelValue.trim() !== ''"
                >
                    <Sparkles class="h-4 w-4 mr-2" />
                    Format
                </Button>
            </div>
        </div>

        <Textarea
            v-model="localValue"
            :placeholder="placeholder"
            :rows="rows"
            :class="[
                'font-mono text-sm transition-colors',
                useJsonMode && !isValidJson && modelValue.trim() !== ''
                    ? 'border-destructive focus-visible:ring-destructive'
                    : '',
            ]"
        />

        <div class="flex items-start justify-between">
            <p class="text-xs text-muted-foreground">
                {{
                    useJsonMode
                        ? "JSON mode: validation and formatting enabled"
                        : "Enter a string or valid JSON (object, array, number, boolean)"
                }}
            </p>
            <p
                v-if="useJsonMode && !isValidJson && modelValue.trim() !== ''"
                class="text-xs text-destructive font-medium"
            >
                {{ validationMessage }}
            </p>
        </div>
    </div>
</template>
