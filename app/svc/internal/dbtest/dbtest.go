// Package dbtest provides a fresh, fully-migrated Postgres schema for use in
// tests. Tests skip unless TEST_DATABASE_URL points at a Postgres database the
// test user can create schemas in, e.g. the docker-compose one:
//
//	TEST_DATABASE_URL=postgres://fitter:fitter@localhost:5432/fitter?sslmode=disable
package dbtest

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"net/url"
	"os"
	"testing"

	dbfs "github.com/fitter/svc/db"
	"github.com/fitter/svc/internal/db"
	"github.com/pressly/goose/v3"

	_ "github.com/jackc/pgx/v5/stdlib"
)

// Same key the service holds while migrating. Test packages run in parallel
// processes, and 00002 creates a cluster-wide role, so migrations serialize.
const migrationLockKey int64 = 0x66697474

// New creates a uniquely named schema, runs every migration into it, and
// returns a *sql.DB whose connections use that schema as their search_path,
// plus a *db.Queries wrapping it. The schema is dropped via t.Cleanup.
func New(t *testing.T) (*sql.DB, *db.Queries) {
	t.Helper()
	ctx := context.Background()

	base := os.Getenv("TEST_DATABASE_URL")
	if base == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping Postgres-backed test")
	}

	admin, err := sql.Open("pgx", base)
	if err != nil {
		t.Fatalf("open admin connection: %v", err)
	}
	admin.SetMaxOpenConns(1)
	t.Cleanup(func() { admin.Close() })

	suffix := make([]byte, 6)
	if _, err := rand.Read(suffix); err != nil {
		t.Fatalf("random schema name: %v", err)
	}
	schema := "dbtest_" + hex.EncodeToString(suffix)
	if _, err := admin.ExecContext(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatalf("create schema: %v", err)
	}
	t.Cleanup(func() {
		if _, err := admin.ExecContext(ctx, "DROP SCHEMA "+schema+" CASCADE"); err != nil {
			t.Errorf("drop schema %s: %v", schema, err)
		}
	})

	dsn, err := withSearchPath(base, schema)
	if err != nil {
		t.Fatalf("build schema DSN: %v", err)
	}
	sqlDB, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("open schema connection: %v", err)
	}
	// Registered after the schema drop, so it runs first: close before dropping.
	t.Cleanup(func() { sqlDB.Close() })

	if _, err := admin.ExecContext(ctx, "SELECT pg_advisory_lock($1)", migrationLockKey); err != nil {
		t.Fatalf("acquire migration lock: %v", err)
	}
	goose.SetLogger(goose.NopLogger())
	goose.SetBaseFS(dbfs.FS)
	if err := goose.SetDialect("postgres"); err != nil {
		t.Fatalf("goose set dialect: %v", err)
	}
	migrateErr := goose.UpContext(ctx, sqlDB, "migrations")
	if _, err := admin.ExecContext(ctx, "SELECT pg_advisory_unlock($1)", migrationLockKey); err != nil {
		t.Fatalf("release migration lock: %v", err)
	}
	if migrateErr != nil {
		t.Fatalf("run migrations: %v", migrateErr)
	}

	return sqlDB, db.New(sqlDB)
}

// withSearchPath adds search_path to the DSN; pgx sends unknown query
// parameters to the server as runtime parameters, so every pooled
// connection starts in the test's schema.
func withSearchPath(dsn, schema string) (string, error) {
	u, err := url.Parse(dsn)
	if err != nil {
		return "", err
	}
	q := u.Query()
	q.Set("search_path", schema)
	u.RawQuery = q.Encode()
	return u.String(), nil
}
