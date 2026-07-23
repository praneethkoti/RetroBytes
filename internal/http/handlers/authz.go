package handlers

import (
	applog "retrobytes/internal/log"
	"retrobytes/internal/services"

	"github.com/gofiber/fiber/v2"
)

func RequireAdmin(auth *services.AuthService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		sid := c.Cookies("sid")
		if sid == "" {
			return c.Redirect("/login")
		}
		u, err := auth.CurrentUser(sid)
		if err != nil || u == nil || u.Role != "ADMIN" {
			applog.Security(c, "access.denied.admin", map[string]any{"sid": sid})
			return c.Status(fiber.StatusForbidden).Render("notfound", fiber.Map{"Message": "Access denied"})
		}
		c.Locals("user", u)
		return c.Next()
	}
}

// RequireUser enforces that a user is logged in; otherwise redirect to login.
func RequireUser(auth *services.AuthService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		sid := c.Cookies("sid")
		if sid == "" {
			return c.Redirect("/login")
		}
		u, err := auth.CurrentUser(sid)
		if err != nil || u == nil {
			return c.Redirect("/login")
		}
		c.Locals("user", u)
		return c.Next()
	}
}
