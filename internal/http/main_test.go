package handlers_test

import (
	"os"
	"testing"
)

// TestMain seeds the demo/admin accounts the handler tests depend on.
// Production seeding is opt-in (SEED_DEMO plus ADMIN_EMAIL/ADMIN_PASSWORD), so
// tests enable it here rather than relying on hardcoded defaults. The admin
// email/password match the values the existing tests use.
func TestMain(m *testing.M) {
	os.Setenv("SEED_DEMO", "true")
	os.Setenv("ADMIN_EMAIL", "admin@retrobytes.test")
	os.Setenv("ADMIN_PASSWORD", "Passw0rd!")
	os.Exit(m.Run())
}
