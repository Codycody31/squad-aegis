<template>
    <div class="p-4">
        <div class="flex justify-between items-center mb-4">
            <h1 class="text-2xl font-bold">Server Settings</h1>
            <p class="text-sm text-muted-foreground">
                Manage your server configuration and settings
            </p>
        </div>

        <!-- Server Status Card -->
        <Card class="mb-4">
            <CardHeader>
                <CardTitle>Server Status</CardTitle>
                <p class="text-sm text-muted-foreground">
                    Current status of your server and its connections
                </p>
            </CardHeader>
            <CardContent>
                <div class="grid grid-cols-2 gap-4">
                    <div class="flex items-center space-x-2">
                        <div
                            :class="[
                                'w-3 h-3 rounded-full',
                                serverStatus?.rcon
                                    ? 'bg-green-500'
                                    : 'bg-red-500',
                            ]"
                        ></div>
                        <span class="text-sm">RCON Connection</span>
                    </div>
                    <div class="flex items-center space-x-2">
                        <div class="w-3 h-3 rounded-full bg-blue-500"></div>
                        <span class="text-sm">Server Online</span>
                    </div>
                </div>
            </CardContent>
        </Card>

        <!-- Server Details Form -->
        <Card class="mb-4">
            <CardHeader>
                <CardTitle>Server Details</CardTitle>
                <p class="text-sm text-muted-foreground">
                    Update your server configuration and connection settings
                </p>
            </CardHeader>
            <CardContent>
                <form @submit.prevent="updateServer" class="space-y-4">
                    <div class="grid gap-4">
                        <div class="grid grid-cols-4 items-center gap-4">
                            <label for="name" class="text-right"
                                >Server Name</label
                            >
                            <Input
                                id="name"
                                v-model="serverForm.name"
                                class="col-span-3"
                                required
                                placeholder="My Squad Server"
                            />
                        </div>

                        <div class="grid grid-cols-4 items-center gap-4">
                            <label for="ip_address" class="text-right"
                                >IP Address</label
                            >
                            <Input
                                id="ip_address"
                                v-model="serverForm.ip_address"
                                class="col-span-3"
                                required
                                placeholder="e.g., 192.168.1.1"
                            />
                        </div>

                        <div class="grid grid-cols-4 items-center gap-4">
                            <label for="game_port" class="text-right"
                                >Game Port</label
                            >
                            <Input
                                id="game_port"
                                v-model="serverForm.game_port"
                                type="number"
                                class="col-span-3"
                                required
                                placeholder="Default: 2302"
                            />
                        </div>

                        <div class="grid grid-cols-4 items-center gap-4">
                            <label for="rcon_ip_address" class="text-right"
                                >RCON IP Address</label
                            >
                            <Input
                                id="rcon_ip_address"
                                v-model="serverForm.rcon_ip_address"
                                class="col-span-3"
                                placeholder="Leave blank to use server IP"
                            />
                        </div>

                        <div class="grid grid-cols-4 items-center gap-4">
                            <label for="rcon_port" class="text-right"
                                >RCON Port</label
                            >
                            <Input
                                id="rcon_port"
                                v-model="serverForm.rcon_port"
                                type="number"
                                class="col-span-3"
                                required
                                placeholder="Default: 2302"
                            />
                        </div>

                        <div class="grid grid-cols-4 items-center gap-4">
                            <label for="rcon_password" class="text-right"
                                >RCON Password</label
                            >
                            <Input
                                id="rcon_password"
                                v-model="serverForm.rcon_password"
                                type="password"
                                class="col-span-3"
                                placeholder="••••••••"
                            />
                        </div>

                        <!-- Log Configuration Section -->
                        <div class="border-t pt-4 mt-4">
                            <div class="grid grid-cols-4 items-center gap-4 mb-4">
                                <div class="text-right">
                                    <h4 class="text-sm font-medium">Log Configuration</h4>
                                    <p class="text-xs text-muted-foreground mt-1">
                                        Optional log monitoring setup
                                    </p>
                                </div>
                                <div class="col-span-3">
                                    <p class="text-xs text-muted-foreground">
                                        Configure log monitoring to enable advanced event tracking and analytics.
                                    </p>
                                </div>
                            </div>

                            <div class="grid grid-cols-4 items-center gap-4 pt-4">
                                <label for="log_source_type" class="text-right"
                                    >Log Source Type</label
                                >
                                <Select v-model="selectedLogSourceType" @update:modelValue="(value) => { serverForm.log_source_type = value; selectedLogSourceType = value; }">
                                    <SelectTrigger class="col-span-3">
                                        <SelectValue placeholder="Select log source type" />
                                    </SelectTrigger>
                                    <SelectContent>
                                        <SelectItem value="local">Local File</SelectItem>
                                        <SelectItem value="sftp">SFTP</SelectItem>
                                        <SelectItem value="ftp">FTP</SelectItem>
                                    </SelectContent>
                                </Select>
                            </div>

                            <div v-if="selectedLogSourceType" class="grid grid-cols-4 items-center gap-4 pt-4">
                                <label for="log_file_path" class="text-right"
                                    >Log File Path</label
                                >
                                <Input
                                    id="log_file_path"
                                    v-model="serverForm.log_file_path"
                                    class="col-span-3"
                                    :placeholder="selectedLogSourceType === 'local' 
                                        ? '/path/to/SquadGame.log' 
                                        : '/remote/path/to/SquadGame.log'"
                                />
                            </div>

                            <!-- Remote connection fields for SFTP/FTP -->
                            <template v-if="selectedLogSourceType === 'sftp' || selectedLogSourceType === 'ftp'">
                                <div class="grid grid-cols-4 items-center gap-4 pt-4">
                                    <label for="log_host" class="text-right"
                                        >{{ selectedLogSourceType?.toUpperCase() }} Host</label
                                    >
                                    <Input
                                        id="log_host"
                                        v-model="serverForm.log_host"
                                        class="col-span-3"
                                        placeholder="192.168.1.100"
                                    />
                                </div>

                                <div class="grid grid-cols-4 items-center gap-4 pt-4">
                                    <label for="log_port" class="text-right"
                                        >{{ selectedLogSourceType?.toUpperCase() }} Port</label
                                    >
                                    <Input
                                        id="log_port"
                                        v-model="serverForm.log_port"
                                        type="number"
                                        class="col-span-3"
                                        :placeholder="selectedLogSourceType === 'sftp' ? '22' : '21'"
                                    />
                                </div>

                                <div class="grid grid-cols-4 items-center gap-4 pt-4">
                                    <label for="log_username" class="text-right"
                                        >Username</label
                                    >
                                    <Input
                                        id="log_username"
                                        v-model="serverForm.log_username"
                                        class="col-span-3"
                                        placeholder="username"
                                    />
                                </div>

                                <div class="grid grid-cols-4 items-center gap-4 pt-4">
                                    <label for="log_password" class="text-right"
                                        >Password</label
                                    >
                                    <Input
                                        id="log_password"
                                        v-model="serverForm.log_password"
                                        type="password"
                                        class="col-span-3"
                                        placeholder="••••••••"
                                    />
                                </div>

                                <div class="grid grid-cols-4 items-center gap-4 pt-4">
                                    <label for="log_poll_frequency" class="text-right"
                                        >Poll Frequency (sec)</label
                                    >
                                    <Input
                                        id="log_poll_frequency"
                                        v-model="serverForm.log_poll_frequency"
                                        type="number"
                                        class="col-span-3"
                                        placeholder="5"
                                        min="1"
                                        max="300"
                                    />
                                </div>
                            </template>

                            <div v-if="selectedLogSourceType" class="grid grid-cols-4 items-center gap-4 pt-4">
                                <label for="log_read_from_start" class="text-right"
                                    >Read from start</label
                                >
                                <div class="col-span-3 flex items-center space-x-2">
                                    <Checkbox
                                        id="log_read_from_start"
                                        v-model:checked="serverForm.log_read_from_start"
                                    />
                                    <label for="log_read_from_start" class="text-sm text-muted-foreground">
                                        Process entire log file from beginning
                                    </label>
                                </div>
                            </div>
                        </div>

                        <!-- Ban Enforcement Section -->
                        <div class="border-t pt-4 mt-4">
                            <div class="grid grid-cols-4 items-center gap-4 mb-4">
                                <div class="text-right">
                                    <h4 class="text-sm font-medium">Ban Enforcement</h4>
                                    <p class="text-xs text-muted-foreground mt-1">
                                        How bans are enforced
                                    </p>
                                </div>
                                <div class="col-span-3">
                                    <p class="text-xs text-muted-foreground">
                                        Choose whether the game server or Squad Aegis handles ban enforcement.
                                    </p>
                                </div>
                            </div>

                            <div class="grid grid-cols-4 items-center gap-4">
                                <label for="ban_enforcement_mode" class="text-right"
                                    >Enforcement Mode</label
                                >
                                <Select v-model="serverForm.ban_enforcement_mode">
                                    <SelectTrigger class="col-span-3">
                                        <SelectValue placeholder="Select enforcement mode" />
                                    </SelectTrigger>
                                    <SelectContent>
                                        <SelectItem value="server">Server (AdminBan via RCON)</SelectItem>
                                        <SelectItem value="aegis">Squad Aegis (watch connections + kick)</SelectItem>
                                    </SelectContent>
                                </Select>
                            </div>

                            <div class="grid grid-cols-4 items-center gap-4 mt-2">
                                <div></div>
                                <p class="col-span-3 text-xs text-muted-foreground">
                                    <strong>Server mode:</strong> Bans are sent via AdminBan RCON command. The game server enforces them via its remote ban list.<br />
                                    <strong>Aegis mode:</strong> Squad Aegis monitors player connections and automatically kicks banned players when they join.
                                </p>
                            </div>
                        </div>
                    </div>

                    <div class="flex justify-end">
                        <Button type="submit" :disabled="isUpdating">
                            <span v-if="isUpdating" class="mr-2">
                                <Icon
                                    name="lucide:loader-2"
                                    class="h-4 w-4 animate-spin"
                                />
                            </span>
                            Update Server
                        </Button>
                    </div>
                </form>
            </CardContent>
        </Card>

        <!-- RCON Management -->
        <Card class="mb-4">
            <CardHeader>
                <CardTitle>RCON Management</CardTitle>
                <p class="text-sm text-muted-foreground">
                    Manage the RCON connection to your server
                </p>
            </CardHeader>
            <CardContent>
                <div class="flex justify-between items-center">
                    <p class="text-sm text-muted-foreground">
                        Restart the RCON connection if you're experiencing
                        connection issues
                    </p>
                    <Button
                        variant="outline"
                        @click="restartRcon"
                        :disabled="isRestarting"
                    >
                        <span v-if="isRestarting" class="mr-2">
                            <Icon
                                name="lucide:loader-2"
                                class="h-4 w-4 animate-spin"
                            />
                        </span>
                        Restart RCON Connection
                    </Button>
                </div>
            </CardContent>
        </Card>

        <!-- Log Watcher Management -->
        <Card class="mb-4" v-if="serverForm.log_source_type">
            <CardHeader>
                <CardTitle>Log Watcher Management</CardTitle>
                <p class="text-sm text-muted-foreground">
                    Manage the log monitoring connection to your server
                </p>
            </CardHeader>
            <CardContent>
                <div class="grid grid-cols-2 gap-4 mb-4">
                    <div class="flex items-center space-x-2">
                        <div class="w-3 h-3 rounded-full bg-blue-500"></div>
                        <span class="text-sm">Log Source: {{ serverForm.log_source_type?.toUpperCase() }}</span>
                    </div>
                    <div class="flex items-center space-x-2">
                        <div class="w-3 h-3 rounded-full bg-green-500"></div>
                        <span class="text-sm">Monitoring Active</span>
                    </div>
                </div>
                <div class="flex justify-between items-center">
                    <p class="text-sm text-muted-foreground">
                        Restart the log watcher if you're experiencing
                        connection issues or updated log configuration
                    </p>
                    <Button
                        variant="outline"
                        @click="restartLogWatcher"
                        :disabled="isRestarting"
                    >
                        <span v-if="isRestarting" class="mr-2">
                            <Icon
                                name="lucide:loader-2"
                                class="h-4 w-4 animate-spin"
                            />
                        </span>
                        Restart Log Watcher
                    </Button>
                </div>
            </CardContent>
        </Card>

        <!-- Danger Zone -->
        <Card class="border-2 border-destructive">
            <CardHeader>
                <CardTitle class="text-destructive">Danger Zone</CardTitle>
                <p class="text-sm text-muted-foreground">
                    Once you delete a server, there is no going back. Please be
                    certain.
                </p>
            </CardHeader>
            <CardContent>
                <div class="flex justify-between items-center">
                    <p class="text-sm text-muted-foreground">
                        This will permanently delete the server and all
                        associated data
                    </p>
                    <Button
                        variant="destructive"
                        @click="confirmDelete"
                        :disabled="isDeleting"
                    >
                        <span v-if="isDeleting" class="mr-2">
                            <Icon
                                name="lucide:loader-2"
                                class="h-4 w-4 animate-spin"
                            />
                        </span>
                        Delete Server
                    </Button>
                </div>
            </CardContent>
        </Card>

        <!-- Delete Confirmation Dialog -->
        <Dialog v-model:open="showDeleteDialog">
            <DialogContent>
                <DialogHeader>
                    <DialogTitle>Delete Server</DialogTitle>
                    <DialogDescription>
                        Are you sure you want to delete this server? This action
                        cannot be undone.
                    </DialogDescription>
                </DialogHeader>
                <DialogFooter>
                    <Button variant="outline" @click="showDeleteDialog = false"
                        >Cancel</Button
                    >
                    <Button
                        variant="destructive"
                        @click="deleteServer"
                        :disabled="isDeleting"
                    >
                        <span v-if="isDeleting" class="mr-2">
                            <Icon
                                name="lucide:loader-2"
                                class="h-4 w-4 animate-spin"
                            />
                        </span>
                        Delete Server
                    </Button>
                </DialogFooter>
            </DialogContent>
        </Dialog>
    </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from "vue";
