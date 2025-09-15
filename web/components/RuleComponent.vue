<script setup lang="ts">
import { ref, watch, computed, onMounted } from 'vue'
import { FileText, Layers, Zap, Plus, Trash2, ChevronDown, ChevronUp, GripVertical } from 'lucide-vue-next'
import { Button } from '~/components/ui/button'

interface ServerRuleAction {
  id: string;
  rule_id: string;
  violation_count: number;
  action_type: 'WARN' | 'KICK' | 'BAN';
  duration_days?: number;
  duration_minutes?: number;
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

const props = defineProps<{
  rule: ServerRule;
  depth: number;
  sectionNumber?: number;
  subRuleIndex?: number;
  forceCollapsed?: boolean;
}>();

const emit = defineEmits<{
  update: [rule: ServerRule];
  delete: [ruleId: string];
  'add-sub-rule': [parentId: string];
  'add-action': [parentId: string];
  reorder: [payload: { ruleId: string; targetIndex: number; targetParentId?: string }];
}>();

const localRule = ref<ServerRule>({ ...props.rule });
const isCollapsed = ref<boolean>(false);

const effectivelyCollapsed = computed(() => {
  // If forceCollapsed is explicitly set (true or false), use that
  // Otherwise, use the individual rule's collapsed state
  return props.forceCollapsed !== undefined ? props.forceCollapsed : isCollapsed.value;
});

// Watch for prop changes and update local copy
watch(() => props.rule, (newRule) => {
  localRule.value = { ...newRule };
}, { deep: true });

const emitUpdate = () => {
  emit('update', localRule.value);
}

const updateChild = (updatedChild: ServerRule) => {
  if (!localRule.value.sub_rules) return;
  
  const updatedSubRules = localRule.value.sub_rules.map(subRule => 
    subRule.id === updatedChild.id ? updatedChild : subRule
  );
  
  localRule.value = { 
    ...localRule.value, 
    sub_rules: updatedSubRules,
    updated_at: new Date().toISOString()
  };
  
  emitUpdate();
}

const deleteChild = (childId: string) => {
  if (!localRule.value.sub_rules) return;
  
  const updatedSubRules = localRule.value.sub_rules.filter(subRule => subRule.id !== childId);
  
  localRule.value = { 
    ...localRule.value, 
    sub_rules: updatedSubRules,
    updated_at: new Date().toISOString()
  };
  
  emitUpdate();
}

const addChildSubRule = (childId: string) => {
  emit('add-sub-rule', childId);
}

const addChildAction = (childId: string) => {
  emit('add-action', childId);
}

const handleChildReorder = (payload: { ruleId: string; targetIndex: number; targetParentId?: string }) => {
  emit('reorder', payload);
}

const deleteAction = (actionId: string) => {
  if (!localRule.value.actions) return;
  
  const updatedActions = localRule.value.actions.filter(action => action.id !== actionId);
  
  localRule.value = { 
    ...localRule.value, 
    actions: updatedActions,
    updated_at: new Date().toISOString()
  };
  
  emitUpdate();
}

const isDragging = ref(false);
const isDragOver = ref(false);
const dragPosition = ref<'above' | 'below' | 'nested'>('above');

const handleDragStart = (event: DragEvent) => {
  console.log('Drag start triggered for rule:', localRule.value.id);
  isDragging.value = true;
  
  const dragData = JSON.stringify({
    type: 'rule',
    ruleId: localRule.value.id,
    sourceParentId: localRule.value.parent_id
  });
  
  event.dataTransfer?.setData('text/plain', dragData);
  
  // Set drag effect
  if (event.dataTransfer) {
    event.dataTransfer.effectAllowed = 'move';
    // Add a custom drag image to make it more obvious
    event.dataTransfer.setDragImage(event.currentTarget as HTMLElement, 10, 10);
  }
}

const handleDragEnd = () => {
  isDragging.value = false;
  isDragOver.value = false;
}

const handleDragOver = (event: DragEvent) => {
  event.preventDefault();
  event.stopPropagation();
  
  // Check if we have drag data by looking at the drag types
  if (!event.dataTransfer?.types.includes('text/plain')) return;
  
  isDragOver.value = true;
  
  // Calculate drop position based on mouse position
  const rect = (event.currentTarget as HTMLElement).getBoundingClientRect();
  const y = event.clientY - rect.top;
  const height = rect.height;
  
  if (y < height * 0.25) {
    dragPosition.value = 'above';
  } else if (y > height * 0.75) {
    dragPosition.value = 'below';
  } else {
    // Only allow nesting if this is not already a sub-rule
    dragPosition.value = localRule.value.parent_id ? 'below' : 'nested';
  }
  
  if (event.dataTransfer) {
    event.dataTransfer.dropEffect = 'move';
  }
}

const handleDragLeave = (event: DragEvent) => {
  // Only clear drag over if we're actually leaving this element
  const rect = (event.currentTarget as HTMLElement).getBoundingClientRect();
  const x = event.clientX;
  const y = event.clientY;
  
  if (x < rect.left || x > rect.right || y < rect.top || y > rect.bottom) {
    isDragOver.value = false;
  }
}

const handleDrop = (event: DragEvent) => {
  event.preventDefault();
  event.stopPropagation();
  
  const data = event.dataTransfer?.getData('text/plain');
  if (!data) return;
  
  try {
    const dragData = JSON.parse(data);
    
    if (dragData.type === 'rule' && dragData.ruleId !== localRule.value.id) {
      let targetIndex: number;
      let targetParentId: string | undefined;
      
      if (dragPosition.value === 'above') {
        targetIndex = localRule.value.display_order;
        targetParentId = localRule.value.parent_id === null ? undefined : localRule.value.parent_id;
      } else if (dragPosition.value === 'below') {
        targetIndex = localRule.value.display_order + 1;
        targetParentId = localRule.value.parent_id === null ? undefined : localRule.value.parent_id;
      } else { // nested
        targetIndex = localRule.value.sub_rules ? localRule.value.sub_rules.length : 0;
        targetParentId = localRule.value.id;
      }
      
      emit('reorder', {
        ruleId: dragData.ruleId,
        targetIndex: targetIndex,
        targetParentId: targetParentId
      });
    }
  } catch (error) {
    console.error('Error handling drop:', error);
  } finally {
    isDragOver.value = false;
  }
}

const ruleNumber = computed(() => {
  if (props.subRuleIndex !== undefined && props.sectionNumber !== undefined) {
    return `${props.sectionNumber}.${props.subRuleIndex}`;
  }
  return props.sectionNumber ? `${props.sectionNumber}.` : '';
});

const getRuleClasses = () => {
  return props.rule.parent_id 
    ? 'bg-secondary/10 border-secondary/20' 
    : 'bg-primary/10 border-primary/20';
}

const getIcon = () => {
  return props.rule.parent_id ? Layers : FileText;
}

const getIconColor = () => {
  return props.rule.parent_id ? 'text-secondary' : 'text-primary';
}

const getTextColor = () => {
  return 'text-foreground';
}

const getNumberColor = () => {
  return props.rule.parent_id ? 'text-secondary' : 'text-primary';
}
</script>

<template>
  <div 
    :class="[
      'border rounded-lg p-3 transition-all relative',
      getRuleClasses(),
      { 'ml-3': depth > 0 },
      { 'opacity-50': isDragging },
      { 'ring-2 ring-primary': isDragOver && dragPosition === 'nested' },
      { 'border-t-4 border-t-primary': isDragOver && dragPosition === 'above' },
      { 'border-b-4 border-b-primary': isDragOver && dragPosition === 'below' }
    ]"
    @dragover="handleDragOver"
    @dragleave="handleDragLeave"
    @drop="handleDrop"
  >
    <div class="flex items-start justify-between mb-2">
      <div class="flex items-center" style="width: 100%;">
        <!-- Drag handle: only this element is draggable so text inputs remain editable -->
        <div
          class="drag-handle cursor-grab active:cursor-grabbing hover:bg-muted/50 p-1 rounded transition-colors"
          title="Drag to reorder or nest"
          draggable="true"
          @dragstart="handleDragStart"
          @dragend="handleDragEnd"
          @mousedown="(e) => e.stopPropagation()"
        >
          <GripVertical class="h-4 w-4 text-muted-foreground" />
        </div>

        <Button
          variant="ghost"
          size="icon"
          @click="isCollapsed = !isCollapsed"
          class="h-6 w-6 mr-1 p-0.5"
          :class="{ 'opacity-50': props.forceCollapsed !== undefined }"
          title="Toggle collapse"
          :disabled="props.forceCollapsed !== undefined"
        >
          <ChevronDown v-if="effectivelyCollapsed" class="h-3 w-3" />
          <ChevronUp v-else class="h-3 w-3" />
        </Button>
        <component :is="getIcon()" :class="['w-4 h-4 mr-2']" />
        <div class="flex items-center min-w-0 flex-1">
          <span :class="['font-mono text-sm mr-2']">{{ ruleNumber }}</span>
          <input
            v-model="localRule.title"
            @input="emitUpdate"
            class="font-medium bg-transparent border-none outline-none flex-1 min-w-0 text-sm"
            :class="getTextColor()"
            placeholder="Rule title..."
          />
        </div>
      </div>
      
      <div class="flex items-center gap-1">
        <Button
          @click="$emit('add-sub-rule', rule.id)"
          variant="ghost"
          size="icon"
          title="Add Sub-Rule"
          class="h-6 w-6"
        >
          <Plus class="w-3 h-3" />
        </Button>
        
        <Button
          @click="$emit('add-action', rule.id)"
          variant="ghost"
          size="icon"
          title="Add Action"
          class="h-6 w-6"
        >
          <Zap class="w-3 h-3" />
        </Button>
        
        <Button
          @click="$emit('delete', rule.id)"
          variant="destructive"
          size="icon"
          title="Delete"
          class="h-6 w-6"
        >
          <Trash2 class="w-3 h-3" />
        </Button>
      </div>
    </div>

    <!-- Rule-specific fields -->
    <div class="space-y-2">
      <!-- Description is always shown for root rules (depth=0), but hidden for sub-rules when collapsed -->
      <div v-if="!effectivelyCollapsed || depth === 0">
        <label class="block text-xs font-medium mb-1" :class="getTextColor()">Description</label>
        <textarea
          v-model="localRule.description"
          @input="emitUpdate"
          class="w-full p-2 border border-input bg-background rounded text-xs"
          rows="2"
          placeholder="Describe this rule..."
        />
      </div>
      
      <!-- Short name field removed -->
      
      <!-- Display order field is always hidden when collapsed -->
      <!-- <div v-if="!effectivelyCollapsed">
        <label class="block text-xs font-medium mb-1" :class="getTextColor()">Display Order</label>
        <input
          v-model.number="localRule.display_order"
          @input="emitUpdate"
          type="number"
          class="w-full p-2 border border-input bg-background rounded text-xs"
          placeholder="0"
        />
      </div> -->
    </div>

    <!-- Actions -->
    <div v-if="!effectivelyCollapsed && localRule.actions && localRule.actions.length > 0" class="mt-4">
      <h3 class="text-sm font-medium mb-2" :class="getTextColor()">Actions</h3>
      <div class="space-y-2">
        <div 
          v-for="action in localRule.actions" 
          :key="action.id"
          class="bg-warning/10 border border-warning/20 rounded p-3"
        >
          <div class="flex items-center justify-between mb-2">
            <div class="flex items-center">
              <Zap class="w-4 h-4 mr-2 text-warning" />
              <span class="font-medium">Action</span>
            </div>
            <Button
              @click="deleteAction(action.id)"
              variant="destructive"
              size="icon"
              title="Delete Action"
              class="h-6 w-6"
            >
              <Trash2 class="w-3 h-3" />
            </Button>
          </div>
          
          <div class="space-y-2">
            <div>
              <label class="block text-xs font-medium mb-1">Violation Count</label>
              <input
                v-model.number="action.violation_count"
                @input="emitUpdate"
                type="number"
                min="1"
                class="w-full p-1 border border-input bg-background rounded text-sm"
                placeholder="Number of violations"
              />
            </div>
            
            <div>
              <label class="block text-xs font-medium mb-1">Action Type</label>
              <select
                v-model="action.action_type"
                @change="emitUpdate"
                class="w-full p-1 border border-input bg-background rounded text-sm"
              >
                <option value="WARN">Warning</option>
                <option value="KICK">Kick</option>
                <option value="BAN">Ban</option>
              </select>
            </div>
            
            <div v-if="action.action_type === 'BAN'">
              <label class="block text-xs font-medium mb-1">Duration (days, 0 = permanent)</label>
              <input
                v-model.number="action.duration_days"
                @input="emitUpdate"
                type="number"
                min="0"
                class="w-full p-1 border border-input bg-background rounded text-sm"
                placeholder="0 for permanent"
              />
            </div>
            
            <div>
              <label class="block text-xs font-medium mb-1">Message</label>
              <input
                v-model="action.message"
                @input="emitUpdate"
                class="w-full p-1 border border-input bg-background rounded text-sm"
                placeholder="Violation message..."
              />
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- Sub-rules -->
    <div v-if="!effectivelyCollapsed && localRule.sub_rules && localRule.sub_rules.length > 0" class="mt-3 space-y-2">
      <RuleComponent
        v-for="(subRule, index) in localRule.sub_rules"
        :key="subRule.id"
        :rule="subRule"
        :depth="depth + 1"
        :section-number="sectionNumber"
        :sub-rule-index="index + 1"
        :force-collapsed="props.forceCollapsed"
        @update="updateChild"
        @delete="deleteChild"
        @add-sub-rule="addChildSubRule"
        @add-action="addChildAction"
        @reorder="handleChildReorder"
      />
    </div>
  </div>
</template>

<style scoped>
input:focus, textarea:focus, select:focus {
  outline: none;
  box-shadow: 0 0 0 2px hsl(var(--ring));
}

[draggable="true"]:hover {
  opacity: 0.8;
}

/* Dedicated drag handle styles */
.drag-handle {
  cursor: grab;
}
.drag-handle:active {
  cursor: grabbing;
}

/* Drag over visual feedback */
.border-t-4 {
  border-top-width: 4px !important;
}

.border-b-4 {
  border-bottom-width: 4px !important;
}

/* Drag position indicators */
.drag-position-above::before {
  content: '';
  position: absolute;
  top: -2px;
  left: 0;
  right: 0;
  height: 4px;
  background: hsl(var(--primary));
  border-radius: 2px;
  z-index: 10;
}

.drag-position-below::after {
  content: '';
  position: absolute;
  bottom: -2px;
  left: 0;
  right: 0;
  height: 4px;
  background: hsl(var(--primary));
  border-radius: 2px;
  z-index: 10;
}

.drag-position-nested {
  background: hsl(var(--primary) / 0.1);
  border: 2px dashed hsl(var(--primary));
}
</style>
