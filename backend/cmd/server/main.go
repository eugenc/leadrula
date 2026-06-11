package main

import (
	"context"
	"encoding/hex"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/echayko/leadrula/backend/db"
	"github.com/echayko/leadrula/backend/internal/accounts"
	"github.com/echayko/leadrula/backend/internal/apikeys"
	"github.com/echayko/leadrula/backend/internal/auth"
	"github.com/echayko/leadrula/backend/internal/billing"
	"github.com/echayko/leadrula/backend/internal/calendar"
	"github.com/echayko/leadrula/backend/internal/collaboration"
	"github.com/echayko/leadrula/backend/internal/config"
	"github.com/echayko/leadrula/backend/internal/contracts"
	"github.com/echayko/leadrula/backend/internal/customfields"
	"github.com/echayko/leadrula/backend/internal/dashboard"
	"github.com/echayko/leadrula/backend/internal/database"
	"github.com/echayko/leadrula/backend/internal/integrations"
	"github.com/echayko/leadrula/backend/internal/intake"
	"github.com/echayko/leadrula/backend/internal/leads"
	"github.com/echayko/leadrula/backend/internal/notifications"
	"github.com/echayko/leadrula/backend/internal/oversight"
	"github.com/echayko/leadrula/backend/internal/partnerships"
	"github.com/echayko/leadrula/backend/internal/pipelines"
	"github.com/echayko/leadrula/backend/internal/routing"
	stripeClient "github.com/echayko/leadrula/backend/internal/stripe"
	"github.com/echayko/leadrula/backend/internal/webhooks"
	"github.com/echayko/leadrula/backend/internal/storage"
	mw "github.com/echayko/leadrula/backend/pkg/middleware"
	"github.com/go-chi/chi/v5"
)

