#!/usr/bin/env bash
# Runs app/svc's Go tests, including the Postgres-backed ones, against a
# throwaway Postgres on whichever Docker engine DOCKER_CONTEXT points at, e.g.
#
#   DOCKER_CONTEXT=quillit scripts/test-db.sh
#
# Test binaries are cross-compiled here and copied in, so the engine only runs
# Postgres plus a tiny Alpine container: no source, Go toolchain, published
# ports or bind mounts are needed on the remote host. Everything it creates is
# named fitter-test-* and removed on exit.
set -euo pipefail

cd "$(dirname "$0")/../app/svc"
arch=$(docker version --format '{{.Server.Arch}}')
bin=$(mktemp -d)
net="fitter-test-$$"

cleanup() {
  docker rm -f "$net-run" "$net-pg" >/dev/null 2>&1 || true
  docker network rm "$net" >/dev/null 2>&1 || true
  rm -rf "$bin"
}
trap cleanup EXIT

echo "Compiling test binaries for linux/${arch}…"
# Global GOFLAGS may set -mod=mod, which workspace mode rejects.
export GOFLAGS=-mod=readonly CGO_ENABLED=0 GOOS=linux GOARCH="$arch"
# One binary per package with tests, named by path: db/ and internal/db/ share a basename.
for pkg in $(go list -f '{{if or .TestGoFiles .XTestGoFiles}}{{.Dir}}{{end}}' ./...); do
  rel=${pkg#"$PWD"/}
  go test -c -o "$bin/${rel//\//_}.test" "./$rel"
done

docker network create "$net" >/dev/null
docker run -d --name "$net-pg" --network "$net" \
  -e POSTGRES_USER=fitter -e POSTGRES_PASSWORD=fitter -e POSTGRES_DB=fitter \
  postgres:18-alpine >/dev/null
until docker exec "$net-pg" pg_isready -U fitter -q 2>/dev/null; do sleep 1; done

docker create --name "$net-run" --network "$net" \
  -e TEST_DATABASE_URL="postgres://fitter:fitter@$net-pg:5432/fitter?sslmode=disable" \
  alpine:3 sh -c 'status=0; for t in /t/*.test; do echo "== $(basename "$t" .test)"; "$t" "$@" || status=1; done; exit $status' -- "$@" >/dev/null
docker cp "$bin/." "$net-run:/t"
docker start -a "$net-run"
