<script setup lang="ts">
import { ref, watch } from "vue";
import { Button } from "~/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "~/components/ui/card";
import { Input } from "~/components/ui/input";
import { Label } from "~/components/ui/label";
import { Switch } from "~/components/ui/switch";
import { Textarea } from "~/components/ui/textarea";
import { toast } from "~/components/ui/toast";
import type { Workflow } from "~/components/workflows/types";

const props = defineProps<{
  serverId: string;
  workflowId: string;
  workflow: Workflow;
}>();

const emit = defineEmits<{
  (e: "saved"): void;
}>();

const isUpdating = ref(false);

// Form state for settings
const workflowName = ref(props.workflow.name);
const workflowDescription = ref(props.workflow.description || "");
const workflowEnabled = ref(props.workflow.enabled);

watch(
  () => props.workflow,
  (wf) => {
    workflowName.value = wf.name;
    workflowDescription.value = wf.description || "";
    workflowEnabled.value = wf.enabled;
  }
);

const saveBasicSettings = async () => {
  isUpdating.value = true;
  const runtimeConfig = useRuntimeConfig();

  try {
    await useAuthFetchImperative(
      `${runtimeConfig.public.backendApi}/servers/${props.serverId}/workflows/${props.workflowId}`,
      {
        method: "PUT",
        headers: {
          "Content-Type": "application/json",
        },
        body: JSON.stringify({
          name: workflowName.value,
          description: workflowDescription.value,
          enabled: workflowEnabled.value,
          definition: props.workflow.definition,
        }),
      }
    );

    toast({
      title: "Success",
      description: "Settings updated successfully",
    });

    emit("saved");
  } catch (err: any) {
    toast({
      title: "Error",
      description: err.data?.error || "Failed to update settings",
      variant: "destructive",
    });
  } finally {
    isUpdating.value = false;
  }
};
</script>

<template>
  <Card>
    <CardHeader>
      <CardTitle>Basic Settings</CardTitle>
      <p class="text-sm text-muted-foreground">
        Manage workflow name, description, and status
      </p>
    </CardHeader>
    <CardContent class="space-y-4">
      <div class="space-y-2">
        <Label for="workflow-name">Name</Label>
        <Input
          id="workflow-name"
          v-model="workflowName"
          placeholder="Enter workflow name"
        />
      </div>
      <div class="space-y-2">
        <Label for="workflow-description">Description</Label>
        <Textarea
          id="workflow-description"
          v-model="workflowDescription"
          placeholder="Enter workflow description"
          rows="3"
        />
      </div>
      <div class="flex items-center justify-between">
        <div class="space-y-0.5">
          <Label>Workflow Status</Label>
          <p class="text-sm text-muted-foreground">
            {{
              workflow.enabled
                ? "Workflow is active and will process events"
                : "Workflow is paused and won't process events"
            }}
          </p>
        </div>
        <Switch
          :checked="workflowEnabled"
          @update:checked="workflowEnabled = $event"
        />
      </div>
      <div class="flex justify-end">
        <Button :disabled="isUpdating" @click="saveBasicSettings">
          {{ isUpdating ? "Saving..." : "Save Settings" }}
        </Button>
      </div>
    </CardContent>
  </Card>
</template>
