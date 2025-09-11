<script setup lang="ts">
import { Card, CardHeader, CardContent, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import { Badge } from "@/components/ui/badge";
import { 
    Table, 
    TableBody, 
    TableCell, 
    TableHead, 
    TableHeader, 
    TableRow 
} from "@/components/ui/table";

useHead({
    title: "Backup & Restore",
});

definePageMeta({
    middleware: "auth",
});

const runtimeConfig = useRuntimeConfig();

// Reactive state
const loading = ref(false);
const restoreLoading = ref(false);
const alert = ref<{ type: 'success' | 'error' | 'info'; message: string } | null>(null);
const restoreFile = ref<File | null>(null);
const backupHistory = ref<any[]>([]);

// File input ref
const fileInputRef = ref<HTMLInputElement | null>(null);

// Create backup
const createBackup = async () => {
    if (loading.value) return;
    
    loading.value = true;
    alert.value = null;
    
    try {
        const response = await fetch(`${runtimeConfig.public.backendApi}/admin/backup`, {
            headers: {
                'Authorization': `Bearer ${useCookie(runtimeConfig.public.sessionCookieName).value}`,
                'Content-Type': 'application/json',
            },
        });

        if (!response.ok) {
            const errorData = await response.json();
            throw new Error(errorData.message || 'Failed to create backup');
        }

        // The response is a file download
        const blob = await response.blob();
        const url = window.URL.createObjectURL(blob);
        const link = document.createElement('a');
        link.href = url;
        
        // Extract filename from Content-Disposition header if available
        const contentDisposition = response.headers.get('Content-Disposition');
        let filename = 'squad-aegis-backup.tar.gz';
        if (contentDisposition) {
            const filenameMatch = contentDisposition.match(/filename="?([^"]+)"?/);
            if (filenameMatch) {
                filename = filenameMatch[1];
            }
        }
        
        link.download = filename;
        document.body.appendChild(link);
        link.click();
        document.body.removeChild(link);
        window.URL.revokeObjectURL(url);

        alert.value = {
            type: 'success',
            message: 'Backup created successfully and download started!'
        };
        
    } catch (error: any) {
        alert.value = {
            type: 'error',
            message: error.message || 'Failed to create backup'
        };
    } finally {
        loading.value = false;
    }
};

// Handle file selection
const handleFileSelect = (event: Event) => {
    const target = event.target as HTMLInputElement;
    if (target.files && target.files.length > 0) {
        restoreFile.value = target.files[0];
    }
};

// Restore backup
const restoreBackup = async () => {
    if (restoreLoading.value || !restoreFile.value) return;
    
    restoreLoading.value = true;
    alert.value = null;
    
    try {
        const formData = new FormData();
        formData.append('backup', restoreFile.value);
        
        const response = await fetch(`${runtimeConfig.public.backendApi}/admin/restore`, {
            method: 'POST',
            headers: {
                'Authorization': `Bearer ${useCookie(runtimeConfig.public.sessionCookieName).value}`,
            },
            body: formData
        });

        const data = await response.json();

        if (!response.ok) {
            throw new Error(data.message || 'Failed to restore backup');
        }

        alert.value = {
            type: 'success',
            message: 'Backup restored successfully! The system has been restored to the backup state.'
        };
        
        // Clear the file input
        restoreFile.value = null;
        if (fileInputRef.value) {
            fileInputRef.value.value = '';
        }
        
    } catch (error: any) {
        alert.value = {
            type: 'error',
            message: error.message || 'Failed to restore backup'
        };
    } finally {
        restoreLoading.value = false;
    }
};

