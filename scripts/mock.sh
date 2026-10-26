#!/usr/bin/env bash
# Runs a fresh, seeded fitter instance on this machine so the UI can be tried
# without a Strava account:
#
#   make mock
#
# Postgres comes from Homebrew (postgresql@17) on MOCK_PG_PORT, because Docker
# published ports are not reachable from the host under Rancher Desktop. svc and
# the UI run from source. The database is recreated on every run, so the seed
# never duplicates the sample campaign. Ctrl-C stops everything.
set -euo pipefail

root=$(cd "$(dirname "$0")/.." && pwd)
mock="$root/.mock"
pg_port=${MOCK_PG_PORT:-5433}
pg_bin=${PG_BIN:-$(brew --prefix postgresql@17 2>/dev/null || true)/bin}
ui_port=5173
svc_port=8081

if [ ! -x "$pg_bin/initdb" ]; then
  echo "mock needs Postgres 17: brew install postgresql@17 (or set PG_BIN to its bin dir)" >&2
  exit 1
fi

export DATABASE_URL="postgres://fitter:fitter@localhost:$pg_port/fitter?sslmode=disable"
export JWT_SECRET=mock-only-not-a-secret
export STRAVA_CLIENT_ID=mock STRAVA_CLIENT_SECRET=mock STRAVA_WEBHOOK_VERIFY_TOKEN=mock
export FRONTEND_URL="http://localhost:$ui_port"
export PUBLIC_BASE_URL="http://localhost:$svc_port"
export CORS_ORIGIN=$FRONTEND_URL
export PORT=$svc_port
# Global GOFLAGS may set -mod=mod, which workspace mode rejects (see test-db.sh).
export GOFLAGS=-mod=readonly

cleanup() {
  [ -n "${svc_pid:-}" ] && kill "$svc_pid" 2>/dev/null || true
  "$pg_bin/pg_ctl" -D "$mock/pgdata" stop -m fast >/dev/null 2>&1 || true
}
trap cleanup EXIT INT TERM

# Stop a Postgres left over from an interrupted run before its data dir is wiped.
"$pg_bin/pg_ctl" -D "$mock/pgdata" stop -m fast >/dev/null 2>&1 || true
rm -rf "$mock" && mkdir -p "$mock"

"$pg_bin/initdb" -D "$mock/pgdata" -U fitter --auth=trust >/dev/null
"$pg_bin/pg_ctl" -D "$mock/pgdata" -l "$mock/postgres.log" -w \
  -o "-p $pg_port -c listen_addresses=localhost" start >/dev/null
"$pg_bin/createdb" -h localhost -p "$pg_port" -U fitter fitter

# svc applies migrations before it starts listening, so healthz means the schema is ready to seed.
(cd "$root/app/svc" && go build -o "$mock/svc" .)
"$mock/svc" >"$mock/svc.log" 2>&1 &
svc_pid=$!
for _ in $(seq 60); do
  curl -fsS "http://localhost:$svc_port/healthz" >/dev/null 2>&1 && break
  sleep 0.5
done
curl -fsS "http://localhost:$svc_port/healthz" >/dev/null

(cd "$root/app/svc" && go run ./cmd/seed)
echo "svc log: $mock/svc.log"

cd "$root/app/ui"
[ -d node_modules ] || npm install
VITE_API_BASE_URL=$PUBLIC_BASE_URL npm run dev -- --port "$ui_port" --strictPort
