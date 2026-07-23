package handlers_test

import (
	"io"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/requestid"
	html "github.com/gofiber/template/html/v2"

	applog "retrobytes/internal/log"
)

// SR-ERR-01: friendly error surface, no internal leakage
func TestErrorHandlerFriendlyMessage(t *testing.T) {
	engine := html.New("../../web/templates", ".html")
	app := fiber.New(fiber.Config{
		Views: engine,
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			applog.Error(c, "server.error", err, nil)
			if rerr := c.Status(fiber.StatusInternalServerError).Render("notfound", fiber.Map{
				"Message": "Something went wrong. Please try again.",
			}); rerr != nil {
				return c.Status(fiber.StatusInternalServerError).SendString("Something went wrong. Please try again.")
			}
			return nil
		},
	})
	app.Use(requestid.New())

	// Route that triggers an internal error
	app.Get("/err", func(c *fiber.Ctx) error {
		return fiber.NewError(fiber.StatusInternalServerError, "db timeout: secret trace")
	})

	req := httptest.NewRequest("GET", "/err", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("test request failed: %v", err)
	}
	if resp.StatusCode != fiber.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	s := string(body)
	if !strings.Contains(s, "Something went wrong") {
		t.Fatalf("friendly message missing; body=%s", s)
	}
	if strings.Contains(s, "db timeout") || strings.Contains(s, "secret") {
		t.Fatalf("internal details leaked to user; body=%s", s)
	}
}
