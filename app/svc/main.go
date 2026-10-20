package main

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"
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

	_ "github.com/jackc/pgx/v5/stdlib"
)

// Advisory lock key held for the duration of the migration run. Rollouts overlap: the
// new container starts while the old one is still serving, and two new containers can
// start at once on a retry. Only one may migrate.
const migrationLockKey int64 = 0x66697474

func main() {
	cfg := loadConfig()
	ctx := context.Background()

	if err := migrateUp(ctx, cfg.databaseMigrationURL); err != nil {
		slog.Error("run migrations failed", "err", err)
		os.Exit(1)
	}

	// Served as the restricted role over the transaction-mode pooler: no prepared
	// statements and no session state. The DSN carries ?default_query_exec_mode=exec,
	// which is what enforces that on the pgx side — do not strip it.
	sqlDB, err := sql.Open("pgx", cfg.databaseURL)
	if err != nil {
		slog.Error("open database failed", "err", err)
		os.Exit(1)
	}
	defer sqlDB.Close()

	if err := sqlDB.PingContext(ctx); err != nil {
		slog.Error("database unreachable", "err", err)
		os.Exit(1)
	}

	queries := db.New(sqlDB)
	stravaClient := strava.NewClient(strava.Config{
		ClientID:           cfg.stravaClientID,
		ClientSecret:       cfg.stravaClientSecret,
		WebhookVerifyToken: cfg.stravaWebhookVerifyToken,
	})

	svcDeps := &service.Deps{
		DB:        sqlDB,
		Queries:   queries,
		Strava:    stravaClient,
		JWTSecret: cfg.jwtSecret,
		JWTTTL:    cfg.jwtTTL,
	}
	handlerDeps := &handler.Deps{
		Queries:               queries,
		Strava:                stravaClient,
		JWTSecret:             cfg.jwtSecret,
		JWTTTL:                cfg.jwtTTL,
		FrontendURL:           cfg.frontendURL,
		PublicBaseURL:         cfg.publicBaseURL,
		AdminStravaAthleteIDs: cfg.adminStravaAthleteIDs,
	}

	authInterceptor := auth.NewInterceptor(cfg.jwtSecret, queries)
	connectOpts := connect.WithInterceptors(authInterceptor)

	api := http.NewServeMux()
	registerService := func(path string, h http.Handler) { api.Handle(path, h) }
	registerService(fitterv1connect.NewUserServiceHandler(service.NewUserService(svcDeps), connectOpts))
	registerService(fitterv1connect.NewCampaignServiceHandler(service.NewCampaignService(svcDeps), connectOpts))
	registerService(fitterv1connect.NewGoalServiceHandler(service.NewGoalService(svcDeps), connectOpts))
	registerService(fitterv1connect.NewFeedServiceHandler(service.NewFeedService(svcDeps), connectOpts))
	registerService(fitterv1connect.NewCatalogServiceHandler(service.NewCatalogService(svcDeps), connectOpts))
	registerService(fitterv1connect.NewAdminServiceHandler(service.NewAdminService(svcDeps), connectOpts))

	api.HandleFunc("/auth/strava/login", handlerDeps.Login)
	api.HandleFunc("/auth/strava/callback", handlerDeps.Callback)
	api.HandleFunc("/webhooks/strava", handlerDeps.Webhook)

	// The CDN routes /api/* to this service and forwards the path unmodified, so the
	// prefix is stripped here rather than baked into every Connect procedure path.
	// Everything behind /api is origin-guarded; the health check is not.
	var apiHandler http.Handler = httpmw.CORS(cfg.corsOrigin, api)
	if cfg.originSharedSecret != "" {
		apiHandler = httpmw.OriginAuth(cfg.originSharedSecret, apiHandler)
	}

	root := http.NewServeMux()
	root.HandleFunc("/healthz", handler.Health)
	root.Handle("/api/", http.StripPrefix("/api", apiHandler))

	// Local development talks to the service directly, with no CDN in front to add the
	// /api prefix, so the same routes stay mounted at the root there.
	if cfg.originSharedSecret == "" {
		root.Handle("/", apiHandler)
	}

	addr := ":" + cfg.port
	slog.Info("fitter svc listening", "addr", addr, "origin_guard", cfg.originSharedSecret != "")
	if err := http.ListenAndServe(addr, root); err != nil {
		slog.Error("server exited", "err", err)
		os.Exit(1)
	}
}

