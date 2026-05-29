package main

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/echayko/leadrula/backend/db"
	"github.com/echayko/leadrula/backend/internal/accounts"
	"github.com/echayko/leadrula/backend/internal/apikeys"
	"github.com/echayko/leadrula/backend/internal/auth"
	"github.com/echayko/leadrula/backend/internal/billing"
	"github.com/echayko/leadrula/backend/internal/calendar"
	"github.com/echayko/leadrula/backend/internal/config"
	"github.com/echayko/leadrula/backend/internal/contracts"
	"github.com/echayko/leadrula/backend/internal/customfields"
	"github.com/echayko/leadrula/backend/internal/database"
	"github.com/echayko/leadrula/backend/internal/intake"
	"github.com/echayko/leadrula/backend/internal/leads"
	"github.com/echayko/leadrula/backend/internal/notifications"
	"github.com/echayko/leadrula/backend/internal/oversight"
	"github.com/echayko/leadrula/backend/internal/pipelines"
	"github.com/echayko/leadrula/backend/internal/routing"
	mw "github.com/echayko/leadrula/backend/pkg/middleware"
	"github.com/go-chi/chi/v5"
)

func main() {
	cfg := config.Load()
	ctx := context.Background()

	pool, err := database.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("database: %v", err)
	}
	defer pool.Close()

	if err := database.Migrate(ctx, pool, db.Migrations, db.Dir); err != nil {
		log.Fatalf("migrate: %v", err)
	}
	log.Println("migrations applied")

	// find the single publisher (may not exist before bootstrap)
	var publisherID int64
	_ = pool.QueryRow(ctx, `SELECT id FROM accounts WHERE type='publisher' LIMIT 1`).Scan(&publisherID)

	// ── wiring ───────────────────────────────────────────────────
	tokens := auth.NewTokenManager(cfg.JWTAccessSecret, cfg.JWTRefreshSecret, cfg.AccessTokenTTL, cfg.RefreshTokenTTL)
	email := notifications.NewEmailSender(cfg.SMTPHost, cfg.SMTPPort, cfg.SMTPUser, cfg.SMTPPass, cfg.SMTPFrom, cfg.AppBaseURL)

	accountsRepo := accounts.NewRepository(pool)
	accountsSvc := accounts.NewService(accountsRepo, tokens, email)
	accountsH := accounts.NewHandler(accountsSvc)

	notifSvc := notifications.NewService(pool)
	notifH := notifications.NewHandler(notifSvc)

	apikeysSvc := apikeys.NewService(pool)
	apikeysH := apikeys.NewHandler(apikeysSvc)

	pipelinesSvc := pipelines.NewService(pool)
	pipelinesH := pipelines.NewHandler(pipelinesSvc)

	cfSvc := customfields.NewService(pool)
	cfH := customfields.NewHandler(cfSvc)

	contractsSvc := contracts.NewService(pool)
	contractsH := contracts.NewHandler(contractsSvc)

	billingSvc := billing.NewService(pool, notifSvc, accountsRepo)
	billingH := billing.NewHandler(billingSvc)

	leadsRepo := leads.NewRepository(pool)
	leadsSvc := leads.NewService(leadsRepo, notifSvc, accountsRepo)
	leadsH := leads.NewHandler(leadsSvc)

	routingSvc := routing.NewService(pool)
	routingH := routing.NewHandler(routingSvc)

	calSvc := calendar.NewService(pool)
	calH := calendar.NewHandler(calSvc)

	intakeSvc := intake.NewService(pool, leadsRepo, notifSvc, accountsRepo)
	intakeH := intake.NewHandler(intakeSvc, publisherID)

	oversightH := oversight.NewHandler(accountsRepo, leadsRepo, pipelinesSvc, billingSvc, calSvc)

	// ── router ───────────────────────────────────────────────────
	r := chi.NewRouter()
	r.Use(mw.RequestID, mw.RealIP, mw.Recoverer, mw.Logger, mw.CORS(cfg.CORSOrigins))

	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) { w.Write([]byte("ok")) })

	// public intake API (API-key auth)
	r.Group(func(pub chi.Router) {
		pub.Use(apikeysSvc.RequireAPIKey)
		intakeH.RegisterPublicRoutes(pub)
	})

	// public auth
	accountsH.RegisterAuthRoutes(r)

	requireAuth := auth.RequireAuth(tokens, accountsRepo.LoadPrincipal)

	// /auth/me (any authenticated account)
	r.Group(func(a chi.Router) {
		a.Use(requireAuth)
		accountsH.RegisterMeRoute(a)
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
		accountsH.RegisterUserRoutes(p)
		apikeysH.RegisterRoutes(p)
		notifH.RegisterRoutes(p)
	})

	// buyer namespace
	r.Route("/buyer", func(b chi.Router) {
		b.Use(requireAuth, auth.RequireAccountType("buyer"))
		leadsH.RegisterBuyer(b)
		pipelinesH.RegisterRoutes(b)
		cfH.RegisterRoutes(b)
		contractsH.RegisterBuyer(b)
		billingH.RegisterBuyer(b)
		calH.RegisterRoutes(b)
		accountsH.RegisterUserRoutes(b)
		apikeysH.RegisterRoutes(b)
		notifH.RegisterRoutes(b)
	})

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           r,
		ReadHeaderTimeout: 10 * time.Second,
	}
	log.Printf("listening on :%s", cfg.Port)
	if err := srv.ListenAndServe(); err != nil {
		log.Fatalf("server: %v", err)
	}
}
