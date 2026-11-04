import { api } from './client'

/** Bookings at an owned venue starting on a venue-local date (YYYY-MM-DD). Includes `guest`. */
export const listOwnerBookings = ({ venueId, date }, opts) =>
  api.get('/owner/bookings', { query: { venue_id: venueId, date }, ...opts })

export const confirmBooking = (id) => api.post(`/owner/bookings/${id}/confirm`)

export const rejectBooking = (id) => api.post(`/owner/bookings/${id}/reject`)

/** Change venue settings; send only the fields to change. Returns the updated venue. */
export const updateVenue = (id, patch) => api.patch(`/owner/venues/${id}`, patch)

/** Every table of an owned venue, inactive ones included (`is_active`). */
export const listOwnerTables = (venueId, opts) => api.get(`/owner/venues/${venueId}/tables`, opts)

export const createTable = (venueId, table) => api.post(`/owner/venues/${venueId}/tables`, table)

/** Edit, move or (de)activate a table; send only the fields to change. */
export const updateTable = (id, patch) => api.patch(`/owner/tables/${id}`, patch)
