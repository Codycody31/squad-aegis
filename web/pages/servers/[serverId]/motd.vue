<template>
    <div class="p-4">
        <div class="flex justify-between items-center mb-4">
            <h1 class="text-2xl font-bold">MOTD Configuration</h1>
            <p class="text-sm text-muted-foreground">
                Configure the Message of the Day for your server
            </p>
        </div>

        <!-- Credentials Warning -->
        <Card v-if="!hasCredentials" class="mb-4 border-yellow-500">
            <CardHeader>
                <CardTitle class="text-yellow-600 flex items-center gap-2">
                    <Icon name="lucide:alert-triangle" class="h-5 w-5" />
                    FTP/SFTP Not Configured
                </CardTitle>
            </CardHeader>
            <CardContent>
                <p class="text-sm text-muted-foreground mb-2">
                    Upload functionality requires FTP or SFTP credentials. You can either:
                </p>
                <ul class="list-disc list-inside text-sm text-muted-foreground mb-4">
                    <li>Configure FTP/SFTP credentials in <RouterLink :to="`/servers/${serverId}/settings`" class="text-primary hover:underline">Server Settings</RouterLink> (Log Configuration section)</li>
                    <li>Set custom upload credentials below</li>
                </ul>
                <p class="text-sm text-muted-foreground">
                    You can still preview and copy the generated MOTD content manually.
                </p>
            </CardContent>
        </Card>

        <!-- Content Configuration -->
        <Card class="mb-4">
            <CardHeader>
                <CardTitle>Content</CardTitle>
                <p class="text-sm text-muted-foreground">
                    Configure the prefix and suffix text that wraps around your rules
                </p>
            </CardHeader>
            <CardContent class="space-y-4">
                <div class="space-y-2">
                    <label class="text-sm font-medium">Prefix Text</label>
                    <p class="text-xs text-muted-foreground">
                        Text displayed before the rules (e.g., Discord link, admin help info)
                    </p>
                    <Textarea
                        v-model="config.prefix_text"
                        placeholder="Visit our <a href='https://discord.gg/example'>discord</a> for the latest info!&#10;Use '!admin <your message>' in any chat for urgent issues."
                        rows="4"
                        @input="() => markDirty()"
                    />
                </div>

                <div class="space-y-2">
                    <label class="text-sm font-medium">Suffix Text</label>
                    <p class="text-xs text-muted-foreground">
                        Text displayed after the rules (e.g., acknowledgment text)
                    </p>
                    <Textarea
                        v-model="config.suffix_text"
                        placeholder="By playing on this server, you have acknowledged that you have read these rules and will be punished, new or old."
                        rows="4"
                        @input="() => markDirty()"
                    />
                </div>
            </CardContent>
        </Card>

        <!-- Generation Settings -->
        <Card class="mb-4">
            <CardHeader>
                <CardTitle>Generation Settings</CardTitle>
                <p class="text-sm text-muted-foreground">
                    Control how rules are included in the MOTD
                </p>
            </CardHeader>
            <CardContent class="space-y-4">
                <div class="flex items-center justify-between">
                    <div class="space-y-0.5">
                        <label class="text-sm font-medium">Auto-generate from Rules</label>
                        <p class="text-xs text-muted-foreground">
                            Automatically include server rules in the MOTD
                        </p>
                    </div>
                    <Switch
                        v-model="config.auto_generate_from_rules"
                        @update:modelValue="() => markDirty()"
                    />
                </div>

                <div class="flex items-center justify-between">
                    <div class="space-y-0.5">
                        <label class="text-sm font-medium">Include Rule Descriptions</label>
                        <p class="text-xs text-muted-foreground">
                            Show rule descriptions as bullet points under each rule
                        </p>
                    </div>
                    <Switch
                        v-model="config.include_rule_descriptions"
                        @update:modelValue="() => markDirty()"
                    />
                </div>
            </CardContent>
        </Card>

        <!-- Upload Configuration -->
        <Card class="mb-4">
            <CardHeader>
                <CardTitle>Upload Configuration</CardTitle>
                <p class="text-sm text-muted-foreground">
                    Configure how the MOTD is uploaded to your game server
                </p>
            </CardHeader>
            <CardContent class="space-y-4">
                <div class="flex items-center justify-between">
                    <div class="space-y-0.5">
                        <label class="text-sm font-medium">Enable Upload</label>
                        <p class="text-xs text-muted-foreground">
                            Allow uploading MOTD to the game server via FTP/SFTP
                        </p>
                    </div>
                    <Switch
                        v-model="config.upload_enabled"
                        @update:modelValue="() => markDirty()"
                    />
                </div>

                <div v-if="config.upload_enabled" class="flex items-center justify-between">
                    <div class="space-y-0.5">
                        <label class="text-sm font-medium">Auto-upload on Change</label>
                        <p class="text-xs text-muted-foreground">
                            Automatically upload when MOTD config or rules change
                        </p>
                    </div>
                    <Switch
                        v-model="config.auto_upload_on_change"
                        @update:modelValue="() => markDirty()"
                    />
                </div>

                <div v-if="config.upload_enabled" class="grid grid-cols-4 items-center gap-4">
                    <label class="text-right text-sm">File Path</label>
                    <Input
                        v-model="config.motd_file_path"
                        class="col-span-3"
                        placeholder="/SquadGame/ServerConfig/MOTD.cfg"
                        @input="() => markDirty()"
                    />
                </div>

                <div v-if="config.upload_enabled" class="flex items-center justify-between">
                    <div class="space-y-0.5">
                        <label class="text-sm font-medium">Use Log Credentials</label>
                        <p class="text-xs text-muted-foreground">
                            Use the FTP/SFTP credentials from server log configuration
                        </p>
                    </div>
                    <Switch
                        v-model="config.use_log_credentials"
                        @update:modelValue="() => markDirty()"
                    />
                </div>

                <!-- Custom Credentials -->
                <template v-if="config.upload_enabled && !config.use_log_credentials">
                    <div class="border-t pt-4 mt-4">
                        <h4 class="text-sm font-medium mb-4">Custom Upload Credentials</h4>

                        <div class="grid grid-cols-4 items-center gap-4 mb-4">
                            <label class="text-right text-sm">Protocol</label>
                            <Select v-model="config.upload_protocol" @update:modelValue="() => markDirty()">
                                <SelectTrigger class="col-span-3">
                                    <SelectValue placeholder="Select protocol" />
                                </SelectTrigger>
                                <SelectContent>
                                    <SelectItem value="sftp">SFTP</SelectItem>
                                    <SelectItem value="ftp">FTP</SelectItem>
                                </SelectContent>
                            </Select>
                        </div>

                        <div class="grid grid-cols-4 items-center gap-4 mb-4">
                            <label class="text-right text-sm">Host</label>
                            <Input
                                v-model="config.upload_host"
                                class="col-span-3"
                                placeholder="192.168.1.100"
                                @input="() => markDirty()"
                            />
                        </div>

                        <div class="grid grid-cols-4 items-center gap-4 mb-4">
                            <label class="text-right text-sm">Port</label>
                            <Input
                                v-model.number="config.upload_port"
                                type="number"
                                class="col-span-3"
                                :placeholder="config.upload_protocol === 'sftp' ? '22' : '21'"
                                @input="() => markDirty()"
                            />
                        </div>

                        <div class="grid grid-cols-4 items-center gap-4 mb-4">
                            <label class="text-right text-sm">Username</label>
                            <Input
                                v-model="config.upload_username"
                                class="col-span-3"
                                placeholder="username"
                                @input="() => markDirty()"
                            />
                        </div>

                        <div class="grid grid-cols-4 items-center gap-4">
                            <label class="text-right text-sm">Password</label>
                            <Input
                                v-model="config.upload_password"
                                type="password"
                                class="col-span-3"
                                placeholder="********"
                                @input="() => markDirty()"
                            />
                        </div>
                    </div>
                </template>

                <!-- Last Upload Status -->
                <div v-if="config.last_uploaded_at || config.last_upload_error" class="border-t pt-4 mt-4">
                    <h4 class="text-sm font-medium mb-2">Last Upload Status</h4>
                    <div v-if="config.last_uploaded_at" class="text-sm text-green-600">
                        Last uploaded: {{ formatDate(config.last_uploaded_at) }}
                    </div>
                    <div v-if="config.last_upload_error" class="text-sm text-red-600">
                        Error: {{ config.last_upload_error }}
                    </div>
                </div>
            </CardContent>
        </Card>

        <!-- Preview -->
        <Card class="mb-4">
            <CardHeader class="flex flex-row items-center justify-between">
                <div>
                    <CardTitle>Preview</CardTitle>
                    <p class="text-sm text-muted-foreground">
                        Preview of the generated MOTD content
                    </p>
                </div>
                <div class="flex gap-2">
                    <Button variant="outline" size="sm" @click="refreshPreview" :disabled="isLoadingPreview">
                        <Icon v-if="isLoadingPreview" name="lucide:loader-2" class="h-4 w-4 mr-2 animate-spin" />
                        <Icon v-else name="lucide:refresh-cw" class="h-4 w-4 mr-2" />
                        Refresh
                    </Button>
                    <Button variant="outline" size="sm" @click="copyToClipboard">
                        <Icon name="lucide:copy" class="h-4 w-4 mr-2" />
                        Copy
                    </Button>
                </div>
            </CardHeader>
            <CardContent>
                <div v-if="previewContent" class="bg-muted p-4 rounded-md font-mono text-sm whitespace-pre-wrap max-h-96 overflow-y-auto">
                    {{ previewContent }}
                </div>
                <div v-else class="text-muted-foreground text-sm">
                    Click "Refresh" to generate preview
                </div>
                <div v-if="previewRulesCount !== null" class="mt-2 text-xs text-muted-foreground">
                    {{ previewRulesCount }} rules included
                </div>
            </CardContent>
        </Card>

        <!-- Actions -->
        <div class="flex justify-between items-center">
            <div class="flex gap-2">
                <Button
                    variant="outline"
                    @click="testConnection"
                    :disabled="!config.upload_enabled || isTestingConnection"
                >
                    <Icon v-if="isTestingConnection" name="lucide:loader-2" class="h-4 w-4 mr-2 animate-spin" />
                    <Icon v-else name="lucide:plug" class="h-4 w-4 mr-2" />
                    Test Connection
                </Button>
            </div>
            <div class="flex gap-2">
                <Button
                    variant="outline"
                    @click="saveConfig"
                    :disabled="!isDirty || isSaving"
                >
                    <Icon v-if="isSaving" name="lucide:loader-2" class="h-4 w-4 mr-2 animate-spin" />
                    <Icon v-else name="lucide:save" class="h-4 w-4 mr-2" />
                    Save Configuration
                </Button>
                <Button
                    @click="uploadMOTD"
                    :disabled="!config.upload_enabled || !hasCredentials || isUploading"
                >
                    <Icon v-if="isUploading" name="lucide:loader-2" class="h-4 w-4 mr-2 animate-spin" />
                    <Icon v-else name="lucide:upload" class="h-4 w-4 mr-2" />
                    Upload MOTD
                </Button>
            </div>
        </div>
    </div>
