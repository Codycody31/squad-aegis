<script setup lang="ts">
import { ref, onMounted, onUnmounted, computed } from "vue";
import { Button } from "~/components/ui/button";
import { Input } from "~/components/ui/input";
import { Card, CardContent, CardHeader, CardTitle } from "~/components/ui/card";
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
import { useForm } from "vee-validate";
import { toTypedSchema } from "@vee-validate/zod";
import * as z from "zod";
import { useAuthStore } from "@/stores/auth";

const runtimeConfig = useRuntimeConfig();
const loading = ref(true);
const error = ref<string | null>(null);
const servers = ref<Server[]>([]);
const refreshInterval = ref<NodeJS.Timeout | null>(null);
const searchQuery = ref("");
const showAddServerDialog = ref(false);
const addServerLoading = ref(false);
const authStore = useAuthStore();

// Check if user is super admin
const isSuperAdmin = computed(() => {
  return authStore.user?.super_admin || false;
});

interface Server {
  id: string;
  name: string;
  ip_address: string;
  game_port: number;
  rcon_port: number;
  created_at: string;
  updated_at: string;
}

interface ServersResponse {
  data: {
    servers: Server[];
  };
}

// Form schema for adding a server
const formSchema = toTypedSchema(
  z.object({
    name: z.string().min(1, "Server name is required"),
    ip_address: z.string().min(1, "IP address is required"),
    game_port: z.coerce
      .number()
      .min(1, "Game port is required")
      .max(65535, "Port must be between 1 and 65535"),
    rcon_port: z.coerce
      .number()
      .min(1, "RCON port is required")
      .max(65535, "Port must be between 1 and 65535"),
    rcon_password: z.string().min(1, "RCON password is required"),
  })
);

// Setup form
const form = useForm({
  validationSchema: formSchema,
  initialValues: {
    name: "",
    ip_address: "",
    game_port: 7787,
    rcon_port: 21114,
    rcon_password: "",
  },
});

// Computed property for filtered servers
const filteredServers = computed(() => {
  if (!searchQuery.value.trim()) {
    return servers.value;
  }

  const query = searchQuery.value.toLowerCase();
  return servers.value.filter(
    (server) =>
      server.name.toLowerCase().includes(query) ||
      server.ip_address.toLowerCase().includes(query)
  );
});

// Function to fetch servers data
async function fetchServers() {
  loading.value = true;
  error.value = null;

  const runtimeConfig = useRuntimeConfig();
  const cookieToken = useCookie(
    runtimeConfig.public.sessionCookieName as string
  );
  const token = cookieToken.value;

  if (!token) {
    error.value = "Authentication required";
    loading.value = false;
    return;
  }

  try {
    const { data, error: fetchError } = await useFetch<ServersResponse>(
      `${runtimeConfig.public.backendApi}/servers`,
      {
        method: "GET",
        headers: {
          Authorization: `Bearer ${token}`,
        },
      }
    );

    if (fetchError.value) {
      throw new Error(
        fetchError.value.message || "Failed to fetch servers data"
      );
    }

    if (data.value && data.value.data) {
      servers.value = data.value.data.servers || [];

      // Sort by creation date (most recent first)
      servers.value.sort((a, b) => {
        return (
          new Date(b.created_at).getTime() - new Date(a.created_at).getTime()
        );
      });
    }
  } catch (err: any) {
    error.value =
      err.message || "An error occurred while fetching servers data";
    console.error(err);
  } finally {
    loading.value = false;
  }
}

// Function to add a server
async function addServer(values: any) {
  const { name, ip_address, game_port, rcon_port, rcon_password } = values;

  addServerLoading.value = true;
  error.value = null;

  const runtimeConfig = useRuntimeConfig();
  const cookieToken = useCookie(
    runtimeConfig.public.sessionCookieName as string
  );
  const token = cookieToken.value;

  if (!token) {
    error.value = "Authentication required";
    addServerLoading.value = false;
    return;
  }

  try {
    const { data, error: fetchError } = await useFetch(
      `${runtimeConfig.public.backendApi}/servers`,
      {
        method: "POST",
        body: {
          name,
          ip_address,
          game_port: parseInt(game_port),
          rcon_port: parseInt(rcon_port),
          rcon_password,
        },
        headers: {
          Authorization: `Bearer ${token}`,
        },
      }
    );

    if (fetchError.value) {
      throw new Error(fetchError.value.message || "Failed to add server");
    }

    // Reset form and close dialog
    form.resetForm();
    showAddServerDialog.value = false;

    // Refresh the servers list
    fetchServers();
  } catch (err: any) {
    error.value = err.message || "An error occurred while adding the server";
    console.error(err);
  } finally {
    addServerLoading.value = false;
  }
}

