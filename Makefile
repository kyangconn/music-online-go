.PHONY: help install-fe install-fe-dev build build-fe build-be build-silent dev dev-fe dev-be test test-be test-cover check verify check-fe check-be typecheck-fe lint lint-fe lint-be audit-be docker clean

.DEFAULT_GOAL := help

FE_DIR := web
GO_MODULE := $(shell go list -m)

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
	@echo "  make test           Run all tests (Go + frontend lint)"
	@echo "  make test-cover     Run Go tests with a coverage summary"
	@echo ""
	@echo "=== Docker ==="
	@echo "  make docker         Build Docker image"
	@echo "  make clean          Remove build artifacts"

install-fe: ## Install frontend dependencies from lockfile
	pnpm --dir $(FE_DIR) install --frozen-lockfile

install-fe-dev: ## Install/update frontend dependencies for local development
	pnpm --dir $(FE_DIR) install

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

test: test-be lint-fe ## Run backend tests + frontend lint

test-be: ## Run Go tests
	go test -v ./...

test-cover: ## Run Go tests and print statement coverage by function
	go test -coverpkg=$(GO_MODULE)/... -coverprofile=coverage.out ./...
	go tool cover -func coverage.out

check: check-be check-fe ## Run non-mutating checks

verify: test-be check ## Run tests + non-mutating checks

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
	docker build -t music-online-go .

clean: ## Remove build artifacts
	rm -rf cmd/server/dist
	rm -f music-server music-server.exe
