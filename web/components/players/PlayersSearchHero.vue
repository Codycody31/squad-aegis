<script setup lang="ts">
import { ref, watch } from "vue";
import { Button } from "~/components/ui/button";
import { Input } from "~/components/ui/input";

const props = defineProps<{
  modelValue: string;
  loading?: boolean;
}>();

const emit = defineEmits<{
  (e: "update:modelValue", value: string): void;
  (e: "search"): void;
}>();

const localValue = ref(props.modelValue);

watch(
  () => props.modelValue,
  (newVal) => {
    localValue.value = newVal;
  }
);

watch(localValue, (newVal) => {
  emit("update:modelValue", newVal);
});

function handleKeyPress(event: KeyboardEvent) {
  if (event.key === "Enter") {
    emit("search");
  }
}

function handleSearch() {
  emit("search");
}
</script>

<template>
  <div class="mb-4">
    <div class="flex flex-col sm:flex-row gap-2 max-w-2xl">
      <div class="relative flex-1">
        <Input
          v-model="localValue"
          placeholder="Search by name, Steam ID, or EOS ID..."
          class="h-10"
          @keypress="handleKeyPress"
        />
      </div>
      <Button
        @click="handleSearch"
        :disabled="loading"
        class="h-10 px-6"
      >
        <span v-if="loading" class="flex items-center gap-2">
          <span class="animate-spin h-4 w-4 border-2 border-current border-t-transparent rounded-full"></span>
          Searching...
        </span>
        <span v-else>Search</span>
      </Button>
    </div>
  </div>
</template>
