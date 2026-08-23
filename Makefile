.PHONY: help fetch install-fe install-fe-dev build build-fe build-be build-silent dev dev-fe dev-be test test-be test-fe test-cover test-cover-fe test-postgres benchmark-analysis check verify check-fe check-be typecheck-fe lint lint-fe lint-be audit-be docker docker-config docker-config-media docker-config-postgres docker-config-secrets docker-config-musicbee-secrets docker-config-postgres-secrets docker-config-analyzer docker-config-analyzer-secrets docker-up docker-up-media docker-up-postgres docker-up-secrets docker-up-musicbee-secrets docker-up-postgres-secrets docker-up-analyzer docker-up-analyzer-secrets docker-down clean

.DEFAULT_GOAL := help

FE_DIR := web
GO_MODULE := $(shell go list -m)
DOCKER_IMAGE ?= music-online-go:local
VERSION ?= dev
VCS_REF ?= unknown
BUILD_DATE ?= 1970-01-01T00:00:00Z

# Detect OS for binary extension
ifeq ($(OS),Windows_NT)
    BINARY := music-online.exe
    DEV_CMD := powershell -NoProfile -ExecutionPolicy Bypass -File scripts/dev.ps1
else
    BINARY := music-online
    DEV_CMD := sh scripts/dev.sh
endif

help: ## Show available commands
	@echo "Music Online Go - Available Commands"
	@echo ""
	@echo "=== Dependencies ==="
	@echo "  make fetch          Fetch frontend deps and sync Go vendor"
	@echo ""
	@echo "=== Install ==="
	@echo "  make install-fe      Install frontend deps from lockfile"
	@echo "  make install-fe-dev  Install/update frontend deps for development"
	@echo ""
	@echo "=== Build ==="
	@echo "  make build          Build frontend + backend (production)"
	@echo "  make build-silent   Build frontend with quieter output + backend"
	@echo "  make build-fe       Build frontend only"
	@echo "  make build-be       Build backend only (requires dist/)"
	@echo ""
	@echo "=== Develop ==="
	@echo "  make dev            Start frontend + backend together"
	@echo "  make dev-fe         Start frontend dev server (hot reload)"
	@echo "  make dev-be         Start backend dev server"
	@echo ""
	@echo "=== Quality ==="
	@echo "  make check          Run non-mutating checks"
	@echo "  make verify         Run tests + non-mutating checks"
	@echo "  make lint           Run all linters (Go + ESLint)"
	@echo "  make lint-fe        Run ESLint on frontend"
	@echo "  make lint-be        Run Go vet"
	@echo "  make audit-be       Run non-mutating golangci-lint audit"
	@echo "  make test           Run backend + frontend tests"
	@echo "  make test-cover     Run Go tests with a coverage summary"
	@echo "  make test-cover-fe  Run frontend tests with a coverage summary"
	@echo "  make test-postgres  Run opt-in PostgreSQL integration tests"
	@echo "  make benchmark-analysis ARGS='...' Compare analyzer result files"
	@echo ""
	@echo "=== Docker ==="
	@echo "  make docker                 Build Docker image"
	@echo "  make docker-config          Validate the SQLite Compose configuration"
	@echo "  make docker-config-media    Validate Compose with a read-only media mount"
	@echo "  make docker-config-postgres Validate the PostgreSQL Compose configuration"
	@echo "  make docker-config-secrets  Validate the SQLite Compose secrets override"
	@echo "  make docker-config-musicbee-secrets Validate JWT + MusicBee secret overrides"
	@echo "  make docker-config-postgres-secrets Validate PostgreSQL + secrets overrides"
	@echo "  make docker-config-analyzer Validate the optional analyzer profile"
	@echo "  make docker-config-analyzer-secrets Validate analyzer with a file-backed token"
	@echo "  make docker-up              Start the SQLite Compose deployment"
	@echo "  make docker-up-media        Start SQLite with a read-only media mount"
	@echo "  make docker-up-postgres     Start the PostgreSQL Compose deployment"
	@echo "  make docker-up-secrets      Start SQLite with a Docker secret"
	@echo "  make docker-up-musicbee-secrets Start SQLite with JWT + MusicBee secrets"
	@echo "  make docker-up-postgres-secrets Start PostgreSQL with Docker secrets"
	@echo "  make docker-up-analyzer      Start SQLite with the optional analyzer profile"
	@echo "  make docker-up-analyzer-secrets Start analyzer with a file-backed token"
	@echo "  make docker-down            Stop Compose services (preserves data volumes)"
	@echo "  make clean          Remove build artifacts"

install-fe: ## Install frontend dependencies from lockfile
	pnpm --dir $(FE_DIR) install --frozen-lockfile

install-fe-dev: ## Install/update frontend dependencies for local development
	pnpm --dir $(FE_DIR) install

fetch: ## Fetch frontend deps and sync Go vendor
	pnpm --dir $(FE_DIR) install --frozen-lockfile
	go mod vendor

build: build-fe build-be ## Build frontend then backend

build-silent: ## Build frontend with quieter output, then backend
	pnpm --dir $(FE_DIR) install --frozen-lockfile --silent
	pnpm --dir $(FE_DIR) exec vue-tsc -b
	pnpm --dir $(FE_DIR) exec vite build --logLevel warn
	go build -v -o $(BINARY) ./cmd/server

build-fe: ## Build Vue frontend, output to cmd/server/dist/
	pnpm --dir $(FE_DIR) install --frozen-lockfile
	pnpm --dir $(FE_DIR) build

