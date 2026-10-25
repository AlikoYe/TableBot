package auth

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// UserRepo is the PostgreSQL UserStore.
type UserRepo struct {
	db *pgxpool.Pool
}

func NewUserRepo(db *pgxpool.Pool) *UserRepo {
	return &UserRepo{db: db}
}

// Upsert inserts the user or refreshes their name and username, and returns their role.
func (r *UserRepo) Upsert(ctx context.Context, u User) (string, error) {
	var username *string
	if u.Username != "" {
		username = &u.Username
	}
	var role string
	err := r.db.QueryRow(ctx, `
		INSERT INTO users (telegram_id, name, username)
		VALUES ($1, $2, $3)
		ON CONFLICT (telegram_id) DO UPDATE
		SET name = EXCLUDED.name, username = EXCLUDED.username, updated_at = now()
		RETURNING role`,
		u.ID, u.DisplayName(), username).Scan(&role)
	if err != nil {
		return "", fmt.Errorf("upsert user: %w", err)
	}
	return role, nil
}
