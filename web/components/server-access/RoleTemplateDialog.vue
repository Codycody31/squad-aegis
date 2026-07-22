<script setup lang="ts">
import { computed, ref, watch } from "vue";
import { Badge } from "~/components/ui/badge";
import { Button } from "~/components/ui/button";
import { Input } from "~/components/ui/input";
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
  SelectLabel,
} from "~/components/ui/select";
import { toast } from "~/components/ui/toast";
import { useForm } from "vee-validate";
import { toTypedSchema } from "@vee-validate/zod";
import * as z from "zod";
import {
  formatPermissionDisplay,
  getPermissionCategoryColor,
  type PermissionGroup,
  type RoleTemplate,
} from "./shared";

const props = defineProps<{
  serverId: string;
  templates: RoleTemplate[];
  permissionGroups: PermissionGroup[];
}>();

const open = defineModel<boolean>("open", { default: false });

const emit = defineEmits<{
  (e: "saved"): void;
}>();

const templateFormSchema = toTypedSchema(
  z.object({
    template_id: z.string().min(1, "Please select a template"),
    name: z
      .string()
      .min(1, "Role name is required")
      .regex(
        /^[a-zA-Z0-9_]+$/,
        "Role name can only contain letters, numbers, and underscores."
      ),
  })
);

const form = useForm({
  validationSchema: templateFormSchema,
  initialValues: {
    template_id: "",
    name: "",
  },
});

const creating = ref(false);
const error = ref<string | null>(null);

// Watch for template selection to auto-fill name
watch(
  () => form.values.template_id,
  (newValue) => {
    if (newValue) {
      const template = props.templates.find((t) => t.id === newValue);
      if (template) {
        form.setFieldValue("name", template.name);
      }
    }
  }
);

watch(open, (isOpen) => {
  if (isOpen) {
    error.value = null;
    form.resetForm({ values: { template_id: "", name: "" } });
  }
});

const selectedTemplate = computed(() =>
  props.templates.find((t) => t.id === form.values.template_id)
);

// Function to create role from template
const onSubmit = form.handleSubmit(async (values) => {
  creating.value = true;
  error.value = null;

  const runtimeConfig = useRuntimeConfig();

  try {
    await useAuthFetchImperative(
      `${runtimeConfig.public.backendApi}/servers/${props.serverId}/roles/from-template`,
      {
        method: "POST",
        body: {
          template_id: values.template_id,
          name: values.name,
        },
      }
    );

    // Reset form and close dialog
    form.resetForm();
    open.value = false;

    toast({
      title: "Success",
      description: "Role created from template successfully",
    });

    emit("saved");
  } catch (err: any) {
    error.value = extractApiErrorMessage(
      err,
      "An error occurred while creating the role"
    );
    console.error(err);
  } finally {
    creating.value = false;
  }
});
</script>

<template>
  <Dialog v-model:open="open">
    <DialogContent
      class="w-[95vw] sm:max-w-[500px] max-h-[85vh] sm:max-h-[80vh] overflow-y-auto p-4 sm:p-6"
    >
      <DialogHeader>
        <DialogTitle class="text-base sm:text-lg">
          Create Role from Template
        </DialogTitle>
        <DialogDescription class="text-xs sm:text-sm">
          Select a predefined role template to quickly create a new role with standard permissions.
        </DialogDescription>
      </DialogHeader>

      <div v-if="error" class="bg-red-500 text-white p-3 rounded text-sm">
        {{ error }}
      </div>

      <form @submit="onSubmit">
        <div class="grid gap-4 py-4">
          <FormField name="template_id" v-slot="{ componentField }">
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
                      v-for="template in templates"
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
            v-if="selectedTemplate"
            class="border rounded-md p-3 bg-muted/50"
          >
            <div class="text-sm font-medium mb-2">Template Preview</div>
            <div class="text-xs text-muted-foreground mb-2">
              {{ selectedTemplate.description }}
            </div>
            <div class="flex flex-wrap gap-1">
              <Badge
                v-for="perm in selectedTemplate.permissions || []"
                :key="perm"
                variant="outline"
                :class="['text-xs', getPermissionCategoryColor(perm)]"
              >
                {{ formatPermissionDisplay(perm, permissionGroups) }}
              </Badge>
            </div>
          </div>

          <FormField name="name" v-slot="{ componentField }">
            <FormItem>
              <FormLabel>Role Name</FormLabel>
              <FormControl>
                <Input placeholder="e.g., SeniorAdmin" v-bind="componentField" />
              </FormControl>
              <FormDescription>
                Customize the name for this server's role.
              </FormDescription>
              <FormMessage />
            </FormItem>
          </FormField>
        </div>
        <DialogFooter>
          <Button type="button" variant="outline" @click="open = false">
            Cancel
          </Button>
          <Button type="submit" :disabled="creating">
            {{ creating ? "Creating..." : "Create Role" }}
          </Button>
        </DialogFooter>
      </form>
    </DialogContent>
  </Dialog>
</template>
