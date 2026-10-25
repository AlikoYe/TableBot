# Load .env (if present) and export its variables to every recipe.
ifneq (,$(wildcard .env))
include .env
export
endif

COMPOSE_PROJECT ?= tablebot
COMPOSE := docker compose -p $(COMPOSE_PROJECT) -f deploy/docker-compose.yml $(if $(wildcard .env),--env-file .env)

POSTGRES_USER ?= tablebot
POSTGRES_PASSWORD ?= tablebot
POSTGRES_PORT ?= 5432
TEST_DB ?= tablebot_test
TEST_DATABASE_URL ?= postgres://$(POSTGRES_USER):$(POSTGRES_PASSWORD)@localhost:$(POSTGRES_PORT)/$(TEST_DB)?sslmode=disable
OWNER ?= 1

.DEFAULT_GOAL := help

.PHONY: help
help: ## Show available targets
	@grep -E '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-14s\033[0m %s\n", $$1, $$2}'

# ---- Local development -----------------------------------------------------

.PHONY: dev
dev: db-up migrate-up ## Run Postgres (docker), then API and Vite on the host
	@$(MAKE) -j2 --no-print-directory dev-api dev-web

.PHONY: dev-api
dev-api:
	cd backend && go run ./cmd/api

.PHONY: dev-web
dev-web: frontend/node_modules
	cd frontend && npm run dev

frontend/node_modules: frontend/package-lock.json
	cd frontend && npm ci
	@touch $@

.PHONY: db-up
db-up: ## Start only Postgres and wait until it is healthy
	$(COMPOSE) up -d --wait postgres

.PHONY: seed
seed: ## Load dev seed data; OWNER=<telegram_id> owns all venues (default 1)
	$(COMPOSE) exec -T postgres sh -c 'psql -v ON_ERROR_STOP=1 -v owner_id=$(OWNER) -U "$$POSTGRES_USER" -d "$$POSTGRES_DB"' < backend/seed/dev.sql

.PHONY: db-reset
db-reset: ## DELETE the local Postgres volume and start fresh (dev data is lost)
	$(COMPOSE) rm -sf postgres
	docker volume rm -f $(COMPOSE_PROJECT)_pgdata
	$(MAKE) --no-print-directory db-up migrate-up

# ---- Migrations ------------------------------------------------------------

.PHONY: migrate-up
migrate-up: ## Apply all pending migrations
	cd backend && go run ./cmd/migrate up

.PHONY: migrate-down
migrate-down: ## Roll back the most recent migration
	cd backend && go run ./cmd/migrate down

.PHONY: migrate-status
migrate-status: ## Show migration status
	cd backend && go run ./cmd/migrate status

# ---- Build / test / lint ---------------------------------------------------

.PHONY: build
build: build-api build-web ## Build backend binaries and the frontend bundle

.PHONY: build-api
build-api:
	cd backend && CGO_ENABLED=0 go build -o bin/api ./cmd/api && CGO_ENABLED=0 go build -o bin/migrate ./cmd/migrate

.PHONY: build-web
build-web: frontend/node_modules
	cd frontend && npm run build

.PHONY: test
test: ## Run backend unit tests (DB tests are skipped)
	cd backend && go test -race ./...

.PHONY: test-db
test-db: db-up ## Run all backend tests, including DB integration tests, against $(TEST_DB)
	$(COMPOSE) exec -T postgres sh -c 'psql -U "$$POSTGRES_USER" -d postgres -tAc "SELECT 1 FROM pg_database WHERE datname = '"'"'$(TEST_DB)'"'"'" | grep -q 1 || createdb -U "$$POSTGRES_USER" $(TEST_DB)'
	cd backend && TEST_DATABASE_URL='$(TEST_DATABASE_URL)' go test -race -count=1 -p 1 ./...

.PHONY: lint
lint: ## gofmt check + go vet (+ golangci-lint if installed)
	@cd backend && out=$$(gofmt -l .); if [ -n "$$out" ]; then echo "gofmt needed:"; echo "$$out"; exit 1; fi
	cd backend && go vet ./...
	@if command -v golangci-lint >/dev/null; then cd backend && golangci-lint run; else echo "golangci-lint not installed, skipping"; fi

# ---- Full stack in docker --------------------------------------------------

.PHONY: up
up: build-web ## Build the frontend and start postgres, migrate, api, caddy
	$(COMPOSE) up -d --build

.PHONY: down
down: ## Stop the docker stack (keeps data)
	$(COMPOSE) down

.PHONY: logs
logs: ## Tail docker stack logs
	$(COMPOSE) logs -f
