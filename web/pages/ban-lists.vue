<script setup lang="ts">
import { ref, onMounted } from "vue";
import { Button } from "~/components/ui/button";
import { Input } from "~/components/ui/input";
import { Card, CardContent, CardHeader, CardTitle } from "~/components/ui/card";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "~/components/ui/table";
import { Badge } from "~/components/ui/badge";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle, DialogTrigger } from "~/components/ui/dialog";
import { Form, FormControl, FormDescription, FormField, FormItem, FormLabel, FormMessage } from "~/components/ui/form";
import { Textarea } from "~/components/ui/textarea";
import { Switch } from "~/components/ui/switch";
import { useForm } from "vee-validate";
import { toTypedSchema } from "@vee-validate/zod";
import * as z from "zod";
import { useToast } from "~/components/ui/toast";

const authStore = useAuthStore();
const { toast } = useToast();

const loading = ref(true);
const error = ref<string | null>(null);
const banLists = ref<BanList[]>([]);
const showCreateDialog = ref(false);
const showEditDialog = ref(false);
const selectedBanList = ref<BanList | null>(null);
const isSubmitting = ref(false);

interface BanList {
  id: string;
  name: string;
  description: string;
  isGlobal: boolean;
  createdAt: string;
  updatedAt: string;
  createdBy: string;
  creatorName: string;
  banCount: number;
}

interface BanListResponse {
  data: {
    banLists: BanList[];
  };
}

// Form schema for creating/editing a ban list
const formSchema = toTypedSchema(
  z.object({
    name: z.string().min(1, "Name is required"),
    description: z.string().optional(),
    isGlobal: z.boolean().default(false),
  })
);

// Setup form
const form = useForm({
  validationSchema: formSchema,
  initialValues: {
    name: "",
    description: "",
    isGlobal: false,
  },
});

// Function to fetch ban lists
async function fetchBanLists() {
  loading.value = true;
  error.value = null;

  const runtimeConfig = useRuntimeConfig();
  const cookieToken = useCookie(runtimeConfig.public.sessionCookieName as string);
  const token = cookieToken.value;

  if (!token) {
    error.value = "Authentication required";
    loading.value = false;
    return;
  }

  try {
    const { data, error: fetchError } = await useFetch<BanListResponse>(
      `${runtimeConfig.public.backendApi}/ban-lists`,
      {
        method: "GET",
        headers: {
          Authorization: `Bearer ${token}`,
        },
      }
    );

    if (fetchError.value) {
      throw new Error(fetchError.value.message || "Failed to fetch ban lists");
    }

    if (data.value && data.value.data) {
      banLists.value = data.value.data.banLists || [];
    }
  } catch (err: any) {
    error.value = err.message || "An error occurred while fetching ban lists";
    console.error(err);
  } finally {
    loading.value = false;
  }
}

// Function to create a ban list
async function createBanList(values: any) {
  isSubmitting.value = true;
  error.value = null;

  const runtimeConfig = useRuntimeConfig();
  const cookieToken = useCookie(runtimeConfig.public.sessionCookieName as string);
  const token = cookieToken.value;

  if (!token) {
    error.value = "Authentication required";
    isSubmitting.value = false;
    return;
  }

  try {
    const { data, error: fetchError } = await useFetch(
      `${runtimeConfig.public.backendApi}/ban-lists`,
      {
        method: "POST",
        headers: {
          Authorization: `Bearer ${token}`,
        },
        body: values,
      }
    );

    if (fetchError.value) {
      throw new Error(fetchError.value.message || "Failed to create ban list");
    }

    toast({
      title: "Success",
      description: "Ban list created successfully",
      variant: "default",
    });

    // Reset form and close dialog
    form.resetForm();
    showCreateDialog.value = false;

    // Refresh the ban lists
    fetchBanLists();
  } catch (err: any) {
    error.value = err.message || "An error occurred while creating the ban list";
    console.error(err);
  } finally {
    isSubmitting.value = false;
  }
}

