<script setup>
import { computed, nextTick, onMounted, ref, watch } from 'vue'

import { addDays, dayOfMonth, monthShort, weekdayShort } from '@/utils/format'

// A horizontal strip of consecutive days, starting at the venue's today.
const props = defineProps({
  modelValue: { type: String, default: '' },
  start: { type: String, required: true },
  days: { type: Number, default: 31 },
})
const emit = defineEmits(['update:modelValue'])

const list = computed(() => Array.from({ length: props.days }, (_, i) => addDays(props.start, i)))
const strip = ref(null)

function isMonthStart(d, i) {
  return i === 0 || dayOfMonth(d) === 1
}

function scrollToSelected() {
  nextTick(() => strip.value?.querySelector('[aria-pressed="true"]')?.scrollIntoView({ inline: 'center', block: 'nearest' }))
}
onMounted(scrollToSelected)
watch(() => props.modelValue, scrollToSelected)
</script>

<template>
  <div ref="strip" class="strip" role="group" aria-label="Date">
    <button
      v-for="(d, i) in list"
      :key="d"
      type="button"
      class="day"
      :aria-pressed="d === modelValue"
      @click="emit('update:modelValue', d)"
    >
      <small>{{ isMonthStart(d, i) ? monthShort(d) : weekdayShort(d) }}</small>
      <b>{{ dayOfMonth(d) }}</b>
    </button>
  </div>
</template>

<style scoped>
.strip {
  display: flex;
  gap: 8px;
  overflow-x: auto;
  margin: 0 -16px;
  padding: 2px 16px 6px;
  scroll-snap-type: x proximity;
  scrollbar-width: none;
}
.strip::-webkit-scrollbar {
  display: none;
}
.day {
  flex: 0 0 auto;
  width: 54px;
  padding: 8px 0;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 2px;
  border: 1px solid var(--separator);
  border-radius: var(--radius-sm);
  background: var(--section-bg);
  color: var(--text);
  font: inherit;
  cursor: pointer;
  scroll-snap-align: start;
}
.day small {
  font-size: 0.72rem;
  color: var(--hint);
}
.day b {
  font-size: 1.15rem;
}
.day[aria-pressed='true'] {
  background: var(--button);
  border-color: var(--button);
  color: var(--button-text);
}
.day[aria-pressed='true'] small {
  color: inherit;
  opacity: 0.85;
}
</style>
