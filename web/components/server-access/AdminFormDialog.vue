<script setup lang="ts">
import { computed, ref, watch } from "vue";
import { Badge } from "~/components/ui/badge";
import { Button } from "~/components/ui/button";
import { Input } from "~/components/ui/input";
import { Textarea } from "~/components/ui/textarea";
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
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
  SelectGroup,
} from "~/components/ui/select";
import { toast } from "~/components/ui/toast";
import { useForm } from "vee-validate";
import { toTypedSchema } from "@vee-validate/zod";
import * as z from "zod";
import type { ServerAdmin, ServerRole, User } from "./shared";

const props = defineProps<{
  mode: "create" | "edit";
  serverId: string;
  roles: ServerRole[];
  users: User[];
  allAdmins: ServerAdmin[];
  admin?: ServerAdmin | null;
}>();

const open = defineModel<boolean>("open", { default: false });

const emit = defineEmits<{
  (e: "saved"): void;
}>();

const adminFormSchema = toTypedSchema(
  z
    .object({
      adminType: z.enum(["user", "steam_id", "eos_id"], {
        required_error: "Please select admin type",
      }),
      user_id: z.string().optional(),
      steam_id: z.string().optional(),
      eos_id: z.string().optional(),
      server_role_id: z.string().min(1, "Role is required"),
      expires_at: z.string().optional(),
      notes: z.string().optional(),
    })
    .superRefine((data, ctx) => {
      if (data.adminType === "user") {
        if (!data.user_id || data.user_id.length === 0) {
          ctx.addIssue({
            code: z.ZodIssueCode.custom,
            message: "Please select a user",
            path: ["user_id"],
          });
        }
        return;
      }

      if (data.adminType === "steam_id") {
        if (!data.steam_id || data.steam_id.length !== 17) {
          ctx.addIssue({
            code: z.ZodIssueCode.custom,
            message: "Please enter a valid Steam ID",
            path: ["steam_id"],
          });
        }
        return;
      }

      if (!data.eos_id || !/^[a-fA-F0-9]{32}$/.test(data.eos_id)) {
        ctx.addIssue({
          code: z.ZodIssueCode.custom,
          message: "Please enter a valid EOS ID",
          path: ["eos_id"],
        });
      }
    })
);

// Edit admin form schema (only for notes)
const editAdminFormSchema = toTypedSchema(
  z.object({
    notes: z.string().optional(),
  })
);

const validationSchema = computed(() =>
  props.mode === "edit" ? editAdminFormSchema : adminFormSchema
);

const form = useForm<Record<string, any>>({
  validationSchema,
  initialValues: {
    adminType: "user",
    user_id: "",
    steam_id: "",
    eos_id: "",
    server_role_id: "",
    expires_at: "",
    notes: "",
  },
});

const saving = ref(false);
const error = ref<string | null>(null);
const selectedAdminType = ref("user");

// Watch for changes in adminType to update our reactive reference
watch(
  () => form.values.adminType,
  (newValue) => {
    selectedAdminType.value = newValue || "user";
  },
  { immediate: true }
);

// Watch for dialog state changes to reset form
watch(open, (isOpen) => {
  if (!isOpen) return;
  error.value = null;
  if (props.mode === "edit") {
    form.resetForm({
      values: {
        notes: props.admin?.notes || "",
      },
    });
  } else {
    // Reset to default when dialog opens
    selectedAdminType.value = "user";
    form.resetForm({
      values: {
        adminType: "user",
        user_id: "",
        steam_id: "",
        eos_id: "",
        server_role_id: "",
        expires_at: "",
        notes: "",
      },
    });
  }
});

// Get users that already have the selected role (to prevent duplicates)
const getUsersWithRole = (roleId: string) => {
  return props.allAdmins
    .filter((admin) => admin.user_id && admin.server_role_id === roleId)
    .map((admin) => admin.user_id);
};

// Get filtered users based on selected role (exclude users who already have this specific role)
const getFilteredUsersForRole = (roleId: string) => {
  if (!roleId) return props.users;
  const usersWithRole = getUsersWithRole(roleId);
  return props.users.filter((user) => !usersWithRole.includes(user.id));
};

// Get filtered users for user selection based on selected role
const filteredUsersForSelection = computed(() => {
  const selectedRole = form.values.server_role_id;
  if (!selectedRole) return props.users;
  return getFilteredUsersForRole(selectedRole);
});

const onSubmit = form.handleSubmit(async (values: any) => {
  if (props.mode === "edit") {
    await updateAdminNotes(values);
  } else {
    await addAdmin(values);
  }
});

// Function to add an admin
async function addAdmin(values: any) {
  const {
    adminType,
    user_id,
    steam_id,
    eos_id,
    server_role_id,
    expires_at,
    notes,
  } = values;

  saving.value = true;
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
    } else if (adminType === "eos_id") {
      requestBody.eos_id = eos_id;
    }

    // Add expires_at if provided
    if (expires_at && expires_at.trim() !== "") {
      requestBody.expires_at = new Date(expires_at).toISOString();
    }

    // Add notes if provided
    if (notes && notes.trim() !== "") {
      requestBody.notes = notes;
    }

    await useAuthFetchImperative(
      `${runtimeConfig.public.backendApi}/servers/${props.serverId}/admins`,
      {
        method: "POST",
        body: requestBody,
      }
    );

    // Reset form and close dialog
    form.resetForm();
    open.value = false;

    toast({
      title: "Success",
      description: "Admin added successfully",
    });

    emit("saved");
  } catch (err: any) {
    error.value = extractApiErrorMessage(
      err,
      "An error occurred while adding the admin"
    );
    console.error(err);
  } finally {
    saving.value = false;
  }
}

