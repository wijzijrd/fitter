package dbtest_test

import (
	"testing"

	"github.com/fitter/svc/internal/dbtest"
)

// TestNew is the first test in this repo. It exists purely to prove the
// fixture itself works before anything is built on top of it: the database
// opens, migrations apply, and the users table is queryable.
func TestNew(t *testing.T) {
	sqlDB, queries := dbtest.New(t)
	if sqlDB == nil {
		t.Fatal("New returned nil *sql.DB")
	}
	if queries == nil {
		t.Fatal("New returned nil *db.Queries")
	}

	var count int
	if err := sqlDB.QueryRow("SELECT count(*) FROM users").Scan(&count); err != nil {
		t.Fatalf("users table not queryable: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected empty users table, got count=%d", count)
	}
}
