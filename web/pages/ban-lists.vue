<script setup lang="ts">
import { ref, onMounted } from "vue";
import { Button } from "~/components/ui/button";
import {
    Card,
    CardContent,
    CardDescription,
    CardHeader,
    CardTitle,
} from "~/components/ui/card";
import {
    Table,
    TableBody,
    TableCell,
    TableHead,
    TableHeader,
    TableRow,
} from "~/components/ui/table";
import { Badge } from "~/components/ui/badge";
import {
    Dialog,
    DialogContent,
    DialogDescription,
    DialogFooter,
    DialogHeader,
    DialogTitle,
    DialogTrigger,
} from "~/components/ui/dialog";
import { Input } from "~/components/ui/input";
import { Label } from "~/components/ui/label";
import { Textarea } from "~/components/ui/textarea";
import { Switch } from "~/components/ui/switch";
import { toast } from "~/components/ui/toast";
import { useAuthStore } from "~/stores/auth";
import {
    Settings,
    Plus,
    Trash2,
    Edit,
    ExternalLink,
    Users,
    Download,
    Shield,
    Link,
    Eye,
} from "lucide-vue-next";

definePageMeta({
    middleware: ["auth"],
});

useHead({
    title: "Ban Lists Management",
});

const authStore = useAuthStore();
const runtimeConfig = useRuntimeConfig();

// Helper function to get ban list cfg URL
const getBanListCfgUrl = (banListId: string) => {
    if (runtimeConfig.public.backendApi.startsWith("/")) {
        // Relative URL, construct full URL
        const origin = window.location.origin;
        return `${origin}${runtimeConfig.public.backendApi}/ban-lists/${banListId}/cfg`;
    }
    return `${runtimeConfig.public.backendApi}/ban-lists/${banListId}/cfg`;
};

// Helper function to copy cfg URL to clipboard
const copyCfgUrl = async (banListId: string) => {
    const url = getBanListCfgUrl(banListId);
    try {
        await navigator.clipboard.writeText(url);
        toast({
            title: "URL Copied",
            description: "Ban list CFG URL copied to clipboard",
        });
    } catch (err) {
        toast({
            title: "Copy Failed",
            description: "Failed to copy URL to clipboard",
            variant: "destructive",
        });
    }
};

// Helper function to open cfg in new tab
const viewCfg = (banListId: string) => {
    const url = getBanListCfgUrl(banListId);
    window.open(url, "_blank");
};

// State variables
const loading = ref(true);
const banLists = ref<any[]>([]);
const remoteSources = ref<any[]>([]);
const ignoredSteamIDs = ref<any[]>([]);
const showCreateDialog = ref(false);
const showEditDialog = ref(false);
const showRemoteSourceDialog = ref(false);
const showIgnoredSteamIDDialog = ref(false);
const currentBanList = ref<any>(null);
const currentRemoteSource = ref<any>(null);
const currentIgnoredSteamID = ref<any>(null);

// Form data
const banListForm = ref({
    name: "",
    description: "",
    is_remote: false,
    remote_url: "",
    remote_sync_enabled: false,
});

const remoteSourceForm = ref({
    name: "",
    url: "",
    sync_enabled: true,
    sync_interval_minutes: 180,
});

const ignoredSteamIDForm = ref({
    steam_id: "",
    reason: "",
});

// Load all data
const loadData = async () => {
    loading.value = true;
    try {
        await Promise.all([
            loadBanLists(),
            loadRemoteSources(),
            loadIgnoredSteamIDs(),
        ]);
    } finally {
        loading.value = false;
    }
};

// Load ban lists
const loadBanLists = async () => {
    const cookieToken = useCookie(
        runtimeConfig.public.sessionCookieName as string,
    );
    const token = cookieToken.value;

    if (!token) {
        toast({
            title: "Error",
            description: "Authentication required",
            variant: "destructive",
        });
        return;
    }

    try {
        const response = (await $fetch(
            `${runtimeConfig.public.backendApi}/ban-lists`,
            {
                headers: {
                    Authorization: `Bearer ${token}`,
                },
            },
        )) as any;
        banLists.value = response.data.ban_lists || [];
    } catch (error: any) {
        console.error("Failed to load ban lists:", error);
        toast({
            title: "Error",
            description: "Failed to load ban lists",
            variant: "destructive",
        });
    }
};

// Load remote sources
const loadRemoteSources = async () => {
    const cookieToken = useCookie(
        runtimeConfig.public.sessionCookieName as string,
    );
    const token = cookieToken.value;

    if (!token) {
        toast({
            title: "Error",
            description: "Authentication required",
            variant: "destructive",
        });
        return;
    }

    try {
        const response = (await $fetch(
            `${runtimeConfig.public.backendApi}/remote-ban-sources`,
            {
                headers: {
                    Authorization: `Bearer ${token}`,
                },
            },
        )) as any;
        remoteSources.value = response.data.sources || [];
    } catch (error: any) {
        console.error("Failed to load remote sources:", error);
        toast({
            title: "Error",
            description: "Failed to load remote sources",
            variant: "destructive",
        });
    }
};

