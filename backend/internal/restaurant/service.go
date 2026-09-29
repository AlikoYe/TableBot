// Package restaurant serves restaurant listings and bookable time slots.
package restaurant

import (
	"context"
	"errors"
	"time"
)

var ErrNotFound = errors.New("restaurant not found")

type Restaurant struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Address     string `json:"address"`
	Description string `json:"description"`
	Timezone    string `json:"timezone"`
}

// Slot is a bookable start time and how many more guests it can take.
type Slot struct {
	StartsAt  time.Time `json:"starts_at"`
	Available int       `json:"available"`
}

type Service struct {
	repo *Repo
}

func NewService(repo *Repo) *Service {
	return &Service{repo: repo}
}

func (s *Service) List(ctx context.Context) ([]Restaurant, error) {
	return s.repo.List(ctx)
}

func (s *Service) Get(ctx context.Context, id int64) (Restaurant, error) {
	return s.repo.Get(ctx, id)
}

// Slots returns slots on the given date that can fit the party.
// TODO: interpret date in the restaurant's timezone, hide past slots,
// filter by remaining capacity >= guests.
func (s *Service) Slots(ctx context.Context, restaurantID int64, date time.Time, guests int) ([]Slot, error) {
	if _, err := s.repo.Get(ctx, restaurantID); err != nil {
		return nil, err
	}
	return s.repo.Slots(ctx, restaurantID, date)
}
