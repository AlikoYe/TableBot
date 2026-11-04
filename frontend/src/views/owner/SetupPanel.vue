<script setup>
import { computed, onMounted, reactive, ref } from 'vue'

import { createTable, listOwnerTables, updateTable, updateVenue } from '@/api/owner'
import AsyncState from '@/components/AsyncState.vue'
import { useTelegram } from '@/composables/useTelegram'
import { formError } from '@/utils/errors'
import { plural } from '@/utils/format'

// Owner setup: venue settings and the table layout on the floor plan.
const props = defineProps({
  venue: { type: Object, required: true },
})
const emit = defineEmits(['venue-updated'])
const { haptic } = useTelegram()

// ---- Venue settings ----
const form = reactive({
  name: props.venue.name,
  address: props.venue.address,
  description: props.venue.description,
  opens_at: props.venue.opens_at,
  closes_at: props.venue.closes_at,
  booking_minutes: props.venue.booking_minutes,
  slot_step_minutes: props.venue.slot_step_minutes,
})
const savingVenue = ref(false)
const venueError = ref('')
const venueSaved = ref(false)

const clocks = (from, to) => {
  const out = []
  for (let m = from; m <= to; m += 30) out.push(`${String(Math.floor(m / 60)).padStart(2, '0')}:${String(m % 60).padStart(2, '0')}`)
  return out
}
const withCurrent = (list, value) => (list.includes(value) ? list : [...list, value].sort())
const openOptions = computed(() => withCurrent(clocks(0, 23 * 60 + 30), form.opens_at))
const closeOptions = computed(() => withCurrent(clocks(30, 24 * 60), form.closes_at))
const lengthOptions = computed(() => {
  const base = [30, 45, 60, 90, 120, 150, 180, 240]
  return base.includes(form.booking_minutes) ? base : [...base, form.booking_minutes].sort((a, b) => a - b)
})
const lengthLabel = (m) => (m % 60 === 0 ? plural(m / 60, 'hour') : m > 60 ? `${Math.floor(m / 60)} h ${m % 60} min` : `${m} min`)

async function saveVenue() {
  savingVenue.value = true
  venueError.value = ''
  venueSaved.value = false
  try {
    const v = await updateVenue(props.venue.id, { ...form })
    emit('venue-updated', v)
    venueSaved.value = true
    haptic('success')
  } catch (err) {
    venueError.value = formError(err)
    haptic('error')
  } finally {
    savingVenue.value = false
  }
}

// ---- Tables ----
const tables = ref([])
const loading = ref(true)
const loadError = ref(null)
const draft = ref(null) // table being edited or added
const placing = ref(false) // waiting for a tap on the plan
const savingTable = ref(false)
const tableError = ref('')
const plan = ref(null)

async function loadTables() {
  loading.value = true
  loadError.value = null
  try {
    tables.value = await listOwnerTables(props.venue.id)
  } catch (err) {
    loadError.value = err
  } finally {
    loading.value = false
  }
}

const activeCount = computed(() => tables.value.filter((t) => t.is_active).length)
const seatCount = computed(() => tables.value.filter((t) => t.is_active).reduce((n, t) => n + t.seats, 0))

// Tables drawn on the plan; the one being edited is drawn from the draft.
const pins = computed(() => {
  const list = tables.value.filter((t) => t.id !== draft.value?.id)
  if (draft.value && draft.value.x != null) list.push({ ...draft.value, editing: true })
  return list
})

function pinStyle(t) {
  const w = t.shape === 'round' ? 5.5 + t.seats * 0.75 : 7 + t.seats * 1.15
  return { left: `${t.x}%`, top: `${t.y}%`, '--w': `${w}%`, '--ratio': t.shape === 'round' ? 1 : 1.55 }
}

function nextLabel() {
  const nums = tables.value.map((t) => Number(/^T(\d+)$/.exec(t.label)?.[1] ?? 0))
  return `T${Math.max(0, ...nums) + 1}`
}

function edit(t) {
  if (placing.value) return
  draft.value = { ...t }
  tableError.value = ''
  haptic('tap')
}

function startAdd() {
  draft.value = { label: nextLabel(), seats: 4, shape: 'round', is_active: true, x: null, y: null }
  tableError.value = ''
  placing.value = true
}

