<script setup lang="ts">
import { onMounted, ref } from "vue";
import { Button } from "~/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "~/components/ui/card";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "~/components/ui/dialog";
import { Input } from "~/components/ui/input";
import { Label } from "~/components/ui/label";
import { Textarea } from "~/components/ui/textarea";
import { toast } from "~/components/ui/toast";
import DataTable from "~/components/data-table/DataTable.vue";
import type { DataTableColumn } from "~/components/data-table/types";
import { Plus, Trash2, Users } from "lucide-vue-next";

const runtimeConfig = useRuntimeConfig();

const loading = ref(true);
const ignoredSteamIDs = ref<any[]>([]);
const showDialog = ref(false);

const form = ref({
  steam_id: "",
  reason: "",
});

const resetForm = () => {
  form.value = {
    steam_id: "",
    reason: "",
  };
};

const loadIgnoredSteamIDs = async () => {
  loading.value = true;
  try {
    const response = await useAuthFetchImperative<any>(
      `${runtimeConfig.public.backendApi}/ignored-steam-ids`
    );
    ignoredSteamIDs.value = response.data.ignored_steam_ids || [];
  } catch (error: any) {
    console.error("Failed to load ignored Steam IDs:", error);
    toast({
      title: "Error",
      description: "Failed to load ignored Steam IDs",
      variant: "destructive",
    });
  } finally {
    loading.value = false;
  }
};

const createIgnoredSteamID = async () => {
  try {
    await useAuthFetchImperative(
      `${runtimeConfig.public.backendApi}/ignored-steam-ids`,
      {
        method: "POST",
        body: form.value,
      }
    );

    toast({
      title: "Success",
      description: "Steam ID added to ignore list successfully",
    });

    showDialog.value = false;
    resetForm();
    await loadIgnoredSteamIDs();
  } catch (error: any) {
    console.error("Failed to create ignored Steam ID:", error);
    toast({
      title: "Error",
      description: "Failed to create ignored Steam ID",
      variant: "destructive",
    });
  }
};

const deleteIgnoredSteamID = async (ignoredSteamID: any) => {
  if (
    !confirm(
      `Are you sure you want to remove Steam ID "${ignoredSteamID.steam_id}" from the ignore list?`
    )
  ) {
    return;
  }

  try {
    await useAuthFetchImperative(
      `${runtimeConfig.public.backendApi}/ignored-steam-ids/${ignoredSteamID.id}`,
      {
        method: "DELETE",
      }
    );

    toast({
      title: "Success",
      description: "Steam ID removed from ignore list successfully",
    });

    await loadIgnoredSteamIDs();
  } catch (error: any) {
    console.error("Failed to delete ignored Steam ID:", error);
    toast({
      title: "Error",
      description: "Failed to delete ignored Steam ID",
      variant: "destructive",
    });
  }
};

const columns: DataTableColumn<any>[] = [
  { key: "steam_id", label: "Steam ID" },
  {
    key: "reason",
    label: "Reason",
    accessor: (row) => row.reason || "No reason provided",
  },
  {
    key: "created_at",
    label: "Added",
    accessor: (row) => new Date(row.created_at).toLocaleDateString(),
  },
];

onMounted(loadIgnoredSteamIDs);

defineExpose({ refresh: loadIgnoredSteamIDs });
</script>

<template>
  <Card>
    <CardHeader class="pb-2 sm:pb-3">
      <div
        class="flex flex-col sm:flex-row sm:justify-between sm:items-center gap-3 sm:gap-0"
      >
        <div>
          <CardTitle class="flex items-center gap-2 text-base sm:text-lg">
            <Users class="h-4 w-4 sm:h-5 sm:w-5" />
            Ignored Steam IDs
          </CardTitle>
          <CardDescription class="text-xs sm:text-sm">
            Manage Steam IDs that should be ignored when syncing from remote
            ban sources.
          </CardDescription>
        </div>
        <Dialog v-model:open="showDialog">
          <DialogTrigger asChild>
            <Button class="w-full sm:w-auto text-sm sm:text-base">
              <Plus class="h-4 w-4 mr-2" />
              Add Ignored Steam ID
            </Button>
          </DialogTrigger>
          <DialogContent
            class="w-[95vw] sm:max-w-[500px] max-h-[90vh] overflow-y-auto p-4 sm:p-6"
          >
            <DialogHeader>
              <DialogTitle class="text-base sm:text-lg"
                >Add Ignored Steam ID</DialogTitle
              >
              <DialogDescription class="text-xs sm:text-sm">
                Add a Steam ID to ignore when syncing bans from remote sources.
              </DialogDescription>
            </DialogHeader>
            <div class="grid gap-4 py-4">
              <div>
                <Label htmlFor="steam_id">Steam ID</Label>
                <Input
                  id="steam_id"
                  v-model="form.steam_id"
                  placeholder="76561198012345678"
                />
              </div>
              <div>
                <Label htmlFor="reason">Reason</Label>
                <Textarea
                  id="reason"
                  v-model="form.reason"
                  placeholder="Reason for ignoring this Steam ID"
                />
              </div>
            </div>
            <DialogFooter>
              <Button variant="outline" @click="showDialog = false">
                Cancel
              </Button>
              <Button @click="createIgnoredSteamID"> Add to Ignore List </Button>
            </DialogFooter>
          </DialogContent>
        </Dialog>
      </div>
    </CardHeader>
    <CardContent>
      <DataTable
        :columns="columns"
        :rows="ignoredSteamIDs"
        row-key="id"
        :loading="loading && ignoredSteamIDs.length === 0"
        empty-text="No Steam IDs are currently ignored. Add Steam IDs to prevent them from being banned via remote sources."
      >
        <template #cell-steam_id="{ row }">
          <span class="font-mono text-xs sm:text-sm">{{ row.steam_id }}</span>
        </template>
        <template #actions="{ row }">
          <Button
            variant="destructive"
            size="sm"
            class="text-xs"
            @click="deleteIgnoredSteamID(row)"
          >
            <Trash2 class="h-4 w-4" />
          </Button>
        </template>
      </DataTable>
    </CardContent>
  </Card>
</template>
