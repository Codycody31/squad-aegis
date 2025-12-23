<script setup lang="ts">
import { ref, onMounted, computed } from "vue";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "~/components/ui/card";
import { Button } from "~/components/ui/button";
import { Input } from "~/components/ui/input";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "~/components/ui/table";
import { Checkbox } from "~/components/ui/checkbox";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "~/components/ui/dialog";
import { Badge } from "~/components/ui/badge";
import type { StorageSummary, StorageFile } from "~/types";

definePageMeta({
  middleware: "auth",
  layout: "sudo",
});

const runtimeConfig = useRuntimeConfig();
const authStore = useAuthStore();

if (!authStore.user?.super_admin) {
  navigateTo("/dashboard");
}

const loading = ref(true);
const summary = ref<StorageSummary | null>(null);
const files = ref<StorageFile[]>([]);
const selectedFiles = ref<Set<string>>(new Set());
const searchQuery = ref("");
const currentPage = ref(1);
const totalPages = ref(1);
const totalFiles = ref(0);
const error = ref<string | null>(null);
const deleteDialogOpen = ref(false);
const fileToDelete = ref<string | null>(null);

const fetchSummary = async () => {
  try {
    const res = await $fetch<any>(`${runtimeConfig.public.backendApi}/sudo/storage/summary`, {
      headers: { Authorization: `Bearer ${authStore.token}` },
    });
    summary.value = res.data.data;
  } catch (err: any) {
    console.error("Error fetching storage summary:", err);
  }
};

const fetchFiles = async () => {
  loading.value = true;
  error.value = null;

  try {
    const res = await $fetch<any>(`${runtimeConfig.public.backendApi}/sudo/storage/files`, {
      params: {
        page: currentPage.value,
        limit: 50,
        prefix: searchQuery.value,
      },
      headers: { Authorization: `Bearer ${authStore.token}` },
    });

    files.value = res.data.files;
    totalPages.value = res.data.pagination.totalPages;
    totalFiles.value = res.data.pagination.total;
  } catch (err: any) {
    error.value = err.message || "Failed to load files";
    console.error("Error fetching files:", err);
  } finally {
    loading.value = false;
  }
};

const filteredFiles = computed(() => {
  if (!searchQuery.value) return files.value;
  return files.value.filter(file => 
    file.path.toLowerCase().includes(searchQuery.value.toLowerCase())
  );
});

const toggleFileSelection = (path: string) => {
  if (selectedFiles.value.has(path)) {
    selectedFiles.value.delete(path);
  } else {
    selectedFiles.value.add(path);
  }
};

const toggleAllFiles = () => {
  if (selectedFiles.value.size === filteredFiles.value.length) {
    selectedFiles.value.clear();
  } else {
    filteredFiles.value.forEach(file => {
      if (!file.is_dir) {
        selectedFiles.value.add(file.path);
      }
    });
  }
};

const downloadFile = async (path: string) => {
  try {
    window.open(`${runtimeConfig.public.backendApi}/sudo/storage/files/${path}?token=${authStore.token}`, '_blank');
  } catch (err: any) {
    console.error("Error downloading file:", err);
  }
};

const confirmDelete = (path: string) => {
  fileToDelete.value = path;
  deleteDialogOpen.value = true;
};

const deleteFile = async () => {
  if (!fileToDelete.value) return;

  try {
    await $fetch(`${runtimeConfig.public.backendApi}/sudo/storage/files/${fileToDelete.value}`, {
      method: "DELETE",
      headers: { Authorization: `Bearer ${authStore.token}` },
    });

    await Promise.all([fetchFiles(), fetchSummary()]);
    deleteDialogOpen.value = false;
    fileToDelete.value = null;
  } catch (err: any) {
    error.value = err.message || "Failed to delete file";
    console.error("Error deleting file:", err);
  }
};

