// The API returns venue-local timestamps with an offset (e.g. 2026-10-27T19:00:00+05:00).
// Reading the wall-clock parts straight from the string shows the venue's time
// regardless of the phone's time zone.

const LOCALE = 'en-GB'
const pad = (n) => String(n).padStart(2, '0')

const dayFormat = (options) => new Intl.DateTimeFormat(LOCALE, { timeZone: 'UTC', ...options })
const asDate = (iso) => new Date(`${iso.slice(0, 10)}T12:00:00Z`)

/** Today's calendar date (YYYY-MM-DD) in an IANA time zone. */
export function todayIn(timeZone) {
  try {
    return new Intl.DateTimeFormat('en-CA', {
      timeZone,
      year: 'numeric',
      month: '2-digit',
      day: '2-digit',
    }).format(new Date())
  } catch {
    const d = new Date()
    return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`
  }
}

export function addDays(dateISO, n) {
  const [y, m, d] = dateISO.split('-').map(Number)
  const dt = new Date(Date.UTC(y, m - 1, d + n))
  return `${dt.getUTCFullYear()}-${pad(dt.getUTCMonth() + 1)}-${pad(dt.getUTCDate())}`
}

export const dateOf = (iso) => iso.slice(0, 10)
export const timeOf = (iso) => iso.slice(11, 16)

export const weekdayShort = (iso) => dayFormat({ weekday: 'short' }).format(asDate(iso))
export const dayOfMonth = (iso) => Number(iso.slice(8, 10))
export const monthShort = (iso) => dayFormat({ month: 'short' }).format(asDate(iso))
export const formatDay = (iso) =>
  dayFormat({ weekday: 'short', day: 'numeric', month: 'short' }).format(asDate(iso))

/** "Today", "Tomorrow" or "Tue, 27 Oct", relative to the venue's today. */
export function relativeDay(dateISO, todayISO) {
  if (dateISO === todayISO) return 'Today'
  if (dateISO === addDays(todayISO, 1)) return 'Tomorrow'
  return formatDay(dateISO)
}

export const formatRange = (b) => `${timeOf(b.starts_at)}–${timeOf(b.ends_at)}`

/** "Tue, 27 Oct · 19:00–21:00" */
export const formatWhen = (b) => `${formatDay(b.starts_at)} · ${formatRange(b)}`

export function formatDuration(minutes) {
  if (minutes % 60 === 0) return `${minutes / 60} h`
  if (minutes > 60) return `${Math.floor(minutes / 60)} h ${minutes % 60} min`
  return `${minutes} min`
}

/** Whole minutes from `now` until an ISO timestamp (rounded up, may be negative). */
export const minutesUntil = (iso, now = Date.now()) => Math.ceil((Date.parse(iso) - now) / 60000)

export function formatCountdown(iso, now = Date.now()) {
  const m = minutesUntil(iso, now)
  if (m <= 0) return 'expiring'
  return `${m} min left`
}

export const plural = (n, one, many = `${one}s`) => `${n} ${n === 1 ? one : many}`