// migrateUp applies pending migrations as the migration user and returns an error if any
// step fails, so the container exits non-zero and the rollout keeps the previous
// deployment serving.
//
// The pool is capped at one connection so the advisory lock and the migrations share a
// single backend session. Point DATABASE_MIGRATION_URL at a session-mode endpoint where
// one is available: a transaction-mode pooler may hand the session to another client
// between statements, which would silently drop a session-scoped lock.
func migrateUp(ctx context.Context, dsn string) error {
	mdb, err := sql.Open("pgx", dsn)
	if err != nil {
		return fmt.Errorf("open migration connection: %w", err)
	}
	defer mdb.Close()
	mdb.SetMaxOpenConns(1)

	if _, err := mdb.ExecContext(ctx, "SELECT pg_advisory_lock($1)", migrationLockKey); err != nil {
		return fmt.Errorf("acquire migration lock: %w", err)
	}
	defer func() {
		if _, err := mdb.ExecContext(ctx, "SELECT pg_advisory_unlock($1)", migrationLockKey); err != nil {
			slog.Warn("release migration lock failed", "err", err)
		}
	}()

	goose.SetBaseFS(dbfs.FS)
	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("set goose dialect: %w", err)
	}
	if err := goose.UpContext(ctx, mdb, "migrations"); err != nil {
		return fmt.Errorf("apply migrations: %w", err)
	}
	return nil
}

type config struct {
	port                     string
	databaseURL              string
	databaseMigrationURL     string
	jwtSecret                []byte
	jwtTTL                   time.Duration
	stravaClientID           string
	stravaClientSecret       string
	stravaWebhookVerifyToken string
	frontendURL              string
	publicBaseURL            string
	corsOrigin               string
	originSharedSecret       string
	adminStravaAthleteIDs    []int64
}

func loadConfig() config {
	frontendURL := mustEnv("FRONTEND_URL")

	// APP_ENV is set only by the deployment pipeline. Where it is set the service is
	// reachable at a public origin URL, and running without the guard would expose the
	// API directly, so its absence is fatal rather than a warning.
	originSharedSecret := os.Getenv("ORIGIN_SHARED_SECRET")
	if os.Getenv("APP_ENV") != "" && originSharedSecret == "" {
		slog.Error("ORIGIN_SHARED_SECRET is required when APP_ENV is set")
		os.Exit(1)
	}

	return config{
		port:                     envOr("PORT", "8081"),
		databaseURL:              mustEnv("DATABASE_URL"),
		databaseMigrationURL:     envOr("DATABASE_MIGRATION_URL", os.Getenv("DATABASE_URL")),
		jwtSecret:                []byte(mustEnv("JWT_SECRET")),
		jwtTTL:                   30 * 24 * time.Hour,
		stravaClientID:           mustEnv("STRAVA_CLIENT_ID"),
		stravaClientSecret:       mustEnv("STRAVA_CLIENT_SECRET"),
		stravaWebhookVerifyToken: mustEnv("STRAVA_WEBHOOK_VERIFY_TOKEN"),
		frontendURL:              frontendURL,
		publicBaseURL:            mustEnv("PUBLIC_BASE_URL"),
		corsOrigin:               envOr("CORS_ORIGIN", frontendURL),
		originSharedSecret:       originSharedSecret,
		adminStravaAthleteIDs:    parseInt64CSV("ADMIN_STRAVA_ATHLETE_IDS", os.Getenv("ADMIN_STRAVA_ATHLETE_IDS")),
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

// parseInt64CSV parses a comma-separated list of int64s, e.g.
// ADMIN_STRAVA_ATHLETE_IDS="123,456". Blank entries (from a trailing comma
// or an unset/empty env var) are skipped rather than erroring. key is only
// used to make a malformed-value error message actionable.
func parseInt64CSV(key, raw string) []int64 {
	var ids []int64
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		id, err := strconv.ParseInt(part, 10, 64)
		if err != nil {
			slog.Error("invalid env var: not a comma-separated list of integers", "key", key, "value", raw)
			os.Exit(1)
		}
		ids = append(ids, id)
	}
	return ids
}
