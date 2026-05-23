# Agent Guidelines for Music Online

This file contains **mandatory conventions** for AI agents (or any contributor) working on this project.
Read this before making any changes.

---

## Project Overview

A full-stack music management platform: **Go backend + Vue 3 frontend**, compiled into a single static binary.

- Backend: Go 1.26 + Gin + GORM (SQLite/PostgreSQL)
- Frontend: Vue 3 + TypeScript + Element Plus + Pinia + Vite

---

## Critical Conventions

### 1. Package Managers

| Context  | Tool                         | Lock File            |
| -------- | ---------------------------- | -------------------- |
| Frontend | **pnpm** (NOT npm, NOT yarn) | `web/pnpm-lock.yaml` |
| Backend  | Go modules (`go mod`)        | `go.sum`             |

**Never run `npm install`, `npx`, `yarn`** in the `web/` directory. Always use `pnpm`.

### 2. Use the Makefile

The `Makefile` at the project root provides the canonical shortcuts. **Always prefer these over raw `cd web && pnpm ...` commands.** The Makefile handles the working directory for you.

| Command             | What it does                                       |
| ------------------- | -------------------------------------------------- |
| `make dev-fe`       | Start frontend dev server (hot reload, port 5173)  |
| `make dev-be`       | Start Go backend (port matches config)             |
| `make build`        | Build frontend then backend (production)           |
| `make build-fe`     | Build Vue frontend → `cmd/server/dist/`            |
| `make build-silent` | Build frontend (silent) + backend                  |
| `make build-be`     | Compile Go binary (requires dist/ already present) |
| `make lint`         | Format + vet Go, then ESLint frontend              |
| `make lint-fe`      | Run full ESLint on frontend                        |
| `make lint-quiet`   | ESLint errors+warnings only, compact output        |
| `make lint-fix`     | Run ESLint with `--fix` (auto-fixable issues)      |
| `make typecheck`    | Run TypeScript type check (`vue-tsc --noEmit`)     |
| `make fmt-fe`       | Format frontend with Prettier                      |
| `make lint-be`      | Run `go vet ./...`                                 |
| `make test`         | Go tests + frontend lint                           |
| `make docker`       | Build multi-stage Docker image                     |

**Key principle**: If you find yourself typing `cd web && pnpm eslint ... --format json | node -e ...`, stop — the `make lint-quiet` and `make lint-rules` targets already do this.

When in doubt, run `make help` to see all available commands.

### 3. Frontend Specifics

- **Root**: `web/` is the frontend project root. All `pnpm` commands must be run from there.
- **ESLint**: `cd web && pnpm eslint .` — do not use `npx eslint`.
- **TypeScript**: `cd web && pnpm vue-tsc -b` for type checking.
- **Vite dev server** proxies `/api` to `http://localhost:8080` (configurable via `VITE_API_PROXY_TARGET` env var).
- **Build output** goes to `web/../cmd/server/dist/` (i.e., `cmd/server/dist/` from project root). This directory is `.gitignore`'d.
- **Path alias**: `@` maps to `web/src/` (configured in `vite.config.ts`).

### 4. Backend Specifics

- **Entry point**: `cmd/server/main.go`
- **Config**: Uses Viper. `config.yaml` is gitignored; use `config-example.yaml` as template.
  - `go run ./cmd/server` or `make dev-be`
  - Supports CLI flags: `--config-file`, `--log-file`
  - Environment variable overrides: `SERVER_PORT`, `DATABASE_TYPE`, etc.
- **Vendor directory** (`vendor/`) is gitignored. Use `go mod tidy` / `go mod download` as needed.

### 5. File/Directory Rules

- **Never delete** `cmd/server/dist/` unless told to (it contains the latest frontend build).
- **Do not commit** `config.yaml`, `*.exe`, or anything in `.gitignore`.
- **Do not modify** `config-example.yaml` without also updating documentation in `README.md`.

### 6. Testing & Linting

- Backend tests: `go test -v ./...`
- Frontend lint: `pnpm eslint .` from `web/`
- Run lint before considering any frontend task complete.
- If lint produces new errors after your changes, fix them.

### 7. Code Style

- **Go**: Standard `gofmt`. Run `go fmt ./...` before committing.
- **Vue/TS**: Follow the project's ESLint config (includes `eslint-plugin-vue`, `eslint-config-prettier`, `eslint-plugin-perfectionist`).
- Keep changes minimal and consistent with existing patterns.

### 7a. SCSS Architecture

This project uses **SCSS** with a global-injection pattern. All components can use tokens and mixins without imports.

**File structure:**

```
web/src/styles/
├── tokens/
│   ├── _colors.scss         # Color palette & semantic tokens
│   ├── _spacing.scss         # Spacing scale (xs → 4xl)
│   ├── _typography.scss      # Font families, sizes, weights
│   ├── _breakpoints.scss     # Responsive breakpoints (xs → xl)
│   ├── _borders.scss         # Border radii
│   └── _transitions.scss     # Transition durations
├── _shared.scss              # Forwards tokens + mixins (injected globally)
├── _mixins.scss              # Reusable mixins: respond-to, flex-center, text-ellipsis, card-hover, etc.
├── global.scss               # Global styles (replaces old style.css)
├── common.scss               # Utility classes (replaces old styles/common.css)
└── element-plus.scss         # Element Plus SCSS overrides (@forward ... with)
```

**How it works:**
- `vite.config.ts` → `additionalData` injects `@use "@/styles/shared" as *;` into every SCSS file.
- Components use `<style scoped lang="scss">` and get all tokens (`$color-primary`, `$spacing-md`, etc.) and mixins (`@include flex-center`, `@include mobile { ... }`) for free.
- **Never add `@use` / `@import` for tokens or mixins in component styles** — they're already injected.
- CSS custom properties (`var(--primary-color)`, `var(--text-light)`) remain for dark/light mode theming.

**Modern SCSS rules:**
- Use `@use` / `@forward` (never `@import` — deprecated).
- Use SCSS interpolation `#{$var}` for injecting SCSS variables into CSS custom properties.
- Nest selectors with `&` (e.g., `&:hover`, `&.active`).
- Use `@include mixin-name` for reusable patterns.

### 8. Important Nuances

- The frontend has **i18n** (Chinese + English). When adding UI text, use the i18n system (`web/src/i18n/`).
- Element Plus is the UI component library. Do not introduce other UI frameworks.
- The frontend uses **Pinia** for state management and **Vue Router** for routing.
- The project uses **RSA encryption** for sensitive fields and **JWT + OTP** for authentication — be cautious around auth-related code.
- **Docker** multi-stage build: frontend (Node) → backend (Go) → Alpine runtime. The `Dockerfile` expects `config.yaml` at the final stage.

---

## Quick Reference: Common Tasks

```bash
# Development
make dev-fe          # Start frontend dev server
make dev-be          # Start backend dev server

# Building
make build           # Full production build
make build-fe        # Frontend only

# Quality
make lint            # All linters
make test            # All tests

# Docker
make docker          # Build Docker image
```
