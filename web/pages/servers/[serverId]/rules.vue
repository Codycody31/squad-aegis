<script setup lang="ts">
import { ref, onMounted, onUnmounted, computed } from 'vue'
import { onBeforeRouteLeave } from 'vue-router'
import { FileText, Layers, Download, Code, Upload, Save, ChevronDown, ChevronUp, Minimize2, Maximize2, Plus } from 'lucide-vue-next'
import RuleComponent from '~/components/RuleComponent.vue'
import { Button } from "~/components/ui/button"
import { Card, CardContent, CardHeader, CardTitle } from "~/components/ui/card"
import { useToast } from "~/components/ui/toast"
import { useAuthStore } from "~/stores/auth"

interface ServerRuleAction {
  id: string;
  rule_id: string;
  violation_count: number;
  action_type: 'WARN' | 'KICK' | 'BAN';
  duration?: number; // Duration in days (from backend)
  duration_days?: number; // Duration in days (for frontend use)
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
const authStore = useAuthStore();

// Check if user has manageserver permission
const canManage = computed(() => {
  const user = authStore.user;
  // Super admins have all permissions
  if (user?.super_admin) {
    return true;
  }
  
  const serverPerms = authStore.getServerPermissions(serverId);
  if (!serverPerms || serverPerms.length === 0) {
    return false;
  }
  
  // Check if user has manageserver permission or wildcard permission
  return serverPerms.includes('manageserver') || serverPerms.includes('*');
});

// state
const loading = ref<boolean>(true);
const rules = ref<ServerRule[]>([]);
const error = ref<string | null>(null);
const showExportDropdown = ref<boolean>(false);
const fileInput = ref<HTMLInputElement | null>(null);
const hasUnsavedChanges = ref<boolean>(false);
const isSaving = ref<boolean>(false);
const previewCollapsed = ref<boolean>(true);
const rulesCollapsed = ref<boolean>(false);
const deletedRuleIds = ref<string[]>([]); // Track deleted rule IDs

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
      // Map duration from backend to duration_days for frontend use
      const mapDurationToDays = (rulesArray: ServerRule[]): ServerRule[] => {
        return rulesArray.map(rule => {
          const mappedRule = { ...rule };
          
          // Map actions duration (from backend duration to frontend duration_days)
          if (mappedRule.actions) {
            mappedRule.actions = mappedRule.actions.map(action => {
              const mappedAction = { ...action };
              if (mappedAction.duration !== undefined && mappedAction.duration !== null) {
                mappedAction.duration_days = mappedAction.duration;
              }
              // Remove duration field after mapping to avoid confusion
              delete mappedAction.duration;
              return mappedAction;
            });
          }
          
          // Recursively map sub_rules
          if (mappedRule.sub_rules && mappedRule.sub_rules.length > 0) {
            mappedRule.sub_rules = mapDurationToDays(mappedRule.sub_rules);
          }
          
          return mappedRule;
        });
      };
      
      rules.value = mapDurationToDays(data.value);
      annotateNumbers();
      // Clear deleted rule IDs after successful fetch
      deletedRuleIds.value = [];
    }
  } catch (err: any) {
    error.value = err.message || "Error fetching rules";
  } finally {
    loading.value = false;
  }
}

const addNewRule = (type: 'rule') => {
  const newRule: ServerRule = {
    id: crypto.randomUUID(),
    server_id: serverId,
    parent_id: null,
    display_order: rules.value.length,
    title: `Rule ${rules.value.length + 1}`,
    description: '',
    created_at: new Date().toISOString(),
    updated_at: new Date().toISOString(),
    actions: [],
    sub_rules: []
  };
  
  rules.value.push(newRule);
  hasUnsavedChanges.value = true;
  annotateNumbers();
}

const handleDrop = (event: DragEvent) => {
  event.preventDefault();
  
  // This is now only for reordering existing rules
  // New rules are added via the buttons
}

const handleCanvasDragOver = (event: DragEvent) => {
  event.preventDefault();
  
  // Check if we have valid drag data
  if (event.dataTransfer?.types.includes('text/plain')) {
    event.dataTransfer.dropEffect = 'copy';
  }
}

