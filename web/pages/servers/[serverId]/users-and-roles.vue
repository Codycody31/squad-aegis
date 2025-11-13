<script setup lang="ts">
import { ref, onMounted, computed, watch } from "vue";
import { Button } from "~/components/ui/button";
import { Input } from "~/components/ui/input";
import { Card, CardContent, CardHeader, CardTitle } from "~/components/ui/card";
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
import {
    Form,
    FormControl,
    FormDescription,
    FormField,
    FormItem,
    FormLabel,
    FormMessage,
} from "~/components/ui/form";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "~/components/ui/tabs";
import { Checkbox } from "~/components/ui/checkbox";
import {
    Select,
    SelectContent,
    SelectItem,
    SelectTrigger,
    SelectValue,
} from "~/components/ui/select";
import { Textarea } from "~/components/ui/textarea";
import { useForm } from "vee-validate";
import { toTypedSchema } from "@vee-validate/zod";
import * as z from "zod";

const route = useRoute();
const serverId = route.params.serverId;

const activeTab = ref("roles");
const loading = ref({
    roles: true,
    admins: true,
    users: true,
});
const error = ref<string | null>(null);
const roles = ref<ServerRole[]>([]);
const admins = ref<ServerAdmin[]>([]);
const users = ref<User[]>([]);
const showAddRoleDialog = ref(false);
const showAddAdminDialog = ref(false);
const showEditAdminDialog = ref(false);
const addRoleLoading = ref(false);
const addAdminLoading = ref(false);
const editAdminLoading = ref(false);
const cleanupLoading = ref(false);
const selectedAdminType = ref("user");
const editingAdmin = ref<ServerAdmin | null>(null);

// Squad permission categories
const permissionCategories = [
    {
        name: "Basic Admin",
        permissions: [
            "reserve",
            "balance",
            "canseeadminchat",
            "manageserver",
            "teamchange",
        ],
    },
    {
        name: "Chat",
        permissions: ["chat", "cameraman"],
    },
    {
        name: "Kick & Ban",
        permissions: ["kick", "ban", "forceteamchange", "immune"],
    },
    {
        name: "Map Control",
        permissions: [
            "changemap",
            "pause",
            "cheat",
            "private",
            "config",
            "featuretest",
            "demos",
        ],
    },
    {
        name: "Squad Management",
        permissions: ["disbandSquad", "removeFromSquad", "demoteCommander"],
    },
    {
        name: "Debug",
        permissions: ["debug"],
    },
];

// Interfaces
interface ServerRole {
    id: string;
    serverId: string;
    name: string;
    permissions: string[];
    created_at: string;
}

interface ServerAdmin {
    id: string;
    server_id: string;
    user_id?: string;
    steam_id?: string;
    username: string;
    server_role_id: string;
    expires_at?: string;
    notes?: string;
    is_active?: boolean;
    is_expired?: boolean;
    created_at: string;
}

interface User {
    id: string;
    username: string;
    email: string;
    superAdmin: boolean;
    created_at: string;
    name?: string;
}

interface RolesResponse {
    data: {
        roles: ServerRole[];
    };
}

interface AdminsResponse {
    data: {
        admins: ServerAdmin[];
    };
}

interface UsersResponse {
    data: {
        users: User[];
    };
}

// Form schemas
const roleFormSchema = toTypedSchema(
    z.object({
        name: z
            .string()
            .min(1, "Role name is required")
            .regex(
                /^[a-zA-Z0-9_]+$/,
                "Role name can only contain letters, numbers, and underscores. No spaces or special characters allowed.",
            ),
        permissions: z
            .array(z.string())
            .min(1, "At least one permission is required"),
    }),
);

const adminFormSchema = toTypedSchema(
    z
        .object({
            adminType: z.enum(["user", "steam_id"], {
                required_error: "Please select admin type",
            }),
            user_id: z.string().optional(),
            steam_id: z
                .string()
                .optional()
                .refine((val) => !val || /^\d{17}$/.test(val), {
                    message: "Steam ID must be exactly 17 digits",
                }),
            server_role_id: z.string().min(1, "Role is required"),
            expires_at: z.string().optional(),
            notes: z.string().optional(),
        })
        .refine(
            (data) => {
                if (data.adminType === "user") {
                    return data.user_id && data.user_id.length > 0;
                } else if (data.adminType === "steam_id") {
                    return data.steam_id && data.steam_id.length === 17;
                }
                return false;
            },
            {
                message: "Please select a user or enter a valid Steam ID",
                path: ["user_id"], // This will show the error on the user_id field
            },
        ),
);

// Edit admin form schema (only for notes)
const editAdminFormSchema = toTypedSchema(
    z.object({
        notes: z.string().optional(),
    }),
);

// Setup forms
const roleForm = useForm({
    validationSchema: roleFormSchema,
    initialValues: {
        name: "",
        permissions: [],
    },
});

const adminForm = useForm({
    validationSchema: adminFormSchema,
    initialValues: {
        adminType: "user",
        user_id: "",
        steam_id: "",
        server_role_id: "",
        expires_at: "",
        notes: "",
    },
});

const editAdminForm = useForm({
    validationSchema: editAdminFormSchema,
    initialValues: {
        notes: "",
    },
});

// Watch for changes in adminType to update our reactive reference
watch(
    () => adminForm.values.adminType,
    (newValue) => {
        selectedAdminType.value = newValue || "user";
    },
    { immediate: true },
);

// Watch for dialog state changes to reset form
watch(showAddAdminDialog, (isOpen) => {
    if (isOpen) {
        // Reset to default when dialog opens
        selectedAdminType.value = "user";
        adminForm.resetForm();
    }
});