// Function to delete a server
async function deleteServer(serverId: string) {
  if (
    !confirm(
      "Are you sure you want to delete this server? This action cannot be undone."
    )
  ) {
    return;
  }

  loading.value = true;
  error.value = null;

  const runtimeConfig = useRuntimeConfig();
  const cookieToken = useCookie(
    runtimeConfig.public.sessionCookieName as string
  );
  const token = cookieToken.value;

  if (!token) {
    error.value = "Authentication required";
    loading.value = false;
    return;
  }

  try {
    const { data, error: fetchError } = await useFetch(
      `${runtimeConfig.public.backendApi}/servers/${serverId}`,
      {
        method: "DELETE",
        headers: {
          Authorization: `Bearer ${token}`,
        },
      }
    );

    if (fetchError.value) {
      throw new Error(fetchError.value.message || "Failed to delete server");
    }

    // Refresh the servers list
    fetchServers();
  } catch (err: any) {
    error.value = err.message || "An error occurred while deleting the server";
    console.error(err);
  } finally {
    loading.value = false;
  }
}

// Format date
function formatDate(dateString: string): string {
  return new Date(dateString).toLocaleString();
}

fetchServers();

// Manual refresh function
function refreshData() {
  fetchServers();
}
</script>

<template>
  <div class="p-4">
    <div class="flex justify-between items-center mb-4">
      <h1 class="text-2xl font-bold">Servers</h1>
      <div class="flex gap-2">
        <Form
          v-if="isSuperAdmin"
          v-slot="{ handleSubmit }"
          as=""
          keep-values
          :validation-schema="formSchema"
          :initial-values="{
            name: '',
            ip_address: '',
            game_port: 7787,
            rcon_port: 21114,
            rcon_password: '',
          }"
        >
          <Dialog v-model:open="showAddServerDialog">
            <DialogTrigger asChild>
              <Button>Add Server</Button>
            </DialogTrigger>
            <DialogContent
              class="sm:max-w-[425px] max-h-[90vh] overflow-y-auto"
            >
              <DialogHeader>
                <DialogTitle>Add New Server</DialogTitle>
                <DialogDescription>
                  Enter the details of the Squad server you want to add.
                </DialogDescription>
              </DialogHeader>
              <form id="dialogForm" @submit="handleSubmit($event, addServer)">
                <div class="grid gap-4 py-4">
                  <FormField name="name" v-slot="{ componentField }">
                    <FormItem>
                      <FormLabel>Server Name</FormLabel>
                      <FormControl>
                        <Input
                          placeholder="My Squad Server"
                          v-bind="componentField"
                        />
                      </FormControl>
                      <FormMessage />
                    </FormItem>
                  </FormField>
                  <FormField name="ip_address" v-slot="{ componentField }">
                    <FormItem>
                      <FormLabel>IP Address</FormLabel>
                      <FormControl>
                        <Input
                          placeholder="127.0.0.1"
                          v-bind="componentField"
                        />
                      </FormControl>
                      <FormMessage />
                    </FormItem>
                  </FormField>
                  <FormField name="game_port" v-slot="{ componentField }">
                    <FormItem>
                      <FormLabel>Game Port</FormLabel>
                      <FormControl>
                        <Input
                          type="number"
                          placeholder="7787"
                          v-bind="componentField"
                        />
                      </FormControl>
                      <FormDescription>
                        Default Squad game port is 7787
                      </FormDescription>
                      <FormMessage />
                    </FormItem>
                  </FormField>
                  <FormField name="rcon_port" v-slot="{ componentField }">
                    <FormItem>
                      <FormLabel>RCON Port</FormLabel>
                      <FormControl>
                        <Input
                          type="number"
                          placeholder="21114"
                          v-bind="componentField"
                        />
                      </FormControl>
                      <FormDescription>
                        Default Squad RCON port is 21114
                      </FormDescription>
                      <FormMessage />
                    </FormItem>
                  </FormField>
                  <FormField name="rcon_password" v-slot="{ componentField }">
                    <FormItem>
                      <FormLabel>RCON Password</FormLabel>
                      <FormControl>
                        <Input
                          type="password"
                          placeholder="********"
                          v-bind="componentField"
                        />
                      </FormControl>
                      <FormDescription>
                        The RCON password for your Squad server
                      </FormDescription>
                      <FormMessage />
                    </FormItem>
                  </FormField>
                </div>
                <DialogFooter>
                  <Button
                    type="button"
                    variant="outline"
                    @click="showAddServerDialog = false"
                  >
                    Cancel
                  </Button>
                  <Button type="submit" :disabled="addServerLoading">
                    {{ addServerLoading ? "Adding..." : "Add Server" }}
                  </Button>
                </DialogFooter>
              </form>
            </DialogContent>
          </Dialog>
        </Form>
        <Button @click="refreshData" :disabled="loading" variant="outline">
          {{ loading ? "Refreshing..." : "Refresh" }}
        </Button>
      </div>
    </div>

    <div v-if="error" class="bg-red-500 text-white p-4 rounded mb-4">
      {{ error }}
    </div>

    <Card class="mb-4">
      <CardHeader class="pb-2">
        <CardTitle>Server List</CardTitle>
        <p class="text-sm text-muted-foreground">
          View and manage Squad servers. Data refreshes automatically every 60
          seconds.
        </p>
      </CardHeader>
      <CardContent>
        <div class="flex items-center space-x-2 mb-4">
          <Input
            v-model="searchQuery"
            placeholder="Search by name or IP address..."
            class="flex-grow"
          />
        </div>

        <div class="text-sm text-muted-foreground mb-2">
          Showing {{ filteredServers.length }} of {{ servers.length }} servers
        </div>

        <div v-if="loading && servers.length === 0" class="text-center py-8">
          <div
            class="animate-spin h-8 w-8 border-4 border-primary border-t-transparent rounded-full mx-auto mb-4"
          ></div>
          <p>Loading servers...</p>
        </div>

        <div v-else-if="servers.length === 0" class="text-center py-8">
          <p>No servers found</p>
        </div>

        <div v-else-if="filteredServers.length === 0" class="text-center py-8">
          <p>No servers match your search</p>
        </div>

        <div v-else class="overflow-x-auto">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Name</TableHead>
                <TableHead>IP Address</TableHead>
                <TableHead>Game Port</TableHead>
                <TableHead>RCON Port</TableHead>
                <TableHead>Created</TableHead>
                <TableHead class="text-right">Actions</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              <TableRow
                v-for="server in filteredServers"
                :key="server.id"
                class="hover:bg-muted/50"
              >
                <TableCell>
                  <div class="font-medium">
                    {{ server.name }}
                  </div>
                </TableCell>
                <TableCell>{{ server.ip_address }}</TableCell>
                <TableCell>{{ server.game_port }}</TableCell>
                <TableCell>{{ server.rcon_port }}</TableCell>
                <TableCell>{{ formatDate(server.created_at) }}</TableCell>
                <TableCell class="text-right">
                  <div class="flex justify-end gap-2">
                    <Button
                      variant="outline"
                      size="sm"
                      @click="navigateTo(`/servers/${server.id}`)"
                    >
                      Manage
                    </Button>
                    <Button
                      v-if="isSuperAdmin"
                      variant="destructive"
                      size="sm"
                      @click="deleteServer(server.id)"
                      :disabled="loading"
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

    <Card>
      <CardHeader>
        <CardTitle>About Servers</CardTitle>
      </CardHeader>
      <CardContent>
        <p class="text-sm text-muted-foreground">
          This page shows all Squad servers registered in Squad Aegis. Super
          admins can add new servers or delete existing ones.
        </p>
        <p class="text-sm text-muted-foreground mt-2">
          Click on "Manage" to access server-specific features like player
          management, bans, and admin configuration.
        </p>
      </CardContent>
    </Card>
  </div>
</template>

<style scoped>
/* Add any page-specific styles here */
</style>
