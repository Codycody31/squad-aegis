<template>
  <div class="p-4">
    <div class="flex justify-between items-center mb-4">
      <h1 class="text-2xl font-bold">Server Settings</h1>
      <p class="text-sm text-muted-foreground">
        Manage your server configuration and settings
      </p>
    </div>

    <!-- Server Status Card -->
    <Card class="mb-4">
      <CardHeader>
        <CardTitle>Server Status</CardTitle>
        <p class="text-sm text-muted-foreground">
          Current status of your server and its connections
        </p>
      </CardHeader>
      <CardContent>
        <div class="grid grid-cols-2 gap-4">
          <div class="flex items-center space-x-2">
            <div
              :class="[
                'w-3 h-3 rounded-full',
                serverStatus?.rcon ? 'bg-green-500' : 'bg-red-500',
              ]"
            ></div>
            <span class="text-sm">RCON Connection</span>
          </div>
          <div class="flex items-center space-x-2">
            <div class="w-3 h-3 rounded-full bg-blue-500"></div>
            <span class="text-sm">Server Online</span>
          </div>
        </div>
      </CardContent>
    </Card>

    <!-- Server Details Form -->
    <Card class="mb-4">
      <CardHeader>
        <CardTitle>Server Details</CardTitle>
        <p class="text-sm text-muted-foreground">
          Update your server configuration and connection settings
        </p>
      </CardHeader>
      <CardContent>
        <form @submit.prevent="updateServer" class="space-y-4">
          <div class="grid gap-4">
            <div class="grid grid-cols-4 items-center gap-4">
              <label for="name" class="text-right">Server Name</label>
              <Input
                id="name"
                v-model="serverForm.name"
                class="col-span-3"
                required
              />
            </div>

            <div class="grid grid-cols-4 items-center gap-4">
              <label for="ip_address" class="text-right">IP Address</label>
              <Input
                id="ip_address"
                v-model="serverForm.ip_address"
                class="col-span-3"
                required
              />
            </div>

            <div class="grid grid-cols-4 items-center gap-4">
              <label for="game_port" class="text-right">Game Port</label>
              <Input
                id="game_port"
                v-model="serverForm.game_port"
                type="number"
                class="col-span-3"
                required
              />
            </div>

            <div class="grid grid-cols-4 items-center gap-4">
              <label for="rcon_port" class="text-right">RCON Port</label>
              <Input
                id="rcon_port"
                v-model="serverForm.rcon_port"
                type="number"
                class="col-span-3"
                required
              />
            </div>

            <div class="grid grid-cols-4 items-center gap-4">
              <label for="rcon_password" class="text-right"
                >RCON Password</label
              >
              <Input
                id="rcon_password"
                v-model="serverForm.rcon_password"
                type="password"
                class="col-span-3"
              />
            </div>
          </div>

          <div class="flex justify-end">
            <Button type="submit" :disabled="isUpdating">
              <span v-if="isUpdating" class="mr-2">
                <Icon name="lucide:loader-2" class="h-4 w-4 animate-spin" />
              </span>
              Update Server
            </Button>
          </div>
        </form>
      </CardContent>
    </Card>

    <!-- RCON Management -->
    <Card class="mb-4">
      <CardHeader>
        <CardTitle>RCON Management</CardTitle>
        <p class="text-sm text-muted-foreground">
          Manage the RCON connection to your server
        </p>
      </CardHeader>
      <CardContent>
        <div class="flex justify-between items-center">
          <p class="text-sm text-muted-foreground">
            Restart the RCON connection if you're experiencing connection issues
          </p>
          <Button
            variant="outline"
            @click="restartRcon"
            :disabled="isRestarting"
          >
            <span v-if="isRestarting" class="mr-2">
              <Icon name="lucide:loader-2" class="h-4 w-4 animate-spin" />
            </span>
            Restart RCON Connection
          </Button>
        </div>
      </CardContent>
    </Card>

    <!-- Danger Zone -->
    <Card class="border-2 border-destructive">
      <CardHeader>
        <CardTitle class="text-destructive">Danger Zone</CardTitle>
        <p class="text-sm text-muted-foreground">
          Once you delete a server, there is no going back. Please be certain.
        </p>
      </CardHeader>
      <CardContent>
        <div class="flex justify-between items-center">
          <p class="text-sm text-muted-foreground">
            This will permanently delete the server and all associated data
          </p>
          <Button
            variant="destructive"
            @click="confirmDelete"
            :disabled="isDeleting"
          >
            <span v-if="isDeleting" class="mr-2">
              <Icon name="lucide:loader-2" class="h-4 w-4 animate-spin" />
            </span>
            Delete Server
          </Button>
        </div>
      </CardContent>
    </Card>

    <!-- Delete Confirmation Dialog -->
    <Dialog v-model:open="showDeleteDialog">
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Delete Server</DialogTitle>
          <DialogDescription>
            Are you sure you want to delete this server? This action cannot be
            undone.
          </DialogDescription>
        </DialogHeader>
        <DialogFooter>
          <Button variant="outline" @click="showDeleteDialog = false"
            >Cancel</Button
          >
          <Button
            variant="destructive"
            @click="deleteServer"
            :disabled="isDeleting"
          >
            <span v-if="isDeleting" class="mr-2">
              <Icon name="lucide:loader-2" class="h-4 w-4 animate-spin" />
            </span>
            Delete Server
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>

    <!-- Ban List Subscriptions -->
    <Card class="mb-4">
      <CardHeader>
        <CardTitle>Ban List Subscriptions</CardTitle>
        <p class="text-sm text-muted-foreground">
          Subscribe to ban lists to automatically apply their bans to this server.
        </p>
      </CardHeader>
      <CardContent>
        <div v-if="loading" class="text-center py-8">
          <div class="animate-spin h-8 w-8 border-4 border-primary border-t-transparent rounded-full mx-auto mb-4"></div>
          <p>Loading ban lists...</p>
        </div>

        <div v-else-if="banLists.length === 0" class="text-center py-8">
          <p>No ban lists available</p>
        </div>

        <div v-else class="space-y-4">
          <div v-for="banList in banLists" :key="banList.id" class="flex items-center justify-between p-4 border rounded-lg">
            <div>
              <div class="font-medium">{{ banList.name }}</div>
              <div class="text-sm text-muted-foreground">{{ banList.description }}</div>
              <div class="flex items-center gap-2 mt-2">
                <Badge :variant="banList.isGlobal ? 'default' : 'outline'">
                  {{ banList.isGlobal ? 'Global' : 'Server-Specific' }}
                </Badge>
                <Badge variant="secondary">{{ banList.banCount }} bans</Badge>
              </div>
            </div>
            <Button
              :variant="subscribedBanLists.some(s => s.id === banList.id) ? 'destructive' : 'default'"
              :disabled="isSubscribing"
              @click="subscribedBanLists.some(s => s.id === banList.id) ? unsubscribeFromBanList(banList.id) : subscribeToBanList(banList.id)"
            >
              <span v-if="isSubscribing" class="mr-2">
                <Icon name="lucide:loader-2" class="h-4 w-4 animate-spin" />
              </span>
              {{ subscribedBanLists.some(s => s.id === banList.id) ? 'Unsubscribe' : 'Subscribe' }}
            </Button>
          </div>
        </div>
      </CardContent>
    </Card>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from "vue";