// Function to fetch roles
async function fetchRoles() {
    loading.value.roles = true;
    error.value = null;

    const runtimeConfig = useRuntimeConfig();
    const cookieToken = useCookie(
        runtimeConfig.public.sessionCookieName as string,
    );
    const token = cookieToken.value;

    if (!token) {
        error.value = "Authentication required";
        loading.value.roles = false;
        return;
    }

    try {
        const { data, error: fetchError } = await useFetch<RolesResponse>(
            `${runtimeConfig.public.backendApi}/servers/${serverId}/roles`,
            {
                method: "GET",
                headers: {
                    Authorization: `Bearer ${token}`,
                },
            },
        );

        if (fetchError.value) {
            throw new Error(
                fetchError.value.message || "Failed to fetch roles",
            );
        }

        if (data.value && data.value.data) {
            roles.value = data.value.data.roles || [];
        }
    } catch (err: any) {
        error.value = err.message || "An error occurred while fetching roles";
        console.error(err);
    } finally {
        loading.value.roles = false;
    }
}

// Function to fetch admins
async function fetchAdmins() {
    loading.value.admins = true;
    error.value = null;

    const runtimeConfig = useRuntimeConfig();
    const cookieToken = useCookie(
        runtimeConfig.public.sessionCookieName as string,
    );
    const token = cookieToken.value;

    if (!token) {
        error.value = "Authentication required";
        loading.value.admins = false;
        return;
    }

    try {
        const { data, error: fetchError } = await useFetch<AdminsResponse>(
            `${runtimeConfig.public.backendApi}/servers/${serverId}/admins`,
            {
                method: "GET",
                headers: {
                    Authorization: `Bearer ${token}`,
                },
            },
        );

        if (fetchError.value) {
            throw new Error(
                fetchError.value.message || "Failed to fetch admins",
            );
        }

        if (data.value && data.value.data) {
            admins.value = data.value.data.admins || [];
        }
    } catch (err: any) {
        error.value = err.message || "An error occurred while fetching admins";
        console.error(err);
    } finally {
        loading.value.admins = false;
    }
}

// Function to fetch users
async function fetchUsers() {
    loading.value.users = true;
    error.value = null;

    const runtimeConfig = useRuntimeConfig();
    const cookieToken = useCookie(
        runtimeConfig.public.sessionCookieName as string,
    );
    const token = cookieToken.value;

    if (!token) {
        error.value = "Authentication required";
        loading.value.users = false;
        return;
    }

    try {
        const { data, error: fetchError } = await useFetch<UsersResponse>(
            `${runtimeConfig.public.backendApi}/users`,
            {
                method: "GET",
                headers: {
                    Authorization: `Bearer ${token}`,
                },
            },
        );

        if (fetchError.value) {
            throw new Error(
                fetchError.value.message || "Failed to fetch users",
            );
        }

        if (data.value && data.value.data) {
            users.value = data.value.data.users || [];
        }
    } catch (err: any) {
        error.value = err.message || "An error occurred while fetching users";
        console.error(err);
    } finally {
        loading.value.users = false;
    }
}

// Function to handle role form submission
async function onRoleSubmit(values: any) {
    console.log("Form submitted with values:", values);

    addRoleLoading.value = true;
    error.value = null;

    const runtimeConfig = useRuntimeConfig();
    const cookieToken = useCookie(
        runtimeConfig.public.sessionCookieName as string,
    );
    const token = cookieToken.value;

    if (!token) {
        error.value = "Authentication required";
        addRoleLoading.value = false;
        return;
    }

    try {
        const { data, error: fetchError } = await useFetch(
            `${runtimeConfig.public.backendApi}/servers/${serverId}/roles`,
            {
                method: "POST",
                body: {
                    name: values.name,
                    permissions: values.permissions || [],
                },
                headers: {
                    Authorization: `Bearer ${token}`,
                },
            },
        );

        if (fetchError.value) {
            throw new Error(
                fetchError.value.data.message || fetchError.value.message,
            );
        }

        // Reset form and close dialog
        roleForm.resetForm();
        showAddRoleDialog.value = false;

        // Refresh the roles list
        fetchRoles();
    } catch (err: any) {
        error.value = err.message || "An error occurred while adding the role";
        console.error(err);
    } finally {
        addRoleLoading.value = false;
    }
}

// Function to remove a role
async function removeRole(roleId: string) {
    if (
        !confirm(
            "Are you sure you want to remove this role? This will not affect existing admins with this role.",
        )
    ) {
        return;
    }

    loading.value.roles = true;
    error.value = null;

    const runtimeConfig = useRuntimeConfig();
    const cookieToken = useCookie(
        runtimeConfig.public.sessionCookieName as string,
    );
    const token = cookieToken.value;

    if (!token) {
        error.value = "Authentication required";
        loading.value.roles = false;
        return;
    }

    try {
        const { data, error: fetchError } = await useFetch(
            `${runtimeConfig.public.backendApi}/servers/${serverId}/roles/${roleId}`,
            {
                method: "DELETE",
                headers: {
                    Authorization: `Bearer ${token}`,
                },
            },
        );

        if (fetchError.value) {
            throw new Error(
                fetchError.value.data.message || fetchError.value.message,
            );
        }

        // Refresh the roles list
        fetchRoles();
    } catch (err: any) {
        error.value =
            err.message || "An error occurred while removing the role";
        console.error(err);
    } finally {
        loading.value.roles = false;
    }
}

