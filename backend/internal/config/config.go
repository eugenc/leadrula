package config

import (
	"log"
	"os"
	"strconv"
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
	APIBaseURL       string
	CORSOrigins      []string

	MailgunAPIKey  string
	MailgunDomain  string
	MailgunFrom    string
	MailgunAPIBase string

	S3Endpoint  string
	S3Bucket    string
	S3AccessKey string
	S3SecretKey string
	S3PublicURL string

	StripeSecretKey       string
	StripeWebhookSecret string
	StripePlatformFee   float64
	StripeConnectClient string

	IntegrationEncKey              string
	IntegrationOAuthRedirectBase   string
	PipedriveClientID              string
	PipedriveClientSecret          string
	HubSpotClientID                string
	HubSpotClientSecret            string
	ZohoCRMClientID                string
	ZohoCRMClientSecret            string
	SalesforceClientID             string
	SalesforceClientSecret         string
}

// Load reads configuration from a .env file (if present) and the environment.
func Load() *Config {
	_ = godotenv.Load()

	oauthRedirectBase := getenv("INTEGRATION_OAUTH_REDIRECT_BASE", "http://localhost:8080")

	cfg := &Config{
		Port:             getenv("PORT", "8080"),
		DatabaseURL:      getenv("DATABASE_URL", "postgres://crm:crm@localhost:5432/crm?sslmode=disable"),
		JWTAccessSecret:  getenv("JWT_ACCESS_SECRET", "dev-access-secret"),
		JWTRefreshSecret: getenv("JWT_REFRESH_SECRET", "dev-refresh-secret"),
		AccessTokenTTL:   getdur("ACCESS_TOKEN_TTL", 15*time.Minute),
		RefreshTokenTTL:  getdur("REFRESH_TOKEN_TTL", 720*time.Hour),
		AppBaseURL:       getenv("APP_BASE_URL", "http://localhost:5173"),
		APIBaseURL:       firstNonEmpty(os.Getenv("API_BASE_URL"), oauthRedirectBase, "http://localhost:8080"),
		CORSOrigins:      splitCSV(getenv("CORS_ORIGINS", "http://localhost:5173,http://localhost:5174")),
		MailgunAPIKey:  os.Getenv("MAILGUN_API_KEY"),
		MailgunDomain:  os.Getenv("MAILGUN_DOMAIN"),
		MailgunFrom:    getenv("MAILGUN_FROM", "no-reply@leadrula.local"),
		MailgunAPIBase: getenv("MAILGUN_API_BASE", "https://api.mailgun.net"),
		S3Endpoint:     os.Getenv("S3_ENDPOINT"),
		S3Bucket:       os.Getenv("S3_BUCKET"),
		S3AccessKey:    os.Getenv("S3_ACCESS_KEY"),
		S3SecretKey:    os.Getenv("S3_SECRET_KEY"),
		S3PublicURL:    os.Getenv("S3_PUBLIC_URL"),
		StripeSecretKey:     os.Getenv("STRIPE_SECRET_KEY"),
		StripeWebhookSecret: os.Getenv("STRIPE_WEBHOOK_SECRET"),
		StripePlatformFee:   getfloat("STRIPE_PLATFORM_FEE", 0.10),
		StripeConnectClient: os.Getenv("STRIPE_CONNECT_CLIENT_ID"),

		IntegrationEncKey:            os.Getenv("INTEGRATION_ENC_KEY"),
		IntegrationOAuthRedirectBase: oauthRedirectBase,
		PipedriveClientID:            os.Getenv("PIPEDRIVE_CLIENT_ID"),
		PipedriveClientSecret:        os.Getenv("PIPEDRIVE_CLIENT_SECRET"),
		HubSpotClientID:              os.Getenv("HUBSPOT_CLIENT_ID"),
		HubSpotClientSecret:          os.Getenv("HUBSPOT_CLIENT_SECRET"),
		ZohoCRMClientID:              os.Getenv("ZOHO_CRM_CLIENT_ID"),
		ZohoCRMClientSecret:          os.Getenv("ZOHO_CRM_CLIENT_SECRET"),
		SalesforceClientID:           os.Getenv("SALESFORCE_CLIENT_ID"),
		SalesforceClientSecret:       os.Getenv("SALESFORCE_CLIENT_SECRET"),
	}
	return cfg
}

func getfloat(key string, fallback float64) float64 {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		log.Printf("config: invalid float %q for %s, using default", v, key)
		return fallback
	}
	return f
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

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}