function startMove() {
  placing.value = true
}

function cancel() {
  draft.value = null
  placing.value = false
  tableError.value = ''
}

function place(e) {
  if (!placing.value || !draft.value) return
  const r = plan.value.getBoundingClientRect()
  const pct = (v) => Math.min(100, Math.max(0, Math.round(v * 1000) / 10))
  draft.value.x = pct((e.clientX - r.left) / r.width)
  draft.value.y = pct((e.clientY - r.top) / r.height)
  placing.value = false
  haptic('tap')
}

const changeSeats = (n) => (draft.value.seats = Math.min(20, Math.max(1, draft.value.seats + n)))

async function saveTable() {
  const d = draft.value
  savingTable.value = true
  tableError.value = ''
  const body = { label: d.label, seats: d.seats, x: d.x, y: d.y, shape: d.shape, is_active: d.is_active }
  try {
    const saved = d.id ? await updateTable(d.id, body) : await createTable(props.venue.id, body)
    const i = tables.value.findIndex((t) => t.id === saved.id)
    if (i >= 0) tables.value[i] = saved
    else tables.value.push(saved)
    draft.value = null
    haptic('success')
  } catch (err) {
    tableError.value = formError(err)
    haptic('error')
  } finally {
    savingTable.value = false
  }
}

onMounted(loadTables)
</script>

<template>
  <section class="section">
    <h2 class="section-title">
      <span>Tables</span>
      <span>{{ plural(activeCount, 'table') }} · {{ plural(seatCount, 'seat') }}</span>
    </h2>

    <AsyncState :loading="loading" :error="loadError" @retry="loadTables">
      <p v-if="placing" class="banner">Tap the plan where table {{ draft.label }} stands.</p>

      <div ref="plan" class="plan" :class="{ placing }" @click="place">
        <img v-if="venue.floor_plan_url" :src="venue.floor_plan_url" alt="" draggable="false" />
        <button
          v-for="t in pins"
          :key="t.id ?? 'new'"
          type="button"
          class="pin"
          :class="[t.shape, { off: !t.is_active, editing: t.editing }]"
          :style="pinStyle(t)"
          :aria-label="`Edit table ${t.label}, ${plural(t.seats, 'seat')}${t.is_active ? '' : ', hidden'}`"
          @click.stop="placing ? place($event) : edit(t)"
        >
          <b>{{ t.label }}</b>
          <small>{{ t.seats }}</small>
        </button>
      </div>
      <p class="hint legend">Tap a table to edit it. Hidden tables are dashed; guests do not see them.</p>

      <div v-if="draft" class="card editor">
        <h3>{{ draft.id ? `Table ${draft.label}` : 'New table' }}</h3>

        <label class="field">
          <span>Name on the plan</span>
          <input v-model.trim="draft.label" maxlength="10" />
        </label>

        <div class="row">
          <span id="seats-label">Seats</span>
          <div class="stepper" role="group" aria-labelledby="seats-label">
            <button type="button" aria-label="Fewer seats" :disabled="draft.seats <= 1" @click="changeSeats(-1)">−</button>
            <output aria-live="polite">{{ draft.seats }}</output>
            <button type="button" aria-label="More seats" :disabled="draft.seats >= 20" @click="changeSeats(1)">+</button>
          </div>
        </div>

        <div class="row">
          <span>Shape</span>
          <div class="chips" role="group" aria-label="Shape">
            <button class="chip" type="button" :aria-pressed="draft.shape === 'round'" @click="draft.shape = 'round'">Round</button>
            <button class="chip" type="button" :aria-pressed="draft.shape === 'rect'" @click="draft.shape = 'rect'">Rectangle</button>
          </div>
        </div>

        <div class="row">
          <span>Position</span>
          <button class="btn btn-quiet btn-small" type="button" @click="startMove">
            {{ draft.x == null ? 'Place on plan' : 'Move' }}
          </button>
        </div>

        <label v-if="draft.id" class="row toggle">
          <span>Guests can book it</span>
          <input v-model="draft.is_active" type="checkbox" role="switch" />
        </label>
        <p v-if="draft.id && !draft.is_active" class="hint">
          Existing bookings stay valid; the table just stops taking new ones.
        </p>

        <p v-if="tableError" class="banner error" role="alert">{{ tableError }}</p>
        <div class="actions">
          <button class="btn btn-quiet" type="button" @click="cancel">Cancel</button>
          <button class="btn" type="button" :disabled="savingTable || draft.x == null || !draft.label" @click="saveTable">
            {{ savingTable ? 'Saving…' : draft.id ? 'Save table' : 'Add table' }}
          </button>
        </div>
      </div>
      <button v-else class="btn btn-quiet btn-block add" type="button" @click="startAdd">+ Add table</button>
    </AsyncState>
  </section>

  <section class="section">
    <h2 class="section-title"><span>Venue</span></h2>
    <form class="card" @submit.prevent="saveVenue">
      <label class="field">
        <span>Name</span>
        <input v-model="form.name" maxlength="80" required />
      </label>
      <label class="field">
        <span>Address</span>
        <input v-model="form.address" maxlength="200" />
      </label>
      <label class="field">
        <span>Description</span>
        <textarea v-model="form.description" rows="3" maxlength="500" />
      </label>
      <div class="grid">
        <label class="field">
          <span>Opens</span>
          <select v-model="form.opens_at">
            <option v-for="c in openOptions" :key="c" :value="c">{{ c }}</option>
          </select>
        </label>
        <label class="field">
          <span>Closes</span>
          <select v-model="form.closes_at">
            <option v-for="c in closeOptions" :key="c" :value="c">{{ c }}</option>
          </select>
        </label>
        <label class="field">
          <span>Each booking lasts</span>
          <select v-model.number="form.booking_minutes">
            <option v-for="m in lengthOptions" :key="m" :value="m">{{ lengthLabel(m) }}</option>
          </select>
        </label>
        <label class="field">
          <span>Start times every</span>
          <select v-model.number="form.slot_step_minutes">
            <option :value="15">15 min</option>
            <option :value="30">30 min</option>
            <option :value="60">1 hour</option>
          </select>
        </label>
      </div>
      <p class="hint">Changes apply to new bookings only.</p>
      <p v-if="venueError" class="banner error" role="alert">{{ venueError }}</p>
      <p v-else-if="venueSaved" class="saved" role="status">Saved</p>
      <button class="btn btn-block" type="submit" :disabled="savingVenue">
        {{ savingVenue ? 'Saving…' : 'Save venue' }}
      </button>
    </form>
  </section>