// Format file size
const formatFileSize = (bytes: number): string => {
    if (bytes === 0) return '0 Bytes';
    const k = 1024;
    const sizes = ['Bytes', 'KB', 'MB', 'GB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
};

// Clear alert after 5 seconds
watch(alert, (newAlert) => {
    if (newAlert) {
        setTimeout(() => {
            alert.value = null;
        }, 5000);
    }
});
</script>

<template>
    <div class="p-4 max-w-6xl mx-auto">
        <h1 class="text-2xl font-bold mb-6">Backup & Restore</h1>
        
        <!-- Alert -->
        <div v-if="alert" :class="{
            'border-red-500 bg-red-50 text-red-700': alert.type === 'error',
            'border-green-500 bg-green-50 text-green-700': alert.type === 'success',
            'border-blue-500 bg-blue-50 text-blue-700': alert.type === 'info'
        }" class="mb-6 p-4 border rounded-lg">
            <p>{{ alert.message }}</p>
        </div>

        <div class="grid grid-cols-1 lg:grid-cols-2 gap-6">
            <!-- Create Backup Card -->
            <Card>
                <CardHeader>
                    <CardTitle class="flex items-center gap-2">
                        <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M7 16a4 4 0 01-.88-7.903A5 5 0 1115.9 6L16 6a5 5 0 011 9.9M9 19l3 3m0 0l3-3m-3 3V10" />
                        </svg>
                        Create Backup
                    </CardTitle>
                </CardHeader>
                <CardContent class="space-y-4">                    
                    <div class="bg-blue-50 p-4 rounded-lg">
                        <h4 class="font-semibold text-blue-900 mb-2">What will be backed up:</h4>
                        <ul class="text-sm text-blue-800 space-y-1">
                            <li>• All PostgreSQL data (users, servers, bans, etc.)</li>
                            <li>• All ClickHouse data (events, logs, metrics)</li>
                            <li>• System configuration and metadata</li>
                        </ul>
                    </div>
                    
                    <Button 
                        @click="createBackup" 
                        :disabled="loading"
                        class="w-full"
                    >
                        <svg v-if="loading" class="animate-spin -ml-1 mr-3 h-4 w-4" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24">
                            <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
                            <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
                        </svg>
                        {{ loading ? 'Creating Backup...' : 'Create Backup' }}
                    </Button>
                </CardContent>
            </Card>

            <!-- Restore Backup Card -->
            <Card>
                <CardHeader>
                    <CardTitle class="flex items-center gap-2">
                        <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M7 16a4 4 0 01-.88-7.903A5 5 0 1115.9 6L16 6a5 5 0 011 9.9M15 13l-3-3m0 0l-3 3m3-3v12" />
                        </svg>
                        Restore Backup
                    </CardTitle>
                </CardHeader>
                <CardContent class="space-y-4">
                    <div class="space-y-2">
                        <Label for="backup-file">Select Backup File</Label>
                        <Input 
                            id="backup-file"
                            ref="fileInputRef"
                            type="file" 
                            accept=".tar.gz,.tgz"
                            @change="handleFileSelect"
                            :disabled="restoreLoading"
                        />
                        <p class="text-xs text-muted-foreground">
                            Select a .tar.gz backup file to restore
                        </p>
                    </div>
                    
                    <div v-if="restoreFile" class="bg-green-50 p-3 rounded-lg">
                        <div class="flex items-center justify-between">
                            <div>
                                <p class="font-medium text-green-900">{{ restoreFile.name }}</p>
                                <p class="text-sm text-green-700">{{ formatFileSize(restoreFile.size) }}</p>
                            </div>
                            <Badge variant="secondary" class="bg-green-100 text-green-800">
                                Ready to restore
                            </Badge>
                        </div>
                    </div>
                    
                    <div class="bg-red-50 p-4 rounded-lg">
                        <h4 class="font-semibold text-red-900 mb-2">⚠️ Warning:</h4>
                        <ul class="text-sm text-red-800 space-y-1">
                            <li>• This will overwrite ALL existing data</li>
                            <li>• The operation cannot be undone</li>
                            <li>• Make sure to create a backup first</li>
                            <li>• All users will be disconnected</li>
                        </ul>
                    </div>
                    
                    <Button 
                        @click="restoreBackup" 
                        :disabled="restoreLoading || !restoreFile"
                        variant="destructive"
                        class="w-full"
                    >
                        <svg v-if="restoreLoading" class="animate-spin -ml-1 mr-3 h-4 w-4" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24">
                            <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
                            <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
                        </svg>
                        {{ restoreLoading ? 'Restoring...' : 'Restore Backup' }}
                    </Button>
                </CardContent>
            </Card>
        </div>

        <!-- Info Section -->
        <Card class="mt-6">
            <CardHeader>
                <CardTitle>Backup Information</CardTitle>
            </CardHeader>
            <CardContent class="space-y-4">
                <div class="border-t pt-4">
                    <h4 class="font-semibold mb-2">Technical Details</h4>
                    <div class="grid grid-cols-1 md:grid-cols-3 gap-4 text-sm">
                        <div>
                            <span class="font-medium">Format:</span>
                            <span class="ml-2 text-muted-foreground">Compressed tar.gz archive</span>
                        </div>
                        <div>
                            <span class="font-medium">Compression:</span>
                            <span class="ml-2 text-muted-foreground">GZIP compression</span>
                        </div>
                        <div>
                            <span class="font-medium">Structure:</span>
                            <span class="ml-2 text-muted-foreground">SQL dumps + metadata JSON</span>
                        </div>
                    </div>
                </div>
            </CardContent>
        </Card>
    </div>
</template>

<style scoped>
/* Custom styles if needed */
</style>
