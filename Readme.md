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

# 2. 构建（前端 + 后端）
make build

# 3. 运行
./music-server
```

访问 `http://localhost:8080`。

## 开发

```bash
# 同时启动前后端开发服务器
make dev-be    # 后端 :8080
make dev-fe    # 前端 :5173（自动代理 API 到后端）
```

前端 dev server 已配置 `/api` 代理，开发时无需额外配置。

## 构建

```bash
make build      # 前端构建 + 后端编译
make build-fe   # 仅前端
make build-be   # 仅后端（需已有前端产物）
```

前端构建产物输出到 `cmd/server/dist/`，Go 通过 `embed` 内嵌到最终二进制中，部署只需一个文件。

## 测试 & Lint

```bash
make test       # Go 测试 + 前端 ESLint
make lint       # 前端 ESLint
make lint-be    # Go vet
```

## Docker

```bash
make docker
```

多阶段构建：前端 node → 编译 → Go 编译 → Alpine 运行镜像。

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

找不到配置文件时程序正常启动，全部使用默认值。

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

#### database

| 字段                  | 类型     | 默认值          | 说明                                  |
|---------------------|--------|--------------|-------------------------------------|
| `database.type`     | string | `"postgres"` | 数据库类型：`sqlite` / `postgres`         |
| `database.host`     | string | -            | PostgreSQL 主机（仅 postgres）           |
| `database.port`     | string | -            | PostgreSQL 端口（仅 postgres）           |
| `database.user`     | string | -            | PostgreSQL 用户（仅 postgres）           |
| `database.password` | string | -            | PostgreSQL 密码（仅 postgres）           |
| `database.name`     | string | -            | PostgreSQL 数据库名（仅 postgres）         |
| `database.sslmode`  | string | `"disable"`  | PostgreSQL SSL 模式（仅 postgres）       |
| `database.path`     | string | `"music.db"` | SQLite 文件路径（仅 sqlite，支持 `:memory:`） |

环境变量示例：

```bash
# SQLite
DATABASE_TYPE=sqlite DATABASE_PATH=:memory: ./music-server

# PostgreSQL
DATABASE_HOST=localhost DATABASE_PORT=5432 DATABASE_USER=postgres DATABASE_PASSWORD=postgres DATABASE_NAME=music-online ./music-server
```

#### jwt

| 字段                | 类型     | 默认值  | 说明                 |
|-------------------|--------|------|--------------------|
| `jwt.secret`      | string | -    | JWT 签名密钥（生产环境必须修改） |
| `jwt.expire_hour` | int    | `24` | JWT 过期时间（小时）       |

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

## License

MIT
