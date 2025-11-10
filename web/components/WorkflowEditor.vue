<script setup lang="ts">
import { ref, computed, watch } from "vue";
import {
    Plus,
    Trash2,
    Move,
    Settings,
    Zap,
    GitBranch,
    Variable,
    Clock,
    Play,
    Code,
    Upload,
    FileJson,
} from "lucide-vue-next";
import { Button } from "~/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "~/components/ui/card";
import { Badge } from "~/components/ui/badge";
import { Input } from "~/components/ui/input";
import { Label } from "~/components/ui/label";
import { Textarea } from "~/components/ui/textarea";
import { Switch } from "~/components/ui/switch";
import {
    Select,
    SelectContent,
    SelectItem,
    SelectTrigger,
    SelectValue,
} from "~/components/ui/select";
import {
    Dialog,
    DialogContent,
    DialogDescription,
    DialogFooter,
    DialogHeader,
    DialogTitle,
} from "~/components/ui/dialog";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "~/components/ui/tabs";
import { Separator } from "~/components/ui/separator";
import { CodeEditor } from "monaco-editor-vue3";
import "monaco-editor-vue3/dist/style.css";

interface WorkflowTrigger {
    id: string;
    name: string;
    event_type: string;
    conditions?: WorkflowCondition[];
    enabled: boolean;
}

interface WorkflowCondition {
    field: string;
    operator: string;
    value: any;
    type: string;
}

interface WorkflowStep {
    id: string;
    name: string;
    type: string;
    enabled: boolean;
    config: Record<string, any>;
    on_error?: any;
    next_steps?: string[];
}

interface WorkflowDefinition {
    name?: string;
    description?: string;
    version: string;
    triggers: WorkflowTrigger[];
    variables: Record<string, any>;
    steps: WorkflowStep[];
    error_handling?: any;
}

interface EventType {
    value: string;
    label: string;
}

interface StepType {
    value: string;
    label: string;
    description: string;
}

interface ActionType {
    value: string;
    label: string;
    description: string;
}

const props = defineProps<{
    modelValue: WorkflowDefinition;
    eventTypes: EventType[];
    stepTypes: StepType[];
    actionTypes: ActionType[];
    workflowId?: string;
    workflowName?: string;
    workflowDescription?: string;
    serverId?: string;
}>();

const emit = defineEmits<{
    "update:modelValue": [value: WorkflowDefinition];
}>();

// Local state
const showTriggerDialog = ref(false);
const showStepDialog = ref(false);
const showVariableDialog = ref(false);
const showImportDialog = ref(false);
const showNestedStepDialog = ref(false);
const showJsonEditorDialog = ref(false);
const selectedTrigger = ref<WorkflowTrigger | null>(null);
const selectedStep = ref<WorkflowStep | null>(null);
const selectedVariable = ref<{ key: string; value: any; type: string } | null>(
    null,
);
const selectedNestedStep = ref<WorkflowStep | null>(null);
const editingNestedStepContext = ref<{
    fieldKey: string;
    index: number;
    parentStep: WorkflowStep | null;
} | null>(null);
const importJsonText = ref("");
const importError = ref("");
const importFile = ref<File | null>(null);
const jsonEditorText = ref("");
const jsonEditorError = ref("");
const editingTriggerIndex = ref(-1);
const editingStepIndex = ref(-1);
const editingVariableKey = ref("");

// Watch for step type changes to initialize appropriate configs
watch(
    () => selectedStep.value?.type,
    (newType) => {
        if (!selectedStep.value) return;

        if (newType === "condition") {
            if (!selectedStep.value.config.conditions) {
                selectedStep.value.config.conditions = [];
            }
            if (!selectedStep.value.config.logic) {
                selectedStep.value.config.logic = "AND";
            }
            if (!selectedStep.value.config.true_steps) {
                selectedStep.value.config.true_steps = [];
            }
            if (!selectedStep.value.config.false_steps) {
                selectedStep.value.config.false_steps = [];
            }
        } else if (newType === "variable") {
            if (!selectedStep.value.config.operation) {
                selectedStep.value.config.operation = "set";
            }
        }
    },
    { deep: true },
);

// Condition operators
const operators = [
    { value: "equals", label: "Equals" },
    { value: "not_equals", label: "Not Equals" },
    { value: "contains", label: "Contains" },
    { value: "not_contains", label: "Not Contains" },
    { value: "starts_with", label: "Starts With" },
    { value: "ends_with", label: "Ends With" },
    { value: "regex", label: "Regex Match" },
    { value: "greater_than", label: "Greater Than" },
    { value: "less_than", label: "Less Than" },
    { value: "greater_or_equal", label: "Greater or Equal" },
    { value: "less_or_equal", label: "Less or Equal" },
    { value: "in", label: "In Array" },
    { value: "not_in", label: "Not In Array" },
    { value: "is_null", label: "Is Null" },
    { value: "is_not_null", label: "Is Not Null" },
];

// Condition value types
const valueTypes = [
    { value: "string", label: "Text" },
    { value: "number", label: "Number" },
    { value: "boolean", label: "Boolean" },
    { value: "array", label: "Array" },
    { value: "object", label: "Object" },
];

// Error handling actions
const errorActions = [
    { value: "continue", label: "Continue" },
    { value: "stop", label: "Stop" },
    { value: "retry", label: "Retry" },
];

// Create a computed property that works directly with the prop
const definition = computed({
    get: () => props.modelValue,
    set: (value: WorkflowDefinition) => emit("update:modelValue", value),
});

// Computed property for available steps (for step selection in conditions)
const availableSteps = computed(() => {
    if (!selectedStep.value) return [];

    return definition.value.steps
        .filter((step) => step.id !== selectedStep.value?.id)
        .map((step) => ({
            value: step.name,
            label: `${step.name} (${step.type})`,
        }));
});

// Computed property for error handling with safe defaults
const errorHandling = computed(() => {
    return (
        definition.value.error_handling || {
            default_action: "stop",
            max_retries: 3,
            retry_delay_ms: 1000,
        }
    );
});

// Helper function to check if a field should be shown based on step config
function shouldShowField(field: any, stepConfig: Record<string, any>): boolean {
    if (field.key === "value" && stepConfig.operation === "delete") {
        return false; // Don't show value field for delete operation
    }
    if (field.key === "source_variable" && stepConfig.operation !== "copy") {
        return false; // Only show source_variable for copy operation
    }
    if (
        field.key === "transform_type" &&
        stepConfig.operation !== "transform"
    ) {
        return false; // Only show transform_type for transform operation
    }
    if (
        field.key === "value" &&
        ["is_null", "is_not_null"].includes(stepConfig.operator)
    ) {
        return false; // Don't show value field for null checks
    }
    return true;
}

// Helper functions
function generateId(): string {
    return crypto.randomUUID();
}

function getStepIcon(type: string) {
    switch (type) {
        case "action":
            return Play;
        case "condition":
            return GitBranch;
        case "variable":
            return Variable;
        case "delay":
            return Clock;
        case "lua":
            return Code;
        default:
            return Settings;
    }
}

// Trigger management
function openTriggerDialog(trigger?: WorkflowTrigger, index?: number) {
    if (trigger && typeof index === "number") {
        selectedTrigger.value = { ...trigger };
        editingTriggerIndex.value = index;
    } else {
        selectedTrigger.value = {
            id: generateId(),
            name: "",
            event_type: "",
            conditions: [],
            enabled: true,
        };
        editingTriggerIndex.value = -1;
    }
    showTriggerDialog.value = true;
}

function saveTrigger() {
    if (!selectedTrigger.value) return;

    const newDefinition = { ...definition.value };
    const newTriggers = [...newDefinition.triggers];

    if (editingTriggerIndex.value >= 0) {
        newTriggers[editingTriggerIndex.value] = { ...selectedTrigger.value };
    } else {
        newTriggers.push({ ...selectedTrigger.value });
    }

    newDefinition.triggers = newTriggers;
    definition.value = newDefinition;

    closeTriggerDialog();
}

function deleteTrigger(index: number) {
    const newDefinition = { ...definition.value };
    const newTriggers = [...newDefinition.triggers];
    newTriggers.splice(index, 1);
    newDefinition.triggers = newTriggers;
    definition.value = newDefinition;
}

function closeTriggerDialog() {
    showTriggerDialog.value = false;
    selectedTrigger.value = null;
    editingTriggerIndex.value = -1;
}

// Condition management
function addCondition() {
    if (!selectedTrigger.value) return;

    if (!selectedTrigger.value.conditions) {
        selectedTrigger.value.conditions = [];
    }

    selectedTrigger.value.conditions.push({
        field: "",
        operator: "equals",
        value: "",
        type: "string",
    });
}

function removeCondition(index: number) {
    if (selectedTrigger.value?.conditions) {
        selectedTrigger.value.conditions.splice(index, 1);
    }
}

// Step condition management (for condition steps)
function addStepCondition() {
    if (!selectedStep.value) return;

    if (!selectedStep.value.config.conditions) {
        selectedStep.value.config.conditions = [];
    }

    selectedStep.value.config.conditions.push({
        field: "",
        operator: "equals",
        value: "",
        type: "string",
    });
}

function removeStepCondition(index: number) {
    if (selectedStep.value?.config.conditions) {
        selectedStep.value.config.conditions.splice(index, 1);
    }
}

// Step management
function openStepDialog(step?: WorkflowStep, index?: number) {
    if (step && typeof index === "number") {
        selectedStep.value = { ...step };
        editingStepIndex.value = index;
    } else {
        selectedStep.value = {
            id: generateId(),
            name: "",
            type: "action",
            enabled: true,
            config: {},
            on_error: {
                action: "stop",
                max_retries: 3,
                retry_delay_ms: 1000,
            },
        };
        editingStepIndex.value = -1;
    }

    // Initialize conditions array for condition steps
    if (
        selectedStep.value.type === "condition" &&
        !selectedStep.value.config.conditions
    ) {
        selectedStep.value.config.conditions = [];
    }

    // Initialize default logic for condition steps
    if (
        selectedStep.value.type === "condition" &&
        !selectedStep.value.config.logic
    ) {
        selectedStep.value.config.logic = "AND";
    }

    // Initialize true_steps and false_steps arrays for condition steps
    if (selectedStep.value.type === "condition") {
        if (!selectedStep.value.config.true_steps) {
            selectedStep.value.config.true_steps = [];
        }
        if (!selectedStep.value.config.false_steps) {
            selectedStep.value.config.false_steps = [];
        }
    }

    // Initialize default operation for variable steps
    if (
        selectedStep.value.type === "variable" &&
        !selectedStep.value.config.operation
    ) {
        selectedStep.value.config.operation = "set";
    }

    showStepDialog.value = true;
}

function saveStep() {
    if (!selectedStep.value) return;

    const newDefinition = { ...definition.value };
    const newSteps = [...newDefinition.steps];

    if (editingStepIndex.value >= 0) {
        newSteps[editingStepIndex.value] = { ...selectedStep.value };
    } else {
        newSteps.push({ ...selectedStep.value });
    }

    newDefinition.steps = newSteps;
    definition.value = newDefinition;

    closeStepDialog();
}

