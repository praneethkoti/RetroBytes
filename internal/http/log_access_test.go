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
	"github.com/jmoiron/sqlx"

	"retrobytes/internal/config"
	"retrobytes/internal/http/handlers"
	"retrobytes/internal/repos"
	"retrobytes/internal/services"
)

type accessLogEntry struct {
	Action string                 `json:"action"`
	Fields map[string]interface{} `json:"fields"`
}

type lockedBuf struct {
	b  *bytes.Buffer
	mu *sync.Mutex
}

func (l *lockedBuf) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.Write(p)
}

func captureAccessLogs(t *testing.T, fn func()) []accessLogEntry {
	t.Helper()
	var buf bytes.Buffer
	var mu sync.Mutex
	oldW := log.Writer()
	oldFlags := log.Flags()
	log.SetOutput(&lockedBuf{b: &buf, mu: &mu})
	log.SetFlags(0)
	defer func() {
		log.SetOutput(oldW)
		log.SetFlags(oldFlags)
	}()

	fn()

	var entries []accessLogEntry
	for _, line := range strings.Split(strings.TrimSpace(buf.String()), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var e accessLogEntry
		if err := json.Unmarshal([]byte(line), &e); err == nil {
			entries = append(entries, e)
		}
	}
	return entries
}

// Minimal app for access-denial logging
func newAccessLogApp(t *testing.T) (*fiber.App, *sqlx.DB, *repos.OrderRepo, *repos.UserRepo) {
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
	app.Get("/order/:id", deps.OrderHandler.View)
	app.Get("/login", authH.LoginForm)

	ordRepo := repos.NewOrderRepo(db)
	invRepo := repos.NewInventoryRepo(db)
	adminH := &handlers.AdminHandler{OrderRepo: ordRepo, Inv: invRepo, Users: userRepo}
	admin := app.Group("/admin", handlers.RequireAdmin(authSvc))
	admin.Get("/", adminH.Dashboard)

	return app, db, ordRepo, userRepo
}

// SR-LOG-02: access control denials are logged
func TestAccessDeniedLogs(t *testing.T) {
	app, _, ordRepo, userRepo := newAccessLogApp(t)

	// Prepare order owned by sid-owner
	if err := userRepo.BindSession("sid-owner", "u-alice"); err != nil {
		t.Fatalf("bind owner session: %v", err)
	}
	if err := ordRepo.Create("oid-1", "sid-owner", "20742", "delivery", "Alice", "a@x.com", 10); err != nil {
		t.Fatalf("create order: %v", err)
	}
	if err := ordRepo.InsertItem("oid-1", "gbc-001", 1, 10, "SECOND_HAND"); err != nil {
		t.Fatalf("insert item: %v", err)
	}

	// Non-owner access should log access.denied.order
	entries := captureAccessLogs(t, func() {
		req := httptest.NewRequest("GET", "/order/oid-1", nil)
		req.AddCookie(&http.Cookie{Name: "sid", Value: "sid-other"})
		_, _ = app.Test(req)
	})
	foundOrder := false
	for _, e := range entries {
		if e.Action == "access.denied.order" {
			foundOrder = true
			break
		}
	}
	if !foundOrder {
		t.Fatalf("expected access.denied.order log")
	}

	// Non-admin hitting /admin should log access.denied.admin
	entries2 := captureAccessLogs(t, func() {
		req := httptest.NewRequest("GET", "/admin", nil)
		req.AddCookie(&http.Cookie{Name: "sid", Value: "sid-user"})
		_, _ = app.Test(req)
	})
	foundAdmin := false
	for _, e := range entries2 {
		if e.Action == "access.denied.admin" {
			foundAdmin = true
			break
		}
	}
	if !foundAdmin {
		t.Fatalf("expected access.denied.admin log")
	}
}