// Function to add an admin
async function addAdmin(values: any) {
    const { adminType, user_id, steam_id, server_role_id, expires_at, notes } =
        values;

    addAdminLoading.value = true;
    error.value = null;

    const runtimeConfig = useRuntimeConfig();
    const cookieToken = useCookie(
        runtimeConfig.public.sessionCookieName as string,
    );
    const token = cookieToken.value;

    if (!token) {
        error.value = "Authentication required";
        addAdminLoading.value = false;
        return;
    }

    try {
        // Prepare request body based on admin type
        const requestBody: any = {
            server_role_id,
        };

        if (adminType === "user") {
            requestBody.user_id = user_id;
        } else if (adminType === "steam_id") {
            requestBody.steam_id = steam_id;
        }

        // Add expires_at if provided
        if (expires_at && expires_at.trim() !== "") {
            requestBody.expires_at = new Date(expires_at).toISOString();
        }

        // Add notes if provided
        if (notes && notes.trim() !== "") {
            requestBody.notes = notes;
        }

        const { data, error: fetchError } = await useFetch(
            `${runtimeConfig.public.backendApi}/servers/${serverId}/admins`,
            {
                method: "POST",
                body: requestBody,
                headers: {
                    Authorization: `Bearer ${token}`,
                },
            },
        );

        if (fetchError.value) {
            throw new Error(
                fetchError.value.data.message || fetchError.value.message,
            );
        }

        // Reset form and close dialog
        adminForm.resetForm();
        showAddAdminDialog.value = false;

        // Refresh the admins list
        fetchAdmins();
    } catch (err: any) {
        error.value = err.message || "An error occurred while adding the admin";
        console.error(err);
    } finally {
        addAdminLoading.value = false;
    }
}

// Function to remove an admin
async function removeAdmin(adminId: string) {
    if (!confirm("Are you sure you want to remove this admin?")) {
        return;
    }

    loading.value.admins = true;
    error.value = null;

    const runtimeConfig = useRuntimeConfig();
    const cookieToken = useCookie(
        runtimeConfig.public.sessionCookieName as string,
    );
    const token = cookieToken.value;

    if (!token) {
        error.value = "Authentication required";
        loading.value.admins = false;
        return;
    }

    try {
        const { data, error: fetchError } = await useFetch(
            `${runtimeConfig.public.backendApi}/servers/${serverId}/admins/${adminId}`,
            {
                method: "DELETE",
                headers: {
                    Authorization: `Bearer ${token}`,
                },
            },
        );

        if (fetchError.value) {
            throw new Error(
                fetchError.value.data.message || fetchError.value.message,
            );
        }

        // Refresh the admins list
        fetchAdmins();
    } catch (err: any) {
        error.value =
            err.message || "An error occurred while removing the admin";
        console.error(err);
    } finally {
        loading.value.admins = false;
    }
}

// Function to open edit admin dialog
function openEditAdminDialog(admin: ServerAdmin) {
    editingAdmin.value = admin;
    showEditAdminDialog.value = true;
}

// Function to close edit admin dialog
function closeEditAdminDialog() {
    showEditAdminDialog.value = false;
    editingAdmin.value = null;
}

// Function to update admin notes
async function updateAdminNotes(values: any) {
    if (!editingAdmin.value) return;

    editAdminLoading.value = true;
    error.value = null;

    const runtimeConfig = useRuntimeConfig();
    const cookieToken = useCookie(
        runtimeConfig.public.sessionCookieName as string,
    );
    const token = cookieToken.value;

    if (!token) {
        error.value = "Authentication required";
        editAdminLoading.value = false;
        return;
    }

    try {
        const { data, error: fetchError } = await useFetch(
            `${runtimeConfig.public.backendApi}/servers/${serverId}/admins/${editingAdmin.value.id}`,
            {
                method: "PUT",
                headers: {
                    Authorization: `Bearer ${token}`,
                    "Content-Type": "application/json",
                },
                body: JSON.stringify({
                    notes: values.notes,
                }),
            },
        );

        if (fetchError.value) {
            throw new Error(
                fetchError.value.data.message || fetchError.value.message,
            );
        }

        // Close dialog and refresh admins
        closeEditAdminDialog();
        fetchAdmins();
    } catch (err: any) {
        error.value =
            err.message || "An error occurred while updating the admin";
        console.error(err);
    } finally {
        editAdminLoading.value = false;
    }
}

// Function to cleanup expired admins
async function cleanupExpiredAdmins() {
    if (
        !confirm(
            "Are you sure you want to remove all expired admin roles? This action cannot be undone.",
        )
    ) {
        return;
    }

    cleanupLoading.value = true;
    error.value = null;

    const runtimeConfig = useRuntimeConfig();
    const cookieToken = useCookie(
        runtimeConfig.public.sessionCookieName as string,
    );
    const token = cookieToken.value;

    if (!token) {
        error.value = "Authentication required";
        cleanupLoading.value = false;
        return;
    }

    try {
        const { data, error: fetchError } = await useFetch(
            `${runtimeConfig.public.backendApi}/admin/cleanup-expired-admins`,
            {
                method: "POST",
                headers: {
                    Authorization: `Bearer ${token}`,
                },
            },
        );

        if (fetchError.value) {
            throw new Error(
                fetchError.value.data.message || fetchError.value.message,
            );
        }

        // Refresh the admins list
        fetchAdmins();
    } catch (err: any) {
        error.value =
            err.message || "An error occurred while cleaning up expired admins";
        console.error(err);
    } finally {
        cleanupLoading.value = false;
    }
}

