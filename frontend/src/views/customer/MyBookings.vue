<script setup>
import { onMounted, ref } from 'vue'

import { cancelBooking, listMyBookings } from '@/api/bookings'
import AsyncState from '@/components/AsyncState.vue'
import { useTelegram } from '@/composables/useTelegram'
import { formatDateTime } from '@/utils/format'

const { confirmDialog } = useTelegram()

const bookings = ref([])
const loading = ref(true)
const error = ref(null)

async function load() {
  loading.value = true
  error.value = null
  try {
    bookings.value = (await listMyBookings()).items
  } catch (err) {
    error.value = err
  } finally {
    loading.value = false
  }
}

async function cancel(booking) {
  if (!(await confirmDialog(`Cancel your booking at ${booking.restaurant_name}?`))) return
  try {
    await cancelBooking(booking.id)
    await load()
  } catch (err) {
    error.value = err
  }
}

const isActive = (b) => b.status === 'pending' || b.status === 'confirmed'

onMounted(load)
</script>

<template>
  <h1>My bookings</h1>
  <AsyncState :loading :error :empty="!bookings.length" empty-text="You have no bookings yet." @retry="load">
    <div v-for="b in bookings" :key="b.id" class="card">
      <strong>{{ b.restaurant_name }}</strong>
      <div>{{ formatDateTime(b.starts_at) }} · {{ b.guests }} guests</div>
      <span class="badge">{{ b.status }}</span>
      <div v-if="isActive(b)">
        <button class="btn btn-danger" type="button" @click="cancel(b)">Cancel booking</button>
      </div>
    </div>
  </AsyncState>
</template>