</template>

<script setup lang="ts">
import { ref, onMounted, computed } from "vue";
import { useRoute } from "vue-router";
import { useToast } from "~/components/ui/toast";
import { Button } from "~/components/ui/button";
import { Input } from "~/components/ui/input";
import { Textarea } from "~/components/ui/textarea";
import { Card, CardContent, CardHeader, CardTitle } from "~/components/ui/card";
import { Switch } from "~/components/ui/switch";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "~/components/ui/select";

definePageMeta({ middleware: ["auth"] });

interface MOTDConfig {
    id: string;
    server_id: string;
    prefix_text: string;
    suffix_text: string;
    auto_generate_from_rules: boolean;
    include_rule_descriptions: boolean;
    upload_enabled: boolean;
    auto_upload_on_change: boolean;
    motd_file_path: string;
    use_log_credentials: boolean;
    upload_host: string;
    upload_port: number | null;
    upload_username: string;
    upload_password: string;
    upload_protocol: string;
    last_uploaded_at: string | null;
    last_upload_error: string | null;
    last_generated_content: string | null;
}

const route = useRoute();
const { toast } = useToast();

const runtimeConfig = useRuntimeConfig();
const cookieToken = useCookie(runtimeConfig.public.sessionCookieName as string);
const token = cookieToken.value;