</template>

<style scoped>
.plan {
  position: relative;
  aspect-ratio: 10 / 7;
  border-radius: var(--radius);
  overflow: hidden;
  background: #faf8f4;
  user-select: none;
}
.plan.placing {
  cursor: crosshair;
  box-shadow: 0 0 0 3px var(--button);
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
.pin.off {
  color: #8a8a92;
  background: #f1f1f3;
  border: 2px dashed #b4b4bc;
}
.pin.editing {
  color: var(--button-text);
  background: var(--button);
  border: 2px solid var(--button);
  box-shadow: 0 0 0 4px color-mix(in srgb, var(--button) 28%, transparent);
  z-index: 2;
}
.legend {
  margin: 8px 2px 12px;
  font-size: 0.8rem;
}

.editor h3 {
  margin: 0 0 12px;
  font-size: 1rem;
}
.row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: var(--gap);
}
.chips {
  display: flex;
  gap: 6px;
}
.stepper {
  display: flex;
  align-items: center;
  gap: 4px;
}
.stepper button {
  width: 36px;
  height: 36px;
  border: 0;
  border-radius: 50%;
  font: inherit;
  font-size: 1.3rem;
  line-height: 1;
  background: color-mix(in srgb, var(--button) 12%, transparent);
  color: var(--link);
  cursor: pointer;
}
.stepper button:disabled {
  opacity: 0.35;
}
.stepper output {
  min-width: 32px;
  text-align: center;
  font-weight: 700;
}
.toggle input {
  width: 22px;
  height: 22px;
  accent-color: var(--button);
}
.actions {
  display: grid;
  grid-template-columns: 1fr 2fr;
  gap: 8px;
}
.add {
  margin-top: 4px;
}
.grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 0 10px;
}
.saved {
  color: var(--ok);
  font-weight: 600;
  margin: 0 0 10px;
}
</style>
