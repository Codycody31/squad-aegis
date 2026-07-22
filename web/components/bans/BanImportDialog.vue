<script setup lang="ts">
import { ref, watch } from "vue";
import { Badge } from "~/components/ui/badge";
import { Button } from "~/components/ui/button";
import { Card, CardContent } from "~/components/ui/card";
import {
    Dialog,
    DialogContent,
    DialogDescription,
    DialogFooter,
    DialogHeader,
    DialogTitle,
} from "~/components/ui/dialog";
import {
    Table,
    TableBody,
    TableCell,
    TableHead,
    TableHeader,
    TableRow,
} from "~/components/ui/table";
import type { BanImportPreviewData, BanImportResultData } from "~/types/bans";

const props = defineProps<{
    serverId: string;
}>();

const emit = defineEmits<{
    (e: "imported"): void;
}>();

const open = defineModel<boolean>("open", { default: false });

const runtimeConfig = useRuntimeConfig();

const importStep = ref<'preview' | 'result'>('preview');
const importLoading = ref(false);
const importPreview = ref<BanImportPreviewData | null>(null);
const importResult = ref<BanImportResultData | null>(null);
const importError = ref<string | null>(null);

// Fetch a fresh preview each time the dialog is opened.
watch(open, (isOpen) => {
    if (isOpen) {
        loadPreview();
    }
});

async function loadPreview() {
    importStep.value = 'preview';
    importPreview.value = null;
    importResult.value = null;
    importError.value = null;
    importLoading.value = true;

    try {
        const data = await useAuthFetchImperative<any>(
            `${runtimeConfig.public.backendApi}/servers/${props.serverId}/bans/import-preview`,
        );

        if (data.code === 200) {
            importPreview.value = data.data.preview;
        } else {
            importError.value = data.message || 'Failed to fetch preview';
        }
    } catch (e: any) {
        importError.value = e.message || 'Failed to connect';
    } finally {
        importLoading.value = false;
    }
}

async function executeImport() {
    importLoading.value = true;
    importError.value = null;

    try {
        const data = await useAuthFetchImperative<any>(
            `${runtimeConfig.public.backendApi}/servers/${props.serverId}/bans/import`,
            {
                method: 'POST',
                body: { confirm: true },
            },
        );

        if (data.code === 200) {
            importResult.value = data.data.result;
            importStep.value = 'result';
            // Refresh bans list
            emit("imported");
        } else {
            importError.value = data.message || 'Import failed';
        }
    } catch (e: any) {
        importError.value = e.message || 'Failed to connect';
    } finally {
        importLoading.value = false;
    }
}
</script>

