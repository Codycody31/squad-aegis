<script setup lang="ts">
import { ref, computed, watch, onMounted } from "vue";
import { Button } from "~/components/ui/button";
import {
    Dialog,
    DialogContent,
    DialogDescription,
    DialogFooter,
    DialogHeader,
    DialogTitle,
} from "~/components/ui/dialog";
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
import { Badge } from "~/components/ui/badge";
import { toast } from "~/components/ui/toast";
import { useAuthStore } from "~/stores/auth";
import {
    Terminal,
    Play,
    Loader2,
    CheckCircle,
    XCircle,
    AlertCircle,
    X,
} from "lucide-vue-next";

interface Command {
    id: string;
    name: string;
    description: string;
    category?: string;
    parameters?: {
        fields?: Array<{
            name: string;
            type: string;
            description?: string;
            required?: boolean;
            default?: any;
            options?: string[];
            sensitive?: boolean;
            nested?: any[];
        }>;
    };
    execution_type: "sync" | "async";
    required_permissions?: string[];
    confirm_message?: string;
}

interface CommandResult {
    success: boolean;
    message?: string;
    data?: Record<string, any>;
    execution_id?: string;
    error?: string;
}

interface CommandExecutionStatus {
    execution_id: string;
    command_id: string;
    status: "running" | "completed" | "failed";
    progress?: number;
    message?: string;
    result?: CommandResult;
    started_at: string;
    completed_at?: string;
}

const props = defineProps<{
    open: boolean;
    serverId: string;
    pluginId: string;
    pluginName: string;
}>();

const emit = defineEmits<{
    (e: "update:open", value: boolean): void;
}>();

const authStore = useAuthStore();

// State
const loading = ref(false);
const commands = ref<Command[]>([]);
const selectedCommand = ref<Command | null>(null);
const commandParams = ref<Record<string, any>>({});
const executing = ref(false);
const executionResult = ref<CommandResult | null>(null);
const executionStatus = ref<CommandExecutionStatus | null>(null);
const pollingInterval = ref<NodeJS.Timeout | null>(null);

// Computed
const categorizedCommands = computed(() => {
    const categories: Record<string, Command[]> = {};
    
    commands.value.forEach((cmd) => {
        const category = cmd.category || "General";
        if (!categories[category]) {
            categories[category] = [];
        }
        categories[category].push(cmd);
    });
    
    return categories;
});

// Methods
const loadCommands = async () => {
    loading.value = true;
    try {
        const response = await $fetch(
            `/api/servers/${props.serverId}/plugins/${props.pluginId}/commands`,
            {
                headers: {
                    Authorization: `Bearer ${authStore.token}`,
                },
            },
        );
        commands.value = (response as any).data.commands || [];
    } catch (error: any) {
        console.error("Failed to load commands:", error);
        toast({
            title: "Error",
            description: "Failed to load plugin commands",
            variant: "destructive",
        });
    } finally {
        loading.value = false;
    }
};

const selectCommand = (command: Command) => {
    selectedCommand.value = command;
    commandParams.value = {};
    executionResult.value = null;
    executionStatus.value = null;
    
    // Initialize parameters with defaults
    if (command.parameters?.fields) {
        command.parameters.fields.forEach((field) => {
            if (field.default !== undefined) {
                commandParams.value[field.name] = field.default;
            } else if (field.type === "bool") {
                commandParams.value[field.name] = false;
            } else if (field.type === "int") {
                commandParams.value[field.name] = 0;
            } else if (field.type === "string") {
                commandParams.value[field.name] = "";
            } else if (field.type === "arraystring" || field.type === "arrayint") {
                commandParams.value[field.name] = [];
            }
        });
    }
};

