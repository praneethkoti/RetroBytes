package repos

import (
	"testing"

	"github.com/jmoiron/sqlx"
)

func countUsers(t *testing.T, db *sqlx.DB) int {
	t.Helper()
	var n int
	if err := db.Get(&n, `SELECT COUNT(*) FROM users`); err != nil {
		t.Fatalf("count users: %v", err)
	}
	return n
}

// SR-CONF-01: user seeding is opt-in. With no SEED_DEMO and no admin env, a
// fresh database ships zero accounts (no default credentials). With the envs
// set, the demo users and the admin account are seeded.
func TestSeedingIsOptIn(t *testing.T) {
	// Case 1: nothing opted in -> no users at all.
	t.Run("no env means no users", func(t *testing.T) {
		t.Setenv("SEED_DEMO", "")
		t.Setenv("ADMIN_EMAIL", "")
		t.Setenv("ADMIN_PASSWORD", "")

		db, err := OpenDB(":memory:")
		if err != nil {
			t.Fatalf("open db: %v", err)
		}
		if n := countUsers(t, db); n != 0 {
			t.Fatalf("expected 0 seeded users with no env, got %d", n)
		}
	})

	// Case 2: envs set -> demo users + admin seeded.
	t.Run("envs seed demo users and admin", func(t *testing.T) {
		t.Setenv("SEED_DEMO", "true")
		t.Setenv("ADMIN_EMAIL", "admin@retrobytes.test")
		t.Setenv("ADMIN_PASSWORD", "Passw0rd!")

		db, err := OpenDB(":memory:")
		if err != nil {
			t.Fatalf("open db: %v", err)
		}
		// 4 demo users + 1 admin
		if n := countUsers(t, db); n != 5 {
			t.Fatalf("expected 5 seeded users with envs set, got %d", n)
		}

		var role string
		if err := db.Get(&role, `SELECT role FROM users WHERE email=?`, "admin@retrobytes.test"); err != nil {
			t.Fatalf("admin lookup: %v", err)
		}
		if role != "ADMIN" {
			t.Fatalf("expected admin role ADMIN, got %q", role)
		}
	})

	// Case 3: only admin env (no demo) -> just the admin.
	t.Run("admin only", func(t *testing.T) {
		t.Setenv("SEED_DEMO", "")
		t.Setenv("ADMIN_EMAIL", "root@example.com")
		t.Setenv("ADMIN_PASSWORD", "Str0ng!Pass")

		db, err := OpenDB(":memory:")
		if err != nil {
			t.Fatalf("open db: %v", err)
		}
		if n := countUsers(t, db); n != 1 {
			t.Fatalf("expected 1 user (admin only), got %d", n)
		}
	})
}
