package venue

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

var ErrLabelTaken = errors.New("a table with this label already exists at this venue")

// InvalidError is a validation failure; its message is safe to show to the client.
type InvalidError struct{ Msg string }

func (e *InvalidError) Error() string { return e.Msg }

func invalid(format string, args ...any) error {
	return &InvalidError{Msg: fmt.Sprintf(format, args...)}
}

// EditableTable is a table as the owner sees it in the editor, inactive ones included.
type EditableTable struct {
	ID       int64   `json:"id"`
	Label    string  `json:"label"`
	Seats    int     `json:"seats"`
	X        float64 `json:"x"`
	Y        float64 `json:"y"`
	Shape    string  `json:"shape"`
	IsActive bool    `json:"is_active"`
}

// VenuePatch changes venue settings; nil fields stay as they are.
type VenuePatch struct {
	Name            *string `json:"name"`
	Address         *string `json:"address"`
	Description     *string `json:"description"`
	OpensAt         *string `json:"opens_at"`
	ClosesAt        *string `json:"closes_at"`
	BookingMinutes  *int    `json:"booking_minutes"`
	SlotStepMinutes *int    `json:"slot_step_minutes"`
}

// TablePatch changes a table; nil fields stay as they are. Create requires all but IsActive.
type TablePatch struct {
	Label    *string  `json:"label"`
	Seats    *int     `json:"seats"`
	X        *float64 `json:"x"`
	Y        *float64 `json:"y"`
	Shape    *string  `json:"shape"`
	IsActive *bool    `json:"is_active"`
}

// SlotSteps are the start-time grids an owner can choose.
var SlotSteps = []int{15, 30, 60}

// ApplyVenuePatch returns v with p applied, or an *InvalidError.
func ApplyVenuePatch(v Venue, p VenuePatch) (Venue, error) {
	if p.Name != nil {
		v.Name = strings.TrimSpace(*p.Name)
	}
	if p.Address != nil {
		v.Address = strings.TrimSpace(*p.Address)
	}
	if p.Description != nil {
		v.Description = strings.TrimSpace(*p.Description)
	}
	if p.OpensAt != nil {
		m, err := parseClock(*p.OpensAt)
		if err != nil || m >= 24*60 {
			return Venue{}, invalid("opens_at must be HH:MM between 00:00 and 23:59")
		}
		v.OpensMinute = m
	}
	if p.ClosesAt != nil {
		m, err := parseClock(*p.ClosesAt)
		if err != nil || m == 0 {
			return Venue{}, invalid("closes_at must be HH:MM between 00:01 and 24:00")
		}
		v.ClosesMinute = m
	}
	if p.BookingMinutes != nil {
		v.BookingMinutes = *p.BookingMinutes
	}
	if p.SlotStepMinutes != nil {
		v.SlotStepMinutes = *p.SlotStepMinutes
	}

	switch {
	case v.Name == "" || utf8.RuneCountInString(v.Name) > 80:
		return Venue{}, invalid("name must be 1-80 characters")
	case utf8.RuneCountInString(v.Address) > 200:
		return Venue{}, invalid("address must be at most 200 characters")
	case utf8.RuneCountInString(v.Description) > 500:
		return Venue{}, invalid("description must be at most 500 characters")
	case v.ClosesMinute <= v.OpensMinute:
		return Venue{}, invalid("closes_at must be later than opens_at (overnight hours are not supported)")
	case !contains(SlotSteps, v.SlotStepMinutes):
		return Venue{}, invalid("slot_step_minutes must be 15, 30 or 60")
	case v.BookingMinutes < 15 || v.BookingMinutes > 720 || v.BookingMinutes%15 != 0:
		return Venue{}, invalid("booking_minutes must be a multiple of 15 between 15 and 720")
	case v.BookingMinutes > v.ClosesMinute-v.OpensMinute:
		return Venue{}, invalid("booking_minutes must fit between opens_at and closes_at")
	}
	v.OpensAt, v.ClosesAt = clock(v.OpensMinute), clock(v.ClosesMinute)
	return v, nil
}

// NewTable validates a create request; every field but IsActive is required.
func NewTable(p TablePatch) (EditableTable, error) {
	if p.Label == nil || p.Seats == nil || p.X == nil || p.Y == nil {
		return EditableTable{}, invalid("label, seats, x and y are required")
	}
	t := EditableTable{Shape: "round", IsActive: true}
	if p.Shape == nil {
		p.Shape = &t.Shape
	}
	return ApplyTablePatch(t, p)
}

