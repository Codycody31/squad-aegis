<script setup lang="ts">
import { ref, onMounted, computed, watch } from "vue";
import { Button } from "~/components/ui/button";
import { Input } from "~/components/ui/input";
import { Textarea } from "~/components/ui/textarea";
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
  CardDescription,
} from "~/components/ui/card";
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
import { Switch } from "~/components/ui/switch";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "~/components/ui/select";
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from "~/components/ui/tooltip";
import { useForm } from "vee-validate";
import { toTypedSchema } from "@vee-validate/zod";
import * as z from "zod";
import { AutoForm } from "~/components/ui/auto-form";

const route = useRoute();
const serverId = route.params.serverId;

const loading = ref({
  extensions: true,
  types: true,
});
const error = ref<string | null>(null);
const extensions = ref<Extension[]>([]);
const extensionTypes = ref<Record<string, ExtensionConfigSchema>>({});
const extensionDefinitions = ref<ExtensionDefinition[]>([]);
const showAddExtensionDialog = ref(false);
const showEditExtensionDialog = ref(false);
const showViewDetailsDialog = ref(false);
const selectedExtension = ref<Extension | null>(null);
const extensionAction = ref<"add" | "edit">("add");
const actionLoading = ref(false);

// Interfaces
interface Extension {
  id: string;
  server_id: string;
  name: string;
  enabled: boolean;
  config: Record<string, any>;
}

interface ExtensionConfigField {
  description: string;
  required: boolean;
  type: string;
  default?: any;
  options?: any[];
  nested?: {
    name: string;
    description: string;
    required: boolean;
    type: string;
    default?: any;
  }[];
}

type ExtensionConfigSchema = Record<string, ExtensionConfigField>;

interface ExtensionDefinition {
  id: string;
  name: string;
  description: string;
  version: string;
  author: string;
  schema: ExtensionConfigSchema;
}

interface ExtensionDefinitionsResponse {
  definitions: ExtensionDefinition[];
}

interface ExtensionListResponse {
  extensions: Extension[];
}

// Dynamic form schema based on selected extension type
const createExtensionFormSchema = computed(() => {
  const baseSchema = {
    name: z.string().min(1, "Extension type is required"),
    enabled: z.boolean().default(true),
    config: z.record(z.string(), z.any()).default({}),
  };

  // If a type is selected, add config fields based on the schema
  if (extensionAction.value === "add" && addFormValues.value.name) {
    const selectedType = addFormValues.value.name;
    const typeSchema = extensionTypes.value[selectedType];

    if (!typeSchema) {
      return z.object(baseSchema);
    }

    const configSchema: Record<string, any> = {};

    // Add each config field to the schema
    for (const [fieldName, field] of Object.entries(typeSchema)) {
      let fieldSchema: z.ZodTypeAny = z.any();

      if (field.type === "string") {
        fieldSchema = field.required
          ? z.string().min(1, `${fieldName} is required`)
          : z.string().optional();
      } else if (field.type === "number") {
        fieldSchema = field.required
          ? z
              .number({ coerce: true })
              .min(0, `${fieldName} must be a positive number`)
          : z.number({ coerce: true }).optional();
      } else if (field.type === "boolean") {
        fieldSchema = z.boolean().default(!!field.default);
      } else if (field.type === "array") {
        fieldSchema = z.array(z.string()).default([]);
      } else if (field.type === "object") {
        fieldSchema = z.record(z.string(), z.any()).default({});
      }

      // Apply default values if they exist
      if (field.default !== undefined && field.type !== "boolean") {
        fieldSchema = fieldSchema.default(field.default);
      }

      configSchema[fieldName] = fieldSchema;
    }

    return z.object({
      ...baseSchema,
      config: z.object(configSchema).default({}),
    });
  } else if (extensionAction.value === "edit") {
    // For edit form, we don't require name since it's pre-selected
    return z.object(baseSchema);
  }

  // Default basic schema
  return z.object(baseSchema);
});

// Initial form values
const addFormValues = ref({
  name: "",
  enabled: true,
  config: {} as Record<string, any>,
});

const editFormValues = ref({
  name: "",
  enabled: true,
  config: {} as Record<string, any>,
});

// Extension field config for AutoForm
const extensionFieldConfig = computed(() => {
  const config: any = {
    name: {
      label: "Extension Type",
      description: "Choose the type of extension to install",
      inputProps: {
        disabled: extensionAction.value === "edit",
        options: extensionDefinitions.value.map((def) => ({
          label: def.name,
          value: def.id,
        })),
      },
    },
    enabled: {
      label: "Enable Extension",
      description: "Enable the extension immediately after installation",
    },
  };

  // Add config field configurations based on selected type
  if (
    (extensionAction.value === "add" && addFormValues.value.name) ||
    (extensionAction.value === "edit" && selectedExtension.value)
  ) {
    const typeName =
      extensionAction.value === "add"
        ? addFormValues.value.name
        : selectedExtension.value?.name || "";

    const typeSchema = extensionTypes.value[typeName];

    if (typeSchema) {
      config.config = {
        label: "Configuration",
        description: "Configure the extension settings",
      };
    }
  }

  return config;
});

