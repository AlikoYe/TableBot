package venue

import (
	"errors"
	"time"
)

var (
	ErrOutsideHours = errors.New("start time is outside opening hours")
	ErrInPast       = errors.New("start time must be in the future")
)

// Schedule is a venue's booking grid. Minutes are since local midnight (0..1440).
type Schedule struct {
	Location       *time.Location
	OpensMinute    int
	ClosesMinute   int
	BookingMinutes int
	StepMinutes    int
}

// Duration is how long one booking lasts.
func (s Schedule) Duration() time.Duration {
	return time.Duration(s.BookingMinutes) * time.Minute
}

// grid returns every start time on the local date, past or not.
func (s Schedule) grid(year int, month time.Month, day int) []time.Time {
	if s.StepMinutes <= 0 {
		return nil
	}
	var out []time.Time
	for m := s.OpensMinute; m+s.BookingMinutes <= s.ClosesMinute; m += s.StepMinutes {
		out = append(out, time.Date(year, month, day, 0, m, 0, 0, s.Location))
	}
	return out
}

// StartTimes returns the start times on the local date that are after now.
func (s Schedule) StartTimes(year int, month time.Month, day int, now time.Time) []time.Time {
	all := s.grid(year, month, day)
	out := make([]time.Time, 0, len(all))
	for _, t := range all {
		if t.After(now) {
			out = append(out, t)
		}
	}
	return out
}

// Check returns ErrOutsideHours if startsAt is not on the grid, ErrInPast if it is not after now.
func (s Schedule) Check(startsAt, now time.Time) error {
	local := startsAt.In(s.Location)
	onGrid := false
	for _, t := range s.grid(local.Date()) {
		if t.Equal(startsAt) {
			onGrid = true
			break
		}
	}
	if !onGrid {
		return ErrOutsideHours
	}
	if !startsAt.After(now) {
		return ErrInPast
	}
	return nil
}
