<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'
import { FileText, Layers, Download, Code, Upload, Save } from 'lucide-vue-next'
import RuleComponent from '~/components/RuleComponent.vue'
import { Button } from "~/components/ui/button"
import { Card, CardContent, CardHeader, CardTitle } from "~/components/ui/card"
import { useToast } from "~/components/ui/toast"

interface ServerRuleAction {
  id: string;
  rule_id: string;
  violation_count: number;
  action_type: 'WARN' | 'KICK' | 'BAN';
  duration_minutes?: number;
  duration_days?: number;
  message?: string;
  created_at: string;
  updated_at: string;
}

interface ServerRule {
  id: string;
  server_id: string;
  parent_id?: string | null;
  display_order: number;
  title: string;
  description?: string;
  created_at: string;
  updated_at: string;
  actions?: ServerRuleAction[];
  sub_rules?: ServerRule[];
  // Support legacy property for compatibility
  short_name?: string;
}

const route = useRoute();
const serverId = route.params.serverId as string;
const { toast } = useToast();

// state
const loading = ref<boolean>(true);
const rules = ref<ServerRule[]>([]);
const error = ref<string | null>(null);
const draggedType = ref<string>('');
const showExportDropdown = ref<boolean>(false);
const fileInput = ref<HTMLInputElement | null>(null);
const hasUnsavedChanges = ref<boolean>(false);
const isSaving = ref<boolean>(false);

// fetch rules
async function fetchRules() {
  loading.value = true;
  error.value = null;

  const runtimeConfig = useRuntimeConfig();
  const cookieToken = useCookie(runtimeConfig.public.sessionCookieName as string);
  const token = cookieToken.value;
  if (!token) {
    error.value = "Authentication required";
    loading.value = false;
    return;
  }

  try {
    const { data, error: fetchError } = await useFetch<ServerRule[]>(
      `${runtimeConfig.public.backendApi}/servers/${serverId}/rules`,
      {
        method: "GET",
        headers: {
          Authorization: `Bearer ${token}`,
        },
      }
    );

    if (fetchError.value) {
      throw new Error(fetchError.value.message || "Failed to fetch rules");
    }

    if (data.value) {
      rules.value = data.value;
      annotateNumbers();
    }
  } catch (err: any) {
    error.value = err.message || "Error fetching rules";
  } finally {
    loading.value = false;
  }
}

// Drag and drop handlers
const handleDragStart = (event: DragEvent, type: string) => {
  draggedType.value = type;
  event.dataTransfer?.setData('text/plain', type);
}

const handleDrop = (event: DragEvent) => {
  event.preventDefault();
  const type = event.dataTransfer?.getData('text/plain');
  
  if (type === 'rule' || type === 'sub-rule') {
    const newRule: ServerRule = {
      id: crypto.randomUUID(),
      server_id: serverId,
      parent_id: type === 'sub-rule' ? undefined : null,
      display_order: rules.value.length,
      title: `${type.charAt(0).toUpperCase() + type.slice(1)} ${rules.value.length + 1}`,
      description: '',
      created_at: new Date().toISOString(),
      updated_at: new Date().toISOString(),
      actions: [],
      sub_rules: []
    };
    
    rules.value.push(newRule);
  }
}

const handleFileImport = (event: Event) => {
  const target = event.target as HTMLInputElement;
  const file = target.files?.[0];
  
  if (!file) return;
  
  const reader = new FileReader();
  
  reader.onload = (e) => {
    const content = e.target?.result as string;
    
    try {
      if (file.name.endsWith('.json')) {
        // Import JSON
        const importedRules = JSON.parse(content);
        if (Array.isArray(importedRules)) {
          rules.value = importedRules.map((rule, index) => ({
            ...rule,
            id: rule.id || crypto.randomUUID(),
            display_order: index,
            server_id: serverId,
            created_at: rule.created_at || new Date().toISOString(),
            updated_at: new Date().toISOString()
          }));
          hasUnsavedChanges.value = true;
        }
      } else if (file.name.endsWith('.txt')) {
        // Import text format
        rules.value = parseTextImport(content);
        hasUnsavedChanges.value = true;
      }
    } catch (error) {
      console.error('Error importing file:', error);
      alert('Error importing file. Please check the format.');
    }
  };
  
  reader.readAsText(file);
  
  // Reset file input
  if (fileInput.value) {
    fileInput.value.value = '';
  }
}