func main() {
	cfg := config.Load()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := database.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("database: %v", err)
	}
	defer pool.Close()

	if err := database.Migrate(ctx, pool, db.Migrations, db.Dir); err != nil {
		log.Fatalf("migrate: %v", err)
	}
	log.Println("migrations applied")

	// ── wiring ───────────────────────────────────────────────────
	tokens := auth.NewTokenManager(cfg.JWTAccessSecret, cfg.JWTRefreshSecret, cfg.AccessTokenTTL, cfg.RefreshTokenTTL)
	email := notifications.NewEmailSender(cfg.MailgunAPIKey, cfg.MailgunDomain, cfg.MailgunFrom, cfg.MailgunAPIBase, cfg.AppBaseURL)
	avatars := storage.NewAvatarStore(cfg.S3Endpoint, cfg.S3Bucket, cfg.S3AccessKey, cfg.S3SecretKey, cfg.S3PublicURL)

	accountsRepo := accounts.NewRepository(pool)
	accountsSvc := accounts.NewService(accountsRepo, tokens, email, avatars)
	accountsH := accounts.NewHandler(accountsSvc)

	notifSvc := notifications.NewService(pool, accountsRepo, email, cfg.AppBaseURL)
	notifH := notifications.NewHandler(notifSvc)

	collabRepo := collaboration.NewRepository(pool)
	collabSvc := collaboration.NewService(collabRepo, notifSvc, tokens)
	collabH := collaboration.NewHandler(collabSvc)

	partnersRepo := partnerships.NewRepository(pool)
	partnersSvc := partnerships.NewService(partnersRepo, accountsRepo, notifSvc)
	partnersH := partnerships.NewHandler(partnersSvc)

	apikeysSvc := apikeys.NewService(pool)
	apikeysH := apikeys.NewHandler(apikeysSvc)

	pipelinesSvc := pipelines.NewService(pool)
	pipelinesH := pipelines.NewHandler(pipelinesSvc)

	cfSvc := customfields.NewService(pool)
	cfH := customfields.NewHandler(cfSvc)

	var sc *stripeClient.Client
	if cfg.StripeSecretKey != "" {
		sc = stripeClient.New(cfg.StripeSecretKey, cfg.StripeConnectClient, cfg.StripePlatformFee)
	} else {
		log.Println("warning: STRIPE_SECRET_KEY not set — Stripe billing endpoints disabled")
	}

	var encKey []byte
	if k, err := hex.DecodeString(cfg.IntegrationEncKey); err == nil && len(k) == 32 {
		encKey = k
	} else if cfg.IntegrationEncKey != "" {
		log.Println("warning: INTEGRATION_ENC_KEY must be 64 hex chars (32 bytes) — publisher stripe keys disabled")
	}

	stripeOAuthRedirect := strings.TrimRight(cfg.IntegrationOAuthRedirectBase, "/") + "/publisher/billing/stripe/oauth/callback"
	billingSvc := billing.NewService(pool, notifSvc, accountsRepo, sc, encKey, stripeOAuthRedirect)
	billingH := billing.NewHandler(billingSvc, cfg.StripeWebhookSecret)

	contractsSvc := contracts.NewService(pool)
	contractsSvc.SetPayoutExecutor(billingSvc)
	contractsSvc.SetNotifier(notifSvc, accountsRepo)
	contractsH := contracts.NewHandler(contractsSvc)

	var integrationsSvc *integrations.Service
	var integrationsEnq leads.IntegrationEnqueuer
	if len(encKey) == 32 {
		oauthCfg := integrations.OAuthConfig{
			RedirectBase:       cfg.IntegrationOAuthRedirectBase,
			PipedriveClientID:  cfg.PipedriveClientID,
			PipedriveSecret:    cfg.PipedriveClientSecret,
			HubSpotClientID:    cfg.HubSpotClientID,
			HubSpotSecret:      cfg.HubSpotClientSecret,
			ZohoClientID:       cfg.ZohoCRMClientID,
			ZohoSecret:         cfg.ZohoCRMClientSecret,
			SalesforceClientID: cfg.SalesforceClientID,
			SalesforceSecret:   cfg.SalesforceClientSecret,
		}
		integrationsSvc = integrations.NewService(pool, encKey, oauthCfg)
		integrationsEnq = integrationsSvc
		go integrationsSvc.RunWorker(ctx)
	} else if cfg.IntegrationEncKey != "" {
		log.Println("warning: INTEGRATION_ENC_KEY must be 64 hex chars (32 bytes) — integrations disabled")
	}

	leadsRepo := leads.NewRepository(pool)
	leadsSvc := leads.NewService(leadsRepo, notifSvc, accountsRepo, pipelinesSvc, integrationsEnq)
	leadsH := leads.NewHandler(leadsSvc)

	dashboardRepo := dashboard.NewRepository(pool)
	dashboardSvc := dashboard.NewService(dashboardRepo)
	dashboardH := dashboard.NewHandler(dashboardSvc)

	routingSvc := routing.NewService(pool)
	routingH := routing.NewHandler(routingSvc)

	calSvc := calendar.NewService(pool)
	calH := calendar.NewHandler(calSvc)

	intakeSvc := intake.NewService(pool, leadsRepo, notifSvc, accountsRepo, integrationsEnq)
	intakeH := intake.NewHandler(intakeSvc)

	webhooksSvc := webhooks.NewService(pool, leadsRepo, leadsSvc, encKey, integrationsSvc)
	webhooksH := webhooks.NewHandler(webhooksSvc)
	leadsSvc.SetWebhookFirer(webhooksSvc)

	oversightH := oversight.NewHandler(accountsRepo, accountsSvc, leadsRepo, pipelinesSvc, billingSvc, calSvc, collabSvc, partnersSvc, partnersSvc)

	// ── router ───────────────────────────────────────────────────
	r := chi.NewRouter()
	r.Use(mw.RequestID, mw.RealIP, mw.Recoverer, mw.Logger, mw.CORS(cfg.CORSOrigins))

	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) { w.Write([]byte("ok")) })

	// public intake API (API-key auth)
	r.Group(func(pub chi.Router) {
		pub.Use(apikeysSvc.RequireAPIKey)
		intakeH.RegisterPublicRoutes(pub)
	})

	// inbound webhooks (per-webhook secret auth)
	webhooksH.RegisterPublicRoutes(r)

	// public auth
	accountsH.RegisterAuthRoutes(r)

	if cfg.StripeWebhookSecret != "" {
		r.Post("/webhooks/stripe", billingH.StripeWebhook)
	}
	billingH.RegisterPublic(r)

	requireAuth := auth.RequireAuth(tokens, collabSvc.ResolvePrincipal)

	// /auth/me + switching (any authenticated account)
	r.Group(func(a chi.Router) {
		a.Use(requireAuth)
		accountsH.RegisterMeRoute(a)
		accountsH.RegisterSwitchRoutes(a)
		collabH.RegisterAuthRoutes(a)
	})

	// platform namespace (operator home; use account switch for publisher/buyer routes)
	r.Route("/platform", func(pl chi.Router) {
		pl.Use(requireAuth, auth.RequireAccountType("platform"))
		accountsH.RegisterPlatformRoutes(pl)
		notifH.RegisterRoutes(pl)
	})

	// publisher namespace
	r.Route("/publisher", func(p chi.Router) {
		p.Use(requireAuth, auth.RequireAccountType("publisher"))
		leadsH.RegisterPublisher(p)
		pipelinesH.RegisterRoutes(p)
		cfH.RegisterRoutes(p)
		contractsH.RegisterPublisher(p)
		billingH.RegisterPublisher(p)
		routingH.RegisterRoutes(p)
		intakeH.RegisterQueueRoutes(p)
		oversightH.RegisterRoutes(p)
		collabH.RegisterPublisherRoutes(p)
		partnersH.RegisterPublisherRoutes(p)
		accountsH.RegisterUserRoutes(p)
		apikeysH.RegisterRoutes(p)
		notifH.RegisterRoutes(p)
		if integrationsSvc != nil {
			integrations.NewHandler(integrationsSvc, webhooksSvc, "publisher", cfg.AppBaseURL, cfg.APIBaseURL).RegisterRoutes(p)
		}
		webhooksH.RegisterRoutes(p)
		dashboardH.RegisterRoutes(p)
	})

	// buyer namespace
	r.Route("/buyer", func(b chi.Router) {
		b.Use(requireAuth, auth.RequireAccountType("buyer"))
		b.Use(auth.LogImpersonationActions(collabSvc.LogImpersonationAction))
		leadsH.RegisterBuyer(b)
		pipelinesH.RegisterRoutes(b)
		cfH.RegisterRoutes(b)
		contractsH.RegisterBuyer(b)
		billingH.RegisterBuyer(b)
		calH.RegisterRoutes(b)
		collabH.RegisterBuyerRoutes(b)
		partnersH.RegisterBuyerRoutes(b)
		oversightH.RegisterBuyerRoutes(b)
		intakeH.RegisterBuyerRoutes(b)
		routingH.RegisterBuyer(b)
		accountsH.RegisterUserRoutes(b)
		apikeysH.RegisterRoutes(b)
		notifH.RegisterRoutes(b)
		if integrationsSvc != nil {
			integrations.NewHandler(integrationsSvc, webhooksSvc, "buyer", cfg.AppBaseURL, cfg.APIBaseURL).RegisterRoutes(b)
		}
		webhooksH.RegisterRoutes(b)
		dashboardH.RegisterRoutes(b)
	})

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           r,
		ReadHeaderTimeout: 10 * time.Second,
	}
	log.Printf("listening on :%s", cfg.Port)
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server: %v", err)
		}
	}()

	<-ctx.Done()
	log.Println("shutting down...")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("shutdown: %v", err)
	}
}
