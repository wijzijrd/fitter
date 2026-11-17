# CLAUDE.md

Guidance for Claude Code working in this repository. Read this first; follow the pointers only
when the task needs them.

## What this repository is

The application for **fitter** (`https://kombeweeg.com`, staging at
`https://staging.kombeweeg.com`). It was scaffolded from the infrastructure repository
`wijzijrd/infrastructure`, which owns every AWS/Supabase resource, the IAM roles this repo's workflows
assume, and the SSM secrets the API reads. **Nothing in this repo creates infrastructure** — if a
task needs a bucket, a domain, a secret or a role, it belongs in the infra repo.

Layout:

```
app/ui/   Vite + Vue frontend (builds to app/ui/dist)
app/svc/  Go ConnectRPC backend (container; listens on 8081)
.github/workflows/   already present — the CI/CD contract (see README.md)
```

## Where to start

1. `README.md` — what's wired up and the contract the code must honour. Two minutes.
2. If the task touches deployment, config, secrets or environments: the infra repo's
   `docs/app-contract.md` (variables, SSM parameters, request path, roles).
3. Scaffolding a new service from scratch: build the smallest thing that satisfies the contract
   first (`/healthz`, `X-Origin-Auth` guard, startup migrations creating `fitter_app`), push to
   `staging`, watch `Deploy api`/`Deploy web` go green, then build features.
4. Debugging a failed deploy: `gh run view <id> --log-failed`; for the API, Lightsail keeps the
   previous deployment serving — read the "Wait for ACTIVE" step's container log.

## Rules

- **Branches:** feature branch from `staging` → PR → squash-merge into `staging`. Promotion
  `staging → main` is a PR merged with a **merge commit** (`gh pr merge --merge`), never squash,
  never `--delete-branch`. Never push to `staging` or `main` directly. `promotion-guard` red = stop.
- **Environments:** `staging`/`production` GitHub Environments are the OIDC trust boundary. Never
  add `environment:` to a scheduled workflow (`db-backup`, `supabase-keepalive`) — they run on
  `main` with ref-pinned roles by design. Never use `pull_request_target`.
- **Secrets:** none in this repo. Runtime config comes from SSM via `deploy-api.yml`; build-time
  `PUBLIC_*` values are GitHub environment variables set by the infra repo. Don't paste values into
  workflows or `.env` files that get committed.
- **API contract:** `/healthz` public; everything else requires `X-Origin-Auth ==
  ORIGIN_SHARED_SECRET`; migrate on startup with `DATABASE_MIGRATION_URL`, serve with
  `DATABASE_URL` as `fitter_app`; migrations forward-only and expand/contract (old and new
  containers overlap during a rollout).
- **Web contract:** `deploy-web.yml`'s `env:` block must match the build (`BUILD_DIR`,
  `IMMUTABLE_GLOB`, `SPA_FALLBACK` — and `SPA_FALLBACK` must equal `spa_fallback_path` in the infra
  repo's `envs/*/variables.tf`).
- **Retries:** ECR tags are immutable — don't re-run a failed `Deploy api` on the same commit;
  dispatch it with `image_tag=sha-<commit>` for downstream failures, or push a new commit.
- **Costs:** don't add paid services or raise `api_power` without asking; that's an infra-repo
  change anyway.
