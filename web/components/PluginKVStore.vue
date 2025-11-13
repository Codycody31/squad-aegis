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
import JSONInput from "~/components/JSONInput.vue";

interface PluginDataItem {
    key: string;
    value: string;
}

interface Props {
    serverId: string;
    pluginId: string;
    pluginName?: string;
}

const props = defineProps<Props>();
const { toast } = useToast();

const loading = ref(false);
const dataItems = ref<PluginDataItem[]>([]);
const showAddDialog = ref(false);
const showEditDialog = ref(false);
const showDeleteDialog = ref(false);
const selectedKey = ref<string | null>(null);

const newData = ref({
    key: "",
    value: "",
});

const editData = ref({
    key: "",
    value: "",
    originalKey: "",
});

const searchQuery = ref("");

// Fetch plugin data
async function fetchPluginData() {
    loading.value = true;
    try {
        const runtimeConfig = useRuntimeConfig();
        const cookieToken = useCookie(
            runtimeConfig.public.sessionCookieName as string,
        );
        const token = cookieToken.value;

        const response = await fetch(
            `${runtimeConfig.public.backendApi}/servers/${props.serverId}/plugins/${props.pluginId}/data`,
            {
                headers: {
                    Authorization: `Bearer ${token}`,
                },
            },
        );

        if (!response.ok) {
            const errorData = await response.json();
            throw new Error(errorData.message || "Failed to fetch plugin data");
        }

        const result = await response.json();
        if (result.data.data && Array.isArray(result.data.data)) {
            dataItems.value = result.data.data;
        } else {
            dataItems.value = [];
        }
    } catch (err: any) {
        toast({
            title: "Error",
            description: err.message || "Failed to fetch plugin data",
            variant: "destructive",
        });
    } finally {
        loading.value = false;
    }
}

// Add data item
async function addDataItem() {
    if (!newData.value.key.trim()) {
        toast({
            title: "Validation Error",
            description: "Key is required",
            variant: "destructive",
        });
        return;
    }

    try {
        const runtimeConfig = useRuntimeConfig();
        const cookieToken = useCookie(
            runtimeConfig.public.sessionCookieName as string,
        );
        const token = cookieToken.value;

        const response = await fetch(
            `${runtimeConfig.public.backendApi}/servers/${props.serverId}/plugins/${props.pluginId}/data`,
            {
                method: "POST",
                headers: {
                    "Content-Type": "application/json",
                    Authorization: `Bearer ${token}`,
                },
                body: JSON.stringify({
                    key: newData.value.key,
                    value: newData.value.value,
                }),
            },
        );

        if (!response.ok) {
            const errorData = await response.json();
            throw new Error(errorData.message || "Failed to add data item");
        }

        toast({
            title: "Success",
            description: "Data item added successfully",
        });

        showAddDialog.value = false;
        resetNewData();
        await fetchPluginData();
    } catch (err: any) {
        toast({
            title: "Error",
            description: err.message || "Failed to add data item",
            variant: "destructive",
        });
    }
}

// Edit data item
async function updateDataItem() {
    if (!editData.value.key.trim()) {
        toast({
            title: "Validation Error",
            description: "Key is required",
            variant: "destructive",
        });
        return;
    }

    try {
        const runtimeConfig = useRuntimeConfig();
        const cookieToken = useCookie(
            runtimeConfig.public.sessionCookieName as string,
        );
        const token = cookieToken.value;

        // If key changed, delete old key first
        if (editData.value.key !== editData.value.originalKey) {
            await deleteDataItemByKey(editData.value.originalKey, false);
        }

        const response = await fetch(
            `${runtimeConfig.public.backendApi}/servers/${props.serverId}/plugins/${props.pluginId}/data`,
            {
                method: "POST",
                headers: {
                    "Content-Type": "application/json",
                    Authorization: `Bearer ${token}`,
                },
                body: JSON.stringify({
                    key: editData.value.key,
                    value: editData.value.value,
                }),
            },
        );

        if (!response.ok) {
            const errorData = await response.json();
            throw new Error(errorData.message || "Failed to update data item");
        }

        toast({
            title: "Success",
            description: "Data item updated successfully",
        });

        showEditDialog.value = false;
        await fetchPluginData();
    } catch (err: any) {
        toast({
            title: "Error",
            description: err.message || "Failed to update data item",
            variant: "destructive",
        });
    }
}

