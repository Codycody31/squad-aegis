import { ref, toValue, type MaybeRefOrGetter } from "vue";
import { toast } from "~/components/ui/toast";
import type { Player } from "~/types";

export type PlayerAdminAction =
  | "kick"
  | "ban"
  | "warn"
  | "move"
  | "remove-from-squad";

export interface FlatRule {
  id: string;
  title: string;
  displayNumber: string;
  description?: string;
}

export function getPlayerEOSID(player: Player): string {
  return player.eosId || player.eos_id || "";
}

export function getPlayerProfileID(player: Player): string {
  return player.steam_id || getPlayerEOSID(player);
}

// Flatten rules hierarchy for dropdown
export function flattenRulesForDropdown(
  rules: any[],
  parentTitle = "",
  parentNumber = "",
  result: any[] = []
): FlatRule[] {
  rules.forEach((rule, idx) => {
    // Calculate rule number (e.g., "1", "1.2", "1.2.3")
    const ruleNumber = parentNumber
      ? `${parentNumber}.${idx + 1}`
      : `${idx + 1}`;
    const formattedTitle = parentTitle
      ? `${parentTitle} > ${rule.title}`
      : rule.title;

    result.push({
      id: rule.id,
      title: formattedTitle,
      displayNumber: ruleNumber,
      description: rule.description,
    });

    if (rule.sub_rules && rule.sub_rules.length > 0) {
      flattenRulesForDropdown(rule.sub_rules, formattedTitle, ruleNumber, result);
    }
  });

  return result;
}

const banDurationPattern = /^(0|permanent|\d+[dDhHmM])$/;

/**
 * Shared state + logic for the kick/ban/warn/move/remove-from-squad
 * moderation flow, consumed by PlayerActionDialog (which is embedded by
 * both PlayerActionMenu and the player context panel).
 */
