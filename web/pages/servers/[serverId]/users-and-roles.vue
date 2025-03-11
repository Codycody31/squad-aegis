<script setup lang="ts">
import { ref, onMounted, computed } from "vue";
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
const addRoleLoading = ref(false);
const addAdminLoading = ref(false);

// Squad permission categories
const permissionCategories = [
  {
    name: "Basic Admin",
    permissions: ["admin", "reserve", "balance", "canseeadminchat"],
  },
  {
    name: "Chat",
    permissions: ["chat", "cameraman"],
  },
  {
    name: "Kick & Ban",
    permissions: ["kick", "ban", "teamchange", "forceteamchange", "immunity"],
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
  createdAt: string;
}

interface ServerAdmin {
  id: string;
  serverId: string;
  name: string;
  userId: string;
  username: string;
  serverRoleId: string;
  roleName: string;
  createdAt: string;
}

interface User {
  id: string;
  username: string;
  email: string;
  superAdmin: boolean;
  createdAt: string;
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
        "Role name can only contain letters, numbers, and underscores. No spaces or special characters allowed."
      ),
    permissions: z
      .array(z.string())
      .min(1, "At least one permission is required"),
  })
);

const adminFormSchema = toTypedSchema(
  z.object({
    userId: z.string().min(1, "User is required"),
    serverRoleId: z.string().min(1, "Role is required"),
  })
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
    userId: "",
    serverRoleId: "",
  },
});

// Function to fetch roles
async function fetchRoles() {
  loading.value.roles = true;
  error.value = null;

  const runtimeConfig = useRuntimeConfig();
  const cookieToken = useCookie(
    runtimeConfig.public.sessionCookieName as string
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
      }
    );

    if (fetchError.value) {
      throw new Error(fetchError.value.message || "Failed to fetch roles");
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
    runtimeConfig.public.sessionCookieName as string
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
      }
    );

    if (fetchError.value) {
      throw new Error(fetchError.value.message || "Failed to fetch admins");
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
    runtimeConfig.public.sessionCookieName as string
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
      }
    );

    if (fetchError.value) {
      throw new Error(fetchError.value.message || "Failed to fetch users");
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
    runtimeConfig.public.sessionCookieName as string
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
      }
    );

    if (fetchError.value) {
      throw new Error(
        fetchError.value.data.message || fetchError.value.message
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
      "Are you sure you want to remove this role? This will not affect existing admins with this role."
    )
  ) {
    return;
  }

  loading.value.roles = true;
  error.value = null;

  const runtimeConfig = useRuntimeConfig();
  const cookieToken = useCookie(
    runtimeConfig.public.sessionCookieName as string
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
      }
    );

    if (fetchError.value) {
      throw new Error(
        fetchError.value.data.message || fetchError.value.message
      );
    }

    // Refresh the roles list
    fetchRoles();
  } catch (err: any) {
    error.value = err.message || "An error occurred while removing the role";
    console.error(err);
  } finally {
    loading.value.roles = false;
  }
}

// Function to add an admin
async function addAdmin(values: any) {
  const { name, userId, serverRoleId } = values;

  addAdminLoading.value = true;
  error.value = null;

  const runtimeConfig = useRuntimeConfig();
  const cookieToken = useCookie(
    runtimeConfig.public.sessionCookieName as string
  );
  const token = cookieToken.value;

  if (!token) {
    error.value = "Authentication required";
    addAdminLoading.value = false;
    return;
  }

  try {
    const { data, error: fetchError } = await useFetch(
      `${runtimeConfig.public.backendApi}/servers/${serverId}/admins`,
      {
        method: "POST",
        body: {
          name,
          userId,
          serverRoleId,
        },
        headers: {
          Authorization: `Bearer ${token}`,
        },
      }
    );

    if (fetchError.value) {
      throw new Error(
        fetchError.value.data.message || fetchError.value.message
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
    runtimeConfig.public.sessionCookieName as string
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
      }
    );

    if (fetchError.value) {
      throw new Error(
        fetchError.value.data.message || fetchError.value.message
      );
    }

    // Refresh the admins list
    fetchAdmins();
  } catch (err: any) {
    error.value = err.message || "An error occurred while removing the admin";
    console.error(err);
  } finally {
    loading.value.admins = false;
  }
}

function copyAdminCfgUrl() {
  const runtimeConfig = useRuntimeConfig();
  navigator.clipboard.writeText(
    `${runtimeConfig.public.backendApi}/servers/${serverId}/admins/cfg`
  );
}

// Format date
function formatDate(dateString: string): string {
  return new Date(dateString).toLocaleString();
}

