<script setup>
import { computed, onMounted, ref } from 'vue'

import { confirmBooking, listOwnerBookings, rejectBooking } from '@/api/owner'
import AsyncState from '@/components/AsyncState.vue'
import StatusBadge from '@/components/StatusBadge.vue'
import { useNow } from '@/composables/useNow'
import { usePolling } from '@/composables/usePolling'
import { useTelegram } from '@/composables/useTelegram'
import { errorCode, friendlyError } from '@/utils/errors'
import { formatCountdown, formatRange, minutesUntil, plural, timeOf } from '@/utils/format'

const props = defineProps({
  venue: { type: Object, required: true },
  date: { type: String, required: true },
})

const { confirmDialog, haptic, openChat } = useTelegram()
const now = useNow(10000)

const bookings = ref([])
const loading = ref(true)
const error = ref(null)
const notice = ref('')
const busyId = ref(null)

// A pending request past expires_at is already `expired` on the next fetch;
// hide it from the action list as soon as the clock says so.
const pending = computed(() =>
  bookings.value
    .filter((b) => b.status === 'pending' && b.expires_at && minutesUntil(b.expires_at, now.value) > 0)
    .sort((a, b) => Date.parse(a.expires_at) - Date.parse(b.expires_at)),
)
const schedule = computed(() => bookings.value.filter((b) => !pending.value.includes(b)))
const confirmed = computed(() => bookings.value.filter((b) => b.status === 'confirmed'))
const confirmedGuests = computed(() => confirmed.value.reduce((sum, b) => sum + b.guests, 0))

async function fetchBookings() {
  bookings.value = await listOwnerBookings({ venueId: props.venue.id, date: props.date })
}

async function load() {
  loading.value = true
  error.value = null
  try {
    await fetchBookings()
  } catch (err) {
    error.value = err
  } finally {
    loading.value = false
  }
}

async function decide(b, action) {
  if (action === 'reject') {
    const ok = await confirmDialog(`Decline ${b.guest?.name ?? 'this guest'}'s request for table ${b.table.label}?`)
    if (!ok) return
  }
  busyId.value = b.id
  notice.value = ''
  try {
    const updated = await (action === 'confirm' ? confirmBooking(b.id) : rejectBooking(b.id))
    const i = bookings.value.findIndex((x) => x.id === b.id)
    if (i !== -1) bookings.value[i] = updated
    haptic('success')
  } catch (err) {
    haptic('error')
    notice.value = friendlyError(err)
    if (errorCode(err) === 'not_pending') await fetchBookings().catch(() => {})
  } finally {
    busyId.value = null
  }
}

usePolling(fetchBookings, 15000)
onMounted(load)
</script>

<template>
  <p v-if="notice" class="banner error" role="alert">{{ notice }}</p>

  <AsyncState
    :loading="loading"
    :error="error"
    :empty="!bookings.length"
    empty-text="No bookings for this day."
    @retry="load"
  >
    <p class="stats hint">
      {{ confirmed.length }} confirmed · {{ plural(confirmedGuests, 'guest') }} expected
    </p>

    <section v-if="pending.length" class="section">
      <h2 class="section-title">
        <span>Needs your answer</span><span>{{ pending.length }}</span>
      </h2>
      <article v-for="b in pending" :key="b.id" class="card request">
        <div class="top">
          <strong class="time">{{ formatRange(b) }}</strong>
          <span class="wait">{{ formatCountdown(b.expires_at, now) }}</span>
        </div>
        <div>
          Table {{ b.table.label }} · {{ plural(b.guests, 'guest') }}
        </div>
        <div class="who">
          <span>{{ b.guest?.name }}</span>
          <button v-if="b.guest?.username" class="btn-text handle" type="button" @click="openChat(b.guest.username)">
            @{{ b.guest.username }}
          </button>
        </div>
        <div v-if="b.special_request" class="note">“{{ b.special_request }}”</div>
        <div class="actions">
          <button class="btn btn-danger" type="button" :disabled="busyId === b.id" @click="decide(b, 'reject')">
            Decline
          </button>
          <button class="btn" type="button" :disabled="busyId === b.id" @click="decide(b, 'confirm')">
            Confirm
          </button>
        </div>
      </article>
    </section>

    <section v-if="schedule.length" class="section">
      <h2 class="section-title">Schedule</h2>
      <article v-for="b in schedule" :key="b.id" class="card row" :class="{ muted: b.status !== 'confirmed' }">
        <strong class="start">{{ timeOf(b.starts_at) }}</strong>
        <div class="main">
          <span>
            {{ b.guest?.name }} · {{ plural(b.guests, 'guest') }}
          </span>
          <span class="hint">Table {{ b.table.label }} · until {{ timeOf(b.ends_at) }}</span>
          <span v-if="b.special_request" class="hint">“{{ b.special_request }}”</span>
        </div>
        <StatusBadge :status="b.status" />
      </article>
    </section>
  </AsyncState>
</template>

<style scoped>
.stats {
  margin: 0 0 14px;
}
.request {
  display: grid;
  gap: 4px;
}
.top {
  display: flex;
  justify-content: space-between;
  align-items: baseline;
}
.time {
  font-size: 1.2rem;
}
.wait {
  font-size: 0.85rem;
  font-weight: 600;
  color: var(--wait);
}
.who {
  display: flex;
  gap: 8px;
  align-items: baseline;
  flex-wrap: wrap;
}
.handle {
  border: 0;
  font: inherit;
  font-size: 0.9rem;
  padding: 0;
  cursor: pointer;
}
.note {
  font-size: 0.9rem;
  color: var(--hint);
}
.actions {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 10px;
  margin-top: 10px;
}
.row {
  display: flex;
  gap: 12px;
  align-items: flex-start;
}
.start {
  flex: none;
  width: 3.2em;
  font-size: 1.05rem;
}
.main {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
}
.muted {
  opacity: 0.65;
}
</style>
