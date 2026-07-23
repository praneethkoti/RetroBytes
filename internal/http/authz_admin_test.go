package handlers_test

import (
	"net/http"
	"net/http/httptest"
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

// Minimal app for admin guard testing
func newAdminApp(t *testing.T) (*fiber.App, *repos.UserRepo) {
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

	admin := app.Group("/admin", handlers.RequireAdmin(authSvc))
	admin.Get("/", func(c *fiber.Ctx) error { return c.SendStatus(http.StatusOK) })

	app.Get("/login", authH.LoginForm)
	return app, userRepo
}

// SR-AUTHZ-04: /admin requires ADMIN role
func TestAdminGuardRequiresAdmin(t *testing.T) {
	app, userRepo := newAdminApp(t)

	// Anonymous -> redirect or 403
	resp, err := app.Test(httptest.NewRequest("GET", "/admin", nil))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusFound && resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected redirect/forbidden, got %d", resp.StatusCode)
	}

	// Logged-in non-admin -> 403/redirect
	_ = userRepo.BindSession("sid-user", "u-alice")
	reqUser := httptest.NewRequest("GET", "/admin", nil)
	reqUser.AddCookie(&http.Cookie{Name: "sid", Value: "sid-user"})
	respUser, err := app.Test(reqUser)
	if err != nil {
		t.Fatal(err)
	}
	if respUser.StatusCode != http.StatusForbidden && respUser.StatusCode != http.StatusFound {
		t.Fatalf("expected forbidden/redirect for non-admin, got %d", respUser.StatusCode)
	}

	// Admin -> 200
	_ = userRepo.BindSession("sid-admin", "u-admin")
	reqAdmin := httptest.NewRequest("GET", "/admin", nil)
	reqAdmin.AddCookie(&http.Cookie{Name: "sid", Value: "sid-admin"})
	respAdmin, err := app.Test(reqAdmin)
	if err != nil {
		t.Fatal(err)
	}
	if respAdmin.StatusCode != http.StatusOK {
		t.Fatalf("admin expected 200, got %d", respAdmin.StatusCode)
	}
}
