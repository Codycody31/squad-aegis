<script setup lang="ts">
import { Button } from "~/components/ui/button";
import {
    Select,
    SelectContent,
    SelectItem,
    SelectTrigger,
    SelectValue,
} from "~/components/ui/select";
import { Input } from "~/components/ui/input";
import { Label } from "~/components/ui/label";
import { Textarea } from "~/components/ui/textarea";
import { Switch } from "~/components/ui/switch";
import { Plus, X } from "lucide-vue-next";
import { initializeConfigFromSchema } from "~/composables/useConfigSchema";
import type { ConfigSchemaField } from "~/types";

const props = withDefaults(
    defineProps<{
        fields: ConfigSchemaField[];
        /** Prefix for input ids/labels; keeps ids unique across dialogs. */
        idPrefix?: string;
    }>(),
    {
        idPrefix: "config",
    },
);

const config = defineModel<Record<string, any>>({ required: true });

// Helper function to normalize field names for display
const normalizeFieldName = (fieldName: string): string => {
    // Replace underscores with spaces
    let normalized = fieldName.replace(/_/g, " ");

    // Handle camelCase by inserting space before capital letters
    normalized = normalized.replace(/([a-z])([A-Z])/g, "$1 $2");

    // Capitalize first letter of each word
    normalized = normalized.replace(/\b\w/g, (char) => char.toUpperCase());

    // Handle common acronyms
    const acronyms = [
        "Id",
        "Ip",
        "Url",
        "Api",
        "Fob",
        "Hab",
        "Tk",
        "Eos",
        "Rcon",
        "Ui",
        "Db",
    ];
    acronyms.forEach((acronym) => {
        const regex = new RegExp(`\\b${acronym}\\b`, "gi");
        normalized = normalized.replace(regex, acronym.toUpperCase());
    });

    return normalized;
};

// Enum options for select-style fields (schema delivers them as `options`)
const fieldOptions = (field: any): any[] | undefined => field.options;

// Get input type for field
const getInputType = (field: any) => {
    switch (field.type) {
        case "string":
            return field.sensitive ||
                field.name.toLowerCase().includes("password")
                ? "password"
                : "text";
        case "int":
        case "number":
            return "number";
        default:
            return "text";
    }
};

// Array bool helper functions
const updateArrayBool = (
    fieldName: string,
    index: number,
    checked: boolean,
) => {
    if (!config.value[fieldName]) {
        config.value[fieldName] = [];
    }
    config.value[fieldName][index] = checked;
};

const addArrayBoolItem = (fieldName: string) => {
    if (!config.value[fieldName]) {
        config.value[fieldName] = [];
    }
    config.value[fieldName].push(false);
};

const removeArrayBoolItem = (fieldName: string, index: number) => {
    if (config.value[fieldName] && Array.isArray(config.value[fieldName])) {
        config.value[fieldName].splice(index, 1);
    }
};

// Array object helper functions
const addArrayObjectItem = (fieldName: string, nestedFields: any[]) => {
    if (!config.value[fieldName]) {
        config.value[fieldName] = [];
    }

    config.value[fieldName].push(initializeConfigFromSchema({}, nestedFields));
};

const removeArrayObjectItem = (fieldName: string, index: number) => {
    if (config.value[fieldName] && Array.isArray(config.value[fieldName])) {
        config.value[fieldName].splice(index, 1);
    }
};
</script>

