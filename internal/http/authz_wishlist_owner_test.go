package handlers_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/csrf"
	html "github.com/gofiber/template/html/v2"

	"retrobytes/internal/config"
	"retrobytes/internal/http/handlers"
	"retrobytes/internal/repos"
	"retrobytes/internal/services"
)

// loginAs logs in the given seeded user and returns the rotated, authenticated
// sid cookie value.
func loginAs(t *testing.T, app *fiber.App, csrfTok, email string) string {
	t.Helper()
	req := httptest.NewRequest("POST", "/login",
		strings.NewReader("csrf="+csrfTok+"&email="+email+"&password=Passw0rd!"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: "csrf_", Value: csrfTok})
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusFound {
		t.Fatalf("login for %s failed with %d", email, resp.StatusCode)
	}
	sid := extractCookieAuth(resp, "sid")
	if sid == "" {
		t.Fatalf("no sid after login for %s", email)
	}
	return sid
}

// SR-AUTHZ-08: the wishlist is scoped to the authenticated user's id, so two
// different users have independent wishlists. A product saved by user A must
// never appear in user B's wishlist, even though both are logged-in users.
func TestWishlistScopedToUser(t *testing.T) {
	db, err := repos.OpenDB(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	userRepo := repos.NewUserRepo(db)
	authSvc := &services.AuthService{Users: userRepo, SessionTTL: time.Hour}
	authH := &handlers.AuthHandler{Auth: authSvc}
	deps := handlers.NewDeps(db, config.Config{}, authSvc)

	engine := html.New("../../web/templates", ".html")
	app := fiber.New(fiber.Config{Views: engine})
	app.Use(csrf.New(csrf.Config{KeyLookup: "form:csrf", CookieName: "csrf_", CookieSameSite: "Lax"}))
	app.Get("/login", authH.LoginForm)
	app.Post("/login", authH.Login)
	app.Get("/wishlist", handlers.RequireUser(authSvc), deps.WishlistHandler.List)
	app.Post("/wishlist", handlers.RequireUser(authSvc), deps.WishlistHandler.Save)

	// Bootstrap a csrf token.
	respTok, _ := app.Test(httptest.NewRequest("GET", "/login", nil))
	csrfTok := extractCookieAuth(respTok, "csrf_")
	if csrfTok == "" {
		t.Fatal("csrf token missing")
	}

	// User A logs in and saves a product to their wishlist.
	sidA := loginAs(t, app, csrfTok, "alice@retrobytes.test")
	saveA := httptest.NewRequest("POST", "/wishlist",
		strings.NewReader("csrf="+csrfTok+"&productId=gbc-001"))
	saveA.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	saveA.AddCookie(&http.Cookie{Name: "csrf_", Value: csrfTok})
	saveA.AddCookie(&http.Cookie{Name: "sid", Value: sidA})
	if _, err := app.Test(saveA); err != nil {
		t.Fatal(err)
	}

	// Decisive check: the wishlist must be keyed by the user id (u-alice), not
	// by the session id. This is what fails if the handler keys by sid instead
	// of the authenticated user id.
	var wlKeyForUser int
	if err := db.Get(&wlKeyForUser, `SELECT COUNT(*) FROM wishlists WHERE id=?`, "u-alice"); err != nil {
		t.Fatalf("count wishlist by user id: %v", err)
	}
	if wlKeyForUser != 1 {
		t.Fatalf("expected wishlist keyed by user id u-alice, found %d (handler is not scoping by user id)", wlKeyForUser)
	}
	var wlKeyForSid int
	if err := db.Get(&wlKeyForSid, `SELECT COUNT(*) FROM wishlists WHERE id=?`, sidA); err != nil {
		t.Fatalf("count wishlist by sid: %v", err)
	}
	if wlKeyForSid != 0 {
		t.Fatalf("wishlist was keyed by session id (found %d), it should be keyed by user id", wlKeyForSid)
	}

	// User B logs in (a separate session) and views their wishlist.
	sidB := loginAs(t, app, csrfTok, "bob@retrobytes.test")
	viewB := httptest.NewRequest("GET", "/wishlist", nil)
	viewB.AddCookie(&http.Cookie{Name: "sid", Value: sidB})
	respB, err := app.Test(viewB)
	if err != nil {
		t.Fatal(err)
	}
	bbytes, _ := io.ReadAll(respB.Body)
	bodyB := string(bbytes)
	if strings.Contains(bodyB, "gbc-001") || strings.Contains(bodyB, "Game Boy") {
		t.Fatalf("user B's wishlist leaked user A's saved product; body=%s", bodyB)
	}

	// And user A still sees their own item.
	viewA := httptest.NewRequest("GET", "/wishlist", nil)
	viewA.AddCookie(&http.Cookie{Name: "sid", Value: sidA})
	respA, err := app.Test(viewA)
	if err != nil {
		t.Fatal(err)
	}
	abytes, _ := io.ReadAll(respA.Body)
	bodyA := string(abytes)
	if !strings.Contains(bodyA, "Game Boy") {
		t.Fatalf("user A's own wishlist did not contain their saved product; body=%s", bodyA)
	}
}

// TestWishlistRepoKeyedByOwner asserts at the repo level that two different
// owner ids yield independent wishlists (the direct binding, no HTTP layer).
func TestWishlistRepoKeyedByOwner(t *testing.T) {
	db, err := repos.OpenDB(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	repo := repos.NewWishlistRepo(db)
	svc := services.NewWishlistService(repo)

	if err := svc.Save("u-alice", "gbc-001"); err != nil {
		t.Fatalf("save for alice: %v", err)
	}

	bobItems, err := svc.List("u-bob")
	if err != nil {
		t.Fatalf("list for bob: %v", err)
	}
	if len(bobItems) != 0 {
		t.Fatalf("bob's wishlist should be empty, got %d items", len(bobItems))
	}

	aliceItems, err := svc.List("u-alice")
	if err != nil {
		t.Fatalf("list for alice: %v", err)
	}
	if len(aliceItems) != 1 || aliceItems[0].ProductID != "gbc-001" {
		t.Fatalf("alice's wishlist should have gbc-001, got %+v", aliceItems)
	}
}
