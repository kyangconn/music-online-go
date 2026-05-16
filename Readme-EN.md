# Music Online

Full-stack music management platform — Go backend + Vue 3 frontend, compiled into a single static binary.

## Technology Stack

**Backend**
- Go 1.26 + Gin framework
- GORM (SQLite / PostgreSQL)
- JWT authentication + OTP two-factor
- Prometheus metrics
- Rate limiting middleware
- RSA-encrypted sensitive fields

**Frontend** (see [web/](./web/))
- Vue 3 + TypeScript
- Element Plus component library
- Pinia state management + Vue Router
- i18n (Chinese / English)
- Vite build tool

## Project Structure

```
.
├── cmd/server/          # Entry point, embeds frontend dist
├── internal/
│   ├── config/          # Configuration (viper)
│   ├── domain/          # Domain models
│   ├── handler/         # HTTP handlers
│   ├── middleware/       # Middleware (auth/ratelimit/logger)
│   ├── pkg/             # Utilities (database/jwt/password)
│   ├── repository/      # Data access layer
│   ├── router/          # Route registration
│   └── service/         # Business logic layer
├── web/                 # Vue 3 frontend (standalone subproject)
├── Dockerfile           # Multi-stage build
├── Makefile             # Build & dev shortcuts
├── go.mod
└── config-example.yaml  # Configuration template
```

## Quick Start

```bash
# 1. Copy and edit config file
cp config-example.yaml config.yaml

# 2. Build (frontend + backend)
make build

# 3. Run
./music-server
```

Visit `http://localhost:8080`.

## Development

```bash
# Start both dev servers
make dev-be    # Backend :8080
make dev-fe    # Frontend :5173 (proxies /api to backend)
```

The frontend dev server proxies `/api` requests to the backend automatically.

## Build

```bash
make build      # Build frontend then backend
make build-fe   # Frontend only
make build-be   # Backend only (requires dist/)
```

Frontend output goes to `cmd/server/dist/` and is embedded into the Go binary via `embed`. Deploy a single file.

## Test & Lint

```bash
make test       # Go tests + frontend ESLint
make lint       # Frontend ESLint
make lint-be    # Go vet
```

## Docker

```bash
make docker
```

Multi-stage: Node build → Go build → Alpine runtime.

## Configuration

See [config-example.yaml](./config-example.yaml). Environment variable overrides are supported.

| Key | Description | Default |
|---|---|---|
| `server.port` | Listen port | `8080` |
| `server.mode` | Run mode | `debug` |
| `database.type` | Database type | `sqlite` |
| `jwt.secret` | JWT signing key | - |

## License

MIT
