import { defineStore } from 'pinia'
import { ref } from 'vue'

import { createBooking } from '@/api/bookings'

/** Holds the booking just created so BookingDone can show it without refetching. */
export const useBookingStore = defineStore('booking', () => {
  const lastBooking = ref(null)

  async function create(input) {
    lastBooking.value = await createBooking(input)
    return lastBooking.value
  }

  return { lastBooking, create }
})
