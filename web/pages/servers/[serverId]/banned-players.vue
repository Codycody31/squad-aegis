<script setup lang="ts">
import { ref, onMounted, onUnmounted, computed, nextTick, watch } from "vue";
import { Button } from "~/components/ui/button";
import { Input } from "~/components/ui/input";
import {
    Card,
    CardContent,
    CardHeader,
    CardTitle,
    CardDescription,
} from "~/components/ui/card";
import {
    Table,
    TableBody,
    TableCell,
    TableHead,
    TableHeader,
    TableRow,
} from "~/components/ui/table";
import { Badge } from "~/components/ui/badge";
import { Switch } from "~/components/ui/switch";
import { Label } from "~/components/ui/label";
import {
    Dialog,
    DialogContent,
    DialogDescription,
    DialogFooter,
    DialogHeader,
    DialogTitle,
    DialogTrigger,
} from "~/components/ui/dialog";
import {
    Form,
    FormControl,
    FormDescription,
    FormField,
    FormItem,
    FormLabel,
    FormMessage,
} from "~/components/ui/form";
import { Textarea } from "~/components/ui/textarea";
import {
    Select,
    SelectContent,
    SelectItem,
    SelectTrigger,
    SelectValue,
} from "~/components/ui/select";
import {
    Tabs,
    TabsList,
    TabsTrigger,
    TabsContent,
} from "~/components/ui/tabs";
import { Plus, Trash2 } from "lucide-vue-next";
import { useForm } from "vee-validate";
import { toTypedSchema } from "@vee-validate/zod";
import * as z from "zod";
import { toast } from "~/components/ui/toast";
import { canPreview, getMediaCategory } from "~/utils/mediaTypes";

const authStore = useAuthStore();
const runtimeConfig = useRuntimeConfig();
const route = useRoute();
const serverId = Array.isArray(route.params.serverId)
    ? route.params.serverId[0]
    : route.params.serverId;

const loading = ref(true);
const error = ref<string | null>(null);
const bannedPlayers = ref<BannedPlayer[]>([]);
const banLists = ref<any[]>([]);
const subscribedBanLists = ref<any[]>([]);
const availableBanLists = ref<any[]>([]);
const serverRules = ref<any[]>([]);
const playerHistory = ref<any[]>([]);
const suggestedDuration = ref<number>(24);
const isLoadingHistory = ref(false);
const searchQuery = ref("");
const hideExpiredBans = ref(false);
const showAddBanDialog = ref(false);
const showEditBanDialog = ref(false);
const showBanListDialog = ref(false);
const showEvidenceViewDialog = ref(false);
const viewingBanEvidence = ref<BannedPlayer | null>(null);
const addBanLoading = ref(false);
const editBanLoading = ref(false);
const selectedBanListId = ref("");
const subscribing = ref(false);
const unsubscribing = ref<string>("");
const editingBan = ref<BannedPlayer | null>(null);
const evidenceSearchQuery = ref("");
const evidenceSearchResults = ref<any[]>([]);
const selectedEvidence = ref<any[]>([]);
const isSearchingEvidence = ref(false);
const evidenceText = ref("");
const evidenceSearchType = ref("chat_message");
const evidenceSearchSteamId = ref("");
const uploadedFiles = ref<any[]>([]);
const textEvidenceItems = ref<any[]>([]);
const isUploadingFile = ref(false);
const evidenceTab = ref("events"); // 'events', 'files', 'text'
const fileInputRef = ref<HTMLInputElement | null>(null);
const fileInputRefEdit = ref<HTMLInputElement | null>(null);

// Player search state for add ban dialog
const playerSearchQuery = ref("");
const playerSearchResults = ref<any[]>([]);
const selectedPlayer = ref<any | null>(null);
const isSearchingPlayers = ref(false);
const showPlayerDropdown = ref(false);

// Media preview state
const showMediaPreviewModal = ref(false);
const previewingFile = ref<BanEvidence | null>(null);
const previewFileIndex = ref(0);

interface BanEvidence {
    id: string;
    evidence_type: string;
    clickhouse_table?: string | null;
    record_id?: string | null;
    event_time?: string | null;
    metadata?: any;
    // File upload evidence fields
    file_path?: string | null;
    file_name?: string | null;
    file_size?: number | null;
    file_type?: string | null;
    // Text paste evidence field
    text_content?: string | null;
}

interface BannedPlayer {
    id: string;
    server_id: string;
    admin_id: string;
    admin_name: string;
    steam_id: string;
    name: string;
    reason: string;
    duration: number;
    permanent: boolean;
    expires_at: string;
    created_at: string;
    updated_at: string;
    ban_list_id?: string;
    ban_list_name?: string;
    rule_id?: string;
    rule_name?: string;
    rule_number?: string;
    player_name?: string;
    evidence_text?: string;
    evidence?: BanEvidence[];
}

interface BannedPlayersResponse {
    data: {
        bans: BannedPlayer[];
    };
}

// Form schema for adding a ban
const formSchema = toTypedSchema(
    z.object({
        steam_id: z
            .string()
            .min(17, "Steam ID must be at least 17 characters")
            .max(17, "Steam ID must be exactly 17 characters")
            .regex(/^\d+$/, "Steam ID must contain only numbers"),
        reason: z.string().optional(), // Now optional - will be auto-generated when rule is selected
        duration: z.number().min(0, "Duration must be at least 0"),
        ban_list_id: z.string().optional(),
        rule_id: z.string().optional(),
        evidence_text: z.string().optional(),
    }),
);

// Form schema for editing a ban
const editFormSchema = toTypedSchema(
    z.object({
        reason: z.string().min(1, "Reason is required"),
        duration: z.number().min(0, "Duration must be at least 0"),
        ban_list_id: z.string().optional(),
        rule_id: z.string().optional(),
        evidence_text: z.string().optional(),
    }),
);

// Setup forms
const form = useForm({
    validationSchema: formSchema,
    initialValues: {
        steam_id: "",
        reason: "",
        duration: 1,
        ban_list_id: "",
        rule_id: "",
        evidence_text: "",
    },
});

// Helper function to get rule details by ID
function getSelectedRuleDetails(ruleId: string) {
    if (!ruleId) return null;
    return serverRules.value.find((r) => r.id === ruleId);
}

// Generate ban reason from rule and duration
function generateBanReason(ruleId: string | undefined, duration: number | undefined): string {
    if (!ruleId || ruleId === "__none__") return "";

    const rule = getSelectedRuleDetails(ruleId);
    if (!rule) return "";

    // Format: "rule_number | rule_title | duration"
    // Extract the last part of the title (after the last ">")
    const titleParts = rule.title.split(" > ");
    const shortTitle = titleParts[titleParts.length - 1];

    const durationValue = duration ?? 0;
    const durationText = durationValue === 0 ? "perm" : `${durationValue}d`;
    return `${rule.number} | ${shortTitle} | ${durationText}`;
}

// Search for players
async function searchPlayers(query: string) {
    if (!query || query.length < 2) {
        playerSearchResults.value = [];
        showPlayerDropdown.value = false;
        return;
    }

    isSearchingPlayers.value = true;

    try {
        const { data, error: fetchError } = await useAuthFetch(
            `${runtimeConfig.public.backendApi}/players?search=${encodeURIComponent(query)}&limit=10`,
            {
                method: "GET",
            },
        );

        if (fetchError.value) {
            throw new Error(fetchError.value.message || "Failed to search players");
        }

        if (data.value && (data.value as any).data) {
            playerSearchResults.value = (data.value as any).data.players || [];
            showPlayerDropdown.value = playerSearchResults.value.length > 0;
        }
    } catch (err: any) {
        console.error("Failed to search players:", err);
        playerSearchResults.value = [];
    } finally {
        isSearchingPlayers.value = false;
    }
}

// Select a player from search results
function selectPlayer(player: any) {
    selectedPlayer.value = player;
    playerSearchQuery.value = player.player_name || "";

    // Ensure steam_id is a clean string (API might return it with quotes or as number)
    const steamId = String(player.steam_id).replace(/"/g, "");
    form.setFieldValue("steam_id", steamId);
    showPlayerDropdown.value = false;

    // Fetch ban history for the selected player
    if (steamId) {
        fetchPlayerBanHistory(steamId);
    }
}

// Clear selected player
function clearSelectedPlayer() {
    selectedPlayer.value = null;
    playerSearchQuery.value = "";
    form.setFieldValue("steam_id", "");
    playerHistory.value = [];
}

// Debounced player search
let playerSearchTimeout: ReturnType<typeof setTimeout> | null = null;
function debouncedPlayerSearch(query: string) {
    if (playerSearchTimeout) {
        clearTimeout(playerSearchTimeout);
    }
    playerSearchTimeout = setTimeout(() => {
        searchPlayers(query);
    }, 300);
}

// Helper function to check if a ban is expired
function isBanExpired(ban: BannedPlayer): boolean {
    // Permanent bans never expire
    if (ban.permanent) {
        return false;
    }
    
    // Check if expires_at exists and is in the past
    if (ban.expires_at) {
        const expiresAt = new Date(ban.expires_at);
        const now = new Date();
        return expiresAt < now;
    }
    
    // If no expires_at and not permanent, consider it expired
    // (shouldn't happen in normal cases, but handle it)
    return false;
}

// Computed property for filtered banned players
const filteredBannedPlayers = computed(() => {
    let filtered = bannedPlayers.value;
    
    // Filter out expired bans if the switch is enabled
    if (hideExpiredBans.value) {
        filtered = filtered.filter((player) => !isBanExpired(player));
    }
    
    // Apply search query filter
    if (searchQuery.value.trim()) {
        const query = searchQuery.value.toLowerCase();
        filtered = filtered.filter(
            (player) =>
                player.name.toLowerCase().includes(query) ||
                player.steam_id.includes(query) ||
                player.reason.toLowerCase().includes(query),
        );
    }
    
    return filtered;
});

// Function to fetch banned players data
async function fetchBannedPlayers() {
    loading.value = true;
    error.value = null;

    const runtimeConfig = useRuntimeConfig();

    try {
        const { data, error: fetchError } =
            await useAuthFetch<BannedPlayersResponse>(
                `${runtimeConfig.public.backendApi}/servers/${serverId}/bans`,
                {
                    method: "GET",
                },
            );

        if (fetchError.value) {
            throw new Error(
                fetchError.value.message ||
                    "Failed to fetch banned players data",
            );
        }

        if (data.value && data.value.data) {
            bannedPlayers.value = data.value.data.bans || [];

            // Sort by ban date (most recent first)
            bannedPlayers.value.sort((a, b) => {
                return (
                    new Date(b.created_at).getTime() -
                    new Date(a.created_at).getTime()
                );
            });
        }
    } catch (err: any) {
        error.value =
            err.message ||
            "An error occurred while fetching banned players data";
        console.error(err);
    } finally {
        loading.value = false;
    }
}

// Function to add a ban
async function addBan(values: any) {
    const { steam_id, reason, duration, ban_list_id, rule_id, evidence_text } =
        values;

    addBanLoading.value = true;
    error.value = null;

    const runtimeConfig = useRuntimeConfig();

    try {
        // Clean and validate steam_id
        const cleanId = cleanSteamId(steam_id);

        // Check if a valid rule is selected (not __none__ sentinel value)
        const hasValidRule = rule_id && rule_id.trim() && rule_id !== "__none__";

        // Generate reason: if rule is selected, generate it dynamically, otherwise use provided reason
        let finalReason = reason || "";
        if (hasValidRule) {
            finalReason = generateBanReason(rule_id, duration ?? 0);
        }

        // Validate that we have a reason
        if (!finalReason) {
            throw new Error("Reason is required. Either select a rule or provide a custom reason.");
        }

        const requestBody: any = {
            steam_id: cleanId,
            reason: finalReason,
            duration,
        };

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
            `${runtimeConfig.public.backendApi}/servers/${serverId}/bans`,
            {
                method: "POST",
                body: requestBody,
            },
        );

        if (fetchError.value) {
            throw new Error(fetchError.value.message || "Failed to add ban");
        }

        // Reset form and close dialog
        form.resetForm();
        selectedEvidence.value = [];
        evidenceText.value = "";
        uploadedFiles.value = [];
        textEvidenceItems.value = [];
        evidenceSearchResults.value = [];
        evidenceTab.value = "events";
        showAddBanDialog.value = false;
        // Reset player search state
        selectedPlayer.value = null;
        playerSearchQuery.value = "";
        playerSearchResults.value = [];
        playerHistory.value = [];

        toast({
            title: "Success",
            description: "Ban added successfully",
        });

        // Refresh the banned players list
        fetchBannedPlayers();
    } catch (err: any) {
        error.value = err.message || "An error occurred while adding the ban";
        console.error(err);
    } finally {
        addBanLoading.value = false;
    }
}

