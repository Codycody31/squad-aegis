<script setup lang="ts">
import { computed, onMounted, ref, watch } from "vue";
import { Button } from "~/components/ui/button";
import { Input } from "~/components/ui/input";
import { Textarea } from "~/components/ui/textarea";
import {
    Dialog,
    DialogContent,
    DialogDescription,
    DialogFooter,
    DialogHeader,
    DialogTitle,
} from "~/components/ui/dialog";
import {
    FormControl,
    FormDescription,
    FormField,
    FormItem,
    FormLabel,
    FormMessage,
} from "~/components/ui/form";
import {
    Select,
    SelectContent,
    SelectItem,
    SelectTrigger,
    SelectValue,
} from "~/components/ui/select";
import { toast } from "~/components/ui/toast";
import { useForm } from "vee-validate";
import { toTypedSchema } from "@vee-validate/zod";
import * as z from "zod";
import BanEvidenceEditor from "~/components/bans/BanEvidenceEditor.vue";
import PlayerSearchField from "~/components/bans/PlayerSearchField.vue";
import { cleanPlayerId } from "~/utils/evidence";
import type { BanList, BannedPlayer, ServerRuleOption } from "~/types/bans";

// Merged add/edit ban dialog.
// - create: POST /servers/:id/bans with the full request body.
// - edit:   PUT /servers/:id/bans/:banId with a changed-fields-only diff,
//           including the hadOriginalEvidence rule for clearing evidence.
// Rules and ban lists are fetched here since this dialog is their only
// consumer on the page.

const props = defineProps<{
    mode: "create" | "edit";
    serverId: string;
    ban?: BannedPlayer | null;
}>();

const open = defineModel<boolean>("open", { default: false });

const emit = defineEmits<{
    (e: "saved"): void;
}>();

// Form schema for adding a ban
const formSchema = toTypedSchema(
    z.object({
        steam_id: z
            .string()
            .min(1, "Player ID is required")
            .refine(
                (val) => /^\d{17}$/.test(val) || /^[0-9a-fA-F]{32}$/.test(val),
                "Must be a 17-digit Steam ID or 32-character hex EOS ID"
            ),
        reason: z.string().optional(), // Now optional - will be auto-generated when rule is selected
        duration: z.string().default("0").refine(
            (val) => /^(0|permanent|\d+[dDhHmM])$/.test(val),
            "Duration must be '0' for permanent, or a number followed by 'd', 'h', or 'm' (e.g., '7d', '2h', '30m')"
        ),
        ban_list_id: z.string().optional(),
        rule_id: z.string().optional(),
        evidence_text: z.string().optional(),
    }),
);

// Form schema for editing a ban
const editFormSchema = toTypedSchema(
    z.object({
        reason: z.string().min(1, "Reason is required"),
        duration: z.string().default("0").refine(
            (val) => /^(0|permanent|\d+[dDhHmM])$/.test(val),
            "Duration must be '0' for permanent, or a number followed by 'd', 'h', or 'm' (e.g., '7d', '2h', '30m')"
        ),
        ban_list_id: z.string().optional(),
        rule_id: z.string().optional(),
        evidence_text: z.string().optional(),
    }),
);

const form = useForm<Record<string, any>>({
    validationSchema: computed(() =>
        props.mode === "create" ? formSchema : editFormSchema,
    ),
    initialValues: {
        steam_id: "",
        reason: "",
        duration: "1d",
        ban_list_id: "",
        rule_id: "",
    },
    keepValuesOnUnmount: true,
});
const formValues = form.values as Record<string, any>;

const saving = ref(false);
const error = ref<string | null>(null);
const banLists = ref<BanList[]>([]);
const serverRules = ref<ServerRuleOption[]>([]);
const initialDuration = ref("0");
// Bumped on every open so the inner form subtree (player picker, evidence
// editor) remounts with fresh internal state between dialog sessions.
const formSession = ref(0);

// Evidence draft state is dialog-local and reset on every open, so drafts
// never leak between dialog sessions.
const selectedEvidence = ref<any[]>([]);
const uploadedFiles = ref<any[]>([]);
const textEvidenceItems = ref<any[]>([]);

// Suggested-duration hint reported by PlayerSearchField (create mode only).
const historyCount = ref(0);
const suggestedDuration = ref(24);

const CREATE_SEARCH_HINT =
    'Select a player above and click "Search Events" to find chat messages or connections';

