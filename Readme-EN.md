# Music Online

Full-stack music management platform — Go backend + Vue 3 frontend, compiled into a single static binary.

## Technology Stack

**Backend**

- Go 1.26 + Gin framework
- GORM (SQLite / PostgreSQL)
- JWT authentication + OTP two-factor
- Optional Prometheus metrics protected by a bearer token
- Configurable global and authentication rate limits
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
├── cmd/server/                 # Entry point; embeds frontend dist
├── internal/
│   ├── config/                 # Configuration and validation
│   ├── domain/                 # Domain models
│   ├── handler/                # HTTP handlers
│   ├── middleware/             # Auth, rate-limit, logging middleware
│   ├── pkg/                    # Database, JWT, logging utilities
│   ├── repository/             # Data access layer
│   ├── router/                 # Route registration
│   └── service/                # Business logic layer
├── web/                        # Vue 3 frontend
├── Dockerfile                  # Multi-stage non-root image
├── compose.yaml                # SQLite deployment
├── compose.postgres.yaml       # Optional PostgreSQL override
├── config-example.yaml         # General configuration template
├── config-docker-example.yaml  # Container configuration template
└── Makefile                    # Canonical build and development commands
```

## Quick Start

```bash
# 1. Copy the template and replace jwt.secret with a strong random value
cp config-example.yaml config.yaml
openssl rand -hex 32

# Optional: enable admin.bootstrap in config.yaml to create the first admin

# 2. Build the frontend and backend
make build

# 3. Run
./music-online
```

On Windows, use `Copy-Item config-example.yaml config.yaml` and run `music-online.exe`. Visit `http://localhost:8080`.

## Development

```bash
# Install/update frontend dependencies from the repo root
make install-fe      # Install exactly from pnpm-lock.yaml
make install-fe-dev  # Install/update dependencies for development

# Start development servers
make dev       # Backend + frontend
make dev-be    # Backend :8080
make dev-fe    # Frontend :5173 (proxies /api to the backend)
```

The frontend development proxy defaults to `http://localhost:8080`. Set `VITE_API_PROXY_TARGET` in `web/.env` to change it.

## Build

```bash
make build         # Build frontend, then backend
make build-silent  # Quieter frontend build, then backend
make build-fe      # Frontend only
make build-be      # Backend only (requires cmd/server/dist/)
```

Frontend output goes to `cmd/server/dist/` and is embedded into the Go binary.

## Test & Lint

```bash
make test       # Backend and frontend tests
make check      # Non-mutating Go vet, frontend typecheck, and ESLint
make verify     # Tests plus all non-mutating checks
make lint       # Go fmt/vet plus frontend ESLint fixes
make lint-fe    # Frontend ESLint fixes
make lint-be    # Go vet
make audit-be   # Non-mutating golangci-lint audit
```

## Docker

### SQLite Compose quick start

The default Compose deployment uses SQLite and is the simplest production-style setup:

```bash
cp .env.example .env                 # PowerShell: Copy-Item .env.example .env
openssl rand -hex 32                 # Paste the result into JWT_SECRET in .env
make docker-config
make docker-up
docker compose ps
```

Open `http://127.0.0.1:8080`. Use `docker compose logs -f app` to follow logs and `make docker-down` to stop the deployment without deleting persistent volumes.

The default Compose contract is intentionally explicit:

- `config-docker-example.yaml` is bind-mounted read-only at `/etc/music-online/config.yaml`.
- A named volume persists the SQLite database and uploads under `/data`.
- `JWT_SECRET` is injected at runtime; it is not baked into the image or example YAML.
- The application runs as UID/GID `10001`, with a read-only root filesystem, all Linux capabilities dropped, `no-new-privileges`, and an isolated `/tmp` tmpfs.
- The `/ready` health check verifies both database access and writable upload storage.
- Graceful shutdown gets 30 seconds by default, while the application uses its configured shutdown timeout.

If `DATA_PATH` or `POSTGRES_DATA_PATH` is changed from a named volume to a host directory, create it first and grant container UID `10001` write access. Otherwise SQLite, uploads, or PostgreSQL startup will fail rather than weakening the container user.