import { useRoute, useRouter } from "vue-router";
import { useToast } from "~/components/ui/toast";
import { Button } from "~/components/ui/button";
import { Input } from "~/components/ui/input";
import { Card, CardContent, CardHeader, CardTitle } from "~/components/ui/card";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "~/components/ui/select";
import { Checkbox } from "~/components/ui/checkbox";
import {
    Dialog,
    DialogContent,
    DialogDescription,
    DialogFooter,
    DialogHeader,
    DialogTitle,
} from "~/components/ui/dialog";

definePageMeta({ middleware: ["auth"] });

const route = useRoute();
const router = useRouter();
const { toast } = useToast();

const runtimeConfig = useRuntimeConfig();
const cookieToken = useCookie(runtimeConfig.public.sessionCookieName as string);
const token = cookieToken.value;

const serverId = route.params.serverId;
const serverStatus = ref<any>(null);
const serverForm = ref({
    name: "",
    ip_address: "",
    game_port: "",
    rcon_ip_address: "",
    rcon_port: "",
    rcon_password: "",

    // Log configuration fields
    log_source_type: "",
    log_file_path: "",
    log_host: "",
    log_port: null,
    log_username: "",
    log_password: "",
    log_poll_frequency: 5,
    log_read_from_start: false,

    // Ban enforcement
    ban_enforcement_mode: "server" as string,
});

