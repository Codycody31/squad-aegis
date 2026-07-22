<script setup lang="ts">
import { ref, watch } from "vue";
import { Button } from "~/components/ui/button";
import { Input } from "~/components/ui/input";
import { Checkbox } from "~/components/ui/checkbox";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "~/components/ui/dialog";
import {
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from "~/components/ui/form";
import { toast } from "~/components/ui/toast";
import { useForm } from "vee-validate";
import { toTypedSchema } from "@vee-validate/zod";
import * as z from "zod";
import PermissionMatrix from "./PermissionMatrix.vue";
import type { PermissionGroup, ServerRole } from "./shared";

const props = defineProps<{
  mode: "create" | "edit";
  serverId: string;
  permissionGroups: PermissionGroup[];
  permissionsLoading?: boolean;
  role?: ServerRole | null;
}>();

const open = defineModel<boolean>("open", { default: false });

const emit = defineEmits<{
  (e: "saved"): void;
}>();

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
    is_admin: z.boolean().default(true),
  })
);

const form = useForm({
  validationSchema: roleFormSchema,
  initialValues: {
    name: "",
    permissions: [] as string[],
    is_admin: true,
  },
});

const saving = ref(false);
const error = ref<string | null>(null);

watch(open, (isOpen) => {
  if (!isOpen) return;
  error.value = null;
  if (props.mode === "edit" && props.role) {
    form.resetForm({
      values: {
        name: props.role.name,
        permissions: props.role.permissions,
        is_admin: props.role.is_admin,
      },
    });
  } else {
    form.resetForm({
      values: {
        name: "",
        permissions: [],
        is_admin: true,
      },
    });
  }
});

const onSubmit = form.handleSubmit(async (values) => {
  if (props.mode === "edit") {
    await updateRole(values);
  } else {
    await createRole(values);
  }
});

// Function to create a role
async function createRole(values: any) {
  saving.value = true;
  error.value = null;

  const runtimeConfig = useRuntimeConfig();

  try {
    await useAuthFetchImperative(
      `${runtimeConfig.public.backendApi}/servers/${props.serverId}/roles`,
      {
        method: "POST",
        body: {
          name: values.name,
          permissions: values.permissions || [],
          is_admin: values.is_admin !== undefined ? values.is_admin : true,
        },
      }
    );

    // Reset form and close dialog
    form.resetForm();
    open.value = false;

    toast({
      title: "Success",
      description: "Role created successfully",
    });

    emit("saved");
  } catch (err: any) {
    error.value = extractApiErrorMessage(
      err,
      "An error occurred while adding the role"
    );
    console.error(err);
  } finally {
    saving.value = false;
  }
}

// Function to update a role
async function updateRole(values: any) {
  if (!props.role) return;

  saving.value = true;
  error.value = null;

  const runtimeConfig = useRuntimeConfig();

  try {
    const updateBody: any = {};
    if (values.name !== props.role.name) {
      updateBody.name = values.name;
    }
    if (
      JSON.stringify(values.permissions) !==
      JSON.stringify(props.role.permissions)
    ) {
      updateBody.permissions = values.permissions || [];
    }
    if (values.is_admin !== props.role.is_admin) {
      updateBody.is_admin = values.is_admin;
    }

    if (Object.keys(updateBody).length === 0) {
      error.value = "No changes to update";
      saving.value = false;
      return;
    }

    await useAuthFetchImperative(
      `${runtimeConfig.public.backendApi}/servers/${props.serverId}/roles/${props.role.id}`,
      {
        method: "PUT",
        body: updateBody,
      }
    );

    toast({
      title: "Success",
      description: "Role updated successfully",
    });

    open.value = false;
    emit("saved");
  } catch (err: any) {
    error.value = extractApiErrorMessage(
      err,
      "An error occurred while updating the role"
    );
    console.error(err);
  } finally {
    saving.value = false;
  }
}
</script>

<template>
  <Dialog v-model:open="open">
    <DialogContent
      class="w-[95vw] sm:max-w-[700px] max-h-[85vh] sm:max-h-[80vh] overflow-y-auto p-4 sm:p-6"
    >
      <DialogHeader>
        <DialogTitle class="text-base sm:text-lg">
          {{ mode === "edit" ? "Edit Role" : "Create Custom Role" }}
        </DialogTitle>
        <DialogDescription class="text-xs sm:text-sm">
          {{
            mode === "edit"
              ? "Update the role name, permissions, and admin status."
              : "Create a new role with custom permissions. Permissions are grouped by category."
          }}
        </DialogDescription>
      </DialogHeader>

      <div v-if="error" class="bg-red-500 text-white p-3 rounded text-sm">
        {{ error }}
      </div>

      <form @submit="onSubmit">
        <div class="grid gap-4 py-4">
          <FormField name="name" v-slot="{ componentField }">
            <FormItem>
              <FormLabel>Role Name</FormLabel>
              <FormControl>
                <Input placeholder="e.g., SeniorAdmin" v-bind="componentField" />
              </FormControl>
              <FormDescription>
                Role name can only contain letters, numbers, and underscores.
              </FormDescription>
              <FormMessage />
            </FormItem>
          </FormField>

          <FormField name="permissions" v-slot="{ value, handleChange }">
            <FormItem>
              <FormLabel>Permissions</FormLabel>
              <FormDescription>
                Select permissions for this role. Permissions are organized by category.
              </FormDescription>
              <PermissionMatrix
                :groups="permissionGroups"
                :loading="permissionsLoading"
                :model-value="(value as string[]) || []"
                @update:model-value="handleChange"
              />
              <FormMessage />
            </FormItem>
          </FormField>

          <FormField name="is_admin" v-slot="{ value, handleChange }">
            <FormItem class="flex flex-row items-start space-x-3 space-y-0">
              <FormControl>
                <Checkbox
                  :modelValue="value"
                  @update:modelValue="handleChange"
                />
              </FormControl>
              <div class="space-y-1 leading-none">
                <FormLabel> Is Admin Role </FormLabel>
                <FormDescription>
                  If checked, this role will be included when plugins or workflows fetch "admins" (e.g., for pings, admin lists). Uncheck for non-management roles like "Reserved" or "VIP". All RCON roles appear in admin.cfg regardless of this setting.
                </FormDescription>
              </div>
            </FormItem>
          </FormField>
        </div>
        <DialogFooter>
          <Button type="button" variant="outline" @click="open = false">
            Cancel
          </Button>
          <Button type="submit" :disabled="saving">
            {{
              mode === "edit"
                ? saving
                  ? "Updating..."
                  : "Update Role"
                : saving
                  ? "Adding..."
                  : "Add Role"
            }}
          </Button>
        </DialogFooter>
      </form>
    </DialogContent>
  </Dialog>
</template>