const parseTextImport = (content: string): ServerRule[] => {
  const lines = content.split('\n').filter(line => line.trim());
  const importedRules: ServerRule[] = [];
  let currentRule: ServerRule | null = null;
  
  lines.forEach(line => {
    const trimmed = line.trim();
    
    // Main rule (e.g., "1. Respect & Conduct")
    const mainRuleMatch = trimmed.match(/^(\d+)\.\s*(.+)$/);
    if (mainRuleMatch) {
      if (currentRule) {
        importedRules.push(currentRule);
      }
      currentRule = {
        id: crypto.randomUUID(),
        server_id: serverId,
        display_order: importedRules.length,
        title: mainRuleMatch[2],
        description: '',
        created_at: new Date().toISOString(),
        updated_at: new Date().toISOString(),
        actions: [],
        sub_rules: []
      };
      return;
    }
    
    // Sub-rule (e.g., "1.1. No discrimination...")
    const subRuleMatch = trimmed.match(/^(\d+\.\d+)\.\s*(.+)$/);
    if (subRuleMatch && currentRule) {
      const subRule: ServerRule = {
        id: crypto.randomUUID(),
        server_id: currentRule.server_id,
        parent_id: currentRule.id,
        display_order: currentRule.sub_rules?.length || 0,
        title: subRuleMatch[2],
        description: '',
        created_at: new Date().toISOString(),
        updated_at: new Date().toISOString(),
        actions: [],
        sub_rules: []
      };
      if (!currentRule.sub_rules) currentRule.sub_rules = [];
      currentRule.sub_rules.push(subRule);
      return;
    }
    
    // Description (e.g., "* Description/details")
    const descriptionMatch = trimmed.match(/^\*\s*(.+)$/);
    if (descriptionMatch && currentRule && currentRule.sub_rules && currentRule.sub_rules.length > 0) {
      const lastSubRule = currentRule.sub_rules[currentRule.sub_rules.length - 1];
      lastSubRule.description = descriptionMatch[1];
      lastSubRule.updated_at = new Date().toISOString();
    }
  });
  
  if (currentRule) {
    importedRules.push(currentRule);
  }
  
  return importedRules;
}

const updateRule = (updatedRule: ServerRule) => {
  const updateRecursive = (rulesList: ServerRule[]): ServerRule[] => {
    return rulesList.map(rule => {
      if (rule.id === updatedRule.id) {
        return { ...updatedRule, updated_at: new Date().toISOString() };
      }
      if (rule.sub_rules && rule.sub_rules.length > 0) {
        return { 
          ...rule, 
          sub_rules: updateRecursive(rule.sub_rules),
          updated_at: new Date().toISOString()
        };
      }
      return rule;
    });
  };
  
  rules.value = updateRecursive(rules.value);
  hasUnsavedChanges.value = true;
}

