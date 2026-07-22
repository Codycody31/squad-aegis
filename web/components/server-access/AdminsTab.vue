<script setup lang="ts">
import { computed, onMounted, reactive, ref } from "vue";
import { Button } from "~/components/ui/button";
import { Input } from "~/components/ui/input";
import { Card, CardContent, CardHeader, CardTitle } from "~/components/ui/card";
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from "~/components/ui/popover";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "~/components/ui/select";
import { toast } from "~/components/ui/toast";
import { isSecureOrLocalConnection } from "~/utils/security";
import AdminTable from "./AdminTable.vue";
import AdminFormDialog from "./AdminFormDialog.vue";
import type { ServerAdmin, ServerRole, User } from "./shared";

const props = defineProps<{
  serverId: string;
}>();

interface RolesResponse {
  data: {
    roles: ServerRole[];
  };
}

interface AdminsResponse {
  data: {
    admins: ServerAdmin[];
    pagination?: {
      total: number;
      pages: number;
      page: number;
      limit: number;
    };
  };
}

interface UsersResponse {
  data: {
    users: User[];
  };
}

const runtimeConfig = useRuntimeConfig();

const admins = ref<ServerAdmin[]>([]);
const allAdmins = ref<ServerAdmin[]>([]);
const users = ref<User[]>([]);
const roles = ref<ServerRole[]>([]);
const loading = reactive({
  admins: true,
  users: true,
  roles: true,
});
const error = ref<string | null>(null);
const cleanupLoading = ref(false);
const showAdminCfgPopover = ref(false);

const adminSearchQuery = ref("");
const adminCurrentPage = ref(1);
const adminItemsPerPage = ref("30");
const adminTotalItems = ref(0);
const adminTotalPages = ref(0);

const showAdminDialog = ref(false);
const adminDialogMode = ref<"create" | "edit">("create");
const editingAdmin = ref<ServerAdmin | null>(null);

// Function to fetch roles (needed for role selection and display)
async function fetchRoles() {
  loading.roles = true;

  try {
    const data = await useAuthFetchImperative<RolesResponse>(
      `${runtimeConfig.public.backendApi}/servers/${props.serverId}/roles`,
      { method: "GET" }
    );

    if (data && data.data) {
      roles.value = data.data.roles || [];
    }
  } catch (err: any) {
    error.value = extractApiErrorMessage(
      err,
      "An error occurred while fetching roles"
    );
    console.error(err);
  } finally {
    loading.roles = false;
  }
}

// Function to fetch admins
async function fetchAdmins() {
  loading.admins = true;
  error.value = null;

  const queryParams = new URLSearchParams({
    page: adminCurrentPage.value.toString(),
    limit: adminItemsPerPage.value,
  });

  if (adminSearchQuery.value.trim()) {
    queryParams.append("search", adminSearchQuery.value.trim());
  }

  try {
    const data = await useAuthFetchImperative<AdminsResponse>(
      `${runtimeConfig.public.backendApi}/servers/${props.serverId}/admins?${queryParams.toString()}`,
      { method: "GET" }
    );

    if (data && data.data) {
      admins.value = data.data.admins || [];
      adminTotalItems.value =
        data.data.pagination?.total ?? admins.value.length;
      adminTotalPages.value =
        data.data.pagination?.pages ?? (admins.value.length > 0 ? 1 : 0);

      if (
        adminTotalPages.value > 0 &&
        adminCurrentPage.value > adminTotalPages.value
      ) {
        adminCurrentPage.value = adminTotalPages.value;
        await fetchAdmins();
        return;
      }
    }
  } catch (err: any) {
    error.value = extractApiErrorMessage(
      err,
      "An error occurred while fetching admins"
    );
    console.error(err);
  } finally {
    loading.admins = false;
  }
}

async function fetchAllAdmins() {
  try {
    const data = await useAuthFetchImperative<AdminsResponse>(
      `${runtimeConfig.public.backendApi}/servers/${props.serverId}/admins`,
      { method: "GET" }
    );

    if (data && data.data) {
      allAdmins.value = data.data.admins || [];
    }
  } catch (err) {
    console.error("Error fetching full admin list:", err);
  }
}

// Function to fetch users
async function fetchUsers() {
  loading.users = true;

  try {
    const data = await useAuthFetchImperative<UsersResponse>(
      `${runtimeConfig.public.backendApi}/users`,
      { method: "GET" }
    );

    if (data && data.data) {
      users.value = data.data.users || [];
    }
  } catch (err: any) {
    error.value = extractApiErrorMessage(
      err,
      "An error occurred while fetching users"
    );
    console.error(err);
  } finally {
    loading.users = false;
  }
}

function applyAdminFilters() {
  adminCurrentPage.value = 1;
  fetchAdmins();
}

function resetAdminFilters() {
  adminSearchQuery.value = "";
  adminCurrentPage.value = 1;
  fetchAdmins();
}