<template>
    <!-- Import from Bans.cfg Dialog -->
    <Dialog v-model:open="open">
        <DialogContent class="w-[95vw] sm:max-w-[600px] max-h-[80vh] overflow-y-auto p-4 sm:p-6">
            <DialogHeader>
                <DialogTitle>Import Bans from Bans.cfg</DialogTitle>
                <DialogDescription>
                    Import existing bans from the game server's Bans.cfg file into the Aegis database.
                </DialogDescription>
            </DialogHeader>

            <!-- Loading -->
            <div v-if="importLoading" class="py-8 text-center text-muted-foreground">
                Loading...
            </div>

            <!-- Error -->
            <div v-else-if="importError" class="py-4">
                <p class="text-destructive text-sm">{{ importError }}</p>
            </div>

            <!-- Preview Step -->
            <div v-else-if="importStep === 'preview' && importPreview" class="space-y-4 py-4">
                <div v-if="!importPreview.cfg_available" class="text-sm text-muted-foreground">
                    <p>File access is not configured for this server.</p>
                    <p class="mt-1">Configure the SquadGame base path and log source type in server settings to enable Bans.cfg import.</p>
                </div>

                <template v-else>
                    <p class="text-sm text-muted-foreground">File: <code class="bg-muted px-1 py-0.5 rounded text-xs">{{ importPreview.cfg_path }}</code></p>

                    <div class="grid grid-cols-2 sm:grid-cols-4 gap-3">
                        <Card>
                            <CardContent class="p-3 text-center">
                                <div class="text-2xl font-bold">{{ importPreview.new_bans?.length || 0 }}</div>
                                <div class="text-xs text-muted-foreground">New bans</div>
                            </CardContent>
                        </Card>
                        <Card>
                            <CardContent class="p-3 text-center">
                                <div class="text-2xl font-bold">{{ importPreview.existing_bans?.length || 0 }}</div>
                                <div class="text-xs text-muted-foreground">Already in DB</div>
                            </CardContent>
                        </Card>
                        <Card>
                            <CardContent class="p-3 text-center">
                                <div class="text-2xl font-bold">{{ importPreview.expired_bans?.length || 0 }}</div>
                                <div class="text-xs text-muted-foreground">Expired (skip)</div>
                            </CardContent>
                        </Card>
                        <Card>
                            <CardContent class="p-3 text-center">
                                <div class="text-2xl font-bold">{{ importPreview.auto_bans?.length || 0 }}</div>
                                <div class="text-xs text-muted-foreground">Auto-bans (skip)</div>
                            </CardContent>
                        </Card>
                    </div>

                    <div v-if="importPreview.unparseable_count > 0" class="text-xs text-destructive">
                        {{ importPreview.unparseable_count }} line(s) could not be parsed. Fix or remove those active lines in Bans.cfg before importing.
                    </div>

                    <!-- New bans preview table -->
                    <div v-if="importPreview.new_bans?.length" class="space-y-2">
                        <h4 class="text-sm font-medium">Bans to import:</h4>
                        <div class="max-h-48 overflow-y-auto border rounded">
                            <Table>
                                <TableHeader>
                                    <TableRow>
                                        <TableHead class="text-xs">Player ID</TableHead>
                                        <TableHead class="text-xs">Reason</TableHead>
                                        <TableHead class="text-xs">Type</TableHead>
                                    </TableRow>
                                </TableHeader>
                                <TableBody>
                                    <TableRow v-for="ban in importPreview.new_bans" :key="ban.steam_id || ban.eos_id">
                                        <TableCell class="text-xs font-mono">
                                            {{ ban.steam_id || ban.eos_id }}
                                            <Badge v-if="ban.eos_id" variant="outline" class="ml-1 text-[10px] px-1 py-0">EOS</Badge>
                                        </TableCell>
                                        <TableCell class="text-xs">{{ ban.reason || '(no reason)' }}</TableCell>
                                        <TableCell class="text-xs">
                                            <Badge :variant="ban.permanent ? 'destructive' : 'secondary'">
                                                {{ ban.permanent ? 'Permanent' : 'Timed' }}
                                            </Badge>
                                        </TableCell>
                                    </TableRow>
                                </TableBody>
                            </Table>
                        </div>
                    </div>

                    <!-- Auto-bans info -->
                    <div v-if="importPreview.auto_bans?.length" class="text-xs text-muted-foreground">
                        {{ importPreview.auto_bans.length }} automatic server ban(s) (e.g., teamkill kicks) will be skipped.
                    </div>

                    <div v-if="!importPreview.new_bans?.length" class="text-sm text-muted-foreground py-2">
                        No new bans to import. All entries in Bans.cfg are already in the database, expired, or are automatic server bans.
                    </div>
                </template>
            </div>

            <!-- Result Step -->
            <div v-else-if="importStep === 'result' && importResult" class="space-y-4 py-4">
                <div class="grid grid-cols-3 gap-3">
                    <Card>
                        <CardContent class="p-3 text-center">
                            <div class="text-2xl font-bold text-green-600">{{ importResult.imported_count }}</div>
                            <div class="text-xs text-muted-foreground">Imported</div>
                        </CardContent>
                    </Card>
                    <Card>
                        <CardContent class="p-3 text-center">
                            <div class="text-2xl font-bold">{{ importResult.skipped_count }}</div>
                            <div class="text-xs text-muted-foreground">Skipped</div>
                        </CardContent>
                    </Card>
                    <Card>
                        <CardContent class="p-3 text-center">
                            <div class="text-2xl font-bold">{{ importResult.expired_count }}</div>
                            <div class="text-xs text-muted-foreground">Expired</div>
                        </CardContent>
                    </Card>
                </div>

                <div v-if="importResult.errors?.length" class="space-y-1">
                    <p class="text-sm font-medium text-destructive">Errors:</p>
                    <ul class="text-xs text-destructive list-disc pl-4">
                        <li v-for="(err, i) in importResult.errors" :key="i">{{ err }}</li>
                    </ul>
                </div>
            </div>

            <DialogFooter>
                <template v-if="importStep === 'preview' && importPreview?.cfg_available && (importPreview?.new_bans?.length || 0) > 0">
                    <Button variant="outline" @click="open = false">Cancel</Button>
                    <Button @click="executeImport" :disabled="importLoading || importPreview.unparseable_count > 0">
                        Import {{ importPreview?.new_bans?.length }} Ban(s)
                    </Button>
                </template>
                <template v-else>
                    <Button @click="open = false">Close</Button>
                </template>
            </DialogFooter>
        </DialogContent>
    </Dialog>
</template>
