<script setup lang="ts">
import { ref, onMounted, computed } from "vue";
import { Trash2, Plus, Edit, Database, Key, X, Check } from "lucide-vue-next";
import { Button } from "~/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "~/components/ui/card";
import {
    Table,
    TableBody,
    TableCell,
    TableHead,
    TableHeader,
    TableRow,
} from "~/components/ui/table";
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
import { Badge } from "~/components/ui/badge";
import { useToast } from "~/components/ui/toast";

interface KVPair {
    key: string;
    value: any;
}

interface Props {
    serverId: string;
    workflowId: string;
    workflowName?: string;
}

const props = defineProps<Props>();
const { toast } = useToast();

const loading = ref(false);
const kvPairs = ref<KVPair[]>([]);
const showAddDialog = ref(false);
const showEditDialog = ref(false);
const showDeleteDialog = ref(false);
const selectedKey = ref<string | null>(null);

const newKV = ref({
    key: "",
    value: "",
});

const editKV = ref({
    key: "",
    value: "",
    originalKey: "",
});

const searchQuery = ref("");

// Fetch KV pairs
async function fetchKVPairs() {
    loading.value = true;
    try {
        const runtimeConfig = useRuntimeConfig();
        const cookieToken = useCookie("auth_token", {
            path: "/",
        });
        const token = cookieToken.value;

        const { data, error: fetchError } = await useFetch<any>(
            `${runtimeConfig.public.apiBase}/api/v1/servers/${props.serverId}/workflows/${props.workflowId}/kv`,
            {
                headers: {
                    Authorization: `Bearer ${token}`,
                },
            },
        );

        if (fetchError.value) {
            throw new Error(fetchError.value.message);
        }

        if (data.value && data.value.data && data.value.data.kv_pairs) {
            kvPairs.value = data.value.data.kv_pairs;
        } else {
            kvPairs.value = [];
        }
    } catch (err: any) {
        toast({
            title: "Error",
            description: err.message || "Failed to fetch KV pairs",
            variant: "destructive",
        });
    } finally {
        loading.value = false;
    }
}

// Add KV pair
async function addKVPair() {
    if (!newKV.value.key.trim()) {
        toast({
            title: "Validation Error",
            description: "Key is required",
            variant: "destructive",
        });
        return;
    }

    try {
        let parsedValue: any;
        try {
            parsedValue = JSON.parse(newKV.value.value);
        } catch {
            // If not valid JSON, treat as string
            parsedValue = newKV.value.value;
        }

        const runtimeConfig = useRuntimeConfig();
        const cookieToken = useCookie("auth_token", {
            path: "/",
        });
        const token = cookieToken.value;

        const response = await fetch(
            `${runtimeConfig.public.apiBase}/api/v1/servers/${props.serverId}/workflows/${props.workflowId}/kv`,
            {
                method: "POST",
                headers: {
                    "Content-Type": "application/json",
                    Authorization: `Bearer ${token}`,
                },
                body: JSON.stringify({
                    key: newKV.value.key,
                    value: parsedValue,
                }),
            },
        );

        if (!response.ok) {
            const errorData = await response.json();
            throw new Error(errorData.message || "Failed to add KV pair");
        }

        toast({
            title: "Success",
            description: "KV pair added successfully",
        });

        showAddDialog.value = false;
        resetNewKV();
        await fetchKVPairs();
    } catch (err: any) {
        toast({
            title: "Error",
            description: err.message || "Failed to add KV pair",
            variant: "destructive",
        });
    }
}

