<script setup lang="ts">
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "~/components/ui/dialog";
import { Badge } from "~/components/ui/badge";
import { Button } from "~/components/ui/button";
import { Separator } from "~/components/ui/separator";
import {
  type AuditLog,
  getActionBadgeColor,
  formatActionType,
  formatJsonForDisplay,
  hasDetailedChanges,
} from "~/utils/audit";

defineProps<{
  log: AuditLog | null;
}>();

const open = defineModel<boolean>("open", { default: false });

function formatDate(dateString: string): string {
  return new Date(dateString).toLocaleString();
}
</script>

<template>
  <Dialog v-model:open="open">
    <DialogContent
      class="max-h-[85vh] w-[95vw] p-4 sm:max-h-[80vh] sm:max-w-[700px] sm:p-6"
    >
      <DialogHeader>
        <DialogTitle class="flex items-center gap-2 text-base sm:text-lg">
          <Icon name="lucide:file-text" class="h-5 w-5" />
          Audit Log Details
        </DialogTitle>
        <DialogDescription class="text-xs sm:text-sm">
          Complete information about this administrative action.
        </DialogDescription>
      </DialogHeader>

      <div v-if="log" class="max-h-[60vh] overflow-y-auto">
        <div class="space-y-4 pr-2 sm:space-y-6 sm:pr-4">
          <div class="grid grid-cols-1 gap-4 sm:grid-cols-2 sm:gap-6">
            <div class="space-y-3">
              <div>
                <h3 class="text-xs font-medium text-muted-foreground sm:text-sm">
                  Timestamp
                </h3>
                <p class="break-all font-mono text-xs sm:text-sm">
                  {{ formatDate(log.timestamp) }}
                </p>
              </div>
              <div>
                <h3 class="text-xs font-medium text-muted-foreground sm:text-sm">
                  User
                </h3>
                <div class="mt-1 flex items-center gap-2">
                  <div
                    class="flex h-6 w-6 shrink-0 items-center justify-center rounded-full bg-primary/10"
                  >
                    <span class="text-xs font-medium text-primary">
                      {{ log.username.charAt(0).toUpperCase() }}
                    </span>
                  </div>
                  <span class="break-all text-sm font-medium sm:text-base">
                    {{ log.username }}
                  </span>
                </div>
              </div>
            </div>

            <div class="space-y-3">
              <div>
                <h3 class="text-xs font-medium text-muted-foreground sm:text-sm">
                  Action Type
                </h3>
                <Badge
                  variant="outline"
                  :class="getActionBadgeColor(log.action)"
                  class="mt-1 text-xs font-medium"
                >
                  {{ formatActionType(log.action) }}
                </Badge>
              </div>
              <div>
                <h3 class="text-xs font-medium text-muted-foreground sm:text-sm">
                  Action
                </h3>
                <Badge
                  variant="outline"
                  :class="getActionBadgeColor(log.action)"
                  class="mt-1 break-all text-xs font-medium"
                >
                  {{ log.action }}
                </Badge>
              </div>
              <div>
                <h3 class="text-xs font-medium text-muted-foreground sm:text-sm">
                  Log ID
                </h3>
                <p class="break-all font-mono text-xs text-muted-foreground">
                  {{ log.id }}
                </p>
              </div>
            </div>
          </div>

          <Separator />

          <div v-if="hasDetailedChanges(log.changes)">
            <h3 class="mb-2 text-xs font-medium text-muted-foreground sm:text-sm">
              Raw Data
            </h3>
            <div class="rounded-md border bg-muted/50">
              <div class="border-b bg-muted/30 p-2 sm:p-3">
                <div class="flex items-center justify-between">
                  <span class="text-xs font-medium text-muted-foreground">
                    JSON
                  </span>
                  <Button
                    variant="ghost"
                    size="sm"
                    class="h-6 px-2 text-xs"
                    @click="copyToClipboard(formatJsonForDisplay(log.changes))"
                  >
                    Copy
                  </Button>
                </div>
              </div>
              <div class="max-h-[250px] overflow-y-auto sm:max-h-[300px]">
                <pre
                  class="whitespace-pre-wrap break-words p-2 font-mono text-xs sm:p-3"
                  >{{ formatJsonForDisplay(log.changes) }}</pre
                >
              </div>
            </div>
          </div>

          <div v-else class="py-6 text-center text-muted-foreground sm:py-8">
            <Icon name="lucide:inbox" class="mx-auto mb-2 h-8 w-8" />
            <p class="text-xs sm:text-sm">
              No additional data available for this action
            </p>
          </div>
        </div>
      </div>
    </DialogContent>
  </Dialog>
</template>
