<script setup>
import { plural } from '@/utils/format'

// Draws the venue's plan (SVG, viewBox 1000x700, no tables) and puts each table
// on top at x% / y% of the plan, as the API contract describes.
const props = defineProps({
  planUrl: { type: String, default: '' },
  tables: { type: Array, required: true },
  modelValue: { type: Number, default: null },
  guests: { type: Number, default: 1 },
  readonly: { type: Boolean, default: false },
})
const emit = defineEmits(['update:modelValue'])

// Pin size grows with seats; the minimum keeps every pin a comfortable tap target.
function pinStyle(t) {
  const w = t.shape === 'round' ? 5.5 + t.seats * 0.75 : 7 + t.seats * 1.15
  return {
    left: `${t.x}%`,
    top: `${t.y}%`,
    '--w': `${w}%`,
    '--ratio': t.shape === 'round' ? 1 : 1.55,
  }
}

function label(t) {
  const state = { free: 'free', taken: 'booked', too_small: `too small for ${plural(props.guests, 'guest')}` }
  return `Table ${t.label}, ${plural(t.seats, 'seat')}, ${state[t.status]}`
}

function pick(t) {
  if (props.readonly || t.status !== 'free') return
  emit('update:modelValue', t.id === props.modelValue ? null : t.id)
}
</script>

<template>
  <div class="floor">
    <div class="plan">
      <img v-if="planUrl" :src="planUrl" alt="" draggable="false" />
      <button
        v-for="t in tables"
        :key="t.id"
        type="button"
        class="pin"
        :class="[t.shape, t.status, { selected: t.id === modelValue, ro: readonly }]"
        :style="pinStyle(t)"
        :disabled="readonly || t.status !== 'free'"
        :aria-label="label(t)"
        :aria-pressed="readonly ? undefined : t.id === modelValue"
        @click="pick(t)"
      >
        <b>{{ t.label }}</b>
        <small>{{ t.seats }}</small>
      </button>
    </div>
    <ul class="legend" aria-hidden="true">
      <li><i class="dot free" /> Free</li>
      <li><i class="dot taken" /> Booked</li>
      <li v-if="!readonly"><i class="dot small" /> Too small for {{ guests }}</li>
    </ul>
  </div>
</template>

<style scoped>
.plan {
  position: relative;
  aspect-ratio: 10 / 7;
  border-radius: var(--radius);
  overflow: hidden;
  background: #faf8f4;
  user-select: none;
  /* The plan is always a light drawing; keep it readable in dark mode. */
}
.plan img {
  position: absolute;
  inset: 0;
  width: 100%;
  height: 100%;
  object-fit: contain;
  pointer-events: none;
}

.pin {
  position: absolute;
  transform: translate(-50%, -50%);
  width: var(--w);
  min-width: 36px;
  aspect-ratio: var(--ratio);
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 0;
  font: inherit;
  line-height: 1.05;
  color: #123d2a;
  background: #e3f4eb;
  border: 2px solid var(--free);
  cursor: pointer;
  transition:
    transform 0.12s ease,
    background-color 0.12s ease;
}
.pin::after {
  /* Extends the tap area without changing the drawn size. */
  content: '';
  position: absolute;
  inset: -6px;
}
.pin.round {
  border-radius: 50%;
}
.pin.rect {
  border-radius: 8px;
}
.pin b {
  font-size: 0.72rem;
}
.pin small {
  font-size: 0.62rem;
  opacity: 0.75;
}

.pin.taken {
  color: #6c6c74;
  background: repeating-linear-gradient(135deg, #e4e4e8 0 4px, #d3d3d9 4px 8px);
  border-color: #b4b4bc;
  cursor: not-allowed;
}
.pin.too_small {
  color: #8a8a92;
  background: #f1f1f3;
  border: 2px dashed #c4c4cb;
  cursor: not-allowed;
}
.pin.selected {
  color: var(--button-text);
  background: var(--button);
  border-color: var(--button);
  transform: translate(-50%, -50%) scale(1.14);
  box-shadow:
    0 0 0 4px color-mix(in srgb, var(--button) 28%, transparent),
    0 4px 10px rgb(0 0 0 / 0.25);
  z-index: 2;
}
.pin.ro {
  cursor: default;
}
.pin:not(:disabled):active {
  transform: translate(-50%, -50%) scale(0.95);
}
.pin.selected:active {
  transform: translate(-50%, -50%) scale(1.14);
}

.legend {
  display: flex;
  flex-wrap: wrap;
  gap: 6px 16px;
  list-style: none;
  margin: 10px 2px 0;
  padding: 0;
  font-size: 0.8rem;
  color: var(--hint);
}
.dot {
  display: inline-block;
  width: 12px;
  height: 12px;
  border-radius: 50%;
  vertical-align: -1px;
  margin-right: 4px;
}
.dot.free {
  background: #e3f4eb;
  border: 2px solid var(--free);
}
.dot.taken {
  background: repeating-linear-gradient(135deg, #e4e4e8 0 3px, #cfcfd5 3px 6px);
  border: 2px solid #b4b4bc;
}
.dot.small {
  background: #f1f1f3;
  border: 2px dashed #c4c4cb;
}
</style>
