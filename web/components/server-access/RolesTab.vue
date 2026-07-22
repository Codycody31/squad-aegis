<script setup lang="ts">
import { onMounted, reactive, ref } from "vue";
import { Button } from "~/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "~/components/ui/card";
import { toast } from "~/components/ui/toast";
import RoleTable from "./RoleTable.vue";
import RoleFormDialog from "./RoleFormDialog.vue";
import RoleTemplateDialog from "./RoleTemplateDialog.vue";
import type { PermissionGroup, RoleTemplate, ServerRole } from "./shared";

const props = defineProps<{
  serverId: string;
}>();

interface RolesResponse {
  data: {
    roles: ServerRole[];
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

const runtimeConfig = useRuntimeConfig();

const roles = ref<ServerRole[]>([]);
const permissions = ref<PermissionGroup[]>([]);
const roleTemplates = ref<RoleTemplate[]>([]);
const loading = reactive({
  roles: true,
  permissions: true,
  templates: true,
});
const error = ref<string | null>(null);

const showTemplateDialog = ref(false);
const showRoleDialog = ref(false);
const roleDialogMode = ref<"create" | "edit">("create");
const editingRole = ref<ServerRole | null>(null);

// Function to fetch permissions from API
async function fetchPermissions() {
  loading.permissions = true;

  try {
    const data = await useAuthFetchImperative<PermissionsResponse>(
      `${runtimeConfig.public.backendApi}/permissions`,
      { method: "GET" }
    );

    if (data && data.data) {
      permissions.value = data.data.permissions || [];
    }
  } catch (err: any) {
    console.error("Error fetching permissions:", err);
  } finally {
    loading.permissions = false;
  }
}

// Function to fetch role templates from API
async function fetchRoleTemplates() {
  loading.templates = true;

  try {
    const data = await useAuthFetchImperative<RoleTemplatesResponse>(
      `${runtimeConfig.public.backendApi}/role-templates`,
      { method: "GET" }
    );

    if (data && data.data) {
      roleTemplates.value = data.data.templates || [];
    }
  } catch (err: any) {
    console.error("Error fetching role templates:", err);
  } finally {
    loading.templates = false;
  }
}

// Function to fetch roles
async function fetchRoles() {
  loading.roles = true;
  error.value = null;

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

// Function to remove a role
async function removeRole(role: ServerRole) {
  if (
    !confirm(
      "Are you sure you want to remove this role? This will not affect existing admins with this role."
    )
  ) {
    return;
  }

  loading.roles = true;
  error.value = null;

  try {
    await useAuthFetchImperative(
      `${runtimeConfig.public.backendApi}/servers/${props.serverId}/roles/${role.id}`,
      { method: "DELETE" }
    );

    toast({
      title: "Success",
      description: "Role removed successfully",
    });

    // Refresh the roles list
    fetchRoles();
  } catch (err: any) {
    error.value = extractApiErrorMessage(
      err,
      "An error occurred while removing the role"
    );
    console.error(err);
  } finally {
    loading.roles = false;
  }
}

function openCreateRoleDialog() {
  roleDialogMode.value = "create";
  editingRole.value = null;
  showRoleDialog.value = true;
}

// Function to open edit role dialog
function openEditRoleDialog(role: ServerRole) {
  roleDialogMode.value = "edit";
  editingRole.value = role;
  showRoleDialog.value = true;
}

onMounted(() => {
  fetchPermissions();
  fetchRoleTemplates();
  fetchRoles();
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
      <CardHeader
        class="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-3 sm:gap-0 pb-2 sm:pb-3"
      >
        <CardTitle class="text-base sm:text-lg">Role Management</CardTitle>
        <div class="flex flex-col sm:flex-row gap-2 w-full sm:w-auto">
          <Button
            variant="outline"
            class="w-full sm:w-auto text-sm sm:text-base"
            @click="showTemplateDialog = true"
          >
            From Template
          </Button>
          <Button
            class="w-full sm:w-auto text-sm sm:text-base"
            @click="openCreateRoleDialog"
          >
            Custom Role
          </Button>
        </div>
      </CardHeader>
      <CardContent>
        <RoleTable
          :roles="roles"
          :loading="loading.roles"
          :permission-groups="permissions"
          @edit="openEditRoleDialog"
          @delete="removeRole"
        />
      </CardContent>
    </Card>

    <RoleTemplateDialog
      v-model:open="showTemplateDialog"
      :server-id="serverId"
      :templates="roleTemplates"
      :permission-groups="permissions"
      @saved="fetchRoles"
    />

    <RoleFormDialog
      v-model:open="showRoleDialog"
      :mode="roleDialogMode"
      :role="editingRole"
      :server-id="serverId"
      :permission-groups="permissions"
      :permissions-loading="loading.permissions"
      @saved="fetchRoles"
    />
  </div>
</template>