function copyAdminCfgUrl() {
    const runtimeConfig = useRuntimeConfig();
    var url = "";
    if (runtimeConfig.public.backendApi.startsWith("/")) {
        const origin = window.location.origin;
        url = `${origin}${runtimeConfig.public.backendApi}/servers/${serverId}/admins/cfg`;
    } else {
        url = `${runtimeConfig.public.backendApi}/servers/${serverId}/admins/cfg`;
    }

    navigator.clipboard.writeText(url);
}

// Format date
function formatDate(dateString: string): string {
    return new Date(dateString).toLocaleString();
}

// Format expiration status
function formatExpirationStatus(admin: ServerAdmin): {
    text: string;
    variant: "default" | "destructive" | "secondary" | "outline";
    class: string;
} {
    if (!admin.expires_at) {
        return { text: "Permanent", variant: "default", class: "" };
    }

    const expiresAt = new Date(admin.expires_at);
    const now = new Date();

    if (expiresAt < now) {
        return { text: "Expired", variant: "destructive", class: "" };
    }

    const timeUntilExpiry = expiresAt.getTime() - now.getTime();
    const daysUntilExpiry = Math.floor(timeUntilExpiry / (1000 * 60 * 60 * 24));

    if (daysUntilExpiry <= 7) {
        return {
            text: `Expires in ${daysUntilExpiry} day(s)`,
            variant: "secondary",
            class: "text-orange-600",
        };
    }

    return {
        text: `Expires ${formatDate(admin.expires_at)}`,
        variant: "outline",
        class: "text-blue-600",
    };
}

// Get available users (all users are available since they can have multiple roles)
const availableUsers = computed(() => {
    return users.value;
});

// Get users that already have the selected role (to prevent duplicates)
const getUsersWithRole = (roleId: string) => {
    return admins.value
        .filter((admin) => admin.user_id && admin.server_role_id === roleId)
        .map((admin) => admin.user_id);
};

// Get filtered users based on selected role (exclude users who already have this specific role)
const getFilteredUsersForRole = (roleId: string) => {
    if (!roleId) return users.value;
    const usersWithRole = getUsersWithRole(roleId);
    return users.value.filter((user) => !usersWithRole.includes(user.id));
};

// Get filtered users for user selection based on selected role
const filteredUsersForSelection = computed(() => {
    const selectedRole = adminForm.values.server_role_id;
    if (!selectedRole) return users.value;
    return getFilteredUsersForRole(selectedRole);
});

// Group admins by user for better display
const groupedAdmins = computed(() => {
    const grouped: { [key: string]: ServerAdmin[] } = {};

    admins.value.forEach((admin) => {
        let key: string;
        if (admin.user_id) {
            key = `user_${admin.user_id}`;
        } else if (admin.steam_id) {
            key = `steam_${admin.steam_id}`;
        } else {
            key = `unknown_${admin.id}`;
        }

        if (!grouped[key]) {
            grouped[key] = [];
        }
        grouped[key].push(admin);
    });

    return grouped;
});

// Get display name for an admin
const getAdminDisplayName = (admin: ServerAdmin): string => {
    if (admin.steam_id) {
        return `Steam ID: ${admin.steam_id}`;
    } else if (admin.user_id) {
        const user = users.value.find((u) => u.id === admin.user_id);
        return user ? `${user.name} (${user.username})` : "Unknown User";
    }
    return "Unknown";
};

// Setup initial data load
onMounted(() => {
    fetchRoles();
    fetchAdmins();
    fetchUsers();
});
</script>