function changeAdminPage(page: number) {
  if (page < 1 || page === adminCurrentPage.value) {
    return;
  }

  if (adminTotalPages.value > 0 && page > adminTotalPages.value) {
    return;
  }

  adminCurrentPage.value = page;
  fetchAdmins();
}

function changeAdminPageSize(value: any) {
  adminItemsPerPage.value = value;
  adminCurrentPage.value = 1;
  fetchAdmins();
}

// Function to remove an admin
async function removeAdmin(admin: ServerAdmin) {
  if (!confirm("Are you sure you want to remove this admin?")) {
    return;
  }

  loading.admins = true;
  error.value = null;

  try {
    await useAuthFetchImperative(
      `${runtimeConfig.public.backendApi}/servers/${props.serverId}/admins/${admin.id}`,
      { method: "DELETE" }
    );

    toast({
      title: "Success",
      description: "Admin removed successfully",
    });

    // Refresh the admins list
    await fetchAdmins();
    await fetchAllAdmins();
  } catch (err: any) {
    error.value = extractApiErrorMessage(
      err,
      "An error occurred while removing the admin"
    );
    console.error(err);
  } finally {
    loading.admins = false;
  }
}

// Function to cleanup expired admins
async function cleanupExpiredAdmins() {
  if (
    !confirm(
      "Are you sure you want to remove all expired admin roles? This action cannot be undone."
    )
  ) {
    return;
  }

  cleanupLoading.value = true;
  error.value = null;

  try {
    await useAuthFetchImperative(
      `${runtimeConfig.public.backendApi}/admin/cleanup-expired-admins`,
      { method: "POST" }
    );

    toast({
      title: "Success",
      description: "Expired admins cleaned up successfully",
    });

    // Refresh the admins list
    await fetchAdmins();
    await fetchAllAdmins();
  } catch (err: any) {
    error.value = extractApiErrorMessage(
      err,
      "An error occurred while cleaning up expired admins"
    );
    console.error(err);
  } finally {
    cleanupLoading.value = false;
  }
}

function openAddAdminDialog() {
  adminDialogMode.value = "create";
  editingAdmin.value = null;
  showAdminDialog.value = true;
}

// Function to open edit admin dialog
function openEditAdminDialog(admin: ServerAdmin) {
  adminDialogMode.value = "edit";
  editingAdmin.value = admin;
  showAdminDialog.value = true;
}

async function handleAdminSaved() {
  if (adminDialogMode.value === "create") {
    adminCurrentPage.value = 1;
    await fetchAdmins();
    await fetchAllAdmins();
  } else {
    fetchAdmins();
  }
}

// Security check for copy buttons
const canCopyConfigUrl = computed(() => isSecureOrLocalConnection());

// Computed property for admin config URL
const adminCfgUrl = computed(() => {
  var url = "";
  if (runtimeConfig.public.backendApi.startsWith("/")) {
    const origin = window.location.origin;
    url = `${origin}${runtimeConfig.public.backendApi}/servers/${props.serverId}/admins/cfg`;
  } else {
    url = `${runtimeConfig.public.backendApi}/servers/${props.serverId}/admins/cfg`;
  }
  return url;
});

async function copyAdminCfgUrl() {
  const ok = await copyToClipboard(adminCfgUrl.value);
  if (ok) {
    toast({
      title: "Success",
      description: "Admin configuration URL copied to clipboard",
    });
    return;
  }

  // Fallback: open the popover to show the URL for manual copy
  showAdminCfgPopover.value = true;
  toast({
    title: "Copy failed",
    description: "Clipboard unavailable — copy the URL manually.",
    variant: "destructive",
  });
}

function selectAdminCfgUrl(event: Event) {
  const input = event.target as HTMLInputElement;
  input.select();
}

const adminRangeStart = computed(() => {
  if (adminTotalItems.value === 0 || admins.value.length === 0) {
    return 0;
  }

  return (adminCurrentPage.value - 1) * Number(adminItemsPerPage.value) + 1;
});

const adminRangeEnd = computed(() => {
  if (adminTotalItems.value === 0 || admins.value.length === 0) {
    return 0;
  }

  return adminRangeStart.value + admins.value.length - 1;
});

const adminVisiblePages = computed(() => {
  const totalPages = adminTotalPages.value;
  if (totalPages <= 0) {
    return [];
  }

  if (totalPages <= 5) {
    return Array.from({ length: totalPages }, (_, index) => index + 1);
  }

  const visiblePages = new Set<number>([
    1,
    totalPages,
    adminCurrentPage.value - 1,
    adminCurrentPage.value,
    adminCurrentPage.value + 1,
  ]);

  return Array.from(visiblePages)
    .filter((page) => page >= 1 && page <= totalPages)
    .sort((left, right) => left - right);
});

onMounted(() => {
  fetchRoles();
  fetchUsers();
  fetchAdmins();
  fetchAllAdmins();
});
</script>

