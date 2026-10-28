<script setup>
import { computed, onMounted, ref } from 'vue'

import { listVenues } from '@/api/venues'
import AsyncState from '@/components/AsyncState.vue'
import { formatDuration } from '@/utils/format'
import { categoryIcon, categoryLabel, categoryOrder, hoursText } from '@/utils/venue'

const venues = ref([])
const loading = ref(true)
const error = ref(null)
const category = ref('all')

async function load() {
  loading.value = true
  error.value = null
  try {
    venues.value = await listVenues()
  } catch (err) {
    error.value = err
  } finally {
    loading.value = false
  }
}

// Only offer filters for categories that have venues.
const categories = computed(() =>
  categoryOrder.filter((c) => venues.value.some((v) => v.category === c)),
)
const shown = computed(() =>
  category.value === 'all' ? venues.value : venues.value.filter((v) => v.category === category.value),
)

onMounted(load)
</script>

<template>
  <h1>Book a table</h1>

  <div v-if="categories.length > 1" class="filters" role="group" aria-label="Category">
    <button class="chip" type="button" :aria-pressed="category === 'all'" @click="category = 'all'">All</button>
    <button
      v-for="c in categories"
      :key="c"
      class="chip"
      type="button"
      :aria-pressed="category === c"
      @click="category = c"
    >
      {{ categoryLabel(c) }}
    </button>
  </div>

  <AsyncState
    :loading
    :error
    :empty="!shown.length"
    empty-text="No venues to book yet. Check back soon."
    @retry="load"
  >
    <RouterLink v-for="v in shown" :key="v.id" :to="{ name: 'venue', params: { id: v.id } }" class="card venue">
      <span class="icon" aria-hidden="true">{{ categoryIcon(v.category) }}</span>
      <span class="body">
        <strong>{{ v.name }}</strong>
        <span class="hint">{{ v.address }}</span>
        <span class="desc">{{ v.description }}</span>
        <span class="meta">Open {{ hoursText(v) }}</span>
      </span>
    </RouterLink>
  </AsyncState>
</template>

<style scoped>
.filters {
  display: flex;
  gap: 8px;
  overflow-x: auto;
  margin: 0 -16px 14px;
  padding: 0 16px 2px;
  scrollbar-width: none;
}
.filters::-webkit-scrollbar {
  display: none;
}
.venue {
  display: flex;
  gap: 14px;
  align-items: flex-start;
}
.icon {
  flex: none;
  width: 48px;
  height: 48px;
  display: grid;
  place-items: center;
  font-size: 1.6rem;
  border-radius: 12px;
  background: var(--secondary-bg);
}
.body {
  display: flex;
  flex-direction: column;
  gap: 2px;
  min-width: 0;
}
.body strong {
  font-size: 1.05rem;
}
.desc {
  display: -webkit-box;
  -webkit-line-clamp: 2;
  line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
  margin-top: 4px;
  font-size: 0.92rem;
}
.meta {
  margin-top: 6px;
  font-size: 0.82rem;
  color: var(--hint);
}
</style>
