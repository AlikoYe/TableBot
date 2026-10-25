// Package bookingstatus is the single source of truth for which bookings hold a
// table and what status a reader sees. SQL fragments expect bookings aliased as "b".
package bookingstatus

// Values of bookings.status.
const (
	Pending   = "pending"
	Confirmed = "confirmed"
	Rejected  = "rejected"
	Cancelled = "cancelled"
	Expired   = "expired"
)

// BlockingSQL is true when booking b holds its table and its user.
const BlockingSQL = `(b.status = 'confirmed' OR (b.status = 'pending' AND b.expires_at > now()))`

// EffectiveSQL is the status to report: an overdue pending booking reads as expired.
const EffectiveSQL = `(CASE WHEN b.status = 'pending' AND b.expires_at <= now() THEN 'expired' ELSE b.status END)`

// ExpireOverdueSQL flips overdue holds on table $1 or user $2 to expired; run it before inserting.
const ExpireOverdueSQL = `
	UPDATE bookings SET status = 'expired', updated_at = now()
	WHERE status = 'pending' AND expires_at <= now() AND (table_id = $1 OR user_id = $2)`

// ExpireAllOverdueSQL flips every overdue hold to expired and returns the ids (worker).
const ExpireAllOverdueSQL = `
	UPDATE bookings SET status = 'expired', updated_at = now()
	WHERE status = 'pending' AND expires_at <= now()
	RETURNING id`

// DecideSQL sets $2 ('confirmed' or 'rejected') on booking $1 only while it is still a live hold.
const DecideSQL = `
	UPDATE bookings SET status = $2, decided_at = now(), updated_at = now()
	WHERE id = $1 AND status = 'pending' AND expires_at > now()`

// ClaimRemindersSQL marks confirmed bookings starting within $1 seconds as reminded and returns the ids.
const ClaimRemindersSQL = `
	UPDATE bookings SET reminder_sent = true, updated_at = now()
	WHERE status = 'confirmed' AND NOT reminder_sent
	  AND lower(during) > now() AND lower(during) <= now() + make_interval(secs => $1)
	RETURNING id`
