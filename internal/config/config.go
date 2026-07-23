package config

import (
	"log"
	"os"
)

type Config struct {
	Port     string
	DBDSN    string
	MediaDir string
	LogFile  string
}

func Load() Config {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
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

	cfg := Config{Port: port, DBDSN: dsn, MediaDir: media, LogFile: logFile}
	log.Printf("[config] PORT=%s DB_DSN=%s MEDIA_DIR=%s LOG_FILE=%s", cfg.Port, cfg.DBDSN, cfg.MediaDir, cfg.LogFile)
	return cfg
}
