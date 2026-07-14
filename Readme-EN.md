# Music Online

Full-stack music management platform — Go backend + Vue 3 frontend, compiled into a single static binary.

## Technology Stack

**Backend**

- Go 1.26 + Gin framework
- GORM (SQLite / PostgreSQL)
- JWT authentication + OTP two-factor
- Optional Prometheus metrics protected by a bearer token
- Rate limiting middleware
- bcrypt password hashing

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
# Optional: enable admin.bootstrap in config.yaml to create the first admin

# 2. Build (frontend + backend)
make build

# 3. Run
./music-server
```

Visit `http://localhost:8080`.

## Development

```bash
# Install/update frontend dependencies from the repo root
make install-fe      # Install from pnpm-lock
make install-fe-dev  # Install/update dependencies for development

# Start dev servers
make dev       # Backend + frontend
make dev-be    # Backend :8080
make dev-fe    # Frontend :5173 (proxies /api to backend)
```

The frontend dev server proxies `/api` requests to the backend automatically.

## Build

```bash
make build      # Build frontend then backend
make build-silent # Quieter frontend build + backend build
make build-fe   # Frontend only
make build-be   # Backend only (requires dist/)
```

Frontend output goes to `cmd/server/dist/` and is embedded into the Go binary via `embed`. Deploy a single file.

## Test & Lint

```bash
make test       # Go tests + frontend ESLint
make check      # Non-mutating checks: Go vet + frontend typecheck/ESLint
make verify     # Go tests + check
make lint       # Mutating checks: Go fmt + Go vet + frontend ESLint --fix
make lint-fe    # Frontend ESLint --fix
make lint-be    # Go vet
```

## Docker

```bash
make docker
mkdir -p data
cp config-example.yaml data/config.yaml
docker run --rm -p 8080:8080 -v "$PWD/data:/data" music-online-go
```

Multi-stage: Node build → Go build → Alpine runtime. The image does not bake in `config.yaml`; provide configuration at runtime through `/data/config.yaml`, environment variables, or CLI flags. Container defaults store data in `/data/music.db` and `/data/uploads`, so mount `/data` for persistence.

PWA installation and the offline app shell work directly on `localhost`. When accessing a self-hosted instance from another device, expose it through an HTTPS reverse proxy; plain LAN HTTP addresses do not enable these secure-context capabilities.

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

If no config file is found, the program starts normally with all defaults. The default database is a SQLite file named `music.db`; PostgreSQL is used only when explicitly configured with host/user/name.

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

CORS and metrics settings can also be overridden with `SERVER_ALLOWED_ORIGINS`,
`METRICS_ENABLED`, and `METRICS_TOKEN`; use comma-separated origins in the environment variable.

The first admin can also be bootstrapped entirely from environment variables:

```bash
ADMIN_BOOTSTRAP_ENABLED=true \
ADMIN_BOOTSTRAP_USERNAME=admin \
ADMIN_BOOTSTRAP_EMAIL=admin@example.com \
ADMIN_BOOTSTRAP_PASSWORD='change-me-please' \
./music-server
```

### Complete YAML Reference

See [config-example.yaml](./config-example.yaml).

#### server

| Key                    | Type   | Default     | Description                         |
|------------------------|--------|-------------|-------------------------------------|
| `server.port`          | string | `"8080"`    | Listen port                         |
| `server.mode`          | string | `"debug"`   | Run mode: `debug` / `release`       |
| `server.read_timeout`  | int    | `30`        | HTTP read timeout (seconds)         |
| `server.write_timeout` | int    | `30`        | HTTP write timeout (seconds)        |
| `server.upload_dir`    | string | `"uploads"` | Upload file storage directory       |
| `server.log_file`      | string | `""`        | Log file path (empty = stdout only) |
| `server.max_audio_size_mb` | int | `200`       | Maximum size per audio file (MB)    |
| `server.max_cover_size_mb` | int | `10`        | Maximum size per cover image (MB)   |
| `server.allowed_origins` | string[] | `[]`      | Additional credentialed cross-origin API origins |

`server.upload_dir` may be absolute or relative. Relative paths are resolved from the server process working directory. Local `make dev-be` writes to `uploads/` under the repo root by default; Docker defaults to `SERVER_UPLOAD_DIR=/data/uploads` inside the mounted volume.

Uploads are validated by size, extension, request MIME, and file header signature. The frontend also pre-checks files to avoid wasted uploads, but backend validation is the security boundary.

Same-origin requests do not need to be listed. Configure `server.allowed_origins` only when the frontend is served from another origin, for example `["https://music-ui.example.com"]`. Wildcards and URLs with paths are rejected.

#### database

