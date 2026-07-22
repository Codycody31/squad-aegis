<script setup lang="ts">
import { SidebarTrigger } from "@/components/ui/sidebar";
import { Separator } from "@/components/ui/separator";
import { Button } from "@/components/ui/button";
import ThemeToggle from "@/components/ThemeToggle.vue";
import Breadcrumbs from "@/components/Breadcrumbs.vue";

const { open } = useCommandPalette();
</script>

<template>
  <header
    class="flex h-16 shrink-0 items-center justify-between gap-2 transition-[width,height] ease-linear group-has-[[data-collapsible=icon]]/sidebar-wrapper:h-12"
  >
    <div class="flex min-w-0 items-center gap-2 px-4">
      <SidebarTrigger class="-ml-1" />
      <Separator orientation="vertical" class="mr-2 h-4" />
      <Breadcrumbs />
    </div>

    <div class="flex shrink-0 items-center gap-2 px-4">
      <Button
        variant="outline"
        size="sm"
        class="gap-2 text-muted-foreground"
        @click="open"
      >
        <Icon name="lucide:search" class="h-4 w-4" />
        <span class="hidden sm:inline">Search…</span>
        <kbd
          class="pointer-events-none hidden rounded border bg-muted px-1.5 font-mono text-[10px] font-medium sm:inline-block"
        >
          ⌘K
        </kbd>
      </Button>
      <ThemeToggle />
    </div>
  </header>
</template>
