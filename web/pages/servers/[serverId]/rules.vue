<script setup lang="ts">
import { ref, onMounted, onUnmounted, computed, watch } from "vue";
import { Button } from "~/components/ui/button";
import { Input } from "~/components/ui/input";
import { Card, CardContent, CardHeader, CardTitle } from "~/components/ui/card";
import {
    Dialog,
    DialogContent,
    DialogDescription,
    DialogFooter,
    DialogHeader,
    DialogTitle,
    DialogTrigger,
} from "~/components/ui/dialog";
import {
    Form,
    FormControl,
    FormDescription,
    FormField,
    FormItem,
    FormLabel,
    FormMessage,
} from "~/components/ui/form";
import { Textarea } from "~/components/ui/textarea";
import {
    Select,
    SelectContent,
    SelectItem,
    SelectTrigger,
    SelectValue,
} from "~/components/ui/select";
import { useForm } from "vee-validate";
import { toTypedSchema } from "@vee-validate/zod";
import * as z from "zod";
import draggable from "~/modules/vuedraggable/vuedraggable.js";
import { ChevronRight, ChevronDown, GripVertical } from 'lucide-vue-next';
import RuleItem from '~/components/rules/RuleItem.vue';

const authStore = useAuthStore();
const runtimeConfig = useRuntimeConfig();
const route = useRoute();
const serverId = route.params.serverId;

const loading = ref(true);
const error = ref<string | null>(null);
const rules = ref<ServerRule[]>([]);
const refreshInterval = ref<NodeJS.Timeout | null>(null);
const showAddRuleDialog = ref(false);
const addRuleLoading = ref(false);
const editingRule = ref<ServerRule | null>(null);
const localRulesTree = ref<any[]>([]);

interface ServerRule {
    id: string;
    serverId: string;
    parentId: string | null;
    name: string;
    description: string | null;
    suggestedDuration: number;
    orderKey: string;
}

// Form schema for adding/editing a rule
const formSchema = toTypedSchema(
    z.object({
        name: z.string().min(1, "Name is required"),
        description: z.string().optional(),
        suggestedDuration: z.number().min(0, "Duration must be at least 0"),
    }),
);

// Setup form
const form = useForm({
    validationSchema: formSchema,
    initialValues: {
        name: "",
        description: "",
        suggestedDuration: 24,
    },
});

// Update the computed property to use the local tree
const rulesTree = computed({
    get: () => localRulesTree.value,
    set: (newValue) => {
        localRulesTree.value = newValue;
    }
});

// Add this watch to update local tree when rules change
watch(() => rules.value, (newRules) => {
    const buildTree = (parentId: string | null = null): any[] => {
        return newRules
            .filter(rule => rule.parentId === parentId)
            .sort((a, b) => a.orderKey.localeCompare(b.orderKey, undefined, { numeric: true }))
            .map(rule => ({
                ...rule,
                children: buildTree(rule.id),
            }));
    };
    localRulesTree.value = buildTree(null);
}, { immediate: true });

// Function to generate next order key
function generateNextOrderKey(parentRule: ServerRule | null = null): string {
    const siblingRules = rules.value.filter(r => r.parentId === (parentRule?.id ?? null));
    if (siblingRules.length === 0) {
        return parentRule ? `${parentRule.orderKey}.1` : "1";
    }

    const lastRule = siblingRules.sort((a, b) => 
        a.orderKey.localeCompare(b.orderKey, undefined, { numeric: true })
    ).pop();

    if (!lastRule) return "1";

    const parts = lastRule.orderKey.split(".");
    const lastPart = parts[parts.length - 1];

    // Check if the last part ends with a letter
    const match = lastPart.match(/^(\d+)([A-Z])?$/);
    if (!match) return lastRule.orderKey + "A";

    const [, num, letter] = match;
    if (!letter) {
        // No letter suffix, just increment the number
        return parts.slice(0, -1).concat((parseInt(num) + 1).toString()).join(".");
    } else {
        // Has letter suffix, increment the letter
        return parts.slice(0, -1).concat(`${num}${String.fromCharCode(letter.charCodeAt(0) + 1)}`).join(".");
    }
}

