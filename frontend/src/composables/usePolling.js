import { onBeforeUnmount, onMounted } from 'vue'

/**
 * Calls `fn` every `ms` while the page is visible, and once when it becomes
 * visible again. Errors are swallowed: the caller keeps showing the last data.
 */
export function usePolling(fn, ms) {
  let timer = null

  const run = () => {
    if (document.hidden) return
    Promise.resolve(fn()).catch(() => {})
  }
  const onVisible = () => {
    if (!document.hidden) run()
  }

  onMounted(() => {
    timer = setInterval(run, ms)
    document.addEventListener('visibilitychange', onVisible)
  })
  onBeforeUnmount(() => {
    clearInterval(timer)
    document.removeEventListener('visibilitychange', onVisible)
  })
}
