# Fitter

A Strava activity tracker for you and your friends — join shared campaigns, track personal fitness goals, and see a per-campaign feed, leaderboard, and progress timeline.

## Structure

```
fitter/
├── app/
│   ├── ui/    Vue 3 + Vite + Pinia frontend
│   └── svc/   Go backend — ConnectRPC + goose + sqlc, Postgres storage
├── proto/     fitter.v1 service/message definitions (shared source for Go + TS codegen)
├── infra/     docker-compose files + Caddyfile
├── buf.yaml / buf.gen.yaml   codegen config for proto/ -> app/svc/gen (Go) and app/ui/src/gen (TS)
└── go.work    Go workspace tying together app/svc
```

## 1. Register a Strava API app

Create one at <https://www.strava.com/settings/api>. You'll need:

- **Authorization Callback Domain**: the bare host of your `PUBLIC_BASE_URL` (e.g. `api.fitter.example`, or `localhost` for local dev).
- The generated **Client ID** and **Client Secret**.

## 2. Docker (recommended for a real deployment)

```sh
git clone <this repo> fitter
cd fitter
cp .env.example .env
```

Edit `.env`: set `STRAVA_CLIENT_ID`, `STRAVA_CLIENT_SECRET`, `STRAVA_WEBHOOK_VERIFY_TOKEN`, `JWT_SECRET` (`openssl rand -hex 32`), and real `FRONTEND_URL`/`PUBLIC_BASE_URL` domains.

```sh
make up
```

This builds and starts `svc` (backend) and `ui` (frontend, nginx) via `infra/docker-compose.yml`. For a public deployment, add the Caddy overlay so both services get real HTTPS certs and Strava's webhook can reach the backend:

```sh
docker compose -f infra/docker-compose.yml -f infra/docker-compose.caddy.yml --project-directory . up --build -d
```

Once running, register the app's one-time Strava webhook subscription against the live backend:

```sh
cd app/svc
STRAVA_CLIENT_ID=... STRAVA_CLIENT_SECRET=... STRAVA_WEBHOOK_VERIFY_TOKEN=... \
  PUBLIC_BASE_URL=https://api.fitter.example \
  go run ./cmd/webhooksetup create
```

Other useful `make` targets: `down`, `logs`, `ps`, `help`.

## 3. Native local development

**Prerequisites:** Go 1.27+, Node.js 22+, and — only if you're changing `proto/` — buf plus its codegen plugins:

```sh
brew install bufbuild/buf/buf
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest connectrpc.com/connect/cmd/protoc-gen-connect-go@latest
npm install -g @bufbuild/protoc-gen-es
```

### Backend (port 8081)

```sh
cd app/svc
make setup   # copies .env.example -> .env
# edit .env: STRAVA_CLIENT_ID/SECRET, STRAVA_WEBHOOK_VERIFY_TOKEN, JWT_SECRET
make dev
```

Migrations run automatically on startup (goose, embedded). To change the schema: add a file under `db/migrations/`, add/edit queries under `db/queries/`, then `make sqlc-generate` (from repo root) to refresh `internal/db`.

### Frontend (port 5173)

```sh
cd app/ui
cp .env.example .env   # VITE_API_BASE_URL=http://localhost:8081
npm install
npm run dev
```

Open **http://localhost:5173** and log in with Strava.

### Regenerating proto code

After editing anything under `proto/`:

```sh
make buf-generate
```

This regenerates Go types + ConnectRPC service stubs into `app/svc/gen`, and TypeScript types + Connect client descriptors into `app/ui/src/gen`.

## Webhooks note

Strava allows exactly one webhook subscription per API application (not per user), which is why registering it is a one-off `cmd/webhooksetup` command rather than something the server does on every boot. The backend must be reachable over public HTTPS for Strava to deliver events to `/webhooks/strava` — see the Caddy overlay above.
