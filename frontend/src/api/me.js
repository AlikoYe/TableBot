import { api } from './client'

/** @returns {Promise<{telegram_id: number, name: string, username: string|null, role: string, is_owner: boolean, owned_venue_ids: number[]}>} */
export const getMe = () => api.get('/me')
