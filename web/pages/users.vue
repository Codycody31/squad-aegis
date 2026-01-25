<script setup lang="ts">
import { ref, onMounted, onUnmounted, computed, nextTick, watch } from "vue";
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
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "~/components/ui/select";
import { Checkbox } from "~/components/ui/checkbox";
import { useForm } from "vee-validate";
import { toTypedSchema } from "@vee-validate/zod";
import * as z from "zod";
import { useAuthStore } from "@/stores/auth";

const authStore = useAuthStore();
const runtimeConfig = useRuntimeConfig();
const loading = ref(true);
const error = ref<string | null>(null);
const users = ref<User[]>([]);
const refreshInterval = ref<NodeJS.Timeout | null>(null);
const searchQuery = ref("");
const showAddUserDialog = ref(false);
const showEditUserDialog = ref(false);
const addUserLoading = ref(false);
const editUserLoading = ref(false);
const editingUser = ref<User | null>(null);

interface User {
  id: string;
  steam_id: string;
  name: string;
  username: string;
  super_admin: boolean;
  created_at: string;
  updated_at: string;
}

interface UsersResponse {
  data: {
    users: User[];
  };
}

// Form schema for adding a user
const addFormSchema = toTypedSchema(
  z.object({
    steam_id: z
      .string()
      .min(1, "Steam ID is required")
      .regex(/^\d+$/, "Steam ID must contain only numbers"),
    name: z.string().min(1, "Name is required"),
    username: z
      .string()
      .min(1, "Username is required")
      .regex(
        /^[a-z0-9_]{1,32}$/,
        "Username must be 1-32 characters long, all lowercase, and only contain a-z, 0-9, and _"
      ),
    password: z.string().min(8, "Password must be at least 8 characters"),
    superAdmin: z.boolean().default(false),
  })
);

// Form schema for editing a user
const editFormSchema = toTypedSchema(
  z.object({
    steam_id: z
      .string()
      .optional()
      .refine((val) => !val || val === "" || /^\d{17}$/.test(val), {
        message: "Steam ID must be exactly 17 digits",
      }),
    name: z.string().min(1, "Name is required"),
    superAdmin: z.boolean().default(false),
  })
);

// Setup forms
const addForm = useForm({
  validationSchema: addFormSchema,
  initialValues: {
    steam_id: "",
    name: "",
    username: "",
    password: "",
    superAdmin: false,
  },
});

const editForm = useForm({
  validationSchema: editFormSchema,
  initialValues: {
    steam_id: "",
    name: "",
    superAdmin: false,
  },
});

// Computed property for filtered users
const filteredUsers = computed(() => {
  if (!searchQuery.value.trim()) {
    return users.value;
  }

  const query = searchQuery.value.toLowerCase();
  return users.value.filter(
    (user) =>
      user.name.toLowerCase().includes(query) ||
      user.username.toLowerCase().includes(query) ||
      user.steam_id.toString().includes(query)
  );
});

// Function to fetch users data
async function fetchUsers() {
  loading.value = true;
  error.value = null;

  try {
    const { data, error: fetchError } = await useAuthFetch<UsersResponse>(
      `${runtimeConfig.public.backendApi}/users`,
      {
        method: "GET",
      }
    );

    if (fetchError.value) {
      throw new Error(fetchError.value.message || "Failed to fetch users data");
    }

    if (data.value && data.value.data) {
      users.value = data.value.data.users || [];

      // Sort by creation date (most recent first)
      users.value.sort((a, b) => {
        return (
          new Date(b.created_at).getTime() - new Date(a.created_at).getTime()
        );
      });
    }
  } catch (err: any) {
    error.value = err.message || "An error occurred while fetching users data";
    console.error(err);
  } finally {
    loading.value = false;
  }
}

