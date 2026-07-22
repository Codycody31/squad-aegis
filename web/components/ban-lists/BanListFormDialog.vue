<script setup lang="ts">
import { ref, watch } from "vue";
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

const props = defineProps<{
  mode: "create" | "edit";
  banList?: Record<string, any> | null;
}>();

const emit = defineEmits<{
  (e: "saved"): void;
}>();

const open = defineModel<boolean>("open", { default: false });

const runtimeConfig = useRuntimeConfig();
const submitting = ref(false);

const form = ref({
  name: "",
  description: "",
  is_remote: false,
  remote_url: "",
  remote_sync_enabled: false,
});

watch(open, (isOpen) => {
  if (!isOpen) return;
  if (props.mode === "edit" && props.banList) {
    form.value = {
      name: props.banList.name,
      description: props.banList.description || "",
      is_remote: props.banList.is_remote || false,
      remote_url: props.banList.remote_url || "",
      remote_sync_enabled: props.banList.remote_sync_enabled || false,
    };
  } else {
    form.value = {
      name: "",
      description: "",
      is_remote: false,
      remote_url: "",
      remote_sync_enabled: false,
    };
  }
});

const submit = async () => {
  submitting.value = true;
  try {
    if (props.mode === "edit") {
      if (!props.banList) return;
      await useAuthFetchImperative(
        `${runtimeConfig.public.backendApi}/ban-lists/${props.banList.id}`,
        {
          method: "PUT",
          body: form.value,
        }
      );
      toast({
        title: "Success",
        description: "Ban list updated successfully",
      });
    } else {
      await useAuthFetchImperative(
        `${runtimeConfig.public.backendApi}/ban-lists`,
        {
          method: "POST",
          body: form.value,
        }
      );
      toast({
        title: "Success",
        description: "Ban list created successfully",
      });
    }

    open.value = false;
    emit("saved");
  } catch (error: any) {
    console.error(
      props.mode === "edit"
        ? "Failed to update ban list:"
        : "Failed to create ban list:",
      error
    );
    toast({
      title: "Error",
      description:
        props.mode === "edit"
          ? "Failed to update ban list"
          : "Failed to create ban list",
      variant: "destructive",
    });
  } finally {
    submitting.value = false;
  }
};
</script>

<template>
  <Dialog v-model:open="open">
    <DialogContent
      class="w-[95vw] sm:max-w-[500px] max-h-[90vh] overflow-y-auto p-4 sm:p-6"
    >
      <DialogHeader>
        <DialogTitle class="text-base sm:text-lg">
          {{ mode === "edit" ? "Edit Ban List" : "Create New Ban List" }}
        </DialogTitle>
        <DialogDescription class="text-xs sm:text-sm">
          {{
            mode === "edit"
              ? "Update the ban list details."
              : "Create a new ban list that can be shared with other servers."
          }}
        </DialogDescription>
      </DialogHeader>
      <div class="grid gap-4 py-4">
        <div>
          <Label htmlFor="ban-list-name">Name</Label>
          <Input
            id="ban-list-name"
            v-model="form.name"
            placeholder="Ban List Name"
          />
        </div>
        <div>
          <Label htmlFor="ban-list-description">Description</Label>
          <Textarea
            id="ban-list-description"
            v-model="form.description"
            placeholder="Description of this ban list"
          />
        </div>
        <div class="flex items-center space-x-2">
          <Switch id="ban-list-is-remote" v-model="form.is_remote" />
          <Label htmlFor="ban-list-is-remote">Remote Ban List</Label>
        </div>
        <div v-if="form.is_remote">
          <Label htmlFor="ban-list-remote-url">Remote URL</Label>
          <Input
            id="ban-list-remote-url"
            v-model="form.remote_url"
            placeholder="https://example.com/bans.cfg"
          />
        </div>
        <div v-if="form.is_remote" class="flex items-center space-x-2">
          <Switch
            id="ban-list-remote-sync-enabled"
            v-model="form.remote_sync_enabled"
          />
          <Label htmlFor="ban-list-remote-sync-enabled"
            >Enable Auto Sync</Label
          >
        </div>
      </div>
      <DialogFooter>
        <Button variant="outline" @click="open = false"> Cancel </Button>
        <Button :disabled="submitting" @click="submit">
          {{ mode === "edit" ? "Update Ban List" : "Create Ban List" }}
        </Button>
      </DialogFooter>
    </DialogContent>
  </Dialog>
</template>