function onSuggestion(payload: { historyCount: number; suggestedDuration: number }) {
    historyCount.value = payload.historyCount;
    suggestedDuration.value = payload.suggestedDuration;
}

function applySuggestedDuration() {
    form.setFieldValue(
        "duration",
        suggestedDuration.value === 0 ? "0" : suggestedDuration.value + "d",
    );
}

// Helper function to get rule details by ID
function getSelectedRuleDetails(ruleId: string) {
    if (!ruleId) return null;
    return serverRules.value.find((r) => r.id === ruleId);
}

// Generate ban reason from rule and duration
function generateBanReason(ruleId: string | undefined, duration: string | undefined): string {
    if (!ruleId || ruleId === "__none__") return "";

    const rule = getSelectedRuleDetails(ruleId);
    if (!rule) return "";

    // Format: "rule_number | rule_title | duration"
    const titleParts = rule.title.split(" > ");
    const shortTitle = titleParts[titleParts.length - 1];

    const durationStr = duration || "0";
    const durationText = durationStr === "0" || durationStr === "permanent" ? "perm" : durationStr;
    return `${rule.number} | ${shortTitle} | ${durationText}`;
}

function getInitialEditBanDuration(ban: BannedPlayer | null): string {
    if (!ban || ban.permanent || !ban.expires_at) {
        return "0";
    }

    const expiresAt = new Date(ban.expires_at);
    const remainingMs = expiresAt.getTime() - Date.now();

    if (!Number.isFinite(remainingMs) || remainingMs <= 0) {
        return "1m";
    }

    const remainingMinutes = Math.max(1, Math.ceil(remainingMs / (60 * 1000)));

    if (remainingMinutes % (24 * 60) === 0) {
        return `${remainingMinutes / (24 * 60)}d`;
    }

    if (remainingMinutes % 60 === 0) {
        return `${remainingMinutes / 60}h`;
    }

    return `${remainingMinutes}m`;
}

// Function to fetch ban lists
async function fetchBanLists() {
    const runtimeConfig = useRuntimeConfig();

    try {
        const response = (await useAuthFetchImperative(
            `${runtimeConfig.public.backendApi}/ban-lists`,
        )) as any;

        if (response?.data?.ban_lists) {
            banLists.value = response.data.ban_lists;
        }
    } catch (err) {
        console.error("Failed to fetch ban lists:", err);
    }
}

// Function to fetch server rules
async function fetchServerRules() {
    const runtimeConfig = useRuntimeConfig();

    try {
        const { data, error: fetchError } = await useAuthFetch(
            `${runtimeConfig.public.backendApi}/servers/${props.serverId}/rules`,
            {
                method: "GET",
            },
        );

        if (fetchError.value) {
            throw new Error(
                extractApiErrorMessage(fetchError.value, "Failed to fetch server rules"),
            );
        }

        if (data.value) {
            // Flatten the rules hierarchy for easier selection in dropdown
            serverRules.value = flattenRulesForDropdown(
                Array.isArray(data.value) ? data.value : [],
            );
        }
    } catch (err: any) {
        console.error("Failed to fetch server rules:", err);
    }
}

// Helper function to flatten rules hierarchy for dropdown
function flattenRulesForDropdown(
    rules: any[],
    parentTitle = "",
    parentNumber = "",
    result: any[] = [],
) {
    rules.forEach((rule, index) => {
        // Create a formatted title that shows the hierarchy
        const formattedTitle = parentTitle
            ? `${parentTitle} > ${rule.title}`
            : rule.title;

        // Create a rule number (e.g., "1.2.3")
        const ruleNumber = parentNumber
            ? `${parentNumber}.${index + 1}`
            : `${index + 1}`;

        result.push({
            id: rule.id,
            title: formattedTitle,
            description: rule.description,
            number: ruleNumber,
        });

        // Process sub-rules if they exist
        if (rule.sub_rules && rule.sub_rules.length > 0) {
            flattenRulesForDropdown(
                rule.sub_rules,
                formattedTitle,
                ruleNumber,
                result,
            );
        }
    });

    return result;
}

onMounted(() => {
    fetchBanLists();
    fetchServerRules();
});

