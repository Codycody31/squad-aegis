<script setup lang="ts">
import { watch } from "vue";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "~/components/ui/dialog";
import { Textarea } from "~/components/ui/textarea";
import { Input } from "~/components/ui/input";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "~/components/ui/select";
import type { Player } from "~/types";
import {
  usePlayerAdminActions,
  type PlayerAdminAction,
} from "~/composables/usePlayerAdminActions";

const props = defineProps<{
  action: PlayerAdminAction | null;
  player: Player | null;
  serverId: string;
}>();

const open = defineModel<boolean>("open", { default: false });

const emit = defineEmits<{
  (e: "completed"): void;
}>();

const {
  actionType,
  actionReason,
  actionDuration,
  selectedRuleId,
  serverRules,
  escalationSuggestion,
  loadingEscalation,
  isActionLoading,
  beginAction,
  resetAction,
  onRuleSelected,
  switchToSuggestedAction,
  executePlayerAction,
} = usePlayerAdminActions(
  () => props.serverId,
  () => props.player
);

watch(open, (isOpen) => {
  if (isOpen && props.action) {
    beginAction(props.action);
  } else if (!isOpen) {
    resetAction();
  }
});

function getActionTitle() {
  if (!actionType.value || !props.player) return "";

  const actionMap = {
    kick: "Kick",
    ban: "Ban",
    warn: "Warn",
    move: "Move",
    "remove-from-squad": "Remove from Squad",
  } as const;
  return `${actionMap[actionType.value]} ${props.player.name}`;
}

function closeActionDialog() {
  open.value = false;
}

async function submit() {
  const result = await executePlayerAction();
  if (result === "invalid") return; // keep dialog open for correction
  if (result === "success") emit("completed");
  open.value = false;
}
</script>

<template>
  <Dialog v-model:open="open">
    <DialogContent class="sm:max-w-[425px]">
      <DialogHeader>
        <DialogTitle>{{ getActionTitle() }}</DialogTitle>
        <DialogDescription>
          <template v-if="actionType === 'kick'">
            Kick this player from the server. They will be able to rejoin.
          </template>
          <template v-else-if="actionType === 'ban'">
            Ban this player from the server for a specified duration.
          </template>
          <template v-else-if="actionType === 'warn'">
            Send a warning message to this player.
          </template>
          <template v-else-if="actionType === 'move'">
            Force this player to switch to another team.
          </template>
          <template v-else-if="actionType === 'remove-from-squad'">
            Remove this player from their squad.
          </template>
        </DialogDescription>
      </DialogHeader>

      <div class="grid gap-4 py-4">
        <!-- Rule Selection (for kick, ban, warn) -->
        <div
          v-if="
            actionType === 'kick' ||
            actionType === 'ban' ||
            actionType === 'warn'
          "
          class="grid grid-cols-4 items-center gap-4"
        >
          <label for="rule" class="text-right col-span-1">Rule Violation</label>
          <Select
            v-model="selectedRuleId"
            @update:model-value="(value) => onRuleSelected(value as string)"
          >
            <SelectTrigger class="col-span-3">
              <SelectValue placeholder="Optional: Select a violated rule" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="__none__"> No rule violation </SelectItem>
              <SelectItem
                v-for="rule in serverRules"
                :key="rule.id"
                :value="rule.id"
              >
                {{ rule.displayNumber }}: {{ rule.title }}
              </SelectItem>
            </SelectContent>
          </Select>
        </div>

        <div
          v-if="actionType === 'ban'"
          class="grid grid-cols-4 items-center gap-4"
        >
          <label for="duration" class="text-right col-span-1">Duration</label>
          <Input
            id="duration"
            v-model="actionDuration"
            placeholder="0"
            class="col-span-3"
            type="text"
          />
          <div class="col-span-1"></div>
          <div class="text-xs text-muted-foreground col-span-3">
            Duration: 0 = permanent, 7d = 7 days, 2h = 2 hours, 30m = 30 minutes
          </div>
        </div>

        <div
          v-if="actionType !== 'move' && actionType !== 'remove-from-squad'"
          class="grid grid-cols-4 items-center gap-4"
        >
          <label for="reason" class="text-right col-span-1">
            {{ actionType === "warn" ? "Message" : "Reason" }}
          </label>
          <Textarea
            id="reason"
            v-model="actionReason"
            :placeholder="
              actionType === 'warn' ? 'Warning message' : 'Reason for action'
            "
            class="col-span-3"
            rows="3"
          />
        </div>

        <div
          v-if="actionType === 'remove-from-squad' && player"
          class="text-sm text-muted-foreground"
        >
          Are you sure you want to remove {{ player.name }} from their squad?
        </div>
      </div>

      <!-- Escalation Loading -->
      <div
        v-if="loadingEscalation"
        class="p-3 bg-muted border rounded-lg text-sm text-muted-foreground flex items-center gap-2"
      >
        <Icon name="mdi:loading" class="h-4 w-4 animate-spin" />
        Loading escalation suggestion...
      </div>

      <!-- Escalation Suggestion -->
      <div
        v-if="
          !loadingEscalation &&
          escalationSuggestion &&
          (actionType === 'kick' ||
            actionType === 'ban' ||
            actionType === 'warn')
        "
        class="gap-4 p-3 bg-muted border rounded-lg"
      >
        <div class="col-span-4">
          <div class="flex items-start space-x-2">
            <Icon
              name="mdi:information"
              class="h-5 w-5 text-primary mt-0.5 flex-shrink-0"
            />
            <div class="flex-1">
              <div class="flex items-center justify-between">
                <p class="text-sm font-medium">Escalation Suggestion</p>
                <Button
                  v-if="
                    escalationSuggestion.suggested_action &&
                    escalationSuggestion.suggested_action.toLowerCase() !==
                      actionType
                  "
                  @click="switchToSuggestedAction"
                  variant="default"
                  size="sm"
                  class="h-7 text-xs"
                >
                  Switch to
                  {{ escalationSuggestion.suggested_action }}
                </Button>
              </div>
              <p class="text-xs text-muted-foreground mt-1">
                This player has violated this rule
                <strong>{{ escalationSuggestion.violation_count }}</strong>
                time(s).
                <template v-if="escalationSuggestion.suggested_action">
                  Suggested action:
                  <strong>{{ escalationSuggestion.suggested_action }}</strong>
                  <template v-if="escalationSuggestion.suggested_duration">
                    for
                    <strong
                      >{{ escalationSuggestion.suggested_duration }}
                      days</strong
                    >
                  </template>
                </template>
              </p>
            </div>
          </div>
        </div>
      </div>

      <DialogFooter>
        <Button variant="outline" @click="closeActionDialog">Cancel</Button>
        <Button
          :variant="
            actionType === 'warn' || actionType === 'move'
              ? 'default'
              : 'destructive'
          "
          @click="submit"
          :disabled="isActionLoading"
        >
          <span v-if="isActionLoading" class="mr-2">
            <Icon name="mdi:loading" class="h-4 w-4 animate-spin" />
          </span>
          <template v-if="actionType === 'kick'">Kick Player</template>
          <template v-else-if="actionType === 'ban'">Ban Player</template>
          <template v-else-if="actionType === 'warn'">Send Warning</template>
          <template v-else-if="actionType === 'move'">Move Player</template>
          <template v-else-if="actionType === 'remove-from-squad'"
            >Remove from Squad</template
          >
        </Button>
      </DialogFooter>
    </DialogContent>
  </Dialog>
</template>
