import { useTelegram } from '@/composables/useTelegram'

// Used only outside Telegram. The backend accepts it only when APP_ENV=dev.
const DEV_TELEGRAM_ID = import.meta.env.VITE_DEV_TELEGRAM_ID || '1'

export class ApiError extends Error {
  constructor(status, code, message) {
    super(message || code || `HTTP ${status}`)
    this.name = 'ApiError'
    this.status = status
    this.code = code
  }
}
export class BadRequestError extends ApiError {}
export class UnauthorizedError extends ApiError {}
export class ForbiddenError extends ApiError {}
export class NotFoundError extends ApiError {}
export class ConflictError extends ApiError {}
export class UnprocessableError extends ApiError {}

/** The request never got a response (offline, DNS, CORS, aborted…). */
export class NetworkError extends Error {
  constructor(cause) {
    super('Network error, please check your connection')
    this.name = 'NetworkError'
    this.cause = cause
  }
}

const errorClassByStatus = {
  400: BadRequestError,
  401: UnauthorizedError,
  403: ForbiddenError,
  404: NotFoundError,
  409: ConflictError,
  422: UnprocessableError,
}

function authorizationHeader() {
  const { isTelegram, initData } = useTelegram()
  return isTelegram ? `tma ${initData}` : `dev ${DEV_TELEGRAM_ID}`
}

function buildURL(path, query) {
  const url = new URL(`/api${path}`, window.location.origin)
  for (const [key, value] of Object.entries(query ?? {})) {
    if (value !== undefined && value !== null && value !== '') url.searchParams.set(key, value)
  }
  return url.pathname + url.search
}

/**
 * Performs an API request and returns the parsed JSON body.
 * Throws an ApiError subclass for non-2xx responses and NetworkError when
 * there is no response at all.
 */
export async function request(path, { method = 'GET', query, body, signal } = {}) {
  const headers = { Accept: 'application/json', Authorization: authorizationHeader() }
  if (body !== undefined) headers['Content-Type'] = 'application/json'

  let res
  try {
    res = await fetch(buildURL(path, query), {
      method,
      headers,
      body: body === undefined ? undefined : JSON.stringify(body),
      signal,
    })
  } catch (err) {
    if (err?.name === 'AbortError') throw err
    throw new NetworkError(err)
  }

  const isJSON = res.headers.get('content-type')?.includes('application/json')
  const data = res.status === 204 ? null : isJSON ? await res.json().catch(() => null) : await res.text()

  if (!res.ok) {
    const ErrorClass = errorClassByStatus[res.status] ?? ApiError
    const code = data?.error ?? 'http_error'
    const message = data?.message || (typeof data === 'string' ? data : '') || res.statusText
    throw new ErrorClass(res.status, code, message)
  }
  return data
}

export const api = {
  get: (path, opts) => request(path, { ...opts, method: 'GET' }),
  post: (path, body, opts) => request(path, { ...opts, method: 'POST', body }),
}
