package config

import (
	"log"
	"os"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

// Config holds all runtime configuration loaded from the environment.
type Config struct {
	Port             string
	DatabaseURL      string
	JWTAccessSecret  string
	JWTRefreshSecret string
	AccessTokenTTL   time.Duration
	RefreshTokenTTL  time.Duration
	AppBaseURL       string
	CORSOrigins      []string

	SMTPHost string
	SMTPPort string
	SMTPUser string
	SMTPPass string
	SMTPFrom string
}

// Load reads configuration from a .env file (if present) and the environment.
func Load() *Config {
	_ = godotenv.Load()

	cfg := &Config{
		Port:             getenv("PORT", "8080"),
		DatabaseURL:      getenv("DATABASE_URL", "postgres://crm:crm@localhost:5432/crm?sslmode=disable"),
		JWTAccessSecret:  getenv("JWT_ACCESS_SECRET", "dev-access-secret"),
		JWTRefreshSecret: getenv("JWT_REFRESH_SECRET", "dev-refresh-secret"),
		AccessTokenTTL:   getdur("ACCESS_TOKEN_TTL", 15*time.Minute),
		RefreshTokenTTL:  getdur("REFRESH_TOKEN_TTL", 720*time.Hour),
		AppBaseURL:       getenv("APP_BASE_URL", "http://localhost:5173"),
		CORSOrigins:      splitCSV(getenv("CORS_ORIGINS", "http://localhost:5173")),
		SMTPHost:         os.Getenv("SMTP_HOST"),
		SMTPPort:         getenv("SMTP_PORT", "587"),
		SMTPUser:         os.Getenv("SMTP_USER"),
		SMTPPass:         os.Getenv("SMTP_PASS"),
		SMTPFrom:         getenv("SMTP_FROM", "no-reply@leadrula.local"),
	}
	return cfg
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getdur(key string, fallback time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		log.Printf("config: invalid duration %q for %s, using default", v, key)
		return fallback
	}
	return d
}

func splitCSV(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}
