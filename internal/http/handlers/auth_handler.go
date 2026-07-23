package handlers

import (
	"time"

	"retrobytes/internal/log"
	"retrobytes/internal/services"
	"retrobytes/internal/validate"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

type AuthHandler struct {
	Auth *services.AuthService
}

func ensureSID(c *fiber.Ctx) string {
	sid := c.Cookies("sid")
	if sid == "" {
		sid = uuid.NewString()
		c.Cookie(&fiber.Cookie{
			Name:     "sid",
			Value:    sid,
			Path:     "/",
			HTTPOnly: true,
			SameSite: fiber.CookieSameSiteLaxMode,
			Secure:   false,
		})
	}
	return sid
}

func (h *AuthHandler) LoginForm(c *fiber.Ctx) error {
	// Ensure CSRF token is injected for the form even if middleware locals are missing.
	tok, _ := c.Locals("CSRFToken").(string)
	if tok == "" {
		tok = c.Cookies("csrf_")
	}
	return render(c, "login", fiber.Map{"Err": "", "CSRFToken": tok})
}

func (h *AuthHandler) Login(c *fiber.Ctx) error {
	sid := ensureSID(c)
	email := c.FormValue("email")
	pass := c.FormValue("password")
	if _, ok := validate.Email(email); !ok {
		tok := c.Cookies("csrf_")
		log.Security(c, "auth.login.fail", map[string]any{"email": email, "reason": "bad_format"})
		return c.Status(401).Render("login", fiber.Map{"Err": "Invalid email or password", "CSRFToken": tok})
	}
	if !validate.Password(pass) {
		tok := c.Cookies("csrf_")
		log.Security(c, "auth.login.fail", map[string]any{"email": email, "reason": "bad_password_format"})
		return c.Status(401).Render("login", fiber.Map{"Err": "Invalid email or password", "CSRFToken": tok})
	}

	_, err := h.Auth.Login(sid, email, pass)
	if err != nil {
		tok := c.Cookies("csrf_")
		log.Security(c, "auth.login.fail", map[string]any{"email": email})
		return c.Status(401).Render("login", fiber.Map{"Err": "Invalid email or password", "CSRFToken": tok})
	}

	log.Audit(c, "auth.login.success", map[string]any{"email": email})
	return c.Redirect("/")
}

func (h *AuthHandler) Logout(c *fiber.Ctx) error {
	sid := ensureSID(c)
	_ = h.Auth.Logout(sid)
	// Expire cookie
	c.Cookie(&fiber.Cookie{
		Name:     "sid",
		Value:    "",
		Path:     "/",
		HTTPOnly: true,
		SameSite: fiber.CookieSameSiteLaxMode,
		Secure:   false,
		Expires:  time.Now().Add(-1 * time.Hour),
	})
	log.Audit(c, "auth.logout", map[string]any{"sid": sid})
	return c.Redirect("/")
}
