<script setup lang="ts" generic="T extends Record<string, any>">
import { computed, ref, watch } from "vue";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "~/components/ui/table";
import { Button } from "~/components/ui/button";
import { Skeleton } from "~/components/ui/skeleton";
import type { DataTableColumn, DataTableSort } from "./types";

const props = withDefaults(
  defineProps<{
    columns: DataTableColumn<T>[];
    rows: T[];
    rowKey: string | ((row: T) => string | number);
    loading?: boolean;
    emptyText?: string;
    /** Enables client-side pagination when set */
    pageSize?: number;
    /** 'server' never sorts locally, it only emits update:sort */
    sortMode?: "client" | "server";
    /** Makes rows navigate on click */
    rowTo?: (row: T) => string;
  }>(),
  {
    loading: false,
    emptyText: "No results.",
    pageSize: undefined,
    sortMode: "client",
    rowTo: undefined,
  }
);

const emit = defineEmits<{ (e: "row-click", row: T): void }>();

const sort = defineModel<DataTableSort | null>("sort", { default: null });

const keyOf = (row: T): string | number =>
  typeof props.rowKey === "function"
    ? props.rowKey(row)
    : (row[props.rowKey] as string | number);

const valueOf = (row: T, col: DataTableColumn<T>) =>
  col.accessor ? col.accessor(row) : row[col.key];

const toggleSort = (col: DataTableColumn<T>) => {
  if (!col.sortable) return;
  if (sort.value?.key === col.key) {
    sort.value =
      sort.value.dir === "asc" ? { key: col.key, dir: "desc" } : null;
  } else {
    sort.value = { key: col.key, dir: "asc" };
  }
};

const sorted = computed(() => {
  if (!sort.value || props.sortMode === "server") return props.rows;
  const { key, dir } = sort.value;
  const col = props.columns.find((c) => c.key === key);
  const mul = dir === "asc" ? 1 : -1;
  return [...props.rows].sort((a, b) => {
    const av = col ? valueOf(a, col) : a[key];
    const bv = col ? valueOf(b, col) : b[key];
    if (av == null) return 1;
    if (bv == null) return -1;
    if (typeof av === "number" && typeof bv === "number")
      return (av - bv) * mul;
    return String(av).localeCompare(String(bv)) * mul;
  });
});

const page = ref(1);
watch(
  () => props.rows,
  () => {
    page.value = 1;
  }
);
const totalPages = computed(() =>
  props.pageSize
    ? Math.max(1, Math.ceil(sorted.value.length / props.pageSize))
    : 1
);
watch(totalPages, (total) => {
  if (page.value > total) page.value = total;
});
const paged = computed(() => {
  if (!props.pageSize) return sorted.value;
  const start = (page.value - 1) * props.pageSize;
  return sorted.value.slice(start, start + props.pageSize);
});
const rangeLabel = computed(() => {
  if (!props.pageSize) return "";
  const start = (page.value - 1) * props.pageSize + 1;
  const end = Math.min(page.value * props.pageSize, sorted.value.length);
  return `${start}-${end} of ${sorted.value.length}`;
});

const clickable = computed(() => !!props.rowTo);
const onRowClick = (row: T) => {
  emit("row-click", row);
  if (props.rowTo) navigateTo(props.rowTo(row));
};

const mobileColumns = computed(() =>
  props.columns.filter((c) => !c.hideOnMobile)
);
</script>

