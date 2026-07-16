# Music Online

全栈音乐管理平台，Go 后端 + Vue 3 前端，编译为单一静态二进制文件。

## 技术栈

**后端**

- Go 1.26 + Gin 框架
- GORM（支持 SQLite / PostgreSQL）
- JWT 认证 + OTP 双因素
- 可选的 Prometheus 指标监控（Bearer token 保护）
- 零信任限流中间件
- bcrypt 密码哈希

**前端**（详见 [web](./web)）

- Vue 3 + TypeScript
- Element Plus 组件库
- Pinia 状态管理 + Vue Router
- 国际化（中 / 英）
- Vite 构建

## 项目结构

```
.
├── cmd/server/          # 程序入口，embed 前端产物
├── internal/
│   ├── config/          # 配置加载（viper）
│   ├── domain/          # 领域模型
│   ├── handler/         # HTTP 处理器
│   ├── middleware/       # 中间件（auth/ratelimit/logger）
│   ├── pkg/             # 基础包（database/jwt/password）
│   ├── repository/      # 数据访问层
│   ├── router/          # 路由注册
│   └── service/         # 业务逻辑层
├── web/                 # Vue 3 前端（独立子项目）
├── Dockerfile           # 多阶段构建
├── Makefile             # 构建 & 开发快捷命令
├── go.mod
└── config-example.yaml  # 配置文件模板
```

## 快速开始

```bash
# 1. 复制配置，并把 jwt.secret 替换为强随机值
cp config-example.yaml config.yaml
openssl rand -hex 32
# 可选：在 config.yaml 里启用 admin.bootstrap 创建首个管理员

# 2. 构建（前端 + 后端）
make build

# 3. 运行
./music-online
```

Windows PowerShell 可用 `Copy-Item config-example.yaml config.yaml`，构建后运行
`./music-online.exe`。访问 `http://localhost:8080`。

## 开发

```bash
# 安装/更新前端依赖（从根目录控制 web/）
make install-fe      # 按 pnpm-lock 安装
make install-fe-dev  # 开发时安装/更新依赖

# 同时启动前后端开发服务器
make dev       # 后端 + 前端
make dev-be    # 后端 :8080
make dev-fe    # 前端 :5173（自动代理 API 到后端）
```

前端 dev server 已配置 `/api` 代理，开发时无需额外配置。

## 构建

```bash
make build      # 前端构建 + 后端编译
make build-silent # 较少输出的前端构建 + 后端编译
make build-fe   # 仅前端
make build-be   # 仅后端（需已有前端产物）
```

前端构建产物输出到 `cmd/server/dist/`，Go 通过 `embed` 内嵌到最终二进制中，部署只需一个文件。

## 测试 & Lint

```bash
make test       # Go 测试 + 前端 ESLint
make test-cover # Go 测试 + 函数级覆盖率汇总
make check      # 非修改型检查：Go vet + 前端 typecheck/ESLint
make verify     # Go 测试 + check
make lint       # 修改型检查：Go fmt + Go vet + 前端 ESLint --fix
make lint-fe    # 前端 ESLint --fix
make lint-be    # Go vet
make audit-be   # 非修改型 golangci-lint 全量审计
```