`.env` is ignored by Git. Treat it as deployment-only secret material: do not commit it, paste it into tickets, or reuse example passwords. For orchestrated production deployments, prefer the platform's protected secret injection mechanism.

The published port binds to `127.0.0.1` by default. Set `APP_BIND_ADDRESS=0.0.0.0` only when direct remote access is intentional, and normally place the service behind an HTTPS reverse proxy. PWA installation and the offline app shell work on `localhost`; other devices require HTTPS for secure-context browser features.

### PostgreSQL Compose override

Set at least `JWT_SECRET` and `POSTGRES_PASSWORD` in `.env`, then validate and start both files:

```bash
make docker-config-postgres
make docker-up-postgres
docker compose -f compose.yaml -f compose.postgres.yaml ps
```

The override starts PostgreSQL on the internal Compose network, waits for its health check, and switches the application to PostgreSQL. PostgreSQL is not published to the host by default. An incomplete PostgreSQL configuration now fails startup; the application never silently falls back to SQLite.

The default PostgreSQL 18 volume mounts at `/var/lib/postgresql`, allowing the image to create its major-version-specific data directory. Do not change `POSTGRES_IMAGE` across major versions while reusing the volume without a supported `pg_upgrade` or dump/restore migration.

### Compose secrets

For production-style local deployments, use file-backed Compose secrets instead of exposing plaintext values in the container environment:

```bash
mkdir -p secrets
openssl rand -hex 32 > secrets/jwt_secret
make docker-config-secrets
make docker-up-secrets

# Add an independent database password for PostgreSQL
openssl rand -hex 24 > secrets/postgres_password
make docker-config-postgres-secrets
make docker-up-postgres-secrets
```

On PowerShell, create the directory with `New-Item -ItemType Directory -Force secrets` and write the generated value with `Set-Content -NoNewline`. The ignored `secrets/` directory is never part of the image build context. `compose.secrets.yaml` mounts the JWT file; the PostgreSQL secrets override mounts one database-password file into both the application and the official PostgreSQL image.

### Docker Make targets

| Target | Purpose |
|---|---|
| `make docker` | Build the multi-stage image; override `DOCKER_IMAGE`, `VERSION`, `VCS_REF`, or `BUILD_DATE` as needed |
| `make docker-config` | Resolve and validate the SQLite Compose model without starting it |
| `make docker-config-postgres` | Resolve and validate the base model plus PostgreSQL override |
| `make docker-config-secrets` | Validate SQLite plus the JWT secret-file override |
| `make docker-config-postgres-secrets` | Validate PostgreSQL plus JWT and database secret-file overrides |
| `make docker-up` | Build and start the SQLite deployment |
| `make docker-up-postgres` | Build and start the PostgreSQL deployment |
| `make docker-up-secrets` | Build and start SQLite with a file-backed JWT secret |
| `make docker-up-postgres-secrets` | Build and start PostgreSQL with file-backed JWT/database secrets |
| `make docker-down` | Stop either deployment and remove orphan containers while preserving volumes |

### Compose customization

