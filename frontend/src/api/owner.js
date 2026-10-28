import { api } from './client'

/** Bookings at an owned venue starting on a venue-local date (YYYY-MM-DD). Includes `guest`. */
export const listOwnerBookings = ({ venueId, date }, opts) =>
  api.get('/owner/bookings', { query: { venue_id: venueId, date }, ...opts })

export const confirmBooking = (id) => api.post(`/owner/bookings/${id}/confirm`)

export const rejectBooking = (id) => api.post(`/owner/bookings/${id}/reject`)
