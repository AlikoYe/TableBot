import { api } from './client'

/** @param {{restaurant_id: number, starts_at: string, guests: number, special_request?: string}} input */
export const createBooking = (input) => api.post('/bookings', input)

export const listMyBookings = () => api.get('/bookings/my')

export const cancelBooking = (id) => api.post(`/bookings/${id}/cancel`)