<template>
    <div class="space-y-4">
        <div
            v-for="field in fields"
            :key="field.name"
            class="space-y-2"
        >
            <Label :for="`${idPrefix}-${field.name}`">
                {{ normalizeFieldName(field.name) }}
                <span v-if="field.required" class="text-red-500">*</span>
            </Label>
            <p
                v-if="field.description"
                class="text-sm text-muted-foreground"
            >
                {{ field.description }}
            </p>

            <!-- String/Text input -->
            <Input
                v-if="field.type === 'string'"
                :id="`${idPrefix}-${field.name}`"
                v-model="config[field.name]"
                :type="getInputType(field)"
                :placeholder="
                    field.sensitive && config[field.name] === ''
                        ? 'Leave empty to keep current value'
                        : field.sensitive
                          ? 'Enter new sensitive value...'
                          : ''
                "
            />

            <!-- Number input -->
            <Input
                v-else-if="field.type === 'int' || field.type === 'number'"
                :id="`${idPrefix}-${field.name}`"
                v-model.number="config[field.name]"
                type="number"
            />

            <!-- Boolean switch -->
            <div
                v-else-if="field.type === 'bool'"
                class="flex items-center space-x-2"
            >
                <Switch
                    :id="`${idPrefix}-${field.name}`"
                    :model-value="!!config[field.name]"
                    @update:model-value="
                        (checked: boolean) => (config[field.name] = checked)
                    "
                />
                <Label :for="`${idPrefix}-${field.name}`">{{
                    normalizeFieldName(field.name)
                }}</Label>
            </div>

            <!-- Select dropdown for options -->
            <Select
                v-else-if="(fieldOptions(field)?.length || 0) > 0"
                :model-value="config[field.name]"
                @update:model-value="
                    (value) => (config[field.name] = value)
                "
            >
                <SelectTrigger :id="`${idPrefix}-${field.name}`">
                    <SelectValue placeholder="Select an option" />
                </SelectTrigger>
                <SelectContent>
                    <SelectItem
                        v-for="option in fieldOptions(field)"
                        :key="option"
                        :value="option"
                    >
                        {{ option }}
                    </SelectItem>
                </SelectContent>
            </Select>

            <!-- Array of strings -->
            <Textarea
                v-else-if="
                    field.type === 'arraystring' ||
                    field.type === 'array_string'
                "
                :id="`${idPrefix}-${field.name}`"
                :model-value="
                    Array.isArray(config[field.name])
                        ? config[field.name].join(', ')
                        : config[field.name] || ''
                "
                placeholder="Enter values separated by commas"
                @input="
                    config[field.name] = (
                        $event.target as HTMLTextAreaElement
                    ).value
                        .split(',')
                        .map((s: string) => s.trim())
                        .filter((s) => s.length > 0)
                "
            />

            <!-- Array of integers -->
            <Textarea
                v-else-if="field.type === 'arrayint'"
                :id="`${idPrefix}-${field.name}`"
                :model-value="
                    Array.isArray(config[field.name])
                        ? config[field.name].join(', ')
                        : config[field.name] || ''
                "
                placeholder="Enter integer values separated by commas"
                @input="
                    config[field.name] = (
                        $event.target as HTMLTextAreaElement
                    ).value
                        .split(',')
                        .map((s: string) => parseInt(s.trim()))
                        .filter((n) => !isNaN(n))
                "
            />

            <!-- Array of booleans -->
            <div v-else-if="field.type === 'arraybool'" class="space-y-2">
                <div
                    v-for="(item, index) in config[field.name] || []"
                    :key="index"
                    class="flex items-center space-x-2"
                >
                    <Switch
                        :modelValue="!!item"
                        @update:modelValue="
                            (checked: boolean) =>
                                updateArrayBool(field.name, index, checked)
                        "
                    />
                    <Label
                        >{{ normalizeFieldName(field.name) }}
                        {{ index + 1 }}</Label
                    >
                    <Button
                        variant="outline"
                        size="sm"
                        @click="removeArrayBoolItem(field.name, index)"
                    >
                        Remove
                    </Button>
                </div>
                <Button
                    variant="outline"
                    size="sm"
                    @click="addArrayBoolItem(field.name)"
                >
                    Add Item
                </Button>
            </div>

            <!-- Object configuration -->
            <div
                v-else-if="field.type === 'object'"
                class="space-y-4 p-4 border rounded-lg"
            >
                <h5 class="font-medium">
                    {{ normalizeFieldName(field.name) }}
                    Configuration
                </h5>
                <div
                    v-for="nestedField in field.nested || []"
                    :key="nestedField.name"
                    class="space-y-2"
                >
                    <Label :for="`${idPrefix}-${field.name}-${nestedField.name}`">
                        {{ normalizeFieldName(nestedField.name) }}
                        <span
                            v-if="nestedField.required"
                            class="text-red-500"
                            >*</span
                        >
                    </Label>
                    <p
                        v-if="nestedField.description"
                        class="text-sm text-muted-foreground"
                    >
                        {{ nestedField.description }}
                    </p>

                    <!-- Nested field inputs based on type -->
                    <Input
                        v-if="nestedField.type === 'string'"
                        :id="`${idPrefix}-${field.name}-${nestedField.name}`"
                        v-model="
                            (config[field.name] = config[field.name] || {})[
                                nestedField.name
                            ]
                        "
                        :type="
                            nestedField.sensitive ||
                            nestedField.name
                                .toLowerCase()
                                .includes('password')
                                ? 'password'
                                : 'text'
                        "
                        :placeholder="
                            nestedField.sensitive
                                ? 'Enter sensitive value...'
                                : ''
                        "
                    />
                    <Input
                        v-else-if="nestedField.type === 'int'"
                        :id="`${idPrefix}-${field.name}-${nestedField.name}`"
                        v-model.number="
                            (config[field.name] = config[field.name] || {})[
                                nestedField.name
                            ]
                        "
                        type="number"
                    />
                    <div
                        v-else-if="nestedField.type === 'bool'"
                        class="flex items-center space-x-2"
                    >
                        <Switch
                            :id="`${idPrefix}-${field.name}-${nestedField.name}`"
                            :modelValue="
                                !!(config[field.name] || {})[nestedField.name]
                            "
                            @update:modelValue="
                                (checked: boolean) => {
                                    config[field.name] =
                                        config[field.name] || {};
                                    config[field.name][nestedField.name] =
                                        checked;
                                }
                            "
                        />
                        <Label
                            :for="`${idPrefix}-${field.name}-${nestedField.name}`"
                            >{{ normalizeFieldName(nestedField.name) }}</Label
                        >
                    </div>
                </div>
            </div>

            <!-- Array of objects -->
            <div v-else-if="field.type === 'arrayobject'" class="space-y-4">
                <div
                    v-for="(item, index) in config[field.name] || []"
                    :key="index"
                    class="p-4 border rounded-lg space-y-2"
                >
                    <div class="flex justify-between items-center">
                        <h6 class="font-medium">
                            {{ normalizeFieldName(field.name) }}
                            Item {{ index + 1 }}
                        </h6>
                        <Button
                            variant="outline"
                            size="sm"
                            @click="removeArrayObjectItem(field.name, index)"
                        >
                            Remove
                        </Button>
                    </div>

                    <div
                        v-for="nestedField in field.nested || []"
                        :key="nestedField.name"
                        class="space-y-2"
                    >
                        <Label
                            :for="`${idPrefix}-${field.name}-${index}-${nestedField.name}`"
                        >
                            {{ normalizeFieldName(nestedField.name) }}
                            <span
                                v-if="nestedField.required"
                                class="text-red-500"
                                >*</span
                            >
                        </Label>
                        <p
                            v-if="nestedField.description"
                            class="text-sm text-muted-foreground"
                        >
                            {{ nestedField.description }}
                        </p>

                        <Input
                            v-if="nestedField.type === 'string'"
                            :id="`${idPrefix}-${field.name}-${index}-${nestedField.name}`"
                            v-model="item[nestedField.name]"
                            type="text"
                        />
                        <Input
                            v-else-if="nestedField.type === 'int'"
                            :id="`${idPrefix}-${field.name}-${index}-${nestedField.name}`"
                            v-model.number="item[nestedField.name]"
                            type="number"
                        />
                        <div
                            v-else-if="nestedField.type === 'bool'"
                            class="flex items-center space-x-2"
                        >
                            <Switch
                                :id="`${idPrefix}-${field.name}-${index}-${nestedField.name}`"
                                :modelValue="!!item[nestedField.name]"
                                @update:modelValue="
                                    (checked: boolean) =>
                                        (item[nestedField.name] = checked)
                                "
                            />
                            <Label
                                :for="`${idPrefix}-${field.name}-${index}-${nestedField.name}`"
                                >{{
                                    normalizeFieldName(nestedField.name)
                                }}</Label
                            >
                        </div>
                        <!-- Array of strings within nested object -->
                        <div
                            v-else-if="
                                nestedField.type === 'arraystring' ||
                                nestedField.type === 'array_string'
                            "
                            class="space-y-2"
                        >
                            <div
                                v-for="(arrayItem, arrayIndex) in item[
                                    nestedField.name
                                ] || []"
                                :key="arrayIndex"
                                class="flex items-center gap-2"
                            >
                                <Input
                                    :id="`${idPrefix}-${field.name}-${index}-${nestedField.name}-${arrayIndex}`"
                                    v-model="item[nestedField.name][arrayIndex]"
                                    type="text"
                                    :placeholder="`${normalizeFieldName(nestedField.name)} ${arrayIndex + 1}`"
                                    class="flex-1"
                                />
                                <Button
                                    variant="outline"
                                    size="icon"
                                    @click="
                                        item[nestedField.name].splice(
                                            arrayIndex,
                                            1,
                                        )
                                    "
                                >
                                    <X class="w-4 h-4" />
                                </Button>
                            </div>
                            <Button
                                variant="outline"
                                size="sm"
                                @click="
                                    if (!item[nestedField.name]) {
                                        item[nestedField.name] = [];
                                    }
                                    item[nestedField.name].push('');
                                "
                                class="w-full"
                            >
                                <Plus class="w-4 h-4 mr-2" />
                                Add {{ normalizeFieldName(nestedField.name) }}
                            </Button>
                        </div>
                        <!-- Array of integers within nested object -->
                        <div
                            v-else-if="nestedField.type === 'arrayint'"
                            class="space-y-2"
                        >
                            <div
                                v-for="(arrayItem, arrayIndex) in item[
                                    nestedField.name
                                ] || []"
                                :key="arrayIndex"
                                class="flex items-center gap-2"
                            >
                                <Input
                                    :id="`${idPrefix}-${field.name}-${index}-${nestedField.name}-${arrayIndex}`"
                                    v-model.number="
                                        item[nestedField.name][arrayIndex]
                                    "
                                    type="number"
                                    :placeholder="`${normalizeFieldName(nestedField.name)} ${arrayIndex + 1}`"
                                    class="flex-1"
                                />
                                <Button
                                    variant="outline"
                                    size="icon"
                                    @click="
                                        item[nestedField.name].splice(
                                            arrayIndex,
                                            1,
                                        )
                                    "
                                >
                                    <X class="w-4 h-4" />
                                </Button>
                            </div>
                            <Button
                                variant="outline"
                                size="sm"
                                @click="
                                    if (!item[nestedField.name]) {
                                        item[nestedField.name] = [];
                                    }
                                    item[nestedField.name].push(0);
                                "
                                class="w-full"
                            >
                                <Plus class="w-4 h-4 mr-2" />
                                Add {{ normalizeFieldName(nestedField.name) }}
                            </Button>
                        </div>
                    </div>
                </div>
                <Button
                    variant="outline"
                    size="sm"
                    @click="addArrayObjectItem(field.name, field.nested || [])"
                >
                    Add {{ normalizeFieldName(field.name) }} Item
                </Button>
            </div>
        </div>
    </div>
</template>