// Function to remove a ban
async function removeBan(banId: string) {
    if (!confirm("Are you sure you want to remove this ban?")) {
        return;
    }

    loading.value = true;
    error.value = null;

    const runtimeConfig = useRuntimeConfig();

    try {
        const { data, error: fetchError } = await useAuthFetch(
            `${runtimeConfig.public.backendApi}/servers/${serverId}/bans/${banId}`,
            {
                method: "DELETE",
            },
        );

        if (fetchError.value) {
            throw new Error(fetchError.value.message || "Failed to remove ban");
        }

        // Refresh the banned players list
        fetchBannedPlayers();
    } catch (err: any) {
        error.value = err.message || "An error occurred while removing the ban";
        console.error(err);
    } finally {
        loading.value = false;
    }
}

// Function to edit a ban
async function editBan(values: any) {
    const { reason, duration, ban_list_id, rule_id, evidence_text } = values;

    if (!editingBan.value) {
        error.value = "No ban selected for editing";
        return;
    }

    editBanLoading.value = true;
    error.value = null;

    const runtimeConfig = useRuntimeConfig();

    try {
        const requestBody: any = {};

        // Only include fields that were actually changed
        if (reason !== editingBan.value.reason) {
            requestBody.reason = reason;
        }

        if (duration !== editingBan.value.duration) {
            requestBody.duration = duration;
        }

        // Handle ban list changes
        const currentBanListId = editingBan.value.ban_list_id || "";
        const newBanListId = ban_list_id || "";

        if (currentBanListId !== newBanListId) {
            requestBody.ban_list_id = newBanListId || null;
        }

        // Handle rule changes
        const currentRuleId = editingBan.value.rule_id || "";
        const newRuleId = rule_id || "";

        if (currentRuleId !== newRuleId) {
            requestBody.rule_id = newRuleId || null;
        }

        // Handle evidence text changes
        const currentEvidenceText = editingBan.value.evidence_text || "";
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
        const hadOriginalEvidence = editingBan.value.evidence && editingBan.value.evidence.length > 0;
        
        // If ban had evidence but now has none, send empty array to clear it
        // If ban had no evidence and still has none, don't send the field
        if (hadOriginalEvidence || allEvidence.length > 0) {
            requestBody.evidence = allEvidence;
        }

        // If no changes were made, just close the dialog
        if (Object.keys(requestBody).length === 0 && allEvidence.length === 0 && !hadOriginalEvidence) {
            closeEditBanDialog();
            return;
        }

        const { data, error: fetchError } = await useAuthFetch(
            `${runtimeConfig.public.backendApi}/servers/${serverId}/bans/${editingBan.value.id}`,
            {
                method: "PUT",
                body: requestBody,
            },
        );

        if (fetchError.value) {
            throw new Error(fetchError.value.message || "Failed to update ban");
        }

        // Reset form and close dialog
        closeEditBanDialog();

        toast({
            title: "Success",
            description: "Ban updated successfully",
        });

        // Refresh the banned players list
        fetchBannedPlayers();
    } catch (err: any) {
        error.value = err.message || "An error occurred while updating the ban";
        console.error(err);
    } finally {
        editBanLoading.value = false;
    }
}

// Function to get rule details by ID
function getRuleDetails(ruleId: string) {
    const rule = serverRules.value.find((r) => r.id === ruleId);
    return rule || null;
}

// Function to open edit ban dialog
async function openEditBanDialog(ban: BannedPlayer) {
    editingBan.value = ban;

    // Load existing evidence if present
    selectedEvidence.value = [];
    uploadedFiles.value = [];
    textEvidenceItems.value = [];
    
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

    // If the ban has a rule_id but no rule_name/number, try to fetch the details
    if (ban.rule_id && (!ban.rule_name || !ban.rule_number)) {
        const ruleDetails = getRuleDetails(ban.rule_id);
        if (ruleDetails) {
            // Update the ban object with rule details
            editingBan.value = {
                ...ban,
                rule_name: ruleDetails.title,
                rule_number: ruleDetails.number,
            };
        }
    }

    showEditBanDialog.value = true;
}

// Function to close edit ban dialog and reset state
function closeEditBanDialog() {
    showEditBanDialog.value = false;
    editingBan.value = null;
    selectedEvidence.value = [];
    uploadedFiles.value = [];
    textEvidenceItems.value = [];
    evidenceSearchResults.value = [];
    evidenceTab.value = "events";
}

// Function to open evidence view dialog
function openEvidenceViewDialog(ban: BannedPlayer) {
    viewingBanEvidence.value = ban;
    showEvidenceViewDialog.value = true;
}

// Function to close evidence view dialog
function closeEvidenceViewDialog() {
    showEvidenceViewDialog.value = false;
    viewingBanEvidence.value = null;
}

// Function to get evidence count by type
function getEvidenceCounts(ban: BannedPlayer | null) {
    if (!ban || !ban.evidence) {
        return { events: 0, files: 0, text: 0 };
    }
    const events = ban.evidence.filter((e) => 
        e.evidence_type !== "file_upload" && e.evidence_type !== "text_paste"
    ).length;
    const files = ban.evidence.filter((e) => e.evidence_type === "file_upload").length;
    const text = ban.evidence.filter((e) => e.evidence_type === "text_paste").length;
    return { events, files, text };
}

// Function to download evidence file
async function downloadEvidenceFile(filePath: string, fileName: string) {
    const runtimeConfig = useRuntimeConfig();
    const sessionCookie = useCookie(runtimeConfig.public.sessionCookieName as string);

    try {
        // Extract file ID from path (last part before extension)
        const fileId = filePath.split('/').pop()?.split('.')[0];
        if (!fileId) {
            throw new Error("Invalid file path");
        }

        const url = `${runtimeConfig.public.backendApi}/servers/${serverId}/evidence/files/${fileId}`;
        const response = await fetch(url, {
            headers: {
                Authorization: `Bearer ${sessionCookie.value}`,
            },
        });

        if (!response.ok) {
            throw new Error("Failed to download file");
        }

        const blob = await response.blob();
        const downloadUrl = window.URL.createObjectURL(blob);
        const link = document.createElement('a');
        link.href = downloadUrl;
        link.download = fileName;
        document.body.appendChild(link);
        link.click();
        document.body.removeChild(link);
        window.URL.revokeObjectURL(downloadUrl);
    } catch (err: any) {
        console.error("File download error:", err);
        toast({
            title: "Error",
            description: err.message || "Failed to download file",
            variant: "destructive",
        });
    }
}

// Function to get the previewable files from the current ban evidence
const previewableFiles = computed(() => {
    if (!viewingBanEvidence.value?.evidence) return [];
    return viewingBanEvidence.value.evidence.filter(
        (e) => e.evidence_type === "file_upload"
    );
});

// Function to get preview URL for a file
function getPreviewUrl(filePath: string): string {
    const fileId = filePath.split("/").pop()?.split(".")[0];
    return `${runtimeConfig.public.backendApi}/servers/${serverId}/evidence/files/${fileId}?inline=true`;
}

// Function to open media preview modal
function openMediaPreviewModal(evidence: BanEvidence, index: number) {
    previewingFile.value = evidence;
    previewFileIndex.value = index;
    showMediaPreviewModal.value = true;
}

// Function to close media preview modal
function closeMediaPreviewModal() {
    showMediaPreviewModal.value = false;
    previewingFile.value = null;
}

// Function to navigate between files in preview
function navigatePreview(direction: "prev" | "next") {
    const files = previewableFiles.value;
    if (direction === "next" && previewFileIndex.value < files.length - 1) {
        previewFileIndex.value++;
        previewingFile.value = files[previewFileIndex.value];
    } else if (direction === "prev" && previewFileIndex.value > 0) {
        previewFileIndex.value--;
        previewingFile.value = files[previewFileIndex.value];
    }
}