// Delete data item
async function deleteDataItemByKey(key: string, showToast = true) {
    try {
        const runtimeConfig = useRuntimeConfig();
        const cookieToken = useCookie(
            runtimeConfig.public.sessionCookieName as string,
        );
        const token = cookieToken.value;

        const response = await fetch(
            `${runtimeConfig.public.backendApi}/servers/${props.serverId}/plugins/${props.pluginId}/data/${encodeURIComponent(key)}`,
            {
                method: "DELETE",
                headers: {
                    Authorization: `Bearer ${token}`,
                },
            },
        );

        if (!response.ok) {
            const errorData = await response.json();
            throw new Error(errorData.message || "Failed to delete data item");
        }

        if (showToast) {
            toast({
                title: "Success",
                description: "Data item deleted successfully",
            });
        }

        showDeleteDialog.value = false;
        selectedKey.value = null;
        await fetchPluginData();
    } catch (err: any) {
        if (showToast) {
            toast({
                title: "Error",
                description: err.message || "Failed to delete data item",
                variant: "destructive",
            });
        } else {
            throw err;
        }
    }
}

async function deleteDataItem() {
    if (selectedKey.value) {
        await deleteDataItemByKey(selectedKey.value);
    }
}

// Clear all data
async function clearAllData() {
    if (
        !confirm(
            "Are you sure you want to delete ALL plugin data? This action cannot be undone.",
        )
    ) {
        return;
    }

    try {
        const runtimeConfig = useRuntimeConfig();
        const cookieToken = useCookie(
            runtimeConfig.public.sessionCookieName as string,
        );
        const token = cookieToken.value;

        const response = await fetch(
            `${runtimeConfig.public.backendApi}/servers/${props.serverId}/plugins/${props.pluginId}/data`,
            {
                method: "DELETE",
                headers: {
                    Authorization: `Bearer ${token}`,
                },
            },
        );

        if (!response.ok) {
            const errorData = await response.json();
            throw new Error(errorData.message || "Failed to clear plugin data");
        }

        toast({
            title: "Success",
            description: "All plugin data cleared successfully",
        });

        await fetchPluginData();
    } catch (err: any) {
        toast({
            title: "Error",
            description: err.message || "Failed to clear plugin data",
            variant: "destructive",
        });
    }
}

function openEditDialog(item: PluginDataItem) {
    editData.value = {
        key: item.key,
        value: item.value,
        originalKey: item.key,
    };
    showEditDialog.value = true;
}

function openDeleteDialog(key: string) {
    selectedKey.value = key;
    showDeleteDialog.value = true;
}

function resetNewData() {
    newData.value = {
        key: "",
        value: "",
    };
}

function isJSON(str: string): boolean {
    try {
        JSON.parse(str);
        return true;
    } catch {
        return false;
    }
}

function formatValue(value: string): string {
    if (isJSON(value)) {
        try {
            const parsed = JSON.parse(value);
            return JSON.stringify(parsed);
        } catch {
            return value;
        }
    }
    return value;
}

function getValueType(value: string): string {
    if (isJSON(value)) {
        try {
            const parsed = JSON.parse(value);
            if (parsed === null) return "null";
            if (Array.isArray(parsed)) return "array";
            return typeof parsed;
        } catch {
            return "string";
        }
    }
    return "string";
}

const filteredDataItems = computed(() => {
    if (!searchQuery.value.trim()) {
        return dataItems.value;
    }
    const query = searchQuery.value.toLowerCase();
    return dataItems.value.filter(
        (item) =>
            item.key.toLowerCase().includes(query) ||
            item.value.toLowerCase().includes(query),
    );
});

onMounted(() => {
    fetchPluginData();
});
</script>

