<script setup>
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRouter } from 'vue-router'

import { getVenue, listTables, listTimes } from '@/api/venues'
import AsyncState from '@/components/AsyncState.vue'
import DateStrip from '@/components/DateStrip.vue'
import FloorPlan from '@/components/FloorPlan.vue'
import { usePolling } from '@/composables/usePolling'
import { useTelegram } from '@/composables/useTelegram'
import { useBookingStore } from '@/stores/booking'
import { errorCode, friendlyError } from '@/utils/errors'
import { formatDuration, formatRange, plural, relativeDay, timeOf, todayIn } from '@/utils/format'
import { categoryIcon, hoursText } from '@/utils/venue'

const props = defineProps({ id: { type: String, required: true } })

const router = useRouter()
const bookingStore = useBookingStore()
const { isTelegram, showMainButton, setMainButtonLoading, haptic } = useTelegram()

const MAX_GUESTS = 20

const venue = ref(null)
const venueError = ref(null)
const today = ref('')
const date = ref('')
const guests = ref(2)

const times = ref([])
const timesLoading = ref(false)
const timesError = ref(null)
const selectedTime = ref(null)

const tableMap = ref(null)
const tablesLoading = ref(false)
const tablesError = ref(null)
const selectedTableId = ref(null)

const note = ref('')
const summaryEl = ref(null)
const submitting = ref(false)
const notice = ref('')

const selectedTable = computed(
  () => tableMap.value?.tables.find((t) => t.id === selectedTableId.value) ?? null,
)
const freeCount = computed(() => tableMap.value?.tables.filter((t) => t.status === 'free').length ?? 0)

// Ignore responses that arrive after the user already changed the inputs.
let timesRun = 0
let tablesRun = 0

async function loadVenue() {
  venueError.value = null
  try {
    venue.value = await getVenue(props.id)
    today.value = todayIn(venue.value.timezone)
    date.value = today.value
  } catch (err) {
    venueError.value = err
  }
}

async function loadTimes() {
  const run = ++timesRun
  timesLoading.value = true
  timesError.value = null
  try {
    const result = await listTimes(props.id, { date: date.value, guests: guests.value })
    if (run === timesRun) times.value = result
  } catch (err) {
    if (run === timesRun) timesError.value = err
  } finally {
    if (run === timesRun) timesLoading.value = false
  }
}

/** `silent` refreshes in the background without clearing the plan. */
async function loadTables({ silent = false } = {}) {
  if (!selectedTime.value) return
  const run = ++tablesRun
  if (!silent) {
    tablesLoading.value = true
    tableMap.value = null
  }
  tablesError.value = null
  try {
    const result = await listTables(props.id, {
      startsAt: selectedTime.value.starts_at,
      guests: guests.value,
    })
    if (run !== tablesRun) return
    tableMap.value = result
    const chosen = result.tables.find((t) => t.id === selectedTableId.value)
    if (chosen && chosen.status !== 'free') {
      notice.value = `Table ${chosen.label} was just booked. Pick another one.`
      selectedTableId.value = null
    }
  } catch (err) {
    if (run === tablesRun && !silent) tablesError.value = err
  } finally {
    if (run === tablesRun) tablesLoading.value = false
  }
}

function changeGuests(delta) {
  guests.value = Math.min(MAX_GUESTS, Math.max(1, guests.value + delta))
  haptic('tap')
}

function pickTime(t) {
  if (t.free_tables === 0) return
  haptic('tap')
  selectedTime.value = t
}

watch([date, guests], () => {
  selectedTime.value = null
  times.value = []
  loadTimes()
})

// Bring the summary and request button into view once a table is picked.
watch(selectedTableId, async (id) => {
  if (id === null) return
  await nextTick()
  summaryEl.value?.scrollIntoView({ behavior: 'smooth', block: 'nearest' })
})

watch(selectedTime, (t) => {
  selectedTableId.value = null
  notice.value = ''
  tableMap.value = null
  tablesRun++
  if (t) loadTables()
})

async function submit() {
  if (!selectedTable.value || submitting.value) return
  submitting.value = true
  notice.value = ''
  setMainButtonLoading(true)
  try {
    const booking = await bookingStore.create({
      table_id: selectedTable.value.id,
      starts_at: selectedTime.value.starts_at,
      guests: guests.value,
      special_request: note.value.trim(),
    })
    haptic('success')
    router.replace({ name: 'booking-done', params: { id: booking.id } })
  } catch (err) {
    haptic('error')
    notice.value = friendlyError(err)
    const code = errorCode(err)
    if (code === 'table_taken' || code === 'too_many_guests') {
      selectedTableId.value = null
      loadTables({ silent: true })
    } else if (code === 'outside_hours' || code === 'in_past') {
      selectedTime.value = null
      loadTimes()
    }
  } finally {
    submitting.value = false
    setMainButtonLoading(false)
  }
}

// Telegram's MainButton mirrors the in-page button.
const actionText = computed(() =>
  selectedTable.value ? `Request table ${selectedTable.value.label} · ${timeOf(selectedTime.value.starts_at)}` : null,
)
let hideMainButton = () => {}
watch(actionText, (text) => {
  hideMainButton()
  hideMainButton = text ? showMainButton(text, submit) : () => {}
})
onBeforeUnmount(() => hideMainButton())

// Keep the plan live while the diner is choosing.
usePolling(() => loadTables({ silent: true }), 20000)

onMounted(async () => {
  await loadVenue()
  if (venue.value) loadTimes()
})
</script>

