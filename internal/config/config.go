package config

import (
	"log"
	"os"
	"strconv"
	"time"
)

type Config struct {
	Port     string
	DBDSN    string
	MediaDir string
	LogFile  string

	// CookieSecure controls the Secure flag on session/CSRF cookies.
	// Default false so local development over plain http://localhost works.
	// Set COOKIE_SECURE=true behind HTTPS in production.
	CookieSecure bool

	// SessionTTL is how long an authenticated session remains valid after
	// login before it is treated as expired. Configurable via SESSION_TTL_HOURS.
	SessionTTL time.Duration
}

func Load() Config {
	// Listen port comes from PORT (hosting platforms inject this). Default 8081
	// so local behavior is unchanged when PORT is unset.
	port := os.Getenv("PORT")
	if port == "" {
		port = "8081"
	}
	dsn := os.Getenv("DB_DSN")
	if dsn == "" {
		dsn = "retrobytes.db"
	} // sqlite file in project root
	media := os.Getenv("MEDIA_DIR")
	if media == "" {
		// Default to the actual bundled media directory under web/
		// Previous default "./media" caused 404s for /media/* requests.
		media = "./web/media"
	}
	logFile := os.Getenv("LOG_FILE")
	if logFile == "" {
		logFile = "./retrobytes.log" // default log sink in project root
	}

	// Secure cookies are opt-in so local http development is not broken.
	cookieSecure := false
	if v, err := strconv.ParseBool(os.Getenv("COOKIE_SECURE")); err == nil {
		cookieSecure = v
	}

	// Session lifetime (hours). Default 24h; must be positive.
	sessionTTL := 24 * time.Hour
	if h, err := strconv.Atoi(os.Getenv("SESSION_TTL_HOURS")); err == nil && h > 0 {
		sessionTTL = time.Duration(h) * time.Hour
	}

	cfg := Config{
		Port:         port,
		DBDSN:        dsn,
		MediaDir:     media,
		LogFile:      logFile,
		CookieSecure: cookieSecure,
		SessionTTL:   sessionTTL,
	}
	log.Printf("[config] PORT=%s DB_DSN=%s MEDIA_DIR=%s LOG_FILE=%s COOKIE_SECURE=%t SESSION_TTL=%s",
		cfg.Port, cfg.DBDSN, cfg.MediaDir, cfg.LogFile, cfg.CookieSecure, cfg.SessionTTL)
	return cfg
}
