package handlers_test

import (
	"bytes"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/csrf"
	html "github.com/gofiber/template/html/v2"

	"retrobytes/internal/config"
	"retrobytes/internal/http/handlers"
	applog "retrobytes/internal/log"
	"retrobytes/internal/repos"
	"retrobytes/internal/services"
)

// captureRawLogs returns everything written to the standard logger during fn as
// a single string, so tests can assert that sensitive substrings never appear.
func captureRawLogs(t *testing.T, fn func()) string {
	t.Helper()
	var buf bytes.Buffer
	var mu sync.Mutex
	oldW := log.Writer()
	oldFlags := log.Flags()
	log.SetOutput(&lockedWriter{w: &buf, mu: &mu})
	log.SetFlags(0)
	defer func() {
		log.SetOutput(oldW)
		log.SetFlags(oldFlags)
	}()
	fn()
	mu.Lock()
	defer mu.Unlock()
	return buf.String()
}

// SR-LOG-05: security logs must not contain the raw session id, the submitted
// CSRF token, or raw rejected search input (CWE-532). Drives a logout, a CSRF
// failure, and a search validation failure, then asserts none of those secret
// values appear in the log output.
func TestLogsDoNotLeakSensitiveData(t *testing.T) {
	db, err := repos.OpenDB(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	userRepo := repos.NewUserRepo(db)
	authSvc := &services.AuthService{Users: userRepo, SessionTTL: time.Hour}
	authH := &handlers.AuthHandler{Auth: authSvc}
	deps := handlers.NewDeps(db, config.Config{}, authSvc)

	engine := html.New("../../web/templates", ".html")
	app := fiber.New(fiber.Config{
		Views: engine,
		ErrorHandler: func(c *fiber.Ctx, e error) error {
			// Mirror production: CSRF failures land here and must not log the token.
			applog.Security(c, "csrf.fail", nil)
			return c.Status(fiber.StatusForbidden).SendString("Security check failed")
		},
	})
	app.Use(csrf.New(csrf.Config{
		KeyLookup:  "form:csrf",
		CookieName: "csrf_",
		ErrorHandler: func(c *fiber.Ctx, e error) error {
			applog.Security(c, "csrf.fail", nil)
			return c.Status(fiber.StatusForbidden).SendString("Security check failed")
		},
	}))
	app.Get("/csrf", func(c *fiber.Ctx) error { return c.SendString("ok") }) // token bootstrap
	app.Post("/logout", authH.Logout)
	app.Get("/search", deps.SearchHandler.Search)
	app.Post("/wishlist", deps.WishlistHandler.Save) // a CSRF-protected POST target

	// Bootstrap a valid CSRF token so the logout handler actually runs.
	respTok, _ := app.Test(httptest.NewRequest("GET", "/csrf", nil))
	csrfTok := extractCookieAuth(respTok, "csrf_")
	if csrfTok == "" {
		t.Fatal("csrf token missing")
	}

	const secretSID = "SECRET-SESSION-VALUE-123"
	// Contains a disallowed character ('<') so validate.Q rejects it, and a
	// recognizable marker we can assert is NOT logged.
	const evilSearch = "<script>SECRETPAYLOAD"

	out := captureRawLogs(t, func() {
		// 1) Logout carrying a known sid (with a valid CSRF token so it runs):
		//    the sid must not be logged.
		reqLogout := httptest.NewRequest("POST", "/logout",
			strings.NewReader("csrf="+csrfTok))
		reqLogout.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		reqLogout.AddCookie(&http.Cookie{Name: "csrf_", Value: csrfTok})
		reqLogout.AddCookie(&http.Cookie{Name: "sid", Value: secretSID})
		_, _ = app.Test(reqLogout)

		// 2) POST with a bad CSRF token: triggers csrf.fail; the submitted token
		//    must not be logged.
		reqCSRF := httptest.NewRequest("POST", "/wishlist",
			strings.NewReader("csrf=FORGED-CSRF-TOKEN-XYZ&productId=gbc-001"))
		reqCSRF.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		reqCSRF.AddCookie(&http.Cookie{Name: "csrf_", Value: csrfTok})
		_, _ = app.Test(reqCSRF)

		// 3) Search with invalid input: the raw rejected value must not be logged.
		_, _ = app.Test(httptest.NewRequest("GET", "/search?q="+url.QueryEscape(evilSearch), nil))
	})

	if strings.Contains(out, secretSID) {
		t.Fatalf("raw session id leaked into logs:\n%s", out)
	}
	if strings.Contains(out, "FORGED-CSRF-TOKEN-XYZ") {
		t.Fatalf("submitted CSRF token leaked into logs:\n%s", out)
	}
	if strings.Contains(out, evilSearch) {
		t.Fatalf("raw rejected search input leaked into logs:\n%s", out)
	}

	// Sanity: the events we expect were actually exercised (so the test is not
	// passing simply because nothing was logged).
	if !strings.Contains(out, "auth.logout") {
		t.Fatalf("expected an auth.logout log entry; logs:\n%s", out)
	}
	if !strings.Contains(out, "validation.fail") {
		t.Fatalf("expected a validation.fail log entry; logs:\n%s", out)
	}
}
