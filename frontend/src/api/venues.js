import { api } from './client'

export const listVenues = (category) => api.get('/venues', { query: { category } })

export const getVenue = (id) => api.get(`/venues/${id}`)

/** Start times on a venue-local date (YYYY-MM-DD) with the number of free tables. */
export const listTimes = (id, { date, guests }, opts) =>
  api.get(`/venues/${id}/times`, { query: { date, guests }, ...opts })

/** Floor plan tables with their status for one start time (RFC 3339 from listTimes). */
export const listTables = (id, { startsAt, guests }, opts) =>
  api.get(`/venues/${id}/tables`, { query: { starts_at: startsAt, guests }, ...opts })
