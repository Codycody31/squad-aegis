<script setup lang="ts">
import {
  SidebarTrigger,
} from "@/components/ui/sidebar";
import { Separator } from "@/components/ui/separator";
import ThemeToggle from "@/components/ThemeToggle.vue";
// import UserNav from "./UserNav.vue";
import Breadcrumbs from "@/components/Breadcrumbs.vue";
// import SearchInput from "./SearchInput.vue";
</script>

<template>
  <header
    class="flex h-16 shrink-0 items-center justify-between gap-2 transition-[width,height] ease-linear group-has-[[data-collapsible=icon]]/sidebar-wrapper:h-12"
  >
    <div class="flex items-center gap-2 px-4">
      <SidebarTrigger class="-ml-1" />
      <!-- <Separator orientation="vertical" class="mr-2 h-4" /> -->
      <!-- <Breadcrumbs /> -->
    </div>

    <div class="flex items-center gap-2 px-4">
      <!-- <div class="hidden md:flex">
        <SearchInput />
      </div> -->
      <!-- <UserNav /> -->
      <ThemeToggle />
    </div>
  </header>
</template>
