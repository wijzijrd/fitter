// betacopy moves the data beta users wrote by hand from staging to production, once,
// and later removes those users from staging. It runs only from the owner's machine,
// with both database URLs in the environment. No workflow holds either URL.
//
// Usage (both URLs in the shell; use the superuser DSNs, not fitter_app):
//
//	export SOURCE_DATABASE_URL=...   # staging
//	export DEST_DATABASE_URL=...     # production
//
//	go run ./cmd/betacopy -athletes 123,456 -dry-run   # counts only, writes nothing
//	go run ./cmd/betacopy -athletes 123,456            # copy those beta users
//	go run ./cmd/betacopy -all                         # copy every user on staging
//
//	go run ./cmd/betacopy -cleanup -dry-run           # who would leave staging
//	go run ./cmd/betacopy -cleanup                    # after sign-off: typed confirmation
//
// Copy never deletes anything, and rerunning it changes nothing (upsert by key). Cleanup
// deletes only users who also exist on the destination (same id). Strava activities,
// campaigns (other than the copied users' own), goals and admin history are not copied.
package main

import (
	"bufio"
	"context"
	"database/sql"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/fitter/svc/internal/betacopy"

	_ "github.com/jackc/pgx/v5/stdlib"
)

const cleanupPhrase = "delete copied users from staging"

func main() {
	athletes := flag.String("athletes", "", "comma-separated Strava athlete ids of the beta users to copy")
	all := flag.Bool("all", false, "copy every user on the source")
	dryRun := flag.Bool("dry-run", false, "count what would change and roll back; writes nothing")
	cleanup := flag.Bool("cleanup", false, "delete the copied users from the source instead of copying")
	flag.Parse()

	srcURL, dstURL := mustEnv("SOURCE_DATABASE_URL"), mustEnv("DEST_DATABASE_URL")
	if srcURL == dstURL {
		fatal("source and destination are the same database; refusing", nil)
	}
	src := open(srcURL)
	defer src.Close()
	dst := open(dstURL)
	defer dst.Close()
	ctx := context.Background()

	if *cleanup {
		runCleanup(ctx, src, dst, *dryRun)
		return
	}

	opt := betacopy.Options{All: *all}
	if *athletes != "" {
		ids, err := parseIDs(*athletes)
		if err != nil {
			fatal("bad -athletes", err)
		}
		opt.AthleteIDs = ids
	}
	rep, err := betacopy.Copy(ctx, src, dst, opt, *dryRun)
	if err != nil {
		fatal("copy failed; destination unchanged", err)
	}
	printReport("copy", rep)
	if *dryRun {
		fmt.Println("dry run: nothing written")
	}
}

func runCleanup(ctx context.Context, src, dst *sql.DB, dryRun bool) {
	res, err := betacopy.Cleanup(ctx, src, dst, true) // always preview first
	if err != nil {
		fatal("cleanup preview failed; source unchanged", err)
	}
	if len(res.Names) == 0 {
		fmt.Println("no users on the source also exist on the destination; nothing to remove")
		return
	}
	fmt.Println("would remove from staging:", strings.Join(res.Names, ", "))
	printReport("cleanup", res.Report)
	if dryRun {
		fmt.Println("dry run: nothing deleted")
		return
	}
	fmt.Printf("Type %q to continue: ", cleanupPhrase)
	line, _ := bufio.NewReader(os.Stdin).ReadString('\n')
	if strings.TrimSpace(line) != cleanupPhrase {
		fatal("confirmation did not match; nothing deleted", nil)
	}
	if _, err := betacopy.Cleanup(ctx, src, dst, false); err != nil {
		fatal("cleanup failed; source unchanged", err)
	}
	fmt.Println("removed")
}

func printReport(label string, rep betacopy.Report) {
	keys := make([]string, 0, len(rep))
	for k := range rep {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		slog.Info(label, "table", k, "rows", rep[k])
	}
}

func parseIDs(s string) ([]int64, error) {
	var out []int64
	for _, part := range strings.Split(s, ",") {
		id, err := strconv.ParseInt(strings.TrimSpace(part), 10, 64)
		if err != nil || id <= 0 {
			return nil, fmt.Errorf("%q is not a Strava athlete id", part)
		}
		out = append(out, id)
	}
	return out, nil
}

func open(url string) *sql.DB {
	db, err := sql.Open("pgx", url)
	if err != nil {
		fatal("open database failed", err)
	}
	return db
}

func mustEnv(k string) string {
	v := os.Getenv(k)
	if v == "" {
		fatal(k+" is not set", nil)
	}
	return v
}

func fatal(msg string, err error) {
	if err != nil {
		slog.Error(msg, "err", err)
	} else {
		slog.Error(msg)
	}
	os.Exit(1)
}