const deleteRule = async (ruleId: string) => {
  // Find the rule to delete
  let ruleToDelete: ServerRule | null = null;
  const findRule = (rulesList: ServerRule[]): void => {
    for (const rule of rulesList) {
      if (rule.id === ruleId) {
        ruleToDelete = { ...rule };
        return;
      }
      if (rule.sub_rules && rule.sub_rules.length > 0) {
        findRule(rule.sub_rules);
        if (ruleToDelete) return;
      }
    }
  };
  
  findRule(rules.value);
  
  if (!ruleToDelete) {
    console.error("Rule not found for deletion:", ruleId);
    return;
  }
  
  // Delete from server immediately
  const runtimeConfig = useRuntimeConfig();
  const cookieToken = useCookie(runtimeConfig.public.sessionCookieName as string);
  const token = cookieToken.value;
  
  if (!token) {
    error.value = "Authentication required";
    return;
  }
  
  // Show loading state
  isSaving.value = true;
  
  try {
    const { error: fetchError } = await useFetch(
      `${runtimeConfig.public.backendApi}/servers/${serverId}/rules/${ruleId}`,
      {
        method: "DELETE",
        headers: {
          Authorization: `Bearer ${token}`,
        }
      }
    );
    
    if (fetchError.value) {
      throw new Error(fetchError.value.message || "Failed to delete rule");
    }
    
    // Now remove from local state
    const deleteRecursive = (rulesList: ServerRule[]): ServerRule[] => {
      return rulesList.filter(rule => {
        if (rule.id === ruleId) {
          return false;
        }
        if (rule.sub_rules && rule.sub_rules.length > 0) {
          rule.sub_rules = deleteRecursive(rule.sub_rules);
        }
        return true;
      });
    };
    
    rules.value = deleteRecursive(rules.value);
    
    // Show success toast
    if (ruleToDelete && typeof ruleToDelete === 'object') {
      toast({
        title: "Rule Deleted",
        description: `${(ruleToDelete as ServerRule).title} has been deleted`,
        variant: "default",
      });
    }
    
  } catch (err: any) {
    const errorMsg = err.message || "Error deleting rule";
    error.value = errorMsg;
    toast({
      title: "Error",
      description: errorMsg,
      variant: "destructive",
    });
  } finally {
    isSaving.value = false;
  }
}

const addSubRule = (parentId: string) => {
  const addRecursive = (rulesList: ServerRule[]): ServerRule[] => {
    return rulesList.map(rule => {
      if (rule.id === parentId) {
        const newSubRule: ServerRule = {
          id: crypto.randomUUID(),
          server_id: rule.server_id,
          parent_id: rule.id,
          display_order: rule.sub_rules?.length || 0,
          title: `Sub-Rule ${rule.sub_rules?.length || 0 + 1}`,
          description: '',
          created_at: new Date().toISOString(),
          updated_at: new Date().toISOString(),
          actions: [],
          sub_rules: []
        };
        const updatedSubRules = rule.sub_rules ? [...rule.sub_rules, newSubRule] : [newSubRule];
        const updatedRule = { 
          ...rule, 
          sub_rules: updatedSubRules,
          updated_at: new Date().toISOString()
        };
        return updatedRule;
      }
      if (rule.sub_rules && rule.sub_rules.length > 0) {
        return { 
          ...rule, 
          sub_rules: addRecursive(rule.sub_rules),
          updated_at: new Date().toISOString()
        };
      }
      return rule;
    });
  };
  
  rules.value = addRecursive(rules.value);
  hasUnsavedChanges.value = true;
}

const addAction = (ruleId: string) => {
  const addRecursive = (rulesList: ServerRule[]): ServerRule[] => {
    return rulesList.map(rule => {
      if (rule.id === ruleId) {
        const newAction: ServerRuleAction = {
          id: crypto.randomUUID(),
          rule_id: rule.id,
          violation_count: 1,
          action_type: 'WARN',
          message: '',
          created_at: new Date().toISOString(),
          updated_at: new Date().toISOString()
        };
        const updatedActions = rule.actions ? [...rule.actions, newAction] : [newAction];
        const updatedRule = { 
          ...rule, 
          actions: updatedActions,
          updated_at: new Date().toISOString()
        };
        return updatedRule;
      }
      if (rule.sub_rules && rule.sub_rules.length > 0) {
        return { 
          ...rule, 
          sub_rules: addRecursive(rule.sub_rules),
          updated_at: new Date().toISOString()
        };
      }
      return rule;
    });
  };
  
  rules.value = addRecursive(rules.value);
  hasUnsavedChanges.value = true;
}

