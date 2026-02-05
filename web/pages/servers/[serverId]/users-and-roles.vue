<script setup lang="ts">
import { ref, onMounted, computed, watch } from "vue";
import { Button } from "~/components/ui/button";
import { Input } from "~/components/ui/input";
import { isSecureOrLocalConnection } from "~/utils/security";
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
import {
    Popover,
    PopoverContent,
    PopoverTrigger,
} from "~/components/ui/popover";
import { Checkbox } from "~/components/ui/checkbox";
import {
    Select,
    SelectContent,
    SelectItem,
    SelectTrigger,
    SelectValue,
    SelectGroup,
    SelectLabel,
} from "~/components/ui/select";
import { Textarea } from "~/components/ui/textarea";
import { toast } from "~/components/ui/toast";
import { useForm } from "vee-validate";
import { toTypedSchema } from "@vee-validate/zod";
import * as z from "zod";
import {
    PERMISSION_CATEGORY_NAMES,
    getPermissionCategory,
    type PermissionCategory,
} from "@/constants/permissions";

const route = useRoute();
const serverId = route.params.serverId;

const activeTab = ref("roles");
const loading = ref({
    roles: true,
    admins: true,
    users: true,
    permissions: true,
    templates: true,
});
const error = ref<string | null>(null);
const roles = ref<ServerRole[]>([]);
const admins = ref<ServerAdmin[]>([]);
const users = ref<User[]>([]);
const permissions = ref<PermissionGroup[]>([]);
const roleTemplates = ref<RoleTemplate[]>([]);
const showAddRoleDialog = ref(false);
const showEditRoleDialog = ref(false);
const showAddAdminDialog = ref(false);
const showEditAdminDialog = ref(false);
const showCreateFromTemplateDialog = ref(false);
const addRoleLoading = ref(false);
const editRoleLoading = ref(false);
const addAdminLoading = ref(false);
const editAdminLoading = ref(false);
const cleanupLoading = ref(false);
const createFromTemplateLoading = ref(false);
const selectedAdminType = ref("user");
const editingAdmin = ref<ServerAdmin | null>(null);
const showAdminCfgPopover = ref(false);
const editingRole = ref<ServerRole | null>(null);

// Interfaces
interface PermissionDefinition {
    id: string;
    code: string;
    category: string;
    name: string;
    description: string;
    squad_permission?: string;
}

interface PermissionGroup {
    category: string;
    category_name: string;
    permissions: PermissionDefinition[];
}

interface RoleTemplate {
    id: string;
    name: string;
    description: string;
    is_system: boolean;
    is_admin: boolean;
    permissions: string[];
}

interface ServerRole {
    id: string;
    serverId: string;
    name: string;
    permissions: string[];
    is_admin: boolean;
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

interface PermissionsResponse {
    data: {
        permissions: PermissionGroup[];
    };
}

interface RoleTemplatesResponse {
    data: {
        templates: RoleTemplate[];
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
        is_admin: z.boolean().default(true),
    }),
);

const templateFormSchema = toTypedSchema(
    z.object({
        template_id: z.string().min(1, "Please select a template"),
        name: z
            .string()
            .min(1, "Role name is required")
            .regex(
                /^[a-zA-Z0-9_]+$/,
                "Role name can only contain letters, numbers, and underscores.",
            ),
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
                path: ["user_id"],
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
        is_admin: true,
    },
});

const editRoleForm = useForm({
    validationSchema: roleFormSchema,
    initialValues: {
        name: "",
        permissions: [],
        is_admin: true,
    },
});

