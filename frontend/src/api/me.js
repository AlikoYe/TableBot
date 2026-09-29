import { api } from './client'

/** @returns {Promise<{user: object, role: 'customer'|'staff'|'admin'}>} */
export const getMe = () => api.get('/me')