<template>
    <div class="space-y-4">
        <Card>
            <CardHeader>
                <div class="flex items-center justify-between">
                    <div class="flex items-center gap-2">
                        <Database class="h-5 w-5" />
                        <CardTitle>Plugin Data Store</CardTitle>
                    </div>
                    <div class="flex items-center gap-2">
                        <Badge variant="outline">
                            {{ dataItems.length }}
                            {{ dataItems.length === 1 ? "item" : "items" }}
                        </Badge>
                    </div>
                </div>
                <p class="text-sm text-muted-foreground mt-2">
                    Manage persistent data storage for
                    <span class="font-semibold">{{
                        pluginName || "this plugin"
                    }}</span
                    >. Data persists across plugin restarts and is accessible
                    through the plugin API.
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
                                @click="fetchPluginData"
                                :disabled="loading"
                            >
                                <Database class="mr-2 h-4 w-4" />
                                Refresh
                            </Button>
                            <Button
                                variant="destructive"
                                size="sm"
                                @click="clearAllData"
                                :disabled="loading || dataItems.length === 0"
                            >
                                <Trash2 class="mr-2 h-4 w-4" />
                                Clear All
                            </Button>
                            <Button size="sm" @click="showAddDialog = true">
                                <Plus class="mr-2 h-4 w-4" />
                                Add Data Item
                            </Button>
                        </div>
                    </div>

                    <!-- Data Items Table -->
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
                                            Loading plugin data...
                                        </div>
                                    </TableCell>
                                </TableRow>
                                <TableRow
                                    v-else-if="filteredDataItems.length === 0"
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
                                                        ? "No matching data items found"
                                                        : "No data items yet"
                                                }}
                                            </p>
                                            <p
                                                class="text-sm"
                                                v-if="!searchQuery"
                                            >
                                                Add a data item to get started
                                            </p>
                                        </div>
                                    </TableCell>
                                </TableRow>
                                <TableRow
                                    v-for="item in filteredDataItems"
                                    :key="item.key"
                                    v-else
                                >
                                    <TableCell class="font-mono text-sm">
                                        {{ item.key }}
                                    </TableCell>
                                    <TableCell
                                        class="font-mono text-sm max-w-md truncate"
                                    >
                                        {{ formatValue(item.value) }}
                                    </TableCell>
                                    <TableCell>
                                        <Badge
                                            variant="secondary"
                                            class="text-xs"
                                        >
                                            {{ getValueType(item.value) }}
                                        </Badge>
                                    </TableCell>
                                    <TableCell class="text-right">
                                        <div class="flex justify-end gap-1">
                                            <Button
                                                variant="ghost"
                                                size="sm"
                                                @click="openEditDialog(item)"
                                            >
                                                <Edit class="h-4 w-4" />
                                            </Button>
                                            <Button
                                                variant="ghost"
                                                size="sm"
                                                @click="
                                                    openDeleteDialog(item.key)
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

        <!-- Add Data Dialog -->
        <Dialog v-model:open="showAddDialog">
            <DialogContent>
                <DialogHeader>
                    <DialogTitle>Add Data Item</DialogTitle>
                    <DialogDescription>
                        Add a new data item to the plugin's persistent store.
                        Values are stored as strings.
                    </DialogDescription>
                </DialogHeader>
                <div class="space-y-4 py-4">
                    <div class="space-y-2">
                        <Label for="new-key">Key</Label>
                        <Input
                            id="new-key"
                            v-model="newData.key"
                            placeholder="e.g., player_stats, config_value"
                        />
                    </div>
                    <div class="space-y-2">
                        <JSONInput
                            v-model="newData.value"
                            label="Value"
                            placeholder='e.g., "Hello World", {"enabled": true}, ["item1", "item2"]'
                            :rows="6"
                        />
                    </div>
                </div>
                <DialogFooter>
                    <Button variant="outline" @click="showAddDialog = false">
                        Cancel
                    </Button>
                    <Button @click="addDataItem">
                        <Plus class="mr-2 h-4 w-4" />
                        Add
                    </Button>
                </DialogFooter>
            </DialogContent>
        </Dialog>

        <!-- Edit Data Dialog -->
        <Dialog v-model:open="showEditDialog">
            <DialogContent>
                <DialogHeader>
                    <DialogTitle>Edit Data Item</DialogTitle>
                    <DialogDescription>
                        Modify the key or value. Changing the key will delete
                        the old entry and create a new one.
                    </DialogDescription>
                </DialogHeader>
                <div class="space-y-4 py-4">
                    <div class="space-y-2">
                        <Label for="edit-key">Key</Label>
                        <Input id="edit-key" v-model="editData.key" />
                    </div>
                    <div class="space-y-2">
                        <JSONInput
                            v-model="editData.value"
                            label="Value"
                            :rows="6"
                        />
                    </div>
                </div>
                <DialogFooter>
                    <Button variant="outline" @click="showEditDialog = false">
                        Cancel
                    </Button>
                    <Button @click="updateDataItem">
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
                    <DialogTitle>Delete Data Item</DialogTitle>
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
                    <Button variant="destructive" @click="deleteDataItem">
                        <Trash2 class="mr-2 h-4 w-4" />
                        Delete
                    </Button>
                </DialogFooter>
            </DialogContent>
        </Dialog>
    </div>
</template>
