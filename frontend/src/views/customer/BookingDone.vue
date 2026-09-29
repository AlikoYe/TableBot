<script setup>
import { computed } from 'vue'

import { useBookingStore } from '@/stores/booking'
import { formatDateTime } from '@/utils/format'

const props = defineProps({ id: { type: String, required: true } })

const bookingStore = useBookingStore()
// After a reload the store is empty; the page still works from the route id.
const booking = computed(() =>
  String(bookingStore.lastBooking?.id) === props.id ? bookingStore.lastBooking : null,
)
</script>

<template>
  <div class="done">
    <div class="icon" aria-hidden="true">✓</div>
    <h1>Request sent</h1>
    <div v-if="booking" class="card">
      <strong>{{ booking.restaurant_name }}</strong>
      <div>{{ formatDateTime(booking.starts_at) }} · {{ booking.guests }} guests</div>
      <span class="badge">{{ booking.status }}</span>
    </div>
    <p class="hint">Booking #{{ id }}. We'll message you in Telegram when the restaurant confirms.</p>
    <RouterLink to="/my" class="btn btn-block">My bookings</RouterLink>
  </div>
</template>

<style scoped>
.done {
  text-align: center;
}
.icon {
  font-size: 3rem;
  color: var(--button);
}
.card {
  text-align: left;
}
</style>