// Function to fetch server rules
async function fetchServerRules() {
    loading.value = true;
    error.value = null;

    const runtimeConfig = useRuntimeConfig();
    const cookieToken = useCookie(runtimeConfig.public.sessionCookieName as string);
    const token = cookieToken.value;

    if (!token) {
        error.value = "Authentication required";
        loading.value = false;
        return;
    }

    try {
        const response = await useFetch(
            `${runtimeConfig.public.backendApi}/servers/${serverId}/rules`,
            {
                method: "GET",
                headers: {
                    Authorization: `Bearer ${token}`,
                },
            },
        );

        if (response.error.value) {
            throw new Error(response.error.value.message || "Failed to fetch server rules");
        }

        // Safely access the data with proper type checking
        const responseData = response.data.value as any;
        if (responseData && responseData.data) {
            rules.value = responseData.data.rules || [];
        }
    } catch (err: any) {
        error.value = err.message || "An error occurred while fetching server rules";
        console.error(err);
    } finally {
        loading.value = false;
    }
}

// Function to add a rule
async function addRule(values: any) {
    addRuleLoading.value = true;
    error.value = null;

    const runtimeConfig = useRuntimeConfig();
    const cookieToken = useCookie(runtimeConfig.public.sessionCookieName as string);
    const token = cookieToken.value;

    if (!token) {
        error.value = "Authentication required";
        addRuleLoading.value = false;
        return;
    }

    try {
        // Generate order key for new rule
        const orderKey = generateNextOrderKey();

        const { data, error: fetchError } = await useFetch(
            `${runtimeConfig.public.backendApi}/servers/${serverId}/rules`,
            {
                method: "POST",
                body: {
                    name: values.name,
                    description: values.description || "",
                    suggestedDuration: values.suggestedDuration,
                    orderKey: orderKey,
                },
                headers: {
                    Authorization: `Bearer ${token}`,
                },
            },
        );

        if (fetchError.value) {
            throw new Error(fetchError.value.message || "Failed to add rule");
        }

        // Reset form and close dialog
        form.resetForm();
        showAddRuleDialog.value = false;

        // Refresh the rules list
        fetchServerRules();
    } catch (err: any) {
        error.value = err.message || "An error occurred while adding the rule";
        console.error(err);
    } finally {
        addRuleLoading.value = false;
    }
}

// Function to update a rule
async function updateRule(values: any) {
    if (!editingRule.value) return;

    addRuleLoading.value = true;
    error.value = null;

    const runtimeConfig = useRuntimeConfig();
    const cookieToken = useCookie(runtimeConfig.public.sessionCookieName as string);
    const token = cookieToken.value;

    if (!token) {
        error.value = "Authentication required";
        addRuleLoading.value = false;
        return;
    }

    try {
        const { data, error: fetchError } = await useFetch(
            `${runtimeConfig.public.backendApi}/servers/${serverId}/rules/${editingRule.value.id}`,
            {
                method: "PUT",
                body: {
                    name: values.name,
                    description: values.description || "",
                    suggestedDuration: values.suggestedDuration,
                    orderKey: editingRule.value.orderKey, // Keep the existing order key
                },
                headers: {
                    Authorization: `Bearer ${token}`,
                },
            },
        );

        if (fetchError.value) {
            throw new Error(fetchError.value.message || "Failed to update rule");
        }

        // Reset form and close dialog
        form.resetForm();
        showAddRuleDialog.value = false;
        editingRule.value = null;

        // Refresh the rules list
        fetchServerRules();
    } catch (err: any) {
        error.value = err.message || "An error occurred while updating the rule";
        console.error(err);
    } finally {
        addRuleLoading.value = false;
    }
}

// Function to delete a rule
async function deleteRule(ruleId: string) {
    if (!confirm("Are you sure you want to delete this rule? This will also delete any child rules.")) {
        return;
    }

    loading.value = true;
    error.value = null;

    const runtimeConfig = useRuntimeConfig();
    const cookieToken = useCookie(runtimeConfig.public.sessionCookieName as string);
    const token = cookieToken.value;

    if (!token) {
        error.value = "Authentication required";
        loading.value = false;
        return;
    }

    try {
        const { data, error: fetchError } = await useFetch(
            `${runtimeConfig.public.backendApi}/servers/${serverId}/rules/${ruleId}`,
            {
                method: "DELETE",
                headers: {
                    Authorization: `Bearer ${token}`,
                },
            },
        );

        if (fetchError.value) {
            throw new Error(fetchError.value.message || "Failed to delete rule");
        }

        // Refresh the rules list
        fetchServerRules();
    } catch (err: any) {
        error.value = err.message || "An error occurred while deleting the rule";
        console.error(err);
    } finally {
        loading.value = false;
    }
}

