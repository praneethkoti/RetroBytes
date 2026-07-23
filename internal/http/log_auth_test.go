package handlers_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/csrf"
	"github.com/gofiber/fiber/v2/middleware/limiter"
	html "github.com/gofiber/template/html/v2"
	"log"
	"sync"

	"retrobytes/internal/http/handlers"
	"retrobytes/internal/repos"
	"retrobytes/internal/services"
)

type logEntry struct {
	Level  string                 `json:"level"`
	Action string                 `json:"action"`
	Fields map[string]interface{} `json:"fields"`
}

// capture logs by temporarily replacing the standard logger output
func captureLogs(t *testing.T, fn func()) []logEntry {
	t.Helper()
	var buf bytes.Buffer
	var mu sync.Mutex
	oldW := log.Writer()
	oldFlags := log.Flags()
	log.SetOutput(&lockedWriter{w: &buf, mu: &mu})
	log.SetFlags(0) // remove timestamps to make JSON parseable
	defer func() {
		log.SetOutput(oldW)
		log.SetFlags(oldFlags)
	}()

	fn()

	var entries []logEntry
	for _, line := range strings.Split(strings.TrimSpace(buf.String()), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var e logEntry
		if err := json.Unmarshal([]byte(line), &e); err == nil {
			entries = append(entries, e)
		}
	}
	return entries
}

type lockedWriter struct {
	w  *bytes.Buffer
	mu *sync.Mutex
}

func (lw *lockedWriter) Write(p []byte) (int, error) {
	lw.mu.Lock()
	defer lw.mu.Unlock()
	return lw.w.Write(p)
}

func extractCookieLog(resp *http.Response, name string) string {
	for _, c := range resp.Cookies() {
		if c.Name == name {
			return c.Value
		}
	}
	return ""
}

// SR-LOG-01: auth logging on success/fail
func TestAuthLogging(t *testing.T) {
	db, err := repos.OpenDB(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	userRepo := repos.NewUserRepo(db)
	authSvc := &services.AuthService{Users: userRepo}
	authH := &handlers.AuthHandler{Auth: authSvc}
	engine := html.New("../../web/templates", ".html")

	app := fiber.New(fiber.Config{Views: engine})
	app.Use(csrf.New(csrf.Config{KeyLookup: "form:csrf", CookieName: "csrf_", CookieSameSite: "Lax"}))
	app.Post("/login", limiter.New(limiter.Config{Max: 100, Expiration: time.Minute}), authH.Login)
	app.Get("/login", authH.LoginForm)

	// fetch csrf token
	respLogin, _ := app.Test(httptest.NewRequest("GET", "/login", nil))
	csrfTok := extractCookieLog(respLogin, "csrf_")
	if csrfTok == "" {
		t.Fatal("csrf token missing")
	}

	run := func(email, pass string) []logEntry {
		return captureLogs(t, func() {
			form := strings.NewReader("csrf=" + csrfTok + "&email=" + email + "&password=" + pass)
			req := httptest.NewRequest("POST", "/login", form)
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			req.AddCookie(&http.Cookie{Name: "csrf_", Value: csrfTok})
			_, _ = app.Test(req)
		})
	}

	failLogs := run("alice@retrobytes.test", "badpass!")
	if len(failLogs) == 0 {
		t.Fatal("expected auth logs on failure")
	}
	foundFail := false
	for _, e := range failLogs {
		if e.Action == "auth.login.fail" {
			foundFail = true
			if _, ok := e.Fields["email"]; !ok {
				t.Fatalf("auth.login.fail missing email field")
			}
		}
	}
	if !foundFail {
		t.Fatalf("auth.login.fail log not found")
	}

	successLogs := run("alice@retrobytes.test", "Passw0rd!")
	foundSuccess := false
	for _, e := range successLogs {
		if e.Action == "auth.login.success" {
			foundSuccess = true
			if _, ok := e.Fields["email"]; !ok {
				t.Fatalf("auth.login.success missing email field")
			}
		}
	}
	if !foundSuccess {
		t.Fatalf("auth.login.success log not found")
	}
}