function deleteStep(index: number) {
    const newDefinition = { ...definition.value };
    const newSteps = [...newDefinition.steps];
    newSteps.splice(index, 1);
    newDefinition.steps = newSteps;
    definition.value = newDefinition;
}

function closeStepDialog() {
    showStepDialog.value = false;
    selectedStep.value = null;
    editingStepIndex.value = -1;
}

function moveStepUp(index: number) {
    if (index > 0) {
        const newDefinition = { ...definition.value };
        const newSteps = [...newDefinition.steps];
        [newSteps[index - 1], newSteps[index]] = [
            newSteps[index],
            newSteps[index - 1],
        ];
        newDefinition.steps = newSteps;
        definition.value = newDefinition;
    }
}

function moveStepDown(index: number) {
    if (index < definition.value.steps.length - 1) {
        const newDefinition = { ...definition.value };
        const newSteps = [...newDefinition.steps];
        [newSteps[index], newSteps[index + 1]] = [
            newSteps[index + 1],
            newSteps[index],
        ];
        newDefinition.steps = newSteps;
        definition.value = newDefinition;
    }
}

// Variable management functions
function openVariableDialog(key?: string) {
    if (
        key &&
        definition.value.variables &&
        key in definition.value.variables
    ) {
        const value = definition.value.variables[key];
        selectedVariable.value = {
            key,
            value,
            type: getVariableType(value),
        };
        editingVariableKey.value = key;
    } else {
        selectedVariable.value = {
            key: "",
            value: "",
            type: "string",
        };
        editingVariableKey.value = "";
    }
    showVariableDialog.value = true;
}

function saveVariable() {
    if (!selectedVariable.value || !selectedVariable.value.key.trim()) return;

    const newDefinition = { ...definition.value };
    if (!newDefinition.variables) {
        newDefinition.variables = {};
    }

    // If editing an existing variable with a different key, remove the old one
    if (
        editingVariableKey.value &&
        editingVariableKey.value !== selectedVariable.value.key
    ) {
        delete newDefinition.variables[editingVariableKey.value];
    }

    // Convert value based on type
    let processedValue = selectedVariable.value.value;
    try {
        switch (selectedVariable.value.type) {
            case "number":
                processedValue = Number(processedValue);
                break;
            case "boolean":
                processedValue =
                    String(processedValue).toLowerCase() === "true";
                break;
            case "array":
                processedValue = JSON.parse(processedValue);
                if (!Array.isArray(processedValue))
                    throw new Error("Not an array");
                break;
            case "object":
                processedValue = JSON.parse(processedValue);
                if (
                    Array.isArray(processedValue) ||
                    typeof processedValue !== "object"
                )
                    throw new Error("Not an object");
                break;
            default:
                processedValue = String(processedValue);
        }
    } catch (error) {
        console.warn(
            "Invalid value for type:",
            selectedVariable.value.type,
            error,
        );
        return;
    }

    newDefinition.variables = { ...newDefinition.variables };
    newDefinition.variables[selectedVariable.value.key] = processedValue;
    definition.value = newDefinition;

    closeVariableDialog();
}

function deleteVariable(key: string) {
    const newDefinition = { ...definition.value };
    if (newDefinition.variables) {
        newDefinition.variables = { ...newDefinition.variables };
        delete newDefinition.variables[key];
        definition.value = newDefinition;
    }
}

function closeVariableDialog() {
    showVariableDialog.value = false;
    selectedVariable.value = null;
    editingVariableKey.value = "";
}

function getVariableType(value: any): string {
    if (typeof value === "number") return "number";
    if (typeof value === "boolean") return "boolean";
    if (Array.isArray(value)) return "array";
    if (typeof value === "object" && value !== null) return "object";
    return "string";
}

function formatVariableValue(value: any): string {
    if (typeof value === "object") {
        return JSON.stringify(value, null, 2);
    }
    return String(value);
}

function getVariableDisplayValue(value: any): string {
    if (typeof value === "string") return `"${value}"`;
    if (typeof value === "object") return JSON.stringify(value);
    return String(value);
}

// Update error handling settings
function updateErrorHandling(field: string, value: any) {
    const newDefinition = { ...definition.value };
    if (!newDefinition.error_handling) {
        newDefinition.error_handling = {};
    }
    newDefinition.error_handling = { ...newDefinition.error_handling };
    newDefinition.error_handling[field] = value;
    definition.value = newDefinition;
}

// Get config fields based on step type
function getConfigFields(stepType: string, actionType?: string) {
    if (stepType === "action") {
        switch (actionType) {
            case "rcon_command":
                return [
                    {
                        key: "command",
                        label: "RCON Command",
                        type: "text",
                        required: true,
                        placeholder:
                            "e.g. AdminBroadcast ${trigger_event.player_name} has been warned",
                        description:
                            "Use ${trigger_event.field} or ${metadata.field} to access event data",
                    },
                ];
            case "admin_broadcast":
                return [
                    {
                        key: "message",
                        label: "Broadcast Message",
                        type: "text",
                        required: true,
                        placeholder:
                            "e.g. Welcome ${trigger_event.player_name} to the server!",
                        description: "Message will be visible to all players",
                    },
                ];
            case "chat_message":
                return [
                    {
                        key: "message",
                        label: "Chat Message",
                        type: "text",
                        required: true,
                        placeholder: "e.g. Hello ${trigger_event.player_name}!",
                        description: "Send a chat message to the server",
                    },
                    {
                        key: "target_player",
                        label: "Target Player",
                        type: "text",
                        required: true,
                        placeholder:
                            "e.g. ${trigger_event.player_name} or ${trigger_event.steam_id}",
                        description:
                            "Player name or Steam ID to send the message to",
                    },
                ];
            case "kick_player":
                return [
                    {
                        key: "player_id",
                        label: "Player ID",
                        type: "text",
                        required: true,
                        placeholder:
                            "e.g. ${trigger_event.player_name} or ${trigger_event.steam_id}",
                        description:
                            "Player name or Steam ID of the player to kick",
                    },
                    {
                        key: "reason",
                        label: "Kick Reason",
                        type: "text",
                        required: false,
                        placeholder: "e.g. Violation of server rules",
                        description: "Optional reason for the kick",
                    },
                ];
            case "ban_player":
                return [
                    {
                        key: "player_id",
                        label: "Player ID",
                        type: "text",
                        required: true,
                        placeholder:
                            "e.g. ${trigger_event.player_name} or ${trigger_event.steam_id}",
                        description:
                            "Player name or Steam ID of the player to ban",
                    },
                    {
                        key: "duration",
                        label: "Ban Duration (days)",
                        type: "number",
                        required: true,
                        placeholder: "1",
                        description: "Duration in days (0 = permanent ban)",
                    },
                    {
                        key: "reason",
                        label: "Ban Reason",
                        type: "text",
                        required: false,
                        placeholder: "e.g. Cheating detected",
                        description: "Optional reason for the ban",
                    },
                ];
            case "warn_player":
                return [
                    {
                        key: "player_id",
                        label: "Player ID",
                        type: "text",
                        required: true,
                        placeholder:
                            "e.g. ${trigger_event.player_name} or ${trigger_event.steam_id}",
                        description:
                            "Player name or Steam ID of the player to warn",
                    },
                    {
                        key: "message",
                        label: "Warning Message",
                        type: "text",
                        required: true,
                        placeholder: "e.g. Please follow server rules",
                        description: "Warning message to send to the player",
                    },
                ];
            case "http_request":
                return [
                    {
                        key: "url",
                        label: "URL",
                        type: "text",
                        required: true,
                        placeholder: "https://api.example.com/webhook",
                    },
                    {
                        key: "method",
                        label: "Method",
                        type: "select",
                        options: ["GET", "POST", "PUT", "DELETE"],
                        required: true,
                    },
                    {
                        key: "body",
                        label: "Request Body",
                        type: "textarea",
                        required: false,
                        placeholder: "JSON payload or form data",
                    },
                    {
                        key: "headers",
                        label: "Headers (JSON)",
                        type: "textarea",
                        required: false,
                        placeholder:
                            '{"Content-Type": "application/json", "Authorization": "Bearer token"}',
                    },
                ];
            case "webhook":
                return [
                    {
                        key: "url",
                        label: "Webhook URL",
                        type: "text",
                        required: true,
                        placeholder: "https://discord.com/api/webhooks/...",
                        description: "Discord webhook or other service URL",
                    },
                    {
                        key: "body",
                        label: "Custom Body (JSON)",
                        type: "textarea",
                        required: false,
                        placeholder:
                            '{"content": "Player ${trigger_event.player_name} joined the server"}',
                        description:
                            "Custom JSON payload. If empty, default event data will be sent",
                    },
                ];
            case "discord_message":
                return [
                    {
                        key: "webhook_url",
                        label: "Discord Webhook URL",
                        type: "text",
                        required: true,
                        placeholder: "https://discord.com/api/webhooks/...",
                        description:
                            "Discord webhook URL from channel settings",
                    },
                    {
                        key: "message",
                        label: "Message Content",
                        type: "textarea",
                        required: true,
                        placeholder:
                            "Player **${trigger_event.player_name}** has joined the server!",
                        description: "Supports Discord markdown formatting",
                    },
                    {
                        key: "username",
                        label: "Bot Username",
                        type: "text",
                        required: false,
                        placeholder: "Squad Aegis",
                        description: "Custom username for the webhook bot",
                    },
                    {
                        key: "avatar_url",
                        label: "Bot Avatar URL",
                        type: "text",
                        required: false,
                        placeholder: "https://example.com/avatar.png",
                        description: "Custom avatar URL for the webhook bot",
                    },
                ];
            case "log_message":
                return [
                    {
                        key: "message",
                        label: "Log Message",
                        type: "text",
                        required: true,
                        placeholder:
                            "e.g. Workflow executed for player ${trigger_event.player_name}",
                        description: "Message to write to the log",
                    },
                    {
                        key: "level",
                        label: "Log Level",
                        type: "select",
                        options: ["debug", "info", "warn", "error"],
                        required: true,
                    },
                ];
            case "set_variable":
                return [
                    {
                        key: "variable_name",
                        label: "Variable Name",
                        type: "text",
                        required: true,
                        placeholder: "e.g. last_player_name",
                        description: "Name of the variable to set",
                    },
                    {
                        key: "variable_value",
                        label: "Variable Value",
                        type: "text",
                        required: true,
                        placeholder: "e.g. ${trigger_event.player_name}",
                        description: "Value to assign to the variable",
                    },
                ];
            case "lua_script":
                return [
                    {
                        key: "script",
                        label: "Lua Script",
                        type: "lua",
                        required: true,
                        rows: 12,
                        placeholder: `-- Access workflow data
local player_name = workflow.trigger_event.player_name
local workflow_name = workflow.metadata.workflow_name

-- Log a message
log("Processing player: " .. (player_name or "unknown"))

-- Set a variable
set_variable("last_processed_player", player_name)

-- Store result
result.success = true
result.player = player_name`,
                        description:
                            "Lua script with access to workflow.trigger_event, workflow.metadata, etc.",
                    },
                    {
                        key: "timeout_seconds",
                        label: "Timeout (seconds)",
                        type: "number",
                        required: false,
                        placeholder: "30",
                        description:
                            "Maximum execution time (default: 30 seconds)",
                    },
                ];
            default:
                return [];
        }
    } else if (stepType === "condition") {
        return [
            {
                key: "logic",
                label: "Logic Operator",
                type: "select",
                required: true,
                options: ["AND", "OR"],
                description:
                    "How to combine multiple conditions (AND = all must be true, OR = at least one must be true)",
            },
            {
                key: "conditions",
                label: "Conditions",
                type: "conditions_array",
                required: true,
                description: "List of conditions to evaluate",
            },
            {
                key: "true_steps",
                label: "Steps if True",
                type: "nested_steps",
                required: false,
                options: availableSteps.value,
                description: "Steps to execute if condition is true",
            },
            {
                key: "false_steps",
                label: "Steps if False",
                type: "nested_steps",
                required: false,
                options: availableSteps.value,
                description: "Steps to execute if condition is false",
            },
        ];
    } else if (stepType === "variable") {
        return [
            {
                key: "operation",
                label: "Operation",
                type: "select",
                required: true,
                options: [
                    "set",
                    "increment",
                    "decrement",
                    "append",
                    "prepend",
                    "delete",
                    "copy",
                    "transform",
                ],
            },
            {
                key: "variable_name",
                label: "Variable Name",
                type: "text",
                required: true,
                placeholder: "e.g. player_count, last_event_time",
                description: "Name of the variable to operate on",
            },
            {
                key: "value",
                label: "Value",
                type: "text",
                required: false,
                placeholder: 'e.g. ${trigger_event.player_name}, 42, "hello"',
                description:
                    "Value for set, increment/decrement amount, or append/prepend text",
            },
            {
                key: "source_variable",
                label: "Source Variable (for copy)",
                type: "text",
                required: false,
                placeholder: "e.g. original_player_name",
                description: "Source variable name when using copy operation",
            },
            {
                key: "transform_type",
                label: "Transform Type",
                type: "select",
                required: false,
                options: [
                    "uppercase",
                    "lowercase",
                    "trim",
                    "length",
                    "reverse",
                ],
            },
        ];
    } else if (stepType === "delay") {
        return [
            {
                key: "delay_ms",
                label: "Delay (milliseconds)",
                type: "number",
                required: true,
            },
        ];
    } else if (stepType === "lua") {
        return [
            {
                key: "script",
                label: "Lua Script",
                type: "lua",
                required: true,
                rows: 12,
                placeholder: `-- Access workflow data
local player_name = workflow.trigger_event.player_name
local workflow_name = workflow.metadata.workflow_name

-- Log a message
log("Processing player: " .. (player_name or "unknown"))

-- Set a variable
set_variable("last_processed_player", player_name)

-- Store result
result.success = true
result.player = player_name`,
                description:
                    "Lua script with access to workflow.trigger_event, workflow.metadata, etc.",
            },
            {
                key: "timeout_seconds",
                label: "Timeout (seconds)",
                type: "number",
                required: false,
                placeholder: "30",
                description: "Maximum execution time (default: 30 seconds)",
            },
        ];
    }

    return [];
}

