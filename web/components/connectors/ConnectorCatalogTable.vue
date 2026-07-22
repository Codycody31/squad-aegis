<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import type { SystemConfig } from "~/types";
import { Badge } from "~/components/ui/badge";
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
} from "~/components/ui/dialog";
import { Input } from "~/components/ui/input";
import { toast } from "~/components/ui/toast";
import DataTable from "~/components/data-table/DataTable.vue";
import type { DataTableColumn } from "~/components/data-table/types";
import { useAuthStore } from "~/stores/auth";
import { AlertCircle, RefreshCw, Upload } from "lucide-vue-next";

type ConnectorPackage = {
  connector_id: string;
  name: string;
  description?: string;
  version: string;
  source: string;
  distribution?: string;
  official?: boolean;
  install_state: string;
  min_host_api_version?: number;
  required_capabilities?: string[];
  last_error?: string;
  unsafe?: boolean;
};

const emit = defineEmits<{
  /** Fired after an upload or package delete changed the set of available connectors. */
  (e: "changed"): void;
}>();

const authStore = useAuthStore();

const connectorPackages = ref<ConnectorPackage[]>([]);
const loadingPackages = ref(false);
const uploadingPackage = ref(false);
const selectedConnectorBundle = ref<File | null>(null);
const deletePackageTarget = ref<ConnectorPackage | null>(null);
const systemPluginsConfig = ref<SystemConfig["plugins"] | null>(null);

// pendingUnsafeConnectorBundle holds a bundle the operator tried to upload
// that came back from the backend with needs_confirm_unsafe=true. The
// confirm dialog uses this to show context and re-submit with the
// confirm_unsafe=true query param when the operator acknowledges the risk.
const pendingUnsafeConnectorBundle = ref<File | null>(null);
const pendingUnsafeConnectorMessage = ref<string>("");

const trustStoreConfigured = computed(
  () => systemPluginsConfig.value?.trusted_signing_keys_set === true
);
const unsafeSideloadEnabled = computed(
  () => systemPluginsConfig.value?.allow_unsafe_sideload === true
);
const reverifyFailures = computed(() =>
  connectorPackages.value.filter(
    (pkg) =>
      pkg.install_state === "error" &&
      typeof pkg.last_error === "string" &&
      pkg.last_error.includes("signature cannot be re-verified")
  )
);

const fetchSystemPluginsConfig = async () => {
  try {
    if (!authStore.user?.super_admin) return;
    const response = await useAuthFetchImperative<any>(
      "/api/sudo/system/config"
    );
    const cfg = response.data?.data as SystemConfig | undefined;
    systemPluginsConfig.value = cfg?.plugins ?? null;
  } catch (error: any) {
    console.error("Failed to load system plugin config:", error);
    systemPluginsConfig.value = null;
  }
};