// ApplyTablePatch returns t with p applied, or an *InvalidError.
func ApplyTablePatch(t EditableTable, p TablePatch) (EditableTable, error) {
	if p.Label != nil {
		t.Label = strings.TrimSpace(*p.Label)
	}
	if p.Seats != nil {
		t.Seats = *p.Seats
	}
	if p.X != nil {
		t.X = *p.X
	}
	if p.Y != nil {
		t.Y = *p.Y
	}
	if p.Shape != nil {
		t.Shape = *p.Shape
	}
	if p.IsActive != nil {
		t.IsActive = *p.IsActive
	}
	switch {
	case t.Label == "" || utf8.RuneCountInString(t.Label) > 10:
		return EditableTable{}, invalid("label must be 1-10 characters")
	case t.Seats < 1 || t.Seats > 20:
		return EditableTable{}, invalid("seats must be between 1 and 20")
	case !inPercent(t.X) || !inPercent(t.Y):
		return EditableTable{}, invalid("x and y must be between 0 and 100")
	case t.Shape != "round" && t.Shape != "rect":
		return EditableTable{}, invalid("shape must be round or rect")
	}
	t.X, t.Y = round2(t.X), round2(t.Y)
	return t, nil
}

// UpdateVenue saves the editable settings of an active venue (already validated).
func (r *Repo) UpdateVenue(ctx context.Context, v Venue) error {
	tag, err := r.db.Exec(ctx, `
		UPDATE venues SET name = $2, address = $3, description = $4,
			opens_at = $5::time, closes_at = $6::time, booking_minutes = $7, slot_step_minutes = $8
		WHERE id = $1 AND is_active`,
		v.ID, v.Name, v.Address, v.Description, v.OpensAt, v.ClosesAt, v.BookingMinutes, v.SlotStepMinutes)
	if err != nil {
		return fmt.Errorf("update venue: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

const editableColumns = `id, label, seats, x::float8, y::float8, shape, is_active`

func scanEditable(row pgx.Row) (EditableTable, error) {
	var t EditableTable
	err := row.Scan(&t.ID, &t.Label, &t.Seats, &t.X, &t.Y, &t.Shape, &t.IsActive)
	return t, err
}

// AllTables returns every table of the venue, inactive ones included.
func (r *Repo) AllTables(ctx context.Context, venueID int64) ([]EditableTable, error) {
	rows, err := r.db.Query(ctx, `SELECT `+editableColumns+` FROM venue_tables WHERE venue_id = $1 ORDER BY id`, venueID)
	if err != nil {
		return nil, fmt.Errorf("select tables: %w", err)
	}
	tables, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (EditableTable, error) { return scanEditable(row) })
	if err != nil {
		return nil, fmt.Errorf("scan tables: %w", err)
	}
	return tables, nil
}

// TableForEdit returns one table, inactive or not, and its venue id, or ErrNotFound.
func (r *Repo) TableForEdit(ctx context.Context, tableID int64) (int64, EditableTable, error) {
	var venueID int64
	var t EditableTable
	err := r.db.QueryRow(ctx, `SELECT venue_id, `+editableColumns+` FROM venue_tables WHERE id = $1`, tableID).
		Scan(&venueID, &t.ID, &t.Label, &t.Seats, &t.X, &t.Y, &t.Shape, &t.IsActive)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, EditableTable{}, ErrNotFound
	}
	if err != nil {
		return 0, EditableTable{}, fmt.Errorf("select table: %w", err)
	}
	return venueID, t, nil
}

// CreateTable inserts a validated table.
func (r *Repo) CreateTable(ctx context.Context, venueID int64, t EditableTable) (EditableTable, error) {
	out, err := scanEditable(r.db.QueryRow(ctx, `
		INSERT INTO venue_tables (venue_id, label, seats, x, y, shape, is_active)
		VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING `+editableColumns,
		venueID, t.Label, t.Seats, t.X, t.Y, t.Shape, t.IsActive))
	return out, tableWriteErr(err, "insert table")
}

// UpdateTable saves a validated table.
func (r *Repo) UpdateTable(ctx context.Context, t EditableTable) (EditableTable, error) {
	out, err := scanEditable(r.db.QueryRow(ctx, `
		UPDATE venue_tables SET label = $2, seats = $3, x = $4, y = $5, shape = $6, is_active = $7
		WHERE id = $1 RETURNING `+editableColumns,
		t.ID, t.Label, t.Seats, t.X, t.Y, t.Shape, t.IsActive))
	if errors.Is(err, pgx.ErrNoRows) {
		return EditableTable{}, ErrNotFound
	}
	return out, tableWriteErr(err, "update table")
}

func tableWriteErr(err error, op string) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == "venue_tables_venue_label_key" {
		return ErrLabelTaken
	}
	if err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}
	return nil
}

// parseClock reads HH:MM (00:00-24:00) as minutes since midnight.
func parseClock(s string) (int, error) {
	h, m, ok := strings.Cut(strings.TrimSpace(s), ":")
	if !ok || len(h) != 2 || len(m) != 2 {
		return 0, errors.New("bad clock")
	}
	hh, err1 := strconv.Atoi(h)
	mm, err2 := strconv.Atoi(m)
	if err1 != nil || err2 != nil || hh < 0 || mm < 0 || mm > 59 || hh > 24 || (hh == 24 && mm != 0) {
		return 0, errors.New("bad clock")
	}
	return hh*60 + mm, nil
}

func inPercent(v float64) bool { return !math.IsNaN(v) && v >= 0 && v <= 100 }

func round2(v float64) float64 { return math.Round(v*100) / 100 }

func contains(list []int, v int) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}
