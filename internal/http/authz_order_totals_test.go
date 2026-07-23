package handlers_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/csrf"
	"github.com/gofiber/fiber/v2/middleware/limiter"
	"github.com/gofiber/fiber/v2/middleware/requestid"
	html "github.com/gofiber/template/html/v2"
	"github.com/jmoiron/sqlx"

	"retrobytes/internal/config"
	"retrobytes/internal/http/handlers"
	"retrobytes/internal/repos"
	"retrobytes/internal/services"
)

// Helper: minimal app for order placement with recompute check
func newOrderTotalsApp(t *testing.T) (*fiber.App, *sqlx.DB, *repos.OrderRepo, *repos.UserRepo) {
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
	app.Server().MaxRequestBodySize = 1 << 20
	app.Use(requestid.New())
	app.Use(limiter.New(limiter.Config{Max: 100, Expiration: 0}))
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
	app.Post("/cart", deps.CartHandler.Add)
	app.Post("/orders", deps.OrderHandler.Place)
	app.Get("/login", authH.LoginForm)

	return app, db, repos.NewOrderRepo(db), userRepo
}

func extractCookieTotals(resp *http.Response, name string) string {
	for _, c := range resp.Cookies() {
		if c.Name == name {
			return c.Value
		}
	}
	return ""
}

// SR-AUTHZ-02: ignore client price/total; recompute server-side
func TestOrderTotalsRecomputed(t *testing.T) {
	app, db, ordRepo, _ := newOrderTotalsApp(t)

	// Seed a cart with tampered price_at_add
	sid := "sid-tamper"
	_, _ = db.Exec(`INSERT INTO carts(id,session_id,updated_at) VALUES(?,?,CURRENT_TIMESTAMP)`, sid, sid)
	_, _ = db.Exec(`INSERT INTO cart_items(cart_id, product_id, qty, price_at_add, created_at) VALUES(?,?,?,?,CURRENT_TIMESTAMP)`,
		sid, "gbc-001", 2, 1.00) // tampered price $1 instead of real 129.99

	// Get CSRF token
	loginResp, _ := app.Test(httptest.NewRequest("GET", "/login", nil))
	csrfTok := extractCookieTotals(loginResp, "csrf_")
	if csrfTok == "" {
		t.Fatal("csrf token missing")
	}

	formOrder := strings.NewReader("csrf=" + csrfTok + "&region=20742&email=alice@retrobytes.test&name=Alice&fulfillment=delivery")
	reqOrder := httptest.NewRequest("POST", "/orders", formOrder)
	reqOrder.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	reqOrder.AddCookie(&http.Cookie{Name: "csrf_", Value: csrfTok})
	reqOrder.AddCookie(&http.Cookie{Name: "sid", Value: sid})
	respOrder, err := app.Test(reqOrder)
	if err != nil {
		t.Fatal(err)
	}
	if respOrder.StatusCode != http.StatusFound {
		body, _ := io.ReadAll(respOrder.Body)
		t.Fatalf("expected redirect on order, got %d body=%s", respOrder.StatusCode, body)
	}

	// Parse order id from redirect
	loc := respOrder.Header.Get("Location")
	if loc == "" {
		t.Fatal("no redirect location with order id")
	}
	parts := strings.Split(loc, "/")
	oid := parts[len(parts)-1]

	ord, _, err := ordRepo.Get(oid)
	if err != nil {
		t.Fatalf("get order: %v", err)
	}
	// Real price is 129.99; two items => > 1.01; tampered client price should be ignored
	if ord.Total <= 1.01 {
		t.Fatalf("order total not recomputed; got %v", ord.Total)
	}
}
