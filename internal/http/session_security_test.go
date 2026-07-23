package handlers_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/csrf"
	html "github.com/gofiber/template/html/v2"

	"retrobytes/internal/http/handlers"
	"retrobytes/internal/repos"
	"retrobytes/internal/services"
)

// newSessionApp wires a minimal app with the real login handler for session tests.
func newSessionApp(t *testing.T) (*fiber.App, *repos.UserRepo) {
	t.Helper()
	db, err := repos.OpenDB(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	userRepo := repos.NewUserRepo(db)
	authSvc := &services.AuthService{Users: userRepo, SessionTTL: time.Hour}
	authH := &handlers.AuthHandler{Auth: authSvc}
	engine := html.New("../../web/templates", ".html")
	app := fiber.New(fiber.Config{Views: engine})
	app.Use(csrf.New(csrf.Config{KeyLookup: "form:csrf", CookieName: "csrf_", CookieSameSite: "Lax"}))
	app.Get("/login", authH.LoginForm)
	app.Post("/login", authH.Login)
	return app, userRepo
}

// SR-AUTHZ-05: the session id is rotated on login (session fixation defense).
// A sid fixed before login must NOT remain a valid authenticated session, and
// the cookie returned after login must carry a different sid value.
func TestSessionRotatedOnLogin(t *testing.T) {
	app, userRepo := newSessionApp(t)

	// fetch csrf token
	respLogin, _ := app.Test(httptest.NewRequest("GET", "/login", nil))
	csrfTok := extractCookieAuth(respLogin, "csrf_")
	if csrfTok == "" {
		t.Fatal("csrf token missing")
	}

	// Attacker-planted (fixed) session id present before login.
	const fixedSID = "attacker-fixed-sid"

	form := strings.NewReader("csrf=" + csrfTok + "&email=alice@retrobytes.test&password=Passw0rd!")
	req := httptest.NewRequest("POST", "/login", form)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: "csrf_", Value: csrfTok})
	req.AddCookie(&http.Cookie{Name: "sid", Value: fixedSID})

	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusFound {
		t.Fatalf("expected redirect on successful login, got %d", resp.StatusCode)
	}

	// The response must set a new sid cookie whose value differs from the fixed one.
	newSID := extractCookieAuth(resp, "sid")
	if newSID == "" {
		t.Fatal("login did not set a new sid cookie")
	}
	if newSID == fixedSID {
		t.Fatalf("sid was NOT rotated on login: still %q (session fixation)", fixedSID)
	}

	// The pre-login (fixed) sid must not resolve an authenticated user.
	if u, err := userRepo.SessionUser(fixedSID); err == nil && u != nil {
		t.Fatalf("fixed pre-login sid still resolves user %q (fixation not prevented)", u.Email)
	}

	// The rotated sid must resolve the logged-in user.
	u, err := userRepo.SessionUser(newSID)
	if err != nil || u == nil {
		t.Fatalf("rotated sid does not resolve a user: %v", err)
	}
	if u.Email != "alice@retrobytes.test" {
		t.Fatalf("rotated sid resolves wrong user: %q", u.Email)
	}
}

// SR-AUTHZ-06: expired sessions do not resolve a user. A session whose
// expires_at is in the past is treated as logged-out.
func TestExpiredSessionRejected(t *testing.T) {
	db, err := repos.OpenDB(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	userRepo := repos.NewUserRepo(db)

	// A valid (future TTL) session resolves the user.
	if err := userRepo.BindSession("sid-live", "u-alice", time.Hour); err != nil {
		t.Fatalf("bind live session: %v", err)
	}
	if u, err := userRepo.SessionUser("sid-live"); err != nil || u == nil {
		t.Fatalf("live session should resolve a user, got err=%v", err)
	}

	// An already-expired session must NOT resolve a user.
	if err := userRepo.BindSession("sid-expired", "u-alice", -time.Hour); err != nil {
		t.Fatalf("bind expired session: %v", err)
	}
	if u, err := userRepo.SessionUser("sid-expired"); err == nil && u != nil {
		t.Fatalf("expired session still resolves user %q", u.Email)
	}
}
