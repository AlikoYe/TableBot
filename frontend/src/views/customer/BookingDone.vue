<script setup>
import { computed, onMounted, ref } from 'vue'

import { listMyBookings } from '@/api/bookings'
import AsyncState from '@/components/AsyncState.vue'
import StatusBadge from '@/components/StatusBadge.vue'
import { usePolling } from '@/composables/usePolling'
import { useNow } from '@/composables/useNow'
import { useBookingStore } from '@/stores/booking'
import { formatCountdown, formatWhen, plural } from '@/utils/format'

const props = defineProps({ id: { type: String, required: true } })

const bookingStore = useBookingStore()
const now = useNow()

const fetched = ref(null)
const error = ref(null)
const loading = ref(false)

// Right after booking the store has it; after a reload we look it up.
const booking = computed(() => {
  const last = bookingStore.lastBooking
  const fresh = fetched.value
  if (fresh) return fresh
  return last && String(last.id) === props.id ? last : null
})

async function refresh() {
  const list = await listMyBookings()
  const found = list.find((b) => String(b.id) === props.id)
  if (found) fetched.value = found
  return found
}

async function load() {
  loading.value = true
  error.value = null
  try {
    await refresh()
  } catch (err) {
    error.value = err
  } finally {
    loading.value = false
  }
}

// Pick up the owner's decision while this screen is open.
usePolling(refresh, 10000)
onMounted(load)

const headline = computed(() => {
  switch (booking.value?.status) {
    case 'confirmed':
      return 'Table confirmed'
    case 'rejected':
      return 'Request declined'
    case 'expired':
      return 'Request expired'
    case 'cancelled':
      return 'Booking cancelled'
    default:
      return 'Request sent'
  }
})
const detail = computed(() => {
  switch (booking.value?.status) {
    case 'confirmed':
      return 'The venue confirmed your table. We will remind you before you go.'
    case 'rejected':
      return 'The venue could not take this booking. Try another time or table.'
    case 'expired':
      return 'The venue did not answer in time. Try again or pick another venue.'
    case 'cancelled':
      return 'This booking was cancelled.'
    default:
      return 'Waiting for the venue to confirm. We will message you in Telegram as soon as they answer.'
  }
})
</script>

<template>
  <AsyncState :loading="loading && !booking" :error="booking ? null : error" @retry="load">
    <div class="done" :data-status="booking?.status ?? 'pending'">
      <div class="mark" aria-hidden="true">
        <svg v-if="booking?.status === 'confirmed' || !booking || booking.status === 'pending'" viewBox="0 0 24 24"><path d="M5 12.5l4.5 4.5L19 7.5" /></svg>
        <svg v-else viewBox="0 0 24 24"><path d="M6 6l12 12M18 6L6 18" /></svg>
      </div>
      <h1>{{ headline }}</h1>
      <p class="detail">{{ detail }}</p>

      <div v-if="booking" class="card summary">
        <div class="row">
          <strong>{{ booking.venue.name }}</strong>
          <StatusBadge :status="booking.status" />
        </div>
        <div>{{ formatWhen(booking) }}</div>
        <div class="hint">
          Table {{ booking.table.label }} · {{ plural(booking.guests, 'guest') }}
        </div>
        <div v-if="booking.special_request" class="hint">“{{ booking.special_request }}”</div>
        <div v-if="booking.status === 'pending' && booking.expires_at" class="wait">
          Venue answer due: {{ formatCountdown(booking.expires_at, now) }}
        </div>
        <div class="hint">Booking #{{ booking.id }}</div>
      </div>

      <RouterLink :to="{ name: 'my-bookings' }" class="btn btn-block">My bookings</RouterLink>
      <RouterLink :to="{ name: 'venues' }" class="btn btn-quiet btn-block again">Book another table</RouterLink>
    </div>
  </AsyncState>
</template>

<style scoped>
.done {
  --c: var(--wait);
  text-align: center;
  padding-top: 12px;
}
.done[data-status='confirmed'] {
  --c: var(--ok);
}
.done[data-status='rejected'],
.done[data-status='expired'],
.done[data-status='cancelled'] {
  --c: var(--destructive);
}
.mark {
  width: 64px;
  height: 64px;
  margin: 0 auto 14px;
  display: grid;
  place-items: center;
  border-radius: 50%;
  background: color-mix(in srgb, var(--c) 16%, transparent);
  color: var(--c);
}
.mark svg {
  width: 32px;
  height: 32px;
  fill: none;
  stroke: currentColor;
  stroke-width: 2.4;
  stroke-linecap: round;
  stroke-linejoin: round;
}
.detail {
  margin: 0 auto 20px;
  max-width: 32ch;
  color: var(--hint);
}
.summary {
  text-align: left;
  display: grid;
  gap: 4px;
}
.row {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 8px;
}
.wait {
  margin-top: 4px;
  font-weight: 600;
  color: var(--wait);
}
.again {
  margin-top: 10px;
}
</style>
