<script setup>
import { computed, onMounted, ref, watch } from 'vue'

import { listTables, listTimes } from '@/api/venues'
import AsyncState from '@/components/AsyncState.vue'
import FloorPlan from '@/components/FloorPlan.vue'
import { usePolling } from '@/composables/usePolling'
import { formatRange, plural, timeOf } from '@/utils/format'

// Read-only view of which tables are held at a start time (pending or confirmed).
const props = defineProps({
  venue: { type: Object, required: true },
  date: { type: String, required: true },
})

const times = ref([])
const selected = ref(null)
const tableMap = ref(null)
const loading = ref(true)
const error = ref(null)

const taken = computed(() => tableMap.value?.tables.filter((t) => t.status === 'taken').length ?? 0)
const total = computed(() => tableMap.value?.tables.length ?? 0)

async function loadTimes() {
  loading.value = true
  error.value = null
  try {
    times.value = await listTimes(props.venue.id, { date: props.date, guests: 1 })
    selected.value = times.value[0] ?? null
  } catch (err) {
    error.value = err
  } finally {
    loading.value = false
  }
}

async function loadTables() {
  if (!selected.value) {
    tableMap.value = null
    return
  }
  tableMap.value = await listTables(props.venue.id, { startsAt: selected.value.starts_at, guests: 1 })
}

watch(selected, () => {
  tableMap.value = null
  loadTables().catch((err) => (error.value = err))
})

usePolling(loadTables, 15000)
onMounted(loadTimes)
</script>

<template>
  <AsyncState
    :loading="loading"
    :error="error"
    :empty="!times.length"
    empty-text="No start times left on this day."
    @retry="loadTimes"
  >
    <h2 class="section-title"><span>Start time</span></h2>
    <div class="times" role="group" aria-label="Start time">
      <button
        v-for="t in times"
        :key="t.starts_at"
        class="chip"
        type="button"
        :aria-pressed="selected?.starts_at === t.starts_at"
        @click="selected = t"
      >
        {{ timeOf(t.starts_at) }}
      </button>
    </div>

    <template v-if="tableMap">
      <p class="hint summary">
        {{ formatRange(tableMap) }} · {{ plural(taken, 'table') }} booked of {{ total }}
      </p>
      <FloorPlan :plan-url="tableMap.floor_plan_url" :tables="tableMap.tables" readonly />
    </template>
  </AsyncState>
</template>

<style scoped>
.times {
  display: flex;
  gap: 8px;
  overflow-x: auto;
  margin: 0 -16px;
  padding: 0 16px 6px;
  scrollbar-width: none;
}
.times::-webkit-scrollbar {
  display: none;
}
.summary {
  margin: 10px 0 12px;
}
</style>