<template>
  <div>
    <div
      v-if="error"
      class="bg-red-500 text-white p-3 sm:p-4 rounded mb-3 sm:mb-4 text-sm sm:text-base"
    >
      {{ error }}
    </div>

    <Card>
      <CardHeader class="space-y-4 pb-2 sm:pb-3">
        <div
          class="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-3 sm:gap-0"
        >
          <CardTitle class="text-base sm:text-lg">Admin Management</CardTitle>
          <div class="flex flex-col sm:flex-row gap-2 w-full sm:w-auto">
            <Popover v-model:open="showAdminCfgPopover">
              <PopoverTrigger asChild>
                <Button
                  variant="outline"
                  size="sm"
                  @click="copyAdminCfgUrl"
                  :title="
                    canCopyConfigUrl
                      ? 'Copy Admin Config URL'
                      : 'Click to view URL (copy manually on HTTP)'
                  "
                  class="w-full sm:w-auto text-xs sm:text-sm"
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
            <Button
              variant="outline"
              size="sm"
              @click="cleanupExpiredAdmins"
              :disabled="cleanupLoading"
              class="w-full sm:w-auto text-xs sm:text-sm"
            >
              {{ cleanupLoading ? "Cleaning..." : "Cleanup Expired" }}
            </Button>
            <Button
              class="w-full sm:w-auto text-sm sm:text-base"
              @click="openAddAdminDialog"
            >
              Add Admin
            </Button>
          </div>
        </div>

        <div
          class="flex flex-col xl:flex-row xl:items-center xl:justify-between gap-3"
        >
          <div class="flex flex-col sm:flex-row gap-2 w-full xl:max-w-3xl">
            <Input
              v-model="adminSearchQuery"
              placeholder="Search by user, Steam ID, EOS ID, role, or notes"
              class="w-full"
              @keyup.enter="applyAdminFilters"
            />
            <div class="flex gap-2">
              <Button
                variant="outline"
                class="flex-1 sm:flex-none"
                @click="applyAdminFilters"
              >
                Search
              </Button>
              <Button
                variant="ghost"
                class="flex-1 sm:flex-none"
                @click="resetAdminFilters"
                :disabled="!adminSearchQuery.trim()"
              >
                Clear
              </Button>
            </div>
          </div>

          <div class="flex items-center gap-2 self-end xl:self-auto">
            <span class="text-xs text-muted-foreground uppercase tracking-wide">
              Per Page
            </span>
            <Select
              :model-value="adminItemsPerPage"
              @update:model-value="changeAdminPageSize"
            >
              <SelectTrigger class="w-[90px]">
                <SelectValue placeholder="30" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="30">30</SelectItem>
                <SelectItem value="50">50</SelectItem>
                <SelectItem value="100">100</SelectItem>
              </SelectContent>
            </Select>
          </div>
        </div>
      </CardHeader>
      <CardContent>
        <AdminTable
          :admins="admins"
          :loading="loading.admins"
          :users="users"
          :roles="roles"
          :empty-text="
            adminSearchQuery.trim()
              ? 'No matching admin or whitelist entries found.'
              : 'No admins found. Add an admin to get started.'
          "
          @edit="openEditAdminDialog"
          @remove="removeAdmin"
        />

        <div
          v-if="!loading.admins && admins.length > 0"
          class="flex flex-col sm:flex-row justify-between items-center gap-3 pt-4"
        >
          <div
            class="text-xs sm:text-sm text-muted-foreground text-center sm:text-left"
          >
            Showing {{ adminRangeStart }}-{{ adminRangeEnd }} of
            {{ adminTotalItems }} entries
          </div>

          <div
            v-if="adminTotalPages > 1"
            class="flex items-center gap-1 sm:gap-2 flex-wrap justify-center"
          >
            <Button
              variant="outline"
              size="sm"
              @click="changeAdminPage(adminCurrentPage - 1)"
              :disabled="adminCurrentPage === 1"
              class="text-xs sm:text-sm"
            >
              Prev
            </Button>

            <Button
              v-for="page in adminVisiblePages"
              :key="page"
              size="sm"
              :variant="page === adminCurrentPage ? 'default' : 'outline'"
              class="min-w-9 text-xs sm:text-sm"
              @click="changeAdminPage(page)"
            >
              {{ page }}
            </Button>

            <Button
              variant="outline"
              size="sm"
              @click="changeAdminPage(adminCurrentPage + 1)"
              :disabled="adminCurrentPage === adminTotalPages"
              class="text-xs sm:text-sm"
            >
              Next
            </Button>
          </div>
        </div>
      </CardContent>
    </Card>

    <AdminFormDialog
      v-model:open="showAdminDialog"
      :mode="adminDialogMode"
      :server-id="serverId"
      :roles="roles"
      :users="users"
      :all-admins="allAdmins"
      :admin="editingAdmin"
      @saved="handleAdminSaved"
    />
  </div>
</template>