const handleReorder = (payload: { ruleId: string; targetIndex: number; targetParentId?: string }) => {
  const { ruleId, targetIndex, targetParentId } = payload;
  
  // Find and remove the rule from its current position
  let ruleToMove: ServerRule | null = null;
  let sourceParentId: string | undefined | null;
  
  const findAndRemoveRule = (rulesList: ServerRule[]): ServerRule[] => {
    return rulesList.filter(rule => {
      if (rule.id === ruleId) {
        ruleToMove = { ...rule };
        sourceParentId = rule.parent_id;
        return false;
      }
      if (rule.sub_rules && rule.sub_rules.length > 0) {
        rule.sub_rules = findAndRemoveRule(rule.sub_rules);
      }
  return true;
    });
  };
  
  rules.value = findAndRemoveRule(rules.value);
  
  if (!ruleToMove) return;
  
  // TypeScript type assertion to fix type issue
  const ruleToMoveTyped = ruleToMove as ServerRule;
  
  // Update the rule's parent and display order
  ruleToMoveTyped.parent_id = targetParentId;
  ruleToMoveTyped.display_order = targetIndex;
  ruleToMoveTyped.updated_at = new Date().toISOString();
  
  // Insert the rule at the new position
  if (targetParentId) {
    // Insert as sub-rule
    const insertSubRule = (rulesList: ServerRule[]): ServerRule[] => {
      return rulesList.map(rule => {
        if (rule.id === targetParentId) {
          const newSubRules = rule.sub_rules || [];
          newSubRules.splice(targetIndex, 0, ruleToMoveTyped);
          return { 
            ...rule, 
            sub_rules: newSubRules,
            updated_at: new Date().toISOString()
          };
        }
        if (rule.sub_rules && rule.sub_rules.length > 0) {
          return { 
            ...rule, 
            sub_rules: insertSubRule(rule.sub_rules),
            updated_at: new Date().toISOString()
          };
        }
        return rule;
      });
    };
    
    rules.value = insertSubRule(rules.value);
  } else {
    // Insert as main rule
    const newRules = [...rules.value];
    newRules.splice(targetIndex, 0, ruleToMoveTyped);
    rules.value = newRules.map((rule, index) => ({
      ...rule,
      display_order: index,
      updated_at: new Date().toISOString()
    }));
  }
  
  hasUnsavedChanges.value = true;
}

async function saveRule(rule: ServerRule) {
  const runtimeConfig = useRuntimeConfig();
  const cookieToken = useCookie(runtimeConfig.public.sessionCookieName as string);
  const token = cookieToken.value;
  if (!token) {
    error.value = "Authentication required";
    return;
  }

  try {
    let url = `${runtimeConfig.public.backendApi}/servers/${serverId}/rules`;
    if (rule.id) {
      url += `/${rule.id}`;
    }

    const { error: fetchError } = await useFetch(
      url,
      {
        method: rule.id ? "PUT" : "POST",
        headers: {
          Authorization: `Bearer ${token}`,
        },
        body: {
          parent_id: rule.parent_id,
          display_order: rule.display_order,
          title: rule.title,
          description: rule.description
        },
      }
    );
    
    if (fetchError.value) {
      throw new Error(fetchError.value.message || "Failed to save rule");
    }
  } catch (err: any) {
    error.value = err.message || "Error saving rule";
  }
}

async function deleteServerRule(ruleId: string) {
  const runtimeConfig = useRuntimeConfig();
  const cookieToken = useCookie(runtimeConfig.public.sessionCookieName as string);
  const token = cookieToken.value;
  if (!token) {
    error.value = "Authentication required";
    return;
  }

  try {
    const { error: fetchError } = await useFetch(
      `${runtimeConfig.public.backendApi}/servers/${serverId}/rules/${ruleId}`,
      {
        method: "DELETE",
        headers: {
          Authorization: `Bearer ${token}`,
        },
      }
    );
    
    if (fetchError.value) {
      throw new Error(fetchError.value.message || "Failed to delete rule");
    }
  } catch (err: any) {
    error.value = err.message || "Error deleting rule";
  }
}