const isUpdating = ref(false);
const isRestarting = ref(false);
const isDeleting = ref(false);
const showDeleteDialog = ref(false);

// Track selected log source type for conditional fields
const selectedLogSourceType = ref<string>("");

// Fetch server details
const fetchServerDetails = async () => {
    try {
        const response = await fetch(`/api/servers/${serverId}`, {
            headers: {
                Authorization: `Bearer ${token}`,
            },
        });
        const data = await response.json();

        if (data.code === 200) {
            serverForm.value = {
                name: data.data.server.name,
                ip_address: data.data.server.ip_address,
                rcon_ip_address: data.data.server.rcon_ip_address || "",
                game_port: data.data.server.game_port,
                rcon_port: data.data.server.rcon_port,
                rcon_password: data.data.server.rcon_password,

                // Log configuration fields
                log_source_type: data.data.server.log_source_type || "",
                log_file_path: data.data.server.log_file_path || "",
                log_host: data.data.server.log_host || "",
                log_port: data.data.server.log_port,
                log_username: data.data.server.log_username || "",
                log_password: data.data.server.log_password || "",
                log_poll_frequency: data.data.server.log_poll_frequency || 5,
                log_read_from_start: data.data.server.log_read_from_start || false,

                // Ban enforcement
                ban_enforcement_mode: data.data.server.ban_enforcement_mode || "server",
            };
            
            // Update selected log source type for conditional rendering
            selectedLogSourceType.value = data.data.server.log_source_type || "";
        }
    } catch (error) {
        toast({
            title: "Error",
            description: "Failed to fetch server details",
            variant: "destructive",
        });
    }
};

