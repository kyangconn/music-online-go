# Music Online

## Introduction
A full-stack online music platform rewrite.
- **Backend**: Go (Gin + GORM + PostgreSQL)
- **Frontend**: Vue 3 + Element Plus + Vite
- **Deployment**: Single binary distribution (Frontend embedded in Go binary)

## Tech Stack
- **Language**: Go 1.24+, TypeScript
- **Web Framework**: [Gin](https://github.com/gin-gonic/gin) (Go), Vue 3 (JS)
- **Database**: PostgreSQL
- **Build Tool**: Vite

## Getting Started

### 1. Prerequisites
- Go 1.24+
- Node.js & npm
- PostgreSQL

### 2. Development (Hot Reload)
Run backend and frontend in separate terminals.

**Backend:**
```bash
cp config-example.yaml config.yaml
# Update database config in config.yaml
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
Build a single executable file containing the frontend.

```bash
# 1. Build Frontend
cd web
npm run build

# 2. Build Backend (Embeds web/dist)
cd ../
go build -o music-server ./cmd/server
```

Run `./music-server`. Visit `http://localhost:8080`.