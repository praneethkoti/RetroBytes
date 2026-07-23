package handlers

import (
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

// sidCookieSecure controls the Secure flag on the "sid" session cookie.
// It defaults to false so local http development works, and is set to true
// from config (COOKIE_SECURE) when the app runs behind HTTPS. Configure it
// once at startup via SetCookieSecure before serving requests.
var sidCookieSecure = false

// SetCookieSecure sets whether session cookies carry the Secure flag.
// Call this once during wiring (see NewDeps and main) from config.CookieSecure.
func SetCookieSecure(secure bool) { sidCookieSecure = secure }

// writeSIDCookie writes the session id cookie with the standard flags.
// HttpOnly is always on (JS cannot read it); Secure follows configuration so
// production deployments over HTTPS mark the cookie Secure without breaking
// local http testing.
func writeSIDCookie(c *fiber.Ctx, sid string) {
	c.Cookie(&fiber.Cookie{
		Name:     "sid",
		Value:    sid,
		Path:     "/",
		HTTPOnly: true,
		SameSite: fiber.CookieSameSiteLaxMode,
		Secure:   sidCookieSecure,
	})
}

// ensureSID returns the caller's session id, minting and setting a new one if
// no sid cookie is present. Shared by all handlers so cookie flags stay
// consistent in one place.
func ensureSID(c *fiber.Ctx) string {
	sid := c.Cookies("sid")
	if sid == "" {
		sid = uuid.NewString()
		writeSIDCookie(c, sid)
	}
	return sid
}
