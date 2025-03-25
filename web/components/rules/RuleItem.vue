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
    if (!props.rule.isExpanded) {
        emit('update:rule', { ...props.rule, isExpanded: true });
    } else {
        emit('update:rule', { ...props.rule, isExpanded: false });
    }
}
</script>

<template>
    <div class="border rounded-lg p-4 bg-card">
        <div class="flex items-start gap-2">
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

        <div v-if="rule.children && rule.children.length > 0 && rule.isExpanded" class="mt-4 ml-6">
            <draggable
                v-model="rule.children"
                item-key="id"
                handle=".drag-handle"
                @end="(evt) => onDragEnd(evt, rule.id)"
                :group="{ name: 'rules' }"
                class="space-y-2"
            >
                <template #item="childSlotProps">
                    <div
                        class="border rounded-lg p-4 bg-card"
                        @dragover.prevent
                        @drop.prevent="onNest($event, childSlotProps.element)"
                    >
                        <RuleItem
                            :rule="childSlotProps.element"
                            @update:rule="(updatedRule) => {
                                const index = rule.children.findIndex(r => r.id === updatedRule.id);
                                if (index !== -1) {
                                    rule.children[index] = updatedRule;
                                }
                            }"
                            :on-edit="onEdit"
                            :on-delete="onDelete"
                            :on-drag-end="onDragEnd"
                            :on-nest="onNest"
                        />
                    </div>
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
}
</style> 