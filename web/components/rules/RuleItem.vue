<script setup lang="ts">
import { Button } from "~/components/ui/button";
import { ChevronRight, ChevronDown, GripVertical } from 'lucide-vue-next';
import draggable from "~/modules/vuedraggable/vuedraggable.js";

interface ServerRule {
    id: string;
    serverId: string;
    parentId: string | null;
    name: string;
    description: string | null;
    suggestedDuration: number;
    orderKey: string;
    children?: ServerRule[];
    isExpanded?: boolean;
}

const props = defineProps<{
    rule: ServerRule;
    onEdit: (rule: ServerRule) => void;
    onDelete: (ruleId: string) => void;
    onDragEnd: (evt: any, parentId: string | null) => void;
    onNest: (evt: any, targetRule: ServerRule) => void;
}>();

const emit = defineEmits<{
    (e: 'update:rule', rule: ServerRule): void;
}>();

function toggleExpand() {
    console.log("toggleExpand", props.rule);
    const updatedRule = {
        ...props.rule,
        isExpanded: !props.rule.isExpanded
    };
    console.log("Emitting updated rule:", updatedRule);
    emit('update:rule', updatedRule);
}
</script>

<template>
    <div class="border rounded-lg p-4 bg-card">
        <div class="relative">
            <div 
                class="absolute inset-0 drop-target"
                @dragover.prevent="(evt) => {
                    evt.stopPropagation();
                    const target = evt.currentTarget as HTMLElement;
                    target.classList.add('drop-target-active');
                }"
                @dragleave.prevent="(evt) => {
                    evt.stopPropagation();
                    const target = evt.currentTarget as HTMLElement;
                    target.classList.remove('drop-target-active');
                }"
                @drop.prevent="(evt) => {
                    evt.stopPropagation();
                    const target = evt.currentTarget as HTMLElement;
                    target.classList.remove('drop-target-active');
                    onNest(evt, rule);
                }"
            ></div>
            <div 
                class="flex items-start gap-2 relative"
                @dragstart.stop="(evt) => {
                    evt.dataTransfer?.setData('text/plain', rule.id);
                }"
            >
                <div class="drag-handle cursor-move mt-1">
                    <GripVertical class="h-4 w-4 text-muted-foreground" />
                </div>
                <div class="flex-1">
                    <div class="flex items-center gap-2">
                        <Button
                            v-if="rule.children && rule.children.length > 0"
                            variant="ghost"
                            size="sm"
                            class="h-6 w-6 p-0"
                            @click="toggleExpand"
                        >
                            <ChevronRight
                                v-if="!rule.isExpanded"
                                class="h-4 w-4"
                            />
                            <ChevronDown
                                v-else
                                class="h-4 w-4"
                            />
                        </Button>
                        <div class="font-medium">{{ rule.name }}</div>
                    </div>
                    <div v-if="rule.description" class="text-sm text-muted-foreground mt-1">
                        {{ rule.description }}
                    </div>
                    <div class="text-sm text-muted-foreground mt-1">
                        Duration: {{ rule.suggestedDuration === 0 ? 'Permanent' : `${rule.suggestedDuration / (24 * 60)} days` }}
                    </div>
                </div>
                <div class="flex gap-2">
                    <Button
                        variant="outline"
                        size="sm"
                        @click="onEdit(rule)"
                    >
                        Edit
                    </Button>
                    <Button
                        variant="destructive"
                        size="sm"
                        @click="onDelete(rule.id)"
                    >
                        Delete
                    </Button>
                </div>
            </div>
        </div>

        <div v-if="rule.children && rule.children.length > 0 && rule.isExpanded" class="mt-4 ml-6">
            <draggable
                v-model="rule.children"
                item-key="id"
                handle=".drag-handle"
                @end="(evt) => onDragEnd(evt, rule.id)"
                :group="{ name: 'rules' }"
                class="space-y-2"
                :animation="50"
            >
                <template #item="childSlotProps">
                    <RuleItem
                        :rule="childSlotProps.element"
                        @update:rule="(updatedRule) => {
                            if (rule.children) {
                                const index = rule.children.findIndex(r => r.id === updatedRule.id);
                                if (index !== -1) {
                                    rule.children[index] = updatedRule;
                                }
                            }
                        }"
                        :on-edit="onEdit"
                        :on-delete="onDelete"
                        :on-drag-end="onDragEnd"
                        :on-nest="onNest"
                    />
                </template>
            </draggable>
        </div>
    </div>
</template>

<style scoped>
.drag-handle {
    cursor: move;
    user-select: none;
}

.sortable-ghost {
    opacity: 0.5;
    background: #c8ebfb;
}

.sortable-drag {
    opacity: 0.9;
    background: #ffffff;
    box-shadow: 0 2px 8px rgba(0, 0, 0, 0.1);
    transform: scale(1.02);
    transition: transform 0.1s ease;
}

.drop-target {
    transition: all 0.15s ease;
    border: 2px solid transparent;
    border-radius: 0.375rem;
    pointer-events: none;
}

.drop-target-active {
    border-color: #2563eb;
    background-color: rgba(37, 99, 235, 0.1);
    pointer-events: auto;
}

.drop-target-active::after {
    content: "Drop to nest";
    position: absolute;
    top: -20px;
    left: 50%;
    transform: translateX(-50%);
    background-color: #2563eb;
    color: white;
    padding: 2px 8px;
    border-radius: 4px;
    font-size: 12px;
    pointer-events: none;
    z-index: 10;
}
</style> 