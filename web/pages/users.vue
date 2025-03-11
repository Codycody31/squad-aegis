<script setup lang="ts">
import { ref, onMounted, onUnmounted, computed } from "vue";
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
import { useForm } from "vee-validate";
import { toTypedSchema } from "@vee-validate/zod";
import * as z from "zod";

const runtimeConfig = useRuntimeConfig();
const loading = ref(true);
const error = ref<string | null>(null);
const users = ref<User[]>([]);
const refreshInterval = ref<NodeJS.Timeout | null>(null);
const searchQuery = ref("");
const showAddUserDialog = ref(false);
const addUserLoading = ref(false);

interface User {
  id: string;
  steam_id: number;
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
const formSchema = toTypedSchema(
  z.object({
    steamId: z
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

// Setup form
const form = useForm({
  validationSchema: formSchema,
  initialValues: {
    steamId: "",
    name: "",
    username: "",
    password: "",
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

  const runtimeConfig = useRuntimeConfig();
  const cookieToken = useCookie(
    runtimeConfig.public.sessionCookieName as string
  );
  const token = cookieToken.value;

  if (!token) {
    error.value = "Authentication required";
    loading.value = false;
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
  const { steamId, name, username, password, superAdmin } = values;

  addUserLoading.value = true;
  error.value = null;

  const runtimeConfig = useRuntimeConfig();
  const cookieToken = useCookie(
    runtimeConfig.public.sessionCookieName as string
  );
  const token = cookieToken.value;

  if (!token) {
    error.value = "Authentication required";
    addUserLoading.value = false;
    return;
  }

  try {
    const { data, error: fetchError } = await useFetch(
      `${runtimeConfig.public.backendApi}/users`,
      {
        method: "POST",
        body: {
          steam_id: parseInt(steamId),
          name,
          username,
          password,
          super_admin: superAdmin,
        },
        headers: {
          Authorization: `Bearer ${token}`,
        },
      }
    );

    if (fetchError.value) {
      throw new Error(fetchError.value.message || "Failed to add user");
    }

    // Reset form and close dialog
    form.resetForm();
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

// Function to delete a user
async function deleteUser(userId: string) {
  if (!confirm("Are you sure you want to delete this user?")) {
    return;
  }

  loading.value = true;
  error.value = null;

  const runtimeConfig = useRuntimeConfig();
  const cookieToken = useCookie(
    runtimeConfig.public.sessionCookieName as string
  );
  const token = cookieToken.value;

  if (!token) {
    error.value = "Authentication required";
    loading.value = false;
    return;
  }

  try {
    const { data, error: fetchError } = await useFetch(
      `${runtimeConfig.public.backendApi}/users/${userId}`,
      {
        method: "DELETE",
        headers: {
          Authorization: `Bearer ${token}`,
        },
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

// Manual refresh function
function refreshData() {
  fetchUsers();
}
</script>

<template>
  <div class="p-4">
    <div class="flex justify-between items-center mb-4">
      <h1 class="text-2xl font-bold">Users</h1>
      <div class="flex gap-2">
        <Form
          v-slot="{ handleSubmit }"
          as=""
          keep-values
          :validation-schema="formSchema"
          :initial-values="{
            steamId: '',
            name: '',
            username: '',
            password: '',
            superAdmin: false,
          }"
        >
          <Dialog v-model:open="showAddUserDialog">
            <DialogTrigger asChild>
              <Button>Add User</Button>
            </DialogTrigger>
            <DialogContent
              class="sm:max-w-[425px] max-h-[90vh] overflow-y-auto"
            >
              <DialogHeader>
                <DialogTitle>Add New User</DialogTitle>
                <DialogDescription>
                  Enter the details of the user you want to add.
                </DialogDescription>
              </DialogHeader>
              <form id="dialogForm" @submit="handleSubmit($event, addUser)">
                <div class="grid gap-4 py-4">
                  <FormField name="steamId" v-slot="{ componentField }">
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
        <Button @click="refreshData" :disabled="loading" variant="outline">
          {{ loading ? "Refreshing..." : "Refresh" }}
        </Button>
      </div>
    </div>

    <div v-if="error" class="bg-red-500 text-white p-4 rounded mb-4">
      {{ error }}
    </div>

    <Card class="mb-4">
      <CardHeader class="pb-2">
        <CardTitle>User List</CardTitle>
        <p class="text-sm text-muted-foreground">
          View and manage users. Data refreshes automatically every 60 seconds.
        </p>
      </CardHeader>
      <CardContent>
        <div class="flex items-center space-x-2 mb-4">
          <Input
            v-model="searchQuery"
            placeholder="Search by name, username, or Steam ID..."
            class="flex-grow"
          />
        </div>

        <div class="text-sm text-muted-foreground mb-2">
          Showing {{ filteredUsers.length }} of {{ users.length }} users
        </div>

        <div v-if="loading && users.length === 0" class="text-center py-8">
          <div
            class="animate-spin h-8 w-8 border-4 border-primary border-t-transparent rounded-full mx-auto mb-4"
          ></div>
          <p>Loading users...</p>
        </div>

        <div v-else-if="users.length === 0" class="text-center py-8">
          <p>No users found</p>
        </div>

        <div v-else-if="filteredUsers.length === 0" class="text-center py-8">
          <p>No users match your search</p>
        </div>

        <div v-else class="overflow-x-auto">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Name</TableHead>
                <TableHead>Username</TableHead>
                <TableHead>Steam ID</TableHead>
                <TableHead>Created</TableHead>
                <TableHead>Role</TableHead>
                <TableHead class="text-right">Actions</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              <TableRow
                v-for="user in filteredUsers"
                :key="user.id"
                class="hover:bg-muted/50"
              >
                <TableCell>
                  <div class="font-medium">
                    {{ user.name }}
                  </div>
                </TableCell>
                <TableCell>{{ user.username }}</TableCell>
                <TableCell>{{ user.steam_id }}</TableCell>
                <TableCell>{{ formatDate(user.created_at) }}</TableCell>
                <TableCell>
                  <Badge :variant="user.super_admin ? 'default' : 'outline'">
                    {{ user.super_admin ? "Super Admin" : "User" }}
                  </Badge>
                </TableCell>
                <TableCell class="text-right">
                  <Button
                    variant="destructive"
                    size="sm"
                    @click="deleteUser(user.id)"
                    :disabled="loading"
                  >
                    Delete
                  </Button>
                </TableCell>
              </TableRow>
            </TableBody>
          </Table>
        </div>
      </CardContent>
    </Card>

    <Card>
      <CardHeader>
        <CardTitle>About Users</CardTitle>
      </CardHeader>
      <CardContent>
        <p class="text-sm text-muted-foreground">
          This page shows all users registered in Squad Aegis. You can add new
          users or delete existing ones.
        </p>
        <p class="text-sm text-muted-foreground mt-2">
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
