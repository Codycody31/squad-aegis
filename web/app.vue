<script setup lang="ts">
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
  </div>
</template>