// Function to update a ban list
async function updateBanList(values: any) {
  if (!selectedBanList.value) return;

  isSubmitting.value = true;
  error.value = null;

  const runtimeConfig = useRuntimeConfig();
  const cookieToken = useCookie(runtimeConfig.public.sessionCookieName as string);
  const token = cookieToken.value;

  if (!token) {
    error.value = "Authentication required";
    isSubmitting.value = false;
    return;
  }

  try {
    const { data, error: fetchError } = await useFetch(
      `${runtimeConfig.public.backendApi}/ban-lists/${selectedBanList.value.id}`,
      {
        method: "PUT",
        headers: {
          Authorization: `Bearer ${token}`,
        },
        body: values,
      }
    );

    if (fetchError.value) {
      throw new Error(fetchError.value.message || "Failed to update ban list");
    }

    toast({
      title: "Success",
      description: "Ban list updated successfully",
      variant: "default",
    });

    // Reset form and close dialog
    form.resetForm();
    showEditDialog.value = false;
    selectedBanList.value = null;

    // Refresh the ban lists
    fetchBanLists();
  } catch (err: any) {
    error.value = err.message || "An error occurred while updating the ban list";
    console.error(err);
  } finally {
    isSubmitting.value = false;
  }
}

// Function to delete a ban list
async function deleteBanList(banListId: string) {
  if (!confirm("Are you sure you want to delete this ban list? This action cannot be undone.")) {
    return;
  }

  loading.value = true;
  error.value = null;

  const runtimeConfig = useRuntimeConfig();
  const cookieToken = useCookie(runtimeConfig.public.sessionCookieName as string);
  const token = cookieToken.value;

  if (!token) {
    error.value = "Authentication required";
    loading.value = false;
    return;
  }

  try {
    const { data, error: fetchError } = await useFetch(
      `${runtimeConfig.public.backendApi}/ban-lists/${banListId}`,
      {
        method: "DELETE",
        headers: {
          Authorization: `Bearer ${token}`,
        },
      }
    );

    if (fetchError.value) {
      throw new Error(fetchError.value.message || "Failed to delete ban list");
    }

    toast({
      title: "Success",
      description: "Ban list deleted successfully",
      variant: "default",
    });

    // Refresh the ban lists
    fetchBanLists();
  } catch (err: any) {
    error.value = err.message || "An error occurred while deleting the ban list";
    console.error(err);
  } finally {
    loading.value = false;
  }
}

// Function to open edit dialog
function openEditDialog(banList: BanList) {
  selectedBanList.value = banList;
  form.setValues({
    name: banList.name,
    description: banList.description,
    isGlobal: banList.isGlobal,
  });
  showEditDialog.value = true;
}

// Setup initial data load
onMounted(() => {
  fetchBanLists();
});
</script>

