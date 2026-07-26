<script setup lang="ts">
import { ref, computed } from "vue";
import { Button } from "~/components/ui/button";
import { Input } from "~/components/ui/input";
import { Card, CardContent, CardHeader, CardTitle } from "~/components/ui/card";
import { Badge } from "~/components/ui/badge";
import PageHeader from "~/components/layout/PageHeader.vue";
import DataTable from "~/components/data-table/DataTable.vue";
import type { DataTableColumn } from "~/components/data-table/types";
import UserFormDialog from "~/components/users/UserFormDialog.vue";
import type { User } from "~/components/users/types";
import { useAuthStore } from "@/stores/auth";
import { toast } from "~/components/ui/toast";

const authStore = useAuthStore();
const runtimeConfig = useRuntimeConfig();
const loading = ref(true);
const users = ref<User[]>([]);
const searchQuery = ref("");
const dialogOpen = ref(false);
const dialogMode = ref<"create" | "edit">("create");
const editingUser = ref<User | null>(null);

interface UsersResponse {
  data: {
    users: User[];
  };
}

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

  try {
    const response = await useAuthFetchImperative<UsersResponse>(
      `${runtimeConfig.public.backendApi}/users`
    );

    users.value = response.data?.users || [];

    // Sort by creation date (most recent first)
    users.value.sort((a, b) => {
      return (
        new Date(b.created_at).getTime() - new Date(a.created_at).getTime()
      );
    });
  } catch (err: any) {
    toast({
      title: "Error",
      description: extractApiErrorMessage(
        err,
        "An error occurred while fetching users data"
      ),
      variant: "destructive",
    });
  } finally {
    loading.value = false;
  }
}

// Function to open add dialog
function openAddDialog() {
  dialogMode.value = "create";
  editingUser.value = null;
  dialogOpen.value = true;
}

// Function to open edit dialog
function openEditDialog(user: User) {
  dialogMode.value = "edit";
  editingUser.value = user;
  dialogOpen.value = true;
}

// Function to delete a user
async function deleteUser(userId: string) {
  if (!confirm("Are you sure you want to delete this user?")) {
    return;
  }

  loading.value = true;

  try {
    await useAuthFetchImperative(
      `${runtimeConfig.public.backendApi}/users/${userId}`,
      {
        method: "DELETE",
      }
    );

    toast({
      title: "Success",
      description: "User deleted successfully",
    });

    // Refresh the users list
    fetchUsers();
  } catch (err: any) {
    toast({
      title: "Failed to Delete User",
      description: extractApiErrorMessage(
        err,
        "An error occurred while deleting the user"
      ),
      variant: "destructive",
    });
  } finally {
    loading.value = false;
  }
}

// Format date
function formatDate(dateString: string): string {
  return new Date(dateString).toLocaleString();
}

const columns: DataTableColumn<User>[] = [
  { key: "name", label: "Name", sortable: true },
  { key: "username", label: "Username", sortable: true },
  { key: "steam_id", label: "Steam ID" },
  { key: "created_at", label: "Created", sortable: true },
  { key: "role", label: "Role" },
];

fetchUsers();

// Manual refresh function
function refreshData() {
  fetchUsers();
}
</script>

<template>
  <div class="p-4 md:p-6">
    <PageHeader
      title="Users"
      description="Manage who can sign in to Squad Aegis."
    >
      <template #actions>
        <Button
          v-if="authStore.user?.super_admin"
          class="w-full sm:w-auto text-sm sm:text-base"
          @click="openAddDialog"
        >
          Add User
        </Button>

        <Button
          :disabled="loading"
          variant="outline"
          class="w-full sm:w-auto text-sm sm:text-base"
          @click="refreshData"
        >
          {{ loading ? "Refreshing..." : "Refresh" }}
        </Button>
      </template>
    </PageHeader>

    <UserFormDialog
      v-model:open="dialogOpen"
      :mode="dialogMode"
      :user="editingUser"
      @saved="fetchUsers"
    />

    <Card class="mb-3 sm:mb-4">
      <CardHeader class="pb-2 sm:pb-3">
        <CardTitle class="text-base sm:text-lg">User List</CardTitle>
        <p class="text-xs sm:text-sm text-muted-foreground">
          View and manage users.
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

        <DataTable
          :columns="columns"
          :rows="filteredUsers"
          row-key="id"
          :loading="loading && users.length === 0"
          :empty-text="
            users.length === 0 ? 'No users found' : 'No users match your search'
          "
        >
          <template #cell-name="{ row }">
            <span class="font-medium">{{ row.name }}</span>
          </template>
          <template #cell-created_at="{ row }">
            {{ formatDate(row.created_at) }}
          </template>
          <template #cell-role="{ row }">
            <Badge
              :variant="row.super_admin ? 'default' : 'outline'"
              class="text-xs"
            >
              {{ row.super_admin ? "Super Admin" : "User" }}
            </Badge>
          </template>
          <template v-if="authStore.user?.super_admin" #actions="{ row }">
            <div class="flex justify-end gap-2">
              <Button
                variant="outline"
                size="sm"
                :disabled="loading"
                class="text-xs"
                @click="openEditDialog(row)"
              >
                Edit
              </Button>
              <Button
                variant="destructive"
                size="sm"
                :disabled="loading"
                class="text-xs"
                @click="deleteUser(row.id)"
              >
                Delete
              </Button>
            </div>
          </template>
        </DataTable>
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