// Edit KV pair
async function updateKVPair() {
    if (!editKV.value.key.trim()) {
        toast({
            title: "Validation Error",
            description: "Key is required",
            variant: "destructive",
        });
        return;
    }

    try {
        let parsedValue: any;
        try {
            parsedValue = JSON.parse(editKV.value.value);
        } catch {
            // If not valid JSON, treat as string
            parsedValue = editKV.value.value;
        }

        const runtimeConfig = useRuntimeConfig();
        const cookieToken = useCookie("auth_token", {
            path: "/",
        });
        const token = cookieToken.value;

        // If key changed, delete old key and create new one
        if (editKV.value.key !== editKV.value.originalKey) {
            await deleteKVPairByKey(editKV.value.originalKey, false);
        }

        const response = await fetch(
            `${runtimeConfig.public.apiBase}/api/v1/servers/${props.serverId}/workflows/${props.workflowId}/kv`,
            {
                method: "POST",
                headers: {
                    "Content-Type": "application/json",
                    Authorization: `Bearer ${token}`,
                },
                body: JSON.stringify({
                    key: editKV.value.key,
                    value: parsedValue,
                }),
            },
        );

        if (!response.ok) {
            const errorData = await response.json();
            throw new Error(errorData.message || "Failed to update KV pair");
        }

        toast({
            title: "Success",
            description: "KV pair updated successfully",
        });

        showEditDialog.value = false;
        await fetchKVPairs();
    } catch (err: any) {
        toast({
            title: "Error",
            description: err.message || "Failed to update KV pair",
            variant: "destructive",
        });
    }
}

// Delete KV pair
async function deleteKVPairByKey(key: string, showToast = true) {
    try {
        const runtimeConfig = useRuntimeConfig();
        const cookieToken = useCookie("auth_token", {
            path: "/",
        });
        const token = cookieToken.value;

        const response = await fetch(
            `${runtimeConfig.public.apiBase}/api/v1/servers/${props.serverId}/workflows/${props.workflowId}/kv/${encodeURIComponent(key)}`,
            {
                method: "DELETE",
                headers: {
                    Authorization: `Bearer ${token}`,
                },
            },
        );

        if (!response.ok) {
            const errorData = await response.json();
            throw new Error(errorData.message || "Failed to delete KV pair");
        }

        if (showToast) {
            toast({
                title: "Success",
                description: "KV pair deleted successfully",
            });
        }

        showDeleteDialog.value = false;
        selectedKey.value = null;
        await fetchKVPairs();
    } catch (err: any) {
        if (showToast) {
            toast({
                title: "Error",
                description: err.message || "Failed to delete KV pair",
                variant: "destructive",
            });
        } else {
            throw err;
        }
    }
}

async function deleteKVPair() {
    if (selectedKey.value) {
        await deleteKVPairByKey(selectedKey.value);
    }
}

// Clear all KV pairs
async function clearAllKV() {
    if (
        !confirm(
            "Are you sure you want to delete ALL KV pairs? This action cannot be undone.",
        )
    ) {
        return;
    }

    try {
        const runtimeConfig = useRuntimeConfig();
        const cookieToken = useCookie("auth_token", {
            path: "/",
        });
        const token = cookieToken.value;

        const response = await fetch(
            `${runtimeConfig.public.apiBase}/api/v1/servers/${props.serverId}/workflows/${props.workflowId}/kv`,
            {
                method: "DELETE",
                headers: {
                    Authorization: `Bearer ${token}`,
                },
            },
        );

        if (!response.ok) {
            const errorData = await response.json();
            throw new Error(errorData.message || "Failed to clear KV store");
        }

        toast({
            title: "Success",
            description: "All KV pairs cleared successfully",
        });

        await fetchKVPairs();
    } catch (err: any) {
        toast({
            title: "Error",
            description: err.message || "Failed to clear KV store",
            variant: "destructive",
        });
    }
}

function openEditDialog(pair: KVPair) {
    editKV.value = {
        key: pair.key,
        value:
            typeof pair.value === "string"
                ? pair.value
                : JSON.stringify(pair.value, null, 2),
        originalKey: pair.key,
    };
    showEditDialog.value = true;
}

function openDeleteDialog(key: string) {
    selectedKey.value = key;
    showDeleteDialog.value = true;
}

function resetNewKV() {
    newKV.value = {
        key: "",
        value: "",
    };
}

function formatValue(value: any): string {
    if (typeof value === "string") {
        return value;
    }
    return JSON.stringify(value);
}

function getValueType(value: any): string {
    if (value === null) return "null";
    if (Array.isArray(value)) return "array";
    return typeof value;
}

const filteredKVPairs = computed(() => {
    if (!searchQuery.value.trim()) {
        return kvPairs.value;
    }
    const query = searchQuery.value.toLowerCase();
    return kvPairs.value.filter(
        (pair) =>
            pair.key.toLowerCase().includes(query) ||
            formatValue(pair.value).toLowerCase().includes(query),
    );
});

