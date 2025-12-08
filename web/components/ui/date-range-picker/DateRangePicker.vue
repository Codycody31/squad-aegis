<script setup lang="ts">
import { ref, computed } from 'vue'
import { RangeCalendar } from '~/components/ui/range-calendar'
import { Popover, PopoverContent, PopoverTrigger } from '~/components/ui/popover'
import { Button } from '~/components/ui/button'
import { Icon } from '#components'
import type { DateRange } from 'reka-ui'
import { CalendarDate, getLocalTimeZone, fromDate } from '@internationalized/date'

interface Props {
  modelValue?: { start: Date | null; end: Date | null } | null
  class?: string
}

const props = withDefaults(defineProps<Props>(), {
  modelValue: null,
})

const emits = defineEmits<{
  'update:modelValue': [value: { start: Date | null; end: Date | null } | null]
}>()

const open = ref(false)

// Convert Date to CalendarDate for RangeCalendar
const dateRangeForCalendar = computed<DateRange | null>(() => {
  if (!props.modelValue?.start) {
    return null
  }
  return {
    start: fromDate(props.modelValue.start, getLocalTimeZone()),
    end: props.modelValue.end ? fromDate(props.modelValue.end, getLocalTimeZone()) : undefined,
  }
})

// Convert CalendarDate back to Date for emission
const handleDateRangeUpdate = (value: DateRange | null) => {
  if (!value?.start) {
    emits('update:modelValue', null)
    return
  }
  
  emits('update:modelValue', {
    start: value.start.toDate(getLocalTimeZone()),
    end: value.end ? value.end.toDate(getLocalTimeZone()) : null,
  })
}

const formatDate = (date: Date | null | undefined): string => {
  if (!date) return ''
  return new Intl.DateTimeFormat('en-US', {
    month: 'short',
    day: 'numeric',
    year: 'numeric',
  }).format(date)
}

const displayValue = computed(() => {
  if (!props.modelValue?.start) {
    return 'Select date range'
  }
  if (!props.modelValue?.end) {
    return `${formatDate(props.modelValue.start)} - ...`
  }
  return `${formatDate(props.modelValue.start)} - ${formatDate(props.modelValue.end)}`
})

const applyPreset = (days: number) => {
  const end = new Date()
  const start = new Date()
  start.setDate(start.getDate() - days)
  
  emits('update:modelValue', {
    start,
    end,
  })
  open.value = false
}

const clearRange = () => {
  emits('update:modelValue', null)
  open.value = false
}
</script>

<template>
  <Popover v-model:open="open">
    <PopoverTrigger as-child>
      <Button
        variant="outline"
        :class="[
          'w-[280px] justify-start text-left font-normal',
          !modelValue?.start && 'text-muted-foreground',
          props.class,
        ]"
      >
        <Icon name="mdi:calendar-range" class="mr-2 h-4 w-4" />
        {{ displayValue }}
      </Button>
    </PopoverTrigger>
    <PopoverContent class="w-auto p-0" align="start">
      <div class="p-3">
        <div class="mb-3 space-y-1">
          <p class="text-sm font-medium">Presets</p>
          <div class="grid grid-cols-2 gap-2">
            <Button
              variant="outline"
              size="sm"
              class="text-xs"
              @click="applyPreset(7)"
            >
              Last 7 days
            </Button>
            <Button
              variant="outline"
              size="sm"
              class="text-xs"
              @click="applyPreset(30)"
            >
              Last 30 days
            </Button>
            <Button
              variant="outline"
              size="sm"
              class="text-xs"
              @click="applyPreset(60)"
            >
              Last 60 days
            </Button>
            <Button
              variant="outline"
              size="sm"
              class="text-xs"
              @click="applyPreset(90)"
            >
              Last 3 months
            </Button>
          </div>
        </div>
        <div class="border-t pt-3">
          <RangeCalendar
            :model-value="dateRangeForCalendar"
            @update:model-value="handleDateRangeUpdate"
          />
        </div>
        <div v-if="modelValue?.start" class="mt-3 flex justify-end border-t pt-3">
          <Button variant="ghost" size="sm" @click="clearRange">
            Clear
          </Button>
        </div>
      </div>
    </PopoverContent>
  </Popover>
</template>
