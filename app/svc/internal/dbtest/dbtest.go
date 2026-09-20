// Package dbtest provides an in-memory, fully-migrated SQLite database for
// use in tests.
package dbtest

import (
	"database/sql"
	"testing"

	dbfs "github.com/fitter/svc/db"
	"github.com/fitter/svc/internal/db"
	"github.com/pressly/goose/v3"

	_ "modernc.org/sqlite"
)

// New opens an in-memory SQLite database, runs every migration against it,
// and returns both the raw *sql.DB and a *db.Queries wrapping it. The
// database is closed automatically via t.Cleanup.
func New(t *testing.T) (*sql.DB, *db.Queries) {
	t.Helper()

	sqlDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open in-memory sqlite: %v", err)
	}

	// A plain ":memory:" SQLite database is per-connection: without pinning
	// the pool to a single connection, goose migrates one connection while
	// later queries from the pool silently open a second, empty, unmigrated
	// database. This must stay 1.
	sqlDB.SetMaxOpenConns(1)

	if _, err := sqlDB.Exec("PRAGMA foreign_keys = ON"); err != nil {
		sqlDB.Close()
		t.Fatalf("enable foreign_keys pragma: %v", err)
	}

	goose.SetLogger(goose.NopLogger())
	goose.SetBaseFS(dbfs.FS)
	if err := goose.SetDialect("sqlite3"); err != nil {
		sqlDB.Close()
		t.Fatalf("goose set dialect: %v", err)
	}
	if err := goose.Up(sqlDB, "migrations"); err != nil {
		sqlDB.Close()
		t.Fatalf("run migrations: %v", err)
	}

	t.Cleanup(func() { sqlDB.Close() })

	return sqlDB, db.New(sqlDB)
}