const serverId = route.params.serverId as string;

const config = ref<MOTDConfig>({
    id: "",
    server_id: serverId,
    prefix_text: "",
    suffix_text: "",
    auto_generate_from_rules: true,
    include_rule_descriptions: true,
    upload_enabled: false,
    auto_upload_on_change: false,
    motd_file_path: "/SquadGame/ServerConfig/MOTD.cfg",
    use_log_credentials: true,
    upload_host: "",
    upload_port: null,
    upload_username: "",
    upload_password: "",
    upload_protocol: "",
    last_uploaded_at: null,
    last_upload_error: null,
    last_generated_content: null,
});

const hasCredentials = ref(false);
const credentialSource = ref("");
const isDirty = ref(false);
const isSaving = ref(false);
const isUploading = ref(false);
const isTestingConnection = ref(false);
const isLoadingPreview = ref(false);
const previewContent = ref("");
const previewRulesCount = ref<number | null>(null);

const markDirty = () => {
    isDirty.value = true;
};

const formatDate = (dateString: string) => {
    return new Date(dateString).toLocaleString();
};

const fetchConfig = async () => {
    try {
        const response = await fetch(`/api/servers/${serverId}/motd`, {
            headers: {
                Authorization: `Bearer ${token}`,
            },
        });
        const data = await response.json();

        if (data.code === 200) {
            config.value = { ...config.value, ...data.data.config };
            hasCredentials.value = data.data.has_credentials;
            credentialSource.value = data.data.credential_source;
            isDirty.value = false;
        }
    } catch (error) {
        toast({
            title: "Error",
            description: "Failed to fetch MOTD configuration",
            variant: "destructive",
        });
    }
};

