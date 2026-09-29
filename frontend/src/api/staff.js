import { api } from './client'

/** @param {string} date YYYY-MM-DD */
export const listStaffBookings = (date) => api.get('/staff/bookings', { query: { date } })

export const confirmBooking = (id) => api.post(`/staff/bookings/${id}/confirm`)
