.PHONY: up down logs ps setup help buf-generate sqlc-generate test-db

COMPOSE ?= docker compose -f infra/docker-compose.yml --project-directory .

up: setup       ## Build and start the full stack (UI at http://localhost:8080)
	$(COMPOSE) up --build -d
	@echo "Fitter is starting — open http://localhost:8080"

down:           ## Stop and remove all containers
	$(COMPOSE) down

logs:           ## Tail logs from all services
	$(COMPOSE) logs -f

ps:             ## Show service status
	$(COMPOSE) ps

setup:          ## Create .env from .env.example if missing; warn on placeholder secrets
	@test -f .env || (cp .env.example .env && echo "Created .env from .env.example — edit it to set real secrets")
	@grep -q '^JWT_SECRET=replace-with' .env && \
		echo "WARNING: JWT_SECRET is still the placeholder. Generate one with: openssl rand -hex 32" || true
	@grep -q '^STRAVA_CLIENT_ID=replace-with' .env && \
		echo "WARNING: STRAVA_CLIENT_ID is still the placeholder. Set it from https://www.strava.com/settings/api" || true

buf-generate:   ## Regenerate Go + TS code from proto/ (needs buf + plugins on PATH)
	buf generate

sqlc-generate:  ## Regenerate app/svc/internal/db from db/queries + db/migrations
	cd app/svc && sqlc generate

test-db:        ## Run svc Go tests against a throwaway Postgres (set DOCKER_CONTEXT=quillit to run it remotely)
	scripts/test-db.sh

help:           ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*##' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*##"}; {printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2}'