后端深度检查使用 `golangci-lint` v2；版本固定在 `.golangci-lint-version`，CI
只阻止新增问题。本地运行 `make audit-be` 前需安装该版本，参见
[golangci-lint 官方安装文档](https://golangci-lint.run/docs/welcome/install/)。

## Docker

```bash
cp .env.example .env
# 用 `openssl rand -hex 32` 等方式生成随机值，填入 .env 的 JWT_SECRET
make docker-config
make docker-up
```

默认 Compose 部署使用 SQLite，发布到 `127.0.0.1:8080`。配置模板
`config-docker-example.yaml` 以只读方式映射到容器，数据库和上传文件位于命名卷
`music-online-data`。`.env` 已被 Git 忽略，不要提交其中的 JWT、数据库或管理员密码。

PostgreSQL 部署需在 `.env` 中设置 `POSTGRES_PASSWORD`，然后叠加 override：

```bash
make docker-config-postgres
make docker-up-postgres

# 停止服务但保留数据卷
make docker-down
```

生产部署可改用 Compose secrets，避免把明文密钥放进容器环境：

```bash
mkdir -p secrets
openssl rand -hex 32 > secrets/jwt_secret
make docker-config-secrets
make docker-up-secrets

# PostgreSQL 再创建一个独立密码文件
openssl rand -hex 24 > secrets/postgres_password
make docker-config-postgres-secrets
make docker-up-postgres-secrets
```

PowerShell 可先运行 `New-Item -ItemType Directory -Force secrets`，再用
`Set-Content -NoNewline secrets/jwt_secret '<随机值>'` 写入。`secrets/` 已被 Git 忽略。
`compose.secrets.yaml` 挂载 JWT 文件；PostgreSQL 部署再叠加
`compose.postgres-secrets.yaml`，同一个数据库密码文件会同时提供给应用与官方 PostgreSQL 镜像。

默认 PostgreSQL 18 数据卷映射到 `/var/lib/postgresql`，由镜像在其下使用带主版本号的目录。
不要只改 `POSTGRES_IMAGE` 跨主版本复用旧卷；先按 PostgreSQL 的 `pg_upgrade` 或导出/导入流程升级。

镜像采用 Node → Go → Alpine 多阶段构建，并固定基础镜像版本；运行时使用 UID/GID
`10001` 的非 root 用户、只读根文件系统、`no-new-privileges`、移除 Linux capabilities、
`/tmp` tmpfs 和 `/ready` 健康检查。`/data` 是唯一持久写入点。若把 `DATA_PATH`
改为宿主机目录而非命名卷，需要预先保证 UID `10001` 对该目录可写。

常用 Compose 参数都在 [.env.example](./.env.example) 中：

| 变量 | 默认值 | 说明 |
|---|---|---|
| `COMPOSE_PROJECT_NAME` | `music-online` | Compose 项目名 |
| `MUSIC_ONLINE_IMAGE` | `music-online-go:local` | 构建/运行的镜像名 |
| `APP_BIND_ADDRESS` / `APP_PORT` | `127.0.0.1` / `8080` | 宿主机监听地址与端口；对外开放前应配防火墙或 HTTPS 反代 |
| `APP_CONTAINER_PORT` | `8080` | 容器内端口 fallback；Compose 会把它作为 `SERVER_PORT` 传给应用 |
| `CONFIG_FILE` / `MO_CONFIG_FILE` | `./config-docker-example.yaml` / `/etc/music-online/config.yaml` | 宿主配置来源与容器内只读目标 |
| `DATA_PATH` / `DATA_VOLUME_NAME` | `music-online-data` | `/data` 的来源及默认命名卷名称 |
| `RESTART_POLICY` / `READ_ONLY_ROOTFS` | `unless-stopped` / `true` | 重启策略与只读根文件系统开关 |
| `LOG_DRIVER`, `LOG_MAX_SIZE`, `LOG_MAX_FILE` | `local`, `10m`, `3` | Docker stdout/stderr 日志驱动和轮转上限；不同于应用文件日志 |
| `TMPFS_SIZE` / `STOP_GRACE_PERIOD` | `256m` / `30s` | multipart 临时空间与停止宽限期；提高上传上限时也要相应提高 |
| `HEALTHCHECK_INTERVAL`, `HEALTHCHECK_TIMEOUT`, `HEALTHCHECK_START_PERIOD`, `HEALTHCHECK_RETRIES` | `30s`, `5s`, `15s`, `3` | 应用健康检查参数 |
| `VERSION`, `VCS_REF`, `BUILD_DATE` | `dev`, `unknown`, epoch | 镜像标签和二进制版本元数据 |
| `POSTGRES_IMAGE`, `POSTGRES_USER`, `POSTGRES_PASSWORD`, `POSTGRES_DB`, `POSTGRES_PORT` | 见 `.env.example` | PostgreSQL override 的镜像和连接参数；密码可直接提供或改用 secret 文件 |
| `POSTGRES_PASSWORD_FILE` | `""` | 官方 PostgreSQL 容器内的密码文件路径；secrets override 会设为 `/run/secrets/postgres_password` |
| `POSTGRES_DATA_PATH` / `POSTGRES_DATA_VOLUME_NAME` | `music-online-postgres-data` | PostgreSQL 数据来源和默认卷名 |
| `POSTGRES_STOP_GRACE_PERIOD` | `30s` | PostgreSQL 停止宽限期 |
| `POSTGRES_HEALTHCHECK_INTERVAL`, `POSTGRES_HEALTHCHECK_TIMEOUT`, `POSTGRES_HEALTHCHECK_START_PERIOD`, `POSTGRES_HEALTHCHECK_RETRIES` | `10s`, `5s`, `10s`, `5` | PostgreSQL 健康检查参数 |
| `JWT_SECRET_HOST_FILE` | `./secrets/jwt_secret` | `compose.secrets.yaml` 在宿主机读取的 JWT secret 文件 |
| `POSTGRES_PASSWORD_HOST_FILE` | `./secrets/postgres_password` | `compose.postgres-secrets.yaml` 在宿主机读取的数据库密码文件 |

`make docker` 也接受 Make 变量 `DOCKER_IMAGE`、`VERSION`、`VCS_REF`、`BUILD_DATE`。
Compose 中未设置的应用变量保持为空，不会覆盖 YAML；应用启动时要求通过 `JWT_SECRET`、
`JWT_SECRET_FILE` 或 YAML 提供有效密钥。容器端口也由 `SERVER_PORT` 或 `APP_CONTAINER_PORT` 统一传入，避免端口映射、
健康检查和 YAML 监听端口彼此漂移。其他环境变量有值时会覆盖只读 YAML，因此可在不改模板的情况下定制每项应用配置。

PWA 安装和离线应用壳在 `localhost` 可直接使用；从其他设备访问自部署实例时，需要通过 HTTPS 反向代理暴露服务，普通局域网 HTTP 地址不会启用这些安全上下文能力。

## 配置

程序使用 [Viper](https://github.com/spf13/viper) 加载配置。对配置值而言，优先级为
**环境变量 > 第一个找到的 YAML 文件 > 代码默认值**；不会合并多个 YAML 文件。四个敏感字段还支持
`*_FILE`：直接环境值优先于 YAML，文件值也优先于 YAML；同时设置直接值与对应文件变量会拒绝启动。
`--config-file` 和 `--log-file` 分别具有最高的路径覆盖优先级。

### 命令行参数

```bash
./music-online --config-file /path/to/config.yaml --log-file /var/log/music.log
```

| 参数              | 说明                                 |
|-----------------|------------------------------------|
| `--config-file` | 指定配置文件完整路径（覆盖自动搜索）                 |
| `--log-file`    | 日志文件路径（覆盖配置文件中的 `server.log_file`） |

### 配置文件搜索顺序

程序启动时按下表从高到低搜索 `config.yaml`，读取第一个找到的文件后停止：

| 优先级 | 路径 | 适用场景 |
|---|---|---|
| 1（最高） | 二进制文件所在目录 | 单文件生产部署 |
| 2 | `/data/` | 兼容旧 Docker 数据卷布局 |
| 3 | `/etc/music-online/` | 系统级配置 |
| 4 | `$XDG_CONFIG_HOME/music-online/` 或 `$HOME/.config/music-online/` | 用户级配置 |
| 5 | `../`、`../../` | 从 `cmd/server` 运行 |
| 6（最低） | `./`、`./config/` | 当前开发目录 |

如果 `MO_CONFIG_FILE` 环境变量或 `--config-file` 参数被设置，则直接使用指定路径，跳过所有搜索。

找不到配置文件时仍会加载代码默认值和环境变量，但 `jwt.secret` 始终必填，所以至少需要
`JWT_SECRET`。默认数据库是 SQLite 文件 `music.db`。一旦选择 `postgres`，缺少 host、user
或 name 会阻止启动，不会静默写入另一个 SQLite 数据库。

### 环境变量

所有 YAML 键都可按 `.` → `_` 并转为大写后由环境变量覆盖：

```bash
SERVER_PORT=8080 DATABASE_TYPE=sqlite DATABASE_PATH=/data/music.db ./music-online
```

专用或运行时变量如下：

| 环境变量 | 说明 |
|---|---|
| `MO_CONFIG_FILE` | 指定 YAML 完整路径（`--config-file` 会覆盖它） |
| `JWT_SECRET_FILE` | JWT 签名密钥文件；读取后移除结尾换行，不能与 `JWT_SECRET` 同时设置 |
| `DATABASE_PASSWORD_FILE` | 数据库密码文件；不能与 `DATABASE_PASSWORD` 同时设置 |
| `METRICS_TOKEN_FILE` | 指标 Bearer token 文件；不能与 `METRICS_TOKEN` 同时设置 |
| `ADMIN_BOOTSTRAP_PASSWORD_FILE` | 首个管理员密码文件；不能与 `ADMIN_BOOTSTRAP_PASSWORD` 同时设置 |
| `MO_LOG_FILE` | 日志路径；优先级为 `--log-file` > `MO_LOG_FILE` > `SERVER_LOG_FILE`/YAML |
| `MO_LOG_MAX_SIZE`, `MO_LOG_MAX_BACKUPS`, `MO_LOG_MAX_AGE` | 旧版日志轮转兼容别名，有值时覆盖 `LOGGING_*` 对应项 |
| `XDG_CONFIG_HOME` / `HOME` | 决定用户级配置搜索目录 |
| `HOSTNAME` | Admin 系统信息无法读取 OS hostname 时的 fallback |
| `TZ` | 容器系统时区，例如 `UTC` 或 `Asia/Shanghai`；不是 YAML 字段 |
| `VITE_API_PROXY_TARGET` | 仅前端开发服务器使用，默认 `http://localhost:8080`；不会进入后端运行配置 |

首个管理员也可以完全用环境变量初始化：

```bash
ADMIN_BOOTSTRAP_ENABLED=true \
ADMIN_BOOTSTRAP_USERNAME=admin \
ADMIN_BOOTSTRAP_EMAIL=admin@example.com \
ADMIN_BOOTSTRAP_PASSWORD='change-me-please' \
./music-online
```

### 完整应用配置与环境变量

可复制 [config-example.yaml](./config-example.yaml)。下表逐项列出所有 YAML 字段及其环境变量。

#### server

| YAML 字段 | 环境变量 | 默认值 | 说明 |
|---|---|---|---|
| `server.listen_address` | `SERVER_LISTEN_ADDRESS` | `""` | 监听地址；空表示所有接口 |
| `server.port` | `SERVER_PORT` | `"8080"` | 监听端口 |
| `server.mode` | `SERVER_MODE` | `"debug"` | `debug` / `release` / `test` |
| `server.read_header_timeout` | `SERVER_READ_HEADER_TIMEOUT` | `10` | 读取请求头超时（秒） |
| `server.read_timeout` | `SERVER_READ_TIMEOUT` | `0` | 读取完整请求超时；`0` 为不限制 |
| `server.write_timeout` | `SERVER_WRITE_TIMEOUT` | `0` | 写完整响应超时；`0` 为不限制 |
| `server.idle_timeout` | `SERVER_IDLE_TIMEOUT` | `60` | keep-alive 空闲超时（秒） |
| `server.shutdown_timeout` | `SERVER_SHUTDOWN_TIMEOUT` | `15` | 优雅关闭等待时间（秒） |
| `server.readiness_timeout` | `SERVER_READINESS_TIMEOUT` | `2` | 单次数据库就绪探测超时（秒） |
| `server.upload_dir` | `SERVER_UPLOAD_DIR` | `"uploads"` | 上传存储目录 |
| `server.log_file` | `SERVER_LOG_FILE` | `""` | 日志文件；空表示仅 stdout |
| `server.max_json_body_size_mb` | `SERVER_MAX_JSON_BODY_SIZE_MB` | `1` | JSON 请求体硬上限（MiB） |
| `server.max_audio_size_mb` | `SERVER_MAX_AUDIO_SIZE_MB` | `200` | 单个音频上限（MiB） |
| `server.max_cover_size_mb` | `SERVER_MAX_COVER_SIZE_MB` | `10` | 单个封面上限（MiB） |
| `server.allowed_origins` | `SERVER_ALLOWED_ORIGINS` | `[]` | 允许凭据请求的额外 Origin；环境变量逗号分隔 |
| `server.trusted_proxies` | `SERVER_TRUSTED_PROXIES` | `[]` | 可信直接代理的 IP/CIDR；环境变量逗号分隔 |

`server.upload_dir` 支持绝对路径或相对路径；相对路径按服务进程的当前工作目录解析。本地 `make dev-be` 默认会写到仓库根目录下的 `uploads/`，Docker 默认通过 `SERVER_UPLOAD_DIR=/data/uploads` 写入挂载卷。

JSON 请求在进入 handler 前最多缓冲 `max_json_body_size_mb`，已知和 chunked 超限请求都返回 413。上传会在 multipart 解析前限制整个请求体，并继续校验单文件大小、扩展名、请求 MIME 和文件头签名。前端预检只用于减少无效上传，后端才是安全边界。`read_timeout` 保持 `0` 可避免慢速大文件上传被统一超时截断；请求头仍由 `read_header_timeout` 限制，上传体仍受硬大小上限保护。`write_timeout=0` 同样避免媒体流被固定时长切断。Compose 的 `/tmp` 默认是 `256m` tmpfs，约覆盖一个接近默认上限的 multipart 请求；并发上传时按 `(音频上限 + 封面上限 + 约 1 MiB 开销) × 预期并发数` 调整 `TMPFS_SIZE`。反向代理的请求体上限也必须覆盖音频、封面和 multipart 开销，代理上传/响应超时应覆盖最慢受支持客户端，否则请求会在到达应用前失败。

同源请求无需加入 `server.allowed_origins`。只有把前端部署到不同来源时才需要配置完整来源，例如
`["https://music-ui.example.com"]`；不支持通配符或带路径的地址。

默认不信任任何代理地址，也不会采信任意客户端伪造的 `X-Forwarded-*`。反向代理部署时只配置直接代理的 IP/CIDR；不要为了省事使用 `0.0.0.0/0`。

#### database

| YAML 字段 | 环境变量 | 默认值 | 说明 |
|---|---|---|---|
| `database.type` | `DATABASE_TYPE` | `"sqlite"` | `sqlite` / `postgres` |
| `database.path` | `DATABASE_PATH` | `"music.db"` | SQLite 文件路径；`:memory:` 仅用于测试 |
| `database.host` | `DATABASE_HOST` | `""` | PostgreSQL 主机 |
| `database.port` | `DATABASE_PORT` | `"5432"` | PostgreSQL 端口 |
| `database.user` | `DATABASE_USER` | `""` | PostgreSQL 用户 |
| `database.password` | `DATABASE_PASSWORD` | `""` | PostgreSQL 密码 |
| `database.name` | `DATABASE_NAME` | `""` | PostgreSQL 数据库名 |
| `database.sslmode` | `DATABASE_SSLMODE` | `"prefer"` | `disable`、`allow`、`prefer`、`require`、`verify-ca` 或 `verify-full` |
| `database.log_level` | `DATABASE_LOG_LEVEL` | `"auto"` | GORM SQL 日志：`auto` / `silent` / `error` / `warn` / `info` |
| `database.connect_timeout_seconds` | `DATABASE_CONNECT_TIMEOUT_SECONDS` | `10` | PostgreSQL 建连超时（秒） |
| `database.max_open_connections` | `DATABASE_MAX_OPEN_CONNECTIONS` | `0` | 最大连接数；`0` 使用类型默认值 |
| `database.max_idle_connections` | `DATABASE_MAX_IDLE_CONNECTIONS` | `0` | 最大空闲连接数；`0` 使用类型默认值 |
| `database.connection_max_lifetime_minutes` | `DATABASE_CONNECTION_MAX_LIFETIME_MINUTES` | `60` | 连接最长生命周期（分钟）；`0` 不限制 |
| `database.connection_max_idle_time_minutes` | `DATABASE_CONNECTION_MAX_IDLE_TIME_MINUTES` | `10` | 连接最长空闲时间（分钟）；`0` 不限制 |

连接池为 `0` 时，SQLite 自动使用 open/idle `1/1`，避免单文件并发写锁争用；PostgreSQL 使用 `25/5`。外部 PostgreSQL 应按服务端 TLS 配置选择 SSL mode，跨主机生产部署优先 `verify-full`。所有 SQL 日志级别都会保留占位符而不输出绑定参数，避免密码、TOTP 等值进入日志。

环境变量示例：

```bash
# SQLite（默认文件数据库）
DATABASE_TYPE=sqlite DATABASE_PATH=music.db ./music-online

# SQLite（仅测试用内存数据库）
DATABASE_TYPE=sqlite DATABASE_PATH=:memory: ./music-online

# PostgreSQL
DATABASE_TYPE=postgres DATABASE_HOST=localhost DATABASE_PORT=5432 DATABASE_USER=postgres DATABASE_PASSWORD=postgres DATABASE_NAME=music-online DATABASE_SSLMODE=require ./music-online
```

#### jwt、metrics、admin.bootstrap

| YAML 字段 | 环境变量 | 默认值 | 说明 |
|---|---|---|---|
| `jwt.secret` | `JWT_SECRET` | 无 | JWT 签名密钥，至少 32 个 UTF-8 字节；release/test 还会拒绝示例占位值 |
| `jwt.expire_hour` | `JWT_EXPIRE_HOUR` | `24` | JWT 有效期（小时） |
| `metrics.enabled` | `METRICS_ENABLED` | `false` | 是否开放 `/metrics` |
| `metrics.token` | `METRICS_TOKEN` | `""` | 抓取用 Bearer token；启用指标时必填 |
| `admin.bootstrap.enabled` | `ADMIN_BOOTSTRAP_ENABLED` | `false` | 启动时确保首个管理员存在 |
| `admin.bootstrap.username` | `ADMIN_BOOTSTRAP_USERNAME` | `""` | 管理员用户名 |
| `admin.bootstrap.email` | `ADMIN_BOOTSTRAP_EMAIL` | `""` | 新建管理员邮箱 |
| `admin.bootstrap.password` | `ADMIN_BOOTSTRAP_PASSWORD` | `""` | 新建/重置密码：至少 8 个 Unicode 字符、最多 72 个 UTF-8 字节 |
| `admin.bootstrap.full_name` | `ADMIN_BOOTSTRAP_FULL_NAME` | `"Administrator"` | 管理员显示名 |
| `admin.bootstrap.reset_password` | `ADMIN_BOOTSTRAP_RESET_PASSWORD` | `false` | 已存在用户是否重置密码 |

密码使用 bcrypt 哈希保存；所有新密码统一要求至少 8 个 Unicode 字符、最多 72 个 UTF-8 字节（bcrypt 的输入上限）。当前不存在 RSA 字段加密配置。

指标默认关闭。启用时必须同时设置非空 token，并使用
`Authorization: Bearer <token>` 抓取。`/health` 只表示进程存活；`/ready` 会同时检查数据库连接和上传目录可写性。

首个管理员初始化是显式 opt-in：默认不创建任何管理员。启用后，程序会在数据库迁移后确保指定用户是可用的管理员；如果用户名不存在则创建，如果用户名已存在则提升为 admin 并激活账号。

初始化完成后，建议关闭 `admin.bootstrap.enabled` 并重启；如果保留启用状态，程序也只会确保该用户仍是 admin，不会重复创建账号。生产环境不要把管理员密码提交到仓库，优先用环境变量或只读部署配置提供。

#### rate_limit 与 logging

| YAML 字段 | 环境变量 | 默认值 | 说明 |
|---|---|---|---|
| `rate_limit.enabled` | `RATE_LIMIT_ENABLED` | `true` | 是否启用进程内限流 |
| `rate_limit.global_requests_per_second` | `RATE_LIMIT_GLOBAL_REQUESTS_PER_SECOND` | `20` | 全局每 IP 平均请求/秒 |
| `rate_limit.global_burst` | `RATE_LIMIT_GLOBAL_BURST` | `50` | 全局每 IP 突发容量 |
| `rate_limit.auth_requests_per_second` | `RATE_LIMIT_AUTH_REQUESTS_PER_SECOND` | `1` | 登录/注册每 IP 平均请求/秒 |
| `rate_limit.auth_burst` | `RATE_LIMIT_AUTH_BURST` | `5` | 登录/注册每 IP 突发容量 |
| `logging.max_size_mb` | `LOGGING_MAX_SIZE_MB` | `50` | 单个日志文件上限（MB） |
| `logging.level` | `LOGGING_LEVEL` | `"info"` | 应用日志最低级别：`debug` / `info` / `warn` / `error` |
| `logging.access_log` | `LOGGING_ACCESS_LOG` | `true` | 是否记录逐请求访问日志 |
| `logging.max_backups` | `LOGGING_MAX_BACKUPS` | `3` | 保留的旧文件数；`0` 不按数量清理 |
| `logging.max_age_days` | `LOGGING_MAX_AGE_DAYS` | `28` | 保留天数；`0` 不按天数清理 |
| `logging.compress` | `LOGGING_COMPRESS` | `true` | 是否压缩轮转文件 |
| `logging.local_time` | `LOGGING_LOCAL_TIME` | `true` | 轮转文件名是否使用本地时间 |

限流按 Gin 解析出的客户端 IP 统计；可信代理配置错误会直接影响 IP 识别，因此应与反向代理拓扑一起设置。限流状态为单进程内存数据，多副本部署需要在网关层追加全局限流。

### 日志

日志默认仅输出到 stdout；配置 `server.log_file` 后同时写入轮转文件。容器部署建议继续使用 stdout，由容器平台负责采集和保留。

```bash
# 仅 stdout
./music-online

# stdout + 文件
./music-online --log-file /var/log/music.log
```

## 备份与恢复

SQLite 单机部署只需要备份三类内容：`config.yaml`、SQLite 数据库文件（默认 `music.db` 或 `database.path` 指向的文件）、上传目录（默认 `uploads/` 或 `server.upload_dir`）。备份前最好停止服务，或至少确保没有上传正在进行。

```bash
# 示例：把当前目录部署的数据打包
tar -czf music-online-backup.tgz config.yaml music.db uploads/
```

恢复时停止服务，把这三类内容放回原路径，再启动程序。Docker 部署通常只需要备份挂载的 `data/` 目录：

```bash
tar -czf music-online-data.tgz data/
```

PostgreSQL 部署需要同时备份数据库和上传目录：

```bash
pg_dump "$DATABASE_URL" > music-online.sql
tar -czf music-online-files.tgz config.yaml uploads/
```

恢复 PostgreSQL 时先导入 SQL，再放回上传目录和配置文件。

### 数据库升级与回滚

服务启动时会按版本顺序自动执行尚未应用的数据库迁移，并将结果记录在 `schema_migrations` 表中。每条迁移及其记录位于同一个事务内；任一迁移失败都会阻止服务启动，且不会被标记为已应用，修复原因后可直接重试。

升级前请按上面的方式同时备份数据库、上传目录和配置。项目不提供自动向下迁移：需要回滚程序版本时，先停止服务，恢复升级前的完整备份，再部署旧版本程序。不要让旧版本程序直接使用已经执行过新迁移的数据库；旧程序检测到未知迁移版本时会拒绝启动。

## License

MIT
