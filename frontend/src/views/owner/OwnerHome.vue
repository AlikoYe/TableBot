<script setup>
import { computed, onMounted, ref, watch } from 'vue'

import { listVenues } from '@/api/venues'
import AsyncState from '@/components/AsyncState.vue'
import { useSessionStore } from '@/stores/session'
import { addDays, relativeDay, todayIn } from '@/utils/format'
import BookingsPanel from './BookingsPanel.vue'
import FloorPanel from './FloorPanel.vue'

const session = useSessionStore()

const venues = ref([])
const venueId = ref(null)
const date = ref('')
const tab = ref('bookings')
const loading = ref(true)
const error = ref(null)

const venue = computed(() => venues.value.find((v) => v.id === venueId.value) ?? null)
const today = computed(() => (venue.value ? todayIn(venue.value.timezone) : ''))

async function load() {
  loading.value = true
  error.value = null
  try {
    await session.load()
    if (session.error) throw session.error
    const all = await listVenues()
    venues.value = all.filter((v) => session.ownedVenueIds.includes(v.id))
    venueId.value = venues.value[0]?.id ?? null
  } catch (err) {
    error.value = err
  } finally {
    loading.value = false
  }
}

// Each venue has its own time zone, so "today" resets when the venue changes.
watch(
  venue,
  (v) => {
    if (v) date.value = todayIn(v.timezone)
  },
  { immediate: true },
)

const shift = (n) => (date.value = addDays(date.value, n))

onMounted(load)
</script>

<template>
  <h1>Manage venue</h1>

  <AsyncState
    :loading="loading"
    :error="error"
    :empty="!venues.length"
    empty-text="You do not manage any venues yet."
    @retry="load"
  >
    <template v-if="venue">
      <div v-if="venues.length > 1" class="venues" role="group" aria-label="Venue">
        <button
          v-for="v in venues"
          :key="v.id"
          class="chip"
          type="button"
          :aria-pressed="v.id === venueId"
          @click="venueId = v.id"
        >
          {{ v.name }}
        </button>
      </div>
      <p v-else class="hint single">{{ venue.name }}</p>

      <div class="day card">
        <button type="button" aria-label="Previous day" @click="shift(-1)">‹</button>
        <div class="label">
          <strong>{{ relativeDay(date, today) }}</strong>
          <button v-if="date !== today" class="btn-text" type="button" @click="date = today">Back to today</button>
        </div>
        <button type="button" aria-label="Next day" @click="shift(1)">›</button>
      </div>

      <div class="tabs" role="tablist">
        <button role="tab" type="button" :aria-selected="tab === 'bookings'" @click="tab = 'bookings'">Bookings</button>
        <button role="tab" type="button" :aria-selected="tab === 'floor'" @click="tab = 'floor'">Floor</button>
      </div>

      <BookingsPanel v-if="tab === 'bookings'" :key="`b-${venue.id}-${date}`" :venue="venue" :date="date" />
      <FloorPanel v-else :key="`f-${venue.id}-${date}`" :venue="venue" :date="date" />
    </template>
  </AsyncState>
</template>

<style scoped>
.venues {
  display: flex;
  gap: 8px;
  overflow-x: auto;
  margin: 0 -16px 12px;
  padding: 0 16px 2px;
  scrollbar-width: none;
}
.venues::-webkit-scrollbar {
  display: none;
}
.single {
  margin: -6px 0 12px;
}
.day {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 6px;
}
.day > button {
  width: 44px;
  height: 44px;
  border: 0;
  border-radius: 50%;
  background: none;
  font: inherit;
  font-size: 1.6rem;
  line-height: 1;
  color: var(--link);
  cursor: pointer;
}
.label {
  display: flex;
  flex-direction: column;
  align-items: center;
}
.label .btn-text {
  padding: 0;
  font-size: 0.8rem;
  border: 0;
  font-family: inherit;
  cursor: pointer;
}

.tabs {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 2px;
  padding: 2px;
  margin-bottom: 16px;
  border-radius: var(--radius-sm);
  background: color-mix(in srgb, var(--hint) 18%, transparent);
}
.tabs button {
  padding: 8px 0;
  border: 0;
  border-radius: 8px;
  background: none;
  font: inherit;
  font-weight: 600;
  color: var(--hint);
  cursor: pointer;
}
.tabs button[aria-selected='true'] {
  background: var(--section-bg);
  color: var(--text);
  box-shadow: 0 1px 3px rgb(0 0 0 / 0.15);
}
</style>
