.PHONY: help build build-fe build-be dev-fe dev-be test lint lint-fe lint-be docker clean

.DEFAULT_GOAL := help

help: ## Show available commands
	@echo "Music Online Go - Available Commands"
	@echo ""
	@echo "  make build       Build frontend + backend (production)"
	@echo "  make build-fe    Build frontend only"
	@echo "  make build-be    Build backend only (requires dist/)"
	@echo "  make dev-fe      Start frontend dev server (hot reload)"
	@echo "  make dev-be      Start backend dev server"
	@echo "  make test        Run all tests"
	@echo "  make lint        Run all linters"
	@echo "  make docker      Build Docker image"
	@echo "  make clean       Remove build artifacts"

build: build-fe build-be ## Build frontend then backend

build-fe: ## Build Vue frontend, output to cmd/server/dist/
	cd web && pnpm install --frozen-lockfile && pnpm build

build-be: ## Build Go server binary
	go build -v -o music-server ./cmd/server

dev-fe: ## Start Vite dev server at localhost:5173
	cd web && pnpm dev

dev-be: ## Start Go server at localhost:8080
	go run ./cmd/server

test: test-be lint-fe ## Run backend tests + frontend lint

test-be: ## Run Go tests
	go test -v ./...

lint: lint-fe ## Run linters

lint-fe: ## Run ESLint on frontend
	cd web && pnpm eslint .

lint-be: ## Run Go vet
	go vet ./...

docker: ## Build multi-stage Docker image
	docker build -t music-online-go .

clean: ## Remove build artifacts
	rm -rf cmd/server/dist
	rm -f music-server music-server.exe