// Function to add a user
async function addUser(values: any) {
  const { steam_id, name, username, password, superAdmin } = values;

  addUserLoading.value = true;
  error.value = null;

  try {
    const { data, error: fetchError } = await useAuthFetch(
      `${runtimeConfig.public.backendApi}/users`,
      {
        method: "POST",
        body: {
          steam_id,
          name,
          username,
          password,
          super_admin: superAdmin,
        },
      }
    );

    if (fetchError.value) {
      throw new Error(fetchError.value.message || "Failed to add user");
    }

    // Reset form and close dialog
    addForm.resetForm();
    showAddUserDialog.value = false;

    // Refresh the users list
    fetchUsers();
  } catch (err: any) {
    error.value = err.message || "An error occurred while adding the user";
    console.error(err);
  } finally {
    addUserLoading.value = false;
  }
}

// Function to open edit dialog
function openEditDialog(user: User) {
  editingUser.value = user;
  
  // Set the form values directly
  editForm.setFieldValue('name', user.name);
  editForm.setFieldValue('steam_id', user.steam_id);
  editForm.setFieldValue('superAdmin', user.super_admin);
  
  showEditUserDialog.value = true;
}

// Function to edit a user
async function editUser(values: any) {
  if (!editingUser.value) return;

  const { steam_id, name, superAdmin } = values;

  editUserLoading.value = true;
  error.value = null;

  try {
    const { data, error: fetchError } = await useAuthFetch(
      `${runtimeConfig.public.backendApi}/users/${editingUser.value.id}`,
      {
        method: "PUT",
        body: {
          steam_id,
          name,
          super_admin: superAdmin,
        },
      }
    );

    if (fetchError.value) {
      throw new Error(fetchError.value.message || "Failed to update user");
    }

    // Reset form and close dialog
    editForm.resetForm();
    showEditUserDialog.value = false;
    editingUser.value = null;

    // Refresh the users list
    fetchUsers();
  } catch (err: any) {
    error.value = err.message || "An error occurred while updating the user";
    console.error(err);
  } finally {
    editUserLoading.value = false;
  }
}

// Function to delete a user
async function deleteUser(userId: string) {
  if (!confirm("Are you sure you want to delete this user?")) {
    return;
  }

  loading.value = true;
  error.value = null;

  try {
    const { data, error: fetchError } = await useAuthFetch(
      `${runtimeConfig.public.backendApi}/users/${userId}`,
      {
        method: "DELETE",
      }
    );

    if (fetchError.value) {
      throw new Error(fetchError.value.message || "Failed to delete user");
    }

    // Refresh the users list
    fetchUsers();
  } catch (err: any) {
    error.value = err.message || "An error occurred while deleting the user";
    console.error(err);
  } finally {
    loading.value = false;
  }
}

// Format date
function formatDate(dateString: string): string {
  return new Date(dateString).toLocaleString();
}

fetchUsers();

// Watch for dialog close to reset form
watch(showEditUserDialog, (isOpen) => {
  if (!isOpen) {
    editForm.resetForm();
    editingUser.value = null;
  }
});

// Manual refresh function
function refreshData() {
  fetchUsers();
}
</script>