build-be: ## Build Go server binary
	go build -v -o $(BINARY) ./cmd/server

dev: ## Start frontend and backend together
	$(DEV_CMD)

dev-fe: ## Start Vite dev server at localhost:5173
	pnpm --dir $(FE_DIR) dev

dev-be: ## Start Go server at localhost:8080
	go run ./cmd/server

test: test-be test-fe ## Run backend + frontend tests

test-be: ## Run Go tests
	go test -v ./...

test-fe: ## Run frontend unit and component tests
	pnpm --dir $(FE_DIR) test

test-cover: ## Run Go tests and print statement coverage by function
	go test -coverpkg=$(GO_MODULE)/... -coverprofile=coverage.out ./...
	go tool cover -func coverage.out

test-cover-fe: ## Run frontend tests and print coverage
	pnpm --dir $(FE_DIR) test:coverage

test-postgres: ## Run opt-in PostgreSQL integration tests (set MUSIC_ONLINE_TEST_POSTGRES_DSN)
	go test -v ./internal/repository -run 'Postgres'

benchmark-analysis: ## Compare analyzer candidates (pass -manifest/-result through ARGS)
	go run ./cmd/analysis-benchmark $(ARGS)

check: check-be check-fe ## Run non-mutating checks

verify: test-be test-fe check ## Run tests + non-mutating checks

check-be: ## Run Go vet
	go vet ./...

check-fe: typecheck-fe ## Run frontend non-mutating checks
	pnpm --dir $(FE_DIR) exec eslint . --quiet

typecheck-fe: ## Run Vue/TypeScript typecheck
	pnpm --dir $(FE_DIR) exec vue-tsc -b

# ── Lint ──────────────────────────────────────────────────

lint: ## Run all linters (Go fmt + vet + ESLint)
	go fmt ./...
	go vet ./...
	pnpm --dir $(FE_DIR) lint --quiet

lint-fe: ## Run ESLint on frontend
	pnpm --dir $(FE_DIR) exec eslint . --fix --quiet

lint-be: ## Run Go vet
	go vet ./...

audit-be: ## Run non-mutating golangci-lint audit
	golangci-lint run ./...

# ── Docker ────────────────────────────────────────────────

docker: ## Build multi-stage Docker image
	docker build \
		--build-arg VERSION="$(VERSION)" \
		--build-arg VCS_REF="$(VCS_REF)" \
		--build-arg BUILD_DATE="$(BUILD_DATE)" \
		-t "$(DOCKER_IMAGE)" .

docker-config: ## Validate the default SQLite Compose configuration
	docker compose -f compose.yaml config --quiet

docker-config-media: export MEDIA_PATH := $(CURDIR)
docker-config-media: ## Validate the default deployment with a read-only media mount
	docker compose -f compose.yaml -f compose.media.yaml config --quiet

docker-config-postgres: ## Validate the PostgreSQL Compose override
	docker compose -f compose.yaml -f compose.postgres.yaml config --quiet

docker-config-secrets: ## Validate the default deployment with the JWT secret override
	docker compose -f compose.yaml -f compose.secrets.yaml config --quiet

docker-config-musicbee-secrets: ## Validate SQLite with JWT and MusicBee secrets
	docker compose -f compose.yaml -f compose.secrets.yaml -f compose.musicbee-secrets.yaml config --quiet

docker-config-postgres-secrets: ## Validate PostgreSQL with JWT and database secrets
	docker compose -f compose.yaml -f compose.postgres.yaml -f compose.secrets.yaml -f compose.postgres-secrets.yaml config --quiet

docker-config-analyzer: ## Validate the optional HTTP analyzer profile
	docker compose -f compose.yaml -f compose.analyzer.yaml --profile analyzer config --quiet

docker-config-analyzer-secrets: ## Validate analyzer with a file-backed shared token
	docker compose -f compose.yaml -f compose.analyzer.yaml -f compose.analyzer-secrets.yaml --profile analyzer config --quiet

docker-up: ## Start the default SQLite Compose deployment
	docker compose -f compose.yaml up -d --build

docker-up-media: ## Start SQLite with a read-only media mount
	docker compose -f compose.yaml -f compose.media.yaml up -d --build

docker-up-postgres: ## Start the PostgreSQL Compose deployment
	docker compose -f compose.yaml -f compose.postgres.yaml up -d --build

docker-up-secrets: ## Start SQLite with JWT supplied as a Docker secret
	docker compose -f compose.yaml -f compose.secrets.yaml up -d --build

docker-up-musicbee-secrets: ## Start SQLite with JWT and MusicBee credentials supplied as Docker secrets
	docker compose -f compose.yaml -f compose.secrets.yaml -f compose.musicbee-secrets.yaml up -d --build

docker-up-postgres-secrets: ## Start PostgreSQL with JWT and database passwords supplied as Docker secrets
	docker compose -f compose.yaml -f compose.postgres.yaml -f compose.secrets.yaml -f compose.postgres-secrets.yaml up -d --build

docker-up-analyzer: ## Start SQLite with the optional HTTP analyzer profile
	docker compose -f compose.yaml -f compose.analyzer.yaml --profile analyzer up -d --build

docker-up-analyzer-secrets: ## Start analyzer with a file-backed shared token
	docker compose -f compose.yaml -f compose.analyzer.yaml -f compose.analyzer-secrets.yaml --profile analyzer up -d --build

docker-down: ## Stop Compose services without deleting persistent volumes
	docker compose -f compose.yaml down --remove-orphans

clean: ## Remove build artifacts
	rm -rf cmd/server/dist
	rm -f $(BINARY)
