<script setup>
// Renders loading / error / empty states, or the default slot when ready.
defineProps({
  loading: { type: Boolean, default: false },
  error: { type: Error, default: null },
  empty: { type: Boolean, default: false },
  emptyText: { type: String, default: 'Nothing here yet.' },
})
defineEmits(['retry'])
</script>

<template>
  <p v-if="loading" class="hint">Loading…</p>
  <div v-else-if="error" class="card error">
    <p>{{ error.message }}</p>
    <button class="btn-link btn" type="button" @click="$emit('retry')">Try again</button>
  </div>
  <p v-else-if="empty" class="hint">{{ emptyText }}</p>
  <slot v-else />
</template>

<style scoped>
.error p {
  margin: 0 0 4px;
  color: var(--destructive);
}
</style>
