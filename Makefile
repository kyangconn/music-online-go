.PHONY: help build build-fe build-be build-silent dev-fe dev-be test lint lint-fe lint-be docker clean

.DEFAULT_GOAL := help

FE_DIR := web

help: ## Show available commands
	@echo "Music Online Go - Available Commands"
	@echo ""
	@echo "=== Build ==="
	@echo "  make build          Build frontend + backend (production)"
	@echo "  make build-fe       Build frontend only"
	@echo "  make build-be       Build backend only (requires dist/)"
	@echo ""
	@echo "=== Develop ==="
	@echo "  make dev-fe         Start frontend dev server (hot reload)"
	@echo "  make dev-be         Start backend dev server"
	@echo ""
	@echo "=== Quality ==="
	@echo "  make lint           Run all linters (Go + ESLint)"
	@echo "  make lint-fe        Run ESLint on frontend"
	@echo "  make lint-be        Run Go vet"
	@echo "  make test           Run all tests (Go + frontend lint)"
	@echo ""
	@echo "=== Docker ==="
	@echo "  make docker         Build Docker image"
	@echo "  make clean          Remove build artifacts"

build: build-fe build-be ## Build frontend then backend

build-fe: ## Build Vue frontend, output to cmd/server/dist/
	cd $(FE_DIR) && pnpm install --frozen-lockfile && pnpm build

build-silent: build-fe-silent build-be ## Build frontend (silent) then backend

build-fe-silent: ## Build Vue frontend silently (no progress output)
	cd $(FE_DIR) && pnpm install --frozen-lockfile && pnpm --silent build

build-be: ## Build Go server binary
	go build -v -o music-server ./cmd/server

dev-fe: ## Start Vite dev server at localhost:5173
	cd $(FE_DIR) && pnpm dev

dev-be: ## Start Go server at localhost:8080
	go run ./cmd/server

test: test-be lint-fe ## Run backend tests + frontend lint

test-be: ## Run Go tests
	go test -v ./...

# ── Lint ──────────────────────────────────────────────────

lint: ## Run all linters (Go fmt + vet + ESLint)
	go fmt ./...
	go vet ./...
	cd $(FE_DIR) && pnpm lint --quiet --format compact

lint-fe: ## Run ESLint on frontend
	cd $(FE_DIR) && pnpm eslint . --fix --quiet --format compact

lint-be: ## Run Go vet
	go vet ./...

# ── Docker ────────────────────────────────────────────────

docker: ## Build multi-stage Docker image
	docker build -t music-online-go .

clean: ## Remove build artifacts
	rm -rf cmd/server/dist
	rm -f music-server music-server.exe