const templateForm = useForm({
    validationSchema: templateFormSchema,
    initialValues: {
        template_id: "",
        name: "",
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

// Watch for template selection to auto-fill name
watch(
    () => templateForm.values.template_id,
    (newValue) => {
        if (newValue) {
            const template = roleTemplates.value.find((t) => t.id === newValue);
            if (template) {
                templateForm.setFieldValue("name", template.name);
            }
        }
    },
);

// Function to fetch permissions from API
async function fetchPermissions() {
    loading.value.permissions = true;

    const runtimeConfig = useRuntimeConfig();

    try {
        const { data, error: fetchError } = await useAuthFetch<PermissionsResponse>(
            `${runtimeConfig.public.backendApi}/permissions`,
            {
                method: "GET",
            },
        );

        if (fetchError.value) {
            throw new Error(
                fetchError.value.message || "Failed to fetch permissions",
            );
        }

        if (data.value && data.value.data) {
            permissions.value = data.value.data.permissions || [];
        }
    } catch (err: any) {
        console.error("Error fetching permissions:", err);
    } finally {
        loading.value.permissions = false;
    }
}

// Function to fetch role templates from API
async function fetchRoleTemplates() {
    loading.value.templates = true;

    const runtimeConfig = useRuntimeConfig();

    try {
        const { data, error: fetchError } = await useAuthFetch<RoleTemplatesResponse>(
            `${runtimeConfig.public.backendApi}/role-templates`,
            {
                method: "GET",
            },
        );

        if (fetchError.value) {
            throw new Error(
                fetchError.value.message || "Failed to fetch role templates",
            );
        }

        if (data.value && data.value.data) {
            roleTemplates.value = data.value.data.templates || [];
        }
    } catch (err: any) {
        console.error("Error fetching role templates:", err);
    } finally {
        loading.value.templates = false;
    }
}

// Function to fetch roles
async function fetchRoles() {
    loading.value.roles = true;
    error.value = null;

    const runtimeConfig = useRuntimeConfig();

    try {
        const { data, error: fetchError } = await useAuthFetch<RolesResponse>(
            `${runtimeConfig.public.backendApi}/servers/${serverId}/roles`,
            {
                method: "GET",
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

    try {
        const { data, error: fetchError } = await useAuthFetch<AdminsResponse>(
            `${runtimeConfig.public.backendApi}/servers/${serverId}/admins`,
            {
                method: "GET",
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

    try {
        const { data, error: fetchError } = await useAuthFetch<UsersResponse>(
            `${runtimeConfig.public.backendApi}/users`,
            {
                method: "GET",
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
    addRoleLoading.value = true;
    error.value = null;

    const runtimeConfig = useRuntimeConfig();

    try {
        const { data, error: fetchError } = await useAuthFetch(
            `${runtimeConfig.public.backendApi}/servers/${serverId}/roles`,
            {
                method: "POST",
                body: {
                    name: values.name,
                    permissions: values.permissions || [],
                    is_admin: values.is_admin !== undefined ? values.is_admin : true,
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

        toast({
            title: "Success",
            description: "Role created successfully",
        });

        // Refresh the roles list
        fetchRoles();
    } catch (err: any) {
        error.value = err.message || "An error occurred while adding the role";
        console.error(err);
    } finally {
        addRoleLoading.value = false;
    }
}

// Function to create role from template
async function createRoleFromTemplate(values: any) {
    createFromTemplateLoading.value = true;
    error.value = null;

    const runtimeConfig = useRuntimeConfig();

    try {
        const { data, error: fetchError } = await useAuthFetch(
            `${runtimeConfig.public.backendApi}/servers/${serverId}/roles/from-template`,
            {
                method: "POST",
                body: {
                    template_id: values.template_id,
                    name: values.name,
                },
            },
        );

        if (fetchError.value) {
            throw new Error(
                fetchError.value.data.message || fetchError.value.message,
            );
        }

        // Reset form and close dialog
        templateForm.resetForm();
        showCreateFromTemplateDialog.value = false;

        toast({
            title: "Success",
            description: "Role created from template successfully",
        });

        // Refresh the roles list
        fetchRoles();
    } catch (err: any) {
        error.value = err.message || "An error occurred while creating the role";
        console.error(err);
    } finally {
        createFromTemplateLoading.value = false;
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

    try {
        const { data, error: fetchError } = await useAuthFetch(
            `${runtimeConfig.public.backendApi}/servers/${serverId}/roles/${roleId}`,
            {
                method: "DELETE",
            },
        );

        if (fetchError.value) {
            throw new Error(
                fetchError.value.data.message || fetchError.value.message,
            );
        }

        toast({
            title: "Success",
            description: "Role removed successfully",
        });

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

// Function to open edit role dialog
function openEditRoleDialog(role: ServerRole) {
    editingRole.value = role;
    editRoleForm.resetForm({
        values: {
            name: role.name,
            permissions: role.permissions,
            is_admin: role.is_admin,
        },
    });
    showEditRoleDialog.value = true;
}

// Function to close edit role dialog
function closeEditRoleDialog() {
    showEditRoleDialog.value = false;
    editingRole.value = null;
}

// Function to update a role
async function updateRole(values: any) {
    if (!editingRole.value) return;

    editRoleLoading.value = true;
    error.value = null;

    const runtimeConfig = useRuntimeConfig();

    try {
        const updateBody: any = {};
        if (values.name !== editingRole.value.name) {
            updateBody.name = values.name;
        }
        if (JSON.stringify(values.permissions) !== JSON.stringify(editingRole.value.permissions)) {
            updateBody.permissions = values.permissions || [];
        }
        if (values.is_admin !== editingRole.value.is_admin) {
            updateBody.is_admin = values.is_admin;
        }

        if (Object.keys(updateBody).length === 0) {
            error.value = "No changes to update";
            editRoleLoading.value = false;
            return;
        }

        const { data, error: fetchError } = await useAuthFetch(
            `${runtimeConfig.public.backendApi}/servers/${serverId}/roles/${editingRole.value.id}`,
            {
                method: "PUT",
                body: updateBody,
            },
        );

        if (fetchError.value) {
            throw new Error(
                fetchError.value.data.message || fetchError.value.message,
            );
        }

        toast({
            title: "Success",
            description: "Role updated successfully",
        });

        // Close dialog and refresh roles
        closeEditRoleDialog();
        fetchRoles();
    } catch (err: any) {
        error.value = err.message || "An error occurred while updating the role";
        console.error(err);
    } finally {
        editRoleLoading.value = false;
    }
}

// Function to add an admin
async function addAdmin(values: any) {
    const { adminType, user_id, steam_id, server_role_id, expires_at, notes } =
        values;

    addAdminLoading.value = true;
    error.value = null;

    const runtimeConfig = useRuntimeConfig();

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

        const { data, error: fetchError } = await useAuthFetch(
            `${runtimeConfig.public.backendApi}/servers/${serverId}/admins`,
            {
                method: "POST",
                body: requestBody,
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

        toast({
            title: "Success",
            description: "Admin added successfully",
        });

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

    try {
        const { data, error: fetchError } = await useAuthFetch(
            `${runtimeConfig.public.backendApi}/servers/${serverId}/admins/${adminId}`,
            {
                method: "DELETE",
            },
        );

        if (fetchError.value) {
            throw new Error(
                fetchError.value.data.message || fetchError.value.message,
            );
        }

        toast({
            title: "Success",
            description: "Admin removed successfully",
        });

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

    try {
        const { data, error: fetchError } = await useAuthFetch(
            `${runtimeConfig.public.backendApi}/servers/${serverId}/admins/${editingAdmin.value.id}`,
            {
                method: "PUT",
                body: {
                    notes: values.notes,
                },
            },
        );

        if (fetchError.value) {
            throw new Error(
                fetchError.value.data.message || fetchError.value.message,
            );
        }

        toast({
            title: "Success",
            description: "Admin notes updated successfully",
        });

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

    try {
        const { data, error: fetchError } = await useAuthFetch(
            `${runtimeConfig.public.backendApi}/admin/cleanup-expired-admins`,
            {
                method: "POST",
            },
        );

        if (fetchError.value) {
            throw new Error(
                fetchError.value.data.message || fetchError.value.message,
            );
        }

        toast({
            title: "Success",
            description: "Expired admins cleaned up successfully",
        });

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

// Security check for copy buttons
const canCopyConfigUrl = computed(() => isSecureOrLocalConnection());

// Computed property for admin config URL
const adminCfgUrl = computed(() => {
    const runtimeConfig = useRuntimeConfig();
    var url = "";
    if (runtimeConfig.public.backendApi.startsWith("/")) {
        const origin = window.location.origin;
        url = `${origin}${runtimeConfig.public.backendApi}/servers/${serverId}/admins/cfg`;
    } else {
        url = `${runtimeConfig.public.backendApi}/servers/${serverId}/admins/cfg`;
    }
    return url;
});

function copyAdminCfgUrl() {
    if (!canCopyConfigUrl.value) {
        // On HTTP, just open the popover to show the URL
        showAdminCfgPopover.value = true;
        return;
    }

    navigator.clipboard.writeText(adminCfgUrl.value);

    toast({
        title: "Success",
        description: "Admin configuration URL copied to clipboard",
    });
}

function selectAdminCfgUrl(event: Event) {
    const input = event.target as HTMLInputElement;
    input.select();
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

// Get role name by ID
const getRoleName = (roleId: string): string => {
    const role = roles.value.find((r) => r.id === roleId);
    return role ? role.name : "Unknown Role";
};

// Format permission for display
const formatPermissionDisplay = (permCode: string): string => {
    // Try to find in loaded permissions by exact code
    for (const group of permissions.value) {
        const perm = group.permissions.find((p) => p.code === permCode);
        if (perm) {
            return perm.name;
        }
    }

    // For old-style permissions (without prefix), try to find matching rcon permission
    if (!permCode.includes(":")) {
        const rconCode = `rcon:${permCode.toLowerCase()}`;
        for (const group of permissions.value) {
            const perm = group.permissions.find((p) => p.code === rconCode);
            if (perm) {
                return perm.name;
            }
        }
        // Still not found - capitalize the old permission name
        return permCode.charAt(0).toUpperCase() + permCode.slice(1);
    }

    // New format with colon - extract the part after category
    const parts = permCode.split(":");
    if (parts.length >= 2) {
        return parts.slice(1).join(" ").replace(/_/g, " ");
    }
    return permCode;
};

// Get permission category badge color
const getPermissionCategoryColor = (permCode: string): string => {
    const category = getPermissionCategory(permCode);
    switch (category) {
        case "ui":
            return "bg-blue-100 text-blue-800 dark:bg-blue-900 dark:text-blue-200";
        case "api":
            return "bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-200";
        case "rcon":
            return "bg-purple-100 text-purple-800 dark:bg-purple-900 dark:text-purple-200";
        default:
            return "bg-gray-100 text-gray-800 dark:bg-gray-900 dark:text-gray-200";
    }
};

// Setup initial data load
onMounted(() => {
    fetchPermissions();
    fetchRoleTemplates();
    fetchRoles();
    fetchAdmins();
    fetchUsers();
});
</script>

<template>
    <div class="p-3 sm:p-4">
        <div class="flex flex-col sm:flex-row sm:justify-between sm:items-center gap-3 sm:gap-0 mb-3 sm:mb-4">
            <h1 class="text-xl sm:text-2xl font-bold">Users & Roles</h1>

            <Popover v-model:open="showAdminCfgPopover">
                <PopoverTrigger asChild>
                    <Button
                        @click="copyAdminCfgUrl"
                        :title="canCopyConfigUrl ? 'Copy Admin Config URL' : 'Click to view URL (copy manually on HTTP)'"
                        class="w-full sm:w-auto text-sm sm:text-base"
                    >
                        Copy Admin Config URL
                    </Button>
                </PopoverTrigger>
                <PopoverContent v-if="!canCopyConfigUrl" class="w-80">
                    <div class="space-y-2">
                        <h4 class="font-medium text-sm">Admin Config URL</h4>
                        <p class="text-xs text-muted-foreground">
                            Automatic copying requires HTTPS or localhost. Please copy the URL manually:
                        </p>
                        <Input
                            :value="adminCfgUrl"
                            readonly
                            @focus="selectAdminCfgUrl"
                            class="text-xs"
                        />
                    </div>
                </PopoverContent>
            </Popover>
        </div>

        <div v-if="error" class="bg-red-500 text-white p-3 sm:p-4 rounded mb-3 sm:mb-4 text-sm sm:text-base">
            {{ error }}
        </div>

        <Tabs v-model="activeTab" class="w-full">
            <TabsList class="grid w-full grid-cols-2">
                <TabsTrigger value="roles" class="text-xs sm:text-sm">Roles</TabsTrigger>
                <TabsTrigger value="admins" class="text-xs sm:text-sm">Admins</TabsTrigger>
            </TabsList>

            <!-- Roles Tab -->
            <TabsContent value="roles">
                <Card>
                    <CardHeader
                        class="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-3 sm:gap-0 pb-2 sm:pb-3"
                    >
                        <CardTitle class="text-base sm:text-lg">Role Management</CardTitle>
                        <div class="flex flex-col sm:flex-row gap-2 w-full sm:w-auto">
                            <!-- Create from Template Button -->
                            <Form
                                v-slot="{ handleSubmit }"
                                as=""
                                keep-values
                                :validation-schema="templateFormSchema"
                                :initial-values="{
                                    template_id: '',
                                    name: '',
                                }"
                            >
                                <Dialog v-model:open="showCreateFromTemplateDialog">
                                    <DialogTrigger asChild>
                                        <Button variant="outline" class="w-full sm:w-auto text-sm sm:text-base">
                                            From Template
                                        </Button>
                                    </DialogTrigger>
                                    <DialogContent
                                        class="w-[95vw] sm:max-w-[500px] max-h-[85vh] sm:max-h-[80vh] overflow-y-auto p-4 sm:p-6"
                                    >
                                        <DialogHeader>
                                            <DialogTitle class="text-base sm:text-lg">Create Role from Template</DialogTitle>
                                            <DialogDescription class="text-xs sm:text-sm">
                                                Select a predefined role template to quickly create a new role with standard permissions.
                                            </DialogDescription>
                                        </DialogHeader>
                                        <form
                                            id="createFromTemplateForm"
                                            @submit="handleSubmit($event, createRoleFromTemplate)"
                                        >
                                            <div class="grid gap-4 py-4">
                                                <FormField
                                                    name="template_id"
                                                    v-slot="{ componentField }"
                                                >
                                                    <FormItem>
                                                        <FormLabel>Template</FormLabel>
                                                        <Select v-bind="componentField">
                                                            <FormControl>
                                                                <SelectTrigger>
                                                                    <SelectValue placeholder="Select a template" />
                                                                </SelectTrigger>
                                                            </FormControl>
                                                            <SelectContent>
                                                                <SelectGroup>
                                                                    <SelectLabel>Role Templates</SelectLabel>
                                                                    <SelectItem
                                                                        v-for="template in roleTemplates"
                                                                        :key="template.id"
                                                                        :value="template.id"
                                                                    >
                                                                        <div class="flex items-center gap-2">
                                                                            <span>{{ template.name }}</span>
                                                                            <Badge
                                                                                v-if="!template.is_admin"
                                                                                variant="secondary"
                                                                                class="text-xs"
                                                                            >
                                                                                Non-Admin
                                                                            </Badge>
                                                                        </div>
                                                                    </SelectItem>
                                                                </SelectGroup>
                                                            </SelectContent>
                                                        </Select>
                                                        <FormDescription>
                                                            Choose a template to base your new role on.
                                                        </FormDescription>
                                                        <FormMessage />
                                                    </FormItem>
                                                </FormField>

                                                <!-- Template Preview -->
                                                <div
                                                    v-if="templateForm.values.template_id"
                                                    class="border rounded-md p-3 bg-muted/50"
                                                >
                                                    <div class="text-sm font-medium mb-2">Template Preview</div>
                                                    <div class="text-xs text-muted-foreground mb-2">
                                                        {{ roleTemplates.find(t => t.id === templateForm.values.template_id)?.description }}
                                                    </div>
                                                    <div class="flex flex-wrap gap-1">
                                                        <Badge
                                                            v-for="perm in roleTemplates.find(t => t.id === templateForm.values.template_id)?.permissions || []"
                                                            :key="perm"
                                                            variant="outline"
                                                            :class="['text-xs', getPermissionCategoryColor(perm)]"
                                                        >
                                                            {{ formatPermissionDisplay(perm) }}
                                                        </Badge>
                                                    </div>
                                                </div>

                                                <FormField
                                                    name="name"
                                                    v-slot="{ componentField }"
                                                >
                                                    <FormItem>
                                                        <FormLabel>Role Name</FormLabel>
                                                        <FormControl>
                                                            <Input
                                                                placeholder="e.g., SeniorAdmin"
                                                                v-bind="componentField"
                                                            />
                                                        </FormControl>
                                                        <FormDescription>
                                                            Customize the name for this server's role.
                                                        </FormDescription>
                                                        <FormMessage />
                                                    </FormItem>
                                                </FormField>
                                            </div>
                                            <DialogFooter>
                                                <Button
                                                    type="button"
                                                    variant="outline"
                                                    @click="showCreateFromTemplateDialog = false"
                                                >
                                                    Cancel
                                                </Button>
                                                <Button
                                                    type="submit"
                                                    :disabled="createFromTemplateLoading"
                                                >
                                                    {{ createFromTemplateLoading ? "Creating..." : "Create Role" }}
                                                </Button>
                                            </DialogFooter>
                                        </form>
                                    </DialogContent>
                                </Dialog>
                            </Form>

                            <!-- Custom Role Button -->
                            <Form
                                v-slot="{ handleSubmit }"
                                as=""
                                keep-values
                                :validation-schema="roleFormSchema"
                                :initial-values="{
                                    name: '',
                                    permissions: [],
                                    is_admin: true,
                                }"
                            >
                                <Dialog v-model:open="showAddRoleDialog">
                                    <DialogTrigger asChild>
                                        <Button class="w-full sm:w-auto text-sm sm:text-base">Custom Role</Button>
                                    </DialogTrigger>
                                    <DialogContent
                                        class="w-[95vw] sm:max-w-[700px] max-h-[85vh] sm:max-h-[80vh] overflow-y-auto p-4 sm:p-6"
                                    >
                                        <DialogHeader>
                                            <DialogTitle class="text-base sm:text-lg">Create Custom Role</DialogTitle>
                                            <DialogDescription class="text-xs sm:text-sm">
                                                Create a new role with custom permissions. Permissions are grouped by category.
                                            </DialogDescription>
                                        </DialogHeader>
                                        <form
                                            id="addRoleDialogForm"
                                            @submit="handleSubmit($event, onRoleSubmit)"
                                        >
                                            <div class="grid gap-4 py-4">
                                                <FormField
                                                    name="name"
                                                    v-slot="{ componentField }"
                                                >
                                                    <FormItem>
                                                        <FormLabel>Role Name</FormLabel>
                                                        <FormControl>
                                                            <Input
                                                                placeholder="e.g., SeniorAdmin"
                                                                v-bind="componentField"
                                                            />
                                                        </FormControl>
                                                        <FormDescription>
                                                            Role name can only contain letters, numbers, and underscores.
                                                        </FormDescription>
                                                        <FormMessage />
                                                    </FormItem>
                                                </FormField>

                                                <FormField name="permissions">
                                                    <FormItem>
                                                        <FormLabel>Permissions</FormLabel>
                                                        <FormDescription>
                                                            Select permissions for this role. Permissions are organized by category.
                                                        </FormDescription>

                                                        <div v-if="loading.permissions" class="text-center py-4">
                                                            <div class="animate-spin h-6 w-6 border-2 border-primary border-t-transparent rounded-full mx-auto"></div>
                                                        </div>

                                                        <div v-else class="space-y-4 mt-2">
                                                            <div
                                                                v-for="group in permissions"
                                                                :key="group.category"
                                                                class="border rounded-md p-3"
                                                            >
                                                                <h3 class="font-medium mb-2 flex items-center gap-2">
                                                                    <Badge :class="getPermissionCategoryColor(group.category + ':')">
                                                                        {{ group.category_name }}
                                                                    </Badge>
                                                                </h3>
                                                                <div class="grid grid-cols-1 sm:grid-cols-2 gap-2">
                                                                    <div
                                                                        v-for="permission in group.permissions"
                                                                        :key="permission.code"
                                                                        class="flex items-start space-x-2"
                                                                    >
                                                                        <FormField
                                                                            v-slot="{ value, handleChange }"
                                                                            type="checkbox"
                                                                            :value="permission.code"
                                                                            :unchecked-value="false"
                                                                            name="permissions"
                                                                        >
                                                                            <FormItem class="flex flex-row items-start space-x-3 space-y-0">
                                                                                <FormControl>
                                                                                    <Checkbox
                                                                                        :modelValue="Array.isArray(value) && value.includes(permission.code)"
                                                                                        @update:modelValue="handleChange"
                                                                                    />
                                                                                </FormControl>
                                                                                <div class="leading-none">
                                                                                    <FormLabel class="font-normal text-sm">
                                                                                        {{ permission.name }}
                                                                                    </FormLabel>
                                                                                    <p class="text-xs text-muted-foreground">
                                                                                        {{ permission.description }}
                                                                                    </p>
                                                                                </div>
                                                                            </FormItem>
                                                                        </FormField>
                                                                    </div>
                                                                </div>
                                                            </div>
                                                        </div>

                                                        <FormMessage />
                                                    </FormItem>
                                                </FormField>

                                                <FormField
                                                    name="is_admin"
                                                    v-slot="{ value, handleChange }"
                                                >
                                                    <FormItem class="flex flex-row items-start space-x-3 space-y-0">
                                                        <FormControl>
                                                            <Checkbox
                                                                :modelValue="value"
                                                                @update:modelValue="handleChange"
                                                            />
                                                        </FormControl>
                                                        <div class="space-y-1 leading-none">
                                                            <FormLabel>
                                                                Is Admin Role
                                                            </FormLabel>
                                                            <FormDescription>
                                                                If checked, this role will be included when plugins or workflows fetch "admins" (e.g., for pings, admin lists). Uncheck for non-management roles like "Reserved" or "VIP". All RCON roles appear in admin.cfg regardless of this setting.
                                                            </FormDescription>
                                                        </div>
                                                    </FormItem>
                                                </FormField>
                                            </div>
                                            <DialogFooter>
                                                <Button
                                                    type="button"
                                                    variant="outline"
                                                    @click="showAddRoleDialog = false"
                                                >
                                                    Cancel
                                                </Button>
                                                <Button
                                                    type="submit"
                                                    :disabled="addRoleLoading"
                                                >
                                                    {{ addRoleLoading ? "Adding..." : "Add Role" }}
                                                </Button>
                                            </DialogFooter>
                                        </form>
                                    </DialogContent>
                                </Dialog>
                            </Form>

                            <!-- Edit Role Dialog -->
                            <Form
                                :key="editingRole?.id"
                                :validation-schema="roleFormSchema"
                                :initial-values="{
                                    name: editingRole?.name || '',
                                    permissions: editingRole?.permissions || [],
                                    is_admin: editingRole?.is_admin ?? true,
                                }"
                                v-slot="{ handleSubmit }"
                            >
                                <Dialog v-model:open="showEditRoleDialog">
                                    <DialogContent
                                        class="w-[95vw] sm:max-w-[700px] max-h-[85vh] sm:max-h-[80vh] overflow-y-auto p-4 sm:p-6"
                                    >
                                        <DialogHeader>
                                            <DialogTitle class="text-base sm:text-lg">Edit Role</DialogTitle>
                                            <DialogDescription class="text-xs sm:text-sm">
                                                Update the role name, permissions, and admin status.
                                            </DialogDescription>
                                        </DialogHeader>
                                        <form
                                            id="editRoleDialogForm"
                                            @submit="handleSubmit($event, updateRole)"
                                        >
                                            <div class="grid gap-4 py-4">
                                                <FormField
                                                    name="name"
                                                    v-slot="{ componentField }"
                                                >
                                                    <FormItem>
                                                        <FormLabel>Role Name</FormLabel>
                                                        <FormControl>
                                                            <Input
                                                                placeholder="e.g., SeniorAdmin"
                                                                v-bind="componentField"
                                                            />
                                                        </FormControl>
                                                        <FormDescription>
                                                            Role name can only contain letters, numbers, and underscores.
                                                        </FormDescription>
                                                        <FormMessage />
                                                    </FormItem>
                                                </FormField>

                                                <FormField name="permissions">
                                                    <FormItem>
                                                        <FormLabel>Permissions</FormLabel>
                                                        <FormDescription>
                                                            Select permissions for this role. Permissions are organized by category.
                                                        </FormDescription>

                                                        <div v-if="loading.permissions" class="text-center py-4">
                                                            <div class="animate-spin h-6 w-6 border-2 border-primary border-t-transparent rounded-full mx-auto"></div>
                                                        </div>

                                                        <div v-else class="space-y-4 mt-2">
                                                            <div
                                                                v-for="group in permissions"
                                                                :key="group.category"
                                                                class="border rounded-md p-3"
                                                            >
                                                                <h3 class="font-medium mb-2 flex items-center gap-2">
                                                                    <Badge :class="getPermissionCategoryColor(group.category + ':')">
                                                                        {{ group.category_name }}
                                                                    </Badge>
                                                                </h3>
                                                                <div class="grid grid-cols-1 sm:grid-cols-2 gap-2">
                                                                    <div
                                                                        v-for="permission in group.permissions"
                                                                        :key="permission.code"
                                                                        class="flex items-start space-x-2"
                                                                    >
                                                                        <FormField
                                                                            v-slot="{ value, handleChange }"
                                                                            type="checkbox"
                                                                            :value="permission.code"
                                                                            :unchecked-value="false"
                                                                            name="permissions"
                                                                        >
                                                                            <FormItem class="flex flex-row items-start space-x-3 space-y-0">
                                                                                <FormControl>
                                                                                    <Checkbox
                                                                                        :modelValue="Array.isArray(value) && value.includes(permission.code)"
                                                                                        @update:modelValue="handleChange"
                                                                                    />
                                                                                </FormControl>
                                                                                <div class="leading-none">
                                                                                    <FormLabel class="font-normal text-sm">
                                                                                        {{ permission.name }}
                                                                                    </FormLabel>
                                                                                    <p class="text-xs text-muted-foreground">
                                                                                        {{ permission.description }}
                                                                                    </p>
                                                                                </div>
                                                                            </FormItem>
                                                                        </FormField>
                                                                    </div>
                                                                </div>
                                                            </div>
                                                        </div>

                                                        <FormMessage />
                                                    </FormItem>
                                                </FormField>

                                                <FormField
                                                    name="is_admin"
                                                    v-slot="{ value, handleChange }"
                                                >
                                                    <FormItem class="flex flex-row items-start space-x-3 space-y-0">
                                                        <FormControl>
                                                            <Checkbox
                                                                :modelValue="value"
                                                                @update:modelValue="handleChange"
                                                            />
                                                        </FormControl>
                                                        <div class="space-y-1 leading-none">
                                                            <FormLabel>
                                                                Is Admin Role
                                                            </FormLabel>
                                                            <FormDescription>
                                                                If checked, this role will be included when plugins or workflows fetch "admins" (e.g., for pings, admin lists). Uncheck for non-management roles. All RCON roles appear in admin.cfg regardless.
                                                            </FormDescription>
                                                        </div>
                                                    </FormItem>
                                                </FormField>
                                            </div>
                                            <DialogFooter>
                                                <Button
                                                    type="button"
                                                    variant="outline"
                                                    @click="closeEditRoleDialog"
                                                >
                                                    Cancel
                                                </Button>
                                                <Button
                                                    type="submit"
                                                    :disabled="editRoleLoading"
                                                >
                                                    {{ editRoleLoading ? "Updating..." : "Update Role" }}
                                                </Button>
                                            </DialogFooter>
                                        </form>
                                    </DialogContent>
                                </Dialog>
                            </Form>
                        </div>
                    </CardHeader>
                    <CardContent>
                        <div v-if="loading.roles" class="text-center py-6 sm:py-8">
                            <div
                                class="animate-spin h-8 w-8 border-4 border-primary border-t-transparent rounded-full mx-auto mb-4"
                            ></div>
                            <p class="text-sm sm:text-base">Loading roles...</p>
                        </div>

                        <div
                            v-else-if="roles.length === 0"
                            class="text-center py-6 sm:py-8"
                        >
                            <p class="text-sm sm:text-base">No roles found. Create a role to get started.</p>
                        </div>

                        <template v-else>
                            <!-- Desktop Table View -->
                            <div class="hidden md:block overflow-x-auto">
                                <Table>
                                    <TableHeader>
                                        <TableRow>
                                            <TableHead class="text-xs sm:text-sm">Role Name</TableHead>
                                            <TableHead class="text-xs sm:text-sm">Type</TableHead>
                                            <TableHead class="text-xs sm:text-sm">Permissions</TableHead>
                                            <TableHead class="text-xs sm:text-sm">Created At</TableHead>
                                            <TableHead class="text-right text-xs sm:text-sm">Actions</TableHead>
                                        </TableRow>
                                    </TableHeader>
                                    <TableBody>
                                        <TableRow
                                            v-for="role in roles"
                                            :key="role.id"
                                            class="hover:bg-muted/50"
                                        >
                                            <TableCell class="font-medium text-sm sm:text-base">
                                                {{ role.name }}
                                            </TableCell>
                                            <TableCell>
                                                <Badge
                                                    :variant="role.is_admin ? 'default' : 'secondary'"
                                                    class="text-xs"
                                                >
                                                    {{ role.is_admin ? 'Admin' : 'Non-Admin' }}
                                                </Badge>
                                            </TableCell>
                                            <TableCell>
                                                <div class="flex flex-wrap gap-1 max-w-md">
                                                    <Badge
                                                        v-for="permission in role.permissions.slice(0, 5)"
                                                        :key="permission"
                                                        variant="outline"
                                                        :class="['text-xs', getPermissionCategoryColor(permission)]"
                                                    >
                                                        {{ formatPermissionDisplay(permission) }}
                                                    </Badge>
                                                    <Badge
                                                        v-if="role.permissions.length > 5"
                                                        variant="outline"
                                                        class="text-xs"
                                                    >
                                                        +{{ role.permissions.length - 5 }} more
                                                    </Badge>
                                                </div>
                                            </TableCell>
                                            <TableCell class="text-xs sm:text-sm">
                                                {{ formatDate(role.created_at) }}
                                            </TableCell>
                                            <TableCell class="text-right">
                                                <div class="flex gap-2 justify-end">
                                                    <Button
                                                        variant="outline"
                                                        size="sm"
                                                        @click="openEditRoleDialog(role)"
                                                        :disabled="loading.roles"
                                                        class="text-xs"
                                                    >
                                                        Edit
                                                    </Button>
                                                    <Button
                                                        variant="destructive"
                                                        size="sm"
                                                        @click="removeRole(role.id)"
                                                        :disabled="loading.roles"
                                                        class="text-xs"
                                                    >
                                                        Remove
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
                                    v-for="role in roles"
                                    :key="role.id"
                                    class="border rounded-lg p-3 sm:p-4 hover:bg-muted/30 transition-colors"
                                >
                                    <div class="flex items-start justify-between gap-2 mb-2">
                                        <div class="flex-1 min-w-0">
                                            <h3 class="font-semibold text-sm sm:text-base mb-1">{{ role.name }}</h3>
                                            <div class="flex items-center gap-2 mb-1">
                                                <Badge
                                                    :variant="role.is_admin ? 'default' : 'secondary'"
                                                    class="text-xs"
                                                >
                                                    {{ role.is_admin ? 'Admin' : 'Non-Admin' }}
                                                </Badge>
                                            </div>
                                            <p class="text-xs text-muted-foreground">
                                                Created: {{ formatDate(role.created_at) }}
                                            </p>
                                        </div>
                                        <div class="flex flex-col gap-1 flex-shrink-0">
                                            <Button
                                                variant="outline"
                                                size="sm"
                                                @click="openEditRoleDialog(role)"
                                                :disabled="loading.roles"
                                                class="h-8 text-xs"
                                            >
                                                Edit
                                            </Button>
                                            <Button
                                                variant="destructive"
                                                size="sm"
                                                @click="removeRole(role.id)"
                                                :disabled="loading.roles"
                                                class="h-8 text-xs"
                                            >
                                                Remove
                                            </Button>
                                        </div>
                                    </div>
                                    <div class="flex flex-wrap gap-1 mt-2">
                                        <Badge
                                            v-for="permission in role.permissions.slice(0, 4)"
                                            :key="permission"
                                            variant="outline"
                                            :class="['text-xs', getPermissionCategoryColor(permission)]"
                                        >
                                            {{ formatPermissionDisplay(permission) }}
                                        </Badge>
                                        <Badge
                                            v-if="role.permissions.length > 4"
                                            variant="outline"
                                            class="text-xs"
                                        >
                                            +{{ role.permissions.length - 4 }} more
                                        </Badge>
                                    </div>
                                </div>
                            </div>
                        </template>
                    </CardContent>
                </Card>
            </TabsContent>

            <!-- Admins Tab -->
            <TabsContent value="admins">
                <Card>
                    <CardHeader
                        class="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-3 sm:gap-0 pb-2 sm:pb-3"
                    >
                        <CardTitle class="text-base sm:text-lg">Admin Management</CardTitle>
                        <div class="flex flex-col sm:flex-row gap-2 w-full sm:w-auto">
                            <Button
                                variant="outline"
                                size="sm"
                                @click="cleanupExpiredAdmins"
                                :disabled="cleanupLoading"
                                class="w-full sm:w-auto text-xs sm:text-sm"
                            >
                                {{ cleanupLoading ? "Cleaning..." : "Cleanup Expired" }}
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
                                        <Button class="w-full sm:w-auto text-sm sm:text-base">Add Admin</Button>
                                    </DialogTrigger>
                                    <DialogContent
                                        class="w-[95vw] sm:max-w-[425px] max-h-[85vh] sm:max-h-[80vh] overflow-y-auto p-4 sm:p-6"
                                    >
                                        <DialogHeader>
                                            <DialogTitle class="text-base sm:text-lg">Add New Admin</DialogTitle>
                                            <DialogDescription class="text-xs sm:text-sm">
                                                Assign a user as an admin with a specific role.
                                            </DialogDescription>
                                        </DialogHeader>
                                        <form
                                            id="addAdminDialogForm"
                                            @submit="handleSubmit($event, addAdmin)"
                                        >
                                            <div class="grid gap-4 py-4">
                                                <FormField
                                                    name="adminType"
                                                    v-slot="{ componentField }"
                                                >
                                                    <FormItem>
                                                        <FormLabel>Admin Type</FormLabel>
                                                        <Select
                                                            v-bind="componentField"
                                                            @update:model-value="(value: any) => (selectedAdminType = value || 'user')"
                                                        >
                                                            <FormControl>
                                                                <SelectTrigger>
                                                                    <SelectValue placeholder="Select admin type" />
                                                                </SelectTrigger>
                                                            </FormControl>
                                                            <SelectContent>
                                                                <SelectGroup>
                                                                    <SelectItem value="user">Existing User</SelectItem>
                                                                    <SelectItem value="steam_id">Steam ID Only</SelectItem>
                                                                </SelectGroup>
                                                            </SelectContent>
                                                        </Select>
                                                        <FormDescription>
                                                            Choose whether to assign an existing user or add an admin by Steam ID only.
                                                        </FormDescription>
                                                        <FormMessage />
                                                    </FormItem>
                                                </FormField>

                                                <FormField
                                                    name="user_id"
                                                    v-slot="{ componentField }"
                                                >
                                                    <FormItem v-if="selectedAdminType === 'user'">
                                                        <FormLabel>User</FormLabel>
                                                        <Select v-bind="componentField">
                                                            <FormControl>
                                                                <SelectTrigger>
                                                                    <SelectValue placeholder="Select user" />
                                                                </SelectTrigger>
                                                            </FormControl>
                                                            <SelectContent>
                                                                <SelectGroup>
                                                                    <SelectItem
                                                                        v-for="user in filteredUsersForSelection"
                                                                        :key="user.id"
                                                                        :value="user.id"
                                                                    >
                                                                        {{ user.name }} ({{ user.username }})
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
                                                    <FormItem v-if="selectedAdminType === 'steam_id'">
                                                        <FormLabel>Steam ID</FormLabel>
                                                        <FormControl>
                                                            <Input
                                                                placeholder="76561198012345678"
                                                                v-bind="componentField"
                                                            />
                                                        </FormControl>
                                                        <FormDescription>
                                                            Enter the 17-digit Steam ID of the player you want to make an admin.
                                                        </FormDescription>
                                                        <FormMessage />
                                                    </FormItem>
                                                </FormField>

                                                <FormField
                                                    name="server_role_id"
                                                    v-slot="{ componentField }"
                                                >
                                                    <FormItem>
                                                        <FormLabel>Role</FormLabel>
                                                        <Select v-bind="componentField">
                                                            <FormControl>
                                                                <SelectTrigger>
                                                                    <SelectValue placeholder="Select role" />
                                                                </SelectTrigger>
                                                            </FormControl>
                                                            <SelectContent>
                                                                <SelectGroup>
                                                                    <SelectItem
                                                                        v-for="role in roles"
                                                                        :key="role.id"
                                                                        :value="role.id"
                                                                    >
                                                                        <div class="flex items-center gap-2">
                                                                            <span>{{ role.name }}</span>
                                                                            <Badge
                                                                                v-if="!role.is_admin"
                                                                                variant="secondary"
                                                                                class="text-xs"
                                                                            >
                                                                                Non-Admin
                                                                            </Badge>
                                                                        </div>
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
                                                        <FormLabel>Expiration Date (Optional)</FormLabel>
                                                        <FormControl>
                                                            <Input
                                                                type="datetime-local"
                                                                v-bind="componentField"
                                                            />
                                                        </FormControl>
                                                        <FormDescription>
                                                            Leave empty for permanent admin access. Set a date for temporary access.
                                                        </FormDescription>
                                                        <FormMessage />
                                                    </FormItem>
                                                </FormField>

                                                <FormField
                                                    name="notes"
                                                    v-slot="{ componentField }"
                                                >
                                                    <FormItem>
                                                        <FormLabel>Notes (Optional)</FormLabel>
                                                        <FormControl>
                                                            <Input
                                                                placeholder="e.g., Trial period, VIP supporter, etc."
                                                                v-bind="componentField"
                                                            />
                                                        </FormControl>
                                                        <FormDescription>
                                                            Add notes about this admin assignment for future reference.
                                                        </FormDescription>
                                                        <FormMessage />
                                                    </FormItem>
                                                </FormField>
                                            </div>
                                            <DialogFooter>
                                                <Button
                                                    type="button"
                                                    variant="outline"
                                                    @click="showAddAdminDialog = false"
                                                >
                                                    Cancel
                                                </Button>
                                                <Button
                                                    type="submit"
                                                    :disabled="addAdminLoading"
                                                >
                                                    {{ addAdminLoading ? "Adding..." : "Add Admin" }}
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
                                    <DialogContent class="w-[95vw] sm:max-w-[500px] p-4 sm:p-6">
                                        <DialogHeader>
                                            <DialogTitle class="text-base sm:text-lg">Edit Admin Notes</DialogTitle>
                                            <DialogDescription class="text-xs sm:text-sm">
                                                Update the notes for this admin assignment.
                                            </DialogDescription>
                                        </DialogHeader>
                                        <form @submit="handleSubmit($event, updateAdminNotes)">
                                            <div class="grid gap-4 py-4">
                                                <FormField
                                                    v-slot="{ componentField }"
                                                    name="notes"
                                                >
                                                    <FormItem>
                                                        <FormLabel>Notes</FormLabel>
                                                        <FormControl>
                                                            <Textarea
                                                                v-bind="componentField"
                                                                placeholder="Add notes about this admin assignment..."
                                                                rows="4"
                                                            />
                                                        </FormControl>
                                                        <FormDescription>
                                                            Add any notes or context about this admin assignment.
                                                        </FormDescription>
                                                        <FormMessage />
                                                    </FormItem>
                                                </FormField>
                                            </div>
                                            <DialogFooter>
                                                <Button
                                                    type="button"
                                                    variant="outline"
                                                    @click="closeEditAdminDialog"
                                                >
                                                    Cancel
                                                </Button>
                                                <Button
                                                    type="submit"
                                                    :disabled="editAdminLoading"
                                                >
                                                    {{ editAdminLoading ? "Updating..." : "Update Notes" }}
                                                </Button>
                                            </DialogFooter>
                                        </form>
                                    </DialogContent>
                                </Dialog>
                            </Form>
                        </div>
                    </CardHeader>
                    <CardContent>
                        <div v-if="loading.admins" class="text-center py-6 sm:py-8">
                            <div
                                class="animate-spin h-8 w-8 border-4 border-primary border-t-transparent rounded-full mx-auto mb-4"
                            ></div>
                            <p class="text-sm sm:text-base">Loading admins...</p>
                        </div>

                        <div
                            v-else-if="admins.length === 0"
                            class="text-center py-6 sm:py-8"
                        >
                            <p class="text-sm sm:text-base">No admins found. Add an admin to get started.</p>
                        </div>

                        <template v-else>
                            <!-- Desktop Table View -->
                            <div class="hidden md:block overflow-x-auto">
                                <Table>
                                    <TableHeader>
                                        <TableRow>
                                            <TableHead class="text-xs sm:text-sm">Admin</TableHead>
                                            <TableHead class="text-xs sm:text-sm">Role</TableHead>
                                            <TableHead class="text-xs sm:text-sm">Status</TableHead>
                                            <TableHead class="text-xs sm:text-sm">Notes</TableHead>
                                            <TableHead class="text-xs sm:text-sm">Created At</TableHead>
                                            <TableHead class="text-right text-xs sm:text-sm">Actions</TableHead>
                                        </TableRow>
                                    </TableHeader>
                                    <TableBody>
                                        <TableRow
                                            v-for="admin in admins"
                                            :key="admin.id"
                                            class="hover:bg-muted/50"
                                        >
                                            <TableCell class="font-medium text-sm sm:text-base">
                                                {{ getAdminDisplayName(admin) }}
                                            </TableCell>
                                            <TableCell>
                                                <Badge variant="outline" class="text-xs">
                                                    {{ getRoleName(admin.server_role_id) }}
                                                </Badge>
                                            </TableCell>
                                            <TableCell>
                                                <Badge
                                                    :variant="formatExpirationStatus(admin).variant"
                                                    :class="formatExpirationStatus(admin).class"
                                                    class="text-xs"
                                                >
                                                    {{ formatExpirationStatus(admin).text }}
                                                </Badge>
                                            </TableCell>
                                            <TableCell class="text-xs sm:text-sm max-w-[200px] truncate">
                                                {{ admin.notes || '-' }}
                                            </TableCell>
                                            <TableCell class="text-xs sm:text-sm">
                                                {{ formatDate(admin.created_at) }}
                                            </TableCell>
                                            <TableCell class="text-right">
                                                <div class="flex gap-2 justify-end">
                                                    <Button
                                                        variant="outline"
                                                        size="sm"
                                                        @click="openEditAdminDialog(admin)"
                                                        :disabled="loading.admins"
                                                        class="text-xs"
                                                    >
                                                        Edit
                                                    </Button>
                                                    <Button
                                                        variant="destructive"
                                                        size="sm"
                                                        @click="removeAdmin(admin.id)"
                                                        :disabled="loading.admins"
                                                        class="text-xs"
                                                    >
                                                        Remove
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
                                    v-for="admin in admins"
                                    :key="admin.id"
                                    class="border rounded-lg p-3 sm:p-4 hover:bg-muted/30 transition-colors"
                                >
                                    <div class="flex items-start justify-between gap-2 mb-2">
                                        <div class="flex-1 min-w-0">
                                            <h3 class="font-semibold text-sm sm:text-base mb-1">
                                                {{ getAdminDisplayName(admin) }}
                                            </h3>
                                            <div class="flex flex-wrap items-center gap-2 mb-1">
                                                <Badge variant="outline" class="text-xs">
                                                    {{ getRoleName(admin.server_role_id) }}
                                                </Badge>
                                                <Badge
                                                    :variant="formatExpirationStatus(admin).variant"
                                                    :class="formatExpirationStatus(admin).class"
                                                    class="text-xs"
                                                >
                                                    {{ formatExpirationStatus(admin).text }}
                                                </Badge>
                                            </div>
                                            <p class="text-xs text-muted-foreground">
                                                Created: {{ formatDate(admin.created_at) }}
                                            </p>
                                            <p v-if="admin.notes" class="text-xs text-muted-foreground mt-1 truncate">
                                                Notes: {{ admin.notes }}
                                            </p>
                                        </div>
                                        <div class="flex flex-col gap-1 flex-shrink-0">
                                            <Button
                                                variant="outline"
                                                size="sm"
                                                @click="openEditAdminDialog(admin)"
                                                :disabled="loading.admins"
                                                class="h-8 text-xs"
                                            >
                                                Edit
                                            </Button>
                                            <Button
                                                variant="destructive"
                                                size="sm"
                                                @click="removeAdmin(admin.id)"
                                                :disabled="loading.admins"
                                                class="h-8 text-xs"
                                            >
                                                Remove
                                            </Button>
                                        </div>
                                    </div>
                                </div>
                            </div>
                        </template>
                    </CardContent>
                </Card>
            </TabsContent>
        </Tabs>
    </div>
</template>
