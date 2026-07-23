package handlers_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/csrf"
	"github.com/gofiber/fiber/v2/middleware/limiter"
	html "github.com/gofiber/template/html/v2"
	"golang.org/x/crypto/bcrypt"

	"retrobytes/internal/http/handlers"
	"retrobytes/internal/repos"
	"retrobytes/internal/services"
)

func extractCookieAuth(resp *http.Response, name string) string {
	for _, c := range resp.Cookies() {
		if c.Name == name {
			return c.Value
		}
	}
	return ""
}

// SR-AUTH-01: ensure seeded passwords are hashed (not plaintext).
func TestPasswordsSeededAreHashed(t *testing.T) {
	db, err := repos.OpenDB(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	var hashes []string
	if err := db.Select(&hashes, `SELECT password_hash FROM users`); err != nil {
		t.Fatalf("select hashes: %v", err)
	}
	if len(hashes) == 0 {
		t.Fatal("no users seeded")
	}
	for _, h := range hashes {
		if strings.Contains(h, "Passw0rd!") {
			t.Fatalf("hash contains plaintext password")
		}
		if !strings.HasPrefix(h, "$2") {
			t.Fatalf("unexpected hash format: %s", h)
		}
		if err := bcrypt.CompareHashAndPassword([]byte(h), []byte("Passw0rd!")); err != nil {
			t.Fatalf("seed hash does not validate known password: %v", err)
		}
	}
}

// SR-AUTH-02: login throttling + success/fail paths.
func TestLoginSuccessFailAndThrottle(t *testing.T) {
	// Minimal app with real login handler and per-route limiter
	db, err := repos.OpenDB(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	userRepo := repos.NewUserRepo(db)
	authSvc := &services.AuthService{Users: userRepo}
	authH := &handlers.AuthHandler{Auth: authSvc}
	engine := html.New("../../web/templates", ".html")
	app := fiber.New(fiber.Config{Views: engine})
	app.Use(csrf.New(csrf.Config{KeyLookup: "form:csrf", CookieName: "csrf_", CookieSameSite: "Lax"}))

	app.Get("/login", authH.LoginForm)
	app.Post("/login", limiter.New(limiter.Config{Max: 2, Expiration: time.Minute}), authH.Login)

	// fetch csrf token
	respLogin, _ := app.Test(httptest.NewRequest("GET", "/login", nil))
	csrfTok := extractCookieAuth(respLogin, "csrf_")
	if csrfTok == "" {
		t.Fatal("csrf token missing")
	}

	// bad password -> 401
	formBad := strings.NewReader("csrf=" + csrfTok + "&email=alice@retrobytes.test&password=wrongpass!")
	reqBad := httptest.NewRequest("POST", "/login", formBad)
	reqBad.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	reqBad.AddCookie(&http.Cookie{Name: "csrf_", Value: csrfTok})
	respBad, err := app.Test(reqBad)
	if err != nil {
		t.Fatal(err)
	}
	if respBad.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 for bad creds, got %d", respBad.StatusCode)
	}

	// good password -> redirect
	formGood := strings.NewReader("csrf=" + csrfTok + "&email=alice@retrobytes.test&password=Passw0rd!")
	reqGood := httptest.NewRequest("POST", "/login", formGood)
	reqGood.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	reqGood.AddCookie(&http.Cookie{Name: "csrf_", Value: csrfTok})
	respGood, err := app.Test(reqGood)
	if err != nil {
		t.Fatal(err)
	}
	if respGood.StatusCode != http.StatusFound {
		t.Fatalf("expected redirect on success, got %d", respGood.StatusCode)
	}

	// throttle after 2 attempts (we already did 2; a third should 429)
	formThird := strings.NewReader("csrf=" + csrfTok + "&email=alice@retrobytes.test&password=wrongpass!")
	reqThird := httptest.NewRequest("POST", "/login", formThird)
	reqThird.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	reqThird.AddCookie(&http.Cookie{Name: "csrf_", Value: csrfTok})
	respThird, err := app.Test(reqThird)
	if err != nil {
		t.Fatal(err)
	}
	if respThird.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("expected 429 after throttle, got %d", respThird.StatusCode)
	}
}