// Save all rules at once using the bulk update endpoint
async function saveAllRules() {
  if (!hasUnsavedChanges.value) {
    return; // No changes to save
  }

  isSaving.value = true;
  error.value = null;
  
  const runtimeConfig = useRuntimeConfig();
  const cookieToken = useCookie(runtimeConfig.public.sessionCookieName as string);
  const token = cookieToken.value;
  
  if (!token) {
    error.value = "Authentication required";
    isSaving.value = false;
    return;
  }

  // Flatten rules for bulk update, preserving the hierarchy
  const flatRules = flattenRules(rules.value);

  try {
    const { data, error: fetchError } = await useFetch(
      `${runtimeConfig.public.backendApi}/servers/${serverId}/rules/bulk`,
      {
        method: "PUT",
        headers: {
          Authorization: `Bearer ${token}`,
          "Content-Type": "application/json",
        },
        body: flatRules
      }
    );

    if (fetchError.value) {
      throw new Error(fetchError.value.message || "Failed to save rules");
    }

    // Success - refresh rules from server
    hasUnsavedChanges.value = false;
    await fetchRules();

    // Show success toast
    toast({
      title: "Rules Saved",
      description: "All rule changes have been saved successfully",
      variant: "default",
    });
    
  } catch (err: any) {
    error.value = err.message || "Error saving rules";
  } finally {
    isSaving.value = false;
  }
}

// Helper function to flatten the rule hierarchy for bulk save
function flattenRules(rulesArray: ServerRule[]): ServerRule[] {
  let result: ServerRule[] = [];

  for (const rule of rulesArray) {
    // Add the main rule
    result.push({
      ...rule,
      // Remove sub_rules as it will be processed separately
      sub_rules: undefined
    });

    // If there are sub-rules, add them too
    if (rule.sub_rules && rule.sub_rules.length > 0) {
      result = [...result, ...rule.sub_rules];
    }
  }

  return result;
}

async function saveRuleOrder() {
  const runtimeConfig = useRuntimeConfig();
  const cookieToken = useCookie(runtimeConfig.public.sessionCookieName as string);
  const token = cookieToken.value;
  if (!token) return;

  // flatten rules with their new order
  const queue: { rule: ServerRule; parent_id: string | null; order: number }[] = [];
  rules.value.forEach((r, idx) => {
    queue.push({ rule: r, parent_id: null, order: idx });
    if (r.sub_rules) {
      r.sub_rules.forEach((sr, sidx) => {
      queue.push({ rule: sr, parent_id: r.id, order: sidx });
    });
    }
  });

  try {
    await Promise.all(
      queue.map(async ({ rule, parent_id, order }) => {
        const { error: fetchError } = await useFetch(
          `${runtimeConfig.public.backendApi}/servers/${serverId}/rules/${rule.id}`,
          {
            method: "PUT",
            headers: {
              Authorization: `Bearer ${token}`,
            },
            body: {
              parent_id,
              display_order: order,
              title: rule.title,
              description: rule.description,
            },
          }
        );
        if (fetchError.value) {
          console.error("Failed updating rule", rule.id);
        }
      })
    );
    fetchRules();
  } catch (e) {
    console.error(e);
  }
}

const exportAsText = () => {
  const text = generateTextExport();
  const blob = new Blob([text], { type: 'text/plain' });
  const url = URL.createObjectURL(blob);
  const a = document.createElement('a');
  a.href = url;
  a.download = 'server-rules.txt';
  a.click();
  URL.revokeObjectURL(url);
  showExportDropdown.value = false;
}

