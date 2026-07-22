<script setup lang="ts">
import { ref, watch } from "vue";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "~/components/ui/dialog";
import { Button } from "~/components/ui/button";
import { Textarea } from "~/components/ui/textarea";
import { toast } from "~/components/ui/toast";

const props = defineProps<{
  serverId: string;
}>();

const open = defineModel<boolean>("open", { default: false });

const runtimeConfig = useRuntimeConfig();
const message = ref("");
const sending = ref(false);

watch(open, (isOpen) => {
  if (!isOpen) message.value = "";
});

async function sendBroadcast() {
  const text = message.value.trim();
  if (!text) return;

  sending.value = true;
  try {
    await useAuthFetchImperative(
      `${runtimeConfig.public.backendApi}/servers/${props.serverId}/rcon/execute`,
      {
        method: "POST",
        body: { command: `AdminBroadcast ${text}` },
      }
    );
    toast({
      title: "Broadcast sent",
      description: text,
    });
    open.value = false;
  } catch (err: any) {
    toast({
      title: "Broadcast failed",
      description: extractApiErrorMessage(err, "Could not send the broadcast"),
      variant: "destructive",
    });
  } finally {
    sending.value = false;
  }
}
</script>

<template>
  <Dialog v-model:open="open">
    <DialogContent class="sm:max-w-[480px]">
      <DialogHeader>
        <DialogTitle>Broadcast message</DialogTitle>
        <DialogDescription>
          Shown on screen to every player on the server.
        </DialogDescription>
      </DialogHeader>
      <Textarea
        v-model="message"
        rows="3"
        maxlength="256"
        placeholder="Server restarting in 5 minutes"
        @keydown.enter.exact.prevent="sendBroadcast"
      />
      <p class="text-right text-xs text-muted-foreground">
        {{ message.length }}/256
      </p>
      <DialogFooter>
        <Button variant="outline" @click="open = false">Cancel</Button>
        <Button :disabled="sending || !message.trim()" @click="sendBroadcast">
          <Icon
            v-if="sending"
            name="mdi:loading"
            class="mr-2 h-4 w-4 animate-spin"
          />
          Send broadcast
        </Button>
      </DialogFooter>
    </DialogContent>
  </Dialog>
</template>
