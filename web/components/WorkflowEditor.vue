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
import { CodeEditor } from 'monaco-editor-vue3';
import 'monaco-editor-vue3/dist/style.css';

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
const selectedTrigger = ref<WorkflowTrigger | null>(null);
const selectedStep = ref<WorkflowStep | null>(null);
const selectedVariable = ref<{ key: string; value: any; type: string } | null>(
  null
);
const importJsonText = ref("");
const importError = ref("");
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
    } else if (newType === "variable") {
      if (!selectedStep.value.config.operation) {
        selectedStep.value.config.operation = "set";
      }
    }
  },
  { deep: true }
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

// Helper function to check if a field should be shown based on step config
function shouldShowField(field: any, stepConfig: Record<string, any>): boolean {
  if (field.key === "value" && stepConfig.operation === "delete") {
    return false; // Don't show value field for delete operation
  }
  if (field.key === "source_variable" && stepConfig.operation !== "copy") {
    return false; // Only show source_variable for copy operation
  }
  if (field.key === "transform_type" && stepConfig.operation !== "transform") {
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
  if (key && definition.value.variables && key in definition.value.variables) {
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
        processedValue = String(processedValue).toLowerCase() === "true";
        break;
      case "array":
        processedValue = JSON.parse(processedValue);
        if (!Array.isArray(processedValue)) throw new Error("Not an array");
        break;
      case "object":
        processedValue = JSON.parse(processedValue);
        if (Array.isArray(processedValue) || typeof processedValue !== "object")
          throw new Error("Not an object");
        break;
      default:
        processedValue = String(processedValue);
    }
  } catch (error) {
    console.warn("Invalid value for type:", selectedVariable.value.type, error);
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
            description: "Player name or Steam ID to send the message to",
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
            description: "Player name or Steam ID of the player to kick",
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
            description: "Player name or Steam ID of the player to ban",
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
            description: "Player name or Steam ID of the player to warn",
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
            description: "Discord webhook URL from channel settings",
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
            description: "Maximum execution time (default: 30 seconds)",
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
        label: "Steps if True (comma-separated step names)",
        type: "text",
        required: false,
        placeholder: "e.g. send_warning, log_event",
        description: "Steps to execute if condition is true",
      },
      {
        key: "false_steps",
        label: "Steps if False (comma-separated step names)",
        type: "text",
        required: false,
        placeholder: "e.g. send_kick, ban_player",
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
        options: ["uppercase", "lowercase", "trim", "length", "reverse"],
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
  showImportDialog.value = true;
}

function closeImportDialog() {
  showImportDialog.value = false;
  importJsonText.value = "";
  importError.value = "";
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
  const jsonString = JSON.stringify(definition.value, null, 2);
  const blob = new Blob([jsonString], { type: "application/json" });
  const url = URL.createObjectURL(blob);
  const link = document.createElement("a");
  link.href = url;
  link.download = `workflow-${props.workflowId || "export"}.json`;
  document.body.appendChild(link);
  link.click();
  document.body.removeChild(link);
  URL.revokeObjectURL(url);
}
</script>

<template>
  <div class="space-y-6">
    <!-- Import/Export Controls -->
    <div class="flex justify-end gap-2">
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
          <Button @click="openTriggerDialog()" variant="outline" size="sm">
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
                    <Badge :variant="trigger.enabled ? 'default' : 'secondary'">
                      {{ trigger.enabled ? "Active" : "Inactive" }}
                    </Badge>
                  </div>
                  <p class="text-sm text-muted-foreground mt-1">
                    Event:
                    {{
                      eventTypes.find((et) => et.value === trigger.event_type)
                        ?.label || trigger.event_type
                    }}
                  </p>
                  <p
                    v-if="trigger.conditions && trigger.conditions.length > 0"
                    class="text-sm text-muted-foreground"
                  >
                    {{ trigger.conditions.length }} condition(s)
                  </p>
                </div>
                <div class="flex gap-1">
                  <Button
                    @click="openTriggerDialog(trigger, index)"
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
          <Button @click="openStepDialog()" variant="outline" size="sm">
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
            Add steps to define what actions the workflow should perform
          </p>
        </div>

        <div v-else class="space-y-3">
          <Card
            v-for="(step, index) in definition.steps"
            :key="step.id"
            class="relative"
          >
            <CardHeader class="pb-3">
              <div class="flex justify-between items-start">
                <div class="flex items-center gap-3">
                  <div
                    class="flex items-center justify-center w-8 h-8 rounded-full bg-muted"
                  >
                    <component :is="getStepIcon(step.type)" class="w-4 h-4" />
                  </div>
                  <div class="flex-1">
                    <div class="flex items-center gap-2">
                      <CardTitle class="text-base">{{
                        step.name || "Unnamed Step"
                      }}</CardTitle>
                      <Badge variant="outline" class="text-xs">
                        {{
                          stepTypes.find((st) => st.value === step.type)
                            ?.label || step.type
                        }}
                      </Badge>
                      <Badge :variant="step.enabled ? 'default' : 'secondary'">
                        {{ step.enabled ? "Active" : "Inactive" }}
                      </Badge>
                    </div>
                    <p class="text-sm text-muted-foreground mt-1">
                      Step {{ index + 1 }} of {{ definition.steps.length }}
                      <span v-if="step.config.action_type" class="ml-2">
                        •
                        {{
                          actionTypes.find(
                            (at) => at.value === step.config.action_type
                          )?.label || step.config.action_type
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
                    :disabled="index === definition.steps.length - 1"
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
                  <Button @click="deleteStep(index)" variant="ghost" size="sm">
                    <Trash2 class="w-4 h-4" />
                  </Button>
                </div>
              </div>
            </CardHeader>
          </Card>
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
          <Button @click="openVariableDialog()" variant="outline" size="sm">
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
            Add variables to store values that can be used across workflow steps
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
                      <CardTitle class="text-base font-mono">{{
                        key
                      }}</CardTitle>
                      <Badge variant="outline" class="text-xs">
                        {{ getVariableType(value) }}
                      </Badge>
                    </div>
                    <p class="text-sm text-muted-foreground mt-1 font-mono">
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
                  :value="definition.error_handling?.default_action"
                  @update:value="updateErrorHandling('default_action', $event)"
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
                  :value="definition.error_handling?.max_retries"
                  @input="
                    updateErrorHandling(
                      'max_retries',
                      parseInt($event.target.value)
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
                :value="definition.error_handling?.retry_delay_ms"
                @input="
                  updateErrorHandling(
                    'retry_delay_ms',
                    parseInt($event.target.value)
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
            {{ editingTriggerIndex >= 0 ? "Edit Trigger" : "Add Trigger" }}
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
                  <SelectValue placeholder="Select event type" />
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
                (val) => selectedTrigger && (selectedTrigger.enabled = val)
              "
            />
            <Label>Enable this trigger</Label>
          </div>

          <Separator />

          <div>
            <div class="flex justify-between items-center mb-2">
              <Label>Conditions (optional)</Label>
              <Button @click="addCondition" variant="outline" size="sm">
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
              No conditions - trigger will activate for all events of this type
            </div>

            <div v-else class="space-y-3">
              <Card
                v-for="(condition, index) in selectedTrigger.conditions"
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
                    <Input v-model="condition.value" class="text-sm" />
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
          <Button variant="outline" @click="closeTriggerDialog">Cancel</Button>
          <Button @click="saveTrigger">
            {{ editingTriggerIndex >= 0 ? "Update" : "Add" }} Trigger
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
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem
                    v-for="stepType in stepTypes"
                    :key="stepType.value"
                    :value="stepType.value"
                  >
                    <div>
                      <div class="font-medium">{{ stepType.label }}</div>
                      <div class="text-sm text-muted-foreground">
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
                (val) => selectedStep && (selectedStep.enabled = val)
              "
            />
            <Label>Enable this step</Label>
          </div>

          <!-- Action Type Selection (for action steps) -->
          <div v-if="selectedStep.type === 'action'" class="space-y-2">
            <Label>Action Type</Label>
            <Select v-model="selectedStep.config.action_type">
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
                    <div class="font-medium">{{ actionType.label }}</div>
                    <div class="text-sm text-muted-foreground">
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
              selectedStep.config.action_type || selectedStep.type !== 'action'
            "
            class="space-y-4"
          >
            <Separator />
            <div class="space-y-3">
              <Label class="text-base">Configuration</Label>
              <div
                v-for="field in getConfigFields(
                  selectedStep.type,
                  selectedStep.config.action_type
                )"
                :key="field.key"
                v-show="shouldShowField(field, selectedStep.config)"
                class="space-y-2"
              >
                <div class="flex flex-col space-y-1">
                  <Label
                    >{{ field.label }}
                    <span v-if="field.required" class="text-red-500"
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
                  v-model.number="selectedStep.config[field.key]"
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
                <div v-else-if="field.type === 'lua'" class="space-y-2">
                  <div class="h-96 mb-2">
                  <CodeEditor
                    v-model:value="selectedStep.config[field.key]"
                    language="lua"
                    theme="vs-dark"
                    :options="{
                      fontSize: 14,
                      minimap: { enabled: true },
                      automaticLayout: true,
                    }"
                  />
                  </div>
                  <div class="bg-muted p-3 rounded-md text-xs">
                    <p class="font-medium mb-2">Available Lua Functions:</p>
                    <div class="grid grid-cols-2 gap-2">
                      <div>
                        <p class="font-medium text-green-600">Logging:</p>
                        <ul class="space-y-1 text-muted-foreground">
                          <li><code>log(message)</code></li>
                          <li><code>log_debug(message)</code></li>
                          <li><code>log_warn(message)</code></li>
                          <li><code>log_error(message)</code></li>
                        </ul>
                      </div>
                      <div>
                        <p class="font-medium text-blue-600">Variables:</p>
                        <ul class="space-y-1 text-muted-foreground">
                          <li><code>set_variable(name, value)</code></li>
                          <li><code>get_variable(name)</code></li>
                        </ul>
                      </div>
                      <div>
                        <p class="font-medium text-purple-600">Utilities:</p>
                        <ul class="space-y-1 text-muted-foreground">
                          <li><code>json_encode(table)</code></li>
                          <li><code>json_decode(string)</code></li>
                        </ul>
                      </div>
                      <div>
                        <p class="font-medium text-orange-600">
                          Workflow Data:
                        </p>
                        <ul class="space-y-1 text-muted-foreground">
                          <li><code>workflow.trigger_event</code></li>
                          <li><code>workflow.metadata</code></li>
                          <li><code>workflow.variables</code></li>
                          <li><code>workflow.step_results</code></li>
                          <li><code>result</code> - Output table</li>
                        </ul>
                      </div>
                    </div>
                    <div class="mt-3 p-2 bg-background rounded border">
                      <p class="font-medium mb-1">Example Usage:</p>
                      <pre
                        class="text-xs text-muted-foreground"
                      ><code>-- Access trigger event data
local player = workflow.trigger_event.player_name
log("Processing: " .. (player or "unknown"))

-- Set a workflow variable
set_variable("last_player", player)

-- Store results
result.success = true
result.message = "Processed " .. player</code></pre>
                    </div>
                  </div>
                </div>

                <!-- Select -->
                <Select
                  v-else-if="field.type === 'select'"
                  v-model="selectedStep.config[field.key]"
                >
                  <SelectTrigger>
                    <SelectValue :placeholder="(field as any).placeholder" />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem
                      v-for="option in (field as any).options"
                      :key="option"
                      :value="option"
                    >
                      {{ option }}
                    </SelectItem>
                  </SelectContent>
                </Select>

                <!-- Conditions Array -->
                <div
                  v-else-if="field.type === 'conditions_array'"
                  class="space-y-3"
                >
                  <div class="flex justify-between items-center">
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
                      selectedStep.config[field.key].length === 0
                    "
                    class="text-sm text-muted-foreground py-4 text-center"
                  >
                    No conditions defined
                  </div>

                  <div v-else class="space-y-2">
                    <Card
                      v-for="(condition, index) in selectedStep.config[
                        field.key
                      ]"
                      :key="index"
                      class="p-3"
                    >
                      <div class="grid grid-cols-4 gap-2 items-end">
                        <div class="space-y-1">
                          <Label class="text-xs">Field</Label>
                          <Input
                            v-model="condition.field"
                            placeholder="e.g. trigger_event.player_name"
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
                            placeholder="Value to compare"
                          />
                        </div>
                        <Button
                          @click="removeStepCondition(index)"
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
            </div>
          </div>

          <!-- Error Handling -->
          <div class="space-y-4">
            <Separator />
            <div class="space-y-3">
              <Label class="text-base">Error Handling (optional)</Label>
              <div class="grid grid-cols-3 gap-4">
                <div class="space-y-2">
                  <Label class="text-sm">On Error</Label>
                  <Select v-model="selectedStep.on_error.action">
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
                    v-model.number="selectedStep.on_error.max_retries"
                    type="number"
                    min="0"
                    max="10"
                  />
                </div>
                <div class="space-y-2">
                  <Label class="text-sm">Retry Delay (ms)</Label>
                  <Input
                    v-model.number="selectedStep.on_error.retry_delay_ms"
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
                  <strong>Variables:</strong> <code>${variable_name}</code> for
                  any workflow variables
                </p>
                <p class="text-muted-foreground mt-2">
                  Example: "Player ${trigger_event.player_name} said:
                  ${trigger_event.message}"
                </p>
              </div>
            </div>
          </div>
        </div>

        <DialogFooter>
          <Button variant="outline" @click="closeStepDialog">Cancel</Button>
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
            {{ editingVariableKey ? "Edit Variable" : "Add Variable" }}
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
                  selectedVariable.type === "array" ? "array" : "object"
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
                {{ getVariableDisplayValue(selectedVariable.value) }}
              </code>
            </div>
          </div>
        </div>

        <DialogFooter>
          <Button variant="outline" @click="closeVariableDialog">Cancel</Button>
          <Button
            @click="saveVariable"
            :disabled="!selectedVariable.key.trim()"
          >
            {{ editingVariableKey ? "Update" : "Add" }} Variable
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
            Paste your workflow JSON below. This will replace the current
            workflow configuration.
          </DialogDescription>
        </DialogHeader>

        <div class="space-y-4">
          <div class="space-y-2">
            <Label>Workflow JSON</Label>
            <Textarea
              v-model="importJsonText"
              placeholder="Paste your workflow JSON here..."
              rows="20"
              class="font-mono text-sm"
            />
            <p class="text-xs text-muted-foreground">
              Ensure the JSON includes version, triggers, steps, and variables
              fields
            </p>
          </div>

          <div
            v-if="importError"
            class="p-3 bg-destructive/10 border border-destructive/20 rounded-md"
          >
            <p class="text-sm text-destructive font-medium">Import Error:</p>
            <p class="text-sm text-destructive">{{ importError }}</p>
          </div>

          <div class="bg-muted p-3 rounded-md">
            <p class="text-sm font-medium mb-2">📋 Expected JSON Structure:</p>
            <pre class="text-xs text-muted-foreground overflow-x-auto"><code>{
  "version": "1.0",
  "triggers": [
    {
      "id": "trigger-id",
      "name": "Trigger Name",
      "event_type": "player_connected",
      "conditions": [],
      "enabled": true
    }
  ],
  "variables": {
    "variable_name": "value"
  },
  "steps": [
    {
      "id": "step-id",
      "name": "Step Name",
      "type": "action",
      "enabled": true,
      "config": {
        "action_type": "admin_broadcast",
        "message": "Welcome!"
      }
    }
  ],
  "error_handling": {
    "default_action": "stop",
    "max_retries": 3,
    "retry_delay_ms": 1000
  }
}</code></pre>
          </div>
        </div>

        <DialogFooter>
          <Button variant="outline" @click="closeImportDialog">Cancel</Button>
          <Button
            @click="validateAndImportWorkflow"
            :disabled="!importJsonText.trim()"
          >
            Import Workflow
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  </div>
</template>
