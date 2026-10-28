import { defineStore } from 'pinia'
import { computed, ref } from 'vue'

import { getMe } from '@/api/me'

export const useSessionStore = defineStore('session', () => {
  const me = ref(null)
  const error = ref(null)
  const loading = ref(false)
  let pending = null

  const loaded = computed(() => me.value !== null)
  // Owning a venue is per venue, not a role (see /api/me).
  const isOwner = computed(() => Boolean(me.value?.is_owner))
  const ownedVenueIds = computed(() => me.value?.owned_venue_ids ?? [])

  /** Loads /api/me once; concurrent callers share the same request. */
  function load({ force = false } = {}) {
    if (loaded.value && !force) return Promise.resolve()
    if (pending) return pending

    loading.value = true
    error.value = null
    pending = getMe()
      .then((data) => {
        me.value = data
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

  return { me, error, loading, loaded, isOwner, ownedVenueIds, load }
})
