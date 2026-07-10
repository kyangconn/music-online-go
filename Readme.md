# Music Online

全栈音乐管理平台，Go 后端 + Vue 3 前端，编译为单一静态二进制文件。

## 技术栈

**后端**

- Go 1.26 + Gin 框架
- GORM（支持 SQLite / PostgreSQL）
- JWT 认证 + OTP 双因素
- Prometheus 指标监控
- 零信任限流中间件
- RSA 加密敏感字段

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
# 1. 复制并编辑配置文件
cp config-example.yaml config.yaml
# 可选：在 config.yaml 里启用 admin.bootstrap 创建首个管理员

# 2. 构建（前端 + 后端）
make build

# 3. 运行
./music-server
```

访问 `http://localhost:8080`。

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
make check      # 非修改型检查：Go vet + 前端 typecheck/ESLint
make verify     # Go 测试 + check
make lint       # 修改型检查：Go fmt + Go vet + 前端 ESLint --fix
make lint-fe    # 前端 ESLint --fix
make lint-be    # Go vet
```

## Docker

```bash
make docker
mkdir -p data
cp config-example.yaml data/config.yaml
docker run --rm -p 8080:8080 -v "$PWD/data:/data" music-online-go
```

多阶段构建：前端 node → 编译 → Go 编译 → Alpine 运行镜像。镜像不内置 `config.yaml`；运行时通过 `/data/config.yaml`、环境变量或命令行参数提供配置。默认容器数据路径为 `/data/music.db` 和 `/data/uploads`，因此应挂载 `/data` 保持数据持久化。

PWA 安装和离线应用壳在 `localhost` 可直接使用；从其他设备访问自部署实例时，需要通过 HTTPS 反向代理暴露服务，普通局域网 HTTP 地址不会启用这些安全上下文能力。

## 配置

程序使用 [Viper](https://github.com/spf13/viper) 加载配置，支持 **YAML 文件 + 环境变量 + 命令行参数** 三层覆盖。

### 命令行参数

```bash
./music-server --config-file /path/to/config.yaml --log-file /var/log/music.log
```

| 参数              | 说明                                 |
|-----------------|------------------------------------|
| `--config-file` | 指定配置文件完整路径（覆盖自动搜索）                 |
| `--log-file`    | 日志文件路径（覆盖配置文件中的 `server.log_file`） |

### 配置文件搜索顺序

程序启动时按以下顺序搜索 `config.yaml`，**后找到的路径优先级更高**：

| 优先级   | 路径                                                               | 适用场景                   |
|-------|------------------------------------------------------------------|------------------------|
| 1（最低） | `./`、`./config/`                                                 | 开发时当前目录                |
| 2     | `../`、`../../`                                                   | 从 `cmd/server` 运行时向上查找 |
| 3     | `$XDG_CONFIG_HOME/music-online/` 或 `$HOME/.config/music-online/` | XDG 用户级配置              |
| 4     | `/etc/music-online/`                                             | 系统级配置                  |
| 5     | `/data/`                                                         | Docker volume 挂载目录     |
| 6（最高） | 二进制文件所在目录                                                        | 生产部署目录                 |

如果 `MO_CONFIG_FILE` 环境变量或 `--config-file` 参数被设置，则直接使用指定路径，跳过所有搜索。

找不到配置文件时程序正常启动，全部使用默认值。默认数据库是 SQLite 文件 `music.db`；只有显式配置 PostgreSQL 且提供 host/user/name 时才连接 PostgreSQL。

### 环境变量

Viper 启用了 `AutomaticEnv`，所有 YAML 键按 `.` → `_` 替换后均可通过环境变量覆盖：

```bash
SERVER_PORT=8080 DATABASE_TYPE=sqlite DATABASE_PATH=/data/music.db ./music-server
```

此外还有专用的环境变量：

| 环境变量              | 说明                                                           |
|-------------------|--------------------------------------------------------------|
| `MO_CONFIG_FILE`  | 指定配置文件路径（与 `--config-file` 等效）                               |
| `MO_LOG_FILE`     | 日志文件路径（优先级：`--log-file` > `MO_LOG_FILE` > `server.log_file`） |
| `XDG_CONFIG_HOME` | XDG 基础目录，用于搜索用户级配置                                           |
| `HOSTNAME`        | Admin 面板中显示的 host 名（fallback）                                |

首个管理员也可以完全用环境变量初始化：

```bash
ADMIN_BOOTSTRAP_ENABLED=true \
ADMIN_BOOTSTRAP_USERNAME=admin \
ADMIN_BOOTSTRAP_EMAIL=admin@example.com \
ADMIN_BOOTSTRAP_PASSWORD='change-me-please' \
./music-server
```

### 完整 YAML 字段

参考 [config-example.yaml](./config-example.yaml)。

#### server

| 字段                     | 类型     | 默认值         | 说明                       |
|------------------------|--------|-------------|--------------------------|
| `server.port`          | string | `"3060"`    | 监听端口                     |
| `server.mode`          | string | `"debug"`   | 运行模式：`debug` / `release` |
| `server.read_timeout`  | int    | `30`        | HTTP 读超时（秒）              |
| `server.write_timeout` | int    | `30`        | HTTP 写超时（秒）              |
| `server.upload_dir`    | string | `"uploads"` | 上传文件存储目录                 |
| `server.log_file`      | string | `""`        | 日志文件路径（空表示仅 stdout）      |
| `server.max_audio_size_mb` | int | `200`       | 单个音频文件最大大小（MB）          |
| `server.max_cover_size_mb` | int | `10`        | 单个封面图片最大大小（MB）          |

`server.upload_dir` 支持绝对路径或相对路径；相对路径按服务进程的当前工作目录解析。本地 `make dev-be` 默认会写到仓库根目录下的 `uploads/`，Docker 默认通过 `SERVER_UPLOAD_DIR=/data/uploads` 写入挂载卷。

上传会同时校验大小、扩展名、请求 MIME 和文件头签名。前端也会做预检以减少无效上传，但后端校验才是最终安全边界。

#### database

| 字段                  | 类型     | 默认值          | 说明                                  |
|---------------------|--------|--------------|-------------------------------------|
| `database.type`     | string | `"sqlite"`   | 数据库类型：`sqlite` / `postgres`         |
| `database.host`     | string | -            | PostgreSQL 主机（仅 postgres）           |
| `database.port`     | string | -            | PostgreSQL 端口（仅 postgres）           |
| `database.user`     | string | -            | PostgreSQL 用户（仅 postgres）           |
| `database.password` | string | -            | PostgreSQL 密码（仅 postgres）           |
| `database.name`     | string | -            | PostgreSQL 数据库名（仅 postgres）         |
| `database.sslmode`  | string | `"disable"`  | PostgreSQL SSL 模式（仅 postgres）       |
| `database.path`     | string | `"music.db"` | SQLite 文件路径（仅 sqlite，支持 `:memory:`） |

环境变量示例：

```bash
# SQLite（默认文件数据库）
DATABASE_TYPE=sqlite DATABASE_PATH=music.db ./music-server

