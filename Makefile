SHELL := /bin/bash
APP := api
PKG := ./cmd/api
BIN := bin/$(APP)
GOFLAGS := -trimpath
LDFLAGS := -s -w -X main.version=$(shell git describe --tags --always --dirty 2>/dev/null || echo dev)

.PHONY: help run-dev run-stage run-prod build test fmt vet tidy up up-db down down-safe nuke logs-db logs-api migrate up-mailpit

help:
	@grep -E '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | sed -E 's/:.*##/:/;s/Makefile://'

run-dev: ## run API with dev secrets (Infisical)
	infisical run --env=dev -- go run $(PKG)/main.go

run-stage: ## run API with stage secrets
	infisical run --env=stage -- go run $(PKG)/main.go

run-prod: ## run API with prod secrets
	infisical run --env=prod -- go run $(PKG)/main.go

build: ## build binary
	go build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(BIN) $(PKG)/main.go

test: ## run tests
	go test ./...

fmt: ## format & tidy
	go fmt ./...
	go mod tidy

vet: ## go vet
	go vet ./...

# ---- Docker ----
up: ## compose up (all)
	@docker compose up -d

up-db: ## start only db & mailpit
	@docker compose up -d db mailpit

down-safe: ## stop containers, KEEP VOLUMES (recommended)
	@docker compose down

down: down-safe ## alias

nuke: ## WARNING: stop & REMOVE VOLUMES (DB DATA LOST)
	@docker compose down -v

logs-db: ## follow Postgres logs
	docker compose logs -f db

logs-api: ## follow API logs (if containerized later)
	docker compose logs -f api

# ---- Migrations (Goose + Infisical) ----
migrate-dev: ## goose up (dev)
	infisical run --env=dev -- goose -dir ./migrations postgres "$$DATABASE_URL" up

migrate-stage: ## goose up (stage)
	infisical run --env=stage -- goose -dir ./migrations postgres "$$DATABASE_URL" up

migrate-prod: ## goose up (prod)
	infisical run --env=prod -- goose -dir ./migrations postgres "$$DATABASE_URL" up

up-mailpit: ## open Mailpit UI (if needed)
	@open http://localhost:8025 2>/dev/null || true

.PHONY: db-clean-test-dev db-clean-test-stage db-clean-test-prod

# Delete test users (dev). FK cascades will clean related rows.
db-clean-test-dev:
	infisical run --env=dev -- bash -lc 'psql "$$DATABASE_URL" -v ON_ERROR_STOP=1 -c "\
		DELETE FROM users \
		WHERE email ILIKE '\''test-%'\'' OR email ILIKE '\''%@example.test'\''; \
	"'

db-clean-test-stage:
	infisical run --env=stage -- bash -lc 'psql "$$DATABASE_URL" -v ON_ERROR_STOP=1 -c "\
		DELETE FROM users \
		WHERE email ILIKE '\''test-%'\'' OR email ILIKE '\''%@example.test'\''; \
	"'

db-clean-test-prod:
	infisical run --env=prod -- bash -lc 'psql "$$DATABASE_URL" -v ON_ERROR_STOP=1 -c "\
		DELETE FROM users \
		WHERE email ILIKE '\''test-%'\'' OR email ILIKE '\''%@example.test'\''; \
	"'

.PHONY: mailpit-purge db-count-test

# Nuke dev emails in Mailpit (handy between runs)
mailpit-purge:
	curl -s -X DELETE http://localhost:8025/api/v1/messages >/dev/null || true

# See how many rows you'd delete
db-count-test:
	infisical run --env=dev -- bash -lc 'psql "$$DATABASE_URL" -tAc "\
		SELECT count(*) FROM users \
		WHERE email ILIKE '\''test-%'\'' OR email ILIKE '\''%@example.test'\''; \
	"'