// Function to edit a rule
function startEditRule(rule: ServerRule) {
    editingRule.value = rule;
    form.setValues({
        name: rule.name,
        description: rule.description || "",
        suggestedDuration: rule.suggestedDuration / (24 * 60),
    });
    showAddRuleDialog.value = true;
}

// Setup auto-refresh and initial data load
onMounted(() => {
    fetchServerRules();

    // Refresh data every 60 seconds
    refreshInterval.value = setInterval(() => {
        fetchServerRules();
    }, 60000);
});

// Clear interval on component unmount
onUnmounted(() => {
    if (refreshInterval.value) {
        clearInterval(refreshInterval.value);
    }
});

// Reset form when dialog is closed
watch(showAddRuleDialog, (newValue) => {
    if (!newValue) {
        form.resetForm();
        editingRule.value = null;
    }
});

// Function to update order keys after drag and drop
async function updateOrderKeys(newRules: any[], parentId: string | null = null, prefix: string = ''): Promise<Array<{ id: string; orderKey: string }>> {
    const updates: Array<{ id: string; orderKey: string }> = [];
    
    for (let i = 0; i < newRules.length; i++) {
        const rule = newRules[i];
        if (!rule || !rule.id) continue; // Skip invalid rules
        
        const newOrderKey = `${prefix}${i + 1}`;
        
        if (rule.orderKey !== newOrderKey) {
            updates.push({
                id: rule.id,
                orderKey: newOrderKey
            });
        }

        if (rule.children && rule.children.length > 0) {
            const childUpdates = await updateOrderKeys(rule.children, rule.id, `${newOrderKey}.`);
            updates.push(...childUpdates);
        }
    }

    return updates;
}

// Update the onDragEnd function
async function onDragEnd(evt: any, parentId: string | null = null) {
    if (evt.from === evt.to) {
        // Only update if the order has actually changed
        const oldIndex = evt.oldIndex;
        const newIndex = evt.newIndex;
        if (oldIndex !== newIndex) {
            const newRules = evt.to.children;
            const updates = await updateOrderKeys(newRules, parentId);

            // Only make API call if there are actual updates with valid IDs
            if (updates.length > 0 && updates.every(update => update.id)) {
                try {
                    const runtimeConfig = useRuntimeConfig();
                    const cookieToken = useCookie(runtimeConfig.public.sessionCookieName as string);
                    const token = cookieToken.value;

                    if (!token) return;

                    console.log('Sending updates:', updates); // Debug log

                    // Update all rules in a single batch
                    await useFetch(
                        `${runtimeConfig.public.backendApi}/servers/${serverId}/rules/batch-update`,
                        {
                            method: "PUT",
                            body: {
                                updates: updates
                            },
                            headers: {
                                Authorization: `Bearer ${token}`,
                            },
                        },
                    );

                    // Update the rules array directly
                    for (const update of updates) {
                        const rule = rules.value.find(r => r.id === update.id);
                        if (rule) {
                            rule.orderKey = update.orderKey;
                        }
                    }
                } catch (err) {
                    console.error('Failed to update rule orders:', err);
                }
            }
        }
    }
}

