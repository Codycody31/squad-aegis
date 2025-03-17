<script setup lang="ts">
import { ref, onMounted, computed } from "vue";
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

const loading = ref({
  connectors: true,
  types: true,
});
const error = ref<string | null>(null);
const connectors = ref<Connector[]>([]);
const connectorTypes = ref<Record<string, ConnectorConfigSchema>>({});
const connectorInfo = ref<
  Record<string, { name: string; description: string }>
>({});
const showAddConnectorDialog = ref(false);
const showEditConnectorDialog = ref(false);
const showViewDetailsDialog = ref(false);
const selectedConnector = ref<Connector | null>(null);
const actionLoading = ref(false);

// Interfaces
interface Connector {
  id: string;
  name: string;
  config: Record<string, any>;
}

interface ConnectorConfigField {
  description: string;
  required: boolean;
  type: string;
  default?: any;
  options?: any[];
}

type ConnectorConfigSchema = Record<string, ConnectorConfigField>;

// Interface for connector definition response
interface ConnectorDefinitionResponse {
  data: {
    definitions: Array<{
      id: string;
      name: string;
      description: string;
      schema: ConnectorConfigSchema;
    }>;
  };
}

// Initial form values
const addFormValues = ref({
  type: "",
  config: {} as Record<string, any>,
});

// Dynamic form schema based on selected connector type
const createConnectorFormSchema = computed(() => {
  const baseSchema = {
    name: z.string().min(1, "Connector name is required"),
    type: z.string().min(1, "Connector type is required"),
    config: z.record(z.string(), z.any()).default({}),
  };

  // If a type is selected, add config fields based on the schema
  if (addFormValues.value.type) {
    const selectedType = addFormValues.value.type;
    const typeSchema = connectorTypes.value[selectedType];

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
      } else if (field.type === "number" || field.type === "int") {
        fieldSchema = field.required
          ? z
              .number({ coerce: true })
              .min(0, `${fieldName} must be a positive number`)
          : z.number({ coerce: true }).optional();
      } else if (field.type === "boolean" || field.type === "bool") {
        fieldSchema = z.boolean().default(!!field.default);
      } else if (field.type === "array" || field.type === "arraystring") {
        fieldSchema = z.array(z.string()).default([]);
      } else if (field.type === "arrayint") {
        fieldSchema = z.array(z.number({ coerce: true })).default([]);
      } else if (field.type === "arraybool") {
        fieldSchema = z.array(z.boolean()).default([]);
      } else if (field.type === "object") {
        fieldSchema = z.record(z.string(), z.any()).default({});
      }

      // Apply default values if they exist
      if (
        field.default !== undefined &&
        field.type !== "boolean" &&
        field.type !== "bool"
      ) {
        fieldSchema = fieldSchema.default(field.default);
      }

      configSchema[fieldName] = fieldSchema;
    }

    return z.object({
      ...baseSchema,
      config: z.object(configSchema).default({}),
    });
  }

  // Default basic schema
  return z.object(baseSchema);
});

// Function to fetch connector definitions
async function fetchConnectorDefinitions() {
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
    const { data, error: fetchError } =
      await useFetch<ConnectorDefinitionResponse>(
        `${runtimeConfig.public.backendApi}/connectors/definitions`,
        {
          method: "GET",
          headers: {
            Authorization: `Bearer ${token}`,
          },
        }
      );

    if (fetchError.value) {
      throw new Error(
        fetchError.value.message || "Failed to fetch connector definitions"
      );
    }

    if (data.value && data.value.data) {
      // Convert from definitions array to types map
      const typesMap: Record<string, ConnectorConfigSchema> = {};
      const infoMap: Record<string, { name: string; description: string }> = {};

      data.value.data.definitions.forEach((def) => {
        typesMap[def.id] = def.schema;
        infoMap[def.id] = {
          name: def.name,
          description: def.description,
        };
      });

      connectorTypes.value = typesMap;
      connectorInfo.value = infoMap;
    }
  } catch (err: any) {
    error.value =
      err.message || "An error occurred while fetching connector definitions";
    console.error(err);
  } finally {
    loading.value.types = false;
  }
}

// Function to fetch global connectors
async function fetchConnectors() {
  loading.value.connectors = true;
  error.value = null;

  const runtimeConfig = useRuntimeConfig();
  const cookieToken = useCookie(
    runtimeConfig.public.sessionCookieName as string
  );
  const token = cookieToken.value;

  if (!token) {
    error.value = "Authentication required";
    loading.value.connectors = false;
    return;
  }

  try {
    const { data, error: fetchError } = await useFetch<{
      connectors: Connector[];
    }>(`${runtimeConfig.public.backendApi}/connectors/global`, {
      method: "GET",
      headers: {
        Authorization: `Bearer ${token}`,
      },
    });

    if (fetchError.value) {
      throw new Error(fetchError.value.message || "Failed to fetch connectors");
    }

    if (data.value) {
      // Ensure all connectors have the required information
      const processedConnectors = (data.value.data.connectors || []).map(
        (connector) => {
          return connector;
        }
      );

      connectors.value = processedConnectors;
    }
  } catch (err: any) {
    error.value = err.message || "An error occurred while fetching connectors";
    console.error(err);
  } finally {
    loading.value.connectors = false;
  }
}