<template>
  <div class="p-4">
    <div class="flex justify-between items-center mb-4">
      <h1 class="text-2xl font-bold">Ban Lists</h1>
      <Form v-slot="{ handleSubmit }" as="" keep-values :validation-schema="formSchema">
        <Dialog v-model:open="showCreateDialog">
          <DialogTrigger asChild>
            <Button v-if="authStore.isSuperAdmin">Create Ban List</Button>
          </DialogTrigger>
          <DialogContent class="sm:max-w-[425px]">
            <DialogHeader>
              <DialogTitle>Create Ban List</DialogTitle>
              <DialogDescription>
                Create a new ban list to manage bans across multiple servers.
              </DialogDescription>
            </DialogHeader>
            <form @submit="handleSubmit($event, createBanList)">
              <div class="grid gap-4 py-4">
                <FormField name="name" v-slot="{ componentField }">
                  <FormItem>
                    <FormLabel>Name</FormLabel>
                    <FormControl>
                      <Input placeholder="Enter ban list name" v-bind="componentField" />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                </FormField>

                <FormField name="description" v-slot="{ componentField }">
                  <FormItem>
                    <FormLabel>Description</FormLabel>
                    <FormControl>
                      <Textarea placeholder="Enter ban list description" v-bind="componentField" />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                </FormField>

                <FormField name="isGlobal" v-slot="{ componentField }">
                  <FormItem class="flex flex-row items-center justify-between rounded-lg border p-4">
                    <div class="space-y-0.5">
                      <FormLabel class="text-base">Global Ban List</FormLabel>
                      <FormDescription>
                        Global ban lists are automatically applied to all servers.
                      </FormDescription>
                    </div>
                    <FormControl>
                      <Switch v-bind="componentField" />
                    </FormControl>
                  </FormItem>
                </FormField>
              </div>
              <DialogFooter>
                <Button type="button" variant="outline" @click="showCreateDialog = false">
                  Cancel
                </Button>
                <Button type="submit" :disabled="isSubmitting">
                  <span v-if="isSubmitting" class="mr-2">
                    <Icon name="lucide:loader-2" class="h-4 w-4 animate-spin" />
                  </span>
                  Create Ban List
                </Button>
              </DialogFooter>
            </form>
          </DialogContent>
        </Dialog>
      </Form>
    </div>

    <div v-if="error" class="bg-red-500 text-white p-4 rounded mb-4">
      {{ error }}
    </div>

    <Card>
      <CardHeader>
        <CardTitle>Ban Lists</CardTitle>
        <p class="text-sm text-muted-foreground">
          Manage ban lists that can be applied across multiple servers.
        </p>
      </CardHeader>
      <CardContent>
        <div v-if="loading && banLists.length === 0" class="text-center py-8">
          <div class="animate-spin h-8 w-8 border-4 border-primary border-t-transparent rounded-full mx-auto mb-4"></div>
          <p>Loading ban lists...</p>
        </div>

        <div v-else-if="banLists.length === 0" class="text-center py-8">
          <p>No ban lists found</p>
        </div>

        <div v-else class="overflow-x-auto">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Name</TableHead>
                <TableHead>Description</TableHead>
                <TableHead>Status</TableHead>
                <TableHead>Bans</TableHead>
                <TableHead>Created By</TableHead>
                <TableHead>Created At</TableHead>
                <TableHead class="text-right">Actions</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              <TableRow v-for="banList in banLists" :key="banList.id" class="hover:bg-muted/50">
                <TableCell class="font-medium">{{ banList.name }}</TableCell>
                <TableCell>{{ banList.description }}</TableCell>
                <TableCell>
                  <Badge :variant="banList.isGlobal ? 'default' : 'outline'">
                    {{ banList.isGlobal ? 'Global' : 'Server-Specific' }}
                  </Badge>
                </TableCell>
                <TableCell>{{ banList.banCount }}</TableCell>
                <TableCell>{{ banList.creatorName }}</TableCell>
                <TableCell>{{ new Date(banList.createdAt).toLocaleString() }}</TableCell>
                <TableCell class="text-right">
                  <div class="flex items-center justify-end gap-2">
                    <Button
                      variant="outline"
                      size="sm"
                      @click="openEditDialog(banList)"
                      v-if="authStore.isSuperAdmin"
                    >
                      Edit
                    </Button>
                    <Button
                      variant="destructive"
                      size="sm"
                      @click="deleteBanList(banList.id)"
                      v-if="authStore.isSuperAdmin"
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

    <!-- Edit Dialog -->
    <Dialog v-model:open="showEditDialog">
      <DialogContent class="sm:max-w-[425px]">
        <DialogHeader>
          <DialogTitle>Edit Ban List</DialogTitle>
          <DialogDescription>
            Update the ban list details.
          </DialogDescription>
        </DialogHeader>
        <Form v-slot="{ handleSubmit }" as="" keep-values :validation-schema="formSchema">
          <form @submit="handleSubmit($event, updateBanList)">
            <div class="grid gap-4 py-4">
              <FormField name="name" v-slot="{ componentField }">
                <FormItem>
                  <FormLabel>Name</FormLabel>
                  <FormControl>
                    <Input placeholder="Enter ban list name" v-bind="componentField" />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              </FormField>

              <FormField name="description" v-slot="{ componentField }">
                <FormItem>
                  <FormLabel>Description</FormLabel>
                  <FormControl>
                    <Textarea placeholder="Enter ban list description" v-bind="componentField" />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              </FormField>

              <FormField name="isGlobal" v-slot="{ componentField }">
                <FormItem class="flex flex-row items-center justify-between rounded-lg border p-4">
                  <div class="space-y-0.5">
                    <FormLabel class="text-base">Global Ban List</FormLabel>
                    <FormDescription>
                      Global ban lists are automatically applied to all servers.
                    </FormDescription>
                  </div>
                  <FormControl>
                    <Switch v-bind="componentField" />
                  </FormControl>
                </FormItem>
              </FormField>
            </div>
            <DialogFooter>
              <Button type="button" variant="outline" @click="showEditDialog = false">
                Cancel
              </Button>
              <Button type="submit" :disabled="isSubmitting">
                <span v-if="isSubmitting" class="mr-2">
                  <Icon name="lucide:loader-2" class="h-4 w-4 animate-spin" />
                </span>
                Update Ban List
              </Button>
            </DialogFooter>
          </form>
        </Form>
      </DialogContent>
    </Dialog>
  </div>
</template>

<style scoped>
/* Add any page-specific styles here */
</style>