// Function to render form fields based on extension type schema
function renderConfigFields(schema: ExtensionConfigSchema) {
  if (!schema) return [];
  return Object.entries(schema);
}

function getNestedFields(field: ExtensionConfigField) {
  if (field.nested && field.nested.length > 0) {
    return field.nested;
  }
  return [];
}

// Function to fetch extension types
async function fetchExtensionTypes() {
  loading.value.types = true;
  error.value = null;

  const runtimeConfig = useRuntimeConfig();
  const cookieToken = useCookie(
    runtimeConfig.public.sessionCookieName as string
  );
  const token = cookieToken.value;

  if (!token) {
    error.value = "Authentication required";
    loading.value.types = false;
    return;
  }

  try {
    const { data, error: fetchError } = await useFetch(
      `${runtimeConfig.public.backendApi}/extensions/definitions`,
      {
        method: "GET",
        headers: {
          Authorization: `Bearer ${token}`,
        },
      }
    );

    if (fetchError.value) {
      throw new Error(
        fetchError.value.message || "Failed to fetch extension definitions"
      );
    }

    if (data.value) {
      extensionDefinitions.value = data.value.data.definitions;

      // Convert extensionDefinitions to extensionTypes format
      const typesMap: Record<string, ExtensionConfigSchema> = {};
      for (const def of extensionDefinitions.value) {
        typesMap[def.id] = def.schema;
      }
      extensionTypes.value = typesMap;
    }
  } catch (err: any) {
    error.value =
      err.message || "An error occurred while fetching extension definitions";
    console.error(err);
  } finally {
    loading.value.types = false;
  }
}

// Function to fetch server extensions
async function fetchExtensions() {
  loading.value.extensions = true;
  error.value = null;

  const runtimeConfig = useRuntimeConfig();
  const cookieToken = useCookie(
    runtimeConfig.public.sessionCookieName as string
  );
  const token = cookieToken.value;

  if (!token) {
    error.value = "Authentication required";
    loading.value.extensions = false;
    return;
  }

  try {
    const { data, error: fetchError } = await useFetch(
      `${runtimeConfig.public.backendApi}/servers/${serverId}/extensions`,
      {
        method: "GET",
        headers: {
          Authorization: `Bearer ${token}`,
        },
      }
    );

    if (fetchError.value) {
      throw new Error(fetchError.value.message || "Failed to fetch extensions");
    }

    if (data.value) {
      extensions.value = data.value.data.extensions;
    }
  } catch (err: any) {
    error.value = err.message || "An error occurred while fetching extensions";
    console.error(err);
  } finally {
    loading.value.extensions = false;
  }
}

// Function to add extension
async function addExtension(values: any) {
  actionLoading.value = true;
  error.value = null;

  const runtimeConfig = useRuntimeConfig();
  const cookieToken = useCookie(
    runtimeConfig.public.sessionCookieName as string
  );
  const token = cookieToken.value;

  if (!token) {
    error.value = "Authentication required";
    actionLoading.value = false;
    return;
  }

  try {
    const { data, error: fetchError } = await useFetch(
      `${runtimeConfig.public.backendApi}/servers/${serverId}/extensions`,
      {
        method: "POST",
        body: {
          name: values.name,
          enabled: values.enabled,
          config: values.config || {},
        },
        headers: {
          Authorization: `Bearer ${token}`,
        },
      }
    );

    if (fetchError.value) {
      throw new Error(
        fetchError.value.data?.message || fetchError.value.message
      );
    }

    // Reset form and close dialog
    addFormValues.value = {
      name: "",
      enabled: true,
      config: {},
    };
    showAddExtensionDialog.value = false;

    // Refresh the extensions list
    fetchExtensions();
  } catch (err: any) {
    error.value = err.message || "An error occurred while adding the extension";
    console.error(err);
  } finally {
    actionLoading.value = false;
  }
}

// Watch for changes to the selected extension type in Add form
watch(
  () => addFormValues.value.name,
  (newType) => {
    if (newType && extensionTypes.value[newType]) {
      // Initialize config fields with default values
      const configFields = extensionTypes.value[newType];
      const newConfig: Record<string, any> = {};

      for (const [fieldName, field] of Object.entries(configFields)) {
        if (field.default !== undefined) {
          newConfig[fieldName] = field.default;
        } else if (field.type === "bool" || field.type === "boolean") {
          newConfig[fieldName] = false;
        } else if (field.type === "int" || field.type === "number") {
          newConfig[fieldName] = 0;
        } else if (field.type === "string") {
          newConfig[fieldName] = "";
        } else if (field.type === "array" || field.type.startsWith("array")) {
          newConfig[fieldName] = [];
        } else if (field.type === "object") {
          newConfig[fieldName] = {};
        }
      }

      addFormValues.value.config = newConfig;
    } else {
      addFormValues.value.config = {};
    }
  }
);