// Function to handle nesting
async function handleNest(evt: any, targetRule: any) {
    try {
        // Get the dragged rule ID from the data transfer
        const draggedRuleId = evt.dataTransfer?.getData('text/plain');
        if (!draggedRuleId) return;

        const draggedRule = rules.value.find(r => r.id === draggedRuleId);
        if (!draggedRule || draggedRule.id === targetRule.id) return;
        
        // Prevent circular nesting (can't nest parent into its own child)
        if (targetRule.parentId === draggedRule.id) return;
        
        // Check if directly trying to nest into own parent (no change needed)
        if (draggedRule.parentId === targetRule.id) return;

        const runtimeConfig = useRuntimeConfig();
        const cookieToken = useCookie(runtimeConfig.public.sessionCookieName as string);
        const token = cookieToken.value;

        if (!token) return;

        const newOrderKey = generateNextOrderKey(targetRule);
        
        // Ensure the target rule has isExpanded set to true
        if (!targetRule.isExpanded) {
            const updatedTargetRule = { ...targetRule, isExpanded: true };
            updateRuleInArray(updatedTargetRule);
        }

        // Update the rule parent through API
        await useFetch(
            `${runtimeConfig.public.backendApi}/servers/${serverId}/rules/${draggedRule.id}`,
            {
                method: "PUT",
                body: {
                    name: draggedRule.name,
                    description: draggedRule.description || "",
                    suggestedDuration: draggedRule.suggestedDuration,
                    parentId: targetRule.id,
                    orderKey: newOrderKey,
                },
                headers: {
                    Authorization: `Bearer ${token}`,
                },
            },
        );

        // Refresh rules to ensure the UI is updated properly
        fetchServerRules();
    } catch (err) {
        console.error('Failed to update rule parent:', err);
    }
}

// Function to handle drag start
function handleDragStart(evt: DragEvent, rule: any) {
    if (evt.dataTransfer) {
        evt.dataTransfer.setData('text/plain', rule.id);
    }
}

// Function to update a rule in the array
function updateRuleInArray(updatedRule: ServerRule) {
    // Update in the main rules array
    const findAndUpdateRule = (rules: ServerRule[], targetId: string, updatedRule: ServerRule): boolean => {
        for (let i = 0; i < rules.length; i++) {
            if (rules[i].id === targetId) {
                rules[i] = { ...rules[i], ...updatedRule };
                return true;
            }
        }
        return false;
    };
    findAndUpdateRule(rules.value, updatedRule.id, updatedRule);

    // Update in the local tree
    const findAndUpdateTreeRule = (tree: any[], targetId: string, updatedRule: any): boolean => {
        for (let i = 0; i < tree.length; i++) {
            if (tree[i].id === targetId) {
                tree[i] = { ...tree[i], ...updatedRule };
                return true;
            }
            if (tree[i].children && findAndUpdateTreeRule(tree[i].children, targetId, updatedRule)) {
                return true;
            }
        }
        return false;
    };
    findAndUpdateTreeRule(localRulesTree.value, updatedRule.id, updatedRule);
}
</script>

