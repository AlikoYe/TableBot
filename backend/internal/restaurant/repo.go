package restaurant

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Repo is the PostgreSQL data access layer for restaurants and slots.
// TODO: replace the mock data below with queries against restaurants and
// time_slots.
type Repo struct {
	db *pgxpool.Pool
}

func NewRepo(db *pgxpool.Pool) *Repo {
	return &Repo{db: db}
}

var mockRestaurants = []Restaurant{
	{ID: 1, Name: "Bistro Nomad", Address: "12 Abay Ave", Description: "Modern Central Asian cuisine.", Timezone: "Asia/Almaty"},
	{ID: 2, Name: "Trattoria Sole", Address: "5 Dostyk St", Description: "Wood-fired pizza and pasta.", Timezone: "Asia/Almaty"},
}

func (r *Repo) List(ctx context.Context) ([]Restaurant, error) {
	return mockRestaurants, nil
}

func (r *Repo) Get(ctx context.Context, id int64) (Restaurant, error) {
	for _, rest := range mockRestaurants {
		if rest.ID == id {
			return rest, nil
		}
	}
	return Restaurant{}, ErrNotFound
}

func (r *Repo) Slots(ctx context.Context, restaurantID int64, date time.Time) ([]Slot, error) {
	var slots []Slot
	for h := 18; h <= 21; h++ {
		for _, m := range []int{0, 30} {
			slots = append(slots, Slot{
				StartsAt:  time.Date(date.Year(), date.Month(), date.Day(), h, m, 0, 0, time.UTC),
				Available: 8,
			})
		}
	}
	return slots, nil
}