# SQLite（仅测试用内存数据库）
DATABASE_TYPE=sqlite DATABASE_PATH=:memory: ./music-server

# PostgreSQL
DATABASE_TYPE=postgres DATABASE_HOST=localhost DATABASE_PORT=5432 DATABASE_USER=postgres DATABASE_PASSWORD=postgres DATABASE_NAME=music-online ./music-server
```

#### jwt

| 字段                | 类型     | 默认值  | 说明                 |
|-------------------|--------|------|--------------------|
| `jwt.secret`      | string | -    | JWT 签名密钥（生产环境必须修改） |
| `jwt.expire_hour` | int    | `24` | JWT 过期时间（小时）       |

#### admin.bootstrap

首个管理员初始化是显式 opt-in：默认不创建任何管理员。启用后，程序会在数据库迁移后确保指定用户是可用的管理员；如果用户名不存在则创建，如果用户名已存在则提升为 admin 并激活账号。

| 字段                                      | 类型     | 默认值               | 说明                                  |
|-----------------------------------------|--------|-------------------|-------------------------------------|
| `admin.bootstrap.enabled`               | bool   | `false`           | 是否启用启动时管理员初始化                     |
| `admin.bootstrap.username`              | string | -                 | 管理员用户名                              |
| `admin.bootstrap.email`                 | string | -                 | 创建新管理员时使用的邮箱                       |
| `admin.bootstrap.password`              | string | -                 | 创建管理员密码，至少 8 位；不会写入日志             |
| `admin.bootstrap.full_name`             | string | `"Administrator"` | 创建新管理员或补全空姓名时使用                    |
| `admin.bootstrap.reset_password`        | bool   | `false`           | 用户名已存在时，是否用配置中的 password 重置密码      |

初始化完成后，建议关闭 `admin.bootstrap.enabled` 并重启；如果保留启用状态，程序也只会确保该用户仍是 admin，不会重复创建账号。生产环境不要把管理员密码提交到仓库，优先用环境变量或只读部署配置提供。

#### security

| 字段                       | 类型     | 默认值 | 说明         |
|--------------------------|--------|-----|------------|
| `security.password_salt` | string | -   | 密码哈希盐值（预留） |

### 日志

日志默认输出到 **stdout + 文件**（如果配置了 `log_file`）。文件以追加模式写入，OpenFile 失败时仅打印警告，不会阻止程序启动。

```bash
# 仅 stdout
./music-server

# stdout + 文件
./music-server --log-file /var/log/music.log
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

## License

MIT
