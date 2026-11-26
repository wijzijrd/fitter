// Package betacopy moves the data beta users wrote by hand from one database to
// another (staging to production, once, by the owner), and later deletes the
// copied users from the source. Strava activities, campaigns, goals and admin
// history are never copied. Copy reads the source and writes only the
// destination, in one transaction, so a failure leaves the destination as it was.
package betacopy

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/lib/pq"
)

// Options picks the users to copy. Set All, or AthleteIDs (Strava athlete ids).
type Options struct {
	All        bool
	AthleteIDs []int64
}

// Report counts rows written per table. On a dry run it counts what would be written.
type Report map[string]int64

var ErrNoSelection = errors.New("choose who to copy: pass All or a list of AthleteIDs")

// tokenCols are blanked on insert and never overwritten, so a destination user who has
// already signed in keeps their live Strava tokens when the copy runs again.
var tokenCols = []string{"access_token", "refresh_token", "token_expires_at", "last_synced_at"}

// Copy copies the selected users and their hand-written data from src to dst. dryRun
// runs the same writes inside a transaction and rolls back, so the counts are exact.
func Copy(ctx context.Context, src, dst *sql.DB, opt Options, dryRun bool) (Report, error) {
	if !opt.All && len(opt.AthleteIDs) == 0 {
		return nil, ErrNoSelection
	}
	tx, err := dst.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	rep := Report{}

	users, err := selectUsers(ctx, src, opt)
	if err != nil {
		return nil, err
	}
	copied := map[string]bool{}
	ids := []string{}
	for _, u := range users {
		id := u["id"].(string)
		copied[id] = true
		ids = append(ids, id)
	}

	// Users first, with banned_by held back: it may point at a user copied later in this loop.
	bannedBy := map[string]string{}
	for _, u := range users {
		if b, ok := u["banned_by"].(string); ok {
			bannedBy[u["id"].(string)] = b
			u["banned_by"] = nil
		}
		u["access_token"], u["refresh_token"] = "", ""
		u["token_expires_at"], u["last_synced_at"] = int64(0), int64(0)
		if _, err := upsert(ctx, tx, spec{table: "users", keys: []string{"id"}, skip: tokenCols}, u); err != nil {
			return nil, err
		}
		rep["users"]++
	}
	for id, b := range bannedBy {
		if !copied[b] {
			continue // banning admin isn't in this copy; keep the ban, drop the pointer
		}
		if _, err := tx.ExecContext(ctx, `UPDATE users SET banned_by = $1 WHERE id = $2`, b, id); err != nil {
			return nil, fmt.Errorf("users.banned_by: %w", err)
		}
	}

	// Exercises are one shared catalogue. Seeded ones get a random id per database, so a row
	// is matched by its key and the destination keeps its own id; stations are mapped below.
	exMap, err := copyExercises(ctx, src, tx, copied, rep)
	if err != nil {
		return nil, err
	}

	for _, table := range []struct {
		name, query string
		keys        []string
	}{
		{"custom_sports", `SELECT * FROM custom_sports`, []string{"id"}},
		{"custom_metrics", `SELECT * FROM custom_metrics`, []string{"id"}},
		{"custom_metric_sports", `SELECT * FROM custom_metric_sports`, []string{"metric_id", "sport_ref"}},
	} {
		rows, err := readRows(ctx, src, table.query)
		if err != nil {
			return nil, err
		}
		for _, r := range rows {
			if _, err := upsert(ctx, tx, spec{table: table.name, keys: table.keys}, r); err != nil {
				return nil, err
			}
			rep[table.name]++
		}
	}

	if err := copyManualActivities(ctx, src, tx, ids, exMap, rep); err != nil {
		return nil, err
	}

	// Manual activity ids are negative and come from a sequence. Put the next one below every
	// id now on the destination, so new manual activities can't collide with copied ones.
	if !dryRun {
		if _, err := tx.ExecContext(ctx, `SELECT setval('manual_activity_id',
			LEAST((SELECT MIN(strava_activity_id) FROM activities WHERE source = 'manual'),
			      (SELECT last_value FROM manual_activity_id)))`); err != nil {
			return nil, fmt.Errorf("reset manual_activity_id: %w", err)
		}
	}

	if dryRun {
		return rep, nil
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return rep, nil
}

func selectUsers(ctx context.Context, src *sql.DB, opt Options) ([]map[string]any, error) {
	if opt.All {
		return readRows(ctx, src, `SELECT * FROM users ORDER BY created_at`)
	}
	users, err := readRows(ctx, src, `SELECT * FROM users WHERE strava_athlete_id = ANY($1) ORDER BY created_at`, pq.Int64Array(opt.AthleteIDs))
	if err != nil {
		return nil, err
	}
	found := map[int64]bool{}
	for _, u := range users {
		found[u["strava_athlete_id"].(int64)] = true
	}
	var missing []string
	for _, id := range opt.AthleteIDs {
		if !found[id] {
			missing = append(missing, fmt.Sprint(id))
		}
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("not on source: strava athlete ids %s", strings.Join(missing, ", "))
	}
	return users, nil
}

func copyExercises(ctx context.Context, src *sql.DB, tx *sql.Tx, copied map[string]bool, rep Report) (map[string]string, error) {
	rows, err := readRows(ctx, src, `SELECT * FROM exercises ORDER BY created_at`)
	if err != nil {
		return nil, err
	}
	exMap := map[string]string{}
	for _, e := range rows {
		// A creator who isn't being copied can't be referenced; the exercise stays, as seeded.
		if cb, ok := e["created_by"].(string); ok && !copied[cb] {
			e["created_by"] = nil
		}
		var dstID string
		err := tx.QueryRowContext(ctx, `INSERT INTO exercises (id, name, key, measure, loaded, muscles, created_by, created_at, archived_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
			ON CONFLICT (key) DO UPDATE SET name = EXCLUDED.name, measure = EXCLUDED.measure, loaded = EXCLUDED.loaded,
				muscles = EXCLUDED.muscles, created_by = EXCLUDED.created_by, archived_at = EXCLUDED.archived_at
			RETURNING id`,
			e["id"], e["name"], e["key"], e["measure"], e["loaded"], e["muscles"], e["created_by"], e["created_at"], e["archived_at"],
		).Scan(&dstID)
		if err != nil {
			return nil, fmt.Errorf("exercises: %w", err)
		}
		exMap[e["id"].(string)] = dstID
		rep["exercises"]++
	}
	return exMap, nil
}

// copyManualActivities copies each copied user's manual activities with their lifts, blocks
// and stations. Children are replaced, not merged, so a second run ends in the same state.
func copyManualActivities(ctx context.Context, src *sql.DB, tx *sql.Tx, userIDs []string, exMap map[string]string, rep Report) error {
	acts, err := readRows(ctx, src, `SELECT * FROM activities WHERE source = 'manual' AND user_id = ANY($1) ORDER BY strava_activity_id`, pq.StringArray(userIDs))
	if err != nil {
		return err
	}
	actIDs := make([]int64, 0, len(acts))
	for _, a := range acts {
		id := a["strava_activity_id"].(int64)
		n, err := upsert(ctx, tx, spec{table: "activities", keys: []string{"strava_activity_id"}, guard: "activities.user_id = EXCLUDED.user_id"}, a)
		if err != nil {
			return err
		}
		if n == 0 {
			return fmt.Errorf("activity %d already exists on the destination for another user", id)
		}
		rep["activities"]++
		actIDs = append(actIDs, id)
		if _, err := tx.ExecContext(ctx, `DELETE FROM activity_blocks WHERE activity_id = $1`, id); err != nil { // stations cascade
			return err
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM activity_lifts WHERE activity_id = $1`, id); err != nil {
			return err
		}
	}
	if len(actIDs) == 0 {
		return nil
	}

	lifts, err := readRows(ctx, src, `SELECT activity_id, exercise, weight_kg, reps FROM activity_lifts WHERE activity_id = ANY($1) ORDER BY id`, pq.Int64Array(actIDs))
	if err != nil {
		return err
	}
	for _, l := range lifts {
		if _, err := upsert(ctx, tx, spec{table: "activity_lifts"}, l); err != nil {
			return err
		}
		rep["activity_lifts"]++
	}

	blocks, err := readRows(ctx, src, `SELECT id, activity_id, position, rounds FROM activity_blocks WHERE activity_id = ANY($1) ORDER BY id`, pq.Int64Array(actIDs))
	if err != nil {
		return err
	}
	blockMap := map[int64]int64{}
	srcBlockIDs := make([]int64, 0, len(blocks))
	for _, b := range blocks {
		var newID int64
		err := tx.QueryRowContext(ctx, `INSERT INTO activity_blocks (activity_id, position, rounds) VALUES ($1, $2, $3) RETURNING id`,
			b["activity_id"], b["position"], b["rounds"]).Scan(&newID)
		if err != nil {
			return fmt.Errorf("activity_blocks: %w", err)
		}
		blockMap[b["id"].(int64)] = newID
		srcBlockIDs = append(srcBlockIDs, b["id"].(int64))
		rep["activity_blocks"]++
	}
	if len(srcBlockIDs) == 0 {
		return nil
	}

	stations, err := readRows(ctx, src, `SELECT block_id, position, exercise_id, weight_kg, reps, distance_m, duration_s FROM activity_stations WHERE block_id = ANY($1) ORDER BY id`, pq.Int64Array(srcBlockIDs))
	if err != nil {
		return err
	}
	for _, s := range stations {
		exID, ok := exMap[s["exercise_id"].(string)]
		if !ok {
			return fmt.Errorf("station uses exercise %v, which the copy did not map", s["exercise_id"])
		}
		s["block_id"] = blockMap[s["block_id"].(int64)]
		s["exercise_id"] = exID
		if _, err := upsert(ctx, tx, spec{table: "activity_stations"}, s); err != nil {
			return err
		}
		rep["activity_stations"]++
	}
	return nil
}

// spec says how to write one row: a plain insert, or an upsert on keys.
type spec struct {
	table string
	keys  []string // conflict target; empty = plain insert
	skip  []string // columns never overwritten on conflict
	guard string   // extra WHERE on the conflict update; empty = always update
}

// upsert writes row, reports rows affected. The SQL is built only from table and
// column names in this package and from the source schema, never from user input.
func upsert(ctx context.Context, tx *sql.Tx, s spec, row map[string]any) (int64, error) {
	cols := make([]string, 0, len(row))
	for c := range row {
		cols = append(cols, c)
	}
	sort.Strings(cols)
	ph := make([]string, len(cols))
	args := make([]any, len(cols))
	for i, c := range cols {
		ph[i] = fmt.Sprintf("$%d", i+1)
		args[i] = row[c]
	}
	q := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)", s.table, strings.Join(cols, ", "), strings.Join(ph, ", "))
	if len(s.keys) > 0 {
		var set []string
		for _, c := range cols {
			if !contains(s.keys, c) && !contains(s.skip, c) {
				set = append(set, fmt.Sprintf("%s = EXCLUDED.%s", c, c))
			}
		}
		target := strings.Join(s.keys, ", ")
		if len(set) == 0 {
			q += fmt.Sprintf(" ON CONFLICT (%s) DO NOTHING", target)
		} else {
			q += fmt.Sprintf(" ON CONFLICT (%s) DO UPDATE SET %s", target, strings.Join(set, ", "))
			if s.guard != "" {
				q += " WHERE " + s.guard
			}
		}
	}
	res, err := tx.ExecContext(ctx, q, args...)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", s.table, err)
	}
	return res.RowsAffected()
}

// readRows runs query on db and returns each row keyed by column name.
func readRows(ctx context.Context, db *sql.DB, query string, args ...any) ([]map[string]any, error) {
	rs, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rs.Close()
	cols, err := rs.Columns()
	if err != nil {
		return nil, err
	}
	var out []map[string]any
	for rs.Next() {
		vals := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rs.Scan(ptrs...); err != nil {
			return nil, err
		}
		row := make(map[string]any, len(cols))
		for i, c := range cols {
			row[c] = vals[i]
		}
		out = append(out, row)
	}
	return out, rs.Err()
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}