// Populate the form on every open: reset drafts, then rehydrate from the ban
// being edited (edit mode) or start fresh (create mode).
watch(open, async (isOpen) => {
    if (!isOpen) return;

    error.value = null;
    formSession.value++;
    selectedEvidence.value = [];
    uploadedFiles.value = [];
    textEvidenceItems.value = [];
    historyCount.value = 0;
    suggestedDuration.value = 24;

    if (props.mode === "edit" && props.ban) {
        const ban = props.ban;
        initialDuration.value = getInitialEditBanDuration(ban);

        // Load existing evidence if present
        if (ban.evidence && ban.evidence.length > 0) {
            ban.evidence.forEach((ev) => {
                if (ev.evidence_type === "file_upload") {
                    uploadedFiles.value.push({
                        evidence_type: "file_upload",
                        file_path: ev.file_path,
                        file_name: ev.file_name,
                        file_size: ev.file_size,
                        file_type: ev.file_type,
                    });
                } else if (ev.evidence_type === "text_paste") {
                    textEvidenceItems.value.push({
                        evidence_type: "text_paste",
                        text_content: ev.text_content,
                    });
                } else {
                    // ClickHouse event evidence
                    selectedEvidence.value.push({
                        evidence_type: ev.evidence_type,
                        clickhouse_table: ev.clickhouse_table,
                        record_id: ev.record_id,
                        event_time: ev.event_time,
                        metadata: ev.metadata || {},
                    });
                }
            });
        }

        // Ensure rules are loaded
        if (serverRules.value.length === 0) {
            await fetchServerRules();
        }

        form.resetForm({
            values: {
                steam_id: "",
                reason: ban.reason || "",
                duration: initialDuration.value,
                ban_list_id: ban.ban_list_id || "",
                rule_id: ban.rule_id || "",
                evidence_text: ban.evidence_text || "",
            },
        });
    } else {
        initialDuration.value = "0";
        form.resetForm({
            values: {
                steam_id: "",
                reason: "",
                duration: "1d",
                ban_list_id: "",
                rule_id: "",
                evidence_text: "",
            },
        });
    }
});

const onSubmit = form.handleSubmit(async (values) => {
    if (props.mode === "edit") {
        await editBan(values);
    } else {
        await addBan(values);
    }
});

// Function to add a ban
async function addBan(values: any) {
    const { steam_id, reason, duration, ban_list_id, rule_id, evidence_text } =
        values;

    saving.value = true;
    error.value = null;

    const runtimeConfig = useRuntimeConfig();

    try {
        // Clean and validate player ID (Steam ID or EOS ID)
        const playerIds = cleanPlayerId(steam_id);

        // Check if a valid rule is selected (not __none__ sentinel value)
        const hasValidRule = rule_id && rule_id.trim() && rule_id !== "__none__";

        // Generate reason: if rule is selected, generate it dynamically, otherwise use provided reason
        let finalReason = reason || "";
        if (hasValidRule) {
            finalReason = generateBanReason(rule_id, duration ?? "0");
        }

        // Validate that we have a reason
        if (!finalReason) {
            throw new Error("Reason is required. Either select a rule or provide a custom reason.");
        }

        const requestBody: any = {
            reason: finalReason,
            duration,
        };
        if (playerIds.steamId) {
            requestBody.steam_id = playerIds.steamId;
        }
        if (playerIds.eosId) {
            requestBody.eos_id = playerIds.eosId;
        }

        // Add ban_list_id if selected
        if (ban_list_id && ban_list_id.trim()) {
            requestBody.ban_list_id = ban_list_id;
        }

        // Add rule_id if selected (not __none__)
        if (hasValidRule) {
            requestBody.rule_id = rule_id;
        }

        // Add evidence text if provided
        if (evidence_text && evidence_text.trim()) {
            requestBody.evidence_text = evidence_text;
        }

        // Combine all evidence types
        const allEvidence: any[] = [];

        // Add ClickHouse event evidence
        if (selectedEvidence.value.length > 0) {
            allEvidence.push(...selectedEvidence.value.map((ev) => ({
                evidence_type: ev.evidence_type,
                clickhouse_table: ev.clickhouse_table,
                record_id: ev.record_id,
                event_time: ev.event_time,
                metadata: ev.metadata || {},
            })));
        }

        // Add file upload evidence
        if (uploadedFiles.value.length > 0) {
            allEvidence.push(...uploadedFiles.value.map((file) => ({
                evidence_type: "file_upload",
                file_path: file.file_path,
                file_name: file.file_name,
                file_size: file.file_size,
                file_type: file.file_type,
            })));
        }

        // Add text paste evidence
        if (textEvidenceItems.value.length > 0) {
            allEvidence.push(...textEvidenceItems.value.map((text) => ({
                evidence_type: "text_paste",
                text_content: text.text_content,
            })));
        }

        if (allEvidence.length > 0) {
            requestBody.evidence = allEvidence;
        }

        const { data, error: fetchError } = await useAuthFetch(
            `${runtimeConfig.public.backendApi}/servers/${props.serverId}/bans`,
            {
                method: "POST",
                body: requestBody,
            },
        );

        if (fetchError.value) {
            throw new Error(extractApiErrorMessage(fetchError.value, "Failed to add ban"));
        }

        // Reset form and close dialog
        form.resetForm();
        selectedEvidence.value = [];
        uploadedFiles.value = [];
        textEvidenceItems.value = [];
        open.value = false;

        toast({
            title: "Success",
            description: "Ban added successfully",
        });

        // Refresh the banned players list
        emit("saved");
    } catch (err: any) {
        error.value = extractApiErrorMessage(err, "An error occurred while adding the ban");
        console.error(err);
    } finally {
        saving.value = false;
    }
}