<template>
    <div class="p-4">
        <div class="flex justify-between items-center mb-4">
            <h1 class="text-2xl font-bold">Users & Roles</h1>

            <Button @click="copyAdminCfgUrl">Copy Admin Config URL</Button>
        </div>

        <div v-if="error" class="bg-red-500 text-white p-4 rounded mb-4">
            {{ error }}
        </div>

        <Tabs v-model="activeTab" class="w-full">
            <TabsList class="grid w-full grid-cols-2">
                <TabsTrigger value="roles">Roles</TabsTrigger>
                <TabsTrigger value="admins">Admins</TabsTrigger>
            </TabsList>

            <!-- Roles Tab -->
            <TabsContent value="roles">
                <Card>
                    <CardHeader
                        class="flex flex-row items-center justify-between pb-2"
                    >
                        <CardTitle>Role Management</CardTitle>
                        <Form
                            v-slot="{ handleSubmit }"
                            as=""
                            keep-values
                            :validation-schema="roleFormSchema"
                            :initial-values="{
                                name: '',
                                permissions: [],
                            }"
                        >
                            <Dialog v-model:open="showAddRoleDialog">
                                <DialogTrigger asChild>
                                    <Button>Add Role</Button>
                                </DialogTrigger>
                                <DialogContent
                                    class="sm:max-w-[600px] max-h-[80vh] overflow-y-auto"
                                >
                                    <DialogHeader>
                                        <DialogTitle>Add New Role</DialogTitle>
                                        <DialogDescription>
                                            Create a new role with specific
                                            permissions for Squad server
                                            administration.
                                        </DialogDescription>
                                    </DialogHeader>
                                    <form
                                        id="addRoleDialogForm"
                                        @submit="
                                            handleSubmit($event, onRoleSubmit)
                                        "
                                    >
                                        <div class="grid gap-4 py-4">
                                            <FormField
                                                name="name"
                                                v-slot="{ componentField }"
                                            >
                                                <FormItem>
                                                    <FormLabel
                                                        >Role Name</FormLabel
                                                    >
                                                    <FormControl>
                                                        <Input
                                                            placeholder="e.g., SeniorAdmin"
                                                            v-bind="
                                                                componentField
                                                            "
                                                        />
                                                    </FormControl>
                                                    <FormDescription>
                                                        Role name can only
                                                        contain letters,
                                                        numbers, and
                                                        underscores. No spaces
                                                        or special characters
                                                        allowed.
                                                    </FormDescription>
                                                    <FormMessage />
                                                </FormItem>
                                            </FormField>

                                            <FormField name="permissions">
                                                <FormItem>
                                                    <FormLabel
                                                        >Permissions</FormLabel
                                                    >
                                                    <FormDescription>
                                                        Select the permissions
                                                        for this role. Each
                                                        permission grants access
                                                        to specific Squad admin
                                                        commands.
                                                    </FormDescription>

                                                    <div
                                                        class="grid grid-cols-1 md:grid-cols-2 gap-4 mt-2"
                                                    >
                                                        <div
                                                            v-for="category in permissionCategories"
                                                            :key="category.name"
                                                            class="border rounded-md p-3"
                                                        >
                                                            <h3
                                                                class="font-medium mb-2"
                                                            >
                                                                {{
                                                                    category.name
                                                                }}
                                                            </h3>
                                                            <div
                                                                class="space-y-2"
                                                            >
                                                                <div
                                                                    v-for="permission in category.permissions"
                                                                    :key="
                                                                        permission
                                                                    "
                                                                    class="flex items-center space-x-2"
                                                                >
                                                                    <FormField
                                                                        v-slot="{
                                                                            value,
                                                                            handleChange,
                                                                        }"
                                                                        :key="
                                                                            permission
                                                                        "
                                                                        type="checkbox"
                                                                        :value="
                                                                            permission
                                                                        "
                                                                        :unchecked-value="
                                                                            false
                                                                        "
                                                                        name="permissions"
                                                                    >
                                                                        <FormItem
                                                                            class="flex flex-row items-start space-x-3 space-y-0"
                                                                        >
                                                                            <FormControl>
                                                                                <Checkbox
                                                                                    :model-value="
                                                                                        value.includes(
                                                                                            permission,
                                                                                        )
                                                                                    "
                                                                                    @update:model-value="
                                                                                        handleChange
                                                                                    "
                                                                                />
                                                                            </FormControl>
                                                                            <FormLabel
                                                                                class="font-normal"
                                                                            >
                                                                                {{
                                                                                    permission
                                                                                }}
                                                                            </FormLabel>
                                                                        </FormItem>
                                                                    </FormField>
                                                                </div>
                                                            </div>
                                                        </div>
                                                    </div>

                                                    <FormMessage />
                                                </FormItem>
                                            </FormField>
                                        </div>
                                        <DialogFooter>
                                            <Button
                                                type="button"
                                                variant="outline"
                                                @click="
                                                    showAddRoleDialog = false
                                                "
                                            >
                                                Cancel
                                            </Button>
                                            <Button
                                                type="submit"
                                                :disabled="addRoleLoading"
                                            >
                                                {{
                                                    addRoleLoading
                                                        ? "Adding..."
                                                        : "Add Role"
                                                }}
                                            </Button>
                                        </DialogFooter>
                                    </form>
                                </DialogContent>
                            </Dialog>
                        </Form>
                    </CardHeader>
                    <CardContent>
                        <div v-if="loading.roles" class="text-center py-8">
                            <div
                                class="animate-spin h-8 w-8 border-4 border-primary border-t-transparent rounded-full mx-auto mb-4"
                            ></div>
                            <p>Loading roles...</p>
                        </div>

                        <div
                            v-else-if="roles.length === 0"
                            class="text-center py-8"
                        >
                            <p>No roles found. Create a role to get started.</p>
                        </div>

                        <div v-else class="overflow-x-auto">
                            <Table>
                                <TableHeader>
                                    <TableRow>
                                        <TableHead>Role Name</TableHead>
                                        <TableHead>Permissions</TableHead>
                                        <TableHead>Created At</TableHead>
                                        <TableHead class="text-right"
                                            >Actions</TableHead
                                        >
                                    </TableRow>
                                </TableHeader>
                                <TableBody>
                                    <TableRow
                                        v-for="role in roles"
                                        :key="role.id"
                                        class="hover:bg-muted/50"
                                    >
                                        <TableCell class="font-medium">{{
                                            role.name
                                        }}</TableCell>
                                        <TableCell>
                                            <div class="flex flex-wrap gap-1">
                                                <Badge
                                                    v-for="permission in role.permissions"
                                                    :key="permission"
                                                    variant="outline"
                                                    class="text-xs"
                                                >
                                                    {{ permission }}
                                                </Badge>
                                            </div>
                                        </TableCell>
                                        <TableCell>{{
                                            formatDate(role.created_at)
                                        }}</TableCell>
                                        <TableCell class="text-right">
                                            <Button
                                                variant="destructive"
                                                size="sm"
                                                @click="removeRole(role.id)"
                                                :disabled="loading.roles"
                                            >
                                                Remove
                                            </Button>
                                        </TableCell>
                                    </TableRow>
                                </TableBody>
                            </Table>
                        </div>
                    </CardContent>
                </Card>
            </TabsContent>

            <!-- Admins Tab -->
            <TabsContent value="admins">
                <Card>
                    <CardHeader
                        class="flex flex-row items-center justify-between pb-2"
                    >
                        <CardTitle>Admin Management</CardTitle>
                        <div class="flex gap-2">
                            <Button
                                variant="outline"
                                size="sm"
                                @click="cleanupExpiredAdmins"
                                :disabled="cleanupLoading"
                            >
                                {{
                                    cleanupLoading
                                        ? "Cleaning..."
                                        : "Cleanup Expired"
                                }}
                            </Button>
                            <Form
                                v-slot="{ handleSubmit }"
                                as=""
                                keep-values
                                :validation-schema="adminFormSchema"
                                :initial-values="{
                                    adminType: 'user',
                                    user_id: '',
                                    steam_id: '',
                                    server_role_id: '',
                                    expires_at: '',
                                    notes: '',
                                }"
                            >
                                <Dialog v-model:open="showAddAdminDialog">
                                    <DialogTrigger asChild>
                                        <Button>Add Admin</Button>
                                    </DialogTrigger>
                                    <DialogContent
                                        class="sm:max-w-[425px] max-h-[80vh] overflow-y-auto"
                                    >
                                        <DialogHeader>
                                            <DialogTitle
                                                >Add New Admin</DialogTitle
                                            >
                                            <DialogDescription>
                                                Assign a user as an admin with a
                                                specific role.
                                            </DialogDescription>
                                        </DialogHeader>
                                        <form
                                            id="addAdminDialogForm"
                                            @submit="
                                                handleSubmit($event, addAdmin)
                                            "
                                        >
                                            <div class="grid gap-4 py-4">
                                                <FormField
                                                    name="adminType"
                                                    v-slot="{ componentField }"
                                                >
                                                    <FormItem>
                                                        <FormLabel
                                                            >Admin
                                                            Type</FormLabel
                                                        >
                                                        <Select
                                                            v-bind="
                                                                componentField
                                                            "
                                                            @update:model-value="
                                                                (value: any) =>
                                                                    (selectedAdminType =
                                                                        value ||
                                                                        'user')
                                                            "
                                                        >
                                                            <FormControl>
                                                                <SelectTrigger>
                                                                    <SelectValue
                                                                        placeholder="Select admin type"
                                                                    />
                                                                </SelectTrigger>
                                                            </FormControl>
                                                            <SelectContent>
                                                                <SelectGroup>
                                                                    <SelectItem
                                                                        value="user"
                                                                    >
                                                                        Existing
                                                                        User
                                                                    </SelectItem>
                                                                    <SelectItem
                                                                        value="steam_id"
                                                                    >
                                                                        Steam ID
                                                                        Only
                                                                    </SelectItem>
                                                                </SelectGroup>
                                                            </SelectContent>
                                                        </Select>
                                                        <FormDescription>
                                                            Choose whether to
                                                            assign an existing
                                                            user or add an admin
                                                            by Steam ID only.
                                                        </FormDescription>
                                                        <FormMessage />
                                                    </FormItem>
                                                </FormField>

                                                <FormField
                                                    name="user_id"
                                                    v-slot="{ componentField }"
                                                >
                                                    <FormItem
                                                        v-if="
                                                            selectedAdminType ===
                                                            'user'
                                                        "
                                                    >
                                                        <FormLabel
                                                            >User</FormLabel
                                                        >
                                                        <Select
                                                            v-bind="
                                                                componentField
                                                            "
                                                        >
                                                            <FormControl>
                                                                <SelectTrigger>
                                                                    <SelectValue
                                                                        placeholder="Select user"
                                                                    />
                                                                </SelectTrigger>
                                                            </FormControl>
                                                            <SelectContent>
                                                                <SelectGroup>
                                                                    <SelectItem
                                                                        v-for="user in filteredUsersForSelection"
                                                                        :key="
                                                                            user.id
                                                                        "
                                                                        :value="
                                                                            user.id
                                                                        "
                                                                    >
                                                                        {{
                                                                            user.name
                                                                        }}
                                                                        ({{
                                                                            user.username
                                                                        }})
                                                                    </SelectItem>
                                                                </SelectGroup>
                                                            </SelectContent>
                                                        </Select>
                                                        <FormMessage />
                                                    </FormItem>
                                                </FormField>

                                                <FormField
                                                    name="steam_id"
                                                    v-slot="{ componentField }"
                                                >
                                                    <FormItem
                                                        v-if="
                                                            selectedAdminType ===
                                                            'steam_id'
                                                        "
                                                    >
                                                        <FormLabel
                                                            >Steam ID</FormLabel
                                                        >
                                                        <FormControl>
                                                            <Input
                                                                placeholder="76561198012345678"
                                                                v-bind="
                                                                    componentField
                                                                "
                                                            />
                                                        </FormControl>
                                                        <FormDescription>
                                                            Enter the 17-digit
                                                            Steam ID of the
                                                            player you want to
                                                            make an admin.
                                                        </FormDescription>
                                                        <FormMessage />
                                                    </FormItem>
                                                </FormField>

                                                <FormField
                                                    name="server_role_id"
                                                    v-slot="{ componentField }"
                                                >
                                                    <FormItem>
                                                        <FormLabel
                                                            >Role</FormLabel
                                                        >
                                                        <Select
                                                            v-bind="
                                                                componentField
                                                            "
                                                        >
                                                            <FormControl>
                                                                <SelectTrigger>
                                                                    <SelectValue
                                                                        placeholder="Select role"
                                                                    />
                                                                </SelectTrigger>
                                                            </FormControl>
                                                            <SelectContent>
                                                                <SelectGroup>
                                                                    <SelectItem
                                                                        v-for="role in roles"
                                                                        :key="
                                                                            role.id
                                                                        "
                                                                        :value="
                                                                            role.id
                                                                        "
                                                                    >
                                                                        {{
                                                                            role.name
                                                                        }}
                                                                    </SelectItem>
                                                                </SelectGroup>
                                                            </SelectContent>
                                                        </Select>
                                                        <FormMessage />
                                                    </FormItem>
                                                </FormField>

                                                <FormField
                                                    name="expires_at"
                                                    v-slot="{ componentField }"
                                                >
                                                    <FormItem>
                                                        <FormLabel
                                                            >Expiration Date
                                                            (Optional)</FormLabel
                                                        >
                                                        <FormControl>
                                                            <Input
                                                                type="datetime-local"
                                                                v-bind="
                                                                    componentField
                                                                "
                                                            />
                                                        </FormControl>
                                                        <FormDescription>
                                                            Leave empty for
                                                            permanent admin
                                                            access. Set a date
                                                            and time for
                                                            temporary access
                                                            (e.g., 1-month
                                                            trial).
                                                        </FormDescription>
                                                        <FormMessage />
                                                    </FormItem>
                                                </FormField>

                                                <FormField
                                                    name="notes"
                                                    v-slot="{ componentField }"
                                                >
                                                    <FormItem>
                                                        <FormLabel
                                                            >Notes
                                                            (Optional)</FormLabel
                                                        >
                                                        <FormControl>
                                                            <Input
                                                                placeholder="e.g., Trial period, Temporary whitelist, etc."
                                                                v-bind="
                                                                    componentField
                                                                "
                                                            />
                                                        </FormControl>
                                                        <FormDescription>
                                                            Add notes about this
                                                            admin assignment for
                                                            future reference.
                                                        </FormDescription>
                                                        <FormMessage />
                                                    </FormItem>
                                                </FormField>
                                            </div>
                                            <DialogFooter>
                                                <Button
                                                    type="button"
                                                    variant="outline"
                                                    @click="
                                                        showAddAdminDialog = false
                                                    "
                                                >
                                                    Cancel
                                                </Button>
                                                <Button
                                                    type="submit"
                                                    :disabled="addAdminLoading"
                                                >
                                                    {{
                                                        addAdminLoading
                                                            ? "Adding..."
                                                            : "Add Admin"
                                                    }}
                                                </Button>
                                            </DialogFooter>
                                        </form>
                                    </DialogContent>
                                </Dialog>
                            </Form>

                            <!-- Edit Admin Dialog -->
                            <Form
                                :key="editingAdmin?.id"
                                :validation-schema="editAdminFormSchema"
                                :initial-values="{
                                    notes: editingAdmin?.notes || '',
                                }"
                                v-slot="{ handleSubmit }"
                            >
                                <Dialog v-model:open="showEditAdminDialog">
                                    <DialogContent class="sm:max-w-[500px]">
                                        <DialogHeader>
                                            <DialogTitle
                                                >Edit Admin Notes</DialogTitle
                                            >
                                            <DialogDescription>
                                                Update the notes for this admin
                                                assignment.
                                            </DialogDescription>
                                        </DialogHeader>
                                        <form
                                            @submit="
                                                handleSubmit(
                                                    $event,
                                                    updateAdminNotes,
                                                )
                                            "
                                        >
                                            <div class="grid gap-4 py-4">
                                                <FormField
                                                    v-slot="{ componentField }"
                                                    name="notes"
                                                >
                                                    <FormItem>
                                                        <FormLabel
                                                            >Notes</FormLabel
                                                        >
                                                        <FormControl>
                                                            <Textarea
                                                                v-bind="
                                                                    componentField
                                                                "
                                                                placeholder="Add notes about this admin assignment..."
                                                                rows="4"
                                                            />
                                                        </FormControl>
                                                        <FormDescription>
                                                            Add any notes or
                                                            context about this
                                                            admin assignment.
                                                        </FormDescription>
                                                        <FormMessage />
                                                    </FormItem>
                                                </FormField>
                                            </div>
                                            <DialogFooter>
                                                <Button
                                                    type="button"
                                                    variant="outline"
                                                    @click="
                                                        closeEditAdminDialog
                                                    "
                                                >
                                                    Cancel
                                                </Button>
                                                <Button
                                                    type="submit"
                                                    :disabled="editAdminLoading"
                                                >
                                                    {{
                                                        editAdminLoading
                                                            ? "Updating..."
                                                            : "Update Notes"
                                                    }}
                                                </Button>
                                            </DialogFooter>
                                        </form>
                                    </DialogContent>
                                </Dialog>
                            </Form>
                        </div>
                    </CardHeader>
                    <CardContent>
                        <div v-if="loading.admins" class="text-center py-8">
                            <div
                                class="animate-spin h-8 w-8 border-4 border-primary border-t-transparent rounded-full mx-auto mb-4"
                            ></div>
                            <p>Loading admins...</p>
                        </div>

                        <div
                            v-else-if="admins.length === 0"
                            class="text-center py-8"
                        >
                            <p>No admins found. Add an admin to get started.</p>
                        </div>

                        <div v-else class="overflow-x-auto">
                            <Table>
                                <TableHeader>
                                    <TableRow>
                                        <TableHead>User</TableHead>
                                        <TableHead>Role</TableHead>
                                        <TableHead>Status</TableHead>
                                        <TableHead>Notes</TableHead>
                                        <TableHead>Created At</TableHead>
                                        <TableHead class="text-right"
                                            >Actions</TableHead
                                        >
                                    </TableRow>
                                </TableHeader>
                                <TableBody>
                                    <template
                                        v-for="(
                                            adminGroup, key
                                        ) in groupedAdmins"
                                        :key="key"
                                    >
                                        <TableRow
                                            v-for="(admin, index) in adminGroup"
                                            :key="admin.id"
                                            class="hover:bg-muted/50"
                                            :class="{
                                                'opacity-60': admin.is_expired,
                                            }"
                                        >
                                            <TableCell>
                                                <!-- Only show user info for the first row of each group -->
                                                <div v-if="index === 0">
                                                    {{
                                                        getAdminDisplayName(
                                                            admin,
                                                        )
                                                    }}
                                                    <div
                                                        v-if="
                                                            adminGroup.length >
                                                            1
                                                        "
                                                        class="text-xs text-blue-600 font-medium"
                                                    >
                                                        {{ adminGroup.length }}
                                                        roles
                                                    </div>
                                                </div>
                                            </TableCell>
                                            <TableCell>
                                                <Badge variant="outline">
                                                    {{
                                                        roles.find(
                                                            (r) =>
                                                                r.id ===
                                                                admin.server_role_id,
                                                        )?.name
                                                    }}
                                                </Badge>
                                            </TableCell>
                                            <TableCell>
                                                <div
                                                    class="flex flex-col gap-1"
                                                >
                                                    <Badge
                                                        :variant="
                                                            formatExpirationStatus(
                                                                admin,
                                                            ).variant
                                                        "
                                                        :class="
                                                            formatExpirationStatus(
                                                                admin,
                                                            ).class
                                                        "
                                                        class="w-fit"
                                                    >
                                                        {{
                                                            formatExpirationStatus(
                                                                admin,
                                                            ).text
                                                        }}
                                                    </Badge>
                                                    <div
                                                        v-if="
                                                            admin.expires_at &&
                                                            !admin.is_expired
                                                        "
                                                        class="text-xs text-muted-foreground"
                                                    >
                                                        Expires:
                                                        {{
                                                            formatDate(
                                                                admin.expires_at,
                                                            )
                                                        }}
                                                    </div>
                                                </div>
                                            </TableCell>
                                            <TableCell>
                                                <div
                                                    class="max-w-32 truncate"
                                                    :title="admin.notes"
                                                >
                                                    <span
                                                        v-if="admin.notes"
                                                        class="text-sm"
                                                        >{{ admin.notes }}</span
                                                    >
                                                    <span
                                                        v-else
                                                        class="text-sm text-muted-foreground italic"
                                                        >No notes</span
                                                    >
                                                </div>
                                            </TableCell>
                                            <TableCell>{{
                                                formatDate(admin.created_at)
                                            }}</TableCell>
                                            <TableCell class="text-right">
                                                <div
                                                    class="flex gap-2 justify-end"
                                                >
                                                    <Button
                                                        variant="outline"
                                                        size="sm"
                                                        @click="
                                                            openEditAdminDialog(
                                                                admin,
                                                            )
                                                        "
                                                        :disabled="
                                                            loading.admins
                                                        "
                                                    >
                                                        Edit
                                                    </Button>
                                                    <Button
                                                        variant="destructive"
                                                        size="sm"
                                                        @click="
                                                            removeAdmin(
                                                                admin.id,
                                                            )
                                                        "
                                                        :disabled="
                                                            loading.admins
                                                        "
                                                    >
                                                        Remove
                                                    </Button>
                                                </div>
                                            </TableCell>
                                        </TableRow>
                                    </template>
                                </TableBody>
                            </Table>
                        </div>
                    </CardContent>
                </Card>
            </TabsContent>
        </Tabs>

        <Card class="mt-6">
            <CardHeader>
                <CardTitle>About Users & Roles</CardTitle>
            </CardHeader>
            <CardContent>
                <p class="text-sm text-muted-foreground">
                    This page allows you to manage roles and admins for your
                    Squad server. Roles define what permissions an admin has,
                    and admins are users assigned to those roles. Users can have
                    multiple roles simultaneously.
                </p>
                <p class="text-sm text-muted-foreground mt-2">
                    <strong>Roles</strong> - Create roles with specific
                    permissions that determine what actions admins can perform
                    on the server.
                </p>
                <p class="text-sm text-muted-foreground mt-2">
                    <strong>Admins</strong> - Assign users to roles, giving them
                    admin privileges on your server. You can set an expiration
                    date for temporary access (e.g., trial periods, temporary
                    whitelist). Users can be assigned multiple different roles,
                    but cannot have duplicate role assignments.
                </p>
                <p class="text-sm text-muted-foreground mt-2">
                    <strong>Admin Expiration</strong> - Expired admin roles are
                    automatically cleaned up hourly. You can also manually
                    trigger cleanup using the "Cleanup Expired" button.
                </p>
                <p class="text-sm text-muted-foreground mt-2">
                    <strong>Admin Config</strong> - Download the admin
                    configuration file to use with your Squad server. Only
                    active (non-expired) admins are included in the config.
                </p>
                <div class="mt-4">
                    <h3 class="font-medium mb-2">Permission Categories</h3>
                    <div class="grid grid-cols-1 md:grid-cols-3 gap-4">
                        <div
                            v-for="category in permissionCategories"
                            :key="category.name"
                            class="border rounded-md p-3"
                        >
                            <h4 class="font-medium mb-1">
                                {{ category.name }}
                            </h4>
                            <ul
                                class="text-sm text-muted-foreground list-disc list-inside"
                            >
                                <li
                                    v-for="permission in category.permissions"
                                    :key="permission"
                                >
                                    {{ permission }}
                                </li>
                            </ul>
                        </div>
                    </div>
                </div>
            </CardContent>
        </Card>
    </div>
</template>

<style scoped>
/* Add any page-specific styles here */
</style>