const exportAsJson = () => {
  const json = JSON.stringify(rules.value, null, 2);
  const blob = new Blob([json], { type: 'application/json' });
  const url = URL.createObjectURL(blob);
  const a = document.createElement('a');
  a.href = url;
  a.download = 'server-rules.json';
  a.click();
  URL.revokeObjectURL(url);
  showExportDropdown.value = false;
}

const generateTextExport = (): string => {
  let text = '';
  
  const processRule = (rule: ServerRule, number: string): void => {
    // Main rule title
    text += `${number}. ${rule.title}\n`;
    
    // Add description for main rule if it exists
    if (rule.description && rule.description.trim()) {
      text += `    * ${rule.description}\n`;
    }
    
    // Process sub-rules with proper indentation
    if (rule.sub_rules) {
      rule.sub_rules.forEach((subRule, index) => {
        const subNumber = `${number}.${index + 1}`;
        text += `    ${subNumber}. ${subRule.title}\n`;
        
        // Add description with asterisk if it exists
        if (subRule.description && subRule.description.trim()) {
          text += `        * ${subRule.description}\n`;
        }
      });
    }
    
    // Add empty line after each main rule section
    text += '\n';
  };
  
  rules.value.forEach((rule, index) => {
    processRule(rule, `${index + 1}`);
  });
  
  return text.trim();
}

// Close dropdown when clicking outside
const handleClickOutside = (event: MouseEvent) => {
  const target = event.target as HTMLElement;
  if (!target.closest('.relative')) {
    showExportDropdown.value = false;
  }
}

const annotateNumbers = () => {
  rules.value.forEach((rule, idx) => {
    (rule as any).displayId = (idx + 1).toString();
    if (rule.sub_rules) {
      rule.sub_rules.forEach((sub, sidx) => {
      (sub as any).displayId = `${idx + 1}.${sidx + 1}`;
    });
    }
  });
};

onMounted(() => {
  fetchRules();
  document.addEventListener('click', handleClickOutside);
});

onUnmounted(() => {
  document.removeEventListener('click', handleClickOutside);
});
</script>

