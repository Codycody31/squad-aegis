<script setup lang="ts">
import { Badge } from "~/components/ui/badge";
import { Checkbox } from "~/components/ui/checkbox";
import { getPermissionCategoryColor, type PermissionGroup } from "./shared";

defineProps<{
  groups: PermissionGroup[];
  loading?: boolean;
}>();

const model = defineModel<string[]>({ default: () => [] });

function isChecked(code: string): boolean {
  return model.value.includes(code);
}

function togglePermission(code: string, checked: boolean) {
  if (checked) {
    if (!model.value.includes(code)) {
      model.value = [...model.value, code];
    }
  } else {
    model.value = model.value.filter((c) => c !== code);
  }
}
</script>

<template>
  <div v-if="loading" class="text-center py-4">
    <div
      class="animate-spin h-6 w-6 border-2 border-primary border-t-transparent rounded-full mx-auto"
    ></div>
  </div>

  <div v-else class="space-y-4 mt-2">
    <div
      v-for="group in groups"
      :key="group.category"
      class="border rounded-md p-3"
    >
      <h3 class="font-medium mb-2 flex items-center gap-2">
        <Badge :class="getPermissionCategoryColor(group.category + ':')">
          {{ group.category_name }}
        </Badge>
      </h3>
      <div class="grid grid-cols-1 sm:grid-cols-2 gap-2">
        <div
          v-for="permission in group.permissions"
          :key="permission.code"
          class="flex items-start space-x-2"
        >
          <div class="flex flex-row items-start space-x-3 space-y-0">
            <Checkbox
              :modelValue="isChecked(permission.code)"
              @update:modelValue="
                (checked: any) =>
                  togglePermission(permission.code, checked === true)
              "
            />
            <div class="leading-none">
              <label class="font-normal text-sm">
                {{ permission.name }}
              </label>
              <p class="text-xs text-muted-foreground">
                {{ permission.description }}
              </p>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>