onMounted(() => {
    fetchKVPairs();
});
</script>

<template>
    <div class="space-y-4">
        <Card>
            <CardHeader>
                <div class="flex items-center justify-between">
                    <div class="flex items-center gap-2">
                        <Database class="h-5 w-5" />
                        <CardTitle>Persistent KV Store</CardTitle>
                    </div>
                    <div class="flex items-center gap-2">
                        <Badge variant="outline">
                            {{ kvPairs.length }}
                            {{ kvPairs.length === 1 ? "item" : "items" }}
                        </Badge>
                    </div>
                </div>
                <p class="text-sm text-muted-foreground mt-2">
                    Manage persistent key-value storage for
                    <span class="font-semibold">{{
                        workflowName || "this workflow"
                    }}</span
                    >. Data persists across executions and is only accessible
                    through Lua scripts.
                </p>
            </CardHeader>
            <CardContent>
                <div class="space-y-4">
                    <!-- Actions Bar -->
                    <div class="flex items-center gap-2">
                        <Input
                            v-model="searchQuery"
                            placeholder="Search keys or values..."
                            class="max-w-sm"
                        />
                        <div class="ml-auto flex gap-2">
                            <Button
                                variant="outline"
                                size="sm"
                                @click="fetchKVPairs"
                                :disabled="loading"
                            >
                                <Database class="mr-2 h-4 w-4" />
                                Refresh
                            </Button>
                            <Button
                                variant="destructive"
                                size="sm"
                                @click="clearAllKV"
                                :disabled="loading || kvPairs.length === 0"
                            >
                                <Trash2 class="mr-2 h-4 w-4" />
                                Clear All
                            </Button>
                            <Button size="sm" @click="showAddDialog = true">
                                <Plus class="mr-2 h-4 w-4" />
                                Add KV Pair
                            </Button>
                        </div>
                    </div>

                    <!-- KV Pairs Table -->
                    <div class="border rounded-lg">
                        <Table>
                            <TableHeader>
                                <TableRow>
                                    <TableHead class="w-[250px]">Key</TableHead>
                                    <TableHead>Value</TableHead>
                                    <TableHead class="w-[100px]"
                                        >Type</TableHead
                                    >
                                    <TableHead class="w-[100px] text-right"
                                        >Actions</TableHead
                                    >
                                </TableRow>
                            </TableHeader>
                            <TableBody>
                                <TableRow v-if="loading">
                                    <TableCell
                                        colspan="4"
                                        class="text-center py-8"
                                    >
                                        <div
                                            class="flex items-center justify-center gap-2"
                                        >
                                            <div
                                                class="animate-spin h-4 w-4 border-2 border-primary border-t-transparent rounded-full"
                                            ></div>
                                            Loading KV pairs...
                                        </div>
                                    </TableCell>
                                </TableRow>
                                <TableRow
                                    v-else-if="filteredKVPairs.length === 0"
                                >
                                    <TableCell
                                        colspan="4"
                                        class="text-center py-8 text-muted-foreground"
                                    >
                                        <div
                                            class="flex flex-col items-center gap-2"
                                        >
                                            <Key class="h-8 w-8 opacity-20" />
                                            <p>
                                                {{
                                                    searchQuery
                                                        ? "No matching KV pairs found"
                                                        : "No KV pairs yet"
                                                }}
                                            </p>
                                            <p
                                                class="text-sm"
                                                v-if="!searchQuery"
                                            >
                                                Add a key-value pair to get
                                                started
                                            </p>
                                        </div>
                                    </TableCell>
                                </TableRow>
                                <TableRow
                                    v-for="pair in filteredKVPairs"
                                    :key="pair.key"
                                    v-else
                                >
                                    <TableCell class="font-mono text-sm">
                                        {{ pair.key }}
                                    </TableCell>
                                    <TableCell
                                        class="font-mono text-sm max-w-md truncate"
                                    >
                                        {{ formatValue(pair.value) }}
                                    </TableCell>
                                    <TableCell>
                                        <Badge
                                            variant="secondary"
                                            class="text-xs"
                                        >
                                            {{ getValueType(pair.value) }}
                                        </Badge>
                                    </TableCell>
                                    <TableCell class="text-right">
                                        <div class="flex justify-end gap-1">
                                            <Button
                                                variant="ghost"
                                                size="sm"
                                                @click="openEditDialog(pair)"
                                            >
                                                <Edit class="h-4 w-4" />
                                            </Button>
                                            <Button
                                                variant="ghost"
                                                size="sm"
                                                @click="
                                                    openDeleteDialog(pair.key)
                                                "
                                            >
                                                <Trash2
                                                    class="h-4 w-4 text-destructive"
                                                />
                                            </Button>
                                        </div>
                                    </TableCell>
                                </TableRow>
                            </TableBody>
                        </Table>
                    </div>
                </div>
            </CardContent>
        </Card>

        <!-- Add KV Dialog -->
        <Dialog v-model:open="showAddDialog">
            <DialogContent>
                <DialogHeader>
                    <DialogTitle>Add KV Pair</DialogTitle>
                    <DialogDescription>
                        Add a new key-value pair to the persistent store. Values
                        can be strings, numbers, booleans, objects, or arrays
                        (as JSON).
                    </DialogDescription>
                </DialogHeader>
                <div class="space-y-4 py-4">
                    <div class="space-y-2">
                        <Label for="new-key">Key</Label>
                        <Input
                            id="new-key"
                            v-model="newKV.key"
                            placeholder="e.g., player_warnings, config_max_players"
                            maxlength="255"
                        />
                        <p class="text-xs text-muted-foreground">
                            Maximum 255 characters
                        </p>
                    </div>
                    <div class="space-y-2">
                        <Label for="new-value">Value</Label>
                        <Textarea
                            id="new-value"
                            v-model="newKV.value"
                            placeholder='e.g., "Hello World", 42, {"enabled": true}, ["item1", "item2"]'
                            rows="6"
                            class="font-mono text-sm"
                        />
                        <p class="text-xs text-muted-foreground">
                            Enter a string or valid JSON (object, array, number,
                            boolean)
                        </p>
                    </div>
                </div>
                <DialogFooter>
                    <Button variant="outline" @click="showAddDialog = false">
                        Cancel
                    </Button>
                    <Button @click="addKVPair">
                        <Plus class="mr-2 h-4 w-4" />
                        Add
                    </Button>
                </DialogFooter>
            </DialogContent>
        </Dialog>

        <!-- Edit KV Dialog -->
        <Dialog v-model:open="showEditDialog">
            <DialogContent>
                <DialogHeader>
                    <DialogTitle>Edit KV Pair</DialogTitle>
                    <DialogDescription>
                        Modify the key or value. Changing the key will delete
                        the old entry and create a new one.
                    </DialogDescription>
                </DialogHeader>
                <div class="space-y-4 py-4">
                    <div class="space-y-2">
                        <Label for="edit-key">Key</Label>
                        <Input
                            id="edit-key"
                            v-model="editKV.key"
                            maxlength="255"
                        />
                    </div>
                    <div class="space-y-2">
                        <Label for="edit-value">Value</Label>
                        <Textarea
                            id="edit-value"
                            v-model="editKV.value"
                            rows="6"
                            class="font-mono text-sm"
                        />
                        <p class="text-xs text-muted-foreground">
                            Enter a string or valid JSON
                        </p>
                    </div>
                </div>
                <DialogFooter>
                    <Button variant="outline" @click="showEditDialog = false">
                        Cancel
                    </Button>
                    <Button @click="updateKVPair">
                        <Check class="mr-2 h-4 w-4" />
                        Update
                    </Button>
                </DialogFooter>
            </DialogContent>
        </Dialog>

        <!-- Delete Confirmation Dialog -->
        <Dialog v-model:open="showDeleteDialog">
            <DialogContent>
                <DialogHeader>
                    <DialogTitle>Delete KV Pair</DialogTitle>
                    <DialogDescription>
                        Are you sure you want to delete the key "{{
                            selectedKey
                        }}"? This action cannot be undone.
                    </DialogDescription>
                </DialogHeader>
                <DialogFooter>
                    <Button variant="outline" @click="showDeleteDialog = false">
                        Cancel
                    </Button>
                    <Button variant="destructive" @click="deleteKVPair">
                        <Trash2 class="mr-2 h-4 w-4" />
                        Delete
                    </Button>
                </DialogFooter>
            </DialogContent>
        </Dialog>
    </div>
</template>
