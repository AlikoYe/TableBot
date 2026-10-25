package venue

import (
	"errors"
	"testing"
	"time"
)

func almaty(t *testing.T) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation("Asia/Almaty")
	if err != nil {
		t.Fatal(err)
	}
	return loc
}

func hhmm(ts []time.Time) []string {
	out := make([]string, len(ts))
	for i, t := range ts {
		out[i] = t.Format("15:04")
	}
	return out
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// longAgo makes every start time "future".
var longAgo = time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)

func TestStartTimesGridEndsByClose(t *testing.T) {
	s := Schedule{Location: almaty(t), OpensMinute: 12 * 60, ClosesMinute: 15 * 60, BookingMinutes: 90, StepMinutes: 30}
	got := hhmm(s.StartTimes(2030, 6, 1, longAgo))
	want := []string{"12:00", "12:30", "13:00", "13:30"} // 13:30 + 90m = 15:00
	if !equal(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestStartTimesClosingAt2400(t *testing.T) {
	s := Schedule{Location: almaty(t), OpensMinute: 18 * 60, ClosesMinute: 24 * 60, BookingMinutes: 120, StepMinutes: 60}
	got := s.StartTimes(2030, 6, 1, longAgo)
	if want := []string{"18:00", "19:00", "20:00", "21:00", "22:00"}; !equal(hhmm(got), want) {
		t.Fatalf("got %v, want %v", hhmm(got), want)
	}
	last := got[len(got)-1]
	if end := last.Add(s.Duration()); !end.Equal(time.Date(2030, 6, 2, 0, 0, 0, 0, s.Location)) {
		t.Errorf("last booking ends %v, want next local midnight", end)
	}
}

func TestStartTimesAreVenueLocal(t *testing.T) {
	s := Schedule{Location: almaty(t), OpensMinute: 9 * 60, ClosesMinute: 11 * 60, BookingMinutes: 60, StepMinutes: 60}
	got := s.StartTimes(2030, 6, 1, longAgo)
	if got[0].Format(time.RFC3339) != "2030-06-01T09:00:00+05:00" {
		t.Fatalf("first start = %s, want 2030-06-01T09:00:00+05:00", got[0].Format(time.RFC3339))
	}
}

func TestStartTimesExcludePast(t *testing.T) {
	loc := almaty(t)
	s := Schedule{Location: loc, OpensMinute: 12 * 60, ClosesMinute: 18 * 60, BookingMinutes: 60, StepMinutes: 60}
	now := time.Date(2030, 6, 1, 14, 0, 0, 0, loc)
	if got, want := hhmm(s.StartTimes(2030, 6, 1, now)), []string{"15:00", "16:00", "17:00"}; !equal(got, want) {
		t.Fatalf("got %v, want %v (14:00 itself is not in the future)", got, want)
	}
	if got := s.StartTimes(2030, 5, 31, now); len(got) != 0 {
		t.Fatalf("past date: got %v, want none", hhmm(got))
	}
}

func TestCheck(t *testing.T) {
	loc := almaty(t)
	s := Schedule{Location: loc, OpensMinute: 12 * 60, ClosesMinute: 24 * 60, BookingMinutes: 120, StepMinutes: 30}
	now := time.Date(2030, 6, 1, 15, 10, 0, 0, loc)
	at := func(h, m int) time.Time { return time.Date(2030, 6, 1, h, m, 0, 0, loc) }

	tests := []struct {
		name string
		in   time.Time
		want error
	}{
		{"on grid", at(19, 0), nil},
		{"last slot", at(22, 0), nil},
		{"same instant in UTC", at(19, 30).UTC(), nil},
		{"same instant in another offset", at(19, 30).In(time.FixedZone("X", -3*3600)), nil},
		{"off grid", at(19, 15), ErrOutsideHours},
		{"seconds off grid", at(19, 0).Add(time.Second), ErrOutsideHours},
		{"before opening", at(11, 30), ErrOutsideHours},
		{"ends after close", at(22, 30), ErrOutsideHours},
		{"past slot", at(15, 0), ErrInPast},
		{"next slot after now", at(15, 30), nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := s.Check(tt.in, now); !errors.Is(err, tt.want) {
				t.Fatalf("Check(%s) = %v, want %v", tt.in.Format(time.RFC3339), err, tt.want)
			}
		})
	}
	if err := s.Check(at(15, 30), at(15, 30)); !errors.Is(err, ErrInPast) {
		t.Fatalf("start == now: %v, want ErrInPast", err)
	}
}