// fetch server status
const fetchServerStatus = async () => {
    const response = await fetch(`/api/servers/${serverId}/status`, {
        headers: {
            Authorization: `Bearer ${token}`,
        },
    });
    const data = await response.json();
    if (data.code === 200) {
        serverStatus.value = data.data.status;
    }
};

// Update server
const updateServer = async () => {
    isUpdating.value = true;
    try {
        const response = await fetch(`/api/servers/${serverId}`, {
            method: "PUT",
            headers: {
                "Content-Type": "application/json",
                Authorization: `Bearer ${token}`,
            },
            body: JSON.stringify(serverForm.value),
        });

        const data = await response.json();
        if (data.code === 200) {
            toast({
                title: "Success",
                description: "Server updated successfully",
                variant: "default",
            });
            fetchServerDetails();
        } else {
            toast({
                title: "Error",
                description: data.message || "Failed to update server",
                variant: "destructive",
            });
        }
    } catch (error) {
        toast({
            title: "Error",
            description: "Failed to update server",
            variant: "destructive",
        });
    } finally {
        isUpdating.value = false;
    }
};

// Restart RCON connection
const restartRcon = async () => {
    isRestarting.value = true;
    try {
        const response = await fetch(
            `/api/servers/${serverId}/rcon/force-restart`,
            {
                method: "POST",
                headers: {
                    "Content-Type": "application/json",
                    Authorization: `Bearer ${token}`,
                },
            },
        );

        const data = await response.json();
        if (data.code === 200) {
            toast({
                title: "Success",
                description: "RCON connection restarted successfully",
                variant: "default",
            });
            fetchServerDetails();
        } else {
            toast({
                title: "Error",
                description:
                    data.message || "Failed to restart RCON connection",
                variant: "destructive",
            });
        }
    } catch (error) {
        toast({
            title: "Error",
            description: "Failed to restart RCON connection",
            variant: "destructive",
        });
    } finally {
        isRestarting.value = false;
    }
};