// Function to update admin notes
async function updateAdminNotes(values: any) {
  if (!props.admin) return;

  saving.value = true;
  error.value = null;

  const runtimeConfig = useRuntimeConfig();

  try {
    await useAuthFetchImperative(
      `${runtimeConfig.public.backendApi}/servers/${props.serverId}/admins/${props.admin.id}`,
      {
        method: "PUT",
        body: {
          notes: values.notes,
        },
      }
    );

    toast({
      title: "Success",
      description: "Admin notes updated successfully",
    });

    open.value = false;
    emit("saved");
  } catch (err: any) {
    error.value = extractApiErrorMessage(
      err,
      "An error occurred while updating the admin"
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
      :class="
        mode === 'edit'
          ? 'w-[95vw] sm:max-w-[500px] p-4 sm:p-6'
          : 'w-[95vw] sm:max-w-[425px] max-h-[85vh] sm:max-h-[80vh] overflow-y-auto p-4 sm:p-6'
      "
    >
      <DialogHeader>
        <DialogTitle class="text-base sm:text-lg">
          {{ mode === "edit" ? "Edit Admin Notes" : "Add New Admin" }}
        </DialogTitle>
        <DialogDescription class="text-xs sm:text-sm">
          {{
            mode === "edit"
              ? "Update the notes for this admin assignment."
              : "Assign a user as an admin with a specific role."
          }}
        </DialogDescription>
      </DialogHeader>

      <div v-if="error" class="bg-red-500 text-white p-3 rounded text-sm">
        {{ error }}
      </div>

      <form @submit="onSubmit">
        <div v-if="mode === 'edit'" class="grid gap-4 py-4">
          <FormField v-slot="{ componentField }" name="notes">
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

        <div v-else class="grid gap-4 py-4">
          <FormField name="adminType" v-slot="{ componentField }">
            <FormItem>
              <FormLabel>Admin Type</FormLabel>
              <Select
                v-bind="componentField"
                @update:model-value="
                  (value: any) => (selectedAdminType = value || 'user')
                "
              >
                <FormControl>
                  <SelectTrigger>
                    <SelectValue placeholder="Select admin type" />
                  </SelectTrigger>
                </FormControl>
                <SelectContent>
                  <SelectGroup>
                    <SelectItem value="user">Existing User</SelectItem>
                    <SelectItem value="steam_id">Steam ID</SelectItem>
                    <SelectItem value="eos_id">EOS ID</SelectItem>
                  </SelectGroup>
                </SelectContent>
              </Select>
              <FormDescription>
                Choose whether to assign an existing user or add an admin by Steam or EOS ID.
              </FormDescription>
              <FormMessage />
            </FormItem>
          </FormField>

          <FormField name="user_id" v-slot="{ componentField }">
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

          <FormField name="steam_id" v-slot="{ componentField }">
            <FormItem v-if="selectedAdminType === 'steam_id'">
              <FormLabel>Steam ID</FormLabel>
              <FormControl>
                <Input placeholder="76561198012345678" v-bind="componentField" />
              </FormControl>
              <FormDescription>
                Enter the 17-digit Steam ID of the player you want to make an admin.
              </FormDescription>
              <FormMessage />
            </FormItem>
          </FormField>

          <FormField name="eos_id" v-slot="{ componentField }">
            <FormItem v-if="selectedAdminType === 'eos_id'">
              <FormLabel>EOS ID</FormLabel>
              <FormControl>
                <Input
                  placeholder="0123456789abcdef0123456789abcdef"
                  v-bind="componentField"
                />
              </FormControl>
              <FormDescription>
                Enter the 32-character EOS ID of the player you want to assign this role to.
              </FormDescription>
              <FormMessage />
            </FormItem>
          </FormField>

          <FormField name="server_role_id" v-slot="{ componentField }">
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

          <FormField name="expires_at" v-slot="{ componentField }">
            <FormItem>
              <FormLabel>Expiration Date (Optional)</FormLabel>
              <FormControl>
                <Input type="datetime-local" v-bind="componentField" />
              </FormControl>
              <FormDescription>
                Leave empty for permanent admin access. Set a date for temporary access.
              </FormDescription>
              <FormMessage />
            </FormItem>
          </FormField>

          <FormField name="notes" v-slot="{ componentField }">
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
          <Button type="button" variant="outline" @click="open = false">
            Cancel
          </Button>
          <Button type="submit" :disabled="saving">
            {{
              mode === "edit"
                ? saving
                  ? "Updating..."
                  : "Update Notes"
                : saving
                  ? "Adding..."
                  : "Add Admin"
            }}
          </Button>
        </DialogFooter>
      </form>
    </DialogContent>
  </Dialog>
</template>
