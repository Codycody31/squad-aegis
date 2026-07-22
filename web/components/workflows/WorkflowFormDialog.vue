<script setup lang="ts">
import { ref, computed, watch } from "vue";
import { Button } from "~/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "~/components/ui/dialog";
import { Input } from "~/components/ui/input";
import { Label } from "~/components/ui/label";
import { Textarea } from "~/components/ui/textarea";
import { Switch } from "~/components/ui/switch";
import { toast } from "~/components/ui/toast";
import WorkflowEditor from "~/components/WorkflowEditor.vue";
import {
  actionTypes,
  defaultWorkflowDefinition,
  eventTypes,
  stepTypes,
  type Workflow,
  type WorkflowFormValues,
} from "~/components/workflows/types";

const props = withDefaults(
  defineProps<{
    serverId: string;
    mode: "create" | "edit";
    /** Source workflow in edit mode */
    workflow?: Workflow | null;
    /** Initial form values in create mode (e.g. from an import) */
    prefill?: WorkflowFormValues | null;
  }>(),
  {
    workflow: null,
    prefill: null,
  }
);

const emit = defineEmits<{
  (e: "saved"): void;
}>();

const open = defineModel<boolean>("open", { default: false });

const isSaving = ref(false);

const form = ref<WorkflowFormValues>({
  name: "",
  description: "",
  enabled: true,
  definition: defaultWorkflowDefinition(),
});

const isEdit = computed(() => props.mode === "edit");

function resetForm() {
  form.value = {
    name: "",
    description: "",
    enabled: true,
    definition: defaultWorkflowDefinition(),
  };
}

// (Re)initialize the form each time the dialog opens
watch(open, (isOpen) => {
  if (!isOpen) return;
  if (isEdit.value && props.workflow) {
    form.value = JSON.parse(
      JSON.stringify({
        name: props.workflow.name,
        description: props.workflow.description || "",
        enabled: props.workflow.enabled,
        definition: props.workflow.definition,
      })
    );
  } else if (props.prefill) {
    form.value = JSON.parse(JSON.stringify(props.prefill));
  } else {
    resetForm();
  }
});

async function submit() {
  if (isEdit.value) {
    await updateWorkflow();
  } else {
    await createWorkflow();
  }
}

// Create new workflow
async function createWorkflow() {
  if (!form.value.name.trim()) {
    toast({
      title: "Validation Error",
      description: "Workflow name is required",
      variant: "destructive",
    });
    return;
  }

  isSaving.value = true;
  const runtimeConfig = useRuntimeConfig();

  try {
    await useAuthFetchImperative(
      `${runtimeConfig.public.backendApi}/servers/${props.serverId}/workflows`,
      {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
        },
        body: JSON.stringify(form.value),
      }
    );

    toast({
      title: "Success",
      description: "Workflow created successfully",
    });

    open.value = false;
    resetForm();
    emit("saved");
  } catch (err: any) {
    toast({
      title: "Error",
      description: err.data?.error || "Failed to create workflow",
      variant: "destructive",
    });
  } finally {
    isSaving.value = false;
  }
}

// Update workflow
async function updateWorkflow() {
  if (!props.workflow) return;

  isSaving.value = true;
  const runtimeConfig = useRuntimeConfig();

  try {
    await useAuthFetchImperative(
      `${runtimeConfig.public.backendApi}/servers/${props.serverId}/workflows/${props.workflow.id}`,
      {
        method: "PUT",
        headers: {
          "Content-Type": "application/json",
        },
        body: JSON.stringify({
          name: form.value.name,
          description: form.value.description,
          enabled: form.value.enabled,
          definition: form.value.definition,
        }),
      }
    );

    toast({
      title: "Success",
      description: "Workflow updated successfully",
    });

    open.value = false;
    emit("saved");
  } catch (err: any) {
    toast({
      title: "Error",
      description: err.data?.error || "Failed to update workflow",
      variant: "destructive",
    });
  } finally {
    isSaving.value = false;
  }
}
</script>

<template>
  <Dialog v-model:open="open">
    <DialogContent
      class="max-h-[90vh] w-[95vw] overflow-y-auto p-4 sm:max-w-4xl sm:p-6"
    >
      <DialogHeader>
        <DialogTitle>
          {{ isEdit ? "Edit Workflow" : "Create New Workflow" }}
        </DialogTitle>
        <DialogDescription>
          {{
            isEdit
              ? "Modify workflow settings and definition"
              : "Create an automated workflow to respond to server events"
          }}
        </DialogDescription>
      </DialogHeader>

      <div class="space-y-4">
        <div class="grid grid-cols-1 gap-4 sm:grid-cols-2">
          <div class="space-y-2">
            <Label for="workflow-form-name" class="text-xs sm:text-sm">
              Name
            </Label>
            <Input
              id="workflow-form-name"
              v-model="form.name"
              placeholder="Enter workflow name"
              class="text-sm sm:text-base"
            />
          </div>
          <div class="space-y-2">
            <Label for="workflow-form-enabled" class="text-xs sm:text-sm">
              Status
            </Label>
            <div class="flex items-center space-x-2">
              <Switch
                id="workflow-form-enabled"
                v-model:checked="form.enabled"
              />
              <Label for="workflow-form-enabled" class="text-xs sm:text-sm">
                {{ form.enabled ? "Active" : "Inactive" }}
              </Label>
            </div>
          </div>
        </div>

        <div class="space-y-2">
          <Label for="workflow-form-description" class="text-xs sm:text-sm">
            Description
          </Label>
          <Textarea
            id="workflow-form-description"
            v-model="form.description"
            placeholder="Enter workflow description"
            rows="3"
            class="text-sm sm:text-base"
          />
        </div>

        <!-- Workflow Editor Component -->
        <WorkflowEditor
          v-model="form.definition"
          :event-types="eventTypes"
          :step-types="stepTypes"
          :action-types="actionTypes"
          :workflow-id="isEdit ? workflow?.id : undefined"
          :workflow-name="isEdit ? form.name : undefined"
          :workflow-description="isEdit ? form.description : undefined"
          :server-id="serverId"
        />
      </div>

      <DialogFooter>
        <Button variant="outline" @click="open = false">Cancel</Button>
        <Button :disabled="isSaving" @click="submit">
          {{
            isSaving
              ? isEdit
                ? "Updating..."
                : "Creating..."
              : isEdit
                ? "Update Workflow"
                : "Create Workflow"
          }}
        </Button>
      </DialogFooter>
    </DialogContent>
  </Dialog>
</template>
