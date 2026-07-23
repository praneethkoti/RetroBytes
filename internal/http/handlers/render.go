package handlers

import "github.com/gofiber/fiber/v2"

func render(c *fiber.Ctx, tmpl string, data fiber.Map) error {
	if data == nil {
		data = fiber.Map{}
	}
	// Inject user if present
	if u := c.Locals("user"); u != nil {
		data["User"] = u
	}
	// Pick up the token the CSRF middleware put into Locals
	tok, _ := c.Locals("CSRFToken").(string)
	if tok == "" {
		// Fallback: attempt to read the CSRF cookie directly if Locals wasn't populated
		// (e.g., edge cases or library changes). This helps avoid empty hidden fields.
		cookTok := c.Cookies("csrf_")
		if cookTok != "" {
			ok := true
			if ok {
				// Use cookie token as a last resort
				data["CSRFToken"] = cookTok
			}
		}
	}
	if tok != "" {
		data["CSRFToken"] = tok
	}
	return c.Render(tmpl, data)
}