// Function to add connector
async function addConnector(values: any) {
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
      `${runtimeConfig.public.backendApi}/connectors/global`,
      {
        method: "POST",
        body: {
          name: values.type,
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
      type: "",
      config: {},
    };
    showAddConnectorDialog.value = false;

    // Refresh the connectors list
    fetchConnectors();
  } catch (err: any) {
    error.value = err.message || "An error occurred while adding the connector";
    console.error(err);
  } finally {
    actionLoading.value = false;
  }
}

// Function to delete connector
async function deleteConnector(connectorId: string) {
  if (!confirm("Are you sure you want to delete this connector?")) {
    return;
  }

  loading.value.connectors = true;
  error.value = null;

  const runtimeConfig = useRuntimeConfig();
  const cookieToken = useCookie(
    runtimeConfig.public.sessionCookieName as string
  );
  const token = cookieToken.value;

  if (!token) {
    error.value = "Authentication required";
    loading.value.connectors = false;
    return;
  }

  try {
    const { data, error: fetchError } = await useFetch(
      `${runtimeConfig.public.backendApi}/connectors/global/${connectorId}`,
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

    // Refresh the connectors list
    fetchConnectors();
  } catch (err: any) {
    error.value =
      err.message || "An error occurred while deleting the connector";
    console.error(err);
  } finally {
    loading.value.connectors = false;
  }
}

// Function to open view details dialog
function openViewDetails(connector: Connector) {
  selectedConnector.value = connector;
  showViewDetailsDialog.value = true;
}

// Function to display config as a string
function formatConfig(config: Record<string, any>): string {
  // Remove sensitive information before displaying
  const displayConfig = { ...config };

  // Mask all sensitive fields
  for (const key in displayConfig) {
    if (
      key.includes("token") ||
      key.includes("apiKey") ||
      key.includes("api_key") ||
      key.includes("secret") ||
      key.includes("password") ||
      key.includes("key")
    ) {
      displayConfig[key] = "********";
    }
  }

  return JSON.stringify(displayConfig, null, 2);
}

// Function to check if a field is of array type
function isArrayType(fieldType: string): boolean {
  return (
    fieldType === "array" ||
    fieldType === "arraystring" ||
    fieldType === "arrayint" ||
    fieldType === "arraybool" ||
    fieldType.startsWith("array")
  );
}

// Function to get the array item type from a field type
function getArrayItemType(fieldType: string): string {
  if (fieldType === "arraystring") return "string";
  if (fieldType === "arrayint") return "int";
  if (fieldType === "arraybool") return "bool";

  // For custom "array<type>" format or backward compatibility
  if (fieldType.startsWith("array")) {
    const subType = fieldType.substring(5).toLowerCase();
    if (subType === "string" || subType === "int" || subType === "bool") {
      return subType;
    }
  }

  // Default to string
  return "string";
}

// Function to add an array item
function addArrayItem(config: Record<string, any>, fieldName: string, fieldType: string) {
  if (!Array.isArray(config[fieldName])) {
    config[fieldName] = [];
  }

  const itemType = getArrayItemType(fieldType);
  let newValue: any = "";
  
  if (itemType === "int") {
    newValue = 0;
  } else if (itemType === "bool") {
    newValue = false;
  }

  config[fieldName].push(newValue);
}

// Function to remove an array item
function removeArrayItem(config: Record<string, any>, fieldName: string, index: number) {
  if (Array.isArray(config[fieldName]) && index >= 0 && index < config[fieldName].length) {
    config[fieldName].splice(index, 1);
  }
}

// Watch for changes to the selected connector type in Add form
watch(
  () => addFormValues.value.type,
  (newType) => {
    if (newType && connectorTypes.value[newType]) {
      // Initialize config fields with default values
      const configFields = connectorTypes.value[newType];
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
        } else if (isArrayType(field.type)) {
          // Initialize with empty array
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

// Setup initial data load
onMounted(() => {
  fetchConnectorDefinitions();
  fetchConnectors();
});
</script>

<template>
  <div class="p-4">
    <div class="flex justify-between items-center mb-4">
      <h1 class="text-2xl font-bold">Global Connectors</h1>
      <Button @click="showAddConnectorDialog = true" :disabled="loading.types">
        Add Connector
      </Button>
    </div>

    <div v-if="error" class="bg-red-500 text-white p-4 rounded mb-4">
      {{ error }}
    </div>

    <Card>
      <CardHeader>
        <CardTitle>Installed Connectors</CardTitle>
        <CardDescription>
          Connectors enable integration with external services like Discord.
        </CardDescription>
      </CardHeader>
      <CardContent>
        <div v-if="loading.connectors" class="text-center py-8">
          <div
            class="animate-spin h-8 w-8 border-4 border-primary border-t-transparent rounded-full mx-auto mb-4"
          ></div>
          <p>Loading connectors...</p>
        </div>

        <div v-else-if="connectors.length === 0" class="text-center py-8">
          <p>No connectors installed. Add a connector to get started.</p>
        </div>

        <div v-else class="overflow-x-auto">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Connector</TableHead>
                <TableHead>Configuration</TableHead>
                <TableHead class="text-right">Actions</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              <TableRow
                v-for="connector in connectors"
                :key="connector.id"
                class="hover:bg-muted/50"
              >
                <TableCell class="font-medium">
                  <div>
                    {{ connectorInfo[connector.name]?.name || connector.name }}
                  </div>
                  <div class="text-xs text-muted-foreground truncate max-w-md">
                    {{
                      connectorInfo[connector.name]?.description ||
                      "No description available"
                    }}
                  </div>
                </TableCell>
                <TableCell>
                  <code class="text-xs block max-h-32 overflow-y-auto">
                    {{ formatConfig(connector.config) }}
                  </code>
                </TableCell>
                <TableCell class="text-right">
                  <div class="flex space-x-2 justify-end">
                    <Button
                      variant="outline"
                      size="sm"
                      @click="openViewDetails(connector)"
                      :disabled="loading.connectors"
                    >
                      Details
                    </Button>
                    <Button
                      variant="destructive"
                      size="sm"
                      @click="deleteConnector(connector.id)"
                      :disabled="loading.connectors"
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
        <CardTitle>About Connectors</CardTitle>
      </CardHeader>
      <CardContent>
        <p class="text-sm text-muted-foreground">
          Connectors provide integration with external services such as Discord,
          allowing your server to interact with these platforms.
        </p>
        <p class="text-sm text-muted-foreground mt-2">
          <strong>Global Connectors</strong> - These connectors are available to
          all servers in your system. They're ideal for shared integrations.
        </p>
        <p class="text-sm text-muted-foreground mt-2">
          <strong>Configuration</strong> - Each connector requires specific
          configuration details like API keys or webhooks URLs. Make sure to
          keep these secure.
        </p>
        <p class="text-sm text-muted-foreground mt-2">
          <strong>Usage</strong> - Connectors are used by extensions to
          communicate with external services. For example, a Discord connector
          enables extensions to send messages to Discord channels.
        </p>
        <p class="text-sm text-muted-foreground mt-2">
          <strong>Available Connectors</strong> - The system supports various
          connector types, each with its own specific functionality and
          configuration requirements.
        </p>
      </CardContent>
    </Card>

    <!-- Available Connector Types -->
    <Card class="mt-6" v-if="Object.keys(connectorInfo).length > 0">
      <CardHeader>
        <CardTitle>Available Connectors</CardTitle>
        <CardDescription>
          These connectors can be installed and configured for your servers.
        </CardDescription>
      </CardHeader>
      <CardContent>
        <div class="space-y-4">
          <div
            v-for="(info, typeId) in connectorInfo"
            :key="typeId"
            class="p-4 border rounded-md hover:bg-muted/50 transition-colors"
          >
            <div class="flex justify-between items-start">
              <div>
                <h3 class="font-medium text-lg">{{ info.name || typeId }}</h3>
                <p class="text-sm text-muted-foreground mt-1">
                  {{ info.description || "No description available" }}
                </p>
              </div>
              <Button
                variant="outline"
                size="sm"
                @click="
                  () => {
                    addFormValues.type = typeId;
                    showAddConnectorDialog = true;
                  }
                "
              >
                Add This Connector
              </Button>
            </div>
          </div>
        </div>
      </CardContent>
    </Card>

    <!-- Add Connector Dialog -->
    <Dialog
      v-model:open="showAddConnectorDialog"
      @update:open="
        (open) => {
          if (!open) addFormValues.config = {};
        }
      "
    >
      <DialogContent class="sm:max-w-[600px] max-h-[80vh] overflow-y-auto">
        <DialogHeader>
          <DialogTitle>Add New Connector</DialogTitle>
          <DialogDescription>
            Install a connector to enable integration with external services.
          </DialogDescription>
        </DialogHeader>

        <div class="py-4">
          <form @submit.prevent="addConnector(addFormValues)">
            <div class="space-y-4">
              <FormField name="type">
                <FormLabel>Connector Type</FormLabel>
                <FormControl>
                  <Select v-model="addFormValues.type">
                    <SelectTrigger class="w-full">
                      <SelectValue placeholder="Select connector type" />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem
                        v-for="(info, typeId) in connectorInfo"
                        :key="typeId"
                        :value="typeId"
                      >
                        {{ info.name || typeId }}
                      </SelectItem>
                    </SelectContent>
                  </Select>
                </FormControl>
                <FormDescription>
                  Choose the type of connector to install (this will be used as the connector's identifier)
                </FormDescription>
              </FormField>

              <!-- Selected connector type description could go here -->
              <div
                v-if="addFormValues.type"
                class="mb-4 p-3 bg-muted rounded-md"
              >
                <div class="text-sm font-medium">
                  {{
                    connectorInfo[addFormValues.type]?.name ||
                    addFormValues.type
                  }}
                </div>
                <div class="text-sm mt-1 text-muted-foreground">
                  {{
                    connectorInfo[addFormValues.type]?.description ||
                    "Configure the settings for this connector type."
                  }}
                </div>
              </div>

              <!-- Dynamic config fields based on selected connector type -->
              <div
                v-if="addFormValues.type && connectorTypes[addFormValues.type]"
                class="mt-6"
              >
                <h3 class="font-medium mb-4">Configuration</h3>
                <div class="space-y-4 border p-4 rounded-md">
                  <FormField
                    v-for="(field, fieldName) in connectorTypes[
                      addFormValues.type
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
                        :type="
                          fieldName.includes('password') ||
                          fieldName.includes('token') ||
                          fieldName.includes('secret') ||
                          fieldName.includes('key')
                            ? 'password'
                            : 'text'
                        "
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
                        v-else-if="isArrayType(field.type)"
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
                            variant="destructive"
                            size="icon"
                            class="shrink-0"
                            @click="
                              removeArrayItem(
                                addFormValues.config,
                                fieldName,
                                index
                              )
                            "
                          >
                            <Icon name="lucide:minus" class="h-4 w-4" />
                          </Button>
                        </div>

                        <!-- Add array item button -->
                        <Button
                          type="button"
                          variant="outline"
                          size="sm"
                          class="mt-2"
                          @click="
                            addArrayItem(
                              addFormValues.config,
                              fieldName,
                              field.type
                            )
                          "
                        >
                          <Icon name="lucide:plus" class="h-4 w-4 mr-2" />
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
                    <FormDescription v-if="field.description">
                      {{ field.description }}
                    </FormDescription>
                  </FormField>
                </div>
              </div>
            </div>

            <DialogFooter class="mt-6">
              <Button
                type="button"
                variant="outline"
                @click="showAddConnectorDialog = false"
              >
                Cancel
              </Button>
              <Button type="submit" :disabled="actionLoading">
                {{ actionLoading ? "Adding..." : "Add Connector" }}
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
          <DialogTitle>
            {{ selectedConnector.name }}
          </DialogTitle>
          <DialogDescription>
            <Badge>{{ selectedConnector.name }}</Badge>
          </DialogDescription>
        </DialogHeader>

        <div class="py-4" v-if="selectedConnector">
          <div class="space-y-4">
            <!-- Connector Type Information -->
            <div class="mb-4">
              <h3 class="text-lg font-medium mb-1">Connector Type</h3>
              <p class="font-medium">
                {{
                  connectorInfo[selectedConnector.name]?.name ||
                  selectedConnector.name
                }}
              </p>
              <p class="text-sm text-muted-foreground mt-2">
                {{
                  connectorInfo[selectedConnector.name]?.description ||
                  "Connector for external service integration."
                }}
              </p>
            </div>

            <!-- Configuration Schema -->
            <div
              v-if="connectorTypes[selectedConnector.name]"
              class="border rounded-md p-4"
            >
              <h3 class="text-lg font-medium mb-3">Configuration Options</h3>

              <div class="space-y-4">
                <div
                  v-for="(field, fieldName) in connectorTypes[
                    selectedConnector.name
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
                      selectedConnector.config &&
                      fieldName in selectedConnector.config
                    "
                    class="mt-2"
                  >
                    <span class="text-xs text-muted-foreground"
                      >Current Value:</span
                    >
                    <code class="bg-muted px-1 rounded text-xs">
                      {{
                        fieldName.includes("password") ||
                        fieldName.includes("token") ||
                        fieldName.includes("secret") ||
                        fieldName.includes("key")
                          ? "********"
                          : JSON.stringify(selectedConnector.config[fieldName])
                      }}
                    </code>
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
          </DialogFooter>
        </div>
      </DialogContent>
    </Dialog>
  </div>
</template>