<template>
  <div class="rules-builder p-6 max-w-6xl mx-auto">
    <div class="mb-6">
      <div class="flex flex-col md:flex-row md:items-center md:justify-between gap-4">
        <div>
          <h1 class="text-2xl font-bold mb-2">Rules Builder</h1>
          <p class="text-muted-foreground">Drag and drop to create complex rule sets with numbered sections and subsections</p>
        </div>
        
        <div class="flex items-center gap-2">
          <!-- Import Button -->
          <div class="relative">
            <input
              type="file"
              ref="fileInput"
              @change="handleFileImport"
              accept=".json,.txt"
              class="hidden"
            />
            <Button
              @click="fileInput?.click()"
              class="flex items-center"
            >
              <Upload class="w-4 h-4 mr-2" />
              Import Rules
            </Button>
          </div>
          
          <!-- Save Button -->
          <Button
            @click="saveAllRules"
            :disabled="isSaving || !hasUnsavedChanges"
            :variant="hasUnsavedChanges ? 'default' : 'outline'"
            class="flex items-center"
          >
            <span v-if="isSaving" class="h-4 w-4 mr-2 animate-spin">
              <svg class="animate-spin" viewBox="0 0 24 24" fill="none">
                <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
                <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
              </svg>
            </span>
            <Save v-else class="h-4 w-4 mr-2" />
            {{ isSaving ? 'Saving...' : 'Save Changes' }}
          </Button>
          
          <!-- Export Dropdown -->
          <div class="relative">
            <Button
              @click="showExportDropdown = !showExportDropdown"
              variant="secondary"
              class="flex items-center"
            >
              <Download class="w-4 h-4 mr-2" />
              Export Rules
            </Button>
            
            <div 
              v-if="showExportDropdown"
              class="absolute right-0 mt-2 w-48 bg-background border rounded-lg shadow-lg z-10"
            >
              <button
                @click="exportAsText"
                class="w-full text-left px-4 py-2 hover:bg-accent flex items-center"
              >
                <FileText class="w-4 h-4 mr-2" />
                Export as Text
              </button>
              <button
                @click="exportAsJson"
                class="w-full text-left px-4 py-2 hover:bg-accent flex items-center"
              >
                <Code class="w-4 h-4 mr-2" />
                Export as JSON
              </button>
            </div>
          </div>
        </div>
      </div>
    </div>

    <div v-if="error" class="bg-destructive text-destructive-foreground p-4 rounded mb-4">{{ error }}</div>
    <div v-if="loading" class="text-center py-8">
      <div class="animate-spin h-8 w-8 border-4 border-primary border-t-transparent rounded-full mx-auto mb-4"></div>
      <p>Loading rules...</p>
    </div>

    <div v-else class="grid grid-cols-1 lg:grid-cols-3 gap-6">
      <!-- Rule Components Panel -->
      <Card>
        <CardHeader>
          <CardTitle>Rule Components</CardTitle>
        </CardHeader>
        <CardContent>
          <div class="space-y-3">
            <div 
              draggable="true"
              @dragstart="handleDragStart($event, 'rule')"
              class="p-3 bg-primary/10 border border-primary/20 rounded cursor-move hover:bg-primary/15 transition-colors"
            >
              <div class="flex items-center">
                <FileText class="w-4 h-4 mr-2 text-primary" />
                <span class="font-medium">Rule Section</span>
              </div>
              <p class="text-sm text-muted-foreground mt-1">Main rule section (e.g., "1. Respect & Conduct")</p>
    </div>

            <div 
              draggable="true"
              @dragstart="handleDragStart($event, 'sub-rule')"
              class="p-3 bg-secondary/10 border border-secondary/20 rounded cursor-move hover:bg-secondary/15 transition-colors"
            >
              <div class="flex items-center">
                <Layers class="w-4 h-4 mr-2 text-secondary" />
                <span class="font-medium">Sub-Rule</span>
              </div>
              <p class="text-sm text-muted-foreground mt-1">Specific rule (e.g., "1.1 No discrimination...")</p>
            </div>
          </div>
        </CardContent>
      </Card>

      <!-- Rules Canvas -->
      <Card class="lg:col-span-2 min-h-[600px]">
        <CardHeader>
          <CardTitle>Rules Canvas</CardTitle>
            </CardHeader>
            <CardContent>
          <div 
            @dragover.prevent
            @drop="handleDrop($event)"
            class="border-2 border-dashed border-border rounded-lg p-4 min-h-[500px] bg-muted/50"
          >
            <div v-if="rules.length === 0" class="text-center text-muted-foreground mt-20">
              <p>Drag rule components here to start building</p>
            </div>
          
            <div v-else class="space-y-4">
              <RuleComponent
                v-for="(rule, index) in rules"
                :key="rule.id"
                :rule="rule"
                :depth="0"
                :section-number="index + 1"
                @update="updateRule"
                @delete="deleteRule"
                @add-sub-rule="addSubRule"
                @add-action="addAction"
                @reorder="handleReorder"
              />
            </div>
                  </div>
            </CardContent>
          </Card>
    </div>

    <!-- Preview Panel -->
    <Card class="mt-6">
      <CardHeader>
        <CardTitle>Rules Preview</CardTitle>
        <div class="text-muted-foreground text-sm">Text format (as it will appear when exported)</div>
      </CardHeader>
      <CardContent>
        <pre class="bg-muted p-4 rounded text-sm overflow-auto font-mono whitespace-pre-wrap">{{ generateTextExport() }}</pre>
      </CardContent>
    </Card>
  </div>
</template>

<style scoped>
.rules-builder {
  font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
}
</style>