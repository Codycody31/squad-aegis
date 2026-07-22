<script setup lang="ts">
import { ref } from "vue";
import { Button } from "~/components/ui/button";
import PageHeader from "~/components/layout/PageHeader.vue";
import BanListsSection from "~/components/ban-lists/BanListsSection.vue";
import RemoteSourcesSection from "~/components/ban-lists/RemoteSourcesSection.vue";
import IgnoredSteamIdsSection from "~/components/ban-lists/IgnoredSteamIdsSection.vue";
import { Download } from "lucide-vue-next";

definePageMeta({
  middleware: ["auth"],
});

useHead({
  title: "Ban Lists Management",
});

const banListsSection = ref<InstanceType<typeof BanListsSection> | null>(null);
const remoteSourcesSection = ref<InstanceType<
  typeof RemoteSourcesSection
> | null>(null);
const ignoredSteamIdsSection = ref<InstanceType<
  typeof IgnoredSteamIdsSection
> | null>(null);

const refreshing = ref(false);

const refreshAll = async () => {
  refreshing.value = true;
  try {
    await Promise.all([
      banListsSection.value?.refresh(),
      remoteSourcesSection.value?.refresh(),
      ignoredSteamIdsSection.value?.refresh(),
    ]);
  } finally {
    refreshing.value = false;
  }
};
</script>

<template>
  <div class="p-4 md:p-6">
    <PageHeader
      title="Ban Lists"
      description="Manage ban lists, remote sources, and ignored Steam IDs for shared ban coordination."
    >
      <template #actions>
        <Button
          :disabled="refreshing"
          class="w-full sm:w-auto text-sm sm:text-base"
          @click="refreshAll"
        >
          <Download class="h-4 w-4 mr-2" />
          {{ refreshing ? "Loading..." : "Refresh" }}
        </Button>
      </template>
    </PageHeader>

    <div class="space-y-4 sm:space-y-6">
      <BanListsSection ref="banListsSection" />
      <RemoteSourcesSection ref="remoteSourcesSection" />
      <IgnoredSteamIdsSection ref="ignoredSteamIdsSection" />
    </div>
  </div>
</template>
