<script setup lang="ts">
import {
  Breadcrumb,
  BreadcrumbItem,
  BreadcrumbLink,
  BreadcrumbList,
  BreadcrumbPage,
  BreadcrumbSeparator,
} from "@/components/ui/breadcrumb";

import { useBreadcrumbs } from "@/lib/breadcrumbs";

const items = useBreadcrumbs();
</script>

<template>
  <Breadcrumb>
    <BreadcrumbList>
      <template v-for="(item, index) in items" :key="item.title">
        <BreadcrumbItem
          v-if="index !== items.length - 1"
          class="hidden md:block"
        >
          <BreadcrumbLink
            ><RouterLink :to="item.link">{{
              item.title
            }}</RouterLink></BreadcrumbLink
          >
        </BreadcrumbItem>

        <BreadcrumbSeparator v-if="index < items.length - 1" />

        <BreadcrumbPage v-else>
          {{ item.title }}
        </BreadcrumbPage>
      </template>
    </BreadcrumbList>
  </Breadcrumb>
</template>
