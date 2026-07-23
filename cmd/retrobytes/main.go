package main

import (
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/csrf"
	"github.com/gofiber/fiber/v2/middleware/helmet"
	"github.com/gofiber/fiber/v2/middleware/limiter"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/requestid"
	html "github.com/gofiber/template/html/v2"

	"retrobytes/internal/config"
	"retrobytes/internal/http/handlers"
	applog "retrobytes/internal/log"
	"retrobytes/internal/repos"
	"retrobytes/internal/services"
)

func main() {
	cfg := config.Load()

	// Optional file logging
	if cfg.LogFile != "" {
		f, err := os.OpenFile(cfg.LogFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			log.Printf("[warn] could not open log file %s: %v", cfg.LogFile, err)
		} else {
			mw := io.MultiWriter(os.Stdout, f)
			log.SetOutput(mw)
		}
	}

	db, err := repos.OpenDB(cfg.DBDSN)
	if err != nil {
		log.Fatal(err)
	}

	// Auth wiring
	userRepo := repos.NewUserRepo(db)
	authSvc := &services.AuthService{Users: userRepo}
	authH := &handlers.AuthHandler{Auth: authSvc}

	// Templates & app
	engine := html.New("./web/templates", ".html")
	engine.Reload(true)

	app := fiber.New(fiber.Config{
		Views: engine,
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			// Log and show a friendly message
			applog.Error(c, "server.error", err, nil)
			// Avoid leaking internals; best-effort render
			if rerr := c.Status(fiber.StatusInternalServerError).Render("notfound", fiber.Map{
				"Message": "Something went wrong. Please try again.",
			}); rerr != nil {
				return c.Status(fiber.StatusInternalServerError).SendString("Something went wrong. Please try again.")
			}
			return nil
		},
	})
	// Global body size guard
	app.Server().MaxRequestBodySize = 1 << 20 // 1 MiB

	// ---------- Middlewares ----------
	app.Use(requestid.New())
	app.Use(logger.New())
	app.Use(helmet.New())
	// Attach user to context if logged in (for templates/headers)
	app.Use(func(c *fiber.Ctx) error {
		if sid := c.Cookies("sid"); sid != "" {
			if u, err := authSvc.CurrentUser(sid); err == nil && u != nil {
				c.Locals("user", u)
			}
		}
		return c.Next()
	})
	app.Use(limiter.New(limiter.Config{
		Max:        60,
		Expiration: time.Minute,
		Next: func(c *fiber.Ctx) bool {
			p := string(c.Request().URI().Path())
			return strings.HasPrefix(p, "/static/") || strings.HasPrefix(p, "/media/")
		},
	}))
	app.Use(csrf.New(csrf.Config{
		KeyLookup:      "form:csrf",
		CookieName:     "csrf_",
		CookieSameSite: "Lax",
		CookieSecure:   false, // set true behind HTTPS
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			formTok := c.FormValue("csrf")
			applog.Security(c, "csrf.fail", map[string]any{"form": formTok})
			return c.Status(fiber.StatusForbidden).Render("notfound", fiber.Map{"Message": "Security check failed. Please refresh and try again."})
		},
	}))
	app.Use(func(c *fiber.Ctx) error {
		if tok := c.Locals("csrf"); tok != nil {
			c.Locals("CSRFToken", tok.(string))
		}
		return c.Next()
	})

	// ---------- Static assets ----------
	mediaDir := cfg.MediaDir
	if !filepath.IsAbs(mediaDir) {
		if abs, err := filepath.Abs(mediaDir); err == nil {
			mediaDir = abs
		}
	}
	log.Printf("[static] /static -> ./web/static")
	log.Printf("[static] /media  -> %s", mediaDir)

	app.Static("/static", "./web/static")
	// Guarded media to avoid traversal
	app.Get("/media/*", func(c *fiber.Ctx) error {
		path := c.Params("*")
		rawLower := strings.ToLower(path)
		// Block encoded traversal attempts as well as raw .. or null bytes
		if strings.Contains(rawLower, "..") || strings.Contains(rawLower, "%2e") || strings.Contains(rawLower, "\x00") {
			applog.Security(c, "media.traversal.block", map[string]any{"path": path})
			return c.SendStatus(fiber.StatusNotFound)
		}
		clean := filepath.Clean(path)
		if clean == "." || strings.Contains(clean, "..") || filepath.IsAbs(clean) {
			applog.Security(c, "media.traversal.block", map[string]any{"path": path})
			return c.SendStatus(fiber.StatusNotFound)
		}
		full := filepath.Join(mediaDir, clean)
		return c.SendFile(full, true)
	})

	// ---------- App handlers ----------
	deps := handlers.NewDeps(db, cfg, authSvc)

	// Public pages
	app.Get("/", deps.CategoryHandler.Home)
	app.Get("/search", limiter.New(limiter.Config{Max: 20, Expiration: time.Minute}), deps.SearchHandler.Search)
	app.Get("/category/:id", deps.CategoryHandler.List)

	// Product pages
	app.Get("/product", func(c *fiber.Ctx) error {
		return c.Status(404).Render("notfound", fiber.Map{"Message": "This item is no longer available"})
	})
	app.Get("/product/:id", deps.ProductHandler.Detail)

	// API
	api := app.Group("/api/v1")
	availLimiter := limiter.New(limiter.Config{
		Max:        15,
		Expiration: 30 * time.Second,
		KeyGenerator: func(c *fiber.Ctx) string {
			return c.IP() + "|avail"
		},
		LimitReached: func(c *fiber.Ctx) error {
			applog.Security(c, "rate.availability.hit", nil)
			return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{"error": "rate limit exceeded, retry soon"})
		},
	})
	api.Get("/availability", availLimiter, deps.InventoryHandler.Check)

	// Cart & Orders
	app.Get("/cart", deps.CartHandler.View)
	app.Post("/cart", deps.CartHandler.Add)
	app.Get("/checkout", deps.OrderHandler.Checkout)
	app.Post("/orders", deps.OrderHandler.Place)
	app.Get("/order/:id", deps.OrderHandler.View)
	app.Get("/orders", handlers.RequireUser(authSvc), deps.OrderHandler.History)

	// Wishlist
	app.Get("/wishlist", deps.WishlistHandler.List)
	app.Post("/wishlist", deps.WishlistHandler.Save)
	app.Post("/wishlist/delete", deps.WishlistHandler.Unsave)

	// Auth routes (login throttled)
	app.Get("/login", authH.LoginForm)
	app.Post("/login", limiter.New(limiter.Config{
		Max:        5,
		Expiration: 10 * time.Minute,
		LimitReached: func(c *fiber.Ctx) error {
			applog.Security(c, "rate.login.hit", nil)
			return c.Status(fiber.StatusTooManyRequests).Render("login", fiber.Map{"Err": "Too many attempts. Please try again later."})
		},
	}), authH.Login)
	app.Post("/logout", authH.Logout)

	// Admin
	ordRepo := repos.NewOrderRepo(db)
	invRepo := repos.NewInventoryRepo(db)
	adminH := &handlers.AdminHandler{OrderRepo: ordRepo, Inv: invRepo, Users: userRepo}

	admin := app.Group("/admin", handlers.RequireAdmin(authSvc))
	admin.Get("/", adminH.Dashboard)
	admin.Get("/orders", adminH.OrdersPage)
	admin.Post("/orders/:id/status", adminH.UpdateOrderStatus)
	admin.Get("/inventory", adminH.Inventory)
	admin.Post("/inventory", adminH.UpdateInventory)
	admin.Get("/users", adminH.UsersPage)
	admin.Post("/users/:id/delete", adminH.DeleteUser)

	// Health & 404
	app.Get("/healthz", func(c *fiber.Ctx) error { return c.JSON(fiber.Map{"ok": true}) })
	app.Use(func(c *fiber.Ctx) error {
		return c.Status(404).Render("notfound", fiber.Map{"Message": "Page not found"})
	})

	log.Fatal(app.Listen(":8081"))
}
