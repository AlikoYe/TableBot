// Package venue serves venue listings, free start times and table availability.
package venue

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

// MaxDaysAhead is how far ahead /times accepts a date.
const MaxDaysAhead = 30

var (
	ErrNotFound    = errors.New("venue not found")
	ErrTooFarAhead = fmt.Errorf("date is more than %d days ahead", MaxDaysAhead)
)

// Categories mirror the venues.category CHECK constraint.
var Categories = []string{"restaurant", "cafe", "bar", "billiards", "karaoke", "other"}

// Table status values in the table map.
const (
	TableFree     = "free"
	TableTaken    = "taken"
	TableTooSmall = "too_small"
)

type Venue struct {
	ID              int64  `json:"id"`
	Name            string `json:"name"`
	Category        string `json:"category"`
	Address         string `json:"address"`
	Description     string `json:"description"`
	Timezone        string `json:"timezone"`
	OpensAt         string `json:"opens_at"`
	ClosesAt        string `json:"closes_at"`
	BookingMinutes  int    `json:"booking_minutes"`
	SlotStepMinutes int    `json:"slot_step_minutes"`
	FloorPlanURL    string `json:"floor_plan_url"`

	OpensMinute  int `json:"-"`
	ClosesMinute int `json:"-"`
}

// Schedule returns the venue's booking grid in its time zone.
func (v Venue) Schedule() (Schedule, error) {
	loc, err := Location(v.Timezone)
	if err != nil {
		return Schedule{}, err
	}
	return Schedule{
		Location:       loc,
		OpensMinute:    v.OpensMinute,
		ClosesMinute:   v.ClosesMinute,
		BookingMinutes: v.BookingMinutes,
		StepMinutes:    v.SlotStepMinutes,
	}, nil
}

type Table struct {
	ID     int64   `json:"id"`
	Label  string  `json:"label"`
	Seats  int     `json:"seats"`
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Shape  string  `json:"shape"`
	Status string  `json:"status,omitempty"`
}

type TimeOption struct {
	StartsAt   time.Time `json:"starts_at"`
	EndsAt     time.Time `json:"ends_at"`
	FreeTables int       `json:"free_tables"`
}

type TableMap struct {
	VenueID      int64     `json:"venue_id"`
	StartsAt     time.Time `json:"starts_at"`
	EndsAt       time.Time `json:"ends_at"`
	FloorPlanURL string    `json:"floor_plan_url"`
	Tables       []Table   `json:"tables"`
}

type Service struct {
	repo *Repo
	now  func() time.Time
}

func NewService(repo *Repo) *Service {
	return &Service{repo: repo, now: time.Now}
}

func (s *Service) List(ctx context.Context, category string) ([]Venue, error) {
	return s.repo.List(ctx, category)
}

func (s *Service) Get(ctx context.Context, id int64) (Venue, error) {
	return s.repo.Get(ctx, id)
}

// BookableTable returns an active table and its active venue, or ErrNotFound.
func (s *Service) BookableTable(ctx context.Context, tableID int64) (Venue, Table, error) {
	return s.repo.TableWithVenue(ctx, tableID)
}

// Times lists future start times on the venue-local date with the number of free tables for guests.
func (s *Service) Times(ctx context.Context, venueID int64, date time.Time, guests int) ([]TimeOption, error) {
	v, err := s.repo.Get(ctx, venueID)
	if err != nil {
		return nil, err
	}
	sched, err := v.Schedule()
	if err != nil {
		return nil, err
	}
	now := s.now()
	today := now.In(sched.Location)
	y, m, d := date.Date()
	days := time.Date(y, m, d, 0, 0, 0, 0, time.UTC).Sub(time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, time.UTC))
	if days > MaxDaysAhead*24*time.Hour {
		return nil, ErrTooFarAhead
	}

	starts := sched.StartTimes(y, m, d, now)
	out := make([]TimeOption, 0, len(starts))
	if len(starts) == 0 {
		return out, nil
	}
	tables, err := s.repo.Tables(ctx, venueID)
	if err != nil {
		return nil, err
	}
	spans, err := s.repo.blockingSpans(ctx, venueID, starts[0], starts[len(starts)-1].Add(sched.Duration()))
	if err != nil {
		return nil, err
	}
	for _, start := range starts {
		end := start.Add(sched.Duration())
		free := 0
		for _, t := range tables {
			if t.Seats >= guests && !taken(spans, t.ID, start, end) {
				free++
			}
		}
		out = append(out, TimeOption{StartsAt: start, EndsAt: end, FreeTables: free})
	}
	return out, nil
}

// Tables returns every active table with its status for [startsAt, startsAt + booking_minutes).
func (s *Service) Tables(ctx context.Context, venueID int64, startsAt time.Time, guests int) (TableMap, error) {
	v, err := s.repo.Get(ctx, venueID)
	if err != nil {
		return TableMap{}, err
	}
	sched, err := v.Schedule()
	if err != nil {
		return TableMap{}, err
	}
	if err := sched.Check(startsAt, s.now()); err != nil {
		return TableMap{}, err
	}
	start := startsAt.In(sched.Location)
	end := start.Add(sched.Duration())

	tables, err := s.repo.Tables(ctx, venueID)
	if err != nil {
		return TableMap{}, err
	}
	spans, err := s.repo.blockingSpans(ctx, venueID, start, end)
	if err != nil {
		return TableMap{}, err
	}
	for i, t := range tables {
		switch {
		case t.Seats < guests:
			tables[i].Status = TableTooSmall
		case taken(spans, t.ID, start, end):
			tables[i].Status = TableTaken
		default:
			tables[i].Status = TableFree
		}
	}
	return TableMap{VenueID: v.ID, StartsAt: start, EndsAt: end, FloorPlanURL: v.FloorPlanURL, Tables: tables}, nil
}

func taken(spans []span, tableID int64, start, end time.Time) bool {
	for _, sp := range spans {
		if sp.tableID == tableID && sp.overlaps(start, end) {
			return true
		}
	}
	return false
}

// clock formats minutes since midnight as HH:MM (1440 -> "24:00").
func clock(minutes int) string {
	return fmt.Sprintf("%02d:%02d", minutes/60, minutes%60)
}

var locations sync.Map

// Location is a cached time.LoadLocation.
func Location(name string) (*time.Location, error) {
	if loc, ok := locations.Load(name); ok {
		return loc.(*time.Location), nil
	}
	loc, err := time.LoadLocation(name)
	if err != nil {
		return nil, fmt.Errorf("load time zone %q: %w", name, err)
	}
	locations.Store(name, loc)
	return loc, nil
}
