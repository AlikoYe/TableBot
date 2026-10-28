<script setup>
import { computed, onMounted, ref } from 'vue'

import { cancelBooking, listMyBookings } from '@/api/bookings'
import AsyncState from '@/components/AsyncState.vue'
import StatusBadge from '@/components/StatusBadge.vue'
import { useNow } from '@/composables/useNow'
import { usePolling } from '@/composables/usePolling'
import { useTelegram } from '@/composables/useTelegram'
import { friendlyError } from '@/utils/errors'
import { formatCountdown, formatWhen, plural } from '@/utils/format'

const { confirmDialog, haptic } = useTelegram()
const now = useNow()

const bookings = ref([])
const loading = ref(true)
const error = ref(null)
const notice = ref('')
const busyId = ref(null)

const isActive = (b) => b.status === 'pending' || b.status === 'confirmed'
// API order is newest first; upcoming reads better soonest first.
const upcoming = computed(() =>
  bookings.value.filter((b) => isActive(b) && Date.parse(b.ends_at) > now.value).reverse(),
)
const history = computed(() => bookings.value.filter((b) => !upcoming.value.includes(b)))

async function load() {
  loading.value = true
  error.value = null
  try {
    bookings.value = await listMyBookings()
  } catch (err) {
    error.value = err
  } finally {
    loading.value = false
  }
}

async function refresh() {
  bookings.value = await listMyBookings()
}

async function cancel(b) {
  const ok = await confirmDialog(`Cancel your booking at ${b.venue.name}?`)
  if (!ok) return
  busyId.value = b.id
  notice.value = ''
  try {
    await cancelBooking(b.id)
    haptic('success')
  } catch (err) {
    haptic('error')
    notice.value = friendlyError(err)
  } finally {
    busyId.value = null
  }
  await refresh().catch(() => {})
}

usePolling(refresh, 20000)
onMounted(load)
</script>

<template>
  <h1>My bookings</h1>
  <p v-if="notice" class="banner error" role="alert">{{ notice }}</p>

  <AsyncState
    :loading="loading"
    :error="error"
    :empty="!bookings.length"
    empty-text="No bookings yet. Pick a venue and reserve a table."
    @retry="load"
  >
    <section v-if="upcoming.length" class="section">
      <h2 class="section-title">Upcoming</h2>
      <article v-for="b in upcoming" :key="b.id" class="card">
        <div class="row">
          <strong>{{ b.venue.name }}</strong>
          <StatusBadge :status="b.status" />
        </div>
        <div>{{ formatWhen(b) }}</div>
        <div class="hint">Table {{ b.table.label }} · {{ plural(b.guests, 'guest') }}</div>
        <div v-if="b.special_request" class="hint">“{{ b.special_request }}”</div>
        <div v-if="b.status === 'pending' && b.expires_at" class="wait">
          Waiting for the venue · {{ formatCountdown(b.expires_at, now) }}
        </div>
        <button class="btn btn-danger btn-small" type="button" :disabled="busyId === b.id" @click="cancel(b)">
          {{ busyId === b.id ? 'Cancelling…' : 'Cancel booking' }}
        </button>
      </article>
    </section>

    <section v-if="history.length" class="section">
      <h2 class="section-title">Past and closed</h2>
      <article v-for="b in history" :key="b.id" class="card past">
        <div class="row">
          <strong>{{ b.venue.name }}</strong>
          <StatusBadge :status="b.status" />
        </div>
        <div>{{ formatWhen(b) }}</div>
        <div class="hint">Table {{ b.table.label }} · {{ plural(b.guests, 'guest') }}</div>
      </article>
    </section>
  </AsyncState>
</template>

<style scoped>
.card {
  display: grid;
  gap: 3px;
  justify-items: start;
}
.row {
  width: 100%;
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 8px;
}
.wait {
  font-size: 0.9rem;
  font-weight: 600;
  color: var(--wait);
}
.card .btn {
  margin-top: 8px;
}
.past {
  opacity: 0.8;
}
</style>