// Function to update extension
async function updateExtension(values: any) {
  if (!selectedExtension.value) return;

  actionLoading.value = true;
  error.value = null;

  const runtimeConfig = useRuntimeConfig();
  const cookieToken = useCookie(
    runtimeConfig.public.sessionCookieName as string
  );
  const token = cookieToken.value;

  if (!token) {
    error.value = "Authentication required";
    actionLoading.value = false;
    return;
  }

  try {
    const { data, error: fetchError } = await useFetch(
      `${runtimeConfig.public.backendApi}/servers/${serverId}/extensions/${selectedExtension.value.id}`,
      {
        method: "PUT",
        body: {
          enabled: values.enabled,
          config: values.config || {},
        },
        headers: {
          Authorization: `Bearer ${token}`,
        },
      }
    );

    if (fetchError.value) {
      throw new Error(
        fetchError.value.data?.message || fetchError.value.message
      );
    }

    // Reset form and close dialog
    editFormValues.value = {
      name: "",
      enabled: true,
      config: {},
    };
    showEditExtensionDialog.value = false;
    selectedExtension.value = null;

    // Refresh the extensions list
    fetchExtensions();
  } catch (err: any) {
    error.value =
      err.message || "An error occurred while updating the extension";
    console.error(err);
  } finally {
    actionLoading.value = false;
  }
}

// Function to delete extension
async function deleteExtension(extensionId: string) {
  if (!confirm("Are you sure you want to delete this extension?")) {
    return;
  }

  loading.value.extensions = true;
  error.value = null;

  const runtimeConfig = useRuntimeConfig();
  const cookieToken = useCookie(
    runtimeConfig.public.sessionCookieName as string
  );
  const token = cookieToken.value;

  if (!token) {
    error.value = "Authentication required";
    loading.value.extensions = false;
    return;
  }

  try {
    const { data, error: fetchError } = await useFetch(
      `${runtimeConfig.public.backendApi}/servers/${serverId}/extensions/${extensionId}`,
      {
        method: "DELETE",
        headers: {
          Authorization: `Bearer ${token}`,
        },
      }
    );

    if (fetchError.value) {
      throw new Error(
        fetchError.value.data?.message || fetchError.value.message
      );
    }

    // Refresh the extensions list
    fetchExtensions();
  } catch (err: any) {
    error.value =
      err.message || "An error occurred while deleting the extension";
    console.error(err);
  } finally {
    loading.value.extensions = false;
  }
}

// Function to toggle extension enabled status
async function toggleExtension(extensionId: string) {
  loading.value.extensions = true;
  error.value = null;

  const runtimeConfig = useRuntimeConfig();
  const cookieToken = useCookie(
    runtimeConfig.public.sessionCookieName as string
  );
  const token = cookieToken.value;

  if (!token) {
    error.value = "Authentication required";
    loading.value.extensions = false;
    return;
  }

  try {
    const { data, error: fetchError } = await useFetch(
      `${runtimeConfig.public.backendApi}/servers/${serverId}/extensions/${extensionId}/toggle`,
      {
        method: "POST",
        headers: {
          Authorization: `Bearer ${token}`,
        },
      }
    );

    if (fetchError.value) {
      throw new Error(
        fetchError.value.data?.message || fetchError.value.message
      );
    }

    // Refresh the extensions list
    fetchExtensions();
  } catch (err: any) {
    error.value =
      err.message || "An error occurred while toggling the extension";
    console.error(err);
  } finally {
    loading.value.extensions = false;
  }
}

// Function to open edit dialog
function openEditDialog(extension: Extension) {
  selectedExtension.value = extension;
  extensionAction.value = "edit";

  // Set form values with deep copy to ensure reactivity
  editFormValues.value = {
    name: extension.name,
    enabled: extension.enabled,
    config: {},
  };

  // Initialize config fields with values from the extension or defaults
  if (extension.name && extensionTypes.value[extension.name]) {
    const configFields = extensionTypes.value[extension.name];
    const newConfig: Record<string, any> = {};

    for (const [fieldName, field] of Object.entries(configFields)) {
      if (extension.config && fieldName in extension.config) {
        newConfig[fieldName] = extension.config[fieldName];
      } else if (field.default !== undefined) {
        newConfig[fieldName] = field.default;
      } else if (field.type === "bool" || field.type === "boolean") {
        newConfig[fieldName] = false;
      } else if (field.type === "int" || field.type === "number") {
        newConfig[fieldName] = 0;
      } else if (field.type === "string") {
        newConfig[fieldName] = "";
      } else if (field.type === "array" || field.type.startsWith("array")) {
        newConfig[fieldName] = [];
      } else if (field.type === "object") {
        newConfig[fieldName] = {};
      }
    }

    editFormValues.value.config = newConfig;
  }

  showEditExtensionDialog.value = true;
}

