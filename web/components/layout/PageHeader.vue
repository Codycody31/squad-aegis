<script setup lang="ts">
defineProps<{
  title: string;
  description?: string;
}>();
</script>

<template>
  <div
    class="mb-4 flex flex-col gap-2 sm:mb-6 sm:flex-row sm:items-center sm:justify-between"
  >
    <div class="min-w-0">
      <h1 class="flex items-center gap-2 text-2xl font-bold tracking-tight">
        <span class="truncate">{{ title }}</span>
        <slot name="title-extra" />
      </h1>
      <p v-if="description" class="mt-1 text-sm text-muted-foreground">
        {{ description }}
      </p>
    </div>
    <div v-if="$slots.actions" class="flex shrink-0 items-center gap-2">
      <slot name="actions" />
    </div>
  </div>
</template>
