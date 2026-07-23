package handlers_test

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

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

type adminLogEntry struct {
	Action string                 `json:"action"`
	Fields map[string]interface{} `json:"fields"`
}

type lockedBufAdmin struct {
	b  *bytes.Buffer
	mu *sync.Mutex
}

func (l *lockedBufAdmin) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.Write(p)
}

func captureAdminLogs(t *testing.T, fn func()) []adminLogEntry {
	t.Helper()
	var buf bytes.Buffer
	var mu sync.Mutex
	oldW := log.Writer()
	oldFlags := log.Flags()
	log.SetOutput(&lockedBufAdmin{b: &buf, mu: &mu})
	log.SetFlags(0)
	defer func() {
		log.SetOutput(oldW)
		log.SetFlags(oldFlags)
	}()

	fn()

	var entries []adminLogEntry
	for _, line := range strings.Split(strings.TrimSpace(buf.String()), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var e adminLogEntry
		if err := json.Unmarshal([]byte(line), &e); err == nil {
			entries = append(entries, e)
		}
	}
	return entries
}

func extractCookieAdmin(resp *http.Response, name string) string {
	for _, c := range resp.Cookies() {
		if c.Name == name {
			return c.Value
		}
	}
	return ""
}

// SR-LOG-04: admin inventory changes logged
func TestAdminInventoryLogs(t *testing.T) {
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

	ordRepo := repos.NewOrderRepo(db)
	invRepo := repos.NewInventoryRepo(db)
	adminH := &handlers.AdminHandler{OrderRepo: ordRepo, Inv: invRepo, Users: userRepo}
	admin := app.Group("/admin", handlers.RequireAdmin(authSvc))
	admin.Post("/inventory", adminH.UpdateInventory)
	app.Get("/login", authH.LoginForm)

	// Bind admin session
	if err := userRepo.BindSession("sid-admin", "u-admin"); err != nil {
		t.Fatalf("bind admin session: %v", err)
	}

	// get csrf token
	respLogin, _ := app.Test(httptest.NewRequest("GET", "/login", nil))
	csrfTok := extractCookieAdmin(respLogin, "csrf_")
	if csrfTok == "" {
		t.Fatal("csrf token missing")
	}

	entries := captureAdminLogs(t, func() {
		form := strings.NewReader("csrf=" + csrfTok + "&product_id=gbc-001&region=20742&qty=9")
		req := httptest.NewRequest("POST", "/admin/inventory", form)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.AddCookie(&http.Cookie{Name: "csrf_", Value: csrfTok})
		req.AddCookie(&http.Cookie{Name: "sid", Value: "sid-admin"})
		_, _ = app.Test(req)
	})

	found := false
	for _, e := range entries {
		if e.Action == "admin.inventory.save" {
			found = true
			if _, ok := e.Fields["product"]; !ok {
				t.Fatalf("admin.inventory.save missing product")
			}
			if _, ok := e.Fields["region"]; !ok {
				t.Fatalf("admin.inventory.save missing region")
			}
			if _, ok := e.Fields["qty"]; !ok {
				t.Fatalf("admin.inventory.save missing qty")
			}
		}
	}
	if !found {
		t.Fatalf("admin.inventory.save log not found")
	}
}