// Load ignored Steam IDs
const loadIgnoredSteamIDs = async () => {
    const cookieToken = useCookie(
        runtimeConfig.public.sessionCookieName as string,
    );
    const token = cookieToken.value;

    if (!token) {
        toast({
            title: "Error",
            description: "Authentication required",
            variant: "destructive",
        });
        return;
    }

    try {
        const response = (await $fetch(
            `${runtimeConfig.public.backendApi}/ignored-steam-ids`,
            {
                headers: {
                    Authorization: `Bearer ${token}`,
                },
            },
        )) as any;
        ignoredSteamIDs.value = response.data.ignored_steam_ids || [];
    } catch (error: any) {
        console.error("Failed to load ignored Steam IDs:", error);
        toast({
            title: "Error",
            description: "Failed to load ignored Steam IDs",
            variant: "destructive",
        });
    }
};

// Create ban list
const createBanList = async () => {
    const cookieToken = useCookie(
        runtimeConfig.public.sessionCookieName as string,
    );
    const token = cookieToken.value;

    if (!token) return;

    try {
        await $fetch(`${runtimeConfig.public.backendApi}/ban-lists`, {
            method: "POST",
            headers: {
                Authorization: `Bearer ${token}`,
            },
            body: banListForm.value,
        });

        toast({
            title: "Success",
            description: "Ban list created successfully",
        });

        showCreateDialog.value = false;
        resetBanListForm();
        await loadBanLists();
    } catch (error: any) {
        console.error("Failed to create ban list:", error);
        toast({
            title: "Error",
            description: "Failed to create ban list",
            variant: "destructive",
        });
    }
};

// Update ban list
const updateBanList = async () => {
    if (!currentBanList.value) return;

    const cookieToken = useCookie(
        runtimeConfig.public.sessionCookieName as string,
    );
    const token = cookieToken.value;

    if (!token) return;

    try {
        await $fetch(
            `${runtimeConfig.public.backendApi}/ban-lists/${currentBanList.value.id}`,
            {
                method: "PUT",
                headers: {
                    Authorization: `Bearer ${token}`,
                },
                body: banListForm.value,
            },
        );

        toast({
            title: "Success",
            description: "Ban list updated successfully",
        });

        showEditDialog.value = false;
        resetBanListForm();
        await loadBanLists();
    } catch (error: any) {
        console.error("Failed to update ban list:", error);
        toast({
            title: "Error",
            description: "Failed to update ban list",
            variant: "destructive",
        });
    }
};

// Delete ban list
const deleteBanList = async (banList: any) => {
    if (
        !confirm(
            `Are you sure you want to delete the ban list "${banList.name}"?`,
        )
    ) {
        return;
    }

    const cookieToken = useCookie(
        runtimeConfig.public.sessionCookieName as string,
    );
    const token = cookieToken.value;

    if (!token) return;

    try {
        await $fetch(
            `${runtimeConfig.public.backendApi}/ban-lists/${banList.id}`,
            {
                method: "DELETE",
                headers: {
                    Authorization: `Bearer ${token}`,
                },
            },
        );

        toast({
            title: "Success",
            description: "Ban list deleted successfully",
        });

        await loadBanLists();
    } catch (error: any) {
        console.error("Failed to delete ban list:", error);
        toast({
            title: "Error",
            description: "Failed to delete ban list",
            variant: "destructive",
        });
    }
};

// Create remote source
const createRemoteSource = async () => {
    const cookieToken = useCookie(
        runtimeConfig.public.sessionCookieName as string,
    );
    const token = cookieToken.value;

    if (!token) return;

    try {
        await $fetch(`${runtimeConfig.public.backendApi}/remote-ban-sources`, {
            method: "POST",
            headers: {
                Authorization: `Bearer ${token}`,
            },
            body: remoteSourceForm.value,
        });

        toast({
            title: "Success",
            description: "Remote source created successfully",
        });

        showRemoteSourceDialog.value = false;
        resetRemoteSourceForm();
        await loadRemoteSources();
    } catch (error: any) {
        console.error("Failed to create remote source:", error);
        toast({
            title: "Error",
            description: "Failed to create remote source",
            variant: "destructive",
        });
    }
};

// Delete remote source
const deleteRemoteSource = async (remoteSource: any) => {
    if (
        !confirm(
            `Are you sure you want to delete the remote source "${remoteSource.name}"?`,
        )
    ) {
        return;
    }

    const cookieToken = useCookie(
        runtimeConfig.public.sessionCookieName as string,
    );
    const token = cookieToken.value;

    if (!token) return;

    try {
        await $fetch(
            `${runtimeConfig.public.backendApi}/remote-ban-sources/${remoteSource.id}`,
            {
                method: "DELETE",
                headers: {
                    Authorization: `Bearer ${token}`,
                },
            },
        );

        toast({
            title: "Success",
            description: "Remote source deleted successfully",
        });

        await loadRemoteSources();
    } catch (error: any) {
        console.error("Failed to delete remote source:", error);
        toast({
            title: "Error",
            description: "Failed to delete remote source",
            variant: "destructive",
        });
    }
};