const handleCanvasDragLeave = (event: DragEvent) => {
  // No longer needed since we removed the visual feedback
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
          // Clear deleted IDs since import is a fresh start
          deletedRuleIds.value = [];
          hasUnsavedChanges.value = true;
        }
      } else if (file.name.endsWith('.txt')) {
        // Import text format
        rules.value = parseTextImport(content);
        // Clear deleted IDs since import is a fresh start
        deletedRuleIds.value = [];
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

const deleteRule = (ruleId: string) => {
  // Find the rule to delete (for toast message)
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
  
  // Helper to collect all rule IDs including nested sub-rules
  const collectRuleIds = (rule: ServerRule): string[] => {
    const ids = [rule.id];
    if (rule.sub_rules && rule.sub_rules.length > 0) {
      rule.sub_rules.forEach(subRule => {
        ids.push(...collectRuleIds(subRule));
      });
    }
    return ids;
  };
  
  // Add this rule and all its sub-rules to the deletion list
  const idsToDelete = collectRuleIds(ruleToDelete);
  deletedRuleIds.value.push(...idsToDelete);
  
  // Remove from local state
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
  hasUnsavedChanges.value = true;
  
  // Show info toast
  toast({
    title: "Rule Marked for Deletion",
    description: `${ruleToDelete.title} will be deleted when you save`,
    variant: "default",
  });
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
  let sourceParentId: string | undefined | null = undefined;
  
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
  
  // TypeScript assertion to fix type issues
  const moveableRule = ruleToMove as ServerRule;
  
  // Prevent dropping a rule into itself or its descendants
  if (targetParentId && isDescendant(moveableRule.id, targetParentId, rules.value)) {
    // Restore the rule to its original position
    insertRuleAtPosition(moveableRule, sourceParentId || undefined);
    return;
  }
  
  // Update the rule's parent
  moveableRule.parent_id = targetParentId || null;
  moveableRule.updated_at = new Date().toISOString();
  
  // Insert the rule at the new position
  insertRuleAtPosition(moveableRule, targetParentId, targetIndex);
  
  // Re-normalize display orders
  normalizeDisplayOrders();
  
  hasUnsavedChanges.value = true;
}

// Helper function to check if a rule is a descendant of another
const isDescendant = (ancestorId: string, potentialDescendantId: string, rulesList: ServerRule[]): boolean => {
  for (const rule of rulesList) {
    if (rule.id === potentialDescendantId) {
      if (rule.parent_id === ancestorId) {
        return true;
      }
      if (rule.parent_id) {
        return isDescendant(ancestorId, rule.parent_id, rulesList);
      }
    }
    if (rule.sub_rules && rule.sub_rules.length > 0) {
      if (isDescendant(ancestorId, potentialDescendantId, rule.sub_rules)) {
        return true;
      }
    }
  }
  return false;
}

// Helper function to insert a rule at a specific position
const insertRuleAtPosition = (ruleToInsert: ServerRule, parentId?: string, index?: number) => {
  if (parentId) {
    // Insert as sub-rule
    const insertSubRule = (rulesList: ServerRule[]): ServerRule[] => {
      return rulesList.map(rule => {
        if (rule.id === parentId) {
          const newSubRules = rule.sub_rules || [];
          const insertIndex = index !== undefined ? Math.min(index, newSubRules.length) : newSubRules.length;
          newSubRules.splice(insertIndex, 0, ruleToInsert);
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
    const insertIndex = index !== undefined ? Math.min(index, rules.value.length) : rules.value.length;
    rules.value.splice(insertIndex, 0, ruleToInsert);
  }
}

// Helper function to normalize display orders after reordering
const normalizeDisplayOrders = () => {
  // Normalize main rules
  rules.value.forEach((rule, index) => {
    rule.display_order = index;
    rule.updated_at = new Date().toISOString();
    
    // Normalize sub-rules
    if (rule.sub_rules && rule.sub_rules.length > 0) {
      rule.sub_rules.forEach((subRule, subIndex) => {
        subRule.display_order = subIndex;
        subRule.updated_at = new Date().toISOString();
      });
    }
  });
  
  // Re-annotate numbers
  annotateNumbers();
}

// Save all rules at once using the bulk update endpoint
async function saveAllRules() {
  if (!hasUnsavedChanges.value && deletedRuleIds.value.length === 0) {
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
        body: {
          rules: flatRules,
          deleted_rule_ids: deletedRuleIds.value
        }
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
    // Transform actions: map duration_days to duration_days for backend
    const transformedActions = rule.actions ? rule.actions.map(action => {
      const transformedAction: any = {
        ...action,
      };
      // Map duration_days to duration_days for backend (backend expects duration_days key)
      if (transformedAction.duration_days !== undefined) {
        transformedAction.duration_days = transformedAction.duration_days;
        // Remove duration if it exists (keep duration_days)
        delete transformedAction.duration;
      } else if (transformedAction.duration !== undefined) {
        // Fallback: use duration if duration_days doesn't exist
        transformedAction.duration_days = transformedAction.duration;
        delete transformedAction.duration;
      }
      return transformedAction;
    }) : undefined;

    // Add the main rule
    const transformedRule: any = {
      ...rule,
      actions: transformedActions,
      // Remove sub_rules as it will be processed separately
      sub_rules: undefined
    };
    result.push(transformedRule);

    // If there are sub-rules, add them too (recursively)
    if (rule.sub_rules && rule.sub_rules.length > 0) {
      result = [...result, ...flattenRules(rule.sub_rules)];
    }
  }

  return result;
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
  
  const processRule = (rule: ServerRule, number: string, depth: number = 0): void => {
    // Calculate indentation based on depth (4 spaces per level)
    const indent = '    '.repeat(depth);
    
    // Rule title with proper indentation
    text += `${indent}${number}. ${rule.title}\n`;
    
    // Add description for rule if it exists (with extra indentation)
    if (rule.description && rule.description.trim()) {
      text += `${indent}    * ${rule.description}\n`;
    }
    
    // Recursively process sub-rules with proper numbering and indentation
    if (rule.sub_rules && rule.sub_rules.length > 0) {
      rule.sub_rules.forEach((subRule, index) => {
        const subNumber = `${number}.${index + 1}`;
        processRule(subRule, subNumber, depth + 1);
      });
    }
    
    // Add empty line after each top-level rule section (depth 0)
    if (depth === 0) {
      text += '\n';
    }
  };
  
  rules.value.forEach((rule, index) => {
    processRule(rule, `${index + 1}`, 0);
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

const toggleCollapseAll = () => {
  rulesCollapsed.value = !rulesCollapsed.value;
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

// Warn user before leaving page with unsaved changes
const hasUnsavedChangesToWarn = computed(() => {
  return hasUnsavedChanges.value || deletedRuleIds.value.length > 0;
});

// Handle browser navigation (refresh, close tab, etc.)
const handleBeforeUnload = (event: BeforeUnloadEvent) => {
  if (hasUnsavedChangesToWarn.value) {
    event.preventDefault();
    // Modern browsers ignore custom messages and show their own
    event.returnValue = '';
    return '';
  }
};

// Handle Vue Router navigation (within the app)
onBeforeRouteLeave((to, from, next) => {
  if (hasUnsavedChangesToWarn.value) {
    const confirmed = confirm(
      'You have unsaved changes. Are you sure you want to leave? Your changes will be lost.'
    );
    if (confirmed) {
      next();
    } else {
      next(false);
    }
  } else {
    next();
  }
});

onMounted(() => {
  fetchRules();
  document.addEventListener('click', handleClickOutside);
  window.addEventListener('beforeunload', handleBeforeUnload);
});

onUnmounted(() => {
  document.removeEventListener('click', handleClickOutside);
  window.removeEventListener('beforeunload', handleBeforeUnload);
});
</script>

<template>
  <div class="rules-builder p-6 max-w-6xl mx-auto">
    <!-- Header -->
    <div class="mb-6">
      <div class="flex flex-col md:flex-row md:items-center md:justify-between gap-4">
        <div>
          <h1 class="text-2xl font-bold mb-2">Rules Builder</h1>
          <p class="text-muted-foreground">Create and manage server rules with numbered sections and subsections</p>
        </div>
        
        <div class="flex items-center gap-2">
          <!-- Import Button -->
          <div v-if="canManage" class="relative">
            <input
              type="file"
              ref="fileInput"
              @change="handleFileImport"
              accept=".json,.txt"
              class="hidden"
            />
            <Button
              @click="fileInput?.click()"
              variant="outline"
              size="sm"
              class="flex items-center"
            >
              <Upload class="w-4 h-4 mr-2" />
              Import
            </Button>
          </div>
          
          <!-- Save Button -->
          <Button
            v-if="canManage"
            @click="saveAllRules"
            :disabled="isSaving || (!hasUnsavedChanges && deletedRuleIds.length === 0)"
            :variant="(hasUnsavedChanges || deletedRuleIds.length > 0) ? 'default' : 'outline'"
            size="sm"
            class="flex items-center"
          >
            <span v-if="isSaving" class="h-4 w-4 mr-2 animate-spin">
              <svg class="animate-spin" viewBox="0 0 24 24" fill="none">
                <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
                <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
              </svg>
            </span>
            <Save v-else class="h-4 w-4 mr-2" />
            {{ isSaving ? 'Saving...' : 'Save' }}
          </Button>
          
          <!-- Export Dropdown -->
          <div class="relative">
            <Button
              @click="showExportDropdown = !showExportDropdown"
              variant="outline"
              size="sm"
              class="flex items-center"
            >
              <Download class="w-4 h-4 mr-2" />
              Export
            </Button>
            
            <div 
              v-if="showExportDropdown"
              class="absolute right-0 mt-2 w-40 bg-background border rounded-lg shadow-lg z-10"
            >
              <button
                @click="exportAsText"
                class="w-full text-left px-3 py-2 hover:bg-accent flex items-center text-sm"
              >
                <FileText class="w-4 h-4 mr-2" />
                Text
              </button>
              <button
                @click="exportAsJson"
                class="w-full text-left px-3 py-2 hover:bg-accent flex items-center text-sm"
              >
                <Code class="w-4 h-4 mr-2" />
                JSON
              </button>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- Preview Panel (moved to top, collapsed by default) -->
    <Card class="mb-6">
      <CardHeader>
        <div class="flex items-center justify-between">
          <CardTitle class="text-lg">Rules Preview</CardTitle>
          <Button
            @click="previewCollapsed = !previewCollapsed"
            variant="ghost"
            size="sm"
            class="flex items-center"
          >
            <ChevronDown v-if="previewCollapsed" class="h-4 w-4 mr-1" />
            <ChevronUp v-else class="h-4 w-4 mr-1" />
            {{ previewCollapsed ? 'Show' : 'Hide' }}
          </Button>
        </div>
      </CardHeader>
      <CardContent v-if="!previewCollapsed">
        <pre class="bg-muted p-4 rounded text-sm overflow-auto font-mono whitespace-pre-wrap max-h-96">{{ generateTextExport() }}</pre>
      </CardContent>
    </Card>

    <div v-if="error" class="bg-destructive text-destructive-foreground p-4 rounded mb-4">{{ error }}</div>
    <div v-if="loading" class="text-center py-8">
      <div class="animate-spin h-8 w-8 border-4 border-primary border-t-transparent rounded-full mx-auto mb-4"></div>
      <p>Loading rules...</p>
    </div>

    <div v-else class="flex justify-center">
      <!-- Rules Canvas (full width) -->
      <Card class="w-full max-w-6xl min-h-[600px]">
        <CardHeader class="pb-4">
          <div class="flex items-center justify-between">
            <CardTitle class="text-lg">Rules Canvas</CardTitle>
            <div class="flex items-center gap-2">
              <!-- Add Rule Button -->
              <Button
                v-if="canManage"
                @click="addNewRule('rule')"
                variant="outline"
                size="sm"
                class="flex items-center"
              >
                <Plus class="h-4 w-4 mr-1" />
                Add Rule
              </Button>
              
              <Button
                @click="toggleCollapseAll"
                variant="outline"
                size="sm"
                class="flex items-center"
              >
                <Minimize2 v-if="!rulesCollapsed" class="h-4 w-4 mr-1" />
                <Maximize2 v-else class="h-4 w-4 mr-1" />
                {{ rulesCollapsed ? 'Expand All' : 'Collapse All' }}
              </Button>
            </div>
          </div>
        </CardHeader>
        <CardContent>
          <div class="border-2 border-dashed border-border rounded-lg p-4 min-h-[500px] bg-muted/50">
            <div v-if="rules.length === 0" class="text-center text-muted-foreground mt-20">
              <div>
                <p class="text-lg mb-2">Start building your server rules</p>
                <p class="text-sm mb-4">Use the "Add Rule" button above to create your first rule</p>
                <div class="mt-4 text-xs space-y-1 max-w-md mx-auto">
                  <p>• <strong>Drag Handle</strong>: Use the grip icon (⋮⋮) to reorder rules</p>
                  <p>• <strong>Sub-Rules</strong>: Use the + button on any rule to add sub-rules</p>
                  <p>• <strong>Nesting</strong>: Drag a rule onto another to make it a sub-rule</p>
                  <p>• <strong>Actions</strong>: Use the ⚡ button to add enforcement actions</p>
                </div>
              </div>
            </div>
          
            <div v-else class="space-y-4">
              <RuleComponent
                v-for="(rule, index) in rules"
                :key="rule.id"
                :rule="rule"
                :depth="0"
                :section-number="index + 1"
                :force-collapsed="rulesCollapsed ? true : undefined"
                :read-only="!canManage"
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
  </div>
</template>

<style scoped>
.rules-builder {
  font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
}

/* Drag and drop enhancements */
[draggable="true"] {
  transition: opacity 0.2s ease;
}

[draggable="true"]:hover {
  opacity: 0.9;
}

.cursor-move:hover {
  transform: scale(1.05);
  transition: transform 0.15s ease;
}

/* Canvas drop zone animations */
.border-dashed {
  transition: all 0.2s ease;
}

/* Rule drag feedback */
.drag-feedback {
  animation: pulse 1s infinite;
}

@keyframes pulse {
  0%, 100% { opacity: 1; }
  50% { opacity: 0.7; }
}
</style>