import { useRoute, useRouter } from "vue-router";
import { useToast } from "~/components/ui/toast";
import { Button } from "~/components/ui/button";
import { Input } from "~/components/ui/input";
import { Card, CardContent, CardHeader, CardTitle } from "~/components/ui/card";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "~/components/ui/dialog";

const route = useRoute();
const router = useRouter();
const { toast } = useToast();

const runtimeConfig = useRuntimeConfig();
const cookieToken = useCookie(runtimeConfig.public.sessionCookieName as string);
const token = cookieToken.value;

const serverId = route.params.serverId;
const serverStatus = ref<any>(null);
const serverForm = ref({
  name: "",
  ip_address: "",
  game_port: "",
  rcon_port: "",
  rcon_password: "",
});

const isUpdating = ref(false);
const isRestarting = ref(false);
const isDeleting = ref(false);
const showDeleteDialog = ref(false);

// Add to the interface section
interface BanList {
  id: string;
  name: string;
  description: string;
  isGlobal: boolean;
  banCount: number;
}

// Add to the refs section
const banLists = ref<BanList[]>([]);
const subscribedBanLists = ref<BanList[]>([]);
const isSubscribing = ref(false);

// Fetch server details
const fetchServerDetails = async () => {
  try {
    const response = await fetch(`/api/servers/${serverId}`, {
      headers: {
        Authorization: `Bearer ${token}`,
      },
    });
    const data = await response.json();

    if (data.code === 200) {
      serverForm.value = {
        name: data.data.server.name,
        ip_address: data.data.server.ip_address,
        game_port: data.data.server.game_port,
        rcon_port: data.data.server.rcon_port,
        rcon_password: data.data.server.rcon_password,
      };
    }
  } catch (error) {
    toast({
      title: "Error",
      description: "Failed to fetch server details",
      variant: "destructive",
    });
  }
};