// Function to get extension name from its ID
function getExtensionNameById(extensionId: string): string {
  const definition = extensionDefinitions.value.find(
    (def) => def.id === extensionId
  );
  return definition ? definition.name : extensionId;
}

// Function to display config as a string
function formatConfig(config: Record<string, any>): string {
  return JSON.stringify(config, null, 2);
}

// Format date
function formatDate(dateString: string): string {
  return new Date(dateString).toLocaleString();
}

// Function to get the array item type from an array field type
function getArrayItemType(fieldType: string): string {
  if (fieldType.startsWith("array")) {
    const subType = fieldType.substring(5).toLowerCase(); // Remove 'array' prefix
    if (subType === "string" || subType === "int" || subType === "bool") {
      return subType;
    }
  }
  return "string"; // Default to string if no specific type is found
}

// Function to handle array item addition
function addArrayItem(
  config: Record<string, any>,
  fieldName: string,
  itemType: string
) {
  if (!Array.isArray(config[fieldName])) {
    config[fieldName] = [];
  }

  let newValue: any = "";
  if (itemType === "int") {
    newValue = 0;
  } else if (itemType === "bool") {
    newValue = false;
  }

  config[fieldName].push(newValue);
}

// Function to handle array item removal
function removeArrayItem(
  config: Record<string, any>,
  fieldName: string,
  index: number
) {
  if (
    Array.isArray(config[fieldName]) &&
    index >= 0 &&
    index < config[fieldName].length
  ) {
    config[fieldName].splice(index, 1);
  }
}

// Function to get extension detail by ID
function getExtensionDetailById(extensionId: string, detail: string): string {
  const definition = extensionDefinitions.value.find(
    (def) => def.id === extensionId
  );
  if (definition) {
    if (detail === "version") {
      return definition.version;
    } else if (detail === "author") {
      return definition.author;
    } else if (detail === "description") {
      return definition.description;
    }
  }
  return "";
}

// Function to open view details dialog
function openViewDetails(extension: Extension) {
  selectedExtension.value = extension;
  showViewDetailsDialog.value = true;
}

// Setup initial data load
onMounted(() => {
  fetchExtensionTypes();
  fetchExtensions();
});
</script>