const saveConfig = async () => {
    isSaving.value = true;
    try {
        const response = await fetch(`/api/servers/${serverId}/motd`, {
            method: "PUT",
            headers: {
                "Content-Type": "application/json",
                Authorization: `Bearer ${token}`,
            },
            body: JSON.stringify({
                prefix_text: config.value.prefix_text,
                suffix_text: config.value.suffix_text,
                auto_generate_from_rules: config.value.auto_generate_from_rules,
                include_rule_descriptions: config.value.include_rule_descriptions,
                upload_enabled: config.value.upload_enabled,
                auto_upload_on_change: config.value.auto_upload_on_change,
                motd_file_path: config.value.motd_file_path,
                use_log_credentials: config.value.use_log_credentials,
                upload_host: config.value.upload_host || null,
                upload_port: config.value.upload_port || null,
                upload_username: config.value.upload_username || null,
                upload_password: config.value.upload_password || null,
                upload_protocol: config.value.upload_protocol || null,
            }),
        });

        const data = await response.json();
        if (data.code === 200) {
            toast({
                title: "Success",
                description: "MOTD configuration saved",
            });
            config.value = { ...config.value, ...data.data.config };
            isDirty.value = false;
        } else {
            toast({
                title: "Error",
                description: data.message || "Failed to save configuration",
                variant: "destructive",
            });
        }
    } catch (error) {
        toast({
            title: "Error",
            description: "Failed to save configuration",
            variant: "destructive",
        });
    } finally {
        isSaving.value = false;
    }
};

const refreshPreview = async () => {
    isLoadingPreview.value = true;
    try {
        const response = await fetch(`/api/servers/${serverId}/motd/preview`, {
            headers: {
                Authorization: `Bearer ${token}`,
            },
        });
        const data = await response.json();

        if (data.code === 200) {
            previewContent.value = data.data.content;
            previewRulesCount.value = data.data.rules_count;
        }
    } catch (error) {
        toast({
            title: "Error",
            description: "Failed to generate preview",
            variant: "destructive",
        });
    } finally {
        isLoadingPreview.value = false;
    }
};

const copyToClipboard = async () => {
    if (!previewContent.value) {
        await refreshPreview();
    }

    try {
        await navigator.clipboard.writeText(previewContent.value);
        toast({
            title: "Copied",
            description: "MOTD content copied to clipboard",
        });
    } catch (error) {
        toast({
            title: "Error",
            description: "Failed to copy to clipboard",
            variant: "destructive",
        });
    }
};

const testConnection = async () => {
    isTestingConnection.value = true;
    try {
        const response = await fetch(`/api/servers/${serverId}/motd/test-connection`, {
            method: "POST",
            headers: {
                Authorization: `Bearer ${token}`,
            },
        });
        const data = await response.json();

        if (data.code === 200 && data.data.success) {
            toast({
                title: "Connection Successful",
                description: data.data.message,
            });
        } else {
            toast({
                title: "Connection Failed",
                description: data.data?.error || data.message || "Could not connect",
                variant: "destructive",
            });
        }
    } catch (error) {
        toast({
            title: "Error",
            description: "Failed to test connection",
            variant: "destructive",
        });
    } finally {
        isTestingConnection.value = false;
    }
};

const uploadMOTD = async () => {
    isUploading.value = true;
    try {
        const response = await fetch(`/api/servers/${serverId}/motd/upload`, {
            method: "POST",
            headers: {
                Authorization: `Bearer ${token}`,
            },
        });
        const data = await response.json();

        if (data.code === 200) {
            toast({
                title: "Upload Successful",
                description: `MOTD uploaded at ${formatDate(data.data.uploaded_at)}`,
            });
            await fetchConfig();
        } else {
            toast({
                title: "Upload Failed",
                description: data.data?.error || data.message || "Could not upload MOTD",
                variant: "destructive",
            });
        }
    } catch (error) {
        toast({
            title: "Error",
            description: "Failed to upload MOTD",
            variant: "destructive",
        });
    } finally {
        isUploading.value = false;
    }
};

onMounted(() => {
    fetchConfig();
    refreshPreview();
});
</script>