const executeCommand = async () => {
    if (!selectedCommand.value) return;
    
    // Show confirmation if required
    if (selectedCommand.value.confirm_message) {
        if (!confirm(selectedCommand.value.confirm_message)) {
            return;
        }
    }
    
    executing.value = true;
    executionResult.value = null;
    executionStatus.value = null;
    
    try {
        const response = await $fetch(
            `/api/servers/${props.serverId}/plugins/${props.pluginId}/commands/${selectedCommand.value.id}/execute`,
            {
                method: "POST",
                headers: {
                    Authorization: `Bearer ${authStore.token}`,
                },
                body: {
                    params: commandParams.value,
                },
            },
        );
        
        const result = (response as any).data.result as CommandResult;
        executionResult.value = result;
        
        // If async command, start polling for status
        if (result.execution_id && selectedCommand.value.execution_type === "async") {
            startPollingStatus(result.execution_id);
        } else {
            executing.value = false;
            
            if (result.success) {
                toast({
                    title: "Success",
                    description: result.message || "Command executed successfully",
                });
            } else {
                toast({
                    title: "Error",
                    description: result.error || "Command execution failed",
                    variant: "destructive",
                });
            }
        }
    } catch (error: any) {
        console.error("Failed to execute command:", error);
        executing.value = false;
        toast({
            title: "Error",
            description: error.data?.message || "Failed to execute command",
            variant: "destructive",
        });
    }
};

const startPollingStatus = (executionId: string) => {
    // Poll every 1 second
    pollingInterval.value = setInterval(async () => {
        await pollCommandStatus(executionId);
    }, 1000);
};

const pollCommandStatus = async (executionId: string) => {
    try {
        const response = await $fetch(
            `/api/servers/${props.serverId}/plugins/${props.pluginId}/commands/executions/${executionId}`,
            {
                headers: {
                    Authorization: `Bearer ${authStore.token}`,
                },
            },
        );
        
        const status = (response as any).data.status as CommandExecutionStatus;
        executionStatus.value = status;
        
        // Stop polling if completed or failed
        if (status.status === "completed" || status.status === "failed") {
            if (pollingInterval.value) {
                clearInterval(pollingInterval.value);
                pollingInterval.value = null;
            }
            executing.value = false;
            
            if (status.result) {
                if (status.result.success) {
                    toast({
                        title: "Success",
                        description: status.result.message || "Command completed successfully",
                    });
                } else {
                    toast({
                        title: "Error",
                        description: status.result.error || "Command execution failed",
                        variant: "destructive",
                    });
                }
            }
        }
    } catch (error: any) {
        console.error("Failed to poll command status:", error);
        if (pollingInterval.value) {
            clearInterval(pollingInterval.value);
            pollingInterval.value = null;
        }
        executing.value = false;
    }
};

const closeModal = () => {
    // Clean up polling if active
    if (pollingInterval.value) {
        clearInterval(pollingInterval.value);
        pollingInterval.value = null;
    }
    
    selectedCommand.value = null;
    commandParams.value = {};
    executionResult.value = null;
    executionStatus.value = null;
    executing.value = false;
    
    emit("update:open", false);
};

const normalizeFieldName = (fieldName: string): string => {
    let normalized = fieldName.replace(/_/g, " ");
    normalized = normalized.replace(/([a-z])([A-Z])/g, "$1 $2");
    normalized = normalized.replace(/\b\w/g, (char) => char.toUpperCase());
    
    const acronyms = ["Id", "Ip", "Url", "Api"];
    acronyms.forEach((acronym) => {
        const regex = new RegExp(`\\b${acronym}\\b`, "gi");
        normalized = normalized.replace(regex, acronym.toUpperCase());
    });
    
    return normalized;
};

// Watch for open state changes
watch(
    () => props.open,
    (newValue) => {
        if (newValue) {
            loadCommands();
        } else {
            closeModal();
        }
    },
);

onMounted(() => {
    if (props.open) {
        loadCommands();
    }
});
</script>