// Function to edit a ban
async function editBan(values: any) {
    const { reason, duration, ban_list_id, rule_id, evidence_text } = values;

    if (!props.ban) {
        error.value = "No ban selected for editing";
        return;
    }

    saving.value = true;
    error.value = null;

    const runtimeConfig = useRuntimeConfig();

    try {
        const requestBody: any = {};

        // Only include fields that were actually changed
        if (reason !== props.ban.reason) {
            requestBody.reason = reason;
        }

        if (duration && duration !== initialDuration.value) {
            requestBody.duration = duration;
        }

        // Handle ban list changes
        const currentBanListId = props.ban.ban_list_id || "";
        const newBanListId = ban_list_id || "";

        if (currentBanListId !== newBanListId) {
            requestBody.ban_list_id = newBanListId || null;
        }

        // Handle rule changes ("__none__" means the rule was explicitly unset)
        const currentRuleId = props.ban.rule_id || "";
        const newRuleId = rule_id && rule_id !== "__none__" ? rule_id : "";

        if (currentRuleId !== newRuleId) {
            requestBody.rule_id = newRuleId || null;
        }

        // Handle evidence text changes
        const currentEvidenceText = props.ban.evidence_text || "";
        const newEvidenceText = evidence_text || "";

        if (currentEvidenceText !== newEvidenceText) {
            requestBody.evidence_text = newEvidenceText;
        }

        // Combine all evidence types
        const allEvidence: any[] = [];

        // Add ClickHouse event evidence
        if (selectedEvidence.value.length > 0) {
            allEvidence.push(...selectedEvidence.value.map((ev) => ({
                evidence_type: ev.evidence_type,
                clickhouse_table: ev.clickhouse_table,
                record_id: ev.record_id,
                event_time: ev.event_time,
                metadata: ev.metadata || {},
            })));
        }

        // Add file upload evidence
        if (uploadedFiles.value.length > 0) {
            allEvidence.push(...uploadedFiles.value.map((file) => ({
                evidence_type: "file_upload",
                file_path: file.file_path,
                file_name: file.file_name,
                file_size: file.file_size,
                file_type: file.file_type,
            })));
        }

        // Add text paste evidence
        if (textEvidenceItems.value.length > 0) {
            allEvidence.push(...textEvidenceItems.value.map((text) => ({
                evidence_type: "text_paste",
                text_content: text.text_content,
            })));
        }

        // Check if ban originally had evidence
        const hadOriginalEvidence = props.ban.evidence && props.ban.evidence.length > 0;

        // If ban had evidence but now has none, send empty array to clear it
        // If ban had no evidence and still has none, don't send the field
        if (hadOriginalEvidence || allEvidence.length > 0) {
            requestBody.evidence = allEvidence;
        }

        // If no changes were made, just close the dialog
        if (Object.keys(requestBody).length === 0 && allEvidence.length === 0 && !hadOriginalEvidence) {
            open.value = false;
            return;
        }

        const { data, error: fetchError } = await useAuthFetch(
            `${runtimeConfig.public.backendApi}/servers/${props.serverId}/bans/${props.ban.id}`,
            {
                method: "PUT",
                body: requestBody,
            },
        );

        if (fetchError.value) {
            throw new Error(extractApiErrorMessage(fetchError.value, "Failed to update ban"));
        }

        // Close dialog (state is reset on next open)
        open.value = false;

        toast({
            title: "Success",
            description: "Ban updated successfully",
        });

        // Refresh the banned players list
        emit("saved");
    } catch (err: any) {
        error.value = extractApiErrorMessage(err, "An error occurred while updating the ban");
        console.error(err);
    } finally {
        saving.value = false;
    }
}
</script>

