<script setup>
import { onMounted, ref, watch } from 'vue'

import { confirmBooking, listStaffBookings } from '@/api/staff'
import AsyncState from '@/components/AsyncState.vue'
import { formatTime, todayISO } from '@/utils/format'

const date = ref(todayISO())
const bookings = ref([])
const loading = ref(true)
const error = ref(null)

async function load() {
  loading.value = true
  error.value = null
  try {
    bookings.value = (await listStaffBookings(date.value)).items
  } catch (err) {
    error.value = err
  } finally {
    loading.value = false
  }
}

async function confirm(booking) {
  try {
    await confirmBooking(booking.id)
    await load()
  } catch (err) {
    error.value = err
  }
}

watch(date, load)
onMounted(load)
</script>

<template>
  <div class="header">
    <h1>Bookings</h1>
    <RouterLink to="/staff/capacity">Capacity</RouterLink>
  </div>

  <label class="field">
    <span>Date</span>
    <input v-model="date" type="date" />
  </label>

  <AsyncState :loading :error :empty="!bookings.length" empty-text="No bookings for this day." @retry="load">
    <div v-for="b in bookings" :key="b.id" class="card">
      <strong>{{ formatTime(b.starts_at) }}</strong> · {{ b.customer_name }} · {{ b.guests }} guests
      <div v-if="b.special_request" class="hint">“{{ b.special_request }}”</div>
      <span class="badge">{{ b.status }}</span>
      <div v-if="b.status === 'pending'">
        <button class="btn btn-link" type="button" @click="confirm(b)">Confirm</button>
      </div>
    </div>
  </AsyncState>
</template>

<style scoped>
.header {
  display: flex;
  justify-content: space-between;
  align-items: baseline;
}
</style>