| Key                 | Type   | Default      | Description                                         |
|---------------------|--------|--------------|-----------------------------------------------------|
| `database.type`     | string | `"sqlite"`   | Database type: `sqlite` / `postgres`                |
| `database.host`     | string | —            | PostgreSQL host (postgres only)                     |
| `database.port`     | string | —            | PostgreSQL port (postgres only)                     |
| `database.user`     | string | —            | PostgreSQL user (postgres only)                     |
| `database.password` | string | —            | PostgreSQL password (postgres only)                 |
| `database.name`     | string | —            | PostgreSQL database name (postgres only)            |
| `database.sslmode`  | string | `"disable"`  | PostgreSQL SSL mode (postgres only)                 |
| `database.path`     | string | `"music.db"` | SQLite file path (sqlite only, supports `:memory:`) |

Env var examples:

```bash
# SQLite (default file database)
DATABASE_TYPE=sqlite DATABASE_PATH=music.db ./music-server

# SQLite (in-memory, tests only)
DATABASE_TYPE=sqlite DATABASE_PATH=:memory: ./music-server

# PostgreSQL
DATABASE_TYPE=postgres DATABASE_HOST=localhost DATABASE_PORT=5432 DATABASE_USER=postgres DATABASE_PASSWORD=postgres DATABASE_NAME=music-online ./music-server
```

#### jwt

| Key               | Type   | Default | Description                                     |
|-------------------|--------|---------|-------------------------------------------------|
| `jwt.secret`      | string | —       | JWT signing key (must be changed in production) |
| `jwt.expire_hour` | int    | `24`    | JWT expiration time (hours)                     |

Passwords are stored with bcrypt hashes; there is no RSA field-encryption configuration.

#### metrics

| Key               | Type   | Default | Description                                 |
|-------------------|--------|---------|---------------------------------------------|
| `metrics.enabled` | bool   | `false` | Expose `/metrics`                           |
| `metrics.token`   | string | `""`  | Bearer token used by the Prometheus scraper |

Metrics are disabled by default. Enabling them requires a non-empty token and requests must use
`Authorization: Bearer <token>`. `/health` reports process liveness; `/ready` verifies both the database and writable upload storage.

#### admin.bootstrap

First-admin bootstrap is explicit opt-in. By default, no admin account is created. When enabled, the app runs it after database migration: it creates the configured username if missing, or promotes and activates the existing username as admin.

| Key                              | Type    | Default           | Description                                                        |
|----------------------------------|---------|-------------------|--------------------------------------------------------------------|
| `admin.bootstrap.enabled`        | bool    | `false`           | Enable startup admin bootstrap                                     |
| `admin.bootstrap.username`       | string  | —                 | Admin username                                                     |
| `admin.bootstrap.email`          | string  | —                 | Email used when creating a new admin                               |
| `admin.bootstrap.password`       | string  | —                 | Admin password, at least 8 characters; never printed to logs       |
| `admin.bootstrap.full_name`      | string  | `"Administrator"` | Full name used for new admins or blank existing names              |
| `admin.bootstrap.reset_password` | bool    | `false`           | Reset password for an existing username using the configured value |

After the admin exists, disable `admin.bootstrap.enabled` and restart. If it remains enabled, the app only keeps that username as an active admin and does not create duplicates. In production, do not commit admin passwords; prefer env vars or deployment-only config.

### Logging

Logs go to **stdout + file** (if `log_file` is configured). The file is opened in append mode; failure to open only
prints a warning — the program continues to run.

```bash
# stdout only
./music-server

# stdout + file
./music-server --log-file /var/log/music.log
```

## Backup & Restore

For a SQLite single-node deployment, back up three things: `config.yaml`, the SQLite database file (`music.db` by default, or `database.path`), and the upload directory (`uploads/` by default, or `server.upload_dir`). Stop the service first when possible, or at least make sure no upload is running.

```bash
# Example: archive data from a current-directory deployment
tar -czf music-online-backup.tgz config.yaml music.db uploads/
```

To restore, stop the service, put those files/directories back at the same paths, then start the app. Docker deployments usually only need the mounted `data/` directory:

```bash
tar -czf music-online-data.tgz data/
```

For PostgreSQL deployments, back up both the database and uploaded files:

```bash
pg_dump "$DATABASE_URL" > music-online.sql
tar -czf music-online-files.tgz config.yaml uploads/
```

Restore PostgreSQL by importing the SQL dump, then restoring the upload directory and config file.

### Database Upgrades & Rollback

At startup, the service applies pending database migrations in version order and records them in the `schema_migrations` table. Each migration and its history record run in one transaction. A failure stops startup and is not marked as applied, so the migration can be retried after fixing the cause.

Before upgrading, back up the database, uploaded files, and configuration as described above. Automatic down migrations are not provided. To roll back the application, stop the service, restore the complete pre-upgrade backup, and then deploy the older binary. Do not point an older binary at a database that has already received newer migrations; it will refuse to start when it detects an unknown migration version.

## License

MIT
