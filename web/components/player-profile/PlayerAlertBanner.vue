<script setup lang="ts">
import { computed } from "vue";
import { AlertTriangle, Ban, ShieldAlert } from "lucide-vue-next";
import type { ActiveBan, RiskIndicator } from "~/types/player";

const props = defineProps<{
  activeBans: ActiveBan[];
  riskIndicators: RiskIndicator[];
}>();

const criticalIndicators = computed(() =>
  props.riskIndicators.filter((i) => i.severity === "critical")
);

const highIndicators = computed(() =>
  props.riskIndicators.filter((i) => i.severity === "high")
);

const showBanner = computed(
  () => props.activeBans.length > 0 || criticalIndicators.value.length > 0
);

function formatDate(dateString: string): string {
  return new Date(dateString).toLocaleString();
}
</script>

<template>
  <div
    v-if="showBanner"
    class="rounded-lg border-2 border-destructive bg-destructive/10 p-4 mb-6"
  >
    <!-- Active Bans -->
    <div v-if="activeBans.length > 0" class="mb-4">
      <div class="flex items-center gap-2 mb-3">
        <Ban class="h-5 w-5 text-destructive" />
        <span class="font-bold text-destructive text-lg">
          CURRENTLY BANNED ({{ activeBans.length }})
        </span>
      </div>
      <div class="space-y-2">
        <div
          v-for="ban in activeBans"
          :key="ban.ban_id"
          class="bg-destructive/20 rounded p-3"
        >
          <div class="flex flex-wrap items-center gap-2 text-sm">
            <span class="font-semibold">{{ ban.server_name }}</span>
            <span class="text-muted-foreground">|</span>
            <span>{{ ban.reason }}</span>
          </div>
          <div class="text-xs text-muted-foreground mt-1">
            <span v-if="ban.permanent" class="text-destructive font-semibold"
              >PERMANENT</span
            >
            <span v-else>Expires: {{ formatDate(ban.expires_at!) }}</span>
            <span class="mx-2">|</span>
            <span>Banned by: {{ ban.admin_name }}</span>
            <span class="mx-2">|</span>
            <span>{{ formatDate(ban.created_at) }}</span>
          </div>
        </div>
      </div>
    </div>

    <!-- Critical Risk Indicators -->
    <div v-if="criticalIndicators.length > 0 && activeBans.length === 0">
      <div class="flex items-center gap-2 mb-2">
        <ShieldAlert class="h-5 w-5 text-destructive" />
        <span class="font-bold text-destructive">CRITICAL ALERTS</span>
      </div>
      <ul class="list-disc list-inside text-sm space-y-1">
        <li v-for="indicator in criticalIndicators" :key="indicator.type">
          {{ indicator.description }}
        </li>
      </ul>
    </div>

    <!-- High Risk Indicators -->
    <div v-if="highIndicators.length > 0" class="mt-3">
      <div class="flex items-center gap-2 mb-2">
        <AlertTriangle class="h-4 w-4 text-orange-500" />
        <span class="font-semibold text-orange-500 text-sm"
          >HIGH RISK FACTORS</span
        >
      </div>
      <ul class="list-disc list-inside text-xs text-muted-foreground space-y-1">
        <li v-for="indicator in highIndicators" :key="indicator.type">
          {{ indicator.description }}
        </li>
      </ul>
    </div>
  </div>
</template>