// Restart Log Watcher connection
const restartLogWatcher = async () => {
    isRestarting.value = true;
    try {
        const response = await fetch(
            `/api/servers/${serverId}/logwatcher/restart`,
            {
                method: "POST",
                headers: {
                    "Content-Type": "application/json",
                    Authorization: `Bearer ${token}`,
                },
            },
        );

        const data = await response.json();
        if (data.code === 200) {
            toast({
                title: "Success",
                description: "Log watcher restarted successfully",
                variant: "default",
            });
            fetchServerDetails();
        } else {
            toast({
                title: "Error",
                description:
                    data.message || "Failed to restart log watcher",
                variant: "destructive",
            });
        }
    } catch (error) {
        toast({
            title: "Error",
            description: "Failed to restart log watcher",
            variant: "destructive",
        });
    } finally {
        isRestarting.value = false;
    }
};

// Confirm delete
const confirmDelete = () => {
    showDeleteDialog.value = true;
};

// Delete server
const deleteServer = async () => {
    isDeleting.value = true;
    try {
        const response = await fetch(`/api/servers/${serverId}`, {
            method: "DELETE",
            headers: {
                Authorization: `Bearer ${token}`,
            },
        });

        const data = await response.json();
        if (data.code === 200) {
            toast({
                title: "Success",
                description: "Server deleted successfully",
                variant: "default",
            });
            router.push("/servers");
        } else {
            toast({
                title: "Error",
                description: data.message || "Failed to delete server",
                variant: "destructive",
            });
        }
    } catch (error) {
        toast({
            title: "Error",
            description: "Failed to delete server",
            variant: "destructive",
        });
    } finally {
        isDeleting.value = false;
        showDeleteDialog.value = false;
    }
};

onMounted(() => {
    fetchServerDetails();
    fetchServerStatus();
});
</script>