// fetch server status
const fetchServerStatus = async () => {
  const response = await fetch(`/api/servers/${serverId}/status`, {
    headers: {
      Authorization: `Bearer ${token}`,
    },
  });
  const data = await response.json();
  if (data.code === 200) {
    serverStatus.value = data.data.status;
  }
};

// Update server
const updateServer = async () => {
  isUpdating.value = true;
  try {
    const response = await fetch(`/api/servers/${serverId}`, {
      method: "PUT",
      headers: {
        "Content-Type": "application/json",
        Authorization: `Bearer ${token}`,
      },
      body: JSON.stringify(serverForm.value),
    });

    const data = await response.json();
    if (data.code === 200) {
      toast({
        title: "Success",
        description: "Server updated successfully",
        variant: "default",
      });
      fetchServerDetails();
    } else {
      toast({
        title: "Error",
        description: data.message || "Failed to update server",
        variant: "destructive",
      });
    }
  } catch (error) {
    toast({
      title: "Error",
      description: "Failed to update server",
      variant: "destructive",
    });
  } finally {
    isUpdating.value = false;
  }
};

// Restart RCON connection
const restartRcon = async () => {
  isRestarting.value = true;
  try {
    const response = await fetch(
      `/api/servers/${serverId}/rcon/force-restart`,
      {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          Authorization: `Bearer ${token}`,
        },
      }
    );

    const data = await response.json();
    if (data.code === 200) {
      toast({
        title: "Success",
        description: "RCON connection restarted successfully",
        variant: "default",
      });
      fetchServerDetails();
    } else {
      toast({
        title: "Error",
        description: data.message || "Failed to restart RCON connection",
        variant: "destructive",
      });
    }
  } catch (error) {
    toast({
      title: "Error",
      description: "Failed to restart RCON connection",
      variant: "destructive",
    });
  } finally {
    isRestarting.value = false;
  }
};

// Confirm delete
const confirmDelete = () => {
  showDeleteDialog.value = true;
};

// Delete server
const deleteServer = async () => {
  isDeleting.value = true;
  try {
    const response = await fetch(`/api/servers/${serverId}`, {
      method: "DELETE",
    });

    const data = await response.json();
    if (data.success) {
      toast({
        title: "Success",
        description: "Server deleted successfully",
        variant: "default",
      });
      router.push("/servers");
    } else {
      toast({
        title: "Error",
        description: data.message || "Failed to delete server",
        variant: "destructive",
      });
    }
  } catch (error) {
    toast({
      title: "Error",
      description: "Failed to delete server",
      variant: "destructive",
    });
  } finally {
    isDeleting.value = false;
    showDeleteDialog.value = false;
  }
};