const bulkDelete = async () => {
  if (selectedFiles.value.size === 0) return;

  try {
    await $fetch(`${runtimeConfig.public.backendApi}/sudo/storage/files/bulk-delete`, {
      method: "POST",
      headers: { Authorization: `Bearer ${authStore.token}` },
      body: { paths: Array.from(selectedFiles.value) },
    });

    selectedFiles.value.clear();
    await Promise.all([fetchFiles(), fetchSummary()]);
  } catch (err: any) {
    error.value = err.message || "Failed to delete files";
    console.error("Error bulk deleting:", err);
  }
};

const getFileIcon = (file: StorageFile) => {
  if (file.is_dir) return "mdi:folder";
  
  const ext = file.extension.toLowerCase();
  switch (ext) {
    case ".jpg":
    case ".jpeg":
    case ".png":
    case ".gif":
      return "mdi:image";
    case ".mp4":
    case ".avi":
    case ".mov":
      return "mdi:video";
    case ".pdf":
      return "mdi:file-pdf";
    case ".zip":
    case ".tar":
    case ".gz":
      return "mdi:zip-box";
    default:
      return "mdi:file";
  }
};

onMounted(() => {
  Promise.all([fetchSummary(), fetchFiles()]);
});
</script>

<template>
  <div class="p-6 space-y-6">
    <div class="flex items-center justify-between">
      <div>
        <h1 class="text-3xl font-bold">Storage Management</h1>
        <p class="text-muted-foreground">Manage files and view storage usage</p>
      </div>
      <Button @click="fetchFiles" variant="outline">
        <Icon name="mdi:refresh" class="mr-2 h-4 w-4" />
        Refresh
      </Button>
    </div>

    <!-- Storage Summary -->
    <div class="grid gap-4 md:grid-cols-4">
      <Card>
        <CardHeader class="flex flex-row items-center justify-between space-y-0 pb-2">
          <CardTitle class="text-sm font-medium">Total Size</CardTitle>
          <Icon name="mdi:database" class="h-4 w-4 text-muted-foreground" />
        </CardHeader>
        <CardContent>
          <div class="text-2xl font-bold">{{ summary?.total_size_readable || "0 B" }}</div>
          <p class="text-xs text-muted-foreground">{{ summary?.total_files || 0 }} files</p>
        </CardContent>
      </Card>

      <Card>
        <CardHeader class="flex flex-row items-center justify-between space-y-0 pb-2">
          <CardTitle class="text-sm font-medium">Storage Type</CardTitle>
          <Icon name="mdi:harddisk" class="h-4 w-4 text-muted-foreground" />
        </CardHeader>
        <CardContent>
          <div class="text-2xl font-bold capitalize">{{ summary?.storage_type || "Unknown" }}</div>
          <p class="text-xs text-muted-foreground">Backend storage</p>
        </CardContent>
      </Card>

      <Card>
        <CardHeader class="flex flex-row items-center justify-between space-y-0 pb-2">
          <CardTitle class="text-sm font-medium">File Types</CardTitle>
          <Icon name="mdi:file-multiple" class="h-4 w-4 text-muted-foreground" />
        </CardHeader>
        <CardContent>
          <div class="text-2xl font-bold">{{ Object.keys(summary?.files_by_type || {}).length }}</div>
          <p class="text-xs text-muted-foreground">Different types</p>
        </CardContent>
      </Card>

      <Card>
        <CardHeader class="flex flex-row items-center justify-between space-y-0 pb-2">
          <CardTitle class="text-sm font-medium">Selected</CardTitle>
          <Icon name="mdi:checkbox-marked" class="h-4 w-4 text-muted-foreground" />
        </CardHeader>
        <CardContent>
          <div class="text-2xl font-bold">{{ selectedFiles.size }}</div>
          <p class="text-xs text-muted-foreground">Files selected</p>
        </CardContent>
      </Card>
    </div>

    <!-- File Browser -->
    <Card>
      <CardHeader>
        <div class="flex items-center justify-between">
          <div>
            <CardTitle>Files</CardTitle>
            <CardDescription>Browse and manage storage files</CardDescription>
          </div>
          <div class="flex items-center gap-2">
            <Input
              v-model="searchQuery"
              placeholder="Search files..."
              class="w-64"
              @input="fetchFiles"
            />
            <Button
              v-if="selectedFiles.size > 0"
              @click="bulkDelete"
              variant="destructive"
              size="sm"
            >
              <Icon name="mdi:delete" class="mr-2 h-4 w-4" />
              Delete Selected ({{ selectedFiles.size }})
            </Button>
          </div>
        </div>
      </CardHeader>
      <CardContent>
        <div v-if="loading" class="flex items-center justify-center py-12">
          <div class="text-muted-foreground">Loading files...</div>
        </div>

        <div v-else-if="error" class="p-4 bg-destructive/10 border border-destructive rounded-lg">
          <p class="text-destructive">{{ error }}</p>
        </div>

        <div v-else-if="filteredFiles.length === 0" class="text-center py-12 text-muted-foreground">
          No files found
        </div>

        <div v-else>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead class="w-12">
                  <Checkbox
                    :checked="selectedFiles.size === filteredFiles.filter(f => !f.is_dir).length"
                    @update:checked="toggleAllFiles"
                  />
                </TableHead>
                <TableHead>File</TableHead>
                <TableHead>Size</TableHead>
                <TableHead>Modified</TableHead>
                <TableHead class="text-right">Actions</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              <TableRow v-for="file in filteredFiles" :key="file.path">
                <TableCell>
                  <Checkbox
                    v-if="!file.is_dir"
                    :checked="selectedFiles.has(file.path)"
                    @update:checked="() => toggleFileSelection(file.path)"
                  />
                </TableCell>
                <TableCell>
                  <div class="flex items-center gap-2">
                    <Icon :name="getFileIcon(file)" class="h-4 w-4 text-muted-foreground" />
                    <span class="font-medium">{{ file.path }}</span>
                    <Badge v-if="file.extension" variant="outline">{{ file.extension }}</Badge>
                  </div>
                </TableCell>
                <TableCell>{{ file.size_readable }}</TableCell>
                <TableCell>{{ file.modified_time }}</TableCell>
                <TableCell class="text-right">
                  <div class="flex items-center justify-end gap-2">
                    <Button
                      v-if="!file.is_dir"
                      @click="downloadFile(file.path)"
                      size="sm"
                      variant="outline"
                    >
                      <Icon name="mdi:download" class="h-4 w-4" />
                    </Button>
                    <Button
                      v-if="!file.is_dir"
                      @click="confirmDelete(file.path)"
                      size="sm"
                      variant="destructive"
                    >
                      <Icon name="mdi:delete" class="h-4 w-4" />
                    </Button>
                  </div>
                </TableCell>
              </TableRow>
            </TableBody>
          </Table>

          <!-- Pagination -->
          <div v-if="totalPages > 1" class="flex items-center justify-between mt-4">
            <div class="text-sm text-muted-foreground">
              Page {{ currentPage }} of {{ totalPages }} ({{ totalFiles }} total files)
            </div>
            <div class="flex items-center gap-2">
              <Button
                @click="currentPage--; fetchFiles()"
                :disabled="currentPage === 1"
                size="sm"
                variant="outline"
              >
                Previous
              </Button>
              <Button
                @click="currentPage++; fetchFiles()"
                :disabled="currentPage === totalPages"
                size="sm"
                variant="outline"
              >
                Next
              </Button>
            </div>
          </div>
        </div>
      </CardContent>
    </Card>

    <!-- Delete Confirmation Dialog -->
    <Dialog v-model:open="deleteDialogOpen">
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Delete File</DialogTitle>
          <DialogDescription>
            Are you sure you want to delete this file? This action cannot be undone.
          </DialogDescription>
        </DialogHeader>
        <div class="py-4">
          <p class="text-sm font-medium">{{ fileToDelete }}</p>
        </div>
        <DialogFooter>
          <Button @click="deleteDialogOpen = false" variant="outline">Cancel</Button>
          <Button @click="deleteFile" variant="destructive">Delete</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  </div>
</template>

