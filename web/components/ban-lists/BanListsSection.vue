<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import { Badge } from "~/components/ui/badge";
import { Button } from "~/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "~/components/ui/card";
import { toast } from "~/components/ui/toast";
import DataTable from "~/components/data-table/DataTable.vue";
import type { DataTableColumn } from "~/components/data-table/types";
import BanListFormDialog from "~/components/ban-lists/BanListFormDialog.vue";
import { isSecureOrLocalConnection } from "~/utils/security";
import { Edit, Eye, Link, Plus, Shield, Trash2 } from "lucide-vue-next";

const runtimeConfig = useRuntimeConfig();

const loading = ref(true);
const banLists = ref<any[]>([]);
const dialogOpen = ref(false);
const dialogMode = ref<"create" | "edit">("create");
const currentBanList = ref<any>(null);

// Security check for copy buttons
const canCopyConfigUrl = computed(() => isSecureOrLocalConnection());

// Helper function to get ban list cfg URL
const getBanListCfgUrl = (banListId: string) => {
  if (runtimeConfig.public.backendApi.startsWith("/")) {
    // Relative URL, construct full URL
    const origin = window.location.origin;
    return `${origin}${runtimeConfig.public.backendApi}/ban-lists/${banListId}/cfg`;
  }
  return `${runtimeConfig.public.backendApi}/ban-lists/${banListId}/cfg`;
};

// Helper function to copy cfg URL to clipboard
const copyCfgUrl = async (banListId: string) => {
  const url = getBanListCfgUrl(banListId);
  const ok = await copyToClipboard(url);
  if (ok) {
    toast({
      title: "URL Copied",
      description: "Ban list CFG URL copied to clipboard",
    });
  } else {
    toast({
      title: "Copy Failed",
      description: "Failed to copy URL to clipboard",
      variant: "destructive",
    });
  }
};

// Helper function to open cfg in new tab
const viewCfg = (banListId: string) => {
  const url = getBanListCfgUrl(banListId);
  window.open(url, "_blank");
};

const loadBanLists = async () => {
  loading.value = true;
  try {
    const response = await useAuthFetchImperative<any>(
      `${runtimeConfig.public.backendApi}/ban-lists`
    );
    banLists.value = response.data.ban_lists || [];
  } catch (error: any) {
    console.error("Failed to load ban lists:", error);
    toast({
      title: "Error",
      description: "Failed to load ban lists",
      variant: "destructive",
    });
  } finally {
    loading.value = false;
  }
};

const deleteBanList = async (banList: any) => {
  if (
    !confirm(`Are you sure you want to delete the ban list "${banList.name}"?`)
  ) {
    return;
  }

  try {
    await useAuthFetchImperative(
      `${runtimeConfig.public.backendApi}/ban-lists/${banList.id}`,
      {
        method: "DELETE",
      }
    );

    toast({
      title: "Success",
      description: "Ban list deleted successfully",
    });

    await loadBanLists();
  } catch (error: any) {
    console.error("Failed to delete ban list:", error);
    toast({
      title: "Error",
      description: "Failed to delete ban list",
      variant: "destructive",
    });
  }
};

const openCreateDialog = () => {
  dialogMode.value = "create";
  currentBanList.value = null;
  dialogOpen.value = true;
};

const editBanList = (banList: any) => {
  dialogMode.value = "edit";
  currentBanList.value = banList;
  dialogOpen.value = true;
};

const columns: DataTableColumn<any>[] = [
  { key: "name", label: "Name" },
  {
    key: "description",
    label: "Description",
    accessor: (row) => row.description || "No description",
  },
  { key: "type", label: "Type" },
  {
    key: "created_at",
    label: "Created",
    accessor: (row) => new Date(row.created_at).toLocaleDateString(),
  },
];

onMounted(loadBanLists);

defineExpose({ refresh: loadBanLists });
</script>

<template>
  <Card>
    <CardHeader class="pb-2 sm:pb-3">
      <div
        class="flex flex-col sm:flex-row sm:justify-between sm:items-center gap-3 sm:gap-0"
      >
        <div>
          <CardTitle class="flex items-center gap-2 text-base sm:text-lg">
            <Shield class="h-4 w-4 sm:h-5 sm:w-5" />
            Ban Lists
          </CardTitle>
          <CardDescription class="text-xs sm:text-sm">
            Manage local and remote ban lists that can be shared across
            servers.
          </CardDescription>
        </div>
        <Button
          class="w-full sm:w-auto text-sm sm:text-base"
          @click="openCreateDialog"
        >
          <Plus class="h-4 w-4 mr-2" />
          Create Ban List
        </Button>
      </div>
    </CardHeader>
    <CardContent>
      <DataTable
        :columns="columns"
        :rows="banLists"
        row-key="id"
        :loading="loading && banLists.length === 0"
        empty-text="No ban lists found. Create your first ban list to get started."
      >
        <template #cell-name="{ row }">
          <span class="font-medium">{{ row.name }}</span>
        </template>
        <template #cell-type="{ row }">
          <Badge
            :variant="row.is_remote ? 'secondary' : 'default'"
            class="text-xs"
          >
            {{ row.is_remote ? "Remote" : "Local" }}
          </Badge>
        </template>
        <template #actions="{ row }">
          <div class="flex gap-2 justify-end">
            <Button
              variant="ghost"
              size="sm"
              title="View CFG"
              class="text-xs"
              @click="viewCfg(row.id)"
            >
              <Eye class="h-4 w-4" />
            </Button>
            <Button
              variant="ghost"
              size="sm"
              :title="
                canCopyConfigUrl
                  ? 'Copy CFG URL'
                  : 'Copy disabled - use HTTPS or localhost'
              "
              :disabled="!canCopyConfigUrl"
              class="text-xs"
              @click="copyCfgUrl(row.id)"
            >
              <Link class="h-4 w-4" />
            </Button>
            <Button
              variant="outline"
              size="sm"
              class="text-xs"
              @click="editBanList(row)"
            >
              <Edit class="h-4 w-4" />
            </Button>
            <Button
              variant="destructive"
              size="sm"
              class="text-xs"
              @click="deleteBanList(row)"
            >
              <Trash2 class="h-4 w-4" />
            </Button>
          </div>
        </template>
      </DataTable>
    </CardContent>
  </Card>

  <BanListFormDialog
    v-model:open="dialogOpen"
    :mode="dialogMode"
    :ban-list="currentBanList"
    @saved="loadBanLists"
  />
</template>