// Add function to fetch ban lists
async function fetchBanLists() {
  const runtimeConfig = useRuntimeConfig();
  const cookieToken = useCookie(runtimeConfig.public.sessionCookieName as string);
  const token = cookieToken.value;

  if (!token) return;

  try {
    const { data, error: fetchError } = await useFetch(
      `${runtimeConfig.public.backendApi}/ban-lists`,
      {
        method: "GET",
        headers: {
          Authorization: `Bearer ${token}`,
        },
      }
    );

    if (fetchError.value) {
      console.error("Failed to fetch ban lists:", fetchError.value);
      return;
    }

    if (data.value && data.value.data) {
      banLists.value = data.value.data.banLists || [];
    }
  } catch (err: any) {
    console.error("Failed to fetch ban lists:", err);
  }
}

// Add function to fetch subscribed ban lists
async function fetchSubscribedBanLists() {
  const runtimeConfig = useRuntimeConfig();
  const cookieToken = useCookie(runtimeConfig.public.sessionCookieName as string);
  const token = cookieToken.value;

  if (!token) return;

  try {
    const { data, error: fetchError } = await useFetch(
      `${runtimeConfig.public.backendApi}/servers/${serverId}/ban-lists`,
      {
        method: "GET",
        headers: {
          Authorization: `Bearer ${token}`,
        },
      }
    );

    if (fetchError.value) {
      console.error("Failed to fetch subscribed ban lists:", fetchError.value);
      return;
    }

    if (data.value && data.value.data) {
      subscribedBanLists.value = data.value.data.banLists || [];
    }
  } catch (err: any) {
    console.error("Failed to fetch subscribed ban lists:", err);
  }
}

// Add function to subscribe to a ban list
async function subscribeToBanList(banListId: string) {
  isSubscribing.value = true;
  try {
    const response = await fetch(
      `${runtimeConfig.public.backendApi}/servers/${serverId}/ban-lists/subscribe`,
      {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          Authorization: `Bearer ${token}`,
        },
        body: JSON.stringify({
          banListId,
        }),
      }
    );

    const data = await response.json();
    if (data.code === 200) {
      toast({
        title: "Success",
        description: "Subscribed to ban list successfully",
        variant: "default",
      });
      fetchSubscribedBanLists();
    } else {
      toast({
        title: "Error",
        description: data.message || "Failed to subscribe to ban list",
        variant: "destructive",
      });
    }
  } catch (error) {
    toast({
      title: "Error",
      description: "Failed to subscribe to ban list",
      variant: "destructive",
    });
  } finally {
    isSubscribing.value = false;
  }
}

// Add function to unsubscribe from a ban list
async function unsubscribeFromBanList(banListId: string) {
  try {
    const response = await fetch(
      `${runtimeConfig.public.backendApi}/servers/${serverId}/ban-lists/unsubscribe`,
      {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          Authorization: `Bearer ${token}`,
        },
        body: JSON.stringify({
          banListId,
        }),
      }
    );

    const data = await response.json();
    if (data.code === 200) {
      toast({
        title: "Success",
        description: "Unsubscribed from ban list successfully",
        variant: "default",
      });
      fetchSubscribedBanLists();
    } else {
      toast({
        title: "Error",
        description: data.message || "Failed to unsubscribe from ban list",
        variant: "destructive",
      });
    }
  } catch (error) {
    toast({
      title: "Error",
      description: "Failed to unsubscribe from ban list",
      variant: "destructive",
    });
  }
}

onMounted(() => {
  fetchServerDetails();
  fetchServerStatus();
  fetchBanLists();
  fetchSubscribedBanLists();
});
</script>