// Get available users (not already admins)
const availableUsers = computed(() => {
  const adminUserIds = admins.value.map((admin) => admin.userId);
  return users.value.filter((user) => !adminUserIds.includes(user.id));
});

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
          <CardHeader class="flex flex-row items-center justify-between pb-2">
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
                      Create a new role with specific permissions for Squad
                      server administration.
                    </DialogDescription>
                  </DialogHeader>
                  <form
                    id="addRoleDialogForm"
                    @submit="handleSubmit($event, onRoleSubmit)"
                  >
                    <div class="grid gap-4 py-4">
                      <FormField name="name" v-slot="{ componentField }">
                        <FormItem>
                          <FormLabel>Role Name</FormLabel>
                          <FormControl>
                            <Input
                              placeholder="e.g., SeniorAdmin"
                              v-bind="componentField"
                            />
                          </FormControl>
                          <FormDescription>
                            Role name can only contain letters, numbers, and
                            underscores. No spaces or special characters
                            allowed.
                          </FormDescription>
                          <FormMessage />
                        </FormItem>
                      </FormField>

                      <FormField name="permissions">
                        <FormItem>
                          <FormLabel>Permissions</FormLabel>
                          <FormDescription>
                            Select the permissions for this role. Each
                            permission grants access to specific Squad admin
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
                              <h3 class="font-medium mb-2">
                                {{ category.name }}
                              </h3>
                              <div class="space-y-2">
                                <div
                                  v-for="permission in category.permissions"
                                  :key="permission"
                                  class="flex items-center space-x-2"
                                >
                                  <FormField
                                    v-slot="{ value, handleChange }"
                                    :key="permission"
                                    type="checkbox"
                                    :value="permission"
                                    :unchecked-value="false"
                                    name="permissions"
                                  >
                                    <FormItem
                                      class="flex flex-row items-start space-x-3 space-y-0"
                                    >
                                      <FormControl>
                                        <Checkbox
                                          :model-value="
                                            value.includes(permission)
                                          "
                                          @update:model-value="handleChange"
                                        />
                                      </FormControl>
                                      <FormLabel class="font-normal">
                                        {{ permission }}
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
                        @click="showAddRoleDialog = false"
                      >
                        Cancel
                      </Button>
                      <Button type="submit" :disabled="addRoleLoading">
                        {{ addRoleLoading ? "Adding..." : "Add Role" }}
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

            <div v-else-if="roles.length === 0" class="text-center py-8">
              <p>No roles found. Create a role to get started.</p>
            </div>

            <div v-else class="overflow-x-auto">
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>Role Name</TableHead>
                    <TableHead>Permissions</TableHead>
                    <TableHead>Created At</TableHead>
                    <TableHead class="text-right">Actions</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  <TableRow
                    v-for="role in roles"
                    :key="role.id"
                    class="hover:bg-muted/50"
                  >
                    <TableCell class="font-medium">{{ role.name }}</TableCell>
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
                    <TableCell>{{ formatDate(role.createdAt) }}</TableCell>
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
          <CardHeader class="flex flex-row items-center justify-between pb-2">
            <CardTitle>Admin Management</CardTitle>
            <Form
              v-slot="{ handleSubmit }"
              as=""
              keep-values
              :validation-schema="adminFormSchema"
              :initial-values="{
                name: '',
                userId: '',
                serverRoleId: '',
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
                    <DialogTitle>Add New Admin</DialogTitle>
                    <DialogDescription>
                      Assign a user as an admin with a specific role.
                    </DialogDescription>
                  </DialogHeader>
                  <form
                    id="addAdminDialogForm"
                    @submit="handleSubmit($event, addAdmin)"
                  >
                    <div class="grid gap-4 py-4">
                      <FormField name="userId" v-slot="{ componentField }">
                        <FormItem>
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
                                  v-for="user in availableUsers"
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
                        name="serverRoleId"
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
                                  {{ role.name }}
                                </SelectItem>
                              </SelectGroup>
                            </SelectContent>
                          </Select>
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
                      <Button type="submit" :disabled="addAdminLoading">
                        {{ addAdminLoading ? "Adding..." : "Add Admin" }}
                      </Button>
                    </DialogFooter>
                  </form>
                </DialogContent>
              </Dialog>
            </Form>
          </CardHeader>
          <CardContent>
            <div v-if="loading.admins" class="text-center py-8">
              <div
                class="animate-spin h-8 w-8 border-4 border-primary border-t-transparent rounded-full mx-auto mb-4"
              ></div>
              <p>Loading admins...</p>
            </div>

            <div v-else-if="admins.length === 0" class="text-center py-8">
              <p>No admins found. Add an admin to get started.</p>
            </div>

            <div v-else class="overflow-x-auto">
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>User</TableHead>
                    <TableHead>Role</TableHead>
                    <TableHead>Created At</TableHead>
                    <TableHead class="text-right">Actions</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  <TableRow
                    v-for="admin in admins"
                    :key="admin.id"
                    class="hover:bg-muted/50"
                  >
                    <TableCell>{{ admin.username }}</TableCell>
                    <TableCell>
                      <Badge variant="outline">
                        {{ admin.roleName }}
                      </Badge>
                    </TableCell>
                    <TableCell>{{ formatDate(admin.createdAt) }}</TableCell>
                    <TableCell class="text-right">
                      <Button
                        variant="destructive"
                        size="sm"
                        @click="removeAdmin(admin.id)"
                        :disabled="loading.admins"
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
    </Tabs>

    <Card class="mt-6">
      <CardHeader>
        <CardTitle>About Users & Roles</CardTitle>
      </CardHeader>
      <CardContent>
        <p class="text-sm text-muted-foreground">
          This page allows you to manage roles and admins for your Squad server.
          Roles define what permissions an admin has, and admins are users
          assigned to those roles.
        </p>
        <p class="text-sm text-muted-foreground mt-2">
          <strong>Roles</strong> - Create roles with specific permissions that
          determine what actions admins can perform on the server.
        </p>
        <p class="text-sm text-muted-foreground mt-2">
          <strong>Admins</strong> - Assign users to roles, giving them admin
          privileges on your server.
        </p>
        <p class="text-sm text-muted-foreground mt-2">
          <strong>Admin Config</strong> - Download the admin configuration file
          to use with your Squad server.
        </p>
        <div class="mt-4">
          <h3 class="font-medium mb-2">Permission Categories</h3>
          <div class="grid grid-cols-1 md:grid-cols-3 gap-4">
            <div
              v-for="category in permissionCategories"
              :key="category.name"
              class="border rounded-md p-3"
            >
              <h4 class="font-medium mb-1">{{ category.name }}</h4>
              <ul class="text-sm text-muted-foreground list-disc list-inside">
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
