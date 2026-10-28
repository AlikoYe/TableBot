import { onBeforeUnmount, ref } from 'vue'

/** A reactive Date.now() that ticks every `ms`, for countdowns. */
export function useNow(ms = 15000) {
  const now = ref(Date.now())
  const timer = setInterval(() => {
    now.value = Date.now()
  }, ms)
  onBeforeUnmount(() => clearInterval(timer))
  return now
}
