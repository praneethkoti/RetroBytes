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

// Minimal app setup for validation tests
func newValidationApp(t *testing.T) (*fiber.App, *sqlx.DB, *repos.ProductRepo) {
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
	app.Get("/search", deps.SearchHandler.Search)
	app.Get("/product/:id", deps.ProductHandler.Detail)
	api := app.Group("/api/v1")
	api.Get("/availability", deps.InventoryHandler.Check)
	app.Post("/cart", deps.CartHandler.Add)
	app.Post("/orders", deps.OrderHandler.Place)
	app.Get("/login", authH.LoginForm)

	return app, db, repos.NewProductRepo(db)
}

func extractCookie(resp *http.Response, name string) string {
	for _, c := range resp.Cookies() {
		if c.Name == name {
			return c.Value
		}
	}
	return ""
}

// SR-VAL-01: reject malformed inputs early
func TestValidationBadInputs(t *testing.T) {
	app, _, _ := newValidationApp(t)

	// availability with bad region
	req := httptest.NewRequest("GET", "/api/v1/availability?productId=gbc-001&region=abc", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("bad region expected 400, got %d", resp.StatusCode)
	}

	// search with invalid chars
	req2 := httptest.NewRequest("GET", "/search?q=%3Cscript%3E", nil)
	resp2, err := app.Test(req2)
	if err != nil {
		t.Fatal(err)
	}
	if resp2.StatusCode != http.StatusBadRequest {
		t.Fatalf("bad search expected 400, got %d", resp2.StatusCode)
	}

	// order with invalid region (set up cart and csrf/sid)
	loginResp, _ := app.Test(httptest.NewRequest("GET", "/login", nil))
	csrfTok := extractCookie(loginResp, "csrf_")
	if csrfTok == "" {
		t.Fatal("csrf token missing")
	}
	formCart := strings.NewReader("csrf=" + csrfTok + "&productId=gbc-001&qty=1")
	reqCart := httptest.NewRequest("POST", "/cart", formCart)
	reqCart.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	reqCart.AddCookie(&http.Cookie{Name: "csrf_", Value: csrfTok})
	respCart, err := app.Test(reqCart)
	if err != nil {
		t.Fatal(err)
	}
	sid := extractCookie(respCart, "sid")
	if sid == "" {
		t.Fatal("sid not set after cart add")
	}

	formOrder := strings.NewReader("csrf=" + csrfTok + "&region=abc&email=alice@retrobytes.test&name=Alice&fulfillment=delivery")
	reqOrder := httptest.NewRequest("POST", "/orders", formOrder)
	reqOrder.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	reqOrder.AddCookie(&http.Cookie{Name: "csrf_", Value: csrfTok})
	reqOrder.AddCookie(&http.Cookie{Name: "sid", Value: sid})
	respOrder, err := app.Test(reqOrder)
	if err != nil {
		t.Fatal(err)
	}
	if respOrder.StatusCode != http.StatusBadRequest {
		body, _ := io.ReadAll(respOrder.Body)
		t.Fatalf("bad region order expected 400, got %d body=%s", respOrder.StatusCode, body)
	}
}

// SR-VAL-02: templates auto-escape untrusted text
func TestTemplateAutoEscape(t *testing.T) {
	app, db, _ := newValidationApp(t)
	// Insert a product with XSS-y fields
	_, _ = db.Exec(`
		INSERT INTO products(id,category_id,title,description,condition,price,images_json,active)
		VALUES('xss-1','retro-consoles','<script>alert(1)</script>','<b>desc</b>','SECOND_HAND',9.99,'[]',1)
	`)

	req := httptest.NewRequest("GET", "/product/xss-1", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	s := string(body)
	if strings.Contains(s, "<script>alert(1)</script>") {
		t.Fatalf("found unescaped script tag in output")
	}
	if !strings.Contains(s, "&lt;script&gt;alert(1)&lt;/script&gt;") {
		t.Fatalf("escaped script not found; output=%s", s)
	}
}