// Import functions
function openImportDialog() {
    importJsonText.value = "";
    importError.value = "";
    importFile.value = null;
    showImportDialog.value = true;
}

function closeImportDialog() {
    showImportDialog.value = false;
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

function validateAndImportWorkflow() {
    importError.value = "";

    if (!importJsonText.value.trim()) {
        importError.value = "Please enter JSON content";
        return;
    }

    try {
        const parsed = JSON.parse(importJsonText.value);

        // Validate required structure
        if (!parsed.version) {
            importError.value = "Missing required field: version";
            return;
        }

        if (!Array.isArray(parsed.triggers)) {
            importError.value = "Missing or invalid triggers array";
            return;
        }

        if (!Array.isArray(parsed.steps)) {
            importError.value = "Missing or invalid steps array";
            return;
        }

        // Ensure all triggers have required fields and generate IDs if missing
        const validatedTriggers = parsed.triggers.map((trigger: any) => ({
            id: trigger.id || generateId(),
            name: trigger.name || "",
            event_type: trigger.event_type || "",
            conditions: trigger.conditions || [],
            enabled: trigger.enabled !== false, // default to true
        }));

        // Ensure all steps have required fields and generate IDs if missing
        const validatedSteps = parsed.steps.map((step: any) => ({
            id: step.id || generateId(),
            name: step.name || "",
            type: step.type || "action",
            enabled: step.enabled !== false, // default to true
            config: step.config || {},
            on_error: step.on_error || {
                action: "stop",
                max_retries: 3,
                retry_delay_ms: 1000,
            },
            next_steps: step.next_steps || [],
        }));

        // Create the new workflow definition
        const newDefinition: WorkflowDefinition = {
            name: parsed.name,
            description: parsed.description,
            version: parsed.version,
            triggers: validatedTriggers,
            variables: parsed.variables || {},
            steps: validatedSteps,
            error_handling: parsed.error_handling || {},
        };

        // Update the workflow
        definition.value = newDefinition;

        closeImportDialog();
    } catch (error) {
        if (error instanceof SyntaxError) {
            importError.value = `Invalid JSON: ${error.message}`;
        } else {
            importError.value = `Import error: ${
                error instanceof Error ? error.message : "Unknown error"
            }`;
        }
    }
}

function exportWorkflow() {
    const exportData = { ...definition.value, name: props.workflowName, description: props.workflowDescription };
    const jsonString = JSON.stringify(exportData, null, 2);
    const blob = new Blob([jsonString], { type: "application/json" });
    const url = URL.createObjectURL(blob);
    const link = document.createElement("a");
    link.href = url;
    link.download = `workflow-${exportData.name || props.workflowId || "export"}.json`;
    document.body.appendChild(link);
    link.click();
    document.body.removeChild(link);
    URL.revokeObjectURL(url);
}

// JSON Editor functions
function openJsonEditorDialog() {
    const exportData = { ...definition.value, name: props.workflowName, description: props.workflowDescription };
    jsonEditorText.value = JSON.stringify(exportData, null, 2);
    jsonEditorError.value = "";
    showJsonEditorDialog.value = true;
}

function closeJsonEditorDialog() {
    showJsonEditorDialog.value = false;
    jsonEditorText.value = "";
    jsonEditorError.value = "";
}

function validateAndSaveJsonEdit() {
    jsonEditorError.value = "";

    if (!jsonEditorText.value.trim()) {
        jsonEditorError.value = "Please enter JSON content";
        return;
    }

    try {
        const parsed = JSON.parse(jsonEditorText.value);

        // Validate required structure
        if (!parsed.version) {
            jsonEditorError.value = "Missing required field: version";
            return;
        }

        if (!Array.isArray(parsed.triggers)) {
            jsonEditorError.value = "Missing or invalid triggers array";
            return;
        }

        if (!Array.isArray(parsed.steps)) {
            jsonEditorError.value = "Missing or invalid steps array";
            return;
        }

        // Ensure all triggers have required fields and generate IDs if missing
        const validatedTriggers = parsed.triggers.map((trigger: any) => ({
            id: trigger.id || generateId(),
            name: trigger.name || "",
            event_type: trigger.event_type || "",
            conditions: trigger.conditions || [],
            enabled: trigger.enabled !== false, // default to true
        }));

        // Ensure all steps have required fields and generate IDs if missing
        const validatedSteps = parsed.steps.map((step: any) => ({
            id: step.id || generateId(),
            name: step.name || "",
            type: step.type || "action",
            enabled: step.enabled !== false, // default to true
            config: step.config || {},
            on_error: step.on_error || {
                action: "stop",
                max_retries: 3,
                retry_delay_ms: 1000,
            },
            next_steps: step.next_steps || [],
        }));

        // Create the new workflow definition
        const newDefinition: WorkflowDefinition = {
            name: parsed.name,
            description: parsed.description,
            version: parsed.version,
            triggers: validatedTriggers,
            variables: parsed.variables || {},
            steps: validatedSteps,
            error_handling: parsed.error_handling || {},
        };

        // Update the workflow
        definition.value = newDefinition;

        closeJsonEditorDialog();
    } catch (error) {
        if (error instanceof SyntaxError) {
            jsonEditorError.value = `Invalid JSON: ${error.message}`;
        } else {
            jsonEditorError.value = `Validation error: ${
                error instanceof Error ? error.message : "Unknown error"
            }`;
        }
    }
}

// Nested step management functions
function addNestedStep(fieldKey: string) {
    if (!selectedStep.value) return;

    // Initialize the array if it doesn't exist
    if (!selectedStep.value.config[fieldKey]) {
        selectedStep.value.config[fieldKey] = [];
    }

    // Create a new inline step with default values
    const newStep = {
        id: generateId(),
        name: "",
        type: "action",
        enabled: true,
        config: {},
    };

    selectedStep.value.config[fieldKey].push(newStep);
}

function removeNestedStep(fieldKey: string, index: number) {
    if (!selectedStep.value || !selectedStep.value.config[fieldKey]) return;
    selectedStep.value.config[fieldKey].splice(index, 1);
}

function addStepReference(fieldKey: string, stepName: string) {
    if (!selectedStep.value || !stepName) return;

    // Initialize the array if it doesn't exist
    if (!selectedStep.value.config[fieldKey]) {
        selectedStep.value.config[fieldKey] = [];
    }

    // Add the step name as a reference (string)
    if (!selectedStep.value.config[fieldKey].includes(stepName)) {
        selectedStep.value.config[fieldKey].push(stepName);
    }
}

function editNestedStep(fieldKey: string, index: number) {
    if (!selectedStep.value || !selectedStep.value.config[fieldKey]) return;
    
    const nestedStep = selectedStep.value.config[fieldKey][index];
    
    // Store the editing context
    editingNestedStepContext.value = {
        fieldKey,
        index,
        parentStep: selectedStep.value,
    };
    
    // Create a copy of the nested step for editing
    selectedNestedStep.value = JSON.parse(JSON.stringify(nestedStep));
    
    // Ensure required fields exist
    if (!selectedNestedStep.value.config) {
        selectedNestedStep.value.config = {};
    }
    
    // Initialize default values based on type
    if (selectedNestedStep.value.type === "action" && !selectedNestedStep.value.config.action_type) {
        selectedNestedStep.value.config.action_type = "";
    }
    
    showNestedStepDialog.value = true;
}

function saveNestedStep() {
    if (!selectedNestedStep.value || !editingNestedStepContext.value) return;
    
    const { fieldKey, index, parentStep } = editingNestedStepContext.value;
    
    if (!parentStep || !parentStep.config[fieldKey]) return;
    
    // Update the nested step in the parent's config
    parentStep.config[fieldKey][index] = { ...selectedNestedStep.value };
    
    closeNestedStepDialog();
}

function closeNestedStepDialog() {
    showNestedStepDialog.value = false;
    selectedNestedStep.value = null;
    editingNestedStepContext.value = null;
}

// Move nested step up in the array
function moveNestedStepUp(stepsArray: any[], index: number) {
    if (index > 0 && stepsArray && Array.isArray(stepsArray)) {
        [stepsArray[index - 1], stepsArray[index]] = [
            stepsArray[index],
            stepsArray[index - 1],
        ];
    }
}

// Move nested step down in the array
function moveNestedStepDown(stepsArray: any[], index: number) {
    if (stepsArray && Array.isArray(stepsArray) && index < stepsArray.length - 1) {
        [stepsArray[index], stepsArray[index + 1]] = [
            stepsArray[index + 1],
            stepsArray[index],
        ];
    }
}
</script>

<template>
    <div class="space-y-6">
        <!-- Import/Export Controls -->
        <div class="flex justify-end gap-2">
            <Button @click="openJsonEditorDialog" variant="outline" size="sm">
                <Code class="w-4 h-4 mr-2" />
                Edit JSON
            </Button>
            <Button @click="exportWorkflow" variant="outline" size="sm">
                <FileJson class="w-4 h-4 mr-2" />
                Export JSON
            </Button>
            <Button @click="openImportDialog" variant="outline" size="sm">
                <Upload class="w-4 h-4 mr-2" />
                Import JSON
            </Button>
        </div>

        <Tabs default-value="triggers" class="w-full">
            <TabsList class="grid w-full grid-cols-4">
                <TabsTrigger value="triggers">Triggers</TabsTrigger>
                <TabsTrigger value="steps">Steps</TabsTrigger>
                <TabsTrigger value="variables">Variables</TabsTrigger>
                <TabsTrigger value="settings">Settings</TabsTrigger>
            </TabsList>

            <!-- Triggers Tab -->
            <TabsContent value="triggers" class="space-y-4">
                <div class="flex justify-between items-center">
                    <div>
                        <h3 class="text-lg font-medium">Event Triggers</h3>
                        <p class="text-sm text-muted-foreground">
                            Define what events will start this workflow
                        </p>
                    </div>
                    <Button
                        @click="openTriggerDialog()"
                        variant="outline"
                        size="sm"
                    >
                        <Plus class="w-4 h-4 mr-2" />
                        Add Trigger
                    </Button>
                </div>

                <div
                    v-if="definition.triggers.length === 0"
                    class="text-center py-8 text-muted-foreground"
                >
                    <Zap class="w-16 h-16 mx-auto mb-4 opacity-25" />
                    <p>No triggers configured</p>
                    <p class="text-sm">
                        Add a trigger to define when this workflow should run
                    </p>
                </div>

                <div v-else class="space-y-3">
                    <Card
                        v-for="(trigger, index) in definition.triggers"
                        :key="trigger.id"
                        class="relative"
                    >
                        <CardHeader class="pb-3">
                            <div class="flex justify-between items-start">
                                <div class="flex-1">
                                    <div class="flex items-center gap-2">
                                        <CardTitle class="text-base">{{
                                            trigger.name || "Unnamed Trigger"
                                        }}</CardTitle>
                                        <Badge
                                            :variant="
                                                trigger.enabled
                                                    ? 'default'
                                                    : 'secondary'
                                            "
                                        >
                                            {{
                                                trigger.enabled
                                                    ? "Active"
                                                    : "Inactive"
                                            }}
                                        </Badge>
                                    </div>
                                    <p
                                        class="text-sm text-muted-foreground mt-1"
                                    >
                                        Event:
                                        {{
                                            eventTypes.find(
                                                (et) =>
                                                    et.value ===
                                                    trigger.event_type,
                                            )?.label || trigger.event_type
                                        }}
                                    </p>
                                    <p
                                        v-if="
                                            trigger.conditions &&
                                            trigger.conditions.length > 0
                                        "
                                        class="text-sm text-muted-foreground"
                                    >
                                        {{ trigger.conditions.length }}
                                        condition(s)
                                    </p>
                                </div>
                                <div class="flex gap-1">
                                    <Button
                                        @click="
                                            openTriggerDialog(trigger, index)
                                        "
                                        variant="ghost"
                                        size="sm"
                                    >
                                        <Settings class="w-4 h-4" />
                                    </Button>
                                    <Button
                                        @click="deleteTrigger(index)"
                                        variant="ghost"
                                        size="sm"
                                    >
                                        <Trash2 class="w-4 h-4" />
                                    </Button>
                                </div>
                            </div>
                        </CardHeader>
                    </Card>
                </div>
            </TabsContent>

            <!-- Steps Tab -->
            <TabsContent value="steps" class="space-y-4">
                <div class="flex justify-between items-center">
                    <div>
                        <h3 class="text-lg font-medium">Workflow Steps</h3>
                        <p class="text-sm text-muted-foreground">
                            Define the actions to perform when triggered
                        </p>
                    </div>
                    <Button
                        @click="openStepDialog()"
                        variant="outline"
                        size="sm"
                    >
                        <Plus class="w-4 h-4 mr-2" />
                        Add Step
                    </Button>
                </div>

                <div
                    v-if="definition.steps.length === 0"
                    class="text-center py-8 text-muted-foreground"
                >
                    <Play class="w-16 h-16 mx-auto mb-4 opacity-25" />
                    <p>No steps configured</p>
                    <p class="text-sm">
                        Add steps to define what actions the workflow should
                        perform
                    </p>
                </div>

                <div v-else class="space-y-3">
                    <template v-for="(step, index) in definition.steps" :key="step.id">
                        <Card class="relative">
                            <CardHeader class="pb-3">
                                <div class="flex justify-between items-start">
                                    <div class="flex items-center gap-3 flex-1">
                                        <div
                                            class="flex items-center justify-center w-8 h-8 rounded-full bg-muted"
                                        >
                                            <component
                                                :is="getStepIcon(step.type)"
                                                class="w-4 h-4"
                                            />
                                        </div>
                                        <div class="flex-1">
                                            <div class="flex items-center gap-2">
                                                <CardTitle class="text-base">{{
                                                    step.name || "Unnamed Step"
                                                }}</CardTitle>
                                                <Badge
                                                    variant="outline"
                                                    class="text-xs"
                                                >
                                                    {{
                                                        stepTypes.find(
                                                            (st) =>
                                                                st.value ===
                                                                step.type,
                                                        )?.label || step.type
                                                    }}
                                                </Badge>
                                                <Badge
                                                    :variant="
                                                        step.enabled
                                                            ? 'default'
                                                            : 'secondary'
                                                    "
                                                >
                                                    {{
                                                        step.enabled
                                                            ? "Active"
                                                            : "Inactive"
                                                    }}
                                                </Badge>
                                            </div>
                                            <p
                                                class="text-sm text-muted-foreground mt-1"
                                            >
                                                Step {{ index + 1 }} of
                                                {{ definition.steps.length }}
                                                <span
                                                    v-if="step.config.action_type"
                                                    class="ml-2"
                                                >
                                                    •
                                                    {{
                                                        actionTypes.find(
                                                            (at) =>
                                                                at.value ===
                                                                step.config
                                                                    .action_type,
                                                        )?.label ||
                                                        step.config.action_type
                                                    }}
                                                </span>
                                            </p>
                                        </div>
                                    </div>
                                    <div class="flex gap-1">
                                        <Button
                                            @click="moveStepUp(index)"
                                            variant="ghost"
                                            size="sm"
                                            :disabled="index === 0"
                                        >
                                            ↑
                                        </Button>
                                        <Button
                                            @click="moveStepDown(index)"
                                            variant="ghost"
                                            size="sm"
                                            :disabled="
                                                index ===
                                                definition.steps.length - 1
                                            "
                                        >
                                            ↓
                                        </Button>
                                        <Button
                                            @click="openStepDialog(step, index)"
                                            variant="ghost"
                                            size="sm"
                                        >
                                            <Settings class="w-4 h-4" />
                                        </Button>
                                        <Button
                                            @click="deleteStep(index)"
                                            variant="ghost"
                                            size="sm"
                                        >
                                            <Trash2 class="w-4 h-4" />
                                        </Button>
                                    </div>
                                </div>
                            </CardHeader>

                            <!-- Show nested steps for condition steps -->
                            <CardContent v-if="step.type === 'condition'" class="pt-0">
                                <div class="space-y-3 pl-8 border-l-2 border-border">
                                    <!-- True Branch -->
                                    <div v-if="step.config.true_steps && step.config.true_steps.length > 0" class="space-y-2">
                                        <div class="flex items-center gap-2 text-sm font-medium">
                                            <Badge variant="default" class="rounded-full w-5 h-5 flex items-center justify-center p-0">
                                                <span class="text-xs">✓</span>
                                            </Badge>
                                            <span>If True ({{ step.config.true_steps.length }} step{{ step.config.true_steps.length !== 1 ? 's' : '' }})</span>
                                        </div>
                                        <div class="space-y-1 ml-6">
                                            <div
                                                v-for="(nestedStep, idx) in step.config.true_steps"
                                                :key="idx"
                                                class="flex items-center gap-2 p-2 rounded-md border bg-card text-card-foreground shadow-sm text-sm hover:bg-accent/50 transition-colors group"
                                            >
                                                <component
                                                    :is="getStepIcon(typeof nestedStep === 'string' ? 'action' : nestedStep.type)"
                                                    class="w-3 h-3 text-primary"
                                                />
                                                <span v-if="typeof nestedStep === 'string'" class="flex-1">
                                                    {{ nestedStep }}
                                                    <Badge variant="secondary" class="ml-2 text-xs">Reference</Badge>
                                                </span>
                                                <span v-else class="flex-1">
                                                    {{ nestedStep.name || 'Unnamed' }}
                                                    <Badge variant="secondary" class="ml-2 text-xs">Inline</Badge>
                                                    <span v-if="nestedStep.config?.action_type" class="ml-2 text-xs text-muted-foreground">
                                                        • {{ actionTypes.find(at => at.value === nestedStep.config.action_type)?.label }}
                                                    </span>
                                                </span>
                                                <div class="flex gap-1 opacity-0 group-hover:opacity-100 transition-opacity">
                                                    <Button
                                                        @click="moveNestedStepUp(step.config.true_steps, idx)"
                                                        variant="ghost"
                                                        size="sm"
                                                        class="h-6 w-6 p-0"
                                                        :disabled="idx === 0"
                                                    >
                                                        ↑
                                                    </Button>
                                                    <Button
                                                        @click="moveNestedStepDown(step.config.true_steps, idx)"
                                                        variant="ghost"
                                                        size="sm"
                                                        class="h-6 w-6 p-0"
                                                        :disabled="idx === step.config.true_steps.length - 1"
                                                    >
                                                        ↓
                                                    </Button>
                                                </div>
                                            </div>
                                        </div>
                                    </div>

                                    <!-- False Branch -->
                                    <div v-if="step.config.false_steps && step.config.false_steps.length > 0" class="space-y-2">
                                        <div class="flex items-center gap-2 text-sm font-medium">
                                            <Badge variant="destructive" class="rounded-full w-5 h-5 flex items-center justify-center p-0">
                                                <span class="text-xs">✕</span>
                                            </Badge>
                                            <span>If False ({{ step.config.false_steps.length }} step{{ step.config.false_steps.length !== 1 ? 's' : '' }})</span>
                                        </div>
                                        <div class="space-y-1 ml-6">
                                            <div
                                                v-for="(nestedStep, idx) in step.config.false_steps"
                                                :key="idx"
                                                class="flex items-center gap-2 p-2 rounded-md border bg-card text-card-foreground shadow-sm text-sm hover:bg-accent/50 transition-colors group"
                                            >
                                                <component
                                                    :is="getStepIcon(typeof nestedStep === 'string' ? 'action' : nestedStep.type)"
                                                    class="w-3 h-3 text-primary"
                                                />
                                                <span v-if="typeof nestedStep === 'string'" class="flex-1">
                                                    {{ nestedStep }}
                                                    <Badge variant="secondary" class="ml-2 text-xs">Reference</Badge>
                                                </span>
                                                <span v-else class="flex-1">
                                                    {{ nestedStep.name || 'Unnamed' }}
                                                    <Badge variant="secondary" class="ml-2 text-xs">Inline</Badge>
                                                    <span v-if="nestedStep.config?.action_type" class="ml-2 text-xs text-muted-foreground">
                                                        • {{ actionTypes.find(at => at.value === nestedStep.config.action_type)?.label }}
                                                    </span>
                                                </span>
                                                <div class="flex gap-1 opacity-0 group-hover:opacity-100 transition-opacity">
                                                    <Button
                                                        @click="moveNestedStepUp(step.config.false_steps, idx)"
                                                        variant="ghost"
                                                        size="sm"
                                                        class="h-6 w-6 p-0"
                                                        :disabled="idx === 0"
                                                    >
                                                        ↑
                                                    </Button>
                                                    <Button
                                                        @click="moveNestedStepDown(step.config.false_steps, idx)"
                                                        variant="ghost"
                                                        size="sm"
                                                        class="h-6 w-6 p-0"
                                                        :disabled="idx === step.config.false_steps.length - 1"
                                                    >
                                                        ↓
                                                    </Button>
                                                </div>
                                            </div>
                                        </div>
                                    </div>

                                    <!-- No branches configured -->
                                    <div v-if="(!step.config.true_steps || step.config.true_steps.length === 0) && (!step.config.false_steps || step.config.false_steps.length === 0)" class="text-sm text-muted-foreground italic py-2">
                                        No conditional branches configured
                                    </div>
                                </div>
                            </CardContent>
                        </Card>
                    </template>
                </div>
            </TabsContent>

            <!-- Variables Tab -->
            <TabsContent value="variables" class="space-y-4">
                <div class="flex justify-between items-center">
                    <div>
                        <h3 class="text-lg font-medium">Workflow Variables</h3>
                        <p class="text-sm text-muted-foreground">
                            Default variables available to all steps
                        </p>
                    </div>
                    <Button
                        @click="openVariableDialog()"
                        variant="outline"
                        size="sm"
                    >
                        <Plus class="w-4 h-4 mr-2" />
                        Add Variable
                    </Button>
                </div>

                <div
                    v-if="
                        !definition.variables ||
                        Object.keys(definition.variables).length === 0
                    "
                    class="text-center py-8 text-muted-foreground"
                >
                    <Variable class="w-16 h-16 mx-auto mb-4 opacity-25" />
                    <p>No variables defined</p>
                    <p class="text-sm">
                        Add variables to store values that can be used across
                        workflow steps
                    </p>
                </div>

                <div v-else class="space-y-3">
                    <Card
                        v-for="(value, key) in definition.variables"
                        :key="key"
                        class="relative"
                    >
                        <CardHeader class="pb-3">
                            <div class="flex justify-between items-start">
                                <div class="flex items-center gap-3">
                                    <div
                                        class="flex items-center justify-center w-8 h-8 rounded-full bg-muted"
                                    >
                                        <Variable class="w-4 h-4" />
                                    </div>
                                    <div class="flex-1">
                                        <div class="flex items-center gap-2">
                                            <CardTitle
                                                class="text-base font-mono"
                                                >{{ key }}</CardTitle
                                            >
                                            <Badge
                                                variant="outline"
                                                class="text-xs"
                                            >
                                                {{ getVariableType(value) }}
                                            </Badge>
                                        </div>
                                        <p
                                            class="text-sm text-muted-foreground mt-1 font-mono"
                                        >
                                            {{ getVariableDisplayValue(value) }}
                                        </p>
                                    </div>
                                </div>
                                <div class="flex gap-1">
                                    <Button
                                        @click="openVariableDialog(key)"
                                        variant="ghost"
                                        size="sm"
                                    >
                                        <Settings class="w-4 h-4" />
                                    </Button>
                                    <Button
                                        @click="deleteVariable(key)"
                                        variant="ghost"
                                        size="sm"
                                    >
                                        <Trash2 class="w-4 h-4" />
                                    </Button>
                                </div>
                            </div>
                        </CardHeader>
                    </Card>
                </div>
            </TabsContent>

            <!-- Settings Tab -->
            <TabsContent value="settings" class="space-y-4">
                <div>
                    <h3 class="text-lg font-medium">Workflow Settings</h3>
                    <p class="text-sm text-muted-foreground">
                        Configure error handling and other workflow options
                    </p>
                </div>

                <Card>
                    <CardHeader>
                        <CardTitle class="text-base">Error Handling</CardTitle>
                    </CardHeader>
                    <CardContent class="space-y-4">
                        <div class="grid grid-cols-2 gap-4">
                            <div class="space-y-2">
                                <Label>Default Action on Error</Label>
                                <Select
                                    :modelValue="errorHandling.default_action"
                                    @update:modelValue="
                                        updateErrorHandling(
                                            'default_action',
                                            $event,
                                        )
                                    "
                                >
                                    <SelectTrigger>
                                        <SelectValue />
                                    </SelectTrigger>
                                    <SelectContent>
                                        <SelectItem
                                            v-for="action in errorActions"
                                            :key="action.value"
                                            :value="action.value"
                                        >
                                            {{ action.label }}
                                        </SelectItem>
                                    </SelectContent>
                                </Select>
                            </div>
                            <div class="space-y-2">
                                <Label>Max Retries</Label>
                                <Input
                                    :modelValue="errorHandling.max_retries"
                                    @update:modelValue="
                                        updateErrorHandling(
                                            'max_retries',
                                            parseInt($event as string),
                                        )
                                    "
                                    type="number"
                                    min="0"
                                    max="10"
                                />
                            </div>
                        </div>
                        <div class="space-y-2">
                            <Label>Retry Delay (milliseconds)</Label>
                            <Input
                                :modelValue="errorHandling.retry_delay_ms"
                                @update:modelValue="
                                    updateErrorHandling(
                                        'retry_delay_ms',
                                        parseInt($event as string),
                                    )
                                "
                                type="number"
                                min="100"
                                step="100"
                            />
                        </div>
                    </CardContent>
                </Card>
            </TabsContent>
        </Tabs>

        <!-- Trigger Dialog -->
        <Dialog v-model:open="showTriggerDialog">
            <DialogContent v-if="selectedTrigger" class="max-w-2xl">
                <DialogHeader>
                    <DialogTitle>
                        {{
                            editingTriggerIndex >= 0
                                ? "Edit Trigger"
                                : "Add Trigger"
                        }}
                    </DialogTitle>
                    <DialogDescription>
                        Configure when this workflow should be triggered
                    </DialogDescription>
                </DialogHeader>

                <div class="space-y-4">
                    <div class="grid grid-cols-2 gap-4">
                        <div class="space-y-2">
                            <Label>Name</Label>
                            <Input
                                v-model="selectedTrigger.name"
                                placeholder="Enter trigger name"
                            />
                        </div>
                        <div class="space-y-2">
                            <Label>Event Type</Label>
                            <Select v-model="selectedTrigger.event_type">
                                <SelectTrigger>
                                    <SelectValue
                                        placeholder="Select event type"
                                    />
                                </SelectTrigger>
                                <SelectContent>
                                    <SelectItem
                                        v-for="eventType in eventTypes"
                                        :key="eventType.value"
                                        :value="eventType.value"
                                    >
                                        {{ eventType.label }}
                                    </SelectItem>
                                </SelectContent>
                            </Select>
                        </div>
                    </div>

                    <div class="flex items-center space-x-2">
                        <Switch
                            :model-value="selectedTrigger?.enabled"
                            @update:model-value="
                                (val) =>
                                    selectedTrigger &&
                                    (selectedTrigger.enabled = val)
                            "
                        />
                        <Label>Enable this trigger</Label>
                    </div>

                    <Separator />

                    <div>
                        <div class="flex justify-between items-center mb-2">
                            <Label>Conditions (optional)</Label>
                            <Button
                                @click="addCondition"
                                variant="outline"
                                size="sm"
                            >
                                <Plus class="w-4 h-4 mr-2" />
                                Add Condition
                            </Button>
                        </div>

                        <div
                            v-if="
                                !selectedTrigger.conditions ||
                                selectedTrigger.conditions.length === 0
                            "
                            class="text-sm text-muted-foreground py-4"
                        >
                            No conditions - trigger will activate for all events
                            of this type
                        </div>

                        <div v-else class="space-y-3">
                            <Card
                                v-for="(
                                    condition, index
                                ) in selectedTrigger.conditions"
                                :key="index"
                                class="p-4"
                            >
                                <div class="grid grid-cols-4 gap-2 items-end">
                                    <div class="space-y-1">
                                        <Label class="text-xs">Field</Label>
                                        <Input
                                            v-model="condition.field"
                                            placeholder="e.g. event.player_name"
                                            class="text-sm"
                                        />
                                    </div>
                                    <div class="space-y-1">
                                        <Label class="text-xs">Operator</Label>
                                        <Select v-model="condition.operator">
                                            <SelectTrigger class="text-sm">
                                                <SelectValue />
                                            </SelectTrigger>
                                            <SelectContent>
                                                <SelectItem
                                                    v-for="op in operators"
                                                    :key="op.value"
                                                    :value="op.value"
                                                >
                                                    {{ op.label }}
                                                </SelectItem>
                                            </SelectContent>
                                        </Select>
                                    </div>
                                    <div class="space-y-1">
                                        <Label class="text-xs">Value</Label>
                                        <Input
                                            v-model="condition.value"
                                            class="text-sm"
                                        />
                                    </div>
                                    <Button
                                        @click="removeCondition(index)"
                                        variant="ghost"
                                        size="sm"
                                    >
                                        <Trash2 class="w-4 h-4" />
                                    </Button>
                                </div>
                            </Card>
                        </div>
                    </div>
                </div>

                <DialogFooter>
                    <Button variant="outline" @click="closeTriggerDialog"
                        >Cancel</Button
                    >
                    <Button @click="saveTrigger">
                        {{ editingTriggerIndex >= 0 ? "Update" : "Add" }}
                        Trigger
                    </Button>
                </DialogFooter>
            </DialogContent>
        </Dialog>

        <!-- Step Dialog -->
        <Dialog v-model:open="showStepDialog">
            <DialogContent
                v-if="selectedStep"
                class="max-w-3xl max-h-[80vh] overflow-y-auto"
            >
                <DialogHeader>
                    <DialogTitle>
                        {{ editingStepIndex >= 0 ? "Edit Step" : "Add Step" }}
                    </DialogTitle>
                    <DialogDescription>
                        Configure a step in your workflow
                    </DialogDescription>
                </DialogHeader>

                <div class="space-y-4">
                    <div class="grid grid-cols-2 gap-4">
                        <div class="space-y-2">
                            <Label>Name</Label>
                            <Input
                                v-model="selectedStep.name"
                                placeholder="Enter step name"
                            />
                        </div>
                        <div class="space-y-2">
                            <Label>Type</Label>
                            <Select v-model="selectedStep.type">
                                <SelectTrigger>
                                    <SelectValue>
                                        {{
                                            stepTypes.find(
                                                (st) =>
                                                    st.value ===
                                                    selectedStep.type,
                                            )?.label || selectedStep.type
                                        }}
                                    </SelectValue>
                                </SelectTrigger>
                                <SelectContent>
                                    <SelectItem
                                        v-for="stepType in stepTypes"
                                        :key="stepType.value"
                                        :value="stepType.value"
                                    >
                                        <div>
                                            <div class="font-medium">
                                                {{ stepType.label }}
                                            </div>
                                            <div
                                                class="text-sm text-muted-foreground"
                                            >
                                                {{ stepType.description }}
                                            </div>
                                        </div>
                                    </SelectItem>
                                </SelectContent>
                            </Select>
                        </div>
                    </div>

                    <div class="flex items-center space-x-2">
                        <Switch
                            :model-value="selectedStep?.enabled"
                            @update:model-value="
                                (val) =>
                                    selectedStep && (selectedStep.enabled = val)
                            "
                        />
                        <Label>Enable this step</Label>
                    </div>

                    <!-- Action Type Selection (for action steps) -->
                    <div
                        v-if="selectedStep.type === 'action'"
                        class="space-y-2"
                    >
                        <Label>Action Type</Label>
                        <Select v-model="selectedStep.config.action_type">
                            <SelectTrigger>
                                <SelectValue placeholder="Select action type">
                                    {{
                                        actionTypes.find(
                                            (at) =>
                                                at.value ===
                                                selectedStep.config.action_type,
                                        )?.label ||
                                        selectedStep.config.action_type
                                    }}
                                </SelectValue>
                            </SelectTrigger>
                            <SelectContent>
                                <SelectItem
                                    v-for="actionType in actionTypes"
                                    :key="actionType.value"
                                    :value="actionType.value"
                                >
                                    <div>
                                        <div class="font-medium">
                                            {{ actionType.label }}
                                        </div>
                                        <div
                                            class="text-sm text-muted-foreground"
                                        >
                                            {{ actionType.description }}
                                        </div>
                                    </div>
                                </SelectItem>
                            </SelectContent>
                        </Select>
                    </div>

                    <!-- Dynamic Configuration Fields -->
                    <div
                        v-if="
                            selectedStep.config.action_type ||
                            selectedStep.type !== 'action'
                        "
                        class="space-y-4"
                    >
                        <Separator />
                        <div class="space-y-3">
                            <Label class="text-base">Configuration</Label>
                            <div
                                v-for="field in getConfigFields(
                                    selectedStep.type,
                                    selectedStep.config.action_type,
                                )"
                                :key="field.key"
                                v-show="
                                    shouldShowField(field, selectedStep.config)
                                "
                                class="space-y-2"
                            >
                                <div class="flex flex-col space-y-1">
                                    <Label
                                        >{{ field.label }}
                                        <span
                                            v-if="field.required"
                                            class="text-red-500"
                                            >*</span
                                        ></Label
                                    >
                                    <p
                                        v-if="(field as any).description"
                                        class="text-xs text-muted-foreground"
                                    >
                                        {{ (field as any).description }}
                                    </p>
                                </div>

                                <!-- Text Input -->
                                <Input
                                    v-if="field.type === 'text'"
                                    v-model="selectedStep.config[field.key]"
                                    :placeholder="(field as any).placeholder"
                                    :required="field.required"
                                />

                                <!-- Number Input -->
                                <Input
                                    v-else-if="field.type === 'number'"
                                    v-model.number="
                                        selectedStep.config[field.key]
                                    "
                                    type="number"
                                    :placeholder="(field as any).placeholder"
                                    :required="field.required"
                                />

                                <!-- Textarea -->
                                <Textarea
                                    v-else-if="field.type === 'textarea'"
                                    v-model="selectedStep.config[field.key]"
                                    :rows="(field as any).rows || 3"
                                    :placeholder="(field as any).placeholder"
                                    :required="field.required"
                                    class="font-mono text-sm"
                                />

                                <!-- Lua Script Editor -->
                                <div
                                    v-else-if="field.type === 'lua'"
                                    class="space-y-2"
                                >
                                    <div class="h-96 mb-2">
                                        <CodeEditor
                                            v-model:value="
                                                selectedStep.config[field.key]
                                            "
                                            language="lua"
                                            theme="vs-dark"
                                            :options="{
                                                fontSize: 14,
                                                minimap: { enabled: true },
                                                automaticLayout: true,
                                            }"
                                        />
                                    </div>
                                </div>

                                <!-- Select -->
                                <Select
                                    v-else-if="field.type === 'select'"
                                    v-model="selectedStep.config[field.key]"
                                >
                                    <SelectTrigger>
                                        <SelectValue
                                            :placeholder="
                                                (field as any).placeholder
                                            "
                                        />
                                    </SelectTrigger>
                                    <SelectContent>
                                        <SelectItem
                                            v-for="option in (field as any)
                                                .options"
                                            :key="option"
                                            :value="option"
                                        >
                                            {{ option }}
                                        </SelectItem>
                                    </SelectContent>
                                </Select>

                                <!-- Conditions Array -->
                                <div v-else-if="field.type === 'multi_select'">
                                    <div class="space-y-2">
                                        <div
                                            v-if="
                                                selectedStep &&
                                                selectedStep.config[
                                                    field.key
                                                ] &&
                                                selectedStep.config[field.key]
                                                    .length > 0
                                            "
                                            class="flex flex-wrap gap-2 mb-2"
                                        >
                                            <Badge
                                                v-for="(
                                                    item, idx
                                                ) in selectedStep.config[
                                                    field.key
                                                ]"
                                                :key="idx"
                                                variant="secondary"
                                                class="flex items-center gap-1"
                                            >
                                                {{ item }}
                                                <button
                                                    @click="
                                                        selectedStep &&
                                                        selectedStep.config[
                                                            field.key
                                                        ].splice(idx, 1)
                                                    "
                                                    class="ml-1 hover:text-destructive"
                                                >
                                                    <Trash2 class="w-3 h-3" />
                                                </button>
                                            </Badge>
                                        </div>
                                        <Select
                                            :modelValue="undefined"
                                            @update:modelValue="
                                                (value: any) => {
                                                    if (!selectedStep || !value)
                                                        return;
                                                    if (
                                                        !selectedStep.config[
                                                            field.key
                                                        ]
                                                    ) {
                                                        selectedStep.config[
                                                            field.key
                                                        ] = [];
                                                    }
                                                    if (
                                                        !selectedStep.config[
                                                            field.key
                                                        ].includes(value)
                                                    ) {
                                                        selectedStep.config[
                                                            field.key
                                                        ].push(value);
                                                    }
                                                }
                                            "
                                        >
                                            <SelectTrigger>
                                                <SelectValue
                                                    placeholder="Select step to add..."
                                                />
                                            </SelectTrigger>
                                            <SelectContent>
                                                <SelectItem
                                                    v-if="
                                                        !(field as any)
                                                            .options ||
                                                        (field as any).options
                                                            .length === 0
                                                    "
                                                    value=""
                                                    disabled
                                                >
                                                    No steps available
                                                </SelectItem>
                                                <SelectItem
                                                    v-for="option in (
                                                        field as any
                                                    ).options"
                                                    :key="option.value"
                                                    :value="option.value"
                                                >
                                                    {{ option.label }}
                                                </SelectItem>
                                            </SelectContent>
                                        </Select>
                                        <p
                                            v-if="(field as any).description"
                                            class="text-xs text-muted-foreground"
                                        >
                                            {{ (field as any).description }}
                                        </p>
                                    </div>
                                </div>
                                <div
                                    v-else-if="
                                        field.type === 'conditions_array'
                                    "
                                    class="space-y-3"
                                >
                                    <div
                                        class="flex justify-between items-center"
                                    >
                                        <Label>Conditions</Label>
                                        <Button
                                            @click="addStepCondition"
                                            variant="outline"
                                            size="sm"
                                        >
                                            <Plus class="w-4 h-4 mr-2" />
                                            Add Condition
                                        </Button>
                                    </div>

                                    <div
                                        v-if="
                                            !selectedStep.config[field.key] ||
                                            selectedStep.config[field.key]
                                                .length === 0
                                        "
                                        class="text-sm text-muted-foreground py-4 text-center"
                                    >
                                        No conditions defined
                                    </div>

                                    <div v-else class="space-y-2">
                                        <Card
                                            v-for="(
                                                condition, index
                                            ) in selectedStep.config[field.key]"
                                            :key="index"
                                            class="p-3"
                                        >
                                            <div
                                                class="grid grid-cols-4 gap-2 items-end"
                                            >
                                                <div class="space-y-1">
                                                    <Label class="text-xs"
                                                        >Field</Label
                                                    >
                                                    <Input
                                                        v-model="
                                                            condition.field
                                                        "
                                                        placeholder="e.g. trigger_event.player_name"
                                                        class="text-sm"
                                                    />
                                                </div>
                                                <div class="space-y-1">
                                                    <Label class="text-xs"
                                                        >Operator</Label
                                                    >
                                                    <Select
                                                        v-model="
                                                            condition.operator
                                                        "
                                                    >
                                                        <SelectTrigger
                                                            class="text-sm"
                                                        >
                                                            <SelectValue />
                                                        </SelectTrigger>
                                                        <SelectContent>
                                                            <SelectItem
                                                                v-for="op in operators"
                                                                :key="op.value"
                                                                :value="
                                                                    op.value
                                                                "
                                                            >
                                                                {{ op.label }}
                                                            </SelectItem>
                                                        </SelectContent>
                                                    </Select>
                                                </div>
                                                <div class="space-y-1">
                                                    <Label class="text-xs"
                                                        >Value</Label
                                                    >
                                                    <Input
                                                        v-model="
                                                            condition.value
                                                        "
                                                        class="text-sm"
                                                        placeholder="Value to compare"
                                                    />
                                                </div>
                                                <Button
                                                    @click="
                                                        removeStepCondition(
                                                            index,
                                                        )
                                                    "
                                                    variant="ghost"
                                                    size="sm"
                                                >
                                                    <Trash2 class="w-4 h-4" />
                                                </Button>
                                            </div>
                                        </Card>
                                    </div>
                                </div>
                                
                                <!-- Nested Steps Editor -->
                                <div
                                    v-else-if="field.type === 'nested_steps'"
                                    class="space-y-3"
                                >
                                    <div class="border rounded-lg p-4 bg-muted/30">
                                        <div class="flex justify-between items-center mb-3">
                                            <div>
                                                <Label class="text-sm font-medium">{{ field.label }}</Label>
                                                <p class="text-xs text-muted-foreground mt-1">
                                                    {{ field.description }}
                                                </p>
                                            </div>
                                            <Button
                                                @click="addNestedStep(field.key)"
                                                variant="outline"
                                                size="sm"
                                            >
                                                <Plus class="w-4 h-4 mr-2" />
                                                Add Inline Step
                                            </Button>
                                        </div>

                                        <!-- Initialize array if not exists -->
                                        <template v-if="!selectedStep.config[field.key]">
                                            {{ selectedStep.config[field.key] = [] }}
                                        </template>

                                        <!-- Show nested steps -->
                                        <div
                                            v-if="selectedStep.config[field.key].length === 0"
                                            class="text-center py-6 text-muted-foreground text-sm"
                                        >
                                            <Play class="w-12 h-12 mx-auto mb-2 opacity-25" />
                                            <p>No inline steps defined</p>
                                            <p class="text-xs mt-1">Add steps or reference existing ones</p>
                                        </div>

                                        <div v-else class="space-y-2">
                                            <Card
                                                v-for="(nestedStep, idx) in selectedStep.config[field.key]"
                                                :key="idx"
                                                class="p-3 bg-background"
                                            >
                                                <!-- Check if it's a string reference or inline step -->
                                                <div v-if="typeof nestedStep === 'string'" class="flex items-center gap-2">
                                                    <Badge variant="outline" class="flex-1">
                                                        <GitBranch class="w-3 h-3 mr-1" />
                                                        Reference: {{ nestedStep }}
                                                    </Badge>
                                                    <div class="flex gap-1">
                                                        <Button
                                                            @click="moveNestedStepUp(selectedStep.config[field.key], idx)"
                                                            variant="ghost"
                                                            size="sm"
                                                            :disabled="idx === 0"
                                                        >
                                                            ↑
                                                        </Button>
                                                        <Button
                                                            @click="moveNestedStepDown(selectedStep.config[field.key], idx)"
                                                            variant="ghost"
                                                            size="sm"
                                                            :disabled="idx === selectedStep.config[field.key].length - 1"
                                                        >
                                                            ↓
                                                        </Button>
                                                        <Button
                                                            @click="removeNestedStep(field.key, idx)"
                                                            variant="ghost"
                                                            size="sm"
                                                        >
                                                            <Trash2 class="w-4 h-4" />
                                                        </Button>
                                                    </div>
                                                </div>
                                                
                                                <!-- Inline step -->
                                                <div v-else class="space-y-2">
                                                    <div class="flex items-center justify-between">
                                                        <div class="flex items-center gap-2 flex-1">
                                                            <component
                                                                :is="getStepIcon(nestedStep.type || 'action')"
                                                                class="w-4 h-4 text-primary"
                                                            />
                                                            <Input
                                                                v-model="nestedStep.name"
                                                                placeholder="Step name"
                                                                class="text-sm h-8"
                                                            />
                                                            <Select v-model="nestedStep.type" class="w-32">
                                                                <SelectTrigger class="h-8 text-sm">
                                                                    <SelectValue />
                                                                </SelectTrigger>
                                                                <SelectContent>
                                                                    <SelectItem
                                                                        v-for="st in stepTypes.filter(t => t.value !== 'condition')"
                                                                        :key="st.value"
                                                                        :value="st.value"
                                                                    >
                                                                        {{ st.label }}
                                                                    </SelectItem>
                                                                </SelectContent>
                                                            </Select>
                                                        </div>
                                                        <div class="flex gap-1">
                                                            <Button
                                                                @click="moveNestedStepUp(selectedStep.config[field.key], idx)"
                                                                variant="ghost"
                                                                size="sm"
                                                                :disabled="idx === 0"
                                                            >
                                                                ↑
                                                            </Button>
                                                            <Button
                                                                @click="moveNestedStepDown(selectedStep.config[field.key], idx)"
                                                                variant="ghost"
                                                                size="sm"
                                                                :disabled="idx === selectedStep.config[field.key].length - 1"
                                                            >
                                                                ↓
                                                            </Button>
                                                            <Button
                                                                @click="editNestedStep(field.key, idx)"
                                                                variant="ghost"
                                                                size="sm"
                                                            >
                                                                <Settings class="w-4 h-4" />
                                                            </Button>
                                                            <Button
                                                                @click="removeNestedStep(field.key, idx)"
                                                                variant="ghost"
                                                                size="sm"
                                                            >
                                                                <Trash2 class="w-4 h-4" />
                                                            </Button>
                                                        </div>
                                                    </div>
                                                    
                                                    <!-- Quick config preview -->
                                                    <div v-if="nestedStep.type === 'action' && nestedStep.config?.action_type" class="text-xs text-muted-foreground ml-6">
                                                        Action: {{ actionTypes.find(at => at.value === nestedStep.config.action_type)?.label || nestedStep.config.action_type }}
                                                    </div>
                                                </div>
                                            </Card>
                                        </div>

                                        <!-- Option to add reference to existing step -->
                                        <div class="mt-3 pt-3 border-t">
                                            <Label class="text-xs text-muted-foreground mb-2 block">Or reference existing step:</Label>
                                            <Select
                                                :modelValue="undefined"
                                                @update:modelValue="(value) => addStepReference(field.key, value)"
                                            >
                                                <SelectTrigger class="h-8 text-sm">
                                                    <SelectValue placeholder="Select step to reference..." />
                                                </SelectTrigger>
                                                <SelectContent>
                                                    <SelectItem
                                                        v-if="!(field as any).options || (field as any).options.length === 0"
                                                        value=""
                                                        disabled
                                                    >
                                                        No steps available
                                                    </SelectItem>
                                                    <SelectItem
                                                        v-for="option in (field as any).options"
                                                        :key="option.value"
                                                        :value="option.value"
                                                    >
                                                        {{ option.label }}
                                                    </SelectItem>
                                                </SelectContent>
                                            </Select>
                                        </div>
                                    </div>
                                </div>
                            </div>
                        </div>
                    </div>

                    <!-- Error Handling -->
                    <div class="space-y-4">
                        <Separator />
                        <div class="space-y-3">
                            <Label class="text-base"
                                >Error Handling (optional)</Label
                            >
                            <div class="grid grid-cols-3 gap-4">
                                <div class="space-y-2">
                                    <Label class="text-sm">On Error</Label>
                                    <Select
                                        v-model="selectedStep.on_error.action"
                                    >
                                        <SelectTrigger>
                                            <SelectValue />
                                        </SelectTrigger>
                                        <SelectContent>
                                            <SelectItem
                                                v-for="action in errorActions"
                                                :key="action.value"
                                                :value="action.value"
                                            >
                                                {{ action.label }}
                                            </SelectItem>
                                        </SelectContent>
                                    </Select>
                                </div>
                                <div class="space-y-2">
                                    <Label class="text-sm">Max Retries</Label>
                                    <Input
                                        v-model.number="
                                            selectedStep.on_error.max_retries
                                        "
                                        type="number"
                                        min="0"
                                        max="10"
                                    />
                                </div>
                                <div class="space-y-2">
                                    <Label class="text-sm"
                                        >Retry Delay (ms)</Label
                                    >
                                    <Input
                                        v-model.number="
                                            selectedStep.on_error.retry_delay_ms
                                        "
                                        type="number"
                                        min="100"
                                        step="100"
                                    />
                                </div>
                            </div>
                        </div>
                    </div>

                    <!-- Variable Usage Help -->
                    <div
                        v-if="
                            selectedStep.type === 'action' &&
                            selectedStep.config.action_type &&
                            selectedStep.config.action_type !== 'lua_script'
                        "
                        class="space-y-2"
                    >
                        <Separator />
                        <div class="bg-muted p-3 rounded-md">
                            <p class="text-sm font-medium mb-2">
                                💡 Variable Usage in Text Fields
                            </p>
                            <div class="text-xs space-y-1">
                                <p>
                                    <strong>Trigger Event Data:</strong>
                                    <code>${trigger_event.player_name}</code>,
                                    <code>${trigger_event.message}</code>, etc.
                                </p>
                                <p>
                                    <strong>Metadata:</strong>
                                    <code>${metadata.workflow_name}</code>,
                                    <code>${metadata.execution_id}</code>, etc.
                                </p>
                                <p>
                                    <strong>Variables:</strong>
                                    <code>${variable_name}</code> for any
                                    workflow variables
                                </p>
                                <p class="text-muted-foreground mt-2">
                                    Example: "Player
                                    ${trigger_event.player_name} said:
                                    ${trigger_event.message}"
                                </p>
                            </div>
                        </div>
                    </div>
                </div>

                <DialogFooter>
                    <Button variant="outline" @click="closeStepDialog"
                        >Cancel</Button
                    >
                    <Button @click="saveStep">
                        {{ editingStepIndex >= 0 ? "Update" : "Add" }} Step
                    </Button>
                </DialogFooter>
            </DialogContent>
        </Dialog>

        <!-- Variable Dialog -->
        <Dialog v-model:open="showVariableDialog">
            <DialogContent v-if="selectedVariable" class="max-w-lg">
                <DialogHeader>
                    <DialogTitle>
                        {{
                            editingVariableKey
                                ? "Edit Variable"
                                : "Add Variable"
                        }}
                    </DialogTitle>
                    <DialogDescription>
                        Configure a workflow variable that can be used in steps
                    </DialogDescription>
                </DialogHeader>

                <div class="space-y-4">
                    <div class="space-y-2">
                        <Label>Variable Name</Label>
                        <Input
                            v-model="selectedVariable.key"
                            placeholder="e.g. max_retries, server_name"
                            pattern="[a-zA-Z_][a-zA-Z0-9_]*"
                            title="Variable names must start with a letter or underscore, followed by letters, numbers, or underscores"
                        />
                    </div>

                    <div class="space-y-2">
                        <Label>Type</Label>
                        <Select v-model="selectedVariable.type">
                            <SelectTrigger>
                                <SelectValue />
                            </SelectTrigger>
                            <SelectContent>
                                <SelectItem
                                    v-for="type in valueTypes"
                                    :key="type.value"
                                    :value="type.value"
                                >
                                    {{ type.label }}
                                </SelectItem>
                            </SelectContent>
                        </Select>
                    </div>

                    <div class="space-y-2">
                        <Label>Value</Label>

                        <!-- String Input -->
                        <Input
                            v-if="selectedVariable.type === 'string'"
                            v-model="selectedVariable.value"
                            placeholder="Enter text value"
                        />

                        <!-- Number Input -->
                        <Input
                            v-else-if="selectedVariable.type === 'number'"
                            v-model.number="selectedVariable.value"
                            type="number"
                            step="any"
                            placeholder="Enter number"
                        />

                        <!-- Boolean Select -->
                        <Select
                            v-else-if="selectedVariable.type === 'boolean'"
                            v-model="selectedVariable.value"
                        >
                            <SelectTrigger>
                                <SelectValue />
                            </SelectTrigger>
                            <SelectContent>
                                <SelectItem value="true">true</SelectItem>
                                <SelectItem value="false">false</SelectItem>
                            </SelectContent>
                        </Select>

                        <!-- Array/Object Textarea -->
                        <div v-else class="space-y-2">
                            <Textarea
                                v-model="selectedVariable.value"
                                :placeholder="
                                    selectedVariable.type === 'array'
                                        ? '[&quot;item1&quot;, &quot;item2&quot;]'
                                        : '{&quot;key&quot;: &quot;value&quot;}'
                                "
                                rows="4"
                                class="font-mono text-sm"
                            />
                            <p class="text-xs text-muted-foreground">
                                Enter valid JSON for
                                {{
                                    selectedVariable.type === "array"
                                        ? "array"
                                        : "object"
                                }}
                                values
                            </p>
                        </div>
                    </div>

                    <!-- Preview -->
                    <div
                        v-if="selectedVariable.key && selectedVariable.value"
                        class="space-y-2"
                    >
                        <Label>Preview</Label>
                        <div class="p-3 bg-muted rounded-md">
                            <code class="text-sm">
                                {{ selectedVariable.key }}:
                                {{
                                    getVariableDisplayValue(
                                        selectedVariable.value,
                                    )
                                }}
                            </code>
                        </div>
                    </div>
                </div>

                <DialogFooter>
                    <Button variant="outline" @click="closeVariableDialog"
                        >Cancel</Button
                    >
                    <Button
                        @click="saveVariable"
                        :disabled="!selectedVariable.key.trim()"
                    >
                        {{ editingVariableKey ? "Update" : "Add" }} Variable
                    </Button>
                </DialogFooter>
            </DialogContent>
        </Dialog>

        <!-- Nested Step Dialog -->
        <Dialog v-model:open="showNestedStepDialog">
            <DialogContent
                v-if="selectedNestedStep"
                class="max-w-2xl max-h-[80vh] overflow-y-auto"
            >
                <DialogHeader>
                    <DialogTitle>Edit Nested Step</DialogTitle>
                    <DialogDescription>
                        Configure this inline step within the condition branch
                    </DialogDescription>
                </DialogHeader>

                <div class="space-y-4">
                    <div class="grid grid-cols-2 gap-4">
                        <div class="space-y-2">
                            <Label>Name</Label>
                            <Input
                                v-model="selectedNestedStep.name"
                                placeholder="Enter step name"
                            />
                        </div>
                        <div class="space-y-2">
                            <Label>Type</Label>
                            <Select v-model="selectedNestedStep.type">
                                <SelectTrigger>
                                    <SelectValue />
                                </SelectTrigger>
                                <SelectContent>
                                    <SelectItem
                                        v-for="stepType in stepTypes.filter(t => t.value !== 'condition')"
                                        :key="stepType.value"
                                        :value="stepType.value"
                                    >
                                        <div>
                                            <div class="font-medium">
                                                {{ stepType.label }}
                                            </div>
                                            <div
                                                class="text-sm text-muted-foreground"
                                            >
                                                {{ stepType.description }}
                                            </div>
                                        </div>
                                    </SelectItem>
                                </SelectContent>
                            </Select>
                        </div>
                    </div>

                    <!-- Action Type Selection (for action steps) -->
                    <div
                        v-if="selectedNestedStep.type === 'action'"
                        class="space-y-2"
                    >
                        <Label>Action Type</Label>
                        <Select v-model="selectedNestedStep.config.action_type">
                            <SelectTrigger>
                                <SelectValue placeholder="Select action type" />
                            </SelectTrigger>
                            <SelectContent>
                                <SelectItem
                                    v-for="actionType in actionTypes"
                                    :key="actionType.value"
                                    :value="actionType.value"
                                >
                                    <div>
                                        <div class="font-medium">
                                            {{ actionType.label }}
                                        </div>
                                        <div
                                            class="text-sm text-muted-foreground"
                                        >
                                            {{ actionType.description }}
                                        </div>
                                    </div>
                                </SelectItem>
                            </SelectContent>
                        </Select>
                    </div>

                    <!-- Dynamic Configuration Fields -->
                    <div
                        v-if="
                            selectedNestedStep.config.action_type ||
                            selectedNestedStep.type !== 'action'
                        "
                        class="space-y-4"
                    >
                        <Separator />
                        <div class="space-y-3">
                            <Label class="text-base">Configuration</Label>
                            <div
                                v-for="field in getConfigFields(
                                    selectedNestedStep.type,
                                    selectedNestedStep.config.action_type,
                                )"
                                :key="field.key"
                                v-show="
                                    shouldShowField(field, selectedNestedStep.config)
                                "
                                class="space-y-2"
                            >
                                <div class="flex flex-col space-y-1">
                                    <Label
                                        >{{ field.label }}
                                        <span
                                            v-if="field.required"
                                            class="text-red-500"
                                            >*</span
                                        ></Label
                                    >
                                    <p
                                        v-if="(field as any).description"
                                        class="text-xs text-muted-foreground"
                                    >
                                        {{ (field as any).description }}
                                    </p>
                                </div>

                                <!-- Text Input -->
                                <Input
                                    v-if="field.type === 'text'"
                                    v-model="selectedNestedStep.config[field.key]"
                                    :placeholder="(field as any).placeholder"
                                    :required="field.required"
                                />

                                <!-- Number Input -->
                                <Input
                                    v-else-if="field.type === 'number'"
                                    v-model.number="
                                        selectedNestedStep.config[field.key]
                                    "
                                    type="number"
                                    :placeholder="(field as any).placeholder"
                                    :required="field.required"
                                />

                                <!-- Textarea -->
                                <Textarea
                                    v-else-if="field.type === 'textarea'"
                                    v-model="selectedNestedStep.config[field.key]"
                                    :rows="(field as any).rows || 3"
                                    :placeholder="(field as any).placeholder"
                                    :required="field.required"
                                    class="font-mono text-sm"
                                />

                                <!-- Lua Script Editor -->
                                <div
                                    v-else-if="field.type === 'lua'"
                                    class="space-y-2"
                                >
                                    <div class="h-96 mb-2">
                                        <CodeEditor
                                            v-model:value="
                                                selectedNestedStep.config[field.key]
                                            "
                                            language="lua"
                                            theme="vs-dark"
                                            :options="{
                                                fontSize: 14,
                                                minimap: { enabled: true },
                                                automaticLayout: true,
                                            }"
                                        />
                                    </div>
                                </div>

                                <!-- Select -->
                                <Select
                                    v-else-if="field.type === 'select'"
                                    v-model="selectedNestedStep.config[field.key]"
                                >
                                    <SelectTrigger>
                                        <SelectValue
                                            :placeholder="
                                                (field as any).placeholder
                                            "
                                        />
                                    </SelectTrigger>
                                    <SelectContent>
                                        <SelectItem
                                            v-for="option in (field as any)
                                                .options"
                                            :key="option"
                                            :value="option"
                                        >
                                            {{ option }}
                                        </SelectItem>
                                    </SelectContent>
                                </Select>
                            </div>
                        </div>
                    </div>
                </div>

                <DialogFooter>
                    <Button variant="outline" @click="closeNestedStepDialog"
                        >Cancel</Button
                    >
                    <Button @click="saveNestedStep">
                        Save Nested Step
                    </Button>
                </DialogFooter>
            </DialogContent>
        </Dialog>

        <!-- Import Dialog -->
        <Dialog v-model:open="showImportDialog">
            <DialogContent class="max-w-3xl max-h-[80vh]">
                <DialogHeader>
                    <DialogTitle>Import Workflow from JSON</DialogTitle>
                    <DialogDescription>
                        Import a workflow from a JSON file or paste JSON
                        directly. This will replace the current workflow
                        configuration.
                    </DialogDescription>
                </DialogHeader>

                <div class="space-y-4">
                    <!-- File Upload Section -->
                    <div class="space-y-2">
                        <Label for="workflow-file-editor"
                            >Upload JSON File</Label
                        >
                        <Input
                            id="workflow-file-editor"
                            type="file"
                            accept=".json,application/json"
                            @change="handleFileUpload"
                            class="cursor-pointer"
                        />
                        <p class="text-xs text-muted-foreground">
                            Select a JSON file containing workflow configuration
                        </p>
                    </div>

                    <div class="relative">
                        <div class="absolute inset-0 flex items-center">
                            <span class="w-full border-t" />
                        </div>
                        <div
                            class="relative flex justify-center text-xs uppercase"
                        >
                            <span
                                class="bg-background px-2 text-muted-foreground"
                            >
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
                            Ensure the JSON includes version, triggers, steps,
                            and variables fields. Name and description are optional.
                        </p>
                    </div>

                    <div
                        v-if="importError"
                        class="p-3 bg-destructive/10 border border-destructive/20 rounded-md"
                    >
                        <p class="text-sm text-destructive font-medium">
                            Import Error:
                        </p>
                        <p class="text-sm text-destructive">
                            {{ importError }}
                        </p>
                    </div>
                </div>

                <DialogFooter>
                    <Button variant="outline" @click="closeImportDialog"
                        >Cancel</Button
                    >
                    <Button
                        @click="validateAndImportWorkflow"
                        :disabled="!importJsonText.trim()"
                    >
                        Import Workflow
                    </Button>
                </DialogFooter>
            </DialogContent>
        </Dialog>

        <!-- JSON Editor Dialog -->
        <Dialog v-model:open="showJsonEditorDialog">
            <DialogContent class="max-w-4xl max-h-[85vh] flex flex-col">
                <DialogHeader>
                    <DialogTitle>Edit Workflow JSON</DialogTitle>
                    <DialogDescription>
                        Directly edit the workflow configuration as JSON. Changes will be validated before saving.
                    </DialogDescription>
                </DialogHeader>

                <div class="flex-1 overflow-hidden space-y-4">
                    <div class="h-[500px] border rounded-md overflow-hidden">
                        <CodeEditor
                            v-model:value="jsonEditorText"
                            language="json"
                            theme="vs-dark"
                            :options="{
                                fontSize: 14,
                                minimap: { enabled: true },
                                automaticLayout: true,
                                formatOnPaste: true,
                                formatOnType: true,
                            }"
                        />
                    </div>

                    <div
                        v-if="jsonEditorError"
                        class="p-3 bg-destructive/10 border border-destructive/20 rounded-md"
                    >
                        <p class="text-sm text-destructive font-medium">
                            Validation Error:
                        </p>
                        <p class="text-sm text-destructive">
                            {{ jsonEditorError }}
                        </p>
                    </div>

                    <div class="bg-muted/50 p-3 rounded-md">
                        <p class="text-xs text-muted-foreground">
                            <strong>Tip:</strong> The JSON must include <code class="text-xs">version</code>, <code class="text-xs">triggers</code> (array), 
                            <code class="text-xs">steps</code> (array), and <code class="text-xs">variables</code> (object) fields. 
                            The <code class="text-xs">name</code> and <code class="text-xs">description</code> fields are optional and will be synced with the database.
                        </p>
                    </div>
                </div>

                <DialogFooter>
                    <Button variant="outline" @click="closeJsonEditorDialog"
                        >Cancel</Button
                    >
                    <Button
                        @click="validateAndSaveJsonEdit"
                        :disabled="!jsonEditorText.trim()"
                    >
                        Save Changes
                    </Button>
                </DialogFooter>
            </DialogContent>
        </Dialog>
    </div>
</template>
