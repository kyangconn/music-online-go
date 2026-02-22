# Music Online

## Introduction
A full-stack online music platform rewrite. The project aims to provide a self-hosted, maintainable platform for music enthusiasts to listen and manage their music collections.

- **Backend**: Go (Gin + GORM + PostgreSQL)
- **Frontend**: Vue 3 + Element Plus + Vite
- **Deployment**: Single binary distribution (Frontend embedded in Go binary)

## Getting Started

### 1. Development (Hot Reload)
**Backend:**

```bash
# Copy example config and update database config
cp config-example.yaml config.yaml

# Run backend server
go run cmd/server/main.go
```

**Frontend:**

```bash
cd web
npm install
npm run dev
```
Visit `http://localhost:5173`. API requests are proxied to port 8080.

### 3. Production Build

```bash
# 1. Build Frontend
cd web
npm run build

# 2. Build Backend (Embeds web/dist)
cd ../
go build -o music-server ./cmd/server
```

Run `./music-server`. Visit `http://localhost:8080`.

## Project Structure
- `cmd/server`: HTTP entry point, embeds static files, and routes.
- `internal/config`: Configuration loading.
- `internal/domain`: Core domain models (user, music, etc.).
- `internal/repository`: Database access layer (PostgreSQL + GORM).
- `internal/service`: Business logic (like, search, etc.).
- `internal/handler`: HTTP handlers (Gin).
- `web`: Frontend single-page application (Vue 3 + Element Plus).

## Submodules / Frontend Repository

Web frontend is a submodule as a separate repository.