// Function to download the currently previewed file
function downloadPreviewedFile() {
    if (previewingFile.value?.file_path && previewingFile.value?.file_name) {
        downloadEvidenceFile(previewingFile.value.file_path, previewingFile.value.file_name);
    }
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

// Function to fetch server's ban list subscriptions
async function fetchServerBanListSubscriptions() {
    const runtimeConfig = useRuntimeConfig();

    try {
        const response = (await useAuthFetchImperative(
            `${runtimeConfig.public.backendApi}/servers/${serverId}/ban-list-subscriptions`,
        )) as any;

        if (response?.data?.subscriptions) {
            subscribedBanLists.value = response.data.subscriptions;
            // Calculate available ban lists (not subscribed)
            availableBanLists.value = banLists.value.filter(
                (banList) =>
                    !subscribedBanLists.value.some(
                        (sub) => sub.ban_list_id === banList.id,
                    ),
            );
        } else {
            subscribedBanLists.value = [];
            availableBanLists.value = banLists.value;
        }
    } catch (err) {
        console.error("Failed to fetch ban list subscriptions:", err);
    }
}

// Function to subscribe to a ban list
async function subscribeToBanList() {
    if (!selectedBanListId.value) return;

    const runtimeConfig = useRuntimeConfig();

    subscribing.value = true;

    try {
        await useAuthFetchImperative(
            `${runtimeConfig.public.backendApi}/servers/${serverId}/ban-list-subscriptions`,
            {
                method: "POST",
                body: {
                    ban_list_id: selectedBanListId.value,
                },
            },
        );

        selectedBanListId.value = "";
        await fetchServerBanListSubscriptions();
        await fetchBannedPlayers();

        toast({
            title: "Success",
            description:
                "Successfully subscribed to ban list. The server's ban configuration has been updated.",
        });
    } catch (err: any) {
        error.value = err.data?.message || "Failed to subscribe to ban list";
        console.error(err);
    } finally {
        subscribing.value = false;
    }
}

// Function to unsubscribe from a ban list
async function unsubscribeFromBanList(banListId: string) {
    if (!confirm("Are you sure you want to unsubscribe from this ban list?"))
        return;

    const runtimeConfig = useRuntimeConfig();

    unsubscribing.value = banListId;

    try {
        await useAuthFetchImperative(
            `${runtimeConfig.public.backendApi}/servers/${serverId}/ban-list-subscriptions/${banListId}`,
            {
                method: "DELETE",
            },
        );

        await fetchServerBanListSubscriptions();
        await fetchBannedPlayers();

        toast({
            title: "Success",
            description:
                "Ban list subscription removed. The server's ban configuration has been updated and will take effect on the next ban list refresh.",
        });
    } catch (err: any) {
        error.value =
            err.data?.message || "Failed to unsubscribe from ban list";
        console.error(err);
    } finally {
        unsubscribing.value = "";
    }
}

// Format date
function formatDate(dateString: string): string {
    return new Date(dateString).toLocaleString();
}

// Function to fetch server rules
async function fetchServerRules() {
    const runtimeConfig = useRuntimeConfig();

    try {
        const { data, error: fetchError } = await useAuthFetch(
            `${runtimeConfig.public.backendApi}/servers/${serverId}/rules`,
            {
                method: "GET",
            },
        );

        if (fetchError.value) {
            throw new Error(
                fetchError.value.message || "Failed to fetch server rules",
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

// Function to fetch player ban history
async function fetchPlayerBanHistory(steamId: string) {
    if (!steamId || steamId.length !== 17) {
        playerHistory.value = [];
        return;
    }

    isLoadingHistory.value = true;

    const runtimeConfig = useRuntimeConfig();

    try {
        const { data, error: fetchError } = await useAuthFetch<any>(
            `${runtimeConfig.public.backendApi}/players/${steamId}/ban-history`,
            {
                method: "GET",
            },
        );

        if (fetchError.value) {
            throw new Error(
                fetchError.value.message || "Failed to fetch player history",
            );
        }

        if (data.value && data.value.data) {
            playerHistory.value = data.value.data.history || [];
            // Calculate suggested ban duration based on history
            calculateSuggestedDuration();
        } else {
            playerHistory.value = [];
            suggestedDuration.value = 24; // Default for first offenders
        }
    } catch (err: any) {
        console.error("Failed to fetch player history:", err);
        playerHistory.value = [];
    } finally {
        isLoadingHistory.value = false;
    }
}

// Calculate suggested ban duration based on player history
function calculateSuggestedDuration() {
    if (playerHistory.value.length === 0) {
        // First offense - suggest 24 hours
        suggestedDuration.value = 1;
        return;
    }

    // Sort history by date (newest first)
    const sortedHistory = [...playerHistory.value].sort(
        (a, b) =>
            new Date(b.created_at).getTime() - new Date(a.created_at).getTime(),
    );

    // Count previous offenses
    const offenseCount = sortedHistory.length;

    // Progressive ban duration based on number of previous offenses
    if (offenseCount === 1) {
        suggestedDuration.value = 3; // 3 days for second offense
    } else if (offenseCount === 2) {
        suggestedDuration.value = 7; // 7 days for third offense
    } else if (offenseCount === 3) {
        suggestedDuration.value = 14; // 14 days for fourth offense
    } else {
        suggestedDuration.value = 0; // Permanent ban for repeat offenders
    }
}

// Function to open evidence dialog
// Helper function to clean and validate steam_id
function cleanSteamId(steamId: string | number | undefined | null): string {
    if (!steamId) {
        throw new Error("Steam ID is required");
    }
    // Remove any quotes, whitespace, and ensure it's a number string
    const cleaned = String(steamId).trim().replace(/['"]/g, '');
    
    // Validate it's a valid number
    if (!/^\d+$/.test(cleaned)) {
        throw new Error("Steam ID must be a valid number");
    }
    
    return cleaned;
}

// Function to search evidence inline (no longer opens a dialog)
async function searchEvidenceInline(steamId: string) {
    try {
        const cleanId = cleanSteamId(steamId);
        evidenceSearchSteamId.value = cleanId;
        isSearchingEvidence.value = true;
        evidenceSearchResults.value = [];

        const runtimeConfig = useRuntimeConfig();

        const params = new URLSearchParams({
            steam_id: cleanId,
            event_type: evidenceSearchType.value,
        });

        const { data, error: fetchError } = await useAuthFetch(
            `${runtimeConfig.public.backendApi}/servers/${serverId}/events/search?${params.toString()}`,
            {
                method: "GET",
            },
        );

        if (fetchError.value) {
            throw new Error(
                fetchError.value.message || "Failed to search for evidence",
            );
        }

        if (data.value && (data.value as any).data) {
            evidenceSearchResults.value = (data.value as any).data.events || [];
        }
    } catch (err: any) {
        console.error("Failed to search evidence:", err);
        toast({
            title: "Error",
            description: err.message || "Failed to search for evidence",
            variant: "destructive",
        });
    } finally {
        isSearchingEvidence.value = false;
    }
}

// Watch for evidence search type changes and clear results
watch(evidenceSearchType, () => {
    evidenceSearchResults.value = [];
});

// Function to toggle evidence selection
function toggleEvidenceSelection(event: any) {
    // Use record_id if available (from fixed API), otherwise fall back to event_id/message_id
    const recordId = event.record_id || event.event_id || event.message_id;
    
    const index = selectedEvidence.value.findIndex(
        (e) => e.record_id === recordId,
    );

    if (index > -1) {
        selectedEvidence.value.splice(index, 1);
    } else {
        selectedEvidence.value.push({
            evidence_type: evidenceSearchType.value,
            clickhouse_table: getClickhouseTableForType(evidenceSearchType.value),
            record_id: recordId,
            event_time: event.event_time || event.sent_at,
            metadata: event,
        });
    }
}

// Function to check if evidence is selected
function isEvidenceSelected(event: any): boolean {
    // Use record_id if available (from fixed API), otherwise fall back to event_id/message_id
    const recordId = event.record_id || event.event_id || event.message_id;
    return selectedEvidence.value.some((e) => e.record_id === recordId);
}

// Function to get ClickHouse table name for event type
function getClickhouseTableForType(type: string): string {
    const tableMap: Record<string, string> = {
        chat_message: "server_player_chat_messages",
        player_connected: "server_player_connected_events",
    };
    return tableMap[type] || "server_player_chat_messages";
}

// Function to format event for display
function formatEventDescription(event: any, type: string): string {
    if (type === "chat_message") {
        return event.message || "No message";
    } else if (type === "player_connected") {
        const joinedName = event.joined_name || event.player_controller || "Unknown Player";
        const ip = event.ip || "Unknown IP";
        return `${joinedName} @ ${ip}`;
    } else if (type === "file_upload") {
        return event.file_name || "Uploaded file";
    } else if (type === "text_paste") {
        const text = event.text_content || "";
        return text.length > 50 ? text.substring(0, 50) + "..." : text;
    }
    return "Event";
}

// Function to handle file upload
async function handleFileUpload(event: Event) {
    const target = event.target as HTMLInputElement;
    const files = target.files;
    if (!files || files.length === 0) return;

    isUploadingFile.value = true;
    const runtimeConfig = useRuntimeConfig();

    try {
        for (const file of Array.from(files)) {
            const formData = new FormData();
            formData.append("file", file);

            const { data, error: uploadError } = await useAuthFetch(
                `${runtimeConfig.public.backendApi}/servers/${serverId}/evidence/upload`,
                {
                    method: "POST",
                    body: formData,
                },
            );

            if (uploadError.value) {
                throw new Error(uploadError.value.message || "Failed to upload file");
            }

            if (data.value && (data.value as any).data) {
                const fileData = (data.value as any).data;
                uploadedFiles.value.push({
                    evidence_type: "file_upload",
                    file_path: fileData.file_path,
                    file_name: fileData.file_name,
                    file_size: fileData.file_size,
                    file_type: fileData.file_type,
                    file_id: fileData.file_id,
                });
            }
        }

        toast({
            title: "Success",
            description: `Uploaded ${files.length} file(s) successfully`,
        });
    } catch (err: any) {
        console.error("File upload error:", err);
        toast({
            title: "Error",
            description: err.message || "Failed to upload file",
            variant: "destructive",
        });
    } finally {
        isUploadingFile.value = false;
        // Reset file input safely using nextTick to avoid DOM issues
        // Wait for Vue to finish its update cycle before resetting
        nextTick(() => {
            try {
                if (target && target.value !== undefined) {
                    target.value = "";
                }
            } catch (resetErr) {
                // Ignore errors when resetting - element might have been removed or is no longer usable
                // This is safe to ignore as the input will reset naturally on next interaction
                console.debug("File input reset skipped (safe to ignore):", resetErr);
            }
        });
    }
}

// Function to remove uploaded file
function removeUploadedFile(index: number) {
    uploadedFiles.value.splice(index, 1);
}

// Function to add text evidence
function addTextEvidence() {
    if (!evidenceText.value.trim()) {
        toast({
            title: "Error",
            description: "Please enter some text",
            variant: "destructive",
        });
        return;
    }

    textEvidenceItems.value.push({
        evidence_type: "text_paste",
        text_content: evidenceText.value.trim(),
    });

    evidenceText.value = "";

    toast({
        title: "Success",
        description: "Text evidence added",
    });
}

// Function to remove text evidence
function removeTextEvidence(index: number) {
    textEvidenceItems.value.splice(index, 1);
}

// Function to format file size
function formatFileSize(bytes: number): string {
    if (bytes < 1024) return bytes + " B";
    if (bytes < 1024 * 1024) return (bytes / 1024).toFixed(2) + " KB";
    return (bytes / (1024 * 1024)).toFixed(2) + " MB";
}

// Setup auto-refresh
onMounted(async () => {
    await fetchBanLists();
    await fetchServerBanListSubscriptions();
    await fetchBannedPlayers();
    await fetchServerRules();
});

// Manual refresh function
async function refreshData() {
    await fetchBanLists();
    await fetchServerBanListSubscriptions();
    await fetchBannedPlayers();
    await fetchServerRules();
}

function copyBanCfgUrl() {
    var url = "";
    if (runtimeConfig.public.backendApi.startsWith("/")) {
        // Relative URL, construct full URL
        const origin = window.location.origin;
        url = `${origin}${runtimeConfig.public.backendApi}/servers/${serverId}/bans/cfg`;
    } else {
        url = `${runtimeConfig.public.backendApi}/servers/${serverId}/bans/cfg`;
    }
    navigator.clipboard.writeText(url);

    toast({
        title: "Success",
        description: "Ban configuration URL copied to clipboard",
    });
}
</script>

<template>
    <div class="p-3 sm:p-4">
        <div class="flex flex-col sm:flex-row sm:justify-between sm:items-center gap-3 sm:gap-0 mb-3 sm:mb-4">
            <h1 class="text-xl sm:text-2xl font-bold">Banned Players</h1>
            <div class="flex flex-col sm:flex-row gap-2 w-full sm:w-auto">
                <Form
                    v-slot="{ handleSubmit }"
                    as=""
                    keep-values
                    :validation-schema="formSchema"
                    :initial-values="{
                        steam_id: '',
                        reason: '',
                        duration: 1,
                        ban_list_id: '',
                        rule_id: '',
                    }"
                >
                    <Dialog v-model:open="showAddBanDialog" @update:open="(open) => { if (!open) { selectedEvidence = []; evidenceText = ''; } }">
                        <DialogTrigger asChild>
                            <Button
                                v-if="
                                    authStore.getServerPermission(
                                        serverId as string,
                                        'ban',
                                    )
                                "
                                class="w-full sm:w-auto text-sm sm:text-base"
                                >Add Ban Manually</Button
                            >
                        </DialogTrigger>
                        <DialogContent
                            class="w-[95vw] sm:max-w-[700px] max-h-[90vh] overflow-y-auto p-4 sm:p-6"
                        >
                            <DialogHeader>
                                <DialogTitle class="text-base sm:text-lg">Add New Ban</DialogTitle>
                                <DialogDescription class="text-xs sm:text-sm">
                                    Enter the details of the player you want to
                                    ban. You can optionally assign the ban to a
                                    shared ban list.
                                </DialogDescription>
                            </DialogHeader>
                            <form
                                id="dialogForm"
                                @submit="handleSubmit($event, addBan)"
                            >
                                <div class="grid gap-4 py-4">
                                    <!-- Player Search -->
                                    <FormField
                                        name="steam_id"
                                        v-slot="{ componentField }"
                                    >
                                        <FormItem>
                                            <FormLabel>Player</FormLabel>
                                            <FormControl>
                                                <div class="space-y-2">
                                                    <!-- Selected Player Display -->
                                                    <div v-if="selectedPlayer" class="flex items-center gap-2 p-2 border rounded-md bg-muted/50">
                                                        <div class="flex-1">
                                                            <p class="font-medium text-sm">{{ selectedPlayer.player_name }}</p>
                                                            <p class="text-xs text-muted-foreground">{{ selectedPlayer.steam_id }}</p>
                                                        </div>
                                                        <Button
                                                            type="button"
                                                            variant="ghost"
                                                            size="sm"
                                                            @click="clearSelectedPlayer"
                                                        >
                                                            <Icon name="lucide:x" class="h-4 w-4" />
                                                        </Button>
                                                    </div>

                                                    <!-- Search Input (shown when no player selected) -->
                                                    <div v-if="!selectedPlayer" class="relative">
                                                        <Input
                                                            v-model="playerSearchQuery"
                                                            placeholder="Search by player name..."
                                                            @input="(e: Event) => debouncedPlayerSearch((e.target as HTMLInputElement).value)"
                                                            @focus="showPlayerDropdown = playerSearchResults.length > 0"
                                                        />
                                                        <div v-if="isSearchingPlayers" class="absolute right-3 top-1/2 -translate-y-1/2">
                                                            <Icon name="mdi:loading" class="h-4 w-4 animate-spin" />
                                                        </div>

                                                        <!-- Search Results Dropdown -->
                                                        <div
                                                            v-if="showPlayerDropdown && playerSearchResults.length > 0"
                                                            class="absolute z-50 w-full mt-1 bg-popover border rounded-md shadow-md max-h-60 overflow-auto"
                                                        >
                                                            <div
                                                                v-for="player in playerSearchResults"
                                                                :key="player.steam_id"
                                                                class="p-2 hover:bg-muted cursor-pointer border-b last:border-b-0"
                                                                @click="selectPlayer(player)"
                                                            >
                                                                <p class="font-medium text-sm">{{ player.player_name }}</p>
                                                                <p class="text-xs text-muted-foreground">{{ player.steam_id }}</p>
                                                                <p class="text-xs text-muted-foreground">Last seen: {{ new Date(player.last_seen).toLocaleDateString() }}</p>
                                                            </div>
                                                        </div>
                                                    </div>

                                                    <!-- Steam ID Input - always present for form binding, hidden when player selected -->
                                                    <Input
                                                        v-bind="componentField"
                                                        :class="{ 'hidden': selectedPlayer }"
                                                        placeholder="Or enter Steam ID: 76561198012345678"
                                                        @input="(e: Event) => {
                                                            const target = e.target as HTMLInputElement;
                                                            if (target.value.length === 17) {
                                                                fetchPlayerBanHistory(target.value);
                                                            }
                                                        }"
                                                    />
                                                </div>
                                            </FormControl>
                                            <FormDescription v-if="!selectedPlayer">
                                                Search for a player by name, or enter their Steam ID directly
                                            </FormDescription>
                                            <FormMessage />
                                        </FormItem>
                                    </FormField>

                                    <!-- Player History Loading Indicator -->
                                    <div
                                        v-if="isLoadingHistory"
                                        class="border rounded-md p-3 bg-muted/50 flex items-center"
                                    >
                                        <div
                                            class="animate-spin h-5 w-5 border-2 border-primary border-t-transparent rounded-full mr-2"
                                        ></div>
                                        <span class="text-sm"
                                            >Looking up player history...</span
                                        >
                                    </div>

                                    <!-- Player Ban History Card -->
                                    <div
                                        v-else-if="playerHistory.length > 0"
                                        class="border rounded-md p-3 bg-muted/50"
                                    >
                                        <h4
                                            class="font-medium mb-2 flex items-center"
                                        >
                                            <span class="text-orange-500 mr-1"
                                                >⚠️</span
                                            >
                                            Previous Ban History
                                        </h4>
                                        <p
                                            class="text-sm text-muted-foreground mb-2"
                                        >
                                            This player has been banned
                                            {{ playerHistory.length }} time{{
                                                playerHistory.length > 1
                                                    ? "s"
                                                    : ""
                                            }}
                                            before.
                                        </p>
                                        <ul class="text-sm space-y-1 mb-2">
                                            <li
                                                v-for="(
                                                    ban, index
                                                ) in playerHistory.slice(0, 3)"
                                                :key="index"
                                            >
                                                <span
                                                    class="text-muted-foreground"
                                                    >{{
                                                        new Date(
                                                            ban.created_at,
                                                        ).toLocaleDateString()
                                                    }}:</span
                                                >
                                                {{ ban.reason }}
                                            </li>
                                        </ul>
                                        <div
                                            class="flex items-center border-t pt-2 mt-2"
                                        >
                                            <span class="mr-2 text-sm"
                                                >Suggested ban:</span
                                            >
                                            <Badge
                                                variant="destructive"
                                                v-if="suggestedDuration === 0"
                                            >
                                                Permanent
                                            </Badge>
                                            <Badge variant="default" v-else>
                                                {{ suggestedDuration }}
                                                {{
                                                    suggestedDuration === 1
                                                        ? "day"
                                                        : "days"
                                                }}
                                            </Badge>
                                        </div>
                                    </div>

                                    <FormField
                                        name="rule_id"
                                        v-slot="{ componentField }"
                                    >
                                        <FormItem>
                                            <FormLabel
                                                >Rule Violated</FormLabel
                                            >
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
                                                            {{ rule.number }}: {{ rule.title }}
                                                        </SelectItem>
                                                    </SelectContent>
                                                </Select>
                                            </FormControl>
                                            <FormDescription>
                                                Select a rule to auto-generate the ban reason, or choose "No rule" for custom reason
                                            </FormDescription>
                                            <FormMessage />
                                        </FormItem>
                                    </FormField>

                                    <FormField
                                        name="duration"
                                        v-slot="{ componentField, setValue }"
                                    >
                                        <FormItem>
                                            <FormLabel>Days</FormLabel>
                                            <div
                                                class="flex items-center space-x-2"
                                            >
                                                <FormControl>
                                                    <Input
                                                        type="number"
                                                        min="0"
                                                        v-bind="componentField"
                                                    />
                                                </FormControl>
                                                <Button
                                                    type="button"
                                                    variant="outline"
                                                    size="sm"
                                                    @click="
                                                        setValue(
                                                            suggestedDuration,
                                                        )
                                                    "
                                                    v-if="
                                                        playerHistory.length > 0
                                                    "
                                                >
                                                    Use Suggested
                                                </Button>
                                            </div>
                                            <FormDescription
                                                >Duration in days. 0 is
                                                permanent</FormDescription
                                            >
                                            <FormMessage />
                                        </FormItem>
                                    </FormField>

                                    <!-- Ban Reason Preview (shown when rule is selected) -->
                                    <div
                                        v-if="form.values.rule_id && form.values.rule_id !== '__none__'"
                                        class="border rounded-md p-3 bg-muted/50"
                                    >
                                        <h4 class="font-medium text-sm mb-2 flex items-center">
                                            <Icon name="lucide:info" class="h-4 w-4 mr-1" />
                                            Generated Ban Reason
                                        </h4>
                                        <p class="text-sm font-mono bg-background p-2 rounded border">
                                            {{ generateBanReason(form.values.rule_id, form.values.duration) }}
                                        </p>
                                    </div>

                                    <!-- Custom Reason (shown when no rule is selected) -->
                                    <FormField
                                        v-if="!form.values.rule_id || form.values.rule_id === '__none__'"
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

                                    <FormField
                                        name="ban_list_id"
                                        v-slot="{ componentField }"
                                    >
                                        <FormItem>
                                            <FormLabel
                                                >Ban List (Optional)</FormLabel
                                            >
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
                                                                (bl) =>
                                                                    !bl.is_remote,
                                                            )"
                                                            :key="banList.id"
                                                            :value="
                                                                banList.id.toString()
                                                            "
                                                        >
                                                            {{ banList.name }}
                                                        </SelectItem>
                                                    </SelectContent>
                                                </Select>
                                            </FormControl>
                                            <FormDescription>
                                                Select a ban list to add this
                                                ban to for sharing across
                                                servers
                                            </FormDescription>
                                            <FormMessage />
                                        </FormItem>
                                    </FormField>

                                    <FormField
                                        name="evidence_text"
                                        v-slot="{ componentField }"
                                    >
                                        <FormItem>
                                            <FormLabel
                                                >Evidence Description (Optional)</FormLabel
                                            >
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
                                    <div class="border rounded-md p-3 bg-muted/50">
                                        <h4 class="font-medium text-sm mb-3">Attached Evidence</h4>
                                        <Tabs v-model="evidenceTab" class="w-full">
                                            <TabsList class="grid w-full grid-cols-3">
                                                <TabsTrigger value="events">
                                                    Events
                                                    <Badge v-if="selectedEvidence.length > 0" variant="secondary" class="ml-2">
                                                        {{ selectedEvidence.length }}
                                                    </Badge>
                                                </TabsTrigger>
                                                <TabsTrigger value="files">
                                                    Files
                                                    <Badge v-if="uploadedFiles.length > 0" variant="secondary" class="ml-2">
                                                        {{ uploadedFiles.length }}
                                                    </Badge>
                                                </TabsTrigger>
                                                <TabsTrigger value="text">
                                                    Text
                                                    <Badge v-if="textEvidenceItems.length > 0" variant="secondary" class="ml-2">
                                                        {{ textEvidenceItems.length }}
                                                    </Badge>
                                                </TabsTrigger>
                                            </TabsList>

                                            <!-- Events Tab -->
                                            <TabsContent value="events" class="mt-4">
                                                <div class="space-y-3">
                                                    <!-- Search Controls -->
                                                    <div class="flex flex-col sm:flex-row gap-2">
                                                        <Select v-model="evidenceSearchType" class="w-full sm:w-[180px]">
                                                            <SelectTrigger class="text-xs sm:text-sm">
                                                                <SelectValue placeholder="Event Type" />
                                                            </SelectTrigger>
                                                            <SelectContent>
                                                                <SelectItem value="chat_message" class="text-xs sm:text-sm">Chat Messages</SelectItem>
                                                                <SelectItem value="player_connected" class="text-xs sm:text-sm">Connections</SelectItem>
                                                            </SelectContent>
                                                        </Select>
                                                        <Button
                                                            type="button"
                                                            @click="searchEvidenceInline(form.values.steam_id || '')"
                                                            :disabled="!form.values.steam_id || form.values.steam_id.length !== 17 || isSearchingEvidence"
                                                            class="flex-1 text-xs sm:text-sm"
                                                        >
                                                            <Icon v-if="isSearchingEvidence" name="mdi:loading" class="h-3 w-3 sm:h-4 sm:w-4 sm:mr-2 animate-spin" />
                                                            <Icon v-else name="lucide:search" class="h-3 w-3 sm:h-4 sm:w-4 sm:mr-2" />
                                                            <span class="hidden sm:inline">{{ isSearchingEvidence ? "Searching..." : "Search Events" }}</span>
                                                            <span class="sm:hidden">{{ isSearchingEvidence ? "Searching..." : "Search" }}</span>
                                                        </Button>
                                                    </div>

                                                    <!-- Search Results -->
                                                    <div v-if="evidenceSearchResults.length > 0" class="border rounded-md max-h-[300px] overflow-y-auto">
                                                        <div class="divide-y">
                                                            <div
                                                                v-for="event in evidenceSearchResults"
                                                                :key="event.record_id || event.event_id || event.message_id"
                                                                class="p-3 hover:bg-muted/50 cursor-pointer transition-colors"
                                                                :class="{ 'bg-primary/10': isEvidenceSelected(event) }"
                                                                @click="toggleEvidenceSelection(event)"
                                                            >
                                                                <div class="flex items-start justify-between">
                                                                    <div class="flex-1">
                                                                        <div class="font-medium text-sm">
                                                                            {{ formatEventDescription(event, evidenceSearchType) }}
                                                                        </div>
                                                                        <div class="text-xs text-muted-foreground mt-1">
                                                                            {{ new Date(event.event_time || event.sent_at).toLocaleString() }}
                                                                        </div>
                                                                        <div v-if="event.teamkill" class="mt-1">
                                                                            <Badge variant="destructive" class="text-xs">TEAMKILL</Badge>
                                                                        </div>
                                                                    </div>
                                                                    <div v-if="isEvidenceSelected(event)" class="ml-2">
                                                                        <Icon name="lucide:check-circle" class="h-5 w-5 text-primary" />
                                                                    </div>
                                                                </div>
                                                            </div>
                                                        </div>
                                                    </div>

                                                    <!-- Selected Events -->
                                                    <div v-if="selectedEvidence.length > 0" class="space-y-2">
                                                        <div class="text-sm font-medium">Selected Events ({{ selectedEvidence.length }})</div>
                                                        <div class="space-y-2">
                                                            <div
                                                                v-for="(evidence, idx) in selectedEvidence"
                                                                :key="`event-${idx}`"
                                                                class="flex items-center justify-between text-sm p-2 bg-background rounded border"
                                                            >
                                                                <div class="flex-1">
                                                                    <div class="font-medium">{{ formatEventDescription(evidence.metadata, evidence.evidence_type) }}</div>
                                                                    <div class="text-xs text-muted-foreground">
                                                                        {{ new Date(evidence.event_time).toLocaleString() }}
                                                                    </div>
                                                                </div>
                                                                <Button
                                                                    type="button"
                                                                    variant="ghost"
                                                                    size="sm"
                                                                    @click="selectedEvidence.splice(idx, 1)"
                                                                >
                                                                    <Icon name="lucide:x" class="h-4 w-4" />
                                                                </Button>
                                                            </div>
                                                        </div>
                                                    </div>

                                                    <!-- Empty State -->
                                                    <div v-if="evidenceSearchResults.length === 0 && selectedEvidence.length === 0 && !isSearchingEvidence" class="text-sm text-muted-foreground text-center py-4">
                                                        Select a player above and click "Search Events" to find chat messages or connections
                                                    </div>
                                                </div>
                                            </TabsContent>

                                            <!-- Files Tab -->
                                            <TabsContent value="files" class="mt-4">
                                                <div class="space-y-3">
                                                    <div class="flex items-center gap-2">
                                                        <Input
                                                            type="file"
                                                            accept="image/*,video/*,.pdf,.txt"
                                                            @change="handleFileUpload"
                                                            :disabled="isUploadingFile"
                                                            class="flex-1"
                                                            multiple
                                                        />
                                                    </div>
                                                    <div v-if="uploadedFiles.length > 0" class="space-y-2">
                                                        <div class="text-sm font-medium">Uploaded Files ({{ uploadedFiles.length }})</div>
                                                        <div class="space-y-1">
                                                            <div
                                                                v-for="(file, idx) in uploadedFiles"
                                                                :key="`file-${idx}`"
                                                                class="flex items-center justify-between text-sm p-2 bg-background rounded border"
                                                            >
                                                                <div class="flex-1">
                                                                    <div class="font-medium">{{ file.file_name }}</div>
                                                                    <div class="text-xs text-muted-foreground">
                                                                        {{ formatFileSize(file.file_size) }} • {{ file.file_type }}
                                                                    </div>
                                                                </div>
                                                                <Button
                                                                    type="button"
                                                                    variant="ghost"
                                                                    size="sm"
                                                                    @click="removeUploadedFile(idx)"
                                                                >
                                                                    <Icon name="lucide:x" class="h-4 w-4" />
                                                                </Button>
                                                            </div>
                                                        </div>
                                                    </div>
                                                    <div v-else class="text-sm text-muted-foreground text-center py-4">
                                                        No files uploaded. Select files above to upload.
                                                    </div>
                                                </div>
                                            </TabsContent>

                                            <!-- Text Tab -->
                                            <TabsContent value="text" class="mt-4">
                                                <div class="space-y-3">
                                                    <div class="flex items-center gap-2">
                                                        <Textarea
                                                            v-model="evidenceText"
                                                            placeholder="Paste text evidence here..."
                                                            rows="4"
                                                            class="flex-1"
                                                        />
                                                        <Button
                                                            type="button"
                                                            variant="outline"
                                                            @click="addTextEvidence"
                                                            :disabled="!evidenceText.trim()"
                                                        >
                                                            <Icon name="lucide:plus" class="h-4 w-4 mr-1" />
                                                            Add
                                                        </Button>
                                                    </div>
                                                    <div v-if="textEvidenceItems.length > 0" class="space-y-2">
                                                        <div class="text-sm font-medium">Text Evidence ({{ textEvidenceItems.length }})</div>
                                                        <div class="space-y-1">
                                                            <div
                                                                v-for="(text, idx) in textEvidenceItems"
                                                                :key="`text-${idx}`"
                                                                class="flex items-start justify-between text-sm p-2 bg-background rounded border"
                                                            >
                                                                <div class="flex-1">
                                                                    <div class="font-medium">Text Evidence</div>
                                                                    <div class="text-xs text-muted-foreground mt-1">
                                                                        {{ text.text_content.length > 100 ? text.text_content.substring(0, 100) + "..." : text.text_content }}
                                                                    </div>
                                                                </div>
                                                                <Button
                                                                    type="button"
                                                                    variant="ghost"
                                                                    size="sm"
                                                                    @click="removeTextEvidence(idx)"
                                                                >
                                                                    <Icon name="lucide:x" class="h-4 w-4" />
                                                                </Button>
                                                            </div>
                                                        </div>
                                                    </div>
                                                    <div v-else class="text-sm text-muted-foreground text-center py-4">
                                                        No text evidence added. Paste text above and click "Add".
                                                    </div>
                                                </div>
                                            </TabsContent>
                                        </Tabs>
                                    </div>
                                </div>
                                <DialogFooter>
                                    <Button
                                        type="button"
                                        variant="outline"
                                        @click="showAddBanDialog = false"
                                    >
                                        Cancel
                                    </Button>
                                    <Button
                                        type="submit"
                                        :disabled="addBanLoading"
                                    >
                                        {{
                                            addBanLoading
                                                ? "Adding..."
                                                : "Add Ban"
                                        }}
                                    </Button>
                                </DialogFooter>
                            </form>
                        </DialogContent>
                    </Dialog>
                </Form>

                <!-- Edit Ban Dialog -->
                <Form
                    :key="editingBan?.id"
                    v-slot="{ handleSubmit }"
                    as=""
                    keep-values
                    :validation-schema="editFormSchema"
                    :initial-values="{
                        reason: editingBan?.reason || '',
                        duration: editingBan?.duration,
                        ban_list_id: editingBan?.ban_list_id || '',
                        rule_id: editingBan?.rule_id || '',
                        evidence_text: editingBan?.evidence_text || '',
                    }"
                >
                    <Dialog v-model:open="showEditBanDialog">
                        <DialogContent
                            class="w-[95vw] sm:max-w-[700px] max-h-[90vh] overflow-y-auto p-4 sm:p-6"
                        >
                            <DialogHeader>
                                <DialogTitle class="text-base sm:text-lg">Edit Ban</DialogTitle>
                                <DialogDescription class="text-xs sm:text-sm">
                                    Update the ban details for player
                                    {{ editingBan?.steam_id }}.
                                </DialogDescription>
                            </DialogHeader>
                            <form
                                id="editDialogForm"
                                @submit="handleSubmit($event, editBan)"
                            >
                                <div class="grid gap-4 py-4">
                                    <!-- Display player information if available -->
                                    <div
                                        v-if="editingBan?.player_name"
                                        class="border rounded-md p-3 bg-muted/50"
                                    >
                                        <h4 class="font-medium mb-2">
                                            Player Information
                                        </h4>
                                        <div class="text-sm">
                                            <p>
                                                <span
                                                    class="text-muted-foreground"
                                                    >Name:</span
                                                >
                                                {{ editingBan.player_name }}
                                            </p>
                                            <p>
                                                <span
                                                    class="text-muted-foreground"
                                                    >Steam ID:</span
                                                >
                                                {{ editingBan.steam_id }}
                                            </p>
                                        </div>
                                    </div>

                                    <!-- Rule selector -->
                                    <FormField
                                        name="rule_id"
                                        v-slot="{ componentField }"
                                    >
                                        <FormItem>
                                            <FormLabel
                                                >Rule Violated
                                                (Optional)</FormLabel
                                            >
                                            <FormControl>
                                                <Select v-bind="componentField">
                                                    <SelectTrigger>
                                                        <SelectValue
                                                            placeholder="Select the rule that was violated"
                                                        />
                                                    </SelectTrigger>
                                                    <SelectContent>
                                                        <SelectItem
                                                            v-for="rule in serverRules"
                                                            :key="rule.id"
                                                            :value="rule.id"
                                                        >
                                                            {{ rule.title }}
                                                        </SelectItem>
                                                    </SelectContent>
                                                </Select>
                                            </FormControl>
                                            <FormDescription>
                                                Select which server rule was
                                                violated
                                            </FormDescription>
                                            <FormMessage />
                                        </FormItem>
                                    </FormField>

                                    <FormField
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
                                    <FormField
                                        name="duration"
                                        v-slot="{ componentField }"
                                    >
                                        <FormItem>
                                            <FormLabel>Days</FormLabel>
                                            <FormControl>
                                                <Input
                                                    type="number"
                                                    min="0"
                                                    v-bind="componentField"
                                                />
                                            </FormControl>
                                            <FormDescription>
                                                Duration in days. 0 is permanent
                                            </FormDescription>
                                            <FormMessage />
                                        </FormItem>
                                    </FormField>
                                    <FormField
                                        name="ban_list_id"
                                        v-slot="{ componentField }"
                                    >
                                        <FormItem>
                                            <FormLabel
                                                >Ban List (Optional)</FormLabel
                                            >
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
                                                                (bl) =>
                                                                    !bl.is_remote,
                                                            )"
                                                            :key="banList.id"
                                                            :value="
                                                                banList.id.toString()
                                                            "
                                                        >
                                                            {{ banList.name }}
                                                        </SelectItem>
                                                    </SelectContent>
                                                </Select>
                                            </FormControl>
                                            <FormDescription>
                                                Select a ban list to add this
                                                ban to for sharing across
                                                servers
                                            </FormDescription>
                                            <FormMessage />
                                        </FormItem>
                                    </FormField>

                                    <FormField
                                        name="evidence_text"
                                        v-slot="{ componentField }"
                                    >
                                        <FormItem>
                                            <FormLabel
                                                >Evidence Description (Optional)</FormLabel
                                            >
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
                                    <div class="border rounded-md p-3 bg-muted/50">
                                        <h4 class="font-medium text-sm mb-3">Attached Evidence</h4>
                                        <Tabs v-model="evidenceTab" class="w-full">
                                            <TabsList class="grid w-full grid-cols-3">
                                                <TabsTrigger value="events">
                                                    Events
                                                    <Badge v-if="selectedEvidence.length > 0" variant="secondary" class="ml-2">
                                                        {{ selectedEvidence.length }}
                                                    </Badge>
                                                </TabsTrigger>
                                                <TabsTrigger value="files">
                                                    Files
                                                    <Badge v-if="uploadedFiles.length > 0" variant="secondary" class="ml-2">
                                                        {{ uploadedFiles.length }}
                                                    </Badge>
                                                </TabsTrigger>
                                                <TabsTrigger value="text">
                                                    Text
                                                    <Badge v-if="textEvidenceItems.length > 0" variant="secondary" class="ml-2">
                                                        {{ textEvidenceItems.length }}
                                                    </Badge>
                                                </TabsTrigger>
                                            </TabsList>

                                            <!-- Events Tab -->
                                            <TabsContent value="events" class="mt-4">
                                                <div class="space-y-3">
                                                    <!-- Search Controls -->
                                                    <div class="flex flex-col sm:flex-row gap-2">
                                                        <Select v-model="evidenceSearchType" class="w-full sm:w-[180px]">
                                                            <SelectTrigger class="text-xs sm:text-sm">
                                                                <SelectValue placeholder="Event Type" />
                                                            </SelectTrigger>
                                                            <SelectContent>
                                                                <SelectItem value="chat_message" class="text-xs sm:text-sm">Chat Messages</SelectItem>
                                                                <SelectItem value="player_connected" class="text-xs sm:text-sm">Connections</SelectItem>
                                                            </SelectContent>
                                                        </Select>
                                                        <Button
                                                            type="button"
                                                            @click="searchEvidenceInline(editingBan?.steam_id || '')"
                                                            :disabled="!editingBan?.steam_id || isSearchingEvidence"
                                                            class="flex-1 text-xs sm:text-sm"
                                                        >
                                                            <Icon v-if="isSearchingEvidence" name="mdi:loading" class="h-3 w-3 sm:h-4 sm:w-4 sm:mr-2 animate-spin" />
                                                            <Icon v-else name="lucide:search" class="h-3 w-3 sm:h-4 sm:w-4 sm:mr-2" />
                                                            <span class="hidden sm:inline">{{ isSearchingEvidence ? "Searching..." : "Search Events" }}</span>
                                                            <span class="sm:hidden">{{ isSearchingEvidence ? "Searching..." : "Search" }}</span>
                                                        </Button>
                                                    </div>

                                                    <!-- Search Results -->
                                                    <div v-if="evidenceSearchResults.length > 0" class="border rounded-md max-h-[300px] overflow-y-auto">
                                                        <div class="divide-y">
                                                            <div
                                                                v-for="event in evidenceSearchResults"
                                                                :key="event.record_id || event.event_id || event.message_id"
                                                                class="p-3 hover:bg-muted/50 cursor-pointer transition-colors"
                                                                :class="{ 'bg-primary/10': isEvidenceSelected(event) }"
                                                                @click="toggleEvidenceSelection(event)"
                                                            >
                                                                <div class="flex items-start justify-between">
                                                                    <div class="flex-1">
                                                                        <div class="font-medium text-sm">
                                                                            {{ formatEventDescription(event, evidenceSearchType) }}
                                                                        </div>
                                                                        <div class="text-xs text-muted-foreground mt-1">
                                                                            {{ new Date(event.event_time || event.sent_at).toLocaleString() }}
                                                                        </div>
                                                                        <div v-if="event.teamkill" class="mt-1">
                                                                            <Badge variant="destructive" class="text-xs">TEAMKILL</Badge>
                                                                        </div>
                                                                    </div>
                                                                    <div v-if="isEvidenceSelected(event)" class="ml-2">
                                                                        <Icon name="lucide:check-circle" class="h-5 w-5 text-primary" />
                                                                    </div>
                                                                </div>
                                                            </div>
                                                        </div>
                                                    </div>

                                                    <!-- Selected Events -->
                                                    <div v-if="selectedEvidence.length > 0" class="space-y-2">
                                                        <div class="text-sm font-medium">Selected Events ({{ selectedEvidence.length }})</div>
                                                        <div class="space-y-2">
                                                            <div
                                                                v-for="(evidence, idx) in selectedEvidence"
                                                                :key="`event-${idx}`"
                                                                class="flex items-center justify-between text-sm p-2 bg-background rounded border"
                                                            >
                                                                <div class="flex-1">
                                                                    <div class="font-medium">{{ formatEventDescription(evidence.metadata, evidence.evidence_type) }}</div>
                                                                    <div class="text-xs text-muted-foreground">
                                                                        {{ new Date(evidence.event_time).toLocaleString() }}
                                                                    </div>
                                                                </div>
                                                                <Button
                                                                    type="button"
                                                                    variant="ghost"
                                                                    size="sm"
                                                                    @click="selectedEvidence.splice(idx, 1)"
                                                                >
                                                                    <Icon name="lucide:x" class="h-4 w-4" />
                                                                </Button>
                                                            </div>
                                                        </div>
                                                    </div>

                                                    <!-- Empty State -->
                                                    <div v-if="evidenceSearchResults.length === 0 && selectedEvidence.length === 0 && !isSearchingEvidence" class="text-sm text-muted-foreground text-center py-4">
                                                        Click "Search Events" to find game events for this player
                                                    </div>
                                                </div>
                                            </TabsContent>

                                            <!-- Files Tab -->
                                            <TabsContent value="files" class="mt-4">
                                                <div class="space-y-3">
                                                    <div class="flex items-center gap-2">
                                                        <Input
                                                            type="file"
                                                            accept="image/*,video/*,.pdf,.txt"
                                                            @change="handleFileUpload"
                                                            :disabled="isUploadingFile"
                                                            class="flex-1"
                                                            multiple
                                                        />
                                                    </div>
                                                    <div v-if="uploadedFiles.length > 0" class="space-y-2">
                                                        <div class="text-sm font-medium">Uploaded Files ({{ uploadedFiles.length }})</div>
                                                        <div class="space-y-1">
                                                            <div
                                                                v-for="(file, idx) in uploadedFiles"
                                                                :key="`file-${idx}`"
                                                                class="flex items-center justify-between text-sm p-2 bg-background rounded border"
                                                            >
                                                                <div class="flex-1">
                                                                    <div class="font-medium">{{ file.file_name }}</div>
                                                                    <div class="text-xs text-muted-foreground">
                                                                        {{ formatFileSize(file.file_size) }} • {{ file.file_type }}
                                                                    </div>
                                                                </div>
                                                                <Button
                                                                    type="button"
                                                                    variant="ghost"
                                                                    size="sm"
                                                                    @click="removeUploadedFile(idx)"
                                                                >
                                                                    <Icon name="lucide:x" class="h-4 w-4" />
                                                                </Button>
                                                            </div>
                                                        </div>
                                                    </div>
                                                    <div v-else class="text-sm text-muted-foreground text-center py-4">
                                                        No files uploaded. Select files above to upload.
                                                    </div>
                                                </div>
                                            </TabsContent>

                                            <!-- Text Tab -->
                                            <TabsContent value="text" class="mt-4">
                                                <div class="space-y-3">
                                                    <div class="flex items-center gap-2">
                                                        <Textarea
                                                            v-model="evidenceText"
                                                            placeholder="Paste text evidence here..."
                                                            rows="4"
                                                            class="flex-1"
                                                        />
                                                        <Button
                                                            type="button"
                                                            variant="outline"
                                                            @click="addTextEvidence"
                                                            :disabled="!evidenceText.trim()"
                                                        >
                                                            <Icon name="lucide:plus" class="h-4 w-4 mr-1" />
                                                            Add
                                                        </Button>
                                                    </div>
                                                    <div v-if="textEvidenceItems.length > 0" class="space-y-2">
                                                        <div class="text-sm font-medium">Text Evidence ({{ textEvidenceItems.length }})</div>
                                                        <div class="space-y-1">
                                                            <div
                                                                v-for="(text, idx) in textEvidenceItems"
                                                                :key="`text-${idx}`"
                                                                class="flex items-start justify-between text-sm p-2 bg-background rounded border"
                                                            >
                                                                <div class="flex-1">
                                                                    <div class="font-medium">Text Evidence</div>
                                                                    <div class="text-xs text-muted-foreground mt-1">
                                                                        {{ text.text_content.length > 100 ? text.text_content.substring(0, 100) + "..." : text.text_content }}
                                                                    </div>
                                                                </div>
                                                                <Button
                                                                    type="button"
                                                                    variant="ghost"
                                                                    size="sm"
                                                                    @click="removeTextEvidence(idx)"
                                                                >
                                                                    <Icon name="lucide:x" class="h-4 w-4" />
                                                                </Button>
                                                            </div>
                                                        </div>
                                                    </div>
                                                    <div v-else class="text-sm text-muted-foreground text-center py-4">
                                                        No text evidence added. Paste text above and click "Add".
                                                    </div>
                                                </div>
                                            </TabsContent>
                                        </Tabs>
                                    </div>
                                </div>
                                <DialogFooter>
                                    <Button
                                        type="button"
                                        variant="outline"
                                        @click="closeEditBanDialog"
                                    >
                                        Cancel
                                    </Button>
                                    <Button
                                        type="submit"
                                        :disabled="editBanLoading"
                                    >
                                        {{
                                            editBanLoading
                                                ? "Updating..."
                                                : "Update Ban"
                                        }}
                                    </Button>
                                </DialogFooter>
                            </form>
                        </DialogContent>
                    </Dialog>
                </Form>

                <!-- Evidence View Dialog -->
                <Dialog v-model:open="showEvidenceViewDialog">
                    <DialogContent class="w-[95vw] sm:max-w-[900px] max-h-[90vh] overflow-y-auto p-4 sm:p-6">
                        <DialogHeader>
                            <DialogTitle class="text-base sm:text-lg">Ban Evidence</DialogTitle>
                            <DialogDescription class="text-xs sm:text-sm">
                                View all evidence linked to this ban
                            </DialogDescription>
                        </DialogHeader>
                        <div v-if="viewingBanEvidence" class="space-y-4">
                            <!-- Ban Info -->
                            <div class="border rounded-md p-3 bg-muted/50">
                                <div class="grid grid-cols-2 gap-2 text-sm">
                                    <div>
                                        <span class="text-muted-foreground">Steam ID:</span>
                                        <span class="ml-2 font-medium">{{ viewingBanEvidence.steam_id }}</span>
                                    </div>
                                    <div>
                                        <span class="text-muted-foreground">Reason:</span>
                                        <span class="ml-2 font-medium">{{ viewingBanEvidence.reason }}</span>
                                    </div>
                                    <div>
                                        <span class="text-muted-foreground">Banned At:</span>
                                        <span class="ml-2">{{ formatDate(viewingBanEvidence.created_at) }}</span>
                                    </div>
                                    <div>
                                        <span class="text-muted-foreground">Duration:</span>
                                        <span class="ml-2">
                                            {{ viewingBanEvidence.duration === 0 ? "Permanent" : `${viewingBanEvidence.duration} days` }}
                                        </span>
                                    </div>
                                </div>
                            </div>

                            <!-- Evidence Description -->
                            <div v-if="viewingBanEvidence.evidence_text" class="border rounded-md p-3">
                                <h4 class="font-medium text-sm mb-2">Evidence Description</h4>
                                <p class="text-sm text-muted-foreground whitespace-pre-wrap">{{ viewingBanEvidence.evidence_text }}</p>
                            </div>

                            <!-- Evidence Tabs -->
                            <Tabs value="events" class="w-full">
                                <TabsList class="grid w-full grid-cols-3">
                                    <TabsTrigger value="events" class="text-xs sm:text-sm">
                                        <span class="hidden sm:inline">Events</span>
                                        <span class="sm:hidden">Events</span>
                                        <Badge v-if="getEvidenceCounts(viewingBanEvidence).events > 0" variant="secondary" class="ml-1 sm:ml-2 text-xs">
                                            {{ getEvidenceCounts(viewingBanEvidence).events }}
                                        </Badge>
                                    </TabsTrigger>
                                    <TabsTrigger value="files" class="text-xs sm:text-sm">
                                        <span class="hidden sm:inline">Files</span>
                                        <span class="sm:hidden">Files</span>
                                        <Badge v-if="getEvidenceCounts(viewingBanEvidence).files > 0" variant="secondary" class="ml-1 sm:ml-2 text-xs">
                                            {{ getEvidenceCounts(viewingBanEvidence).files }}
                                        </Badge>
                                    </TabsTrigger>
                                    <TabsTrigger value="text" class="text-xs sm:text-sm">
                                        <span class="hidden sm:inline">Text</span>
                                        <span class="sm:hidden">Text</span>
                                        <Badge v-if="getEvidenceCounts(viewingBanEvidence).text > 0" variant="secondary" class="ml-1 sm:ml-2 text-xs">
                                            {{ getEvidenceCounts(viewingBanEvidence).text }}
                                        </Badge>
                                    </TabsTrigger>
                                </TabsList>

                                <!-- Events Tab -->
                                <TabsContent value="events" class="mt-4">
                                    <div v-if="viewingBanEvidence.evidence && viewingBanEvidence.evidence.filter(e => e.evidence_type !== 'file_upload' && e.evidence_type !== 'text_paste').length > 0" class="space-y-2">
                                        <div
                                            v-for="(evidence, idx) in viewingBanEvidence.evidence.filter(e => e.evidence_type !== 'file_upload' && e.evidence_type !== 'text_paste')"
                                            :key="`event-${idx}`"
                                            class="border rounded-md p-3"
                                        >
                                            <div class="flex items-start justify-between">
                                                <div class="flex-1">
                                                    <div class="font-medium text-sm mb-1">
                                                        {{ formatEventDescription(evidence.metadata || {}, evidence.evidence_type) }}
                                                    </div>
                                                    <div class="text-xs text-muted-foreground space-y-1">
                                                        <div>Type: {{ evidence.evidence_type }}</div>
                                                        <div v-if="evidence.event_time">
                                                            Time: {{ new Date(evidence.event_time).toLocaleString() }}
                                                        </div>
                                                        <div v-if="evidence.metadata && evidence.metadata.teamkill">
                                                            <Badge variant="destructive" class="text-xs">TEAMKILL</Badge>
                                                        </div>
                                                    </div>
                                                </div>
                                            </div>
                                        </div>
                                    </div>
                                    <div v-else class="text-sm text-muted-foreground text-center py-8">
                                        No event evidence attached to this ban.
                                    </div>
                                </TabsContent>

                                <!-- Files Tab -->
                                <TabsContent value="files" class="mt-4">
                                    <div v-if="viewingBanEvidence.evidence && viewingBanEvidence.evidence.filter(e => e.evidence_type === 'file_upload').length > 0" class="space-y-2">
                                        <div
                                            v-for="(evidence, idx) in viewingBanEvidence.evidence.filter(e => e.evidence_type === 'file_upload')"
                                            :key="`file-${idx}`"
                                            class="border rounded-md p-3"
                                        >
                                            <div class="flex items-start gap-3">
                                                <!-- Thumbnail Preview -->
                                                <div
                                                    v-if="canPreview(evidence.file_type)"
                                                    class="w-16 h-16 rounded cursor-pointer overflow-hidden bg-muted flex-shrink-0 flex items-center justify-center"
                                                    @click="openMediaPreviewModal(evidence, idx)"
                                                >
                                                    <img
                                                        v-if="getMediaCategory(evidence.file_type) === 'image'"
                                                        :src="getPreviewUrl(evidence.file_path ?? '')"
                                                        :alt="evidence.file_name ?? 'Image'"
                                                        class="w-full h-full object-cover"
                                                    />
                                                    <Icon
                                                        v-else-if="getMediaCategory(evidence.file_type) === 'video'"
                                                        name="lucide:play-circle"
                                                        class="w-8 h-8 text-muted-foreground"
                                                    />
                                                    <Icon
                                                        v-else-if="getMediaCategory(evidence.file_type) === 'pdf'"
                                                        name="lucide:file-text"
                                                        class="w-8 h-8 text-red-500"
                                                    />
                                                    <Icon
                                                        v-else-if="getMediaCategory(evidence.file_type) === 'text'"
                                                        name="lucide:file-code"
                                                        class="w-8 h-8 text-blue-500"
                                                    />
                                                </div>
                                                <div v-else class="w-16 h-16 rounded bg-muted flex items-center justify-center flex-shrink-0">
                                                    <Icon name="lucide:file" class="w-8 h-8 text-muted-foreground" />
                                                </div>

                                                <!-- File Info -->
                                                <div class="flex-1 min-w-0">
                                                    <div class="font-medium text-sm truncate">{{ evidence.file_name }}</div>
                                                    <div class="text-xs text-muted-foreground mt-1">
                                                        {{ formatFileSize(evidence.file_size || 0) }} &bull; {{ evidence.file_type }}
                                                    </div>
                                                </div>

                                                <!-- Actions -->
                                                <div class="flex gap-2 flex-shrink-0">
                                                    <Button
                                                        v-if="canPreview(evidence.file_type)"
                                                        type="button"
                                                        variant="ghost"
                                                        size="sm"
                                                        @click="openMediaPreviewModal(evidence, idx)"
                                                        title="Preview"
                                                    >
                                                        <Icon name="lucide:eye" class="h-4 w-4" />
                                                    </Button>
                                                    <Button
                                                        type="button"
                                                        variant="outline"
                                                        size="sm"
                                                        @click="downloadEvidenceFile(evidence.file_path ?? '', evidence.file_name ?? 'file')"
                                                        title="Download"
                                                    >
                                                        <Icon name="lucide:download" class="h-4 w-4" />
                                                    </Button>
                                                </div>
                                            </div>
                                        </div>
                                    </div>
                                    <div v-else class="text-sm text-muted-foreground text-center py-8">
                                        No file evidence attached to this ban.
                                    </div>
                                </TabsContent>

                                <!-- Text Tab -->
                                <TabsContent value="text" class="mt-4">
                                    <div v-if="viewingBanEvidence.evidence && viewingBanEvidence.evidence.filter(e => e.evidence_type === 'text_paste').length > 0" class="space-y-2">
                                        <div
                                            v-for="(evidence, idx) in viewingBanEvidence.evidence.filter(e => e.evidence_type === 'text_paste')"
                                            :key="`text-${idx}`"
                                            class="border rounded-md p-3"
                                        >
                                            <div class="font-medium text-sm mb-2">Text Evidence</div>
                                            <div class="text-sm text-muted-foreground whitespace-pre-wrap bg-muted/50 p-2 rounded">
                                                {{ evidence.text_content }}
                                            </div>
                                        </div>
                                    </div>
                                    <div v-else class="text-sm text-muted-foreground text-center py-8">
                                        No text evidence attached to this ban.
                                    </div>
                                </TabsContent>
                            </Tabs>
                        </div>
                        <DialogFooter>
                            <Button variant="outline" @click="closeEvidenceViewDialog">
                                Close
                            </Button>
                        </DialogFooter>
                    </DialogContent>
                </Dialog>

                <!-- Media Preview Modal -->
                <MediaPreviewModal
                    :open="showMediaPreviewModal"
                    :file="previewingFile"
                    :files="previewableFiles"
                    :current-index="previewFileIndex"
                    :preview-url="previewingFile?.file_path ? getPreviewUrl(previewingFile.file_path) : ''"
                    @update:open="showMediaPreviewModal = $event"
                    @navigate="navigatePreview"
                    @download="downloadPreviewedFile"
                />

                <Button
                    @click="refreshData"
                    :disabled="loading"
                    variant="outline"
                    class="w-full sm:w-auto text-sm sm:text-base"
                >
                    {{ loading ? "Refreshing..." : "Refresh" }}
                </Button>
                <Button @click="copyBanCfgUrl" class="w-full sm:w-auto text-sm sm:text-base">Copy Ban Config URL</Button>
            </div>
        </div>

        <div v-if="error" class="bg-red-500 text-white p-3 sm:p-4 rounded mb-3 sm:mb-4 text-sm sm:text-base">
            {{ error }}
        </div>

        <Card class="mb-3 sm:mb-4">
            <CardHeader class="pb-2 sm:pb-3">
                <CardTitle class="text-base sm:text-lg">Ban List</CardTitle>
                <p class="text-xs sm:text-sm text-muted-foreground">
                    View and manage banned players. Data refreshes automatically
                    every 60 seconds.
                </p>
            </CardHeader>
            <CardContent>
                <div class="flex flex-col sm:flex-row items-stretch sm:items-center gap-3 mb-3 sm:mb-4">
                    <Input
                        v-model="searchQuery"
                        placeholder="Search by Steam ID, or reason..."
                        class="flex-grow text-sm sm:text-base"
                    />
                    <div class="flex items-center space-x-2">
                        <Switch
                            id="hide-expired-bans"
                            v-model="hideExpiredBans"
                        />
                        <Label
                            for="hide-expired-bans"
                            class="text-sm cursor-pointer"
                        >
                            Hide expired bans
                        </Label>
                    </div>
                </div>

                <div class="text-xs sm:text-sm text-muted-foreground mb-2">
                    Showing {{ filteredBannedPlayers.length }} of
                    {{ bannedPlayers.length }} bans
                </div>

                <div
                    v-if="loading && bannedPlayers.length === 0"
                    class="text-center py-6 sm:py-8"
                >
                    <div
                        class="animate-spin h-8 w-8 border-4 border-primary border-t-transparent rounded-full mx-auto mb-4"
                    ></div>
                    <p class="text-sm sm:text-base">Loading banned players...</p>
                </div>

                <div
                    v-else-if="bannedPlayers.length === 0"
                    class="text-center py-6 sm:py-8"
                >
                    <p class="text-sm sm:text-base">No banned players found</p>
                </div>

                <div
                    v-else-if="filteredBannedPlayers.length === 0"
                    class="text-center py-6 sm:py-8"
                >
                    <p class="text-sm sm:text-base">No players match your search</p>
                </div>

                <template v-else>
                    <!-- Desktop Table View -->
                    <div class="hidden md:block w-full overflow-x-auto">
                        <Table class="min-w-full">
                            <TableHeader>
                                <TableRow>
                                    <TableHead class="text-xs sm:text-sm">Steam ID</TableHead>
                                    <TableHead class="text-xs sm:text-sm">Reason</TableHead>
                                    <TableHead class="text-xs sm:text-sm">Rule</TableHead>
                                    <TableHead class="text-xs sm:text-sm">Evidence</TableHead>
                                    <TableHead class="text-xs sm:text-sm">Banned At</TableHead>
                                    <TableHead class="text-xs sm:text-sm">Duration</TableHead>
                                    <TableHead class="text-xs sm:text-sm">Source</TableHead>
                                    <TableHead class="text-right text-xs sm:text-sm"
                                        >Actions</TableHead
                                    >
                                </TableRow>
                            </TableHeader>
                            <TableBody>
                                <TableRow
                                    v-for="player in filteredBannedPlayers"
                                    :key="player.id"
                                    class="hover:bg-muted/50"
                                >
                                    <TableCell>
                                        <RouterLink
                                            :to="`/players/${player.steam_id}`"
                                            class="hover:underline"
                                        >
                                            <div class="font-medium text-sm sm:text-base text-primary">
                                                {{ player.name && player.name !== player.steam_id ? player.name : player.steam_id }}
                                            </div>
                                            <div
                                                v-if="player.name && player.name !== player.steam_id"
                                                class="text-xs text-muted-foreground"
                                            >
                                                {{ player.steam_id }}
                                            </div>
                                        </RouterLink>
                                    </TableCell>
                                    <TableCell class="text-xs sm:text-sm">{{ player.reason }}</TableCell>
                                    <TableCell>
                                        <div
                                            v-if="player.rule_name"
                                            class="flex flex-col"
                                        >
                                            <span class="font-medium text-xs sm:text-sm">{{
                                                player.rule_name
                                            }}</span>
                                        </div>
                                        <span
                                            v-else
                                            class="text-muted-foreground text-xs"
                                            >No rule specified</span
                                        >
                                    </TableCell>
                                    <TableCell>
                                        <Button
                                            v-if="(player.evidence && player.evidence.length > 0) || (player.evidence_text && player.evidence_text.trim())"
                                            variant="ghost"
                                            size="sm"
                                            @click="openEvidenceViewDialog(player)"
                                            class="h-auto p-1 text-xs"
                                        >
                                            <div class="flex items-center gap-1">
                                                <Icon name="lucide:file-check" class="h-3 w-3 sm:h-4 sm:w-4 text-green-500" />
                                                <span class="text-xs sm:text-sm">
                                                    {{ player.evidence ? player.evidence.length : 0 }} item(s)
                                                </span>
                                            </div>
                                        </Button>
                                        <span v-else class="text-muted-foreground text-xs">None</span>
                                    </TableCell>
                                    <TableCell class="text-xs sm:text-sm">{{
                                        formatDate(player.created_at)
                                    }}</TableCell>
                                    <TableCell>
                                        <Badge
                                            :variant="
                                                player.duration == 0
                                                    ? 'destructive'
                                                    : 'outline'
                                            "
                                            class="text-xs"
                                        >
                                            {{
                                                player.duration == 0
                                                    ? "Permanent"
                                                    : player.duration + " days"
                                            }}
                                        </Badge>
                                    </TableCell>
                                    <TableCell>
                                        <Badge variant="secondary" class="text-xs">
                                            {{ player.ban_list_name || "Manual" }}
                                        </Badge>
                                    </TableCell>
                                    <TableCell class="text-right">
                                        <div class="flex gap-2 justify-end">
                                            <Button
                                                variant="outline"
                                                size="sm"
                                                @click="openEditBanDialog(player)"
                                                :disabled="loading"
                                                v-if="
                                                    authStore.getServerPermission(
                                                        serverId,
                                                        'ban',
                                                    )
                                                "
                                                class="text-xs"
                                            >
                                                Edit
                                            </Button>
                                            <Button
                                                variant="destructive"
                                                size="sm"
                                                @click="removeBan(player.id)"
                                                :disabled="loading"
                                                v-if="
                                                    authStore.getServerPermission(
                                                        serverId,
                                                        'ban',
                                                    )
                                                "
                                                class="text-xs"
                                            >
                                                Unban
                                            </Button>
                                        </div>
                                    </TableCell>
                                </TableRow>
                            </TableBody>
                        </Table>
                    </div>

                    <!-- Mobile Card View -->
                    <div class="md:hidden space-y-3">
                        <div
                            v-for="player in filteredBannedPlayers"
                            :key="player.id"
                            class="border rounded-lg p-3 sm:p-4 hover:bg-muted/30 transition-colors"
                        >
                        <div class="flex items-start justify-between gap-2 mb-2">
                            <div class="flex-1 min-w-0">
                                <RouterLink
                                    :to="`/players/${player.steam_id}`"
                                    class="hover:underline"
                                >
                                    <div class="font-semibold text-sm sm:text-base mb-1 text-primary">
                                        {{ player.name && player.name !== player.steam_id ? player.name : player.steam_id }}
                                    </div>
                                    <div
                                        v-if="player.name && player.name !== player.steam_id"
                                        class="text-xs text-muted-foreground mb-2"
                                    >
                                        {{ player.steam_id }}
                                    </div>
                                </RouterLink>
                                <div class="space-y-1.5">
                                    <div>
                                        <span class="text-xs text-muted-foreground">Reason: </span>
                                        <span class="text-xs sm:text-sm break-words">{{ player.reason }}</span>
                                    </div>
                                    <div v-if="player.rule_name">
                                        <span class="text-xs text-muted-foreground">Rule: </span>
                                        <span class="text-xs sm:text-sm font-medium">{{ player.rule_name }}</span>
                                    </div>
                                    <div class="flex items-center gap-2">
                                        <Badge
                                            :variant="
                                                player.duration == 0
                                                    ? 'destructive'
                                                    : 'outline'
                                            "
                                            class="text-xs"
                                        >
                                            {{
                                                player.duration == 0
                                                    ? "Permanent"
                                                    : player.duration + " days"
                                            }}
                                        </Badge>
                                        <Badge variant="secondary" class="text-xs">
                                            {{ player.ban_list_name || "Manual" }}
                                        </Badge>
                                    </div>
                                    <div class="text-xs text-muted-foreground">
                                        Banned: {{ formatDate(player.created_at) }}
                                    </div>
                                </div>
                            </div>
                        </div>
                        <div class="flex items-center justify-between gap-2 pt-2 border-t">
                            <Button
                                v-if="(player.evidence && player.evidence.length > 0) || (player.evidence_text && player.evidence_text.trim())"
                                variant="ghost"
                                size="sm"
                                @click="openEvidenceViewDialog(player)"
                                class="h-8 text-xs"
                            >
                                <Icon name="lucide:file-check" class="h-3 w-3 mr-1 text-green-500" />
                                {{ player.evidence ? player.evidence.length : 0 }} evidence
                            </Button>
                            <div v-else class="text-xs text-muted-foreground">None</div>
                            <div class="flex gap-1">
                                <Button
                                    variant="outline"
                                    size="sm"
                                    @click="openEditBanDialog(player)"
                                    :disabled="loading"
                                    v-if="
                                        authStore.getServerPermission(
                                            serverId,
                                            'ban',
                                        )
                                    "
                                    class="h-8 text-xs"
                                >
                                    Edit
                                </Button>
                                <Button
                                    variant="destructive"
                                    size="sm"
                                    @click="removeBan(player.id)"
                                    :disabled="loading"
                                    v-if="
                                        authStore.getServerPermission(
                                            serverId,
                                            'ban',
                                        )
                                    "
                                    class="h-8 text-xs"
                                >
                                    Unban
                                </Button>
                            </div>
                        </div>
                        </div>
                    </div>
                </template>
            </CardContent>
        </Card>

        <!-- Ban List Subscriptions -->
        <Card class="mb-3 sm:mb-4">
            <CardHeader>
                <CardTitle class="text-base sm:text-lg">Ban List Subscriptions</CardTitle>
                <CardDescription class="text-xs sm:text-sm">
                    Manage which ban lists this server subscribes to for
                    automatic ban synchronization.
                </CardDescription>
            </CardHeader>
            <CardContent>
                <div class="space-y-3 sm:space-y-4">
                    <!-- Add Ban List Subscription -->
                    <div class="flex flex-col sm:flex-row gap-2 items-stretch sm:items-center">
                        <Select v-model="selectedBanListId">
                            <SelectTrigger class="w-full sm:w-64 text-sm sm:text-base">
                                <SelectValue
                                    placeholder="Select a ban list to subscribe to"
                                />
                            </SelectTrigger>
                            <SelectContent>
                                <SelectItem
                                    v-for="banList in availableBanLists"
                                    :key="banList.id"
                                    :value="banList.id.toString()"
                                >
                                    {{ banList.name }}
                                </SelectItem>
                            </SelectContent>
                        </Select>
                        <Button
                            @click="subscribeToBanList"
                            :disabled="!selectedBanListId || subscribing"
                            class="w-full sm:w-auto text-sm sm:text-base"
                        >
                            <Plus class="h-4 w-4 mr-2" />
                            {{ subscribing ? "Subscribing..." : "Subscribe" }}
                        </Button>
                    </div>

                    <!-- Current Subscriptions -->
                    <div v-if="subscribedBanLists.length > 0">
                        <h4 class="text-xs sm:text-sm font-medium mb-2">
                            Current Subscriptions
                        </h4>
                        <div class="space-y-2">
                            <div
                                v-for="subscription in subscribedBanLists"
                                :key="subscription.ban_list_id"
                                class="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-2 p-3 border rounded-lg"
                            >
                                <div class="flex-1 min-w-0">
                                    <div class="font-medium text-sm sm:text-base">
                                        {{
                                            subscription.ban_list_name ||
                                            "Unknown Ban List"
                                        }}
                                    </div>
                                    <div class="text-xs sm:text-sm text-gray-500">
                                        Subscribed on
                                        {{
                                            new Date(
                                                subscription.created_at,
                                            ).toLocaleDateString()
                                        }}
                                    </div>
                                </div>
                                <Button
                                    variant="destructive"
                                    size="sm"
                                    @click="
                                        unsubscribeFromBanList(
                                            subscription.ban_list_id.toString(),
                                        )
                                    "
                                    :disabled="
                                        unsubscribing ===
                                        subscription.ban_list_id.toString()
                                    "
                                    class="w-full sm:w-auto text-xs sm:text-sm"
                                >
                                    <Trash2 class="h-3 w-3 sm:h-4 sm:w-4 mr-1 sm:mr-2" />
                                    {{
                                        unsubscribing ===
                                        subscription.ban_list_id.toString()
                                            ? "Unsubscribing..."
                                            : "Unsubscribe"
                                    }}
                                </Button>
                            </div>
                        </div>
                    </div>

                    <div v-else class="text-center text-gray-500 py-4 text-xs sm:text-sm">
                        No ban list subscriptions configured
                    </div>
                </div>
            </CardContent>
        </Card>

        <Card>
            <CardHeader>
                <CardTitle class="text-base sm:text-lg">About Bans</CardTitle>
            </CardHeader>
            <CardContent>
                <p class="text-xs sm:text-sm text-muted-foreground">
                    This page shows players who have been banned from the
                    server. You can add new bans manually or remove existing
                    bans.
                </p>
                <p class="text-xs sm:text-sm text-muted-foreground mt-2">
                    Permanent bans will remain in effect until manually removed.
                    Temporary bans will expire after the specified duration.
                </p>
                <p class="text-xs sm:text-sm text-muted-foreground mt-2">
                    Ban list subscriptions allow this server to automatically
                    include bans from other shared ban lists. Players banned on
                    subscribed lists will be automatically banned on this server
                    as well.
                </p>
                <p class="text-xs sm:text-sm text-muted-foreground mt-2">
                    <strong>Note:</strong> Squad servers typically cache ban
                    configurations and refresh them periodically. Changes to ban
                    list subscriptions will be reflected in the ban
                    configuration immediately, but may take some time to take
                    effect in-game depending on your server's ban list refresh
                    interval.
                </p>
            </CardContent>
        </Card>

    </div>
</template>

<style scoped>
/* Add any page-specific styles here */
</style>

