package handlers_test

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/csrf"
	"github.com/gofiber/fiber/v2/middleware/limiter"
	"github.com/gofiber/fiber/v2/middleware/requestid"
	html "github.com/gofiber/template/html/v2"

	"retrobytes/internal/config"
	"retrobytes/internal/http/handlers"
	"retrobytes/internal/repos"
	"retrobytes/internal/services"
)

// Minimal app with real routes and rate/body size limits
func newRateSizeApp(t *testing.T) *fiber.App {
	t.Helper()
	cfg := config.Config{DBDSN: ":memory:", MediaDir: "../../web/media"}
	db, err := repos.OpenDB(cfg.DBDSN)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	userRepo := repos.NewUserRepo(db)
	authSvc := &services.AuthService{Users: userRepo}
	authH := &handlers.AuthHandler{Auth: authSvc}

	engine := html.New("../../web/templates", ".html")
	app := fiber.New(fiber.Config{Views: engine})
	app.Server().MaxRequestBodySize = 1 << 20 // 1 MiB
	app.Use(requestid.New())
	app.Use(csrf.New(csrf.Config{KeyLookup: "form:csrf", CookieName: "csrf_", CookieSameSite: "Lax"}))
	app.Use(func(c *fiber.Ctx) error {
		if sid := c.Cookies("sid"); sid != "" {
			if u, err := authSvc.CurrentUser(sid); err == nil && u != nil {
				c.Locals("user", u)
			}
		}
		return c.Next()
	})

	deps := handlers.NewDeps(db, cfg, authSvc)

	// Rate-limited routes
	app.Get("/search", limiter.New(limiter.Config{Max: 3, Expiration: time.Second}), deps.SearchHandler.Search)
	api := app.Group("/api/v1")
	api.Get("/availability", limiter.New(limiter.Config{Max: 3, Expiration: time.Second}), deps.InventoryHandler.Check)

	// Cart/orders
	app.Post("/cart", deps.CartHandler.Add)
	app.Post("/orders", deps.OrderHandler.Place)
	app.Get("/login", authH.LoginForm)
	return app
}

// SR-RATE-01: burst hits return 429
func TestRateLimits(t *testing.T) {
	app := newRateSizeApp(t)

	// Hit availability more than limit
	for i := 0; i < 4; i++ {
		req := httptest.NewRequest("GET", "/api/v1/availability?productId=gbc-001&region=20742", nil)
		resp, err := app.Test(req)
		if err != nil {
			t.Fatal(err)
		}
		if i < 3 && resp.StatusCode == http.StatusTooManyRequests {
			t.Fatalf("hit rate limit too early at %d", i)
		}
		if i == 3 && resp.StatusCode != http.StatusTooManyRequests {
			t.Fatalf("expected 429 after limit, got %d", resp.StatusCode)
		}
	}

	// Hit search more than limit
	for i := 0; i < 4; i++ {
		req := httptest.NewRequest("GET", "/search?q=test", nil)
		resp, err := app.Test(req)
		if err != nil {
			t.Fatal(err)
		}
		if i < 3 && resp.StatusCode == http.StatusTooManyRequests {
			t.Fatalf("search limit too early at %d", i)
		}
		if i == 3 && resp.StatusCode != http.StatusTooManyRequests {
			t.Fatalf("expected 429 after search limit, got %d", resp.StatusCode)
		}
	}
}

// SR-SIZE-01: oversized POST rejected with 413
func TestBodySizeLimit(t *testing.T) {
	app := newRateSizeApp(t)

	// get csrf token
	respLogin, _ := app.Test(httptest.NewRequest("GET", "/login", nil))
	csrfTok := ""
	for _, c := range respLogin.Cookies() {
		if c.Name == "csrf_" {
			csrfTok = c.Value
			break
		}
	}
	if csrfTok == "" {
		t.Fatal("csrf token missing")
	}

	// Oversized body (>1MiB)
	oversize := bytes.Repeat([]byte("A"), (1<<20)+10)
	req := httptest.NewRequest("POST", "/cart", bytes.NewReader(oversize))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: "csrf_", Value: csrfTok})
	resp, err := app.Test(req)
	// Fiber returns an error instead of a response when body too large; treat that as pass
	if err != nil {
		if strings.Contains(err.Error(), "body size exceeds") || strings.Contains(err.Error(), "too large") {
			return
		}
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusRequestEntityTooLarge {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 413 for oversize, got %d body=%s", resp.StatusCode, string(body))
	}
}
