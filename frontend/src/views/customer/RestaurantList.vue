<script setup>
import { onMounted, ref } from 'vue'

import { listRestaurants } from '@/api/restaurants'
import AsyncState from '@/components/AsyncState.vue'

const restaurants = ref([])
const loading = ref(true)
const error = ref(null)

async function load() {
  loading.value = true
  error.value = null
  try {
    restaurants.value = (await listRestaurants()).items
  } catch (err) {
    error.value = err
  } finally {
    loading.value = false
  }
}

onMounted(load)
</script>

<template>
  <h1>Book a table</h1>
  <AsyncState :loading :error :empty="!restaurants.length" empty-text="No restaurants yet." @retry="load">
    <RouterLink
      v-for="r in restaurants"
      :key="r.id"
      :to="{ name: 'slot-picker', params: { id: r.id } }"
      class="card"
    >
      <strong>{{ r.name }}</strong>
      <div class="hint">{{ r.address }}</div>
      <div v-if="r.description">{{ r.description }}</div>
    </RouterLink>
  </AsyncState>
</template>
