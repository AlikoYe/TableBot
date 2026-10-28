<script setup>
import { friendlyError } from '@/utils/errors'

// Renders loading / error / empty states, or the default slot when ready.
defineProps({
  loading: { type: Boolean, default: false },
  error: { type: Object, default: null },
  empty: { type: Boolean, default: false },
  emptyText: { type: String, default: 'Nothing here yet.' },
})
defineEmits(['retry'])
</script>

<template>
  <div v-if="loading" class="skeleton" role="status" aria-label="Loading">
    <span /><span /><span />
  </div>
  <div v-else-if="error" class="banner error" role="alert">
    <p>{{ friendlyError(error) }}</p>
    <button class="btn btn-quiet btn-small" type="button" @click="$emit('retry')">Try again</button>
  </div>
  <p v-else-if="empty" class="hint empty">{{ emptyText }}</p>
  <slot v-else />
</template>

<style scoped>
.banner p {
  margin: 0 0 8px;
}
.empty {
  text-align: center;
  padding: 24px 0;
}
.skeleton {
  display: grid;
  gap: 10px;
}
.skeleton span {
  height: 64px;
  border-radius: var(--radius);
  background: linear-gradient(
    90deg,
    var(--section-bg) 25%,
    color-mix(in srgb, var(--section-bg) 85%, var(--hint)) 50%,
    var(--section-bg) 75%
  );
  background-size: 200% 100%;
  animation: shimmer 1.4s linear infinite;
}
@keyframes shimmer {
  to {
    background-position: -200% 0;
  }
}
</style>
