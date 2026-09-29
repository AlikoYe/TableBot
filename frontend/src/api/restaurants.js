import { api } from './client'

export const listRestaurants = () => api.get('/restaurants')

export const getRestaurant = (id) => api.get(`/restaurants/${id}`)

/** @param {{date: string, guests: number}} params date is YYYY-MM-DD */
export const getSlots = (id, { date, guests }) =>
  api.get(`/restaurants/${id}/slots`, { query: { date, guests } })
