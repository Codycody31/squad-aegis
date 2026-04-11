<script setup lang="ts">
import { computed, ref, onMounted } from "vue";
import { Button } from "~/components/ui/button";
import { Badge } from "~/components/ui/badge";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "~/components/ui/card";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "~/components/ui/table";
import { Input } from "~/components/ui/input";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "~/components/ui/dialog";
import { toast } from "~/components/ui/toast";
import type { PluginPackage, SystemConfig } from "~/types";

definePageMeta({
  middleware: "auth",
  layout: "sudo",
});

const runtimeConfig = useRuntimeConfig();
const authStore = useAuthStore();

if (!authStore.user?.super_admin) {
  navigateTo("/dashboard");
}

const loadingInstalled = ref(true);
const uploading = ref(false);
const selectedBundle = ref<File | null>(null);
const installedPlugins = ref<PluginPackage[]>([]);
const systemPluginsConfig = ref<SystemConfig["plugins"] | null>(null);
const deleteTarget = ref<PluginPackage | null>(null);

const trustStoreConfigured = computed(
  () => systemPluginsConfig.value?.trusted_signing_keys_set === true,
);
const unsafeSideloadEnabled = computed(
  () => systemPluginsConfig.value?.allow_unsafe_sideload === true,
);
const reverifyFailures = computed(() =>
  installedPlugins.value.filter(
    (p) =>
      p.install_state === "error" &&
      typeof p.last_error === "string" &&
      p.last_error.includes("signature cannot be re-verified"),
  ),
);

const getStateVariant = (state: string) => {
  switch (state) {
    case "ready":
      return "default";
    case "pending_restart":
      return "secondary";
    case "error":
      return "destructive";
    default:
      return "outline";
  }
};

const getSourceLabel = (plugin: Pick<PluginPackage, "source" | "distribution" | "official">) => {
  if (plugin.source === "bundled") return "Bundled";
  if (plugin.source === "wasm") return "Sideload WASM";
  return "Sideload Native";
};

const formatRuntimeRequirements = (minHostAPIVersion?: number, requiredCapabilities?: string[]) => {
  const parts: string[] = [];
  if (minHostAPIVersion) {
    parts.push(`API >= ${minHostAPIVersion}`);
  }
  if (requiredCapabilities?.length) {
    parts.push(requiredCapabilities.join(", "));
  }
  return parts.join(" • ") || "-";
};

const fetchInstalledPlugins = async () => {
  loadingInstalled.value = true;
  try {
    const response = await useAuthFetchImperative<any>(`${runtimeConfig.public.backendApi}/plugins/installed`);
    installedPlugins.value = response.data.plugins || [];
  } catch (error: any) {
    console.error("Failed to load installed plugins:", error);
    toast({
      title: "Error",
      description: error.data?.message || "Failed to load installed plugins",
      variant: "destructive",
    });
  } finally {
    loadingInstalled.value = false;
  }
};

const fetchSystemPluginsConfig = async () => {
  try {
    const response = await useAuthFetchImperative<any>(`${runtimeConfig.public.backendApi}/sudo/system/config`);
    const cfg = response.data?.data as SystemConfig | undefined;
    systemPluginsConfig.value = cfg?.plugins ?? null;
  } catch (error: any) {
    console.error("Failed to load system plugin config:", error);
    systemPluginsConfig.value = null;
  }
};

const confirmDeleteInstalledPlugin = (plugin: PluginPackage) => {
  deleteTarget.value = plugin;
};

const cancelDeleteInstalledPlugin = () => {
  deleteTarget.value = null;
};

const deleteInstalledPlugin = async () => {
  const plugin = deleteTarget.value;
  if (!plugin) return;

  try {
    await useAuthFetchImperative(`${runtimeConfig.public.backendApi}/plugins/installed/${plugin.plugin_id}`, {
      method: "DELETE",
    });
    toast({
      title: "Deleted",
      description: "Plugin package deleted successfully",
    });
    deleteTarget.value = null;
    await fetchInstalledPlugins();
  } catch (error: any) {
    console.error("Failed to delete plugin:", error);
    toast({
      title: "Error",
      description: error.data?.message || "Failed to delete plugin",
      variant: "destructive",
    });
  }
};

