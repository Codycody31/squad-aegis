<script setup lang="ts">
import { ref } from "vue";
import { Button } from "~/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "~/components/ui/card";
import { toast } from "~/components/ui/toast";

const props = defineProps<{
  serverId: string;
  workflowId: string;
}>();

const emit = defineEmits<{
  (e: "deleted"): void;
}>();

const isDeleting = ref(false);

const deleteWorkflow = async () => {
  if (
    !confirm(
      "Are you sure you want to delete this workflow? This action cannot be undone."
    )
  ) {
    return;
  }

  isDeleting.value = true;
  const runtimeConfig = useRuntimeConfig();

  try {
    await useAuthFetchImperative(
      `${runtimeConfig.public.backendApi}/servers/${props.serverId}/workflows/${props.workflowId}`,
      {
        method: "DELETE",
      }
    );

    toast({
      title: "Success",
      description: "Workflow deleted successfully",
    });

    emit("deleted");
  } catch (err: any) {
    toast({
      title: "Error",
      description: err.data?.error || "Failed to delete workflow",
      variant: "destructive",
    });
  } finally {
    isDeleting.value = false;
  }
};
</script>

<template>
  <Card class="border-red-200">
    <CardHeader>
      <CardTitle class="text-red-600">Danger Zone</CardTitle>
      <p class="text-sm text-muted-foreground">
        Irreversible actions for this workflow
      </p>
    </CardHeader>
    <CardContent class="space-y-3">
      <div
        class="flex items-center justify-between rounded-lg border border-red-200 p-3"
      >
        <div>
          <p class="font-medium">Delete Workflow</p>
          <p class="text-sm text-muted-foreground">
            Permanently delete this workflow and all execution history
          </p>
        </div>
        <Button
          variant="destructive"
          :disabled="isDeleting"
          @click="deleteWorkflow"
        >
          {{ isDeleting ? "Deleting..." : "Delete" }}
        </Button>
      </div>
    </CardContent>
  </Card>
</template>