<template>
    <div class="p-4">
        <div class="flex justify-between items-center mb-4">
            <h1 class="text-2xl font-bold">Server Rules</h1>
            <div class="flex gap-2">
                <Form
                    v-slot="{ handleSubmit }"
                    as=""
                    keep-values
                    :validation-schema="formSchema"
                >
                    <Dialog v-model:open="showAddRuleDialog">
                        <DialogTrigger asChild>
                            <Button v-if="authStore.getServerPermission(serverId as string, 'manageserver')">
                                {{ editingRule ? 'Edit Rule' : 'Add Rule' }}
                            </Button>
                        </DialogTrigger>
                        <DialogContent class="sm:max-w-[600px] max-h-[80vh] overflow-y-auto">
                            <DialogHeader>
                                <DialogTitle>
                                    {{ editingRule ? 'Edit Rule' : 'Add New Rule' }}
                                </DialogTitle>
                                <DialogDescription>
                                    {{ editingRule ? 'Edit the rule details below.' : 'Enter the details for the new rule.' }}
                                </DialogDescription>
                            </DialogHeader>
                            <form
                                id="dialogForm"
                                @submit="handleSubmit($event, editingRule ? updateRule : addRule)"
                            >
                                <div class="grid gap-4 py-4">
                                    <FormField
                                        name="name"
                                        v-slot="{ componentField }"
                                    >
                                        <FormItem>
                                            <FormLabel>Name</FormLabel>
                                            <FormControl>
                                                <Input
                                                    placeholder="Rule name"
                                                    v-bind="componentField"
                                                />
                                            </FormControl>
                                            <FormMessage />
                                        </FormItem>
                                    </FormField>
                                    <FormField
                                        name="description"
                                        v-slot="{ componentField }"
                                    >
                                        <FormItem>
                                            <FormLabel>Description</FormLabel>
                                            <FormControl>
                                                <Textarea
                                                    placeholder="Rule description"
                                                    v-bind="componentField"
                                                />
                                            </FormControl>
                                            <FormMessage />
                                        </FormItem>
                                    </FormField>
                                    <FormField
                                        name="suggestedDuration"
                                        v-slot="{ componentField }"
                                    >
                                        <FormItem>
                                            <FormLabel>Suggested Duration (Days)</FormLabel>
                                            <FormControl>
                                                <Input
                                                    type="number"
                                                    min="0"
                                                    v-bind="componentField"
                                                />
                                            </FormControl>
                                            <FormDescription>
                                                Duration in days. 0 means permanent.
                                            </FormDescription>
                                            <FormMessage />
                                        </FormItem>
                                    </FormField>
                                </div>
                                <DialogFooter>
                                    <Button
                                        type="button"
                                        variant="outline"
                                        @click="showAddRuleDialog = false"
                                    >
                                        Cancel
                                    </Button>
                                    <Button
                                        type="submit"
                                        :disabled="addRuleLoading"
                                    >
                                        {{
                                            addRuleLoading
                                                ? editingRule
                                                    ? "Updating..."
                                                    : "Adding..."
                                                : editingRule
                                                    ? "Update Rule"
                                                    : "Add Rule"
                                        }}
                                    </Button>
                                </DialogFooter>
                            </form>
                        </DialogContent>
                    </Dialog>
                </Form>
                <Button
                    @click="fetchServerRules"
                    :disabled="loading"
                    variant="outline"
                >
                    {{ loading ? "Refreshing..." : "Refresh" }}
                </Button>
            </div>
        </div>

        <div v-if="error" class="bg-red-500 text-white p-4 rounded mb-4">
            {{ error }}
        </div>

        <Card>
            <CardHeader>
                <CardTitle>Rules List</CardTitle>
                <p class="text-sm text-muted-foreground">
                    Drag rules to reorder them. Drag a rule onto another rule to nest it.
                </p>
            </CardHeader>
            <CardContent>
                <div
                    v-if="loading && rules.length === 0"
                    class="text-center py-8"
                >
                    <div
                        class="animate-spin h-8 w-8 border-4 border-primary border-t-transparent rounded-full mx-auto mb-4"
                    ></div>
                    <p>Loading rules...</p>
                </div>

                <div
                    v-else-if="rules.length === 0"
                    class="text-center py-8"
                >
                    <p>No rules found</p>
                </div>

                <div v-else>
                    <draggable
                        v-model="rulesTree"
                        item-key="id"
                        handle=".drag-handle"
                        @end="onDragEnd"
                        :group="{ name: 'rules' }"
                        class="space-y-2"
                        :animation="50"
                        ghost-class="sortable-ghost"
                        drag-class="sortable-drag"
                    >
                        <template #item="slotProps">
                            <RuleItem
                                :rule="slotProps.element"
                                @update:rule="updateRuleInArray"
                                :on-edit="startEditRule"
                                :on-delete="deleteRule"
                                :on-drag-end="onDragEnd"
                                :on-nest="handleNest"
                            />
                        </template>
                    </draggable>
                </div>
            </CardContent>
        </Card>

        <Card class="mt-4">
            <CardHeader>
                <CardTitle>About Rules</CardTitle>
            </CardHeader>
            <CardContent>
                <p class="text-sm text-muted-foreground">
                    Rules can be organized hierarchically to create categories and subcategories.
                    Each rule can have a suggested duration that will be automatically applied when
                    selecting the rule during the ban process.
                </p>
                <p class="text-sm text-muted-foreground mt-2">
                    Rules with a duration of 0 days will result in permanent bans when selected.
                </p>
            </CardContent>
        </Card>
    </div>
</template>

<style scoped>
.drag-handle {
    cursor: move;
    user-select: none;
}

.sortable-ghost {
    opacity: 0.5;
    background: #c8ebfb;
}

.sortable-drag {
    opacity: 0.9;
    background: #ffffff;
    box-shadow: 0 2px 8px rgba(0, 0, 0, 0.1);
}

.dragging-fallback {
    opacity: 0.9;
    background: #ffffff;
    box-shadow: 0 2px 8px rgba(0, 0, 0, 0.1);
    transform: scale(1.02);
    max-width: 100%;
    z-index: 9999;
}
</style> 