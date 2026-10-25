package auth_test

import (
	"context"
	"testing"

	"github.com/AlikoYe/TableBot/backend/internal/auth"
	"github.com/AlikoYe/TableBot/backend/internal/platform/dbtest"
)

func TestUserRepoUpsert(t *testing.T) {
	db := dbtest.New(t)
	ctx := context.Background()
	repo := auth.NewUserRepo(db)

	role, err := repo.Upsert(ctx, auth.User{ID: 5, FirstName: "Ann", Username: "ann"})
	if err != nil || role != "customer" {
		t.Fatalf("first upsert: role=%q err=%v", role, err)
	}

	// Name and username are refreshed, role is kept.
	if _, err := db.Exec(ctx, `UPDATE users SET role = 'admin' WHERE telegram_id = 5`); err != nil {
		t.Fatal(err)
	}
	role, err = repo.Upsert(ctx, auth.User{ID: 5, FirstName: "Ann", LastName: "Lee"})
	if err != nil || role != "admin" {
		t.Fatalf("second upsert: role=%q err=%v", role, err)
	}

	var name string
	var username *string
	if err := db.QueryRow(ctx, `SELECT name, username FROM users WHERE telegram_id = 5`).Scan(&name, &username); err != nil {
		t.Fatal(err)
	}
	if name != "Ann Lee" || username != nil {
		t.Errorf("stored name=%q username=%v, want \"Ann Lee\" and NULL", name, username)
	}
}