<template>
  <div class="p-3 sm:p-4">
    <div class="flex flex-col sm:flex-row sm:justify-between sm:items-center gap-3 sm:gap-0 mb-3 sm:mb-4">
      <h1 class="text-xl sm:text-2xl font-bold">Users</h1>
      <div class="flex flex-col sm:flex-row gap-2 w-full sm:w-auto">
        <Form
          v-slot="{ handleSubmit }"
          as=""
          keep-values
          :validation-schema="addFormSchema"
          :initial-values="{
            steam_id: '',
            name: '',
            username: '',
            password: '',
            superAdmin: false,
          }"
        >
          <Dialog v-model:open="showAddUserDialog" v-if="authStore.user?.super_admin">
            <DialogTrigger asChild>
              <Button class="w-full sm:w-auto text-sm sm:text-base">Add User</Button>
            </DialogTrigger>
            <DialogContent
              class="w-[95vw] sm:max-w-[425px] max-h-[90vh] overflow-y-auto p-4 sm:p-6"
            >
              <DialogHeader>
                <DialogTitle class="text-base sm:text-lg">Add New User</DialogTitle>
                <DialogDescription class="text-xs sm:text-sm">
                  Enter the details of the user you want to add.
                </DialogDescription>
              </DialogHeader>
              <form id="dialogForm" @submit="handleSubmit($event, addUser)">
                <div class="grid gap-4 py-4">
                  <FormField name="steam_id" v-slot="{ componentField }">
                    <FormItem>
                      <FormLabel>Steam ID</FormLabel>
                      <FormControl>
                        <Input
                          placeholder="76561198012345678"
                          v-bind="componentField"
                        />
                      </FormControl>
                      <FormMessage />
                    </FormItem>
                  </FormField>
                  <FormField name="name" v-slot="{ componentField }">
                    <FormItem>
                      <FormLabel>Name</FormLabel>
                      <FormControl>
                        <Input placeholder="John Doe" v-bind="componentField" />
                      </FormControl>
                      <FormMessage />
                    </FormItem>
                  </FormField>
                  <FormField name="username" v-slot="{ componentField }">
                    <FormItem>
                      <FormLabel>Username</FormLabel>
                      <FormControl>
                        <Input placeholder="johndoe" v-bind="componentField" />
                      </FormControl>
                      <FormDescription>
                        Must be 1-32 characters, lowercase, and only contain
                        a-z, 0-9, and _
                      </FormDescription>
                      <FormMessage />
                    </FormItem>
                  </FormField>
                  <FormField name="password" v-slot="{ componentField }">
                    <FormItem>
                      <FormLabel>Password</FormLabel>
                      <FormControl>
                        <Input
                          type="password"
                          placeholder="********"
                          v-bind="componentField"
                        />
                      </FormControl>
                      <FormDescription>
                        Must be at least 8 characters
                      </FormDescription>
                      <FormMessage />
                    </FormItem>
                  </FormField>
                  <FormField name="superAdmin" v-slot="{ value, handleChange }">
                    <FormItem
                      class="flex flex-row items-start space-x-3 space-y-0 rounded-md border p-4"
                    >
                      <FormControl>
                        <Checkbox
                          :model-value="value"
                          @update:model-value="handleChange"
                        />
                      </FormControl>
                      <div class="space-y-1 leading-none">
                        <FormLabel>Super Admin</FormLabel>
                        <FormDescription>
                          Grant super admin privileges to this user
                        </FormDescription>
                        <FormMessage />
                      </div>
                    </FormItem>
                  </FormField>
                </div>
                <DialogFooter>
                  <Button
                    type="button"
                    variant="outline"
                    @click="showAddUserDialog = false"
                  >
                    Cancel
                  </Button>
                  <Button type="submit" :disabled="addUserLoading">
                    {{ addUserLoading ? "Adding..." : "Add User" }}
                  </Button>
                </DialogFooter>
              </form>
            </DialogContent>
          </Dialog>
        </Form>

        <!-- Edit User Dialog -->
        <Dialog v-model:open="showEditUserDialog">
          <DialogContent class="w-[95vw] sm:max-w-[425px] max-h-[90vh] overflow-y-auto p-4 sm:p-6">
            <DialogHeader>
              <DialogTitle class="text-base sm:text-lg">Edit User</DialogTitle>
              <DialogDescription class="text-xs sm:text-sm">
                Update the user's information.
              </DialogDescription>
            </DialogHeader>
            <form @submit.prevent="editUser(editForm.values)">
              <div class="grid gap-4 py-4">
                <div>
                  <label class="text-sm font-medium leading-none peer-disabled:cursor-not-allowed peer-disabled:opacity-70">
                    Name
                  </label>
                  <Input 
                    v-model="editForm.values.name"
                    @input="editForm.setFieldValue('name', $event.target.value)"
                    placeholder="John Doe" 
                    class="mt-1"
                  />
                  <p v-if="editForm.errors.value.name" class="text-sm text-red-500 mt-1">
                    {{ editForm.errors.value.name }}
                  </p>
                </div>
                
                <div>
                  <label class="text-sm font-medium leading-none peer-disabled:cursor-not-allowed peer-disabled:opacity-70">
                    Steam ID
                  </label>
                  <Input 
                    v-model="editForm.values.steam_id"
                    @input="editForm.setFieldValue('steam_id', $event.target.value.replace(/[^0-9]/g, ''))"
                    placeholder="76561198012345678" 
                    class="mt-1"
                  />
                  <p class="text-sm text-muted-foreground mt-1">
                    17-digit Steam ID (optional)
                  </p>
                  <p v-if="editForm.errors.value.steam_id" class="text-sm text-red-500 mt-1">
                    {{ editForm.errors.value.steam_id }}
                  </p>
                </div>
                
                <div class="flex flex-row items-start space-x-3 space-y-0 rounded-md border p-4">
                  <Checkbox
                    :model-value="editForm.values.superAdmin"
                    @update:model-value="editForm.setFieldValue('superAdmin', !editForm.values.superAdmin)"
                  />
                  <div class="space-y-1 leading-none">
                    <label class="text-sm font-medium leading-none peer-disabled:cursor-not-allowed peer-disabled:opacity-70">
                      Super Admin
                    </label>
                    <p class="text-sm text-muted-foreground">
                      Grant super admin privileges to this user
                    </p>
                  </div>
                </div>
              </div>
              <div class="flex justify-end space-x-2">
                <Button
                  type="button"
                  variant="outline"
                  @click="showEditUserDialog = false"
                >
                  Cancel
                </Button>
                <Button type="submit" :disabled="editUserLoading">
                  {{ editUserLoading ? "Updating..." : "Update User" }}
                </Button>
              </div>
            </form>
          </DialogContent>
        </Dialog>

        <Button @click="refreshData" :disabled="loading" variant="outline" class="w-full sm:w-auto text-sm sm:text-base">
          {{ loading ? "Refreshing..." : "Refresh" }}
        </Button>
      </div>
    </div>

    <div v-if="error" class="bg-red-500 text-white p-3 sm:p-4 rounded mb-3 sm:mb-4 text-sm sm:text-base">
      {{ error }}
    </div>

    <Card class="mb-3 sm:mb-4">
      <CardHeader class="pb-2 sm:pb-3">
        <CardTitle class="text-base sm:text-lg">User List</CardTitle>
        <p class="text-xs sm:text-sm text-muted-foreground">
          View and manage users. Data refreshes automatically every 60 seconds.
        </p>
      </CardHeader>
      <CardContent>
        <div class="flex items-center space-x-2 mb-3 sm:mb-4">
          <Input
            v-model="searchQuery"
            placeholder="Search by name, username, or Steam ID..."
            class="flex-grow text-sm sm:text-base"
          />
        </div>

        <div class="text-xs sm:text-sm text-muted-foreground mb-2">
          Showing {{ filteredUsers.length }} of {{ users.length }} users
        </div>

        <div v-if="loading && users.length === 0" class="text-center py-6 sm:py-8">
          <div
            class="animate-spin h-8 w-8 border-4 border-primary border-t-transparent rounded-full mx-auto mb-4"
          ></div>
          <p class="text-sm sm:text-base">Loading users...</p>
        </div>

        <div v-else-if="users.length === 0" class="text-center py-6 sm:py-8">
          <p class="text-sm sm:text-base">No users found</p>
        </div>

        <div v-else-if="filteredUsers.length === 0" class="text-center py-6 sm:py-8">
          <p class="text-sm sm:text-base">No users match your search</p>
        </div>

        <template v-else>
          <!-- Desktop Table View -->
          <div class="hidden md:block w-full overflow-x-auto">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead class="text-xs sm:text-sm">Name</TableHead>
                  <TableHead class="text-xs sm:text-sm">Username</TableHead>
                  <TableHead class="text-xs sm:text-sm">Steam ID</TableHead>
                  <TableHead class="text-xs sm:text-sm">Created</TableHead>
                  <TableHead class="text-xs sm:text-sm">Role</TableHead>
                  <TableHead class="text-right text-xs sm:text-sm">Actions</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                <TableRow
                  v-for="user in filteredUsers"
                  :key="user.id"
                  class="hover:bg-muted/50"
                >
                  <TableCell>
                    <div class="font-medium text-sm sm:text-base">
                      {{ user.name }}
                    </div>
                  </TableCell>
                  <TableCell class="text-xs sm:text-sm">{{ user.username }}</TableCell>
                  <TableCell class="text-xs sm:text-sm">{{ user.steam_id }}</TableCell>
                  <TableCell class="text-xs sm:text-sm">{{ formatDate(user.created_at) }}</TableCell>
                  <TableCell>
                    <Badge :variant="user.super_admin ? 'default' : 'outline'" class="text-xs">
                      {{ user.super_admin ? "Super Admin" : "User" }}
                    </Badge>
                  </TableCell>
                  <TableCell class="text-right">
                    <div class="flex gap-2 justify-end" v-if="authStore.user?.super_admin">
                      <Button
                        variant="outline"
                        size="sm"
                        @click="openEditDialog(user)"
                        :disabled="loading"
                        class="text-xs"
                      >
                        Edit
                      </Button>
                      <Button
                        variant="destructive"
                        size="sm"
                        @click="deleteUser(user.id)"
                        :disabled="loading"
                        class="text-xs"
                      >
                        Delete
                      </Button>
                    </div>
                    <div v-else class="text-xs sm:text-sm text-muted-foreground">
                      No actions available
                    </div>
                  </TableCell>
                </TableRow>
              </TableBody>
            </Table>
          </div>

          <!-- Mobile Card View -->
          <div class="md:hidden space-y-3">
            <div
              v-for="user in filteredUsers"
              :key="user.id"
              class="border rounded-lg p-3 sm:p-4 hover:bg-muted/30 transition-colors"
            >
              <div class="flex items-start justify-between gap-2 mb-2">
                <div class="flex-1 min-w-0">
                  <div class="font-semibold text-sm sm:text-base mb-1">
                    {{ user.name }}
                  </div>
                  <div class="space-y-1.5">
                    <div>
                      <span class="text-xs text-muted-foreground">Username: </span>
                      <span class="text-xs sm:text-sm">{{ user.username }}</span>
                    </div>
                    <div>
                      <span class="text-xs text-muted-foreground">Steam ID: </span>
                      <span class="text-xs sm:text-sm">{{ user.steam_id }}</span>
                    </div>
                    <div>
                      <span class="text-xs text-muted-foreground">Created: </span>
                      <span class="text-xs sm:text-sm">{{ formatDate(user.created_at) }}</span>
                    </div>
                    <div class="flex items-center gap-2 mt-2">
                      <Badge :variant="user.super_admin ? 'default' : 'outline'" class="text-xs">
                        {{ user.super_admin ? "Super Admin" : "User" }}
                      </Badge>
                    </div>
                  </div>
                </div>
              </div>
              <div class="flex items-center justify-end gap-2 pt-2 border-t" v-if="authStore.user?.super_admin">
                <Button
                  variant="outline"
                  size="sm"
                  @click="openEditDialog(user)"
                  :disabled="loading"
                  class="h-8 text-xs flex-1"
                >
                  Edit
                </Button>
                <Button
                  variant="destructive"
                  size="sm"
                  @click="deleteUser(user.id)"
                  :disabled="loading"
                  class="h-8 text-xs flex-1"
                >
                  Delete
                </Button>
              </div>
              <div v-else class="text-xs text-muted-foreground pt-2 border-t">
                No actions available
              </div>
            </div>
          </div>
        </template>
      </CardContent>
    </Card>

    <Card>
      <CardHeader>
        <CardTitle class="text-base sm:text-lg">About Users</CardTitle>
      </CardHeader>
      <CardContent>
        <p class="text-xs sm:text-sm text-muted-foreground">
          This page shows all users registered in Squad Aegis. You can add new
          users or delete existing ones.
        </p>
        <p class="text-xs sm:text-sm text-muted-foreground mt-2">
          Super Admin users have full access to all features and servers in the
          system.
        </p>
      </CardContent>
    </Card>
  </div>
</template>

<style scoped>
/* Add any page-specific styles here */
</style>
