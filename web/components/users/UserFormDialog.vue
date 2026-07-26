<script setup lang="ts">
import { computed, ref, watch } from "vue";
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
import type { User } from "~/components/users/types";

const props = defineProps<{
  mode: "create" | "edit";
  user?: User | null;
}>();

const emit = defineEmits<{
  (e: "saved"): void;
}>();

const open = defineModel<boolean>("open", { default: false });

const runtimeConfig = useRuntimeConfig();
const submitting = ref(false);

// Form schema for adding a user
const createSchema = toTypedSchema(
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
const editSchema = toTypedSchema(
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

const validationSchema = computed(() =>
  props.mode === "create" ? createSchema : editSchema
);

const form = useForm<Record<string, any>>({
  validationSchema,
  initialValues: {
    steam_id: "",
    name: "",
    username: "",
    password: "",
    superAdmin: false,
  },
});

// Prefill on open (edit) or reset to blank values (create)
watch(open, (isOpen) => {
  if (!isOpen) {
    form.resetForm();
    return;
  }
  if (props.mode === "edit" && props.user) {
    form.resetForm({
      values: {
        steam_id: props.user.steam_id ?? "",
        name: props.user.name,
        superAdmin: props.user.super_admin,
      },
    });
  } else {
    form.resetForm({
      values: {
        steam_id: "",
        name: "",
        username: "",
        password: "",
        superAdmin: false,
      },
    });
  }
});

const onSubmit = form.handleSubmit(async (values: any) => {
  if (props.mode === "create") {
    await addUser(values);
  } else {
    await editUser(values);
  }
});

async function addUser(values: any) {
  const { steam_id, name, username, password, superAdmin } = values;

  submitting.value = true;

  try {
    await useAuthFetchImperative(
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

    open.value = false;

    toast({
      title: "Success",
      description: "User added successfully",
    });

    emit("saved");
  } catch (err: any) {
    toast({
      title: "Failed to Add User",
      description: extractApiErrorMessage(
        err,
        "An error occurred while adding the user"
      ),
      variant: "destructive",
    });
  } finally {
    submitting.value = false;
  }
}

async function editUser(values: any) {
  if (!props.user) return;

  const { steam_id, name, superAdmin } = values;

  submitting.value = true;

  try {
    await useAuthFetchImperative(
      `${runtimeConfig.public.backendApi}/users/${props.user.id}`,
      {
        method: "PUT",
        body: {
          steam_id,
          name,
          super_admin: superAdmin,
        },
      }
    );

    open.value = false;

    toast({
      title: "Success",
      description: "User updated successfully",
    });

    emit("saved");
  } catch (err: any) {
    toast({
      title: "Failed to Update User",
      description: extractApiErrorMessage(
        err,
        "An error occurred while updating the user"
      ),
      variant: "destructive",
    });
  } finally {
    submitting.value = false;
  }
}
</script>

<template>
  <Dialog v-model:open="open">
    <DialogContent
      class="w-[95vw] sm:max-w-[425px] max-h-[90vh] overflow-y-auto p-4 sm:p-6"
    >
      <DialogHeader>
        <DialogTitle class="text-base sm:text-lg">
          {{ mode === "create" ? "Add New User" : "Edit User" }}
        </DialogTitle>
        <DialogDescription class="text-xs sm:text-sm">
          {{
            mode === "create"
              ? "Enter the details of the user you want to add."
              : "Update the user's information."
          }}
        </DialogDescription>
      </DialogHeader>
      <form @submit="onSubmit">
        <div class="grid gap-4 py-4">
          <template v-if="mode === 'create'">
            <FormField v-slot="{ componentField }" name="steam_id">
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
            <FormField v-slot="{ componentField }" name="name">
              <FormItem>
                <FormLabel>Name</FormLabel>
                <FormControl>
                  <Input placeholder="John Doe" v-bind="componentField" />
                </FormControl>
                <FormMessage />
              </FormItem>
            </FormField>
            <FormField v-slot="{ componentField }" name="username">
              <FormItem>
                <FormLabel>Username</FormLabel>
                <FormControl>
                  <Input placeholder="johndoe" v-bind="componentField" />
                </FormControl>
                <FormDescription>
                  Must be 1-32 characters, lowercase, and only contain a-z,
                  0-9, and _
                </FormDescription>
                <FormMessage />
              </FormItem>
            </FormField>
            <FormField v-slot="{ componentField }" name="password">
              <FormItem>
                <FormLabel>Password</FormLabel>
                <FormControl>
                  <Input
                    type="password"
                    placeholder="********"
                    v-bind="componentField"
                  />
                </FormControl>
                <FormDescription> Must be at least 8 characters </FormDescription>
                <FormMessage />
              </FormItem>
            </FormField>
          </template>
          <template v-else>
            <FormField v-slot="{ componentField }" name="name">
              <FormItem>
                <FormLabel>Name</FormLabel>
                <FormControl>
                  <Input placeholder="John Doe" v-bind="componentField" />
                </FormControl>
                <FormMessage />
              </FormItem>
            </FormField>
            <FormField v-slot="{ value, handleChange }" name="steam_id">
              <FormItem>
                <FormLabel>Steam ID</FormLabel>
                <FormControl>
                  <Input
                    placeholder="76561198012345678"
                    :model-value="value"
                    @update:model-value="
                      (v) => handleChange(String(v).replace(/[^0-9]/g, ''))
                    "
                  />
                </FormControl>
                <FormDescription> 17-digit Steam ID (optional) </FormDescription>
                <FormMessage />
              </FormItem>
            </FormField>
          </template>
          <FormField v-slot="{ value, handleChange }" name="superAdmin">
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
          <Button type="button" variant="outline" @click="open = false">
            Cancel
          </Button>
          <Button type="submit" :disabled="submitting">
            {{
              mode === "create"
                ? submitting
                  ? "Adding..."
                  : "Add User"
                : submitting
                  ? "Updating..."
                  : "Update User"
            }}
          </Button>
        </DialogFooter>
      </form>
    </DialogContent>
  </Dialog>
</template>
