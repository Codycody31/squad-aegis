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
    <div class="p-6 space-y-6">
        <!-- Header -->
        <div class="flex justify-between items-center">
            <div>
                <h1 class="text-3xl font-bold">Ban Lists Management</h1>
                <p class="text-gray-600 mt-2">
                    Manage ban lists, remote sources, and ignored Steam IDs for
                    shared ban coordination.
                </p>
            </div>
            <Button @click="loadData" :disabled="loading">
                <Download class="h-4 w-4 mr-2" />
                {{ loading ? "Loading..." : "Refresh" }}
            </Button>
        </div>

        <!-- Ban Lists Section -->
        <Card>
            <CardHeader>
                <div class="flex justify-between items-center">
                    <div>
                        <CardTitle class="flex items-center gap-2">
                            <Shield class="h-5 w-5" />
                            Ban Lists
                        </CardTitle>
                        <CardDescription>
                            Manage local and remote ban lists that can be shared
                            across servers.
                        </CardDescription>
                    </div>
                    <Dialog v-model:open="showCreateDialog">
                        <DialogTrigger asChild>
                            <Button>
                                <Plus class="h-4 w-4 mr-2" />
                                Create Ban List
                            </Button>
                        </DialogTrigger>
                        <DialogContent>
                            <DialogHeader>
                                <DialogTitle>Create New Ban List</DialogTitle>
                                <DialogDescription>
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
                    class="text-center py-8"
                >
                    <div
                        class="animate-spin h-8 w-8 border-4 border-primary border-t-transparent rounded-full mx-auto mb-4"
                    ></div>
                    <p>Loading ban lists...</p>
                </div>
                <div v-else-if="banLists.length === 0" class="text-center py-8">
                    <p class="text-gray-500">
                        No ban lists found. Create your first ban list to get
                        started.
                    </p>
                </div>
                <div v-else>
                    <Table>
                        <TableHeader>
                            <TableRow>
                                <TableHead>Name</TableHead>
                                <TableHead>Description</TableHead>
                                <TableHead>Type</TableHead>
                                <TableHead>Created</TableHead>
                                <TableHead class="text-right"
                                    >Actions</TableHead
                                >
                            </TableRow>
                        </TableHeader>
                        <TableBody>
                            <TableRow
                                v-for="banList in banLists"
                                :key="banList.id"
                            >
                                <TableCell class="font-medium">{{
                                    banList.name
                                }}</TableCell>
                                <TableCell>{{
                                    banList.description || "No description"
                                }}</TableCell>
                                <TableCell>
                                    <Badge
                                        :variant="
                                            banList.is_remote
                                                ? 'secondary'
                                                : 'default'
                                        "
                                    >
                                        {{
                                            banList.is_remote
                                                ? "Remote"
                                                : "Local"
                                        }}
                                    </Badge>
                                </TableCell>
                                <TableCell>{{
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
                                        >
                                            <Eye class="h-4 w-4" />
                                        </Button>
                                        <Button
                                            variant="ghost"
                                            size="sm"
                                            @click="copyCfgUrl(banList.id)"
                                            title="Copy CFG URL"
                                        >
                                            <Link class="h-4 w-4" />
                                        </Button>
                                        <Button
                                            variant="outline"
                                            size="sm"
                                            @click="editBanList(banList)"
                                        >
                                            <Edit class="h-4 w-4" />
                                        </Button>
                                        <Button
                                            variant="destructive"
                                            size="sm"
                                            @click="deleteBanList(banList)"
                                        >
                                            <Trash2 class="h-4 w-4" />
                                        </Button>
                                    </div>
                                </TableCell>
                            </TableRow>
                        </TableBody>
                    </Table>
                </div>
            </CardContent>
        </Card>

        <!-- Remote Sources Section -->
        <Card>
            <CardHeader>
                <div class="flex justify-between items-center">
                    <div>
                        <CardTitle class="flex items-center gap-2">
                            <ExternalLink class="h-5 w-5" />
                            Remote Ban Sources
                        </CardTitle>
                        <CardDescription>
                            Configure external ban list sources to automatically
                            sync bans from other communities.
                        </CardDescription>
                    </div>
                    <Dialog v-model:open="showRemoteSourceDialog">
                        <DialogTrigger asChild>
                            <Button>
                                <Plus class="h-4 w-4 mr-2" />
                                Add Remote Source
                            </Button>
                        </DialogTrigger>
                        <DialogContent>
                            <DialogHeader>
                                <DialogTitle>Add Remote Ban Source</DialogTitle>
                                <DialogDescription>
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
                    class="text-center py-8"
                >
                    <div
                        class="animate-spin h-8 w-8 border-4 border-primary border-t-transparent rounded-full mx-auto mb-4"
                    ></div>
                    <p>Loading remote sources...</p>
                </div>
                <div
                    v-else-if="remoteSources.length === 0"
                    class="text-center py-8"
                >
                    <p class="text-gray-500">
                        No remote sources configured. Add remote sources to sync
                        bans from other communities.
                    </p>
                </div>
                <div v-else>
                    <Table>
                        <TableHeader>
                            <TableRow>
                                <TableHead>Name</TableHead>
                                <TableHead>URL</TableHead>
                                <TableHead>Sync Status</TableHead>
                                <TableHead>Interval</TableHead>
                                <TableHead>Last Sync</TableHead>
                                <TableHead class="text-right"
                                    >Actions</TableHead
                                >
                            </TableRow>
                        </TableHeader>
                        <TableBody>
                            <TableRow
                                v-for="source in remoteSources"
                                :key="source.id"
                            >
                                <TableCell class="font-medium">{{
                                    source.name
                                }}</TableCell>
                                <TableCell class="font-mono text-sm">{{
                                    source.url
                                }}</TableCell>
                                <TableCell>
                                    <Badge
                                        :variant="
                                            source.sync_enabled
                                                ? 'default'
                                                : 'secondary'
                                        "
                                    >
                                        {{
                                            source.sync_enabled
                                                ? "Enabled"
                                                : "Disabled"
                                        }}
                                    </Badge>
                                </TableCell>
                                <TableCell
                                    >{{
                                        source.sync_interval_minutes
                                    }}
                                    min</TableCell
                                >
                                <TableCell>
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
                                    >
                                        <Trash2 class="h-4 w-4" />
                                    </Button>
                                </TableCell>
                            </TableRow>
                        </TableBody>
                    </Table>
                </div>
            </CardContent>
        </Card>

        <!-- Ignored Steam IDs Section -->
        <Card>
            <CardHeader>
                <div class="flex justify-between items-center">
                    <div>
                        <CardTitle class="flex items-center gap-2">
                            <Users class="h-5 w-5" />
                            Ignored Steam IDs
                        </CardTitle>
                        <CardDescription>
                            Manage Steam IDs that should be ignored when syncing
                            from remote ban sources.
                        </CardDescription>
                    </div>
                    <Dialog v-model:open="showIgnoredSteamIDDialog">
                        <DialogTrigger asChild>
                            <Button>
                                <Plus class="h-4 w-4 mr-2" />
                                Add Ignored Steam ID
                            </Button>
                        </DialogTrigger>
                        <DialogContent>
                            <DialogHeader>
                                <DialogTitle>Add Ignored Steam ID</DialogTitle>
                                <DialogDescription>
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
                    class="text-center py-8"
                >
                    <div
                        class="animate-spin h-8 w-8 border-4 border-primary border-t-transparent rounded-full mx-auto mb-4"
                    ></div>
                    <p>Loading ignored Steam IDs...</p>
                </div>
                <div
                    v-else-if="ignoredSteamIDs.length === 0"
                    class="text-center py-8"
                >
                    <p class="text-gray-500">
                        No Steam IDs are currently ignored. Add Steam IDs to
                        prevent them from being banned via remote sources.
                    </p>
                </div>
                <div v-else>
                    <Table>
                        <TableHeader>
                            <TableRow>
                                <TableHead>Steam ID</TableHead>
                                <TableHead>Reason</TableHead>
                                <TableHead>Added</TableHead>
                                <TableHead class="text-right"
                                    >Actions</TableHead
                                >
                            </TableRow>
                        </TableHeader>
                        <TableBody>
                            <TableRow
                                v-for="ignoredID in ignoredSteamIDs"
                                :key="ignoredID.id"
                            >
                                <TableCell class="font-mono">{{
                                    ignoredID.steam_id
                                }}</TableCell>
                                <TableCell>{{
                                    ignoredID.reason || "No reason provided"
                                }}</TableCell>
                                <TableCell>{{
                                    new Date(
                                        ignoredID.created_at,
                                    ).toLocaleDateString()
                                }}</TableCell>
                                <TableCell class="text-right">
                                    <Button
                                        variant="destructive"
                                        size="sm"
                                        @click="deleteIgnoredSteamID(ignoredID)"
                                    >
                                        <Trash2 class="h-4 w-4" />
                                    </Button>
                                </TableCell>
                            </TableRow>
                        </TableBody>
                    </Table>
                </div>
            </CardContent>
        </Card>

        <!-- Edit Ban List Dialog -->
        <Dialog v-model:open="showEditDialog">
            <DialogContent>
                <DialogHeader>
                    <DialogTitle>Edit Ban List</DialogTitle>
                    <DialogDescription>
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