export function usePlayerAdminActions(
  serverId: MaybeRefOrGetter<string>,
  player: MaybeRefOrGetter<Player | null>
) {
  const actionType = ref<PlayerAdminAction | null>(null);
  const actionReason = ref("");
  const actionDuration = ref("0"); // "0" for permanent, "7d", "2h", "30m"
  const selectedRuleId = ref<string>("__none__");
  const serverRules = ref<FlatRule[]>([]);
  const escalationSuggestion = ref<any>(null);
  const loadingEscalation = ref(false);
  const isActionLoading = ref(false);
  const rulesFetched = ref(false);

  async function fetchServerRules() {
    const runtimeConfig = useRuntimeConfig();
    try {
      const data = await useAuthFetchImperative<any>(
        `${runtimeConfig.public.backendApi}/servers/${toValue(serverId)}/rules`
      );
      serverRules.value = flattenRulesForDropdown(
        Array.isArray(data) ? data : []
      );
      rulesFetched.value = true;
    } catch (err: any) {
      console.error("Failed to fetch server rules:", err);
    }
  }

  async function fetchEscalationSuggestion(ruleId: string) {
    const p = toValue(player);
    if (!ruleId || ruleId === "" || ruleId === "__none__") {
      escalationSuggestion.value = null;
      return;
    }
    if (!p) return;

    const eosID = getPlayerEOSID(p);
    if (!p.steam_id && !eosID) {
      escalationSuggestion.value = null;
      return;
    }

    loadingEscalation.value = true;
    const runtimeConfig = useRuntimeConfig();

    try {
      const params = new URLSearchParams({ rule_id: ruleId });
      if (p.steam_id) {
        params.set("steam_id", p.steam_id);
      }
      if (eosID) {
        params.set("eos_id", eosID);
      }

      const data = await useAuthFetchImperative<any>(
        `${runtimeConfig.public.backendApi}/servers/${toValue(serverId)}/rcon/player/escalation-suggestion?${params.toString()}`
      );

      if (data?.data?.suggestion) {
        const suggestion = data.data.suggestion;
        escalationSuggestion.value = suggestion;

        // Generate reason from rule number and title if no custom message is provided
        if (
          !actionReason.value &&
          suggestion.rule_id &&
          !suggestion.suggested_message
        ) {
          const rule = serverRules.value.find(
            (r) => r.id === suggestion.rule_id
          );
          if (rule) {
            actionReason.value = `${rule.displayNumber} | ${rule.title}`;
          }
        } else if (!actionReason.value && suggestion.suggested_message) {
          // Use custom message from rule action if available
          actionReason.value = suggestion.suggested_message;
        }

        // Prefill duration if suggestion provides one for ban
        if (
          actionType.value === "ban" &&
          suggestion.suggested_duration &&
          actionDuration.value === "0"
        ) {
          actionDuration.value = String(suggestion.suggested_duration) + "d";
        }
      } else {
        escalationSuggestion.value = null;
      }
    } catch (err: any) {
      console.error("Failed to fetch escalation suggestion:", err);
      escalationSuggestion.value = null;
    } finally {
      loadingEscalation.value = false;
    }
  }

  function onRuleSelected(ruleId: string) {
    if (toValue(player) && ruleId && ruleId !== "" && ruleId !== "__none__") {
      fetchEscalationSuggestion(ruleId);
    } else {
      escalationSuggestion.value = null;
    }
  }

  function switchToSuggestedAction() {
    if (
      !escalationSuggestion.value ||
      !escalationSuggestion.value.suggested_action
    ) {
      return;
    }

    const suggestedAction =
      escalationSuggestion.value.suggested_action.toLowerCase();

    if (
      suggestedAction === "kick" ||
      suggestedAction === "ban" ||
      suggestedAction === "warn"
    ) {
      actionType.value = suggestedAction as "kick" | "ban" | "warn";

      if (
        suggestedAction === "ban" &&
        escalationSuggestion.value.suggested_duration
      ) {
        actionDuration.value =
          String(escalationSuggestion.value.suggested_duration) + "d";
      } else if (suggestedAction !== "ban") {
        actionDuration.value = "0";
      }

      if (!actionReason.value) {
        if (escalationSuggestion.value.suggested_message) {
          actionReason.value = escalationSuggestion.value.suggested_message;
        } else if (escalationSuggestion.value.rule_id) {
          const rule = serverRules.value.find(
            (r) => r.id === escalationSuggestion.value.rule_id
          );
          if (rule) {
            actionReason.value = `Rule #${rule.displayNumber}: ${rule.title}`;
          }
        }
      }
    }
  }

  async function beginAction(action: PlayerAdminAction) {
    actionType.value = action;
    actionReason.value = "";
    actionDuration.value = "0";
    selectedRuleId.value = "__none__";
    escalationSuggestion.value = null;

    // Only fetch rules if needed for this action and haven't fetched yet
    if (
      (action === "kick" || action === "ban" || action === "warn") &&
      !rulesFetched.value
    ) {
      await fetchServerRules();
      rulesFetched.value = true;
    }
  }

  function resetAction() {
    actionType.value = null;
    actionReason.value = "";
    actionDuration.value = "0";
    selectedRuleId.value = "__none__";
    escalationSuggestion.value = null;
  }

  /**
   * Executes the pending action.
   * Returns "invalid" when validation failed (dialog should stay open),
   * "success" after a successful call, "error" after a failed call.
   */
  async function executePlayerAction(): Promise<
    "invalid" | "success" | "error"
  > {
    const p = toValue(player);
    if (!actionType.value || !p) return "invalid";

    // Validate ban duration format before submitting
    if (
      actionType.value === "ban" &&
      !banDurationPattern.test(actionDuration.value)
    ) {
      toast({
        title: "Invalid Duration",
        description:
          "Duration must be '0' for permanent, or a number followed by 'd', 'h', or 'm' (e.g., '7d', '2h', '30m')",
        variant: "destructive",
      });
      return "invalid";
    }

    isActionLoading.value = true;
    const runtimeConfig = useRuntimeConfig();
    const backendApi = runtimeConfig.public.backendApi;
    const sid = toValue(serverId);

    try {
      let endpoint = "";
      let payload: any = {};

      switch (actionType.value) {
        case "kick":
          endpoint = `${backendApi}/servers/${sid}/rcon/player/kick`;
          payload = {
            steam_id: p.steam_id,
            eos_id: getPlayerEOSID(p),
            reason: actionReason.value,
          };
          if (
            selectedRuleId.value &&
            selectedRuleId.value !== "" &&
            selectedRuleId.value !== "__none__"
          ) {
            payload.rule_id = selectedRuleId.value;
          }
          break;
        case "ban":
          endpoint = `${backendApi}/servers/${sid}/rcon/player/ban`;
          payload = {
            steam_id: p.steam_id,
            eos_id: getPlayerEOSID(p),
            reason: actionReason.value,
            duration: actionDuration.value,
          };
          if (
            selectedRuleId.value &&
            selectedRuleId.value !== "" &&
            selectedRuleId.value !== "__none__"
          ) {
            payload.rule_id = selectedRuleId.value;
          }
          break;
        case "warn":
          endpoint = `${backendApi}/servers/${sid}/rcon/player/warn`;
          payload = {
            steam_id: p.steam_id,
            eos_id: getPlayerEOSID(p),
            message: actionReason.value,
          };
          if (
            selectedRuleId.value &&
            selectedRuleId.value !== "" &&
            selectedRuleId.value !== "__none__"
          ) {
            payload.rule_id = selectedRuleId.value;
          }
          break;
        case "move":
          endpoint = `${backendApi}/servers/${sid}/rcon/move-player`;
          payload = {
            steam_id: p.steam_id,
            eos_id: getPlayerEOSID(p),
          };
          break;
        case "remove-from-squad":
          endpoint = `${backendApi}/servers/${sid}/rcon/execute`;
          payload = {
            command: `AdminRemovePlayerFromSquadById ${p.playerId}`,
          };
          break;
      }

      await useAuthFetchImperative(endpoint, {
        method: "POST",
        body: payload,
      });

      let successMessage = `Player ${p.name} has been `;
      if (actionType.value === "move") {
        successMessage += "moved";
      } else if (actionType.value === "ban") {
        successMessage += "banned";
        if (actionDuration.value && actionDuration.value !== "0") {
          successMessage += ` for ${actionDuration.value}`;
        } else {
          successMessage += " permanently";
        }
      } else if (actionType.value === "remove-from-squad") {
        successMessage += "removed from squad";
      } else {
        successMessage += actionType.value + "ed";
      }

      toast({
        title: "Success",
        description: successMessage,
        variant: "default",
      });

      return "success";
    } catch (err: any) {
      console.error(err);
      toast({
        title: "Error",
        description: extractApiErrorMessage(
          err,
          `Failed to ${actionType.value}`
        ),
        variant: "destructive",
      });
      return "error";
    } finally {
      isActionLoading.value = false;
    }
  }

  return {
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
  };
}