<template>
  <div class="p-4">
    <div class="flex justify-between items-center mb-4">
      <h1 class="text-2xl font-bold">Server Extensions</h1>
      <Button @click="showAddExtensionDialog = true" :disabled="loading.types">
        Add Extension
      </Button>
    </div>

    <div v-if="error" class="bg-red-500 text-white p-4 rounded mb-4">
      {{ error }}
    </div>

    <Card>
      <CardHeader>
        <CardTitle>Installed Extensions</CardTitle>
        <CardDescription>
          Extensions provide additional functionality for your server.
        </CardDescription>
      </CardHeader>
      <CardContent>
        <div v-if="loading.extensions" class="text-center py-8">
          <div
            class="animate-spin h-8 w-8 border-4 border-primary border-t-transparent rounded-full mx-auto mb-4"
          ></div>
          <p>Loading extensions...</p>
        </div>

        <div v-else-if="extensions.length === 0" class="text-center py-8">
          <p>No extensions installed. Add an extension to get started.</p>
        </div>

        <div v-else class="overflow-x-auto">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Name</TableHead>
                <TableHead>Status</TableHead>
                <TableHead>Configuration</TableHead>
                <TableHead class="text-right">Actions</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              <TableRow
                v-for="extension in extensions"
                :key="extension.id"
                class="hover:bg-muted/50"
              >
                <TableCell class="font-medium">
                  <div>{{ getExtensionNameById(extension.name) }}</div>
                  <div class="text-xs text-muted-foreground">
                    {{ getExtensionDetailById(extension.name, "version") }} by
                    {{ getExtensionDetailById(extension.name, "author") }}
                  </div>
                </TableCell>
                <TableCell>
                  <Badge
                    :variant="extension.enabled ? 'default' : 'outline'"
                    class="text-xs"
                  >
                    {{ extension.enabled ? "Enabled" : "Disabled" }}
                  </Badge>
                </TableCell>
                <TableCell>
                  <code class="text-xs block max-h-32 overflow-y-auto">
                    {{ formatConfig(extension.config) }}
                  </code>
                </TableCell>
                <TableCell class="text-right">
                  <div class="flex space-x-2 justify-end">
                    <Button
                      variant="outline"
                      size="sm"
                      @click="toggleExtension(extension.id)"
                      :disabled="loading.extensions"
                    >
                      {{ extension.enabled ? "Disable" : "Enable" }}
                    </Button>
                    <Button
                      variant="outline"
                      size="sm"
                      @click="openViewDetails(extension)"
                      :disabled="loading.extensions"
                    >
                      Details
                    </Button>
                    <Button
                      variant="outline"
                      size="sm"
                      @click="openEditDialog(extension)"
                      :disabled="loading.extensions"
                    >
                      Edit
                    </Button>
                    <Button
                      variant="destructive"
                      size="sm"
                      @click="deleteExtension(extension.id)"
                      :disabled="loading.extensions"
                    >
                      Delete
                    </Button>
                  </div>
                </TableCell>
              </TableRow>
            </TableBody>
          </Table>
        </div>
      </CardContent>
    </Card>

    <Card class="mt-6">
      <CardHeader>
        <CardTitle>About Extensions</CardTitle>
      </CardHeader>
      <CardContent>
        <p class="text-sm text-muted-foreground">
          Extensions provide additional functionality for your Squad server.
          They can add features like auto-moderation, statistics tracking,
          custom commands, and more.
        </p>
        <p class="text-sm text-muted-foreground mt-2">
          <strong>Adding Extensions</strong> - Choose from the available
          extension types and configure them according to your needs.
        </p>
        <p class="text-sm text-muted-foreground mt-2">
          <strong>Configuration</strong> - Each extension has its own
          configuration options. Make sure to review and adjust these settings
          to suit your server.
        </p>
        <p class="text-sm text-muted-foreground mt-2">
          <strong>Enabling/Disabling</strong> - You can enable or disable
          extensions without removing them, which is useful for testing or
          temporarily disabling functionality.
        </p>
      </CardContent>
    </Card>

    <Card class="mt-6" v-if="Object.keys(extensionDefinitions).length > 0">
      <CardHeader>
        <CardTitle>Available Extensions</CardTitle>
        <CardDescription>
          These extensions can be installed and configured for your server.
        </CardDescription>
      </CardHeader>
      <CardContent>
        <div class="space-y-4">
          <div
            v-for="extension in extensionDefinitions"
            :key="extension.id"
            class="p-4 border rounded-md hover:bg-muted/50 transition-colors"
          >
            <div class="flex justify-between items-start">
              <div>
                <h3 class="font-medium text-lg">
                  {{ extension.name || extension.id }}
                </h3>
                <p class="text-sm text-muted-foreground mt-1">
                  {{ extension.description || "No description available" }}
                </p>
              </div>
              <Button
                variant="outline"
                size="sm"
                @click="
                  () => {
                    addFormValues.name = extension.id;
                    showAddExtensionDialog = true;
                  }
                "
              >
                Add This Extension
              </Button>
            </div>
          </div>
        </div>
      </CardContent>
    </Card>

    <!-- Add Extension Dialog -->
    <Dialog
      v-model:open="showAddExtensionDialog"
      @update:open="
        (open) => {
          if (!open) addFormValues.config = {};
        }
      "
    >
      <DialogContent class="sm:max-w-[600px] max-h-[80vh] overflow-y-auto">
        <DialogHeader>
          <DialogTitle>Add New Extension</DialogTitle>
          <DialogDescription>
            Install an extension to add new functionality to your server.
          </DialogDescription>
        </DialogHeader>

        <div class="py-4">
          <form @submit.prevent="addExtension(addFormValues)">
            <div class="space-y-4">
              <FormField name="extensionType">
                <FormLabel>Extension Type</FormLabel>
                <FormControl>
                  <Select v-model="addFormValues.name">
                    <SelectTrigger class="w-full">
                      <SelectValue placeholder="Select extension type" />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem
                        v-for="def in extensionDefinitions"
                        :key="def.id"
                        :value="def.id"
                      >
                        {{ def.name }}
                      </SelectItem>
                    </SelectContent>
                  </Select>
                </FormControl>
                <FormDescription
                  >Choose the type of extension to install</FormDescription
                >
              </FormField>

              <!-- Selected extension details -->
              <div
                v-if="addFormValues.name"
                class="mb-4 p-3 bg-muted rounded-md"
              >
                <div class="text-sm font-medium">
                  {{ getExtensionNameById(addFormValues.name) }}
                  <span class="text-xs text-muted-foreground ml-2">
                    v{{
                      getExtensionDetailById(addFormValues.name, "version")
                    }}
                    by
                    {{ getExtensionDetailById(addFormValues.name, "author") }}
                  </span>
                </div>
                <div class="text-sm mt-1">
                  {{
                    getExtensionDetailById(addFormValues.name, "description")
                  }}
                </div>
              </div>

              <FormField name="enabled">
                <FormLabel>Enable Extension</FormLabel>
                <FormControl>
                  <div class="flex items-center space-x-2">
                    <Switch v-model="addFormValues.enabled" />
                    <span class="text-sm text-muted-foreground"
                      >Enable the extension immediately after installation</span
                    >
                  </div>
                </FormControl>
              </FormField>

              <!-- Dynamic config fields based on selected extension type -->
              <div
                v-if="addFormValues.name && extensionTypes[addFormValues.name]"
                class="mt-6"
              >
                <h3 class="font-medium mb-4">Configuration</h3>
                <div class="space-y-4 border p-4 rounded-md">
                  <FormField
                    v-for="(field, fieldName) in extensionTypes[
                      addFormValues.name
                    ]"
                    :key="fieldName"
                    :name="`config.${fieldName}`"
                  >
                    <FormLabel>
                      <TooltipProvider>
                        <Tooltip>
                          <TooltipTrigger asChild>
                            <span class="flex items-center cursor-help">
                              {{ fieldName }}
                              <Icon
                                name="lucide:info"
                                class="h-4 w-4 ml-1 text-muted-foreground"
                              />
                            </span>
                          </TooltipTrigger>
                          <TooltipContent>
                            <p class="max-w-xs">{{ field.description }}</p>
                          </TooltipContent>
                        </Tooltip>
                      </TooltipProvider>
                    </FormLabel>
                    <FormControl>
                      <!-- String input -->
                      <Input
                        v-if="field.type === 'string'"
                        v-model="addFormValues.config[fieldName]"
                        :placeholder="`Enter ${fieldName}`"
                      />

                      <!-- Number input -->
                      <Input
                        v-else-if="
                          field.type === 'int' || field.type === 'number'
                        "
                        v-model.number="addFormValues.config[fieldName]"
                        type="number"
                      />

                      <!-- Boolean input -->
                      <div
                        v-else-if="
                          field.type === 'bool' || field.type === 'boolean'
                        "
                        class="flex items-center space-x-2"
                      >
                        <Switch v-model="addFormValues.config[fieldName]" />
                        <span>{{
                          addFormValues.config[fieldName] ? "Yes" : "No"
                        }}</span>
                      </div>

                      <!-- Array input -->
                      <div
                        v-else-if="
                          field.type === 'array' ||
                          field.type.startsWith('array')
                        "
                        class="space-y-2"
                      >
                        <div
                          v-for="(item, index) in addFormValues.config[
                            fieldName
                          ] || []"
                          :key="`${fieldName}-${index}`"
                          class="flex items-center space-x-2"
                        >
                          <!-- String array item -->
                          <Input
                            v-if="getArrayItemType(field.type) === 'string'"
                            v-model="addFormValues.config[fieldName][index]"
                            :placeholder="`Enter item ${index + 1}`"
                            class="flex-grow"
                          />

                          <!-- Number array item -->
                          <Input
                            v-else-if="getArrayItemType(field.type) === 'int'"
                            v-model.number="
                              addFormValues.config[fieldName][index]
                            "
                            type="number"
                            class="flex-grow"
                          />

                          <!-- Boolean array item -->
                          <div
                            v-else-if="getArrayItemType(field.type) === 'bool'"
                            class="flex items-center space-x-2 flex-grow"
                          >
                            <Switch
                              v-model="addFormValues.config[fieldName][index]"
                            />
                            <span>{{
                              addFormValues.config[fieldName][index]
                                ? "Yes"
                                : "No"
                            }}</span>
                          </div>

                          <!-- Remove item button -->
                          <Button
                            type="button"
                            variant="outline"
                            size="sm"
                            @click="
                              removeArrayItem(
                                addFormValues.config,
                                fieldName,
                                index
                              )
                            "
                          >
                            <Icon name="lucide:trash-2" />
                          </Button>
                        </div>

                        <!-- Add item button -->
                        <Button
                          type="button"
                          variant="outline"
                          size="sm"
                          @click="
                            addArrayItem(
                              addFormValues.config,
                              fieldName,
                              getArrayItemType(field.type)
                            )
                          "
                          class="mt-2"
                        >
                          <Icon name="lucide:plus" />
                          Add Item
                        </Button>
                      </div>

                      <!-- Select input for options -->
                      <Select
                        v-else-if="field.options && field.options.length"
                        v-model="addFormValues.config[fieldName]"
                      >
                        <SelectTrigger class="w-full">
                          <SelectValue :placeholder="`Select ${fieldName}`" />
                        </SelectTrigger>
                        <SelectContent>
                          <SelectItem
                            v-for="option in field.options"
                            :key="option"
                            :value="option"
                          >
                            {{ option }}
                          </SelectItem>
                        </SelectContent>
                      </Select>
                    </FormControl>
                    <FormDescription v-if="field.description">{{
                      field.description
                    }}</FormDescription>
                  </FormField>
                </div>
              </div>
            </div>

            <DialogFooter class="mt-6">
              <Button
                type="button"
                variant="outline"
                @click="showAddExtensionDialog = false"
              >
                Cancel
              </Button>
              <Button type="submit" :disabled="actionLoading">
                {{ actionLoading ? "Adding..." : "Add Extension" }}
              </Button>
            </DialogFooter>
          </form>
        </div>
      </DialogContent>
    </Dialog>

    <!-- Edit Extension Dialog -->
    <Dialog v-model:open="showEditExtensionDialog">
      <DialogContent class="sm:max-w-[600px] max-h-[80vh] overflow-y-auto">
        <DialogHeader>
          <DialogTitle>Edit Extension</DialogTitle>
          <DialogDescription v-if="selectedExtension">
            Update the configuration for {{ selectedExtension.name }}
          </DialogDescription>
        </DialogHeader>

        <div class="py-4" v-if="selectedExtension">
          <form @submit.prevent="updateExtension(editFormValues)">
            <div class="space-y-4">
              <FormField name="extensionType">
                <FormLabel>Extension Type</FormLabel>
                <FormControl>
                  <Input v-model="editFormValues.name" disabled />
                </FormControl>
                <FormDescription
                  >Extension type cannot be changed after
                  installation</FormDescription
                >
              </FormField>

              <!-- Selected extension details -->
              <div
                v-if="selectedExtension"
                class="mb-4 p-3 bg-muted rounded-md"
              >
                <div class="text-sm font-medium">
                  {{ getExtensionNameById(selectedExtension.name) }}
                  <span class="text-xs text-muted-foreground ml-2">
                    v{{
                      getExtensionDetailById(selectedExtension.name, "version")
                    }}
                    by
                    {{
                      getExtensionDetailById(selectedExtension.name, "author")
                    }}
                  </span>
                </div>
                <div class="text-sm mt-1">
                  {{
                    getExtensionDetailById(
                      selectedExtension.name,
                      "description"
                    )
                  }}
                </div>
              </div>

              <FormField name="enabled">
                <FormLabel>Enable Extension</FormLabel>
                <FormControl>
                  <div class="flex items-center space-x-2">
                    <Switch v-model="editFormValues.enabled" />
                    <span class="text-sm text-muted-foreground"
                      >Enable or disable this extension</span
                    >
                  </div>
                </FormControl>
              </FormField>

              <!-- Dynamic config fields based on extension type -->
              <div
                v-if="
                  selectedExtension && extensionTypes[selectedExtension.name]
                "
                class="mt-6"
              >
                <h3 class="font-medium mb-4">Configuration</h3>
                <div class="space-y-4 border p-4 rounded-md">
                  <FormField
                    v-for="(field, fieldName) in extensionTypes[
                      selectedExtension.name
                    ]"
                    :key="fieldName"
                    :name="`config.${fieldName}`"
                  >
                    <FormLabel>
                      <TooltipProvider>
                        <Tooltip>
                          <TooltipTrigger asChild>
                            <span class="flex items-center cursor-help">
                              {{ fieldName }}
                              <Icon
                                name="lucide:info"
                                class="h-4 w-4 ml-1 text-muted-foreground"
                              />
                            </span>
                          </TooltipTrigger>
                          <TooltipContent>
                            <p class="max-w-xs">{{ field.description }}</p>
                          </TooltipContent>
                        </Tooltip>
                      </TooltipProvider>
                    </FormLabel>
                    <FormControl>
                      <!-- String input -->
                      <Input
                        v-if="field.type === 'string'"
                        v-model="editFormValues.config[fieldName]"
                        :placeholder="`Enter ${fieldName}`"
                      />

                      <!-- Number input -->
                      <Input
                        v-else-if="
                          field.type === 'int' || field.type === 'number'
                        "
                        v-model.number="editFormValues.config[fieldName]"
                        type="number"
                      />

                      <!-- Boolean input -->
                      <div
                        v-else-if="
                          field.type === 'bool' || field.type === 'boolean'
                        "
                        class="flex items-center space-x-2"
                      >
                        <Switch v-model="editFormValues.config[fieldName]" />
                        <span>{{
                          editFormValues.config[fieldName] ? "Yes" : "No"
                        }}</span>
                      </div>

                      <!-- Array input -->
                      <div
                        v-else-if="
                          field.type === 'array' ||
                          field.type.startsWith('array')
                        "
                        class="space-y-2"
                      >
                        <div
                          v-for="(item, index) in editFormValues.config[
                            fieldName
                          ] || []"
                          :key="`${fieldName}-${index}`"
                          class="flex items-center space-x-2"
                        >
                          <!-- String array item -->
                          <Input
                            v-if="getArrayItemType(field.type) === 'string'"
                            v-model="editFormValues.config[fieldName][index]"
                            :placeholder="`Enter item ${index + 1}`"
                            class="flex-grow"
                          />

                          <!-- Number array item -->
                          <Input
                            v-else-if="getArrayItemType(field.type) === 'int'"
                            v-model.number="
                              editFormValues.config[fieldName][index]
                            "
                            type="number"
                            class="flex-grow"
                          />

                          <!-- Boolean array item -->
                          <div
                            v-else-if="getArrayItemType(field.type) === 'bool'"
                            class="flex items-center space-x-2 flex-grow"
                          >
                            <Switch
                              v-model="editFormValues.config[fieldName][index]"
                            />
                            <span>{{
                              editFormValues.config[fieldName][index]
                                ? "Yes"
                                : "No"
                            }}</span>
                          </div>

                          <!-- Remove item button -->
                          <Button
                            type="button"
                            variant="outline"
                            size="sm"
                            @click="
                              removeArrayItem(
                                editFormValues.config,
                                fieldName,
                                index
                              )
                            "
                          >
                            <Icon name="lucide:trash" />
                          </Button>
                        </div>

                        <!-- Add item button -->
                        <Button
                          type="button"
                          variant="outline"
                          size="sm"
                          @click="
                            addArrayItem(
                              editFormValues.config,
                              fieldName,
                              getArrayItemType(field.type)
                            )
                          "
                          class="mt-2"
                        >
                          <Icon name="lucide:plus" />
                          Add Item
                        </Button>
                      </div>

                      <!-- Select input for options -->
                      <Select
                        v-else-if="field.options && field.options.length"
                        v-model="editFormValues.config[fieldName]"
                      >
                        <SelectTrigger class="w-full">
                          <SelectValue :placeholder="`Select ${fieldName}`" />
                        </SelectTrigger>
                        <SelectContent>
                          <SelectItem
                            v-for="option in field.options"
                            :key="option"
                            :value="option"
                          >
                            {{ option }}
                          </SelectItem>
                        </SelectContent>
                      </Select>
                    </FormControl>
                    <FormDescription v-if="field.description">{{
                      field.description
                    }}</FormDescription>
                  </FormField>
                </div>
              </div>
            </div>

            <DialogFooter class="mt-6">
              <Button
                type="button"
                variant="outline"
                @click="showEditExtensionDialog = false"
              >
                Cancel
              </Button>
              <Button type="submit" :disabled="actionLoading">
                {{ actionLoading ? "Updating..." : "Update Extension" }}
              </Button>
            </DialogFooter>
          </form>
        </div>
      </DialogContent>
    </Dialog>

    <!-- View Details Dialog -->
    <Dialog v-model:open="showViewDetailsDialog">
      <DialogContent class="sm:max-w-[600px] max-h-[80vh] overflow-y-auto">
        <DialogHeader>
          <DialogTitle v-if="selectedExtension">
            {{ getExtensionNameById(selectedExtension.name) }}
          </DialogTitle>
          <DialogDescription v-if="selectedExtension">
            <span class="text-sm">
              v{{
                getExtensionDetailById(selectedExtension.name, "version")
              }}
              by {{ getExtensionDetailById(selectedExtension.name, "author") }}
            </span>
          </DialogDescription>
        </DialogHeader>

        <div class="py-4" v-if="selectedExtension">
          <div class="space-y-4">
            <!-- Extension Description -->
            <div class="mb-4">
              <p>
                {{
                  getExtensionDetailById(selectedExtension.name, "description")
                }}
              </p>
            </div>

            <!-- Configuration Schema -->
            <div
              v-if="extensionTypes[selectedExtension.name]"
              class="border rounded-md p-4"
            >
              <h3 class="text-lg font-medium mb-3">Configuration Options</h3>

              <div class="space-y-4">
                <div
                  v-for="(field, fieldName) in extensionTypes[
                    selectedExtension.name
                  ]"
                  :key="fieldName"
                  class="pb-3 border-b border-gray-100 last:border-0"
                >
                  <div class="flex items-start justify-between">
                    <div>
                      <h4 class="text-sm font-medium">{{ fieldName }}</h4>
                      <p class="text-sm text-muted-foreground">
                        {{ field.description }}
                      </p>

                      <div class="mt-1 text-xs flex flex-wrap gap-2">
                        <Badge variant="outline">{{ field.type }}</Badge>
                        <Badge v-if="field.required" variant="default"
                          >Required</Badge
                        >
                        <Badge v-else variant="outline">Optional</Badge>
                      </div>
                    </div>

                    <div v-if="field.default !== undefined" class="text-sm">
                      <span class="text-xs text-muted-foreground"
                        >Default:</span
                      >
                      <code class="bg-muted px-1 rounded text-xs">{{
                        JSON.stringify(field.default)
                      }}</code>
                    </div>
                  </div>

                  <!-- Current Value -->
                  <div
                    v-if="
                      selectedExtension.config &&
                      fieldName in selectedExtension.config
                    "
                    class="mt-2"
                  >
                    <span class="text-xs text-muted-foreground"
                      >Current Value:</span
                    >
                    <code class="bg-muted px-1 rounded text-xs">{{
                      JSON.stringify(selectedExtension.config[fieldName])
                    }}</code>
                  </div>
                </div>
              </div>
            </div>
          </div>

          <DialogFooter class="mt-6">
            <Button
              type="button"
              variant="outline"
              @click="showViewDetailsDialog = false"
            >
              Close
            </Button>
            <Button
              type="button"
              @click="
                openEditDialog(selectedExtension);
                showViewDetailsDialog = false;
              "
            >
              Edit Configuration
            </Button>
          </DialogFooter>
        </div>
      </DialogContent>
    </Dialog>
  </div>
</template>
