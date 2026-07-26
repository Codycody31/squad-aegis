<script setup lang="ts">
import { Card, CardContent } from "~/components/ui/card";
import { Skeleton } from "~/components/ui/skeleton";

withDefaults(
  defineProps<{
    label: string;
    value?: string | number | null;
    icon?: string;
    iconClass?: string;
    subline?: string;
    loading?: boolean;
  }>(),
  {
    value: undefined,
    icon: undefined,
    iconClass: "bg-primary/10 text-primary",
    subline: undefined,
    loading: false,
  }
);
</script>

<template>
  <Card>
    <CardContent class="p-4 sm:p-6">
      <div class="flex items-center justify-between gap-3">
        <div class="min-w-0 flex-1">
          <p class="text-sm text-muted-foreground">{{ label }}</p>
          <template v-if="loading">
            <Skeleton class="mt-1 h-8 w-20" />
            <Skeleton v-if="subline || $slots.subline" class="mt-2 h-3 w-28" />
          </template>
          <template v-else>
            <div class="truncate text-2xl font-bold sm:text-3xl">
              <slot name="value">{{ value ?? "—" }}</slot>
            </div>
            <p
              v-if="subline || $slots.subline"
              class="mt-1 text-xs text-muted-foreground"
            >
              <slot name="subline">{{ subline }}</slot>
            </p>
          </template>
        </div>
        <div
          v-if="icon"
          class="flex h-10 w-10 shrink-0 items-center justify-center rounded-full"
          :class="iconClass"
        >
          <Icon :name="icon" class="h-5 w-5" />
        </div>
      </div>
    </CardContent>
  </Card>
</template>