<template>
  <div>
    <!-- Loading -->
    <slot v-if="loading" name="loading">
      <div class="space-y-2">
        <Skeleton v-for="i in 5" :key="i" class="h-10 w-full" />
      </div>
    </slot>

    <!-- Empty -->
    <slot v-else-if="rows.length === 0" name="empty">
      <div class="py-10 text-center text-sm text-muted-foreground">
        {{ emptyText }}
      </div>
    </slot>

    <template v-else>
      <!-- Desktop table -->
      <div class="hidden md:block">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead
                v-for="col in columns"
                :key="col.key"
                :class="[col.class, col.sortable && 'cursor-pointer select-none']"
                @click="toggleSort(col)"
              >
                <span class="inline-flex items-center gap-1">
                  {{ col.label }}
                  <Icon
                    v-if="col.sortable && sort?.key === col.key"
                    :name="
                      sort.dir === 'asc'
                        ? 'lucide:chevron-up'
                        : 'lucide:chevron-down'
                    "
                    class="h-3.5 w-3.5"
                  />
                </span>
              </TableHead>
              <TableHead v-if="$slots.actions" class="w-[1%] text-right">
                <span class="sr-only">Actions</span>
              </TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            <TableRow
              v-for="row in paged"
              :key="keyOf(row)"
              :class="clickable && 'cursor-pointer'"
              @click="onRowClick(row)"
            >
              <TableCell
                v-for="col in columns"
                :key="col.key"
                :class="col.class"
              >
                <slot
                  :name="`cell-${col.key}`"
                  :row="row"
                  :value="valueOf(row, col)"
                >
                  {{ valueOf(row, col) ?? "—" }}
                </slot>
              </TableCell>
              <TableCell v-if="$slots.actions" class="text-right" @click.stop>
                <slot name="actions" :row="row" />
              </TableCell>
            </TableRow>
          </TableBody>
        </Table>
      </div>

      <!-- Mobile cards -->
      <div class="space-y-3 md:hidden">
        <div
          v-for="row in paged"
          :key="keyOf(row)"
          class="rounded-lg border p-3"
          :class="clickable && 'cursor-pointer'"
          @click="onRowClick(row)"
        >
          <div class="mb-2 font-medium">
            <slot name="mobile-title" :row="row">
              <slot
                :name="`cell-${mobileColumns[0]?.key}`"
                :row="row"
                :value="mobileColumns[0] && valueOf(row, mobileColumns[0])"
              >
                {{ mobileColumns[0] && (valueOf(row, mobileColumns[0]) ?? "—") }}
              </slot>
            </slot>
          </div>
          <dl class="space-y-1 text-sm">
            <div
              v-for="col in mobileColumns.slice($slots['mobile-title'] ? 0 : 1)"
              :key="col.key"
              class="flex items-start justify-between gap-3"
            >
              <dt class="shrink-0 text-muted-foreground">
                {{ col.mobileLabel ?? col.label }}
              </dt>
              <dd class="min-w-0 text-right">
                <slot
                  :name="`cell-${col.key}`"
                  :row="row"
                  :value="valueOf(row, col)"
                >
                  {{ valueOf(row, col) ?? "—" }}
                </slot>
              </dd>
            </div>
          </dl>
          <div
            v-if="$slots.actions"
            class="mt-3 flex justify-end border-t pt-2"
            @click.stop
          >
            <slot name="actions" :row="row" />
          </div>
        </div>
      </div>

      <!-- Client pagination -->
      <div
        v-if="pageSize && totalPages > 1"
        class="mt-3 flex items-center justify-between"
      >
        <span class="text-xs text-muted-foreground">{{ rangeLabel }}</span>
        <div class="flex items-center gap-1">
          <Button
            variant="outline"
            size="sm"
            :disabled="page <= 1"
            @click="page--"
          >
            <Icon name="lucide:chevron-left" class="h-4 w-4" />
            <span class="sr-only">Previous page</span>
          </Button>
          <span class="px-2 text-xs text-muted-foreground">
            {{ page }} / {{ totalPages }}
          </span>
          <Button
            variant="outline"
            size="sm"
            :disabled="page >= totalPages"
            @click="page++"
          >
            <Icon name="lucide:chevron-right" class="h-4 w-4" />
            <span class="sr-only">Next page</span>
          </Button>
        </div>
      </div>
    </template>
  </div>
</template>
