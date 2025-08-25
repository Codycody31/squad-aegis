<script setup lang="ts">
import Toaster from '@/components/ui/toast/Toaster.vue'

useHead({
  titleTemplate: (titleChunk) => {
    return titleChunk ? `${titleChunk} - Squad Aegis` : "Squad Aegis";
  },
});

const runtimeConfig = useRuntimeConfig();
const authStore = useAuthStore();

if (useCookie(runtimeConfig.public.sessionCookieName as string).value) {
  await authStore.fetch();
}
</script>

<template>
  <div>
    <NuxtLayout>
      <NuxtPage />
    </NuxtLayout>
    <Toaster />
  </div>
</template>