<template>
  <AsyncState :loading="!venue && !venueError" :error="venueError" @retry="loadVenue">
    <template v-if="venue">
      <header class="head">
        <span class="icon" aria-hidden="true">{{ categoryIcon(venue.category) }}</span>
        <div>
          <h1>{{ venue.name }}</h1>
          <p class="hint">{{ venue.address }}</p>
        </div>
      </header>
      <p class="about">{{ venue.description }}</p>
      <p class="hint facts">
        Open {{ hoursText(venue) }} · Each booking lasts {{ formatDuration(venue.booking_minutes) }}
      </p>

      <section class="section">
        <h2 class="section-title">When</h2>
        <DateStrip v-model="date" :start="today" />
      </section>

      <section class="section">
        <div class="guests card">
          <span id="guests-label">Guests</span>
          <div class="stepper" role="group" aria-labelledby="guests-label">
            <button type="button" aria-label="Fewer guests" :disabled="guests <= 1" @click="changeGuests(-1)">−</button>
            <output aria-live="polite">{{ guests }}</output>
            <button type="button" aria-label="More guests" :disabled="guests >= MAX_GUESTS" @click="changeGuests(1)">+</button>
          </div>
        </div>
      </section>

      <section class="section">
        <h2 class="section-title">
          <span>Start time</span>
          <span v-if="date">{{ relativeDay(date, today) }}</span>
        </h2>
        <AsyncState
          :loading="timesLoading"
          :error="timesError"
          :empty="!times.length"
          :empty-text="date === today ? 'No more start times today. Pick another day.' : 'No start times on this day.'"
          @retry="loadTimes"
        >
          <div class="times">
            <button
              v-for="t in times"
              :key="t.starts_at"
              type="button"
              class="time"
              :aria-pressed="selectedTime?.starts_at === t.starts_at"
              :disabled="t.free_tables === 0"
              @click="pickTime(t)"
            >
              <b>{{ timeOf(t.starts_at) }}</b>
              <small>{{ t.free_tables === 0 ? 'Full' : `${t.free_tables} free` }}</small>
            </button>
          </div>
        </AsyncState>
      </section>

      <section v-if="selectedTime" class="section">
        <h2 class="section-title">
          <span>Pick a table</span>
          <span v-if="tableMap">{{ freeCount }} free</span>
        </h2>
        <AsyncState :loading="tablesLoading" :error="tablesError" @retry="loadTables">
          <FloorPlan
            v-if="tableMap"
            v-model="selectedTableId"
            :plan-url="tableMap.floor_plan_url"
            :tables="tableMap.tables"
            :guests="guests"
          />
        </AsyncState>
      </section>

      <p v-if="notice" class="banner error" role="alert">{{ notice }}</p>

      <section v-if="selectedTable" ref="summaryEl" class="section summary card">
        <p class="pick">
          <strong>Table {{ selectedTable.label }}</strong>
          <span class="hint">
            seats {{ selectedTable.seats }} · {{ plural(guests, 'guest') }} · {{ formatRange(selectedTime) }}
          </span>
        </p>
        <label class="field">
          <span>Note for the venue (optional)</span>
          <textarea
            v-model="note"
            rows="2"
            maxlength="300"
            placeholder="Birthday, high chair, window seat…"
          />
        </label>
        <p class="hint">The venue has a few minutes to confirm. You will get a message in Telegram.</p>
        <button v-if="!isTelegram" class="btn btn-block" type="button" :disabled="submitting" @click="submit">
          {{ submitting ? 'Sending…' : actionText }}
        </button>
      </section>
    </template>
  </AsyncState>
</template>

<style scoped>
.head {
  display: flex;
  gap: 14px;
  align-items: center;
  margin-bottom: 8px;
}
.head h1 {
  margin: 0;
}
.head p {
  margin: 2px 0 0;
}
.icon {
  flex: none;
  width: 52px;
  height: 52px;
  display: grid;
  place-items: center;
  font-size: 1.8rem;
  border-radius: 14px;
  background: var(--section-bg);
}
.about {
  margin: 12px 0 6px;
}
.facts {
  margin: 0 0 20px;
}

.guests {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin: 0;
  padding: 8px 8px 8px 16px;
}
.stepper {
  display: flex;
  align-items: center;
  gap: 4px;
}
.stepper button {
  width: 40px;
  height: 40px;
  border: 0;
  border-radius: 50%;
  font: inherit;
  font-size: 1.4rem;
  line-height: 1;
  background: color-mix(in srgb, var(--button) 12%, transparent);
  color: var(--link);
  cursor: pointer;
}
.stepper button:disabled {
  opacity: 0.35;
  cursor: default;
}
.stepper output {
  min-width: 36px;
  text-align: center;
  font-size: 1.2rem;
  font-weight: 700;
}

.times {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 8px;
}
.time {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 1px;
  padding: 8px 0;
  border: 1px solid var(--separator);
  border-radius: var(--radius-sm);
  background: var(--section-bg);
  color: var(--text);
  font: inherit;
  cursor: pointer;
}
.time b {
  font-size: 1rem;
}
.time small {
  font-size: 0.7rem;
  color: var(--hint);
}
.time:disabled {
  opacity: 0.45;
  cursor: default;
}
.time[aria-pressed='true'] {
  background: var(--button);
  border-color: var(--button);
  color: var(--button-text);
}
.time[aria-pressed='true'] small {
  color: inherit;
  opacity: 0.85;
}

.summary .pick {
  display: flex;
  flex-direction: column;
  gap: 2px;
  margin: 0 0 12px;
}
.summary .pick strong {
  font-size: 1.1rem;
}
.summary .hint {
  margin: 0 0 12px;
}
</style>