<template>
    <Dialog :open="open" @update:open="(val) => emit('update:open', val)">
        <DialogContent class="sm:max-w-3xl max-h-[90vh] flex flex-col">
            <DialogHeader>
                <DialogTitle>
                    <div class="flex items-center gap-2">
                        <Terminal class="w-5 h-5" />
                        Plugin Commands - {{ pluginName }}
                    </div>
                </DialogTitle>
                <DialogDescription>
                    Execute commands exposed by this plugin
                </DialogDescription>
            </DialogHeader>

            <div class="flex-1 overflow-y-auto space-y-4 pr-2">
                <!-- Loading state -->
                <div
                    v-if="loading"
                    class="flex items-center justify-center py-8"
                >
                    <Loader2 class="w-8 h-8 animate-spin text-primary" />
                </div>

                <!-- No commands available -->
                <div
                    v-else-if="commands.length === 0"
                    class="text-center py-8"
                >
                    <Terminal
                        class="w-16 h-16 mx-auto mb-4 text-muted-foreground"
                    />
                    <p class="text-muted-foreground">
                        No commands available for this plugin
                    </p>
                </div>

                <!-- Commands list (when no command selected) -->
                <div v-else-if="!selectedCommand" class="space-y-4">
                    <div
                        v-for="(cmds, category) in categorizedCommands"
                        :key="category"
                        class="space-y-2"
                    >
                        <h3 class="font-semibold text-sm text-muted-foreground">
                            {{ category }}
                        </h3>
                        <div class="space-y-2">
                            <div
                                v-for="cmd in cmds"
                                :key="cmd.id"
                                class="p-4 border rounded-lg hover:bg-accent cursor-pointer transition-colors"
                                @click="selectCommand(cmd)"
                            >
                                <div class="flex items-start justify-between">
                                    <div class="flex-1">
                                        <div class="flex items-center gap-2 mb-1">
                                            <h4 class="font-medium">{{ cmd.name }}</h4>
                                            <Badge
                                                variant="outline"
                                                class="text-xs"
                                            >
                                                {{ cmd.execution_type }}
                                            </Badge>
                                        </div>
                                        <p class="text-sm text-muted-foreground">
                                            {{ cmd.description }}
                                        </p>
                                    </div>
                                    <Play class="w-5 h-5 text-muted-foreground flex-shrink-0 ml-2" />
                                </div>
                            </div>
                        </div>
                    </div>
                </div>

                <!-- Command execution form -->
                <div v-else class="space-y-4">
                    <!-- Command header -->
                    <div class="flex items-start justify-between pb-4 border-b">
                        <div class="flex-1">
                            <div class="flex items-center gap-2 mb-1">
                                <h3 class="font-semibold text-lg">
                                    {{ selectedCommand.name }}
                                </h3>
                                <Badge variant="outline" class="text-xs">
                                    {{ selectedCommand.execution_type }}
                                </Badge>
                            </div>
                            <p class="text-sm text-muted-foreground">
                                {{ selectedCommand.description }}
                            </p>
                        </div>
                        <Button
                            variant="ghost"
                            size="sm"
                            @click="selectedCommand = null"
                        >
                            <X class="w-4 h-4" />
                        </Button>
                    </div>

                    <!-- Parameters form -->
                    <div
                        v-if="
                            selectedCommand.parameters?.fields &&
                            selectedCommand.parameters.fields.length > 0
                        "
                        class="space-y-4"
                    >
                        <h4 class="font-medium">Parameters</h4>
                        <div
                            v-for="field in selectedCommand.parameters.fields"
                            :key="field.name"
                            class="space-y-2"
                        >
                            <Label :for="`param-${field.name}`">
                                {{ normalizeFieldName(field.name) }}
                                <span v-if="field.required" class="text-red-500">*</span>
                            </Label>
                            <p
                                v-if="field.description"
                                class="text-sm text-muted-foreground"
                            >
                                {{ field.description }}
                            </p>

                            <!-- String input -->
                            <Input
                                v-if="field.type === 'string'"
                                :id="`param-${field.name}`"
                                v-model="commandParams[field.name]"
                                :type="field.sensitive ? 'password' : 'text'"
                            />

                            <!-- Number input -->
                            <Input
                                v-else-if="field.type === 'int' || field.type === 'number'"
                                :id="`param-${field.name}`"
                                v-model.number="commandParams[field.name]"
                                type="number"
                            />

                            <!-- Boolean switch -->
                            <div
                                v-else-if="field.type === 'bool'"
                                class="flex items-center space-x-2"
                            >
                                <Switch
                                    :id="`param-${field.name}`"
                                    :model-value="!!commandParams[field.name]"
                                    @update:model-value="
                                        (checked: boolean) =>
                                            (commandParams[field.name] = checked)
                                    "
                                />
                            </div>

                            <!-- Select dropdown -->
                            <Select
                                v-else-if="field.options && field.options.length > 0"
                                :model-value="commandParams[field.name]"
                                @update:model-value="
                                    (value) => (commandParams[field.name] = value)
                                "
                            >
                                <SelectTrigger :id="`param-${field.name}`">
                                    <SelectValue placeholder="Select an option" />
                                </SelectTrigger>
                                <SelectContent>
                                    <SelectItem
                                        v-for="option in field.options"
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
                                :id="`param-${field.name}`"
                                :model-value="
                                    Array.isArray(commandParams[field.name])
                                        ? commandParams[field.name].join(', ')
                                        : commandParams[field.name] || ''
                                "
                                placeholder="Enter values separated by commas"
                                @input="
                                    commandParams[field.name] = (
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
                                :id="`param-${field.name}`"
                                :model-value="
                                    Array.isArray(commandParams[field.name])
                                        ? commandParams[field.name].join(', ')
                                        : commandParams[field.name] || ''
                                "
                                placeholder="Enter integer values separated by commas"
                                @input="
                                    commandParams[field.name] = (
                                        $event.target as HTMLTextAreaElement
                                    ).value
                                        .split(',')
                                        .map((s: string) => parseInt(s.trim()))
                                        .filter((n) => !isNaN(n))
                                "
                            />
                        </div>
                    </div>

                    <!-- Execution status -->
                    <div
                        v-if="executionStatus"
                        class="p-4 border rounded-lg space-y-2"
                    >
                        <div class="flex items-center gap-2">
                            <Loader2
                                v-if="executionStatus.status === 'running'"
                                class="w-5 h-5 animate-spin text-blue-500"
                            />
                            <CheckCircle
                                v-else-if="executionStatus.status === 'completed'"
                                class="w-5 h-5 text-green-500"
                            />
                            <XCircle
                                v-else-if="executionStatus.status === 'failed'"
                                class="w-5 h-5 text-red-500"
                            />
                            <span class="font-medium capitalize">
                                {{ executionStatus.status }}
                            </span>
                        </div>
                        
                        <div v-if="executionStatus.progress !== undefined" class="space-y-1">
                            <div class="flex justify-between text-sm">
                                <span>Progress</span>
                                <span>{{ executionStatus.progress }}%</span>
                            </div>
                            <div class="w-full bg-secondary rounded-full h-2">
                                <div
                                    class="bg-primary h-2 rounded-full transition-all"
                                    :style="{ width: `${executionStatus.progress}%` }"
                                ></div>
                            </div>
                        </div>
                        
                        <p v-if="executionStatus.message" class="text-sm text-muted-foreground">
                            {{ executionStatus.message }}
                        </p>
                    </div>

                    <!-- Execution result -->
                    <div
                        v-if="executionResult && !executionStatus"
                        class="p-4 border rounded-lg space-y-2"
                    >
                        <div class="flex items-center gap-2">
                            <CheckCircle
                                v-if="executionResult.success"
                                class="w-5 h-5 text-green-500"
                            />
                            <XCircle v-else class="w-5 h-5 text-red-500" />
                            <span class="font-medium">
                                {{ executionResult.success ? "Success" : "Failed" }}
                            </span>
                        </div>
                        
                        <p
                            v-if="executionResult.message || executionResult.error"
                            class="text-sm"
                        >
                            {{ executionResult.message || executionResult.error }}
                        </p>
                        
                        <div
                            v-if="executionResult.data && Object.keys(executionResult.data).length > 0"
                            class="mt-2"
                        >
                            <p class="text-sm font-medium mb-2">Result Data:</p>
                            <pre
                                class="text-xs bg-muted p-2 rounded overflow-auto max-h-40"
                            >{{ JSON.stringify(executionResult.data, null, 2) }}</pre>
                        </div>
                    </div>
                </div>
            </div>

            <DialogFooter class="flex-shrink-0 pt-4">
                <Button variant="outline" @click="closeModal">Close</Button>
                <Button
                    v-if="selectedCommand"
                    @click="executeCommand"
                    :disabled="executing"
                >
                    <Loader2
                        v-if="executing"
                        class="w-4 h-4 mr-2 animate-spin"
                    />
                    <Play v-else class="w-4 h-4 mr-2" />
                    Execute
                </Button>
            </DialogFooter>
        </DialogContent>
    </Dialog>
</template>

