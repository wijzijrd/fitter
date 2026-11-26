package betacopy

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/lib/pq"
)

// CleanupResult lists the source users that cleanup would delete, and the counts.
type CleanupResult struct {
	Names  []string // display names, for the confirmation prompt
	Report Report
}

// Cleanup deletes from src every user that also exists on dst (same id: the copied beta
// users), with their own data. Users who were never copied are left alone. Run it only
// after the owner has signed off on the destination. dryRun rolls back.
func Cleanup(ctx context.Context, src, dst *sql.DB, dryRun bool) (CleanupResult, error) {
	var dstIDs []string
	rows, err := dst.QueryContext(ctx, `SELECT id FROM users`)
	if err != nil {
		return CleanupResult{}, err
	}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return CleanupResult{}, err
		}
		dstIDs = append(dstIDs, id)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return CleanupResult{}, err
	}

	tx, err := src.BeginTx(ctx, nil)
	if err != nil {
		return CleanupResult{}, err
	}
	defer tx.Rollback()

	var res CleanupResult
	res.Report = Report{}
	users, err := readRows(ctx, src, `SELECT id, name FROM users WHERE id = ANY($1) ORDER BY created_at`, pq.StringArray(dstIDs))
	if err != nil {
		return CleanupResult{}, err
	}
	if len(users) == 0 {
		return res, nil
	}
	ids := make([]string, 0, len(users))
	for _, u := range users {
		ids = append(ids, u["id"].(string))
		res.Names = append(res.Names, u["name"].(string))
	}
	var activities int64
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM activities WHERE user_id = ANY($1)`, pq.StringArray(ids)).Scan(&activities); err != nil {
		return CleanupResult{}, err
	}
	res.Report["users"] = int64(len(ids))
	res.Report["activities"] = activities // Strava and manual; cascaded with their user

	statements := []string{
		// Audit and allowlist rows point at users without cascading; they go with the user.
		`DELETE FROM admin_actions WHERE actor_id = ANY($1)`,
		`DELETE FROM registration_allowlist WHERE added_by = ANY($1)`,
		`UPDATE users SET banned_by = NULL WHERE banned_by = ANY($1)`,
		// Campaigns are the copied users' own data (the owner's on staging); members and goals cascade.
		`DELETE FROM campaigns WHERE created_by = ANY($1)`,
		// Custom sports and metrics made by a copied user are their data too. Their sport keys
		// are plain text elsewhere, so removing them leaves nothing dangling.
		`DELETE FROM custom_metrics WHERE created_by = ANY($1)`,
		`DELETE FROM custom_sports WHERE created_by = ANY($1)`,
	}
	for _, q := range statements {
		if _, err := tx.ExecContext(ctx, q, pq.StringArray(ids)); err != nil {
			return CleanupResult{}, fmt.Errorf("cleanup: %w", err)
		}
	}
	// Cascades remove activities, goals, campaign memberships and the rest. Any other reference
	// that blocks the delete makes the whole cleanup fail and roll back, naming the constraint.
	deleted, err := tx.ExecContext(ctx, `DELETE FROM users WHERE id = ANY($1)`, pq.StringArray(ids))
	if err != nil {
		return CleanupResult{}, fmt.Errorf("cleanup users: %w", err)
	}
	if n, _ := deleted.RowsAffected(); n != int64(len(ids)) {
		return CleanupResult{}, fmt.Errorf("cleanup deleted %d of %d users", n, len(ids))
	}
	if dryRun {
		return res, nil
	}
	if err := tx.Commit(); err != nil {
		return CleanupResult{}, err
	}
	return res, nil
}