// Create ignored Steam ID
const createIgnoredSteamID = async () => {
    const cookieToken = useCookie(
        runtimeConfig.public.sessionCookieName as string,
    );
    const token = cookieToken.value;

    if (!token) return;

    try {
        await $fetch(`${runtimeConfig.public.backendApi}/ignored-steam-ids`, {
            method: "POST",
            headers: {
                Authorization: `Bearer ${token}`,
            },
            body: ignoredSteamIDForm.value,
        });

        toast({
            title: "Success",
            description: "Steam ID added to ignore list successfully",
        });

        showIgnoredSteamIDDialog.value = false;
        resetIgnoredSteamIDForm();
        await loadIgnoredSteamIDs();
    } catch (error: any) {
        console.error("Failed to create ignored Steam ID:", error);
        toast({
            title: "Error",
            description: "Failed to create ignored Steam ID",
            variant: "destructive",
        });
    }
};

// Delete ignored Steam ID
const deleteIgnoredSteamID = async (ignoredSteamID: any) => {
    if (
        !confirm(
            `Are you sure you want to remove Steam ID "${ignoredSteamID.steam_id}" from the ignore list?`,
        )
    ) {
        return;
    }

    const cookieToken = useCookie(
        runtimeConfig.public.sessionCookieName as string,
    );
    const token = cookieToken.value;

    if (!token) return;

    try {
        await $fetch(
            `${runtimeConfig.public.backendApi}/ignored-steam-ids/${ignoredSteamID.id}`,
            {
                method: "DELETE",
                headers: {
                    Authorization: `Bearer ${token}`,
                },
            },
        );

        toast({
            title: "Success",
            description: "Steam ID removed from ignore list successfully",
        });

        await loadIgnoredSteamIDs();
    } catch (error: any) {
        console.error("Failed to delete ignored Steam ID:", error);
        toast({
            title: "Error",
            description: "Failed to delete ignored Steam ID",
            variant: "destructive",
        });
    }
};

// Form reset functions
const resetBanListForm = () => {
    banListForm.value = {
        name: "",
        description: "",
        is_remote: false,
        remote_url: "",
        remote_sync_enabled: false,
    };
};

const resetRemoteSourceForm = () => {
    remoteSourceForm.value = {
        name: "",
        url: "",
        sync_enabled: true,
        sync_interval_minutes: 180,
    };
};

const resetIgnoredSteamIDForm = () => {
    ignoredSteamIDForm.value = {
        steam_id: "",
        reason: "",
    };
};

// Edit functions
const editBanList = (banList: any) => {
    currentBanList.value = banList;
    banListForm.value = {
        name: banList.name,
        description: banList.description || "",
        is_remote: banList.is_remote || false,
        remote_url: banList.remote_url || "",
        remote_sync_enabled: banList.remote_sync_enabled || false,
    };
    showEditDialog.value = true;
};

// Mount hook
onMounted(() => {
    loadData();
});
</script>

