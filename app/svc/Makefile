BINARY := fitter-svc
SETUP_HINT := JWT_SECRET, STRAVA_CLIENT_ID and STRAVA_CLIENT_SECRET

.PHONY: check-env sqlc-generate

check-env:
	@test -f .env || (echo "ERROR: .env missing. Run 'make setup' first." && exit 1)
	@grep -q '^JWT_SECRET=' .env || (echo "ERROR: JWT_SECRET not set in .env" && exit 1)
	@grep '^JWT_SECRET=' .env | grep -q 'replace-with' \
		&& (echo "ERROR: JWT_SECRET is still the placeholder. Generate one with: openssl rand -hex 32" && exit 1) || true
	@grep '^STRAVA_CLIENT_ID=' .env | grep -q 'replace-with' \
		&& (echo "ERROR: STRAVA_CLIENT_ID is still the placeholder. Set it from https://www.strava.com/settings/api" && exit 1) || true

sqlc-generate:  ## Regenerate internal/db from db/queries + db/migrations
	sqlc generate

include ../Makefile.common