<template>
    <Dialog v-model:open="open">
        <DialogContent
            class="w-[95vw] sm:max-w-[700px] max-h-[90vh] overflow-y-auto p-4 sm:p-6"
        >
            <DialogHeader>
                <DialogTitle class="text-base sm:text-lg">
                    {{ mode === "edit" ? "Edit Ban" : "Add New Ban" }}
                </DialogTitle>
                <DialogDescription class="text-xs sm:text-sm">
                    <template v-if="mode === 'edit'">
                        Update the ban details for player
                        {{ ban?.steam_id }}.
                    </template>
                    <template v-else>
                        Enter the details of the player you want to ban. You
                        can optionally assign the ban to a shared ban list.
                    </template>
                </DialogDescription>
            </DialogHeader>

            <div v-if="error" class="bg-red-500 text-white p-3 rounded text-sm">
                {{ error }}
            </div>

            <form
                :key="`${mode}:${ban?.id ?? 'new'}:${formSession}`"
                @submit="onSubmit"
            >
                <div class="grid gap-4 py-4">
                    <!-- Player Search (create) -->
                    <FormField
                        v-if="mode === 'create'"
                        name="steam_id"
                        v-slot="{ componentField }"
                    >
                        <FormItem>
                            <FormLabel>Player</FormLabel>
                            <FormControl>
                                <PlayerSearchField
                                    v-bind="componentField"
                                    @suggestion="onSuggestion"
                                />
                            </FormControl>
                            <FormMessage />
                        </FormItem>
                    </FormField>

                    <!-- Display player information if available (edit) -->
                    <div
                        v-if="mode === 'edit' && ban?.player_name"
                        class="border rounded-md p-3 bg-muted/50"
                    >
                        <h4 class="font-medium mb-2">Player Information</h4>
                        <div class="text-sm">
                            <p>
                                <span class="text-muted-foreground">Name:</span>
                                {{ ban.player_name }}
                            </p>
                            <p v-if="ban.steam_id">
                                <span class="text-muted-foreground">Steam ID:</span>
                                {{ ban.steam_id }}
                            </p>
                            <p v-if="ban.eos_id">
                                <span class="text-muted-foreground">EOS ID:</span>
                                {{ ban.eos_id }}
                            </p>
                        </div>
                    </div>

                    <!-- Rule selector -->
                    <FormField name="rule_id" v-slot="{ componentField }">
                        <FormItem>
                            <FormLabel>
                                {{ mode === "edit" ? "Rule Violated (Optional)" : "Rule Violated" }}
                            </FormLabel>
                            <FormControl>
                                <Select v-bind="componentField">
                                    <SelectTrigger>
                                        <SelectValue
                                            placeholder="Select the rule that was violated"
                                        />
                                    </SelectTrigger>
                                    <SelectContent>
                                        <SelectItem value="__none__">
                                            No rule (custom reason)
                                        </SelectItem>
                                        <SelectItem
                                            v-for="rule in serverRules"
                                            :key="rule.id"
                                            :value="rule.id"
                                        >
                                            <template v-if="mode === 'create'">
                                                {{ rule.number }}: {{ rule.title }}
                                            </template>
                                            <template v-else>
                                                {{ rule.title }}
                                            </template>
                                        </SelectItem>
                                    </SelectContent>
                                </Select>
                            </FormControl>
                            <FormDescription>
                                <template v-if="mode === 'edit'">
                                    Select which server rule was violated
                                </template>
                                <template v-else>
                                    Select a rule to auto-generate the ban reason, or choose "No rule" for custom reason
                                </template>
                            </FormDescription>
                            <FormMessage />
                        </FormItem>
                    </FormField>

                    <!-- Reason (edit: required, prefilled) -->
                    <FormField
                        v-if="mode === 'edit'"
                        name="reason"
                        v-slot="{ componentField }"
                    >
                        <FormItem>
                            <FormLabel>Reason</FormLabel>
                            <FormControl>
                                <Textarea
                                    placeholder="Reason for ban"
                                    v-bind="componentField"
                                />
                            </FormControl>
                            <FormMessage />
                        </FormItem>
                    </FormField>

                    <FormField name="duration" v-slot="{ componentField }">
                        <FormItem>
                            <FormLabel>Duration</FormLabel>
                            <div
                                v-if="mode === 'create'"
                                class="flex items-center space-x-2"
                            >
                                <FormControl>
                                    <Input type="text" v-bind="componentField" />
                                </FormControl>
                                <Button
                                    v-if="historyCount > 0"
                                    type="button"
                                    variant="outline"
                                    size="sm"
                                    @click="applySuggestedDuration"
                                >
                                    Use Suggested
                                </Button>
                            </div>
                            <FormControl v-else>
                                <Input type="text" v-bind="componentField" />
                            </FormControl>
                            <FormDescription>
                                0 = permanent, 7d = 7 days, 2h = 2 hours, 30m = 30 minutes
                            </FormDescription>
                            <FormMessage />
                        </FormItem>
                    </FormField>

                    <!-- Ban Reason Preview (create, shown when rule is selected) -->
                    <div
                        v-if="mode === 'create' && formValues.rule_id && formValues.rule_id !== '__none__'"
                        class="border rounded-md p-3 bg-muted/50"
                    >
                        <h4 class="font-medium text-sm mb-2 flex items-center">
                            <Icon name="lucide:info" class="h-4 w-4 mr-1" />
                            Generated Ban Reason
                        </h4>
                        <p class="text-sm font-mono bg-background p-2 rounded border">
                            {{ generateBanReason(formValues.rule_id, formValues.duration as string) }}
                        </p>
                    </div>

                    <!-- Custom Reason (create, shown when no rule is selected) -->
                    <FormField
                        v-if="mode === 'create' && (!formValues.rule_id || formValues.rule_id === '__none__')"
                        name="reason"
                        v-slot="{ componentField }"
                    >
                        <FormItem>
                            <FormLabel>Custom Reason</FormLabel>
                            <FormControl>
                                <Textarea
                                    placeholder="Enter a custom ban reason..."
                                    v-bind="componentField"
                                />
                            </FormControl>
                            <FormDescription>
                                Required when no rule is selected
                            </FormDescription>
                            <FormMessage />
                        </FormItem>
                    </FormField>

                    <FormField name="ban_list_id" v-slot="{ componentField }">
                        <FormItem>
                            <FormLabel>Ban List (Optional)</FormLabel>
                            <FormControl>
                                <Select v-bind="componentField">
                                    <SelectTrigger>
                                        <SelectValue
                                            placeholder="Select a ban list (optional)"
                                        />
                                    </SelectTrigger>
                                    <SelectContent>
                                        <SelectItem
                                            v-for="banList in banLists.filter(
                                                (bl) => !bl.is_remote,
                                            )"
                                            :key="banList.id"
                                            :value="banList.id.toString()"
                                        >
                                            {{ banList.name }}
                                        </SelectItem>
                                    </SelectContent>
                                </Select>
                            </FormControl>
                            <FormDescription>
                                Select a ban list to add this ban to for
                                sharing across servers
                            </FormDescription>
                            <FormMessage />
                        </FormItem>
                    </FormField>

                    <FormField name="evidence_text" v-slot="{ componentField }">
                        <FormItem>
                            <FormLabel>Evidence Description (Optional)</FormLabel>
                            <FormControl>
                                <Textarea
                                    placeholder="Describe the evidence for this ban..."
                                    v-bind="componentField"
                                    rows="2"
                                />
                            </FormControl>
                            <FormDescription>
                                Provide additional context about the evidence
                            </FormDescription>
                            <FormMessage />
                        </FormItem>
                    </FormField>

                    <!-- Evidence Section with Tabs -->
                    <BanEvidenceEditor
                        :server-id="serverId"
                        :player-id="mode === 'create' ? formValues.steam_id : (ban?.steam_id || ban?.eos_id)"
                        :search-hint="mode === 'create' ? CREATE_SEARCH_HINT : undefined"
                        v-model:events="selectedEvidence"
                        v-model:files="uploadedFiles"
                        v-model:texts="textEvidenceItems"
                    />
                </div>
                <DialogFooter>
                    <Button
                        type="button"
                        variant="outline"
                        @click="open = false"
                    >
                        Cancel
                    </Button>
                    <Button type="submit" :disabled="saving">
                        {{
                            mode === "edit"
                                ? saving
                                    ? "Updating..."
                                    : "Update Ban"
                                : saving
                                  ? "Adding..."
                                  : "Add Ban"
                        }}
                    </Button>
                </DialogFooter>
            </form>
        </DialogContent>
    </Dialog>
</template>
