import { api } from './client'

/** @param {{table_id: number, starts_at: string, guests: number, special_request?: string}} input */
export const createBooking = (input) => api.post('/bookings', input)

export const listMyBookings = (opts) => api.get('/bookings/my', opts)

export const cancelBooking = (id) => api.post(`/bookings/${id}/cancel`)
