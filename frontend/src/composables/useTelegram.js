import { readonly, ref } from 'vue'

// telegram-web-app.js defines window.Telegram.WebApp even in a normal
// browser, but initData is only non-empty when opened from Telegram.
const webApp = typeof window !== 'undefined' ? (window.Telegram?.WebApp ?? null) : null
const isTelegram = Boolean(webApp?.initData)

const prefersDark = () =>
  typeof window !== 'undefined' && window.matchMedia?.('(prefers-color-scheme: dark)').matches

const colorScheme = ref(isTelegram ? webApp.colorScheme : prefersDark() ? 'dark' : 'light')

let initialized = false

/** Call once at startup, before mounting the app. */
export function initTelegram() {
  if (initialized) return
  initialized = true

  if (isTelegram) {
    webApp.ready()
    webApp.expand()
    webApp.onEvent('themeChanged', () => {
      colorScheme.value = webApp.colorScheme
    })
  } else {
    window.matchMedia?.('(prefers-color-scheme: dark)').addEventListener?.('change', (e) => {
      colorScheme.value = e.matches ? 'dark' : 'light'
    })
  }
  document.documentElement.dataset.telegram = String(isTelegram)
}

/**
 * Shows Telegram's MainButton. Returns a cleanup function that hides it and
 * removes the handler; call it from onBeforeUnmount. No-op outside Telegram,
 * where views render a regular button instead.
 */
function showMainButton(text, onClick) {
  if (!isTelegram) return () => {}
  const mb = webApp.MainButton
  mb.setText(text)
  mb.onClick(onClick)
  mb.enable()
  mb.show()
  return () => {
    mb.offClick(onClick)
    mb.hideProgress()
    mb.hide()
  }
}

function setMainButtonLoading(loading) {
  if (!isTelegram) return
  const mb = webApp.MainButton
  if (loading) {
    mb.showProgress(false)
    mb.disable()
  } else {
    mb.hideProgress()
    mb.enable()
  }
}

/** Shows Telegram's BackButton. Returns a cleanup function. */
function showBackButton(onClick) {
  if (!isTelegram) return () => {}
  const bb = webApp.BackButton
  bb.onClick(onClick)
  bb.show()
  return () => {
    bb.offClick(onClick)
    bb.hide()
  }
}

/** Native confirm dialog in Telegram, window.confirm elsewhere. */
function confirmDialog(message) {
  if (!isTelegram) return Promise.resolve(window.confirm(message))
  return new Promise((resolve) => webApp.showConfirm(message, (ok) => resolve(Boolean(ok))))
}

export function useTelegram() {
  return {
    webApp,
    isTelegram,
    /** Raw signed initData string, sent to the API as "Authorization: tma …". */
    initData: isTelegram ? webApp.initData : '',
    /** Unverified user from initDataUnsafe; use /api/me for trusted data. */
    user: isTelegram ? (webApp.initDataUnsafe?.user ?? null) : null,
    colorScheme: readonly(colorScheme),
    showMainButton,
    setMainButtonLoading,
    showBackButton,
    confirmDialog,
  }
}
