package main

import (
	"database/sql"
	"log/slog"
	"net/http"
	"os"
	"time"

	"connectrpc.com/connect"
	dbfs "github.com/fitter/svc/db"
	"github.com/fitter/svc/gen/fitter/v1/fitterv1connect"
	"github.com/fitter/svc/internal/auth"
	"github.com/fitter/svc/internal/db"
	"github.com/fitter/svc/internal/handler"
	"github.com/fitter/svc/internal/httpmw"
	"github.com/fitter/svc/internal/service"
	"github.com/fitter/svc/internal/strava"
	"github.com/pressly/goose/v3"

	_ "modernc.org/sqlite"
)

func main() {
	cfg := loadConfig()

	sqlDB, err := sql.Open("sqlite", cfg.dbPath)
	if err != nil {
		slog.Error("open database failed", "err", err)
		os.Exit(1)
	}
	defer sqlDB.Close()

	goose.SetBaseFS(dbfs.FS)
	if err := goose.SetDialect("sqlite3"); err != nil {
		slog.Error("goose set dialect failed", "err", err)
		os.Exit(1)
	}
	if err := goose.Up(sqlDB, "migrations"); err != nil {
		slog.Error("run migrations failed", "err", err)
		os.Exit(1)
	}

	queries := db.New(sqlDB)
	stravaClient := strava.NewClient(strava.Config{
		ClientID:           cfg.stravaClientID,
		ClientSecret:       cfg.stravaClientSecret,
		WebhookVerifyToken: cfg.stravaWebhookVerifyToken,
	})

	svcDeps := &service.Deps{
		Queries:   queries,
		Strava:    stravaClient,
		JWTSecret: cfg.jwtSecret,
		JWTTTL:    cfg.jwtTTL,
	}
	handlerDeps := &handler.Deps{
		Queries:       queries,
		Strava:        stravaClient,
		JWTSecret:     cfg.jwtSecret,
		JWTTTL:        cfg.jwtTTL,
		FrontendURL:   cfg.frontendURL,
		PublicBaseURL: cfg.publicBaseURL,
	}

	authInterceptor := auth.NewInterceptor(cfg.jwtSecret)
	connectOpts := connect.WithInterceptors(authInterceptor)

	mux := http.NewServeMux()
	registerService := func(path string, h http.Handler) { mux.Handle(path, h) }
	registerService(fitterv1connect.NewUserServiceHandler(service.NewUserService(svcDeps), connectOpts))
	registerService(fitterv1connect.NewCampaignServiceHandler(service.NewCampaignService(svcDeps), connectOpts))
	registerService(fitterv1connect.NewGoalServiceHandler(service.NewGoalService(svcDeps), connectOpts))
	registerService(fitterv1connect.NewFeedServiceHandler(service.NewFeedService(svcDeps), connectOpts))

	mux.HandleFunc("/auth/strava/login", handlerDeps.Login)
	mux.HandleFunc("/auth/strava/callback", handlerDeps.Callback)
	mux.HandleFunc("/webhooks/strava", handlerDeps.Webhook)
	mux.HandleFunc("/healthz", handler.Health)

	addr := ":" + cfg.port
	slog.Info("fitter svc listening", "addr", addr)
	if err := http.ListenAndServe(addr, httpmw.CORS(cfg.corsOrigin, mux)); err != nil {
		slog.Error("server exited", "err", err)
		os.Exit(1)
	}
}

type config struct {
	port                     string
	dbPath                   string
	jwtSecret                []byte
	jwtTTL                   time.Duration
	stravaClientID           string
	stravaClientSecret       string
	stravaWebhookVerifyToken string
	frontendURL              string
	publicBaseURL            string
	corsOrigin               string
}

func loadConfig() config {
	frontendURL := mustEnv("FRONTEND_URL")
	return config{
		port:                     envOr("PORT", "8081"),
		dbPath:                   envOr("DB_PATH", "fitter.db"),
		jwtSecret:                []byte(mustEnv("JWT_SECRET")),
		jwtTTL:                   30 * 24 * time.Hour,
		stravaClientID:           mustEnv("STRAVA_CLIENT_ID"),
		stravaClientSecret:       mustEnv("STRAVA_CLIENT_SECRET"),
		stravaWebhookVerifyToken: mustEnv("STRAVA_WEBHOOK_VERIFY_TOKEN"),
		frontendURL:              frontendURL,
		publicBaseURL:            mustEnv("PUBLIC_BASE_URL"),
		corsOrigin:               envOr("CORS_ORIGIN", frontendURL),
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func mustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		slog.Error("missing required env var", "key", key)
		os.Exit(1)
	}
	return v
}
