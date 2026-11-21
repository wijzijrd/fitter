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

## 4. Deploying to AWS (infrastructure-template)

Production-style deploys use [`infrastructure-template`](https://github.com/wijzijrd/infrastructure-template): Terraform-managed AWS (S3 + CloudFront for `app/ui`, Lightsail container service for `app/svc`), a hosted Supabase Postgres, secrets in SSM, and GitHub Actions deploying through OIDC roles (no AWS keys in GitHub). CloudFront serves the UI and forwards `/api/*` to the svc with an `X-Origin-Auth` header, so the browser is same-origin and CORS is dev-only.

Environments are `staging` and `prod`. Do staging first. Branches: feature → PR → `staging` (squash), then `staging` → `main` (merge commit, never squash).

### How `app/svc` meets the template's contract

| Template requirement | fitter |
| --- | --- |
| `GET /healthz` unauthenticated | `handler.Health` |
| Everything else needs `X-Origin-Auth` | `httpmw.OriginAuth`, enforced when `APP_ENV` is set (it is only set by the pipeline); `ORIGIN_SHARED_SECRET` is required then |
| Migrate on boot under an advisory lock, using `DATABASE_MIGRATION_URL` | `main.go` (`pg_advisory_lock`) |
| Serve with `DATABASE_URL` as the restricted app role, no prepared statements | yes |
| Listen on `API_PORT` (default 8080) | svc listens on `PORT` (default **8081**), so set the `API_PORT=8081` GitHub environment variable |

The deploy workflow reads every SSM parameter under `/<project>/<env>/` and injects it as an upper-cased env var (`database_url` → `DATABASE_URL`). Template-provided parameters (`database_url`, `database_migration_url`, `origin_shared_secret`, …) need no action. fitter's own config does (step 1.3).

### Initial setup (once)

**1. Infra repo (`infrastructure-template`, run from your laptop)**

1. Edit `project.env`: `PROJECT_NAME=fitter`, `DOMAIN`, `AWS_ACCOUNT_ID`, regions (`API_REGION` must support Lightsail), `SUPABASE_ORG_ID`, `GITHUB_OWNER`, `APP_REPO=fitter`, `AWS_PROFILE`. Run `make doctor`.
2. Export `SUPABASE_ACCESS_TOKEN` (Supabase → Account → Access Tokens).
3. Declare fitter's secrets in `manual_secret_names` in both `envs/staging/variables.tf` and `envs/prod/variables.tf`:
   `strava_client_id`, `strava_client_secret`, `strava_webhook_verify_token`, `jwt_secret`, `frontend_url`, `public_base_url` (and optionally `admin_strava_athlete_ids`). They become `STRAVA_CLIENT_ID`, … at deploy time.
4. Bootstrap, in order: `make bootstrap`, `make dns`, `make github-setup`, `make oidc-claims` (commit `project.env`), `make iam`, `make github-roles`, `make verify-roles`.
5. `make plan ENV=staging`, then PR into the infra repo's `staging` branch; merge so CI applies. A merged PR is not a successful apply, so watch the run.
6. `make github-app-vars ENV=staging` pushes bucket, distribution, Lightsail and ECR names into this repo's GitHub `staging` environment.
7. Fill each secret: `make ssm-set ENV=staging NAME=<name>`. Values: Strava client ID/secret from step 1 above; `jwt_secret` from `openssl rand -hex 32`; `strava_webhook_verify_token` any random string; `frontend_url` = `https://staging.<domain>`; `public_base_url` = `https://staging.<domain>/api`.

**2. This repo**

1. Add the deploy workflows (`ci.yml`, `deploy-web.yml`, `deploy-api.yml`, …) from the template's `examples/app/.github/workflows/`, or run `make app-repo` from the infra repo (check `scripts/app-repo.sh` first, since this repo already exists). Point them at the monorepo: API Docker context `app/svc`, web build in `app/ui`.
2. Set the web workflow's `SPA_FALLBACK` to match `spa_fallback_path` in the infra repo's `envs/<env>/variables.tf`.
3. Create the `staging` branch, push, and merge to it. `Deploy web` and `Deploy api` run; a failed API deployment leaves the previous one serving.

**3. Strava (staging)**

1. In your Strava API app, set **Authorization Callback Domain** to the staging domain.
2. Confirm `https://staging.<domain>/api/healthz` answers 200, then log in end to end.
3. Register the webhook once against the live backend:

   ```sh
   cd app/svc
   STRAVA_CLIENT_ID=... STRAVA_CLIENT_SECRET=... STRAVA_WEBHOOK_VERIFY_TOKEN=... \
     PUBLIC_BASE_URL=https://staging.<domain>/api \
     go run ./cmd/webhooksetup create
   ```

**4. Prod**

1. Create a **second Strava API app** (one webhook subscription per app) and use its credentials for prod.
2. `api_enabled` is `false` in `envs/prod/variables.tf` on purpose. Flip it to `true`, `make ssm-set ENV=prod` for each secret (prod `frontend_url` / `public_base_url` use the apex domain), and `make github-app-vars ENV=prod`.
3. `make promote` in the infra repo, merge `staging → main` with a **merge commit**; repeat in this repo to deploy prod.
4. Register the prod webhook as in 3.3 with the prod `PUBLIC_BASE_URL`.

### Routine deploys

1. Branch off `staging`, open a PR into `staging`, squash-merge. CI deploys staging.
2. Verify on `https://staging.<domain>`.
3. Promote `staging → main` with a merge commit. CI deploys prod.

### Rollback

- **Web:** re-run `Deploy web` (`workflow_dispatch`) with an older `ref`.
- **API:** re-run `Deploy api` with `image_tag=sha-<previous>`.
- **Database:** migrations are forward-only (expand/contract: the previous binary must keep working against the new schema). Recovery is a restore from the nightly dump (`docs/runbooks/restore-drill.md` in the infra repo).

## Webhooks note

Strava allows exactly one webhook subscription per API application (not per user), which is why registering it is a one-off `cmd/webhooksetup` command rather than something the server does on every boot. The backend must be reachable over public HTTPS for Strava to deliver events to `/webhooks/strava` — see the Caddy overlay above.
