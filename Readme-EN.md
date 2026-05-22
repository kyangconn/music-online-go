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

**Frontend** (see [web](./web))

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

Configuration uses [Viper](https://github.com/spf13/viper) with three-layer overrides: **YAML file → env vars → CLI
flags**.

### CLI Flags

```bash
./music-server --config-file /path/to/config.yaml --log-file /var/log/music.log
```

| Flag            | Description                                            |
|-----------------|--------------------------------------------------------|
| `--config-file` | Explicit path to config YAML (bypasses auto-discovery) |
| `--log-file`    | Log file path (overrides `server.log_file` in config)  |

### Config File Search Order

On startup, `config.yaml` is searched in the following order — **later paths take higher priority**:

| Priority    | Path                                                              | Use Case                       |
|-------------|-------------------------------------------------------------------|--------------------------------|
| 1 (lowest)  | `./`, `./config/`                                                 | Development, current directory |
| 2           | `../`, `../../`                                                   | Running from `cmd/server`      |
| 3           | `$XDG_CONFIG_HOME/music-online/` or `$HOME/.config/music-online/` | XDG user config                |
| 4           | `/etc/music-online/`                                              | System-wide config             |
| 5           | `/data/`                                                          | Docker volume mount            |
| 6 (highest) | Binary directory                                                  | Production deployment          |

If `MO_CONFIG_FILE` env var or `--config-file` is set, that path is used directly and all search is skipped.

If no config file is found, the program starts normally with all defaults.

### Environment Variables

Viper's `AutomaticEnv` is enabled — every YAML key is available as an env var by replacing `.` with `_`:

```bash
SERVER_PORT=8080 DATABASE_TYPE=sqlite DATABASE_PATH=/data/music.db ./music-server
```

Dedicated environment variables:

| Env Var           | Description                                                                |
|-------------------|----------------------------------------------------------------------------|
| `MO_CONFIG_FILE`  | Explicit config file path (equivalent to `--config-file`)                  |
| `MO_LOG_FILE`     | Log file path (priority: `--log-file` > `MO_LOG_FILE` > `server.log_file`) |
| `XDG_CONFIG_HOME` | XDG base directory for user-level config search                            |
| `HOSTNAME`        | Hostname shown in admin panel (fallback)                                   |

### Complete YAML Reference

See [config-example.yaml](./config-example.yaml).

#### server

| Key                    | Type   | Default     | Description                         |
|------------------------|--------|-------------|-------------------------------------|
| `server.port`          | string | `"3060"`    | Listen port                         |
| `server.mode`          | string | `"debug"`   | Run mode: `debug` / `release`       |
| `server.read_timeout`  | int    | `30`        | HTTP read timeout (seconds)         |
| `server.write_timeout` | int    | `30`        | HTTP write timeout (seconds)        |
| `server.upload_dir`    | string | `"uploads"` | Upload file storage directory       |
| `server.log_file`      | string | `""`        | Log file path (empty = stdout only) |

#### database

| Key                 | Type   | Default      | Description                                         |
|---------------------|--------|--------------|-----------------------------------------------------|
| `database.type`     | string | `"postgres"` | Database type: `sqlite` / `postgres`                |
| `database.host`     | string | —            | PostgreSQL host (postgres only)                     |
| `database.port`     | string | —            | PostgreSQL port (postgres only)                     |
| `database.user`     | string | —            | PostgreSQL user (postgres only)                     |
| `database.password` | string | —            | PostgreSQL password (postgres only)                 |
| `database.name`     | string | —            | PostgreSQL database name (postgres only)            |
| `database.sslmode`  | string | `"disable"`  | PostgreSQL SSL mode (postgres only)                 |
| `database.path`     | string | `"music.db"` | SQLite file path (sqlite only, supports `:memory:`) |

Env var examples:

```bash
# SQLite
DATABASE_TYPE=sqlite DATABASE_PATH=:memory: ./music-server

# PostgreSQL
DATABASE_HOST=localhost DATABASE_PORT=5432 DATABASE_USER=postgres DATABASE_PASSWORD=postgres DATABASE_NAME=music-online ./music-server
```

#### jwt

| Key               | Type   | Default | Description                                     |
|-------------------|--------|---------|-------------------------------------------------|
| `jwt.secret`      | string | —       | JWT signing key (must be changed in production) |
| `jwt.expire_hour` | int    | `24`    | JWT expiration time (hours)                     |

#### security

| Key                      | Type   | Default | Description                   |
|--------------------------|--------|---------|-------------------------------|
| `security.password_salt` | string | —       | Password hash salt (reserved) |

### Logging

Logs go to **stdout + file** (if `log_file` is configured). The file is opened in append mode; failure to open only
prints a warning — the program continues to run.

```bash
# stdout only
./music-server

# stdout + file
./music-server --log-file /var/log/music.log
```

## License

MIT
