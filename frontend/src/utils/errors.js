import { ApiError, NetworkError } from '@/api/client'

const messages = {
  table_taken: 'Someone just booked that table. Pick another one.',
  user_busy: 'You already have a booking that overlaps this time.',
  too_many_guests: 'That table is too small for your group.',
  outside_hours: 'That time is no longer available. Pick another time.',
  in_past: 'That time has already passed. Pick a later time.',
  not_pending: 'This request was already answered or has expired.',
  not_cancellable: 'This booking can no longer be cancelled.',
  forbidden: 'You do not manage this venue.',
  unauthorized: 'Your session has expired. Reopen the app from the bot.',
  not_found: 'We could not find that. It may have been removed.',
  bad_request: 'Something in the request was not valid. Check it and try again.',
  internal: 'Something went wrong on our side. Try again in a moment.',
  unavailable: 'The service is temporarily unavailable. Try again in a moment.',
}

/** Human-readable text for any error thrown by the API client. */
export function friendlyError(err) {
  if (err instanceof NetworkError) return 'No connection. Check your internet and try again.'
  if (err instanceof ApiError) return messages[err.code] ?? err.message
  return 'Something went wrong. Try again.'
}

export const errorCode = (err) => (err instanceof ApiError ? err.code : null)
