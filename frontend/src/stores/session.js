import { defineStore } from 'pinia'
import { computed, ref } from 'vue'

import { getMe } from '@/api/me'

export const useSessionStore = defineStore('session', () => {
  const user = ref(null)
  const role = ref(null)
  const error = ref(null)
  const loading = ref(false)
  let pending = null

  const isStaff = computed(() => role.value === 'staff' || role.value === 'admin')
  const loaded = computed(() => role.value !== null)

  /** Loads /api/me once; concurrent callers share the same request. */
  function load({ force = false } = {}) {
    if (loaded.value && !force) return Promise.resolve()
    if (pending) return pending

    loading.value = true
    error.value = null
    pending = getMe()
      .then((me) => {
        user.value = me.user
        role.value = me.role
      })
      .catch((err) => {
        error.value = err
      })
      .finally(() => {
        loading.value = false
        pending = null
      })
    return pending
  }

  return { user, role, error, loading, loaded, isStaff, load }
})
