<script setup lang="ts">
import Toaster from '@/components/ui/toast/Toaster.vue'
import TooltipProvider from '@/components/ui/tooltip/TooltipProvider.vue'

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
    <TooltipProvider>
      <NuxtLayout>
        <NuxtPage />
      </NuxtLayout>
      <Toaster />
    </TooltipProvider>
  </div>
</template>
