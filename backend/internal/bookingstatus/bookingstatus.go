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