const onBundleSelected = (event: Event) => {
  const input = event.target as HTMLInputElement;
  selectedBundle.value = input.files?.[0] || null;
};

const uploadBundle = async () => {
  if (!selectedBundle.value) {
    toast({
      title: "Missing bundle",
      description: "Choose a plugin bundle to upload",
      variant: "destructive",
    });
    return;
  }

  uploading.value = true;
  try {
    const formData = new FormData();
    formData.append("bundle", selectedBundle.value);

    await useAuthFetchImperative(`${runtimeConfig.public.backendApi}/plugins/upload`, {
      method: "POST",
      body: formData,
    });

    toast({
      title: "Uploaded",
      description: "Plugin bundle uploaded successfully",
    });
    selectedBundle.value = null;
    await fetchInstalledPlugins();
  } catch (error: any) {
    console.error("Failed to upload plugin bundle:", error);
    toast({
      title: "Error",
      description: error.data?.message || "Failed to upload plugin bundle",
      variant: "destructive",
    });
  } finally {
    uploading.value = false;
  }
};

onMounted(async () => {
  await Promise.all([fetchInstalledPlugins(), fetchSystemPluginsConfig()]);
});
</script>

<template>
  <div class="p-6 space-y-6">
    <div class="flex items-start justify-between gap-4">
      <div>
        <h1 class="text-3xl font-bold">Plugin Packages</h1>
        <p class="text-muted-foreground">
          Manage bundled plugins and sideload native (.so) or WASM plugin bundles that can later be enabled per server.
        </p>
      </div>
      <div class="flex gap-2">
        <Button variant="outline" @click="fetchInstalledPlugins">
          <Icon name="mdi:refresh" class="mr-2 h-4 w-4" />
          Refresh Installed
        </Button>
      </div>
    </div>

    <div
      v-if="!trustStoreConfigured && !unsafeSideloadEnabled"
      class="rounded-md border border-destructive/50 bg-destructive/10 p-4 text-sm text-destructive"
    >
      <div class="flex items-start gap-2">
        <Icon name="mdi:shield-alert" class="mt-0.5 h-5 w-5 flex-shrink-0" />
        <div>
          <p class="font-medium">No trusted plugin signing keys configured</p>
          <p class="mt-1">
            <code>plugins.trusted_signing_keys</code> is empty. Signed bundles will be rejected, and
            <code>plugins.allow_unsafe_sideload</code> is also disabled, so no plugin bundles can be uploaded
            until one of these is set.
          </p>
        </div>
      </div>
    </div>
    <div
      v-else-if="!trustStoreConfigured && unsafeSideloadEnabled"
      class="rounded-md border border-yellow-500/50 bg-yellow-500/10 p-4 text-sm text-yellow-700 dark:text-yellow-300"
    >
      <div class="flex items-start gap-2">
        <Icon name="mdi:alert" class="mt-0.5 h-5 w-5 flex-shrink-0" />
        <div>
          <p class="font-medium">Unsafe sideloads are enabled</p>
          <p class="mt-1">
            <code>plugins.allow_unsafe_sideload</code> is on and no trusted signing keys are configured.
            Any uploaded bundle will load without signature verification. Do not use this in production.
          </p>
        </div>
      </div>
    </div>

    <div
      v-if="reverifyFailures.length > 0"
      class="rounded-md border border-destructive/50 bg-destructive/10 p-4 text-sm text-destructive"
    >
      <div class="flex items-start gap-2">
        <Icon name="mdi:key-alert" class="mt-0.5 h-5 w-5 flex-shrink-0" />
        <div>
          <p class="font-medium">
            {{ reverifyFailures.length }}
            {{ reverifyFailures.length === 1 ? "plugin" : "plugins" }}
            failed signature re-verification
          </p>
          <p class="mt-1">
            The stored signature no longer verifies against the current trust store. Re-sign and re-upload
            these bundles, or update <code>plugins.trusted_signing_keys</code> to include the signing key.
          </p>
        </div>
      </div>
    </div>

    <Card>
      <CardHeader>
        <CardTitle>Upload Native Bundle</CardTitle>
        <CardDescription>
          Upload a `.zip` plugin bundle containing `manifest.json`, one or more Linux `.so` libraries, and optionally the `manifest.sig` plus `manifest.pub` signature pair.
        </CardDescription>
      </CardHeader>
      <CardContent class="space-y-4">
        <div class="flex flex-col gap-3 md:flex-row md:items-center">
          <Input type="file" accept=".zip" @change="onBundleSelected" />
          <Button :disabled="uploading || !selectedBundle" @click="uploadBundle">
            <Icon name="mdi:upload" class="mr-2 h-4 w-4" />
            {{ uploading ? "Uploading..." : "Upload Bundle" }}
          </Button>
        </div>
        <p class="text-sm text-muted-foreground">
          Signed bundles must be signed with a key present in
          <code>plugins.trusted_signing_keys</code>. Unsigned bundles are rejected unless
          <code>plugins.allow_unsafe_sideload</code> is enabled.
        </p>
      </CardContent>
    </Card>

    <Card>
      <CardHeader>
        <CardTitle>Installed Plugins</CardTitle>
        <CardDescription>
          Bundled plugins are always available. Native plugin packages are installed globally and then enabled per server from the server plugin page.
        </CardDescription>
      </CardHeader>
      <CardContent>
        <div v-if="loadingInstalled" class="py-8 text-center text-muted-foreground">
          Loading installed plugins...
        </div>
        <div v-else class="overflow-x-auto">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Name</TableHead>
                <TableHead>Source</TableHead>
                <TableHead>Version</TableHead>
                <TableHead>State</TableHead>
                <TableHead class="hidden xl:table-cell">Runtime</TableHead>
                <TableHead class="hidden xl:table-cell">Last Error</TableHead>
                <TableHead class="text-right">Actions</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              <TableRow v-for="plugin in installedPlugins" :key="`${plugin.source}-${plugin.plugin_id}`">
                <TableCell>
                  <div class="flex flex-col">
                    <span class="font-medium">{{ plugin.name }}</span>
                    <span class="text-xs text-muted-foreground">{{ plugin.plugin_id }}</span>
                  </div>
                </TableCell>
                <TableCell>
                  <div class="flex gap-2">
                    <Badge variant="outline">{{ getSourceLabel(plugin) }}</Badge>
                    <Badge v-if="plugin.official" variant="default">Official</Badge>
                    <Badge v-if="plugin.unsafe" variant="destructive">Unsafe</Badge>
                  </div>
                </TableCell>
                <TableCell>{{ plugin.version || "-" }}</TableCell>
                <TableCell>
                  <Badge :variant="getStateVariant(plugin.install_state)" class="capitalize">
                    {{ plugin.install_state.replace("_", " ") }}
                  </Badge>
                </TableCell>
                <TableCell class="hidden xl:table-cell">
                  {{ formatRuntimeRequirements(plugin.min_host_api_version, plugin.required_capabilities) }}
                </TableCell>
                <TableCell class="hidden xl:table-cell">
                  <span v-if="plugin.last_error" class="text-sm text-destructive">
                    {{ plugin.last_error }}
                  </span>
                  <span v-else class="text-muted-foreground">None</span>
                </TableCell>
                <TableCell class="text-right">
                  <div class="flex justify-end gap-2">
                    <Button
                      v-if="plugin.source === 'native' || plugin.source === 'wasm'"
                      size="sm"
                      variant="destructive"
                      @click="confirmDeleteInstalledPlugin(plugin)"
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

    <Dialog :open="deleteTarget !== null" @update:open="(open) => !open && cancelDeleteInstalledPlugin()">
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Delete plugin package?</DialogTitle>
          <DialogDescription>
            Delete the plugin package
            <span class="font-medium">"{{ deleteTarget?.name }}"</span>?
            Existing server plugin instances using this plugin must be removed first.
            The plugin's <code>.so</code> file will be removed from disk.
          </DialogDescription>
        </DialogHeader>
        <DialogFooter>
          <Button variant="outline" @click="cancelDeleteInstalledPlugin">Cancel</Button>
          <Button variant="destructive" @click="deleteInstalledPlugin">Delete</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  </div>
</template>