Copy `.env.example` instead of editing Compose YAML for routine deployment changes. Application settings listed under [Configuration reference](#configuration-reference) can also be set there.

| Variable | Default | Purpose |
|---|---:|---|
| `COMPOSE_PROJECT_NAME` | `music-online` | Compose project name |
| `MUSIC_ONLINE_IMAGE` | `music-online-go:local` | Image name/tag used by Compose |
| `APP_BIND_ADDRESS` | `127.0.0.1` | Host address for the published HTTP port |
| `APP_PORT` | `8080` | Published host port |
| `APP_CONTAINER_PORT` | `8080` | Container-port fallback; Compose passes it to the app as `SERVER_PORT` |
| `CONFIG_FILE` | `./config-docker-example.yaml` | Existing host config file to bind; Compose will not create a missing path |
| `MO_CONFIG_FILE` | `/etc/music-online/config.yaml` | Config path inside the container and explicit application config path |
| `DATA_PATH` | `music-online-data` | `/data` source; keep this value for the declared named volume, or use a host path such as `./data` |
| `DATA_VOLUME_NAME` | `music-online-data` | Engine-level name of the default application data volume |
| `RESTART_POLICY` | `unless-stopped` | Restart policy for application and PostgreSQL services |
| `READ_ONLY_ROOTFS` | `true` | Make the application root filesystem read-only; disabling is not recommended |
| `LOG_DRIVER` | `local` | Docker stdout/stderr logging driver for both services |
| `LOG_MAX_SIZE` / `LOG_MAX_FILE` | `10m` / `3` | Docker log rotation limits; separate from application file logging |
| `TMPFS_SIZE` | `256m` | Size of the application `/tmp` tmpfs used while parsing multipart uploads; raise it with upload limits |
| `STOP_GRACE_PERIOD` | `30s` | Application container stop grace period |
| `HEALTHCHECK_INTERVAL` | `30s` | Application health-check interval |
| `HEALTHCHECK_TIMEOUT` | `5s` | Application health-check timeout |
| `HEALTHCHECK_START_PERIOD` | `15s` | Application startup grace period |
| `HEALTHCHECK_RETRIES` | `3` | Failed checks before the application is unhealthy |
| `VERSION` | `dev` | Image/application version embedded at build time |
| `VCS_REF` | `unknown` | Source revision embedded at build time |
| `BUILD_DATE` | `1970-01-01T00:00:00Z` | OCI image/application build timestamp |
| `POSTGRES_IMAGE` | `postgres:18-alpine3.23` | PostgreSQL image used by the override |
| `POSTGRES_USER` | `music_online` | PostgreSQL role and application database user |
| `POSTGRES_PASSWORD` | required unless file-backed | PostgreSQL role password |
| `POSTGRES_PASSWORD_FILE` | `""` | Password-file path inside the PostgreSQL container; the secrets override sets `/run/secrets/postgres_password` |
| `POSTGRES_DB` | `music_online` | PostgreSQL database name |
| `POSTGRES_PORT` | `5432` | PostgreSQL port on the internal Compose network |
| `POSTGRES_DATA_PATH` | `music-online-postgres-data` | PostgreSQL data source; keep this for the declared volume, or use a host path |
| `POSTGRES_DATA_VOLUME_NAME` | `music-online-postgres-data` | Engine-level name of the default PostgreSQL volume |
| `POSTGRES_STOP_GRACE_PERIOD` | `30s` | PostgreSQL stop grace period |
| `POSTGRES_HEALTHCHECK_INTERVAL` | `10s` | PostgreSQL health-check interval |
| `POSTGRES_HEALTHCHECK_TIMEOUT` | `5s` | PostgreSQL health-check timeout |
| `POSTGRES_HEALTHCHECK_START_PERIOD` | `10s` | PostgreSQL startup grace period |
| `POSTGRES_HEALTHCHECK_RETRIES` | `5` | Failed checks before PostgreSQL is unhealthy |
| `JWT_SECRET_HOST_FILE` | `./secrets/jwt_secret` | Host file consumed by `compose.secrets.yaml` |
| `POSTGRES_PASSWORD_HOST_FILE` | `./secrets/postgres_password` | Host file consumed by `compose.postgres-secrets.yaml` |

`DOCKER_IMAGE` is a Make variable used by `make docker`; `MUSIC_ONLINE_IMAGE` is the corresponding Compose setting used by `make docker-up*`.

Compose always passes the resolved `SERVER_PORT`/`APP_CONTAINER_PORT` into the application, so the listener, port mapping, and health check cannot drift from a different YAML port. Other unset application variables do not override the read-only YAML. Startup requires a valid JWT secret from `JWT_SECRET`, `JWT_SECRET_FILE`, or YAML.

## Configuration

Configuration uses [Viper](https://github.com/spf13/viper). It reads one YAML file, applies environment overrides, then validates the complete result before opening the database or listening on a port.

### Precedence and CLI flags

General values use this order: **environment variable > selected YAML file > built-in default**. Compose first interpolates `.env`, then passes the resulting application environment into the container. Four sensitive values also accept `*_FILE`; either a direct environment value or a file value overrides YAML, and setting both forms is rejected as ambiguous.

Only two CLI flags exist; each takes priority by setting its dedicated environment variable before configuration is loaded:

| Flag | Effective precedence | Description |
|---|---|---|
| `--config-file PATH` | flag > `MO_CONFIG_FILE` > auto-discovery | Use exactly one explicit YAML file |
| `--log-file PATH` | flag > `MO_LOG_FILE` > `SERVER_LOG_FILE`/YAML > default | Enable stdout plus rotating file logs |

```bash
./music-online --config-file /path/to/config.yaml --log-file /var/log/music-online.log
```

There are no generic CLI flags for individual YAML keys; use environment variables for those overrides.

### Config file search order

If no explicit config path is set, Viper checks the following paths from highest to lowest priority and reads the **first** `config.yaml` it finds. Files are not merged.

| Order | Path | Typical use |
|---:|---|---|
| 1 | Directory containing the executable | Side-by-side binary deployment |
| 2 | `/data/` | Legacy/container data mount |
| 3 | `/etc/music-online/` | System or read-only container config |
| 4 | `$XDG_CONFIG_HOME/music-online/` | XDG user config |
| 5 | `$HOME/.config/music-online/` when `XDG_CONFIG_HOME` is unset | User config fallback |
| 6 | `../` | Running below the repository root |
| 7 | `../../` | Running from `cmd/server` |
| 8 | `./` | Current-directory deployment |
| 9 | `./config/` | Current-directory config subfolder |

`MO_CONFIG_FILE` or `--config-file` bypasses discovery. A missing explicit file or malformed YAML is a startup error.

No YAML file is required structurally, but `jwt.secret` has no default and is always required. Therefore a default-only startup fails unless `JWT_SECRET` or `JWT_SECRET_FILE` is supplied. The documented development placeholder is accepted only in `debug` mode; `release` and `test` modes require a different strong secret.

### Environment variable mapping

Every YAML key maps to an uppercase environment variable by replacing `.` with `_`. List values such as origins and proxies are comma-separated:

```bash
JWT_SECRET='replace-with-a-strong-random-secret' \
SERVER_ALLOWED_ORIGINS='https://music.example.com,https://admin.example.com' \
DATABASE_TYPE=sqlite \
DATABASE_PATH=/data/music.db \
./music-online
```

Never commit production JWT, metrics, database, or bootstrap-admin secrets. Environment variables are convenient, but may be visible through process/container inspection; use protected runtime secret injection or a permission-restricted, read-only config file where appropriate.

## Configuration reference

The complete annotated YAML template is [config-example.yaml](./config-example.yaml). The tables below list every YAML key and its corresponding environment variable.

### Server

| YAML key | Environment variable | Type | Default | Description |
|---|---|---|---:|---|
| `server.listen_address` | `SERVER_LISTEN_ADDRESS` | string | `""` | Bind address; empty means all interfaces |
| `server.port` | `SERVER_PORT` | string | `"8080"` | HTTP listen port, `1`–`65535` |
| `server.mode` | `SERVER_MODE` | string | `"debug"` | Gin mode: `debug`, `release`, or `test` |
| `server.read_header_timeout` | `SERVER_READ_HEADER_TIMEOUT` | int | `10` | Maximum seconds to read request headers; `0` disables it |
| `server.read_timeout` | `SERVER_READ_TIMEOUT` | int | `0` | Maximum seconds to read the complete request; `0` disables it |
| `server.write_timeout` | `SERVER_WRITE_TIMEOUT` | int | `0` | Maximum seconds to write the complete response; `0` disables it |
| `server.idle_timeout` | `SERVER_IDLE_TIMEOUT` | int | `60` | Keep-alive idle timeout in seconds; `0` falls back to the read timeout, then no limit |
| `server.shutdown_timeout` | `SERVER_SHUTDOWN_TIMEOUT` | int | `15` | Graceful HTTP shutdown deadline in seconds; must be positive |
| `server.readiness_timeout` | `SERVER_READINESS_TIMEOUT` | int | `2` | Database readiness probe deadline in seconds; must be positive |
| `server.upload_dir` | `SERVER_UPLOAD_DIR` | string | `"uploads"` | Upload storage path; relative to the process working directory |
| `server.log_file` | `SERVER_LOG_FILE` | string | `""` | Rotating log file; empty means stdout only |
| `server.max_json_body_size_mb` | `SERVER_MAX_JSON_BODY_SIZE_MB` | int | `1` | Hard JSON request-body limit in MiB |
| `server.max_audio_size_mb` | `SERVER_MAX_AUDIO_SIZE_MB` | int | `200` | Maximum audio file size in MiB |
| `server.max_cover_size_mb` | `SERVER_MAX_COVER_SIZE_MB` | int | `10` | Maximum cover image size in MiB |
| `server.allowed_origins` | `SERVER_ALLOWED_ORIGINS` | string[] | `[]` | Additional credentialed CORS origins |
| `server.trusted_proxies` | `SERVER_TRUSTED_PROXIES` | string[] | `[]` | Direct reverse-proxy IP addresses or CIDRs trusted to supply forwarding headers |

`read_timeout` and `write_timeout` default to `0` deliberately: a blanket 30-second deadline would abort slow large uploads and long media streams. Keep the bounded header and idle timeouts, and set whole-request timeouts only after accounting for the maximum upload size and the slowest supported client.

Same-origin requests do not need a CORS entry. Additional origins must be exact `http(s)://host[:port]` origins; wildcards, credentials, paths, queries, and fragments are rejected.

The service trusts no proxy headers by default. Add only the IP/CIDR of the reverse proxy that connects directly to the application. Do not use `0.0.0.0/0` or `::/0` merely to make forwarded addresses work: rate limiting and client-IP logs rely on this boundary.

JSON bodies are buffered only up to `max_json_body_size_mb` before handlers run; known-length and chunked oversized requests return 413. Uploads are checked by request size, declared limit, extension, request MIME, and file signature. The frontend pre-check is only a convenience; backend validation is the security boundary. Compose provides a `256m` `/tmp` tmpfs, enough for roughly one multipart request near the default limits. For concurrent uploads, size `TMPFS_SIZE` to about `(audio limit + cover limit + 1 MiB overhead) × expected concurrency`. A reverse proxy must allow the same combined request size and keep upload/response timeouts long enough for the slowest supported client, or it will reject the request before the application sees it.

### Database

| YAML key | Environment variable | Type | Default | Description |
|---|---|---|---:|---|
| `database.type` | `DATABASE_TYPE` | string | `"sqlite"` | `sqlite` or `postgres` |
| `database.path` | `DATABASE_PATH` | string | `"music.db"` | SQLite path; `:memory:` is intended for tests |
| `database.host` | `DATABASE_HOST` | string | `""` | PostgreSQL host or IP |
| `database.port` | `DATABASE_PORT` | string | `"5432"` | PostgreSQL port |
| `database.user` | `DATABASE_USER` | string | `""` | PostgreSQL user |
| `database.password` | `DATABASE_PASSWORD` | string | `""` | PostgreSQL password |
| `database.name` | `DATABASE_NAME` | string | `""` | PostgreSQL database name |
| `database.sslmode` | `DATABASE_SSLMODE` | string | `"prefer"` | `disable`, `allow`, `prefer`, `require`, `verify-ca`, or `verify-full` |
| `database.log_level` | `DATABASE_LOG_LEVEL` | string | `"auto"` | GORM SQL logging: `auto`, `silent`, `error`, `warn`, or `info` |
| `database.connect_timeout_seconds` | `DATABASE_CONNECT_TIMEOUT_SECONDS` | int | `10` | PostgreSQL connection timeout; must be positive |
| `database.max_open_connections` | `DATABASE_MAX_OPEN_CONNECTIONS` | int | `0` | Maximum open connections; `0` selects a database-specific default |
| `database.max_idle_connections` | `DATABASE_MAX_IDLE_CONNECTIONS` | int | `0` | Maximum idle connections; `0` selects a database-specific default |
| `database.connection_max_lifetime_minutes` | `DATABASE_CONNECTION_MAX_LIFETIME_MINUTES` | int | `60` | Maximum connection lifetime; `0` disables age-based expiry |
| `database.connection_max_idle_time_minutes` | `DATABASE_CONNECTION_MAX_IDLE_TIME_MINUTES` | int | `10` | Maximum connection idle time; `0` disables idle-time expiry |

Automatic pool limits are `1` open / `1` idle for SQLite and `25` open / `5` idle for PostgreSQL. Set explicit limits according to database capacity and replica count. When both are explicit, idle connections cannot exceed open connections.

PostgreSQL requires non-empty `host`, `user`, and `name`, plus a valid port and SSL mode. Invalid or incomplete PostgreSQL settings fail fast instead of writing to an unexpected SQLite file. The Compose override uses `sslmode=disable` only for its isolated internal network; prefer certificate verification (`verify-full` where supported by the deployment) for external PostgreSQL. SQL logging always keeps placeholders and omits bound parameters so that passwords, TOTP secrets, and similar values are not written to logs.

### JWT

| YAML key | Environment variable | Type | Default | Description |
|---|---|---|---:|---|
| `jwt.secret` | `JWT_SECRET` | string | none | Required JWT signing secret of at least 32 UTF-8 bytes; release/test also reject the example placeholder |
| `jwt.expire_hour` | `JWT_EXPIRE_HOUR` | int | `24` | Token lifetime in hours; must be positive |

Passwords are stored with bcrypt hashes. Every new password must contain at least 8 Unicode code points and at most 72 UTF-8 bytes, matching bcrypt's input limit. Refresh-token support is not currently part of this configuration contract.

### Metrics

| YAML key | Environment variable | Type | Default | Description |
|---|---|---|---:|---|
| `metrics.enabled` | `METRICS_ENABLED` | bool | `false` | Expose `/metrics` |
| `metrics.token` | `METRICS_TOKEN` | string | `""` | Required bearer token when metrics are enabled |

Requests to enabled metrics must send `Authorization: Bearer <token>`. `/health` reports process liveness; `/ready` verifies database access and writable upload storage.

### Admin bootstrap

| YAML key | Environment variable | Type | Default | Description |
|---|---|---|---:|---|
| `admin.bootstrap.enabled` | `ADMIN_BOOTSTRAP_ENABLED` | bool | `false` | Opt in to first-admin bootstrap during startup |
| `admin.bootstrap.username` | `ADMIN_BOOTSTRAP_USERNAME` | string | `""` | Admin username |
| `admin.bootstrap.email` | `ADMIN_BOOTSTRAP_EMAIL` | string | `""` | Email used when creating the admin |
| `admin.bootstrap.password` | `ADMIN_BOOTSTRAP_PASSWORD` | string | `""` | Admin password; at least 8 Unicode code points and at most 72 UTF-8 bytes |
| `admin.bootstrap.full_name` | `ADMIN_BOOTSTRAP_FULL_NAME` | string | `"Administrator"` | Full name for new admins or blank existing names |
| `admin.bootstrap.reset_password` | `ADMIN_BOOTSTRAP_RESET_PASSWORD` | bool | `false` | Reset an existing matching user's password |

When enabled, startup creates the username if it is absent or promotes and activates the existing username. After the admin exists, disable bootstrap and remove its password from the runtime environment.

### Rate limiting

| YAML key | Environment variable | Type | Default | Description |
|---|---|---|---:|---|
| `rate_limit.enabled` | `RATE_LIMIT_ENABLED` | bool | `true` | Enable global and authentication rate limiters |
| `rate_limit.global_requests_per_second` | `RATE_LIMIT_GLOBAL_REQUESTS_PER_SECOND` | float | `20` | Refill rate for the per-client global bucket |
| `rate_limit.global_burst` | `RATE_LIMIT_GLOBAL_BURST` | int | `50` | Global bucket burst capacity |
| `rate_limit.auth_requests_per_second` | `RATE_LIMIT_AUTH_REQUESTS_PER_SECOND` | float | `1` | Refill rate for login/register endpoints |
| `rate_limit.auth_burst` | `RATE_LIMIT_AUTH_BURST` | int | `5` | Authentication bucket burst capacity |

All four numeric limits must be positive when rate limiting is enabled. Trusted-proxy configuration determines which forwarded client addresses may be used safely.

### Logging

| YAML key | Environment variable | Type | Default | Description |
|---|---|---|---:|---|
| `logging.max_size_mb` | `LOGGING_MAX_SIZE_MB` | int | `50` | Rotate a file after it reaches this size; must be positive |
| `logging.level` | `LOGGING_LEVEL` | string | `"info"` | Minimum application log level: `debug`, `info`, `warn`, or `error` |
| `logging.access_log` | `LOGGING_ACCESS_LOG` | bool | `true` | Emit one access-log entry per HTTP request |
| `logging.max_backups` | `LOGGING_MAX_BACKUPS` | int | `3` | Retained rotated files; `0` means no count limit |
| `logging.max_age_days` | `LOGGING_MAX_AGE_DAYS` | int | `28` | Retention age; `0` means no age limit |
| `logging.compress` | `LOGGING_COMPRESS` | bool | `true` | Gzip rotated files |
| `logging.local_time` | `LOGGING_LOCAL_TIME` | bool | `true` | Use local time in rotated backup filenames |

Logs always go to stdout. If `server.log_file`, `MO_LOG_FILE`, or `--log-file` is configured, logs also go to a rotating file.

### Special and tooling environment variables

| Variable | Scope | Description |
|---|---|---|
| `MO_CONFIG_FILE` | Backend | Explicit YAML path; bypasses config discovery |
| `JWT_SECRET_FILE` | Backend | JWT signing-secret file; trailing newlines are removed; mutually exclusive with `JWT_SECRET` |
| `DATABASE_PASSWORD_FILE` | Backend | Database-password file; mutually exclusive with `DATABASE_PASSWORD` |
| `METRICS_TOKEN_FILE` | Backend | Metrics bearer-token file; mutually exclusive with `METRICS_TOKEN` |
| `ADMIN_BOOTSTRAP_PASSWORD_FILE` | Backend | Bootstrap-admin password file; mutually exclusive with `ADMIN_BOOTSTRAP_PASSWORD` |
| `MO_LOG_FILE` | Backend | File-log path; overrides `SERVER_LOG_FILE` and `server.log_file` |
| `MO_LOG_MAX_SIZE` | Backend | Legacy positive-integer override for `logging.max_size_mb` |
| `MO_LOG_MAX_BACKUPS` | Backend | Legacy positive-integer override for `logging.max_backups` |
| `MO_LOG_MAX_AGE` | Backend | Legacy positive-integer override for `logging.max_age_days` |
| `XDG_CONFIG_HOME` | Backend | Base directory for XDG config discovery |
| `HOME` | Backend | Enables `$HOME/.config/music-online/` discovery when XDG is unset |
| `HOSTNAME` | Backend | Admin system-info fallback when the HTTP request has no host |
| `TZ` | Runtime image | Runtime time zone; the image includes time-zone data |
| `VITE_API_PROXY_TARGET` | Frontend development only | Vite `/api` proxy target; default `http://localhost:8080` |

Prefer the unified `LOGGING_*` variables for new deployments; the `MO_LOG_MAX_*` names remain compatibility aliases. `VITE_API_PROXY_TARGET` is consumed at frontend development/build time and does not configure the embedded production server.

## Backup & Restore

For SQLite, back up the selected YAML file, SQLite database, and upload directory. Stop the service first when possible, or at minimum ensure no upload or database write is running.

For a current-directory deployment:

```bash
tar -czf music-online-backup.tgz config.yaml music.db uploads/
```

The default Compose deployment stores the database and uploads in the `music-online-data` named volume. Stop it with `make docker-down`, then archive the volume separately from the protected `.env` and config file:

```bash
docker run --rm \
  -v music-online-data:/source:ro \
  -v "$PWD:/backup" \
  alpine:3.24.1 tar -czf /backup/music-online-data.tgz -C /source .
```

Restore into an empty volume before starting the application. If `DATA_VOLUME_NAME` was customized, use that engine-level volume name in backup and restore commands.

For PostgreSQL, back up the database with `pg_dump` and back up uploaded files from the application data volume. Restore both before starting the older or replacement deployment.

### Database Upgrades & Rollback

At startup, the service applies pending database migrations in version order and records them in `schema_migrations`. Each migration and its history record run in one transaction. A failure stops startup and is not marked as applied, so it can be retried after the cause is fixed.

Before upgrading, back up the database, uploaded files, and configuration. Automatic down migrations are not provided. To roll back, stop the service, restore the complete pre-upgrade backup, then deploy the older binary. Do not point an older binary at a database with unknown newer migrations; it will refuse to start.

## License

MIT
