package handlers_test

import (
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

// newWishlistAuthzApp wires the wishlist routes behind RequireUser plus a real
// login route, mirroring production route wiring.
func newWishlistAuthzApp(t *testing.T) *fiber.App {
	t.Helper()
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
	app.Post("/wishlist/delete", handlers.RequireUser(authSvc), deps.WishlistHandler.Unsave)
	return app
}

// SR-AUTHZ-07: wishlist writes require an authenticated user. An anonymous
// POST is redirected to /login (not applied), while a logged-in user succeeds.
func TestWishlistRequiresAuth(t *testing.T) {
	app := newWishlistAuthzApp(t)

	// Bootstrap a csrf token.
	respGet, _ := app.Test(httptest.NewRequest("GET", "/login", nil))
	csrfTok := extractCookieAuth(respGet, "csrf_")
	if csrfTok == "" {
		t.Fatal("csrf token missing")
	}

	// Anonymous POST /wishlist -> redirected to /login (RequireUser), not saved.
	anon := httptest.NewRequest("POST", "/wishlist",
		strings.NewReader("csrf="+csrfTok+"&productId=gbc-001"))
	anon.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	anon.AddCookie(&http.Cookie{Name: "csrf_", Value: csrfTok})
	respAnon, err := app.Test(anon)
	if err != nil {
		t.Fatal(err)
	}
	if respAnon.StatusCode != http.StatusFound {
		t.Fatalf("anonymous wishlist write should redirect to login (302), got %d", respAnon.StatusCode)
	}
	if loc := respAnon.Header.Get("Location"); loc != "/login" {
		t.Fatalf("expected redirect to /login, got %q", loc)
	}

	// Log in as a real user (sid is rotated to an authenticated session).
	login := httptest.NewRequest("POST", "/login",
		strings.NewReader("csrf="+csrfTok+"&email=alice@retrobytes.test&password=Passw0rd!"))
	login.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	login.AddCookie(&http.Cookie{Name: "csrf_", Value: csrfTok})
	respLogin, err := app.Test(login)
	if err != nil {
		t.Fatal(err)
	}
	if respLogin.StatusCode != http.StatusFound {
		t.Fatalf("login should redirect, got %d", respLogin.StatusCode)
	}
	authSID := extractCookieAuth(respLogin, "sid")
	if authSID == "" {
		t.Fatal("no sid after login")
	}

	// Authenticated POST /wishlist -> accepted (redirect back), not a login bounce.
	save := httptest.NewRequest("POST", "/wishlist",
		strings.NewReader("csrf="+csrfTok+"&productId=gbc-001"))
	save.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	save.AddCookie(&http.Cookie{Name: "csrf_", Value: csrfTok})
	save.AddCookie(&http.Cookie{Name: "sid", Value: authSID})
	respSave, err := app.Test(save)
	if err != nil {
		t.Fatal(err)
	}
	if respSave.StatusCode != http.StatusFound {
		t.Fatalf("authenticated wishlist write should redirect (302), got %d", respSave.StatusCode)
	}
	if loc := respSave.Header.Get("Location"); loc == "/login" {
		t.Fatal("authenticated wishlist write was bounced to /login")
	}
}