<template>
    <div class="p-3 sm:p-4 lg:p-6 space-y-4 sm:space-y-6">
        <!-- Header -->
        <div class="flex flex-col sm:flex-row sm:justify-between sm:items-center gap-3 sm:gap-0">
            <div>
                <h1 class="text-xl sm:text-2xl lg:text-3xl font-bold">Ban Lists Management</h1>
                <p class="text-xs sm:text-sm text-muted-foreground mt-1 sm:mt-2">
                    Manage ban lists, remote sources, and ignored Steam IDs for
                    shared ban coordination.
                </p>
            </div>
            <Button @click="loadData" :disabled="loading" class="w-full sm:w-auto text-sm sm:text-base">
                <Download class="h-4 w-4 mr-2" />
                {{ loading ? "Loading..." : "Refresh" }}
            </Button>
        </div>

        <!-- Ban Lists Section -->
        <Card>
            <CardHeader class="pb-2 sm:pb-3">
                <div class="flex flex-col sm:flex-row sm:justify-between sm:items-center gap-3 sm:gap-0">
                    <div>
                        <CardTitle class="flex items-center gap-2 text-base sm:text-lg">
                            <Shield class="h-4 w-4 sm:h-5 sm:w-5" />
                            Ban Lists
                        </CardTitle>
                        <CardDescription class="text-xs sm:text-sm">
                            Manage local and remote ban lists that can be shared
                            across servers.
                        </CardDescription>
                    </div>
                    <Dialog v-model:open="showCreateDialog">
                        <DialogTrigger asChild>
                            <Button class="w-full sm:w-auto text-sm sm:text-base">
                                <Plus class="h-4 w-4 mr-2" />
                                Create Ban List
                            </Button>
                        </DialogTrigger>
                        <DialogContent class="w-[95vw] sm:max-w-[500px] max-h-[90vh] overflow-y-auto p-4 sm:p-6">
                            <DialogHeader>
                                <DialogTitle class="text-base sm:text-lg">Create New Ban List</DialogTitle>
                                <DialogDescription class="text-xs sm:text-sm">
                                    Create a new ban list that can be shared
                                    with other servers.
                                </DialogDescription>
                            </DialogHeader>
                            <div class="grid gap-4 py-4">
                                <div>
                                    <Label htmlFor="name">Name</Label>
                                    <Input
                                        id="name"
                                        v-model="banListForm.name"
                                        placeholder="Ban List Name"
                                    />
                                </div>
                                <div>
                                    <Label htmlFor="description"
                                        >Description</Label
                                    >
                                    <Textarea
                                        id="description"
                                        v-model="banListForm.description"
                                        placeholder="Description of this ban list"
                                    />
                                </div>
                                <div class="flex items-center space-x-2">
                                    <Switch
                                        id="is_remote"
                                        v-model="banListForm.is_remote"
                                    />
                                    <Label htmlFor="is_remote"
                                        >Remote Ban List</Label
                                    >
                                </div>
                                <div v-if="banListForm.is_remote">
                                    <Label htmlFor="remote_url"
                                        >Remote URL</Label
                                    >
                                    <Input
                                        id="remote_url"
                                        v-model="banListForm.remote_url"
                                        placeholder="https://example.com/bans.cfg"
                                    />
                                </div>
                                <div
                                    v-if="banListForm.is_remote"
                                    class="flex items-center space-x-2"
                                >
                                    <Switch
                                        id="remote_sync_enabled"
                                        v-model="
                                            banListForm.remote_sync_enabled
                                        "
                                    />
                                    <Label htmlFor="remote_sync_enabled"
                                        >Enable Auto Sync</Label
                                    >
                                </div>
                            </div>
                            <DialogFooter>
                                <Button
                                    variant="outline"
                                    @click="showCreateDialog = false"
                                >
                                    Cancel
                                </Button>
                                <Button @click="createBanList">
                                    Create Ban List
                                </Button>
                            </DialogFooter>
                        </DialogContent>
                    </Dialog>
                </div>
            </CardHeader>
            <CardContent>
                <div
                    v-if="loading && banLists.length === 0"
                    class="text-center py-6 sm:py-8"
                >
                    <div
                        class="animate-spin h-8 w-8 border-4 border-primary border-t-transparent rounded-full mx-auto mb-4"
                    ></div>
                    <p class="text-sm sm:text-base">Loading ban lists...</p>
                </div>
                <div v-else-if="banLists.length === 0" class="text-center py-6 sm:py-8">
                    <p class="text-sm sm:text-base text-muted-foreground">
                        No ban lists found. Create your first ban list to get
                        started.
                    </p>
                </div>
                <template v-else>
                    <!-- Desktop Table View -->
                    <div class="hidden md:block w-full overflow-x-auto">
                        <Table>
                            <TableHeader>
                                <TableRow>
                                    <TableHead class="text-xs sm:text-sm">Name</TableHead>
                                    <TableHead class="text-xs sm:text-sm">Description</TableHead>
                                    <TableHead class="text-xs sm:text-sm">Type</TableHead>
                                    <TableHead class="text-xs sm:text-sm">Created</TableHead>
                                    <TableHead class="text-right text-xs sm:text-sm">Actions</TableHead>
                                </TableRow>
                            </TableHeader>
                            <TableBody>
                                <TableRow
                                    v-for="banList in banLists"
                                    :key="banList.id"
                                    class="hover:bg-muted/50"
                                >
                                    <TableCell class="font-medium text-sm sm:text-base">{{
                                        banList.name
                                    }}</TableCell>
                                    <TableCell class="text-xs sm:text-sm">{{
                                        banList.description || "No description"
                                    }}</TableCell>
                                    <TableCell>
                                        <Badge
                                            :variant="
                                                banList.is_remote
                                                    ? 'secondary'
                                                    : 'default'
                                            "
                                            class="text-xs"
                                        >
                                            {{
                                                banList.is_remote
                                                    ? "Remote"
                                                    : "Local"
                                            }}
                                        </Badge>
                                    </TableCell>
                                    <TableCell class="text-xs sm:text-sm">{{
                                        new Date(
                                            banList.created_at,
                                        ).toLocaleDateString()
                                    }}</TableCell>
                                    <TableCell class="text-right">
                                        <div class="flex gap-2 justify-end">
                                            <Button
                                                variant="ghost"
                                                size="sm"
                                                @click="viewCfg(banList.id)"
                                                title="View CFG"
                                                class="text-xs"
                                            >
                                                <Eye class="h-4 w-4" />
                                            </Button>
                                            <Button
                                                variant="ghost"
                                                size="sm"
                                                @click="copyCfgUrl(banList.id)"
                                                title="Copy CFG URL"
                                                class="text-xs"
                                            >
                                                <Link class="h-4 w-4" />
                                            </Button>
                                            <Button
                                                variant="outline"
                                                size="sm"
                                                @click="editBanList(banList)"
                                                class="text-xs"
                                            >
                                                <Edit class="h-4 w-4" />
                                            </Button>
                                            <Button
                                                variant="destructive"
                                                size="sm"
                                                @click="deleteBanList(banList)"
                                                class="text-xs"
                                            >
                                                <Trash2 class="h-4 w-4" />
                                            </Button>
                                        </div>
                                    </TableCell>
                                </TableRow>
                            </TableBody>
                        </Table>
                    </div>

                    <!-- Mobile Card View -->
                    <div class="md:hidden space-y-3">
                        <div
                            v-for="banList in banLists"
                            :key="banList.id"
                            class="border rounded-lg p-3 sm:p-4 hover:bg-muted/30 transition-colors"
                        >
                            <div class="flex items-start justify-between gap-2 mb-2">
                                <div class="flex-1 min-w-0">
                                    <div class="font-semibold text-sm sm:text-base mb-1">
                                        {{ banList.name }}
                                    </div>
                                    <div class="space-y-1.5">
                                        <div>
                                            <span class="text-xs text-muted-foreground">Description: </span>
                                            <span class="text-xs sm:text-sm">{{ banList.description || "No description" }}</span>
                                        </div>
                                        <div class="flex items-center gap-2 mt-2">
                                            <Badge
                                                :variant="
                                                    banList.is_remote
                                                        ? 'secondary'
                                                        : 'default'
                                                "
                                                class="text-xs"
                                            >
                                                {{
                                                    banList.is_remote
                                                        ? "Remote"
                                                        : "Local"
                                                }}
                                            </Badge>
                                            <span class="text-xs text-muted-foreground">
                                                Created: {{ new Date(banList.created_at).toLocaleDateString() }}
                                            </span>
                                        </div>
                                    </div>
                                </div>
                            </div>
                            <div class="flex items-center justify-end gap-2 pt-2 border-t">
                                <Button
                                    variant="ghost"
                                    size="sm"
                                    @click="viewCfg(banList.id)"
                                    class="h-8 text-xs"
                                >
                                    <Eye class="h-3 w-3" />
                                </Button>
                                <Button
                                    variant="ghost"
                                    size="sm"
                                    @click="copyCfgUrl(banList.id)"
                                    class="h-8 text-xs"
                                >
                                    <Link class="h-3 w-3" />
                                </Button>
                                <Button
                                    variant="outline"
                                    size="sm"
                                    @click="editBanList(banList)"
                                    class="h-8 text-xs"
                                >
                                    <Edit class="h-3 w-3" />
                                </Button>
                                <Button
                                    variant="destructive"
                                    size="sm"
                                    @click="deleteBanList(banList)"
                                    class="h-8 text-xs"
                                >
                                    <Trash2 class="h-3 w-3" />
                                </Button>
                            </div>
                        </div>
                    </div>
                </template>
            </CardContent>
        </Card>

        <!-- Remote Sources Section -->
        <Card>
            <CardHeader class="pb-2 sm:pb-3">
                <div class="flex flex-col sm:flex-row sm:justify-between sm:items-center gap-3 sm:gap-0">
                    <div>
                        <CardTitle class="flex items-center gap-2 text-base sm:text-lg">
                            <ExternalLink class="h-4 w-4 sm:h-5 sm:w-5" />
                            Remote Ban Sources
                        </CardTitle>
                        <CardDescription class="text-xs sm:text-sm">
                            Configure external ban list sources to automatically
                            sync bans from other communities.
                        </CardDescription>
                    </div>
                    <Dialog v-model:open="showRemoteSourceDialog">
                        <DialogTrigger asChild>
                            <Button class="w-full sm:w-auto text-sm sm:text-base">
                                <Plus class="h-4 w-4 mr-2" />
                                Add Remote Source
                            </Button>
                        </DialogTrigger>
                        <DialogContent class="w-[95vw] sm:max-w-[500px] max-h-[90vh] overflow-y-auto p-4 sm:p-6">
                            <DialogHeader>
                                <DialogTitle class="text-base sm:text-lg">Add Remote Ban Source</DialogTitle>
                                <DialogDescription class="text-xs sm:text-sm">
                                    Add a remote ban source to automatically
                                    sync bans from other communities.
                                </DialogDescription>
                            </DialogHeader>
                            <div class="grid gap-4 py-4">
                                <div>
                                    <Label htmlFor="remote_name">Name</Label>
                                    <Input
                                        id="remote_name"
                                        v-model="remoteSourceForm.name"
                                        placeholder="Community Ban List"
                                    />
                                </div>
                                <div>
                                    <Label htmlFor="remote_source_url"
                                        >URL</Label
                                    >
                                    <Input
                                        id="remote_source_url"
                                        v-model="remoteSourceForm.url"
                                        placeholder="https://example.com/bans.cfg"
                                    />
                                </div>
                                <div class="flex items-center space-x-2">
                                    <Switch
                                        id="sync_enabled"
                                        v-model="remoteSourceForm.sync_enabled"
                                    />
                                    <Label htmlFor="sync_enabled"
                                        >Enable Automatic Sync</Label
                                    >
                                </div>
                                <div>
                                    <Label htmlFor="sync_interval"
                                        >Sync Interval (minutes)</Label
                                    >
                                    <Input
                                        id="sync_interval"
                                        type="number"
                                        v-model="
                                            remoteSourceForm.sync_interval_minutes
                                        "
                                        min="60"
                                        max="1440"
                                        placeholder="180"
                                    />
                                </div>
                            </div>
                            <DialogFooter>
                                <Button
                                    variant="outline"
                                    @click="showRemoteSourceDialog = false"
                                >
                                    Cancel
                                </Button>
                                <Button @click="createRemoteSource">
                                    Add Remote Source
                                </Button>
                            </DialogFooter>
                        </DialogContent>
                    </Dialog>
                </div>
            </CardHeader>
            <CardContent>
                <div
                    v-if="loading && remoteSources.length === 0"
                    class="text-center py-6 sm:py-8"
                >
                    <div
                        class="animate-spin h-8 w-8 border-4 border-primary border-t-transparent rounded-full mx-auto mb-4"
                    ></div>
                    <p class="text-sm sm:text-base">Loading remote sources...</p>
                </div>
                <div
                    v-else-if="remoteSources.length === 0"
                    class="text-center py-6 sm:py-8"
                >
                    <p class="text-sm sm:text-base text-muted-foreground">
                        No remote sources configured. Add remote sources to sync
                        bans from other communities.
                    </p>
                </div>
                <template v-else>
                    <!-- Desktop Table View -->
                    <div class="hidden md:block w-full overflow-x-auto">
                        <Table>
                            <TableHeader>
                                <TableRow>
                                    <TableHead class="text-xs sm:text-sm">Name</TableHead>
                                    <TableHead class="text-xs sm:text-sm">URL</TableHead>
                                    <TableHead class="text-xs sm:text-sm">Sync Status</TableHead>
                                    <TableHead class="text-xs sm:text-sm">Interval</TableHead>
                                    <TableHead class="text-xs sm:text-sm">Last Sync</TableHead>
                                    <TableHead class="text-right text-xs sm:text-sm">Actions</TableHead>
                                </TableRow>
                            </TableHeader>
                            <TableBody>
                                <TableRow
                                    v-for="source in remoteSources"
                                    :key="source.id"
                                    class="hover:bg-muted/50"
                                >
                                    <TableCell class="font-medium text-sm sm:text-base">{{
                                        source.name
                                    }}</TableCell>
                                    <TableCell class="font-mono text-xs sm:text-sm break-all">{{
                                        source.url
                                    }}</TableCell>
                                    <TableCell>
                                        <Badge
                                            :variant="
                                                source.sync_enabled
                                                    ? 'default'
                                                    : 'secondary'
                                            "
                                            class="text-xs"
                                        >
                                            {{
                                                source.sync_enabled
                                                    ? "Enabled"
                                                    : "Disabled"
                                            }}
                                        </Badge>
                                    </TableCell>
                                    <TableCell class="text-xs sm:text-sm">{{
                                        source.sync_interval_minutes
                                    }}
                                    min</TableCell>
                                    <TableCell class="text-xs sm:text-sm">
                                        {{
                                            source.last_synced_at
                                                ? new Date(
                                                      source.last_synced_at,
                                                  ).toLocaleString()
                                                : "Never"
                                        }}
                                    </TableCell>
                                    <TableCell class="text-right">
                                        <Button
                                            variant="destructive"
                                            size="sm"
                                            @click="deleteRemoteSource(source)"
                                            class="text-xs"
                                        >
                                            <Trash2 class="h-4 w-4" />
                                        </Button>
                                    </TableCell>
                                </TableRow>
                            </TableBody>
                        </Table>
                    </div>

                    <!-- Mobile Card View -->
                    <div class="md:hidden space-y-3">
                        <div
                            v-for="source in remoteSources"
                            :key="source.id"
                            class="border rounded-lg p-3 sm:p-4 hover:bg-muted/30 transition-colors"
                        >
                            <div class="flex items-start justify-between gap-2 mb-2">
                                <div class="flex-1 min-w-0">
                                    <div class="font-semibold text-sm sm:text-base mb-1">
                                        {{ source.name }}
                                    </div>
                                    <div class="space-y-1.5">
                                        <div>
                                            <span class="text-xs text-muted-foreground">URL: </span>
                                            <span class="text-xs sm:text-sm break-all">{{ source.url }}</span>
                                        </div>
                                        <div class="flex items-center gap-2 mt-2">
                                            <Badge
                                                :variant="
                                                    source.sync_enabled
                                                        ? 'default'
                                                        : 'secondary'
                                                "
                                                class="text-xs"
                                            >
                                                {{
                                                    source.sync_enabled
                                                        ? "Enabled"
                                                        : "Disabled"
                                                }}
                                            </Badge>
                                            <span class="text-xs text-muted-foreground">
                                                {{ source.sync_interval_minutes }} min
                                            </span>
                                        </div>
                                        <div>
                                            <span class="text-xs text-muted-foreground">Last Sync: </span>
                                            <span class="text-xs sm:text-sm">
                                                {{ source.last_synced_at ? new Date(source.last_synced_at).toLocaleString() : "Never" }}
                                            </span>
                                        </div>
                                    </div>
                                </div>
                            </div>
                            <div class="flex items-center justify-end gap-2 pt-2 border-t">
                                <Button
                                    variant="destructive"
                                    size="sm"
                                    @click="deleteRemoteSource(source)"
                                    class="h-8 text-xs"
                                >
                                    <Trash2 class="h-3 w-3 mr-1" />
                                    Delete
                                </Button>
                            </div>
                        </div>
                    </div>
                </template>
            </CardContent>
        </Card>

        <!-- Ignored Steam IDs Section -->
        <Card>
            <CardHeader class="pb-2 sm:pb-3">
                <div class="flex flex-col sm:flex-row sm:justify-between sm:items-center gap-3 sm:gap-0">
                    <div>
                        <CardTitle class="flex items-center gap-2 text-base sm:text-lg">
                            <Users class="h-4 w-4 sm:h-5 sm:w-5" />
                            Ignored Steam IDs
                        </CardTitle>
                        <CardDescription class="text-xs sm:text-sm">
                            Manage Steam IDs that should be ignored when syncing
                            from remote ban sources.
                        </CardDescription>
                    </div>
                    <Dialog v-model:open="showIgnoredSteamIDDialog">
                        <DialogTrigger asChild>
                            <Button class="w-full sm:w-auto text-sm sm:text-base">
                                <Plus class="h-4 w-4 mr-2" />
                                Add Ignored Steam ID
                            </Button>
                        </DialogTrigger>
                        <DialogContent class="w-[95vw] sm:max-w-[500px] max-h-[90vh] overflow-y-auto p-4 sm:p-6">
                            <DialogHeader>
                                <DialogTitle class="text-base sm:text-lg">Add Ignored Steam ID</DialogTitle>
                                <DialogDescription class="text-xs sm:text-sm">
                                    Add a Steam ID to ignore when syncing bans
                                    from remote sources.
                                </DialogDescription>
                            </DialogHeader>
                            <div class="grid gap-4 py-4">
                                <div>
                                    <Label htmlFor="steam_id">Steam ID</Label>
                                    <Input
                                        id="steam_id"
                                        v-model="ignoredSteamIDForm.steam_id"
                                        placeholder="76561198012345678"
                                    />
                                </div>
                                <div>
                                    <Label htmlFor="reason">Reason</Label>
                                    <Textarea
                                        id="reason"
                                        v-model="ignoredSteamIDForm.reason"
                                        placeholder="Reason for ignoring this Steam ID"
                                    />
                                </div>
                            </div>
                            <DialogFooter>
                                <Button
                                    variant="outline"
                                    @click="showIgnoredSteamIDDialog = false"
                                >
                                    Cancel
                                </Button>
                                <Button @click="createIgnoredSteamID">
                                    Add to Ignore List
                                </Button>
                            </DialogFooter>
                        </DialogContent>
                    </Dialog>
                </div>
            </CardHeader>
            <CardContent>
                <div
                    v-if="loading && ignoredSteamIDs.length === 0"
                    class="text-center py-6 sm:py-8"
                >
                    <div
                        class="animate-spin h-8 w-8 border-4 border-primary border-t-transparent rounded-full mx-auto mb-4"
                    ></div>
                    <p class="text-sm sm:text-base">Loading ignored Steam IDs...</p>
                </div>
                <div
                    v-else-if="ignoredSteamIDs.length === 0"
                    class="text-center py-6 sm:py-8"
                >
                    <p class="text-sm sm:text-base text-muted-foreground">
                        No Steam IDs are currently ignored. Add Steam IDs to
                        prevent them from being banned via remote sources.
                    </p>
                </div>
                <template v-else>
                    <!-- Desktop Table View -->
                    <div class="hidden md:block w-full overflow-x-auto">
                        <Table>
                            <TableHeader>
                                <TableRow>
                                    <TableHead class="text-xs sm:text-sm">Steam ID</TableHead>
                                    <TableHead class="text-xs sm:text-sm">Reason</TableHead>
                                    <TableHead class="text-xs sm:text-sm">Added</TableHead>
                                    <TableHead class="text-right text-xs sm:text-sm">Actions</TableHead>
                                </TableRow>
                            </TableHeader>
                            <TableBody>
                                <TableRow
                                    v-for="ignoredID in ignoredSteamIDs"
                                    :key="ignoredID.id"
                                    class="hover:bg-muted/50"
                                >
                                    <TableCell class="font-mono text-xs sm:text-sm">{{
                                        ignoredID.steam_id
                                    }}</TableCell>
                                    <TableCell class="text-xs sm:text-sm">{{
                                        ignoredID.reason || "No reason provided"
                                    }}</TableCell>
                                    <TableCell class="text-xs sm:text-sm">{{
                                        new Date(
                                            ignoredID.created_at,
                                        ).toLocaleDateString()
                                    }}</TableCell>
                                    <TableCell class="text-right">
                                        <Button
                                            variant="destructive"
                                            size="sm"
                                            @click="deleteIgnoredSteamID(ignoredID)"
                                            class="text-xs"
                                        >
                                            <Trash2 class="h-4 w-4" />
                                        </Button>
                                    </TableCell>
                                </TableRow>
                            </TableBody>
                        </Table>
                    </div>

                    <!-- Mobile Card View -->
                    <div class="md:hidden space-y-3">
                        <div
                            v-for="ignoredID in ignoredSteamIDs"
                            :key="ignoredID.id"
                            class="border rounded-lg p-3 sm:p-4 hover:bg-muted/30 transition-colors"
                        >
                            <div class="flex items-start justify-between gap-2 mb-2">
                                <div class="flex-1 min-w-0">
                                    <div class="font-mono text-sm sm:text-base mb-1">
                                        {{ ignoredID.steam_id }}
                                    </div>
                                    <div class="space-y-1.5">
                                        <div>
                                            <span class="text-xs text-muted-foreground">Reason: </span>
                                            <span class="text-xs sm:text-sm">{{ ignoredID.reason || "No reason provided" }}</span>
                                        </div>
                                        <div>
                                            <span class="text-xs text-muted-foreground">Added: </span>
                                            <span class="text-xs sm:text-sm">{{ new Date(ignoredID.created_at).toLocaleDateString() }}</span>
                                        </div>
                                    </div>
                                </div>
                            </div>
                            <div class="flex items-center justify-end gap-2 pt-2 border-t">
                                <Button
                                    variant="destructive"
                                    size="sm"
                                    @click="deleteIgnoredSteamID(ignoredID)"
                                    class="h-8 text-xs"
                                >
                                    <Trash2 class="h-3 w-3 mr-1" />
                                    Delete
                                </Button>
                            </div>
                        </div>
                    </div>
                </template>
            </CardContent>
        </Card>

        <!-- Edit Ban List Dialog -->
        <Dialog v-model:open="showEditDialog">
            <DialogContent class="w-[95vw] sm:max-w-[500px] max-h-[90vh] overflow-y-auto p-4 sm:p-6">
                <DialogHeader>
                    <DialogTitle class="text-base sm:text-lg">Edit Ban List</DialogTitle>
                    <DialogDescription class="text-xs sm:text-sm">
                        Update the ban list details.
                    </DialogDescription>
                </DialogHeader>
                <div class="grid gap-4 py-4">
                    <div>
                        <Label htmlFor="edit_name">Name</Label>
                        <Input
                            id="edit_name"
                            v-model="banListForm.name"
                            placeholder="Ban List Name"
                        />
                    </div>
                    <div>
                        <Label htmlFor="edit_description">Description</Label>
                        <Textarea
                            id="edit_description"
                            v-model="banListForm.description"
                            placeholder="Description of this ban list"
                        />
                    </div>
                    <div class="flex items-center space-x-2">
                        <Switch
                            id="edit_is_remote"
                            v-model="banListForm.is_remote"
                        />
                        <Label htmlFor="edit_is_remote">Remote Ban List</Label>
                    </div>
                    <div v-if="banListForm.is_remote">
                        <Label htmlFor="edit_remote_url">Remote URL</Label>
                        <Input
                            id="edit_remote_url"
                            v-model="banListForm.remote_url"
                            placeholder="https://example.com/bans.cfg"
                        />
                    </div>
                    <div
                        v-if="banListForm.is_remote"
                        class="flex items-center space-x-2"
                    >
                        <Switch
                            id="edit_remote_sync_enabled"
                            v-model="banListForm.remote_sync_enabled"
                        />
                        <Label htmlFor="edit_remote_sync_enabled"
                            >Enable Auto Sync</Label
                        >
                    </div>
                </div>
                <DialogFooter>
                    <Button variant="outline" @click="showEditDialog = false">
                        Cancel
                    </Button>
                    <Button @click="updateBanList"> Update Ban List </Button>
                </DialogFooter>
            </DialogContent>
        </Dialog>
    </div>
</template>