const getPackageStateVariant = (state: string) => {
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

const getConnectorSourceLabel = (pkg: ConnectorPackage) => {
  if (pkg.source === "bundled") return "Bundled";
  return "Sideload Native";
};

const formatConnectorRuntime = (pkg: ConnectorPackage) => {
  const parts: string[] = [];
  if (pkg.min_host_api_version) {
    parts.push(`API >= ${pkg.min_host_api_version}`);
  }
  if (pkg.required_capabilities?.length) {
    parts.push(pkg.required_capabilities.join(", "));
  }
  return parts.join(" · ") || "—";
};

const loadConnectorPackages = async () => {
  loadingPackages.value = true;
  try {
    const response = await useAuthFetchImperative<any>(
      "/api/connectors/packages/installed"
    );
    connectorPackages.value = response.data?.connectors || [];
  } catch (error: any) {
    console.error("Failed to load connector packages:", error);
    toast({
      title: "Error",
      description:
        error.data?.message || "Failed to load installed connector packages",
      variant: "destructive",
    });
  } finally {
    loadingPackages.value = false;
  }
};

const onConnectorBundleSelected = (event: Event) => {
  const input = event.target as HTMLInputElement;
  selectedConnectorBundle.value = input.files?.[0] || null;
};

const submitConnectorUpload = async (bundle: File, confirmUnsafe: boolean) => {
  const formData = new FormData();
  formData.append("bundle", bundle);

  const url = confirmUnsafe
    ? "/api/connectors/packages/upload?confirm_unsafe=true"
    : "/api/connectors/packages/upload";

  return useAuthFetchImperative<any>(url, {
    method: "POST",
    body: formData,
  });
};

const uploadConnectorBundle = async () => {
  if (!selectedConnectorBundle.value) {
    toast({
      title: "Missing bundle",
      description: "Choose a connector .zip bundle to upload",
      variant: "destructive",
    });
    return;
  }

  uploadingPackage.value = true;
  const bundle = selectedConnectorBundle.value;
  try {
    const response = await submitConnectorUpload(bundle, false);

    toast({
      title: "Uploaded",
      description:
        response.message || "Connector package uploaded successfully",
    });

    selectedConnectorBundle.value = null;
    await loadConnectorPackages();
    emit("changed");
  } catch (error: any) {
    // needs_confirm_unsafe in the structured error means the bundle is
    // unsigned; show a confirm dialog rather than just a toast.
    const needsConfirm = !!error?.data?.data?.needs_confirm_unsafe;
    if (needsConfirm) {
      pendingUnsafeConnectorBundle.value = bundle;
      pendingUnsafeConnectorMessage.value =
        error?.data?.message ||
        "This connector bundle is unsigned. Confirm to install anyway.";
      uploadingPackage.value = false;
      return;
    }
    console.error("Failed to upload connector bundle:", error);
    toast({
      title: "Error",
      description: error.data?.message || "Failed to upload connector bundle",
      variant: "destructive",
    });
  } finally {
    uploadingPackage.value = false;
  }
};

const confirmUnsafeConnectorUpload = async () => {
  const bundle = pendingUnsafeConnectorBundle.value;
  if (!bundle) {
    return;
  }
  uploadingPackage.value = true;
  try {
    const response = await submitConnectorUpload(bundle, true);
    toast({
      title: "Uploaded",
      description:
        response.message || "Connector package uploaded successfully",
    });
    selectedConnectorBundle.value = null;
    pendingUnsafeConnectorBundle.value = null;
    pendingUnsafeConnectorMessage.value = "";
    await loadConnectorPackages();
    emit("changed");
  } catch (error: any) {
    console.error("Failed to upload unsigned connector bundle:", error);
    toast({
      title: "Error",
      description: error.data?.message || "Failed to upload connector bundle",
      variant: "destructive",
    });
  } finally {
    uploadingPackage.value = false;
  }
};

const cancelUnsafeConnectorUpload = () => {
  pendingUnsafeConnectorBundle.value = null;
  pendingUnsafeConnectorMessage.value = "";
};

const confirmDeleteConnectorPackage = (pkg: ConnectorPackage) => {
  deletePackageTarget.value = pkg;
};

const cancelDeleteConnectorPackage = () => {
  deletePackageTarget.value = null;
};

const deleteConnectorPackage = async () => {
  const pkg = deletePackageTarget.value;
  if (!pkg) return;

  try {
    await useAuthFetchImperative(
      `/api/connectors/packages/installed/${encodeURIComponent(pkg.connector_id)}`,
      {
        method: "DELETE",
      }
    );
    toast({
      title: "Deleted",
      description: "Connector package removed",
    });
    deletePackageTarget.value = null;
    await loadConnectorPackages();
    emit("changed");
  } catch (error: any) {
    console.error("Failed to delete connector package:", error);
    toast({
      title: "Error",
      description: error.data?.message || "Failed to delete connector package",
      variant: "destructive",
    });
  }
};

const columns: DataTableColumn<ConnectorPackage>[] = [
  { key: "package", label: "Package" },
  { key: "source", label: "Source" },
  { key: "version", label: "Version", accessor: (row) => row.version || "—" },
  { key: "install_state", label: "State" },
  {
    key: "runtime",
    label: "Runtime",
    class: "hidden lg:table-cell",
    hideOnMobile: true,
    accessor: (row) => formatConnectorRuntime(row),
  },
  {
    key: "last_error",
    label: "Last error",
    class: "hidden lg:table-cell",
    hideOnMobile: true,
  },
];

onMounted(() => {
  loadConnectorPackages();
  fetchSystemPluginsConfig();
});

defineExpose({ refresh: loadConnectorPackages });
</script>

<template>
  <div>
    <div
      v-if="
        authStore.user?.super_admin &&
        !trustStoreConfigured &&
        !unsafeSideloadEnabled
      "
      class="mb-4 rounded-md border border-destructive/50 bg-destructive/10 p-4 text-sm text-destructive"
    >
      <div class="flex items-start gap-2">
        <AlertCircle class="mt-0.5 h-5 w-5 flex-shrink-0" />
        <div>
          <p class="font-medium">
            No trusted connector signing keys configured
          </p>
          <p class="mt-1">
            <code>plugins.trusted_signing_keys</code> is empty and
            <code>plugins.allow_unsafe_sideload</code> is disabled. No
            connector bundles can be uploaded until one of these is set.
          </p>
        </div>
      </div>
    </div>
    <div
      v-else-if="
        authStore.user?.super_admin &&
        !trustStoreConfigured &&
        unsafeSideloadEnabled
      "
      class="mb-4 rounded-md border border-yellow-500/50 bg-yellow-500/10 p-4 text-sm text-yellow-700 dark:text-yellow-300"
    >
      <div class="flex items-start gap-2">
        <AlertCircle class="mt-0.5 h-5 w-5 flex-shrink-0" />
        <div>
          <p class="font-medium">Unsafe sideloads are enabled</p>
          <p class="mt-1">
            Any uploaded connector bundle will load without signature
            verification. Do not use this in production.
          </p>
        </div>
      </div>
    </div>

    <div
      v-if="reverifyFailures.length > 0"
      class="mb-4 rounded-md border border-destructive/50 bg-destructive/10 p-4 text-sm text-destructive"
    >
      <div class="flex items-start gap-2">
        <AlertCircle class="mt-0.5 h-5 w-5 flex-shrink-0" />
        <div>
          <p class="font-medium">
            {{ reverifyFailures.length }}
            connector
            {{ reverifyFailures.length === 1 ? "package" : "packages" }} failed
            signature re-verification
          </p>
          <p class="mt-1">
            Re-sign and re-upload these bundles, or update
            <code>plugins.trusted_signing_keys</code> to include the signing
            key.
          </p>
        </div>
      </div>
    </div>

    <Card>
      <CardHeader class="pb-2 sm:pb-3">
        <CardTitle class="text-base sm:text-lg"
          >Sideloaded connector packages</CardTitle
        >
        <CardDescription class="text-xs sm:text-sm">
          Upload a signed
          <code class="rounded bg-muted px-1 text-xs">.zip</code> bundle
          containing
          <code class="rounded bg-muted px-1 text-xs">manifest.json</code> and
          one or more Linux subprocess connector executables. Then add a
          connector instance above if the connector needs config.
        </CardDescription>
      </CardHeader>
      <CardContent class="space-y-4">
        <div class="flex flex-col gap-3 md:flex-row md:items-center">
          <Input type="file" accept=".zip" @change="onConnectorBundleSelected" />
          <Button
            :disabled="uploadingPackage || !selectedConnectorBundle"
            @click="uploadConnectorBundle"
          >
            <Upload class="w-4 h-4 mr-2" />
            {{ uploadingPackage ? "Uploading…" : "Upload bundle" }}
          </Button>
          <Button
            variant="ghost"
            size="sm"
            :disabled="loadingPackages"
            @click="loadConnectorPackages"
          >
            <RefreshCw class="w-4 h-4 mr-1" />
            Packages
          </Button>
        </div>
        <p class="text-xs sm:text-sm text-muted-foreground">
          Bundles must be signed with a key in
          <code>plugins.trusted_signing_keys</code>, unless
          <code>plugins.allow_unsafe_sideload</code> is enabled. After upload,
          restart Aegis if the package says pending restart.
        </p>

        <DataTable
          :columns="columns"
          :rows="connectorPackages"
          row-key="connector_id"
          :loading="loadingPackages && connectorPackages.length === 0"
          empty-text="No sideloaded connector packages installed."
        >
          <template #cell-package="{ row }">
            <div class="flex flex-col max-w-[220px] sm:max-w-xs">
              <span class="font-medium text-sm truncate">{{
                row.name || row.connector_id
              }}</span>
              <span
                class="text-xs text-muted-foreground truncate"
                :title="row.connector_id"
                >{{ row.connector_id }}</span
              >
            </div>
          </template>
          <template #cell-source="{ row }">
            <div class="flex flex-wrap gap-1">
              <Badge variant="outline" class="text-xs">{{
                getConnectorSourceLabel(row)
              }}</Badge>
              <Badge v-if="row.official" variant="default" class="text-xs"
                >Official</Badge
              >
              <Badge v-if="row.unsafe" variant="destructive" class="text-xs"
                >Unsafe</Badge
              >
            </div>
          </template>
          <template #cell-install_state="{ row }">
            <Badge
              :variant="getPackageStateVariant(row.install_state)"
              class="text-xs capitalize"
            >
              {{ String(row.install_state || "").replace(/_/g, " ") }}
            </Badge>
          </template>
          <template #cell-runtime="{ value }">
            <span class="text-xs text-muted-foreground">{{ value }}</span>
          </template>
          <template #cell-last_error="{ row }">
            <span
              v-if="row.last_error"
              class="text-xs text-destructive line-clamp-2"
              >{{ row.last_error }}</span
            >
            <span v-else class="text-xs text-muted-foreground">None</span>
          </template>
          <template #actions="{ row }">
            <Button
              v-if="row.source === 'native'"
              size="sm"
              variant="destructive"
              @click="confirmDeleteConnectorPackage(row)"
            >
              Delete
            </Button>
          </template>
        </DataTable>
      </CardContent>
    </Card>

    <Dialog
      :open="deletePackageTarget !== null"
      @update:open="(open) => !open && cancelDeleteConnectorPackage()"
    >
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Delete connector package?</DialogTitle>
          <DialogDescription>
            Delete the sideloaded connector package
            <span class="font-medium"
              >"{{
                deletePackageTarget?.name || deletePackageTarget?.connector_id
              }}"</span
            >? Remove any running connector instance for this id first. The
            package's executable binary will be removed from disk.
          </DialogDescription>
        </DialogHeader>
        <DialogFooter>
          <Button variant="outline" @click="cancelDeleteConnectorPackage"
            >Cancel</Button
          >
          <Button variant="destructive" @click="deleteConnectorPackage"
            >Delete</Button
          >
        </DialogFooter>
      </DialogContent>
    </Dialog>

    <Dialog
      :open="pendingUnsafeConnectorBundle !== null"
      @update:open="(open) => !open && cancelUnsafeConnectorUpload()"
    >
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Install unsigned connector?</DialogTitle>
          <DialogDescription>
            <p class="mb-2">
              This connector bundle is <strong>unsigned</strong> or its
              signature cannot be verified against the configured trust store.
            </p>
            <p class="mb-2 text-xs">
              {{ pendingUnsafeConnectorMessage }}
            </p>
            <p class="text-xs">
              Installing unsigned bundles bypasses signature verification. Only
              proceed for trusted local builds or sandboxed test environments.
            </p>
          </DialogDescription>
        </DialogHeader>
        <DialogFooter>
          <Button variant="outline" @click="cancelUnsafeConnectorUpload"
            >Cancel</Button
          >
          <Button
            variant="destructive"
            :disabled="uploadingPackage"
            @click="confirmUnsafeConnectorUpload"
          >
            {{ uploadingPackage ? "Installing…" : "Install unsigned" }}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  </div>
</template>
