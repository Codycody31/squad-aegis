<script setup lang="ts">
import { ref } from "vue";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "~/components/ui/tabs";
import PageHeader from "~/components/layout/PageHeader.vue";
import RolesTab from "~/components/server-access/RolesTab.vue";
import AdminsTab from "~/components/server-access/AdminsTab.vue";

definePageMeta({
  middleware: ["auth"],
});

const route = useRoute();
const serverId = route.params.serverId as string;
const authStore = useAuthStore();

// Redirect non-super-admins away from this page
if (!authStore.isSuperAdmin) {
  navigateTo(`/servers/${serverId}`);
}

const activeTab = ref("roles");
</script>

<template>
  <div class="p-4 md:p-6">
    <PageHeader
      title="Users & Roles"
      description="Manage server roles, permissions, and admin assignments."
    />

    <Tabs v-model="activeTab" class="w-full">
      <TabsList class="grid w-full grid-cols-2">
        <TabsTrigger value="roles" class="text-xs sm:text-sm">
          Roles
        </TabsTrigger>
        <TabsTrigger value="admins" class="text-xs sm:text-sm">
          Admins
        </TabsTrigger>
      </TabsList>

      <TabsContent value="roles">
        <RolesTab :server-id="serverId" />
      </TabsContent>

      <TabsContent value="admins">
        <AdminsTab :server-id="serverId" />
      </TabsContent>
    </Tabs>
  </div>
</template>
