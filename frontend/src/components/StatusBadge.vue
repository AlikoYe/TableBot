<script setup>
import { computed } from 'vue'

const props = defineProps({ status: { type: String, required: true } })

const labels = {
  pending: 'Pending',
  confirmed: 'Confirmed',
  rejected: 'Declined',
  cancelled: 'Cancelled',
  expired: 'Expired',
}
const label = computed(() => labels[props.status] ?? props.status)
</script>

<template>
  <span class="badge" :data-status="status">{{ label }}</span>
</template>

<style scoped>
.badge {
  --c: var(--hint);
  display: inline-block;
  font-size: 0.75rem;
  font-weight: 600;
  padding: 2px 9px;
  border-radius: 999px;
  color: var(--c);
  background: color-mix(in srgb, var(--c) 14%, transparent);
}
.badge[data-status='pending'] {
  --c: var(--wait);
}
.badge[data-status='confirmed'] {
  --c: var(--ok);
}
.badge[data-status='rejected'] {
  --c: var(--destructive);
}
</style>
