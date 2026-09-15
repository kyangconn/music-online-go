# Music Online

面向个人、家庭或小团队的 self-hosted 小型音乐平台：管理、检索和播放自己的音乐库，离线运行不依赖外部服务，并兼容 MusicBrainz Picard 常用标签。Go 后端与 Vue 3 前端会编译为单一静态二进制；SQLite 是默认路径，PostgreSQL 可选。

## 快速开始

两种方式任选其一，都能在 <http://localhost:8080> 打开。

### Docker（推荐）

```bash
cp config-docker-example.yaml config.yaml   # 把 jwt.secret 改成随机值：openssl rand -hex 32
docker compose up -d --build
```

数据存放在命名卷 `music-online-data`，配置来自只读挂载进容器的 `config.yaml`。

### 本地运行

```bash
cp config-example.yaml config.yaml   # 把 jwt.secret 改成随机值
make build                            # 首次必须先构建前端产物
./music-online
```

> 新克隆后必须先用 `make build`（或 `make build-fe`）生成 `cmd/server/dist/`：后端用
> `go:embed` 内嵌前端产物，而该目录是构建产物、不入库，直接 `go build` 会报
> `pattern dist/*: no matching files found`。

默认不创建管理员；如需初始化首个管理员，见[配置](#配置)中的 `admin.bootstrap`。

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

## 核心能力

- 浏览器上传与管理员服务端目录扫描共用同一套 Picard/MusicBrainz 兼容元数据模型。
- 艺术家和专辑视图优先使用 MusicBrainz 稳定 ID 分组，无 ID 时使用 Unicode 规范化文本回退；
  多碟专辑按碟号、曲号播放，并可整张加入播放队列。
- 音乐列表可按艺术家、专辑、专辑艺术家、流派、发行年份、类型和收藏状态筛选，筛选与分页保留在 URL 中。
- 登录用户可创建仅自己可见的播放列表，添加、移除和排序曲目；删除音乐时会同步清理播放列表引用。
- 公开或登录后访问两种实例模式共用同一套列表、聚合、封面和音频访问策略。
- 四类场景预设以可解释的元数据规则离线工作，可选叠加音频模型/DSP 弱证据；低置信度结果进入管理员审核队列，人工修正不会被重新分析覆盖。

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
make test       # Go 测试 + 前端 Vitest
make test-cover # Go 测试 + 函数级覆盖率汇总
make test-cover-fe # 前端覆盖率汇总
make test-postgres # 可选：本地 PostgreSQL 集成测试（需设置 MUSIC_ONLINE_TEST_POSTGRES_DSN）
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

上面[快速开始](#快速开始)就是默认的 Docker 部署（SQLite）。项目只内置两个 compose 文件：
`compose.yaml` 和 `compose.postgres.yaml`；其它场景把下面片段加进 `compose.yaml`，
再 `docker compose up -d --build` 即可（也可另存为 override 文件用 `-f` 叠加）。

### 使用 PostgreSQL

`compose.postgres.yaml` 会额外启动一个 Postgres 容器：

```bash
POSTGRES_PASSWORD='强密码' make docker-up-postgres
```

已有 PostgreSQL 则不要用该文件，改在 `config.yaml` 里指向它：

```yaml
database:
  type: postgres
  host: your-postgres-host
  port: "5432"
  user: music_online
  password: ...
  name: music_online
  sslmode: require
```

### 挂载本机音乐目录

在 `compose.yaml` 的 `app` 服务里加一个只读 bind mount：

```yaml
services:
  app:
    volumes:
      - type: bind
        source: /srv/music
        target: /media/music
        read_only: true
```

启动后在管理后台登记容器内路径 `/media/music` 并手动扫描。

### 用 Docker secret 传密钥（可选）

不想把密钥明文写进 `config.yaml` 时，用文件传（`config.yaml` 里对应字段留空）：

```bash
mkdir -p secrets
openssl rand -hex 32 > secrets/jwt_secret
```

```yaml
services:
  app:
    environment:
      JWT_SECRET: ""
      JWT_SECRET_FILE: /run/secrets/jwt_secret
    secrets:
      - jwt_secret

secrets:
  jwt_secret:
    file: ./secrets/jwt_secret
```

其它敏感字段同理：`DATABASE_PASSWORD_FILE`、`METRICS_TOKEN_FILE`、`ADMIN_BOOTSTRAP_PASSWORD_FILE`、
`INTEGRATIONS_MUSICBEE_SUBMIT_TOKEN_FILE`、`CLASSIFICATION_ANALYZER_TOKEN_FILE`。

### 可选音频分析器

需要实现下方「可选 HTTP analyzer 与持久任务」契约的 analyzer 镜像。在 `compose.yaml` 里加：

```yaml
services:
  app:
    environment:
      CLASSIFICATION_ANALYZER_MODE: http
      CLASSIFICATION_ANALYZER_ENDPOINT: http://analyzer:8090/v1/analyze
      CLASSIFICATION_ANALYZER_ID: implementation-id
      CLASSIFICATION_ANALYZER_VERSION: 1.0.0
      CLASSIFICATION_ANALYZER_MODEL_VERSION: model-v1
      CLASSIFICATION_ANALYZER_TOKEN: shared-secret
    depends_on:
      - analyzer

  analyzer:
    image: your-analyzer-image
    environment:
      ANALYZER_LISTEN_ADDRESS: 0.0.0.0
      ANALYZER_PORT: "8090"
      ANALYZER_TOKEN: shared-secret
```

`CLASSIFICATION_ANALYZER_ID` / `_VERSION` / `_MODEL_VERSION` 必须与 analyzer 返回的版本一致，
`ANALYZER_TOKEN` 至少 32 字节；完整字段与返回契约见下方「可选 HTTP analyzer 与持久任务」。

### 反向代理 / Traefik

默认发布到 `127.0.0.1:8080` 不影响容器网络；在自定义 override 中把 `app` 加入你的网络并加 labels 即可。

## 配置

程序使用 [Viper](https://github.com/spf13/viper) 加载配置。对配置值而言，优先级为
**环境变量 > 第一个找到的 YAML 文件 > 代码默认值**；不会合并多个 YAML 文件。五个敏感字段还支持
`*_FILE`：直接环境值优先于 YAML，文件值也优先于 YAML；同时设置直接值与对应文件变量会拒绝启动。
`--config-file` 和 `--log-file` 分别具有最高的路径覆盖优先级。

配置会在私有解析器中完整构造和校验，只有成功后才作为启动快照交给各服务；不支持请求期间热修改。
修改 YAML 或环境变量后需重启实例，失败的加载不会把半成品配置暴露给运行代码。

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
| `INTEGRATIONS_MUSICBEE_SUBMIT_TOKEN_FILE` | MusicBee 提交凭证文件；不能与 `INTEGRATIONS_MUSICBEE_SUBMIT_TOKEN` 同时设置 |
| `CLASSIFICATION_ANALYZER_TOKEN_FILE` | HTTP analyzer Bearer 密钥文件；不能与 `CLASSIFICATION_ANALYZER_TOKEN` 同时设置 |
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

#### library.scanner 与媒体库

| YAML 字段 | 环境变量 | 默认值 | 说明 |
|---|---|---|---|
| `library.health_check_interval_seconds` | `LIBRARY_HEALTH_CHECK_INTERVAL_SECONDS` | `60` | 后台复核媒体根可用性的间隔；`0` 关闭周期检查，但保留管理界面的手动探测 |
| `library.scanner.enabled` | `LIBRARY_SCANNER_ENABLED` | `true` | 是否允许管理员创建服务端媒体库扫描任务；关闭不会删除目录配置或历史 |
| `library.scanner.max_file_size_mb` | `LIBRARY_SCANNER_MAX_FILE_SIZE_MB` | `2048` | 扫描单个来源文件的上限（MiB）；必须大于零，独立于浏览器上传上限 |
| `library.scanner.max_tag_size_mb` | `LIBRARY_SCANNER_MAX_TAG_SIZE_MB` | `16` | 进入标签解析器前允许的元数据上限（MiB），范围 `1`–`64`；还会校验 Vorbis/APE 条目数量和内嵌图片长度 |
| `library.scanner.min_file_age_seconds` | `LIBRARY_SCANNER_MIN_FILE_AGE_SECONDS` | `30` | 跳过修改时间距今不足此秒数的文件；`0` 表示不等待 |
| `library.scanner.hash_recheck_hours` | `LIBRARY_SCANNER_HASH_RECHECK_HOURS` | `168` | 即使大小和修改时间未变，也周期性重算内容哈希；`0` 关闭周期复核 |
| `library.scanner.retry_max_attempts` | `LIBRARY_SCANNER_RETRY_MAX_ATTEMPTS` | `5` | 可重试存储故障的最多扫描尝试次数，必须大于零 |
| `library.scanner.retry_initial_seconds` | `LIBRARY_SCANNER_RETRY_INITIAL_SECONDS` | `30` | 首次重试等待时间，之后指数退避 |
| `library.scanner.retry_max_seconds` | `LIBRARY_SCANNER_RETRY_MAX_SECONDS` | `900` | 单次退避上限，不能小于初始等待时间 |

`server.upload_dir` 同时是默认托管媒体库：用户上传、扫描提取的封面和数据库可控文件写入这里。
管理员添加的其他绝对路径一律是只读来源，音频原地播放；应用不会修改、移动或删除来源文件。
目录路径是服务进程看到的路径，因此 Docker 管理界面中应填写 `/media/music` 之类的容器路径，
而不是宿主机的 `/srv/music`。

扫描仅由管理员手动触发，采用附加语义：已存在且未变化的来源直接跳过；发现新文件才加入；
文件暂时缺失只把物理来源标记为 missing，不会删除逻辑曲目；同一路径内容发生变化时保留原记录和人工元数据，
把来源标记为 changed 并在扫描详情中提示复核。同一内容出现在多个目录时会保存多个物理来源，而不是丢弃副本；
播放可使用仍在线的副本，未来 M5 的分析产物可按内容哈希和分析器版本复用。
扫描会读取常见内嵌标签，包括 MusicBrainz Picard 常用标识，并在安全可用时复制内嵌封面到托管目录，
但不会调用外部 MusicBrainz 服务。外部根目录互相之间以及与托管目录都不能重叠；已有曲目引用某个
外部根目录时，不能修改它的路径或删除它，可先停用以暂停扫描和播放解析。

后台只有一个扫描 worker，缓慢目录不会堆叠并发 I/O。每个根可选择 `auto/local/nfs/smb`、预期文件系统
和根内相对探针文件；Linux 会读取 `/proc/self/mountinfo` 并实际打开目录/探针，区分 `mount_missing`、
`mount_mismatch`、`permission_denied`、`stale_handle`、`network_unreachable`、`io_timeout` 与 `read_failed`。
应用不会把 ping 当作 NFS 证据，也不会自行 mount、`net use` 或修复网络存储；只有配置为 NFS 且探测证据
支持时，界面才明确显示“NFS 离线”。可重试故障采用有上限的指数退避，任务状态和原因保存在数据库中。
Windows 原生部署优先使用 UNC 路径；Windows 能判断远程路径是否可读，却不会向此进程可靠暴露底层 NFS/SMB
协议，因此可读的远程根会显示为“降级可用”，而不会伪造协议已核实的结论。映射盘还会额外提示其登录会话作用域。

外部目录不参加 `/ready` 探测，NFS 暂时不可用不会让 HTTP 服务退出就绪状态；但扫描或播放触发的
内核网络文件系统调用未必能被 Go context 中断。应在宿主机完成 NFS 挂载，并按所用操作系统和存储端
设置合理的连接、重试和故障恢复参数，再只读 bind mount 到容器。Linux 默认 `rprivate` 要求先挂载后启动
容器；如确实需要把容器启动后的宿主重挂事件传播进去，可在确认运行时支持后设置
`MEDIA_BIND_PROPAGATION=rslave`。NFS `hard` 挂载通常更保护数据完整性，但服务线程可能长期等待；`soft`
挂载可能返回 I/O 错误甚至带来数据完整性风险，不能只为了更快超时而盲目启用。

Windows 原生运行优先登记 UNC（如 `\\server\music`）；映射盘符依赖登录会话，服务账户可能看不到，
因此会显示降级告警。Windows 上的 Docker 仍应登记容器内路径，不要让应用执行 `net use`。扫描被取消或
服务停止时，已经导入的曲目会保留；运行任务由数据库租约在过期后安全恢复。

NFS/SMB 只用于媒体来源，不建议承载 SQLite 数据库文件或 `/data` 写入卷；网络文件系统的锁与缓存语义
可能破坏 SQLite 的可靠性。单实例默认使用本地 SQLite；需要多个应用实例或共享数据库时改用 PostgreSQL。

#### classification 本地预设规则

| YAML 字段 | 环境变量 | 默认值 | 说明 |
|---|---|---|---|
| `classification.enabled` | `CLASSIFICATION_ENABLED` | `true` | 启用本地、离线可用的标签规则分类；关闭不影响导入、浏览或播放 |
| `classification.analyze_on_upload` | `CLASSIFICATION_ANALYZE_ON_UPLOAD` | `false` | 音频上传或目录导入提交成功后自动追加音频分析任务；仅可与 HTTP analyzer 一起启用 |
| `classification.auto_threshold` | `CLASSIFICATION_AUTO_THRESHOLD` | `0.65` | 最高分至少达到该值才可能自动归类，范围 `(0, 1]` |
| `classification.review_margin` | `CLASSIFICATION_REVIEW_MARGIN` | `0.12` | 最高两类分差低于该值时进入待确认，范围 `[0, 1]` |
| `classification.weights.calm_flow` | `CLASSIFICATION_WEIGHTS_CALM_FLOW` | `1.0` | 静谧心流规则分数倍率，范围 `(0, 2]` |
| `classification.weights.kinetic_pulse` | `CLASSIFICATION_WEIGHTS_KINETIC_PULSE` | `1.0` | 律动跃迁规则分数倍率，范围 `(0, 2]` |
| `classification.weights.cosmic_drift` | `CLASSIFICATION_WEIGHTS_COSMIC_DRIFT` | `1.0` | 星云漫游规则分数倍率，范围 `(0, 2]` |
| `classification.weights.bass_impact` | `CLASSIFICATION_WEIGHTS_BASS_IMPACT` | `1.0` | 低频震域规则分数倍率，范围 `(0, 2]` |
| `classification.analyzer.mode` | `CLASSIFICATION_ANALYZER_MODE` | `"disabled"` | `disabled` 保持纯 Go 单体部署；`http` 启用可选的字节流分析器 |
| `classification.analyzer.endpoint` | `CLASSIFICATION_ANALYZER_ENDPOINT` | `""` | analyzer 的绝对 HTTP(S) `POST` 地址；不得包含凭据或 fragment |
| `classification.analyzer.token` | `CLASSIFICATION_ANALYZER_TOKEN` / `_FILE` | `""` | 两端共享的 Bearer 密钥，HTTP 模式至少 32 字节；推荐通过文件传入 |
| `classification.analyzer.id` | `CLASSIFICATION_ANALYZER_ID` | `""` | 分析器实现稳定标识，参与缓存键并必须与响应一致 |
| `classification.analyzer.version` | `CLASSIFICATION_ANALYZER_VERSION` | `""` | 分析器实现版本，参与缓存键 |
| `classification.analyzer.model_version` | `CLASSIFICATION_ANALYZER_MODEL_VERSION` | `""` | 模型/规则包版本，参与缓存键 |
| `classification.analyzer.timeout_seconds` | `CLASSIFICATION_ANALYZER_TIMEOUT_SECONDS` | `300` | 单次 HTTP 分析硬超时，范围 `1`–`3600` 秒 |
| `classification.analyzer.concurrency` | `CLASSIFICATION_ANALYZER_CONCURRENCY` | `1` | 后台 worker 数，范围 `1`–`8`；SQLite 仍通过单写连接协调任务 |
| `classification.analyzer.queue_limit` | `CLASSIFICATION_ANALYZER_QUEUE_LIMIT` | `1000` | `pending + running` 任务的背压上限 |
| `classification.analyzer.max_file_size_mb` | `CLASSIFICATION_ANALYZER_MAX_FILE_SIZE_MB` | `2048` | 允许流向 analyzer 的最大源文件大小 |
| `classification.analyzer.max_duration_seconds` | `CLASSIFICATION_ANALYZER_MAX_DURATION_SECONDS` | `1800` | 请求 analyzer 停止解码的最大音频时长，同时校验返回时长 |
| `classification.analyzer.retry_max_attempts` | `CLASSIFICATION_ANALYZER_RETRY_MAX_ATTEMPTS` | `3` | 网络、超时和服务端暂时错误的最大尝试次数 |
| `classification.analyzer.retry_initial_seconds` | `CLASSIFICATION_ANALYZER_RETRY_INITIAL_SECONDS` | `30` | 首次有限指数退避延迟 |
| `classification.analyzer.retry_max_seconds` | `CLASSIFICATION_ANALYZER_RETRY_MAX_SECONDS` | `900` | 单次重试等待上限 |

规则层只读取规范化后的本地标签，保存四类独立分数、规则版本和证据，并允许弃权。管理员人工选择与自动结果分开保存，后续重新分类不会覆盖人工选择。权重用于真实曲库校准，不改变 API/数据库中的稳定预设标识。

启用分类时，用户可从“场景预设”浏览并播放已确认曲目；待确认结果不会被强行放进四个合集。管理员后台提供明确的待确认队列、证据与分析产物来源，并可在当前列表中批量选择最多 100 首曲目。批量指定或清除人工预设在一个数据库事务内完成，任意曲目不存在时整批回滚；人工处理后的曲目会退出待确认队列，清除后按自动状态重新进入。

#### 可选 HTTP analyzer 与持久任务

基础镜像不会安装 FFmpeg、Python 或模型运行时。启用 analyzer 后，应用只把任务状态保存在现有 SQLite/PostgreSQL 中：默认单 worker 领取带租约和 fencing generation 的任务，重启会回收过期的 `running` 任务；重复请求按曲目内容哈希、内容修订及分析器/模型版本幂等复用。相同字节的多个物理来源共享 `music_audio_analyses` 产物。队列满、分析器离线、超时、损坏文件或取消均不会回滚已经成功的上传、目录导入或播放。

应用向配置的 endpoint 发送 `POST application/octet-stream`，使用 `Authorization: Bearer <token>`，并附带以下请求头：

- `X-Music-Online-Music-ID`
- `X-Music-Online-File-Hash`（SHA-256）
- `X-Music-Online-Content-Revision`
- `X-Music-Online-Max-Duration-Ms`

请求体只包含由服务端媒体根解析器选出的受控音频流，不传递宿主机/容器路径，也不接受浏览器提供路径。应用不会跟随 endpoint 重定向，以免 Bearer 密钥泄露。analyzer 必须完整消费声明的 `Content-Length`，并返回不超过 1 MiB 的 JSON：

```json
{
  "analyzer_id": "implementation-id",
  "analyzer_version": "1.0.0",
  "model_version": "model-v1",
  "duration_ms": 213000,
  "features": {
    "bpm": 128.0,
    "bpm_confidence": 0.86,
    "bpm_candidates": [{ "bpm": 128.0, "confidence": 0.86 }],
    "danceability": 0.82,
    "energy": 0.77,
    "pulse_clarity": 0.74
  },
  "model_labels": { "trance": 0.74, "progressive house": 0.43 }
}
```

三个版本字段必须与配置完全一致；模型标签分数必须在 `0..1`，数值必须有限，嵌套深度、键数、数组和字符串都有上限。服务端在流式发送时重新计算 SHA-256；与任务哈希不符时任务标记为 `stale`，结果不会入库。

`hybrid-v1` 只消费下列稳定的顶层特征；未知或嵌套的厂商字段会原样保存在分析产物中，但不会静默变成分类规则：

- 拍速：`bpm`（`20..400`）、`bpm_confidence` 和最多八个 `{bpm, confidence}` 候选。拍速会折叠 half-time/double-time 后仅作弱亲和度，不设硬分界。
- 活动与节奏：`energy`、`arousal`、`dynamic_smoothness`、`dynamic_range_normalized`、`danceability`、`onset_rate_normalized`、`pulse_clarity`、`high_energy_segment_ratio`。
- 频谱与质感：`spectral_centroid_normalized`、`spectral_flatness`、`spectral_flux`、`roughness`、`loudness_normalized`、`bass_energy_ratio`、`sub_bass_energy_ratio`、`drop_contrast`。
- 调性、空间与人声：`tonal_strength`、`harmonicity`、`spatiality`、`instrumental_probability`、`vocal_probability`。

除 `bpm` 外，上述标量都必须由 analyzer 按其版本化定义归一化到 `0..1`；不要把原始 LUFS、Hz 或未标定距离直接塞入同名字段。模型标签会经过同一流派规范化器，权重上限低于本地标签；DSP 再低一级。“纯音乐”只有同时获得低能量和低唤醒度/平滑动态证据时才支持静谧心流。分析产物还必须与曲目当前 SHA-256 匹配，旧文件结果不会被误用于新内容。

仓库暂不指定默认模型镜像。候选筛选、许可证边界、gold set 拆分和可复现命令见
[音频分析候选与基准协议](docs/audio-analysis-benchmark.md)。准备好通过该门槛、实现上述契约的镜像和
至少 32 字节密钥后，按 [Docker](#docker) 一节的「可选音频分析器」片段接入（共享密钥推荐用 Docker
secret 传入）。管理员可通过后台或 `/api/v1/users/admin/analysis/*` 显式回填、查看指标、重试和取消任务。

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
| `jwt.access_token_ttl_minutes` | `JWT_ACCESS_TOKEN_TTL_MINUTES` | `15` | 短期 access token 有效期（分钟）；前端只把 access token 保存在内存 |
| `jwt.refresh_token_ttl_days` | `JWT_REFRESH_TOKEN_TTL_DAYS` | `30` | 服务端会话（refresh token）有效期（天）；到期后必须重新登录 |
| `jwt.refresh_cookie_secure` | `JWT_REFRESH_COOKIE_SECURE` | `false` | 为 refresh httpOnly cookie 附加 `Secure` 标志；仅 HTTPS 部署时开启 |
| `metrics.enabled` | `METRICS_ENABLED` | `false` | 是否开放 `/metrics` |
| `metrics.token` | `METRICS_TOKEN` | `""` | 抓取用 Bearer token；启用指标时必填 |
| `admin.bootstrap.enabled` | `ADMIN_BOOTSTRAP_ENABLED` | `false` | 启动时确保首个管理员存在 |
| `admin.bootstrap.username` | `ADMIN_BOOTSTRAP_USERNAME` | `""` | 管理员用户名 |
| `admin.bootstrap.email` | `ADMIN_BOOTSTRAP_EMAIL` | `""` | 新建管理员邮箱 |
| `admin.bootstrap.password` | `ADMIN_BOOTSTRAP_PASSWORD` | `""` | 新建/重置密码：至少 8 个 Unicode 字符、最多 72 个 UTF-8 字节 |
| `admin.bootstrap.full_name` | `ADMIN_BOOTSTRAP_FULL_NAME` | `"Administrator"` | 管理员显示名 |
| `admin.bootstrap.reset_password` | `ADMIN_BOOTSTRAP_RESET_PASSWORD` | `false` | 已存在用户是否重置密码 |

认证与会话模型：登录成功后服务端为每个设备创建一条可撤销的会话记录，refresh token 只以 SHA-256 哈希形式入库，并通过 `HttpOnly; SameSite=Strict; Path=/api/v1/users` cookie 下发（JavaScript 无法读取）。access token 是短期的（默认 15 分钟）并在每次请求时校验对应会话仍未被撤销，因此单设备登出、全部设备登出或管理员禁用账户都会立即生效。`POST /api/v1/users/refresh` 每次轮换 refresh token；旧 token 在 30 秒宽限窗口内的重放按并发处理，窗口之外的重放会撤销整个会话（防盗窃）。改密码会撤销当前设备之外的所有会话。纯 API 客户端可以在 `POST /api/v1/users/refresh` 请求体中提交 `refresh_token`（登录响应只通过 cookie 下发 refresh token，不写入 JSON 响应体）。

密码使用 bcrypt 哈希保存；所有新密码统一要求至少 8 个 Unicode 字符、最多 72 个 UTF-8 字节（bcrypt 的输入上限）。当前不存在 RSA 字段加密配置。

指标默认关闭。启用时必须同时设置非空 token，并使用
`Authorization: Bearer <token>` 抓取。`/health` 只表示进程存活；`/ready` 会同时检查数据库连接和上传目录可写性。

首个管理员初始化是显式 opt-in：默认不创建任何管理员。启用后，程序会在数据库迁移后确保指定用户是可用的管理员；如果用户名不存在则创建，如果用户名已存在则提升为 admin 并激活账号。

初始化完成后，建议关闭 `admin.bootstrap.enabled` 并重启；如果保留启用状态，程序也只会确保该用户仍是 admin，不会重复创建账号。生产环境不要把管理员密码提交到仓库，优先用环境变量或只读部署配置提供。

#### access 与 integrations.musicbee

| YAML 字段 | 环境变量 | 默认值 | 说明 |
|---|---|---|---|
| `access.library_mode` | `ACCESS_LIBRARY_MODE` | `"public"` | `public` 允许匿名浏览和播放；`authenticated` 要求登录后访问音乐列表、详情、用户音乐、标签查询、封面和音频流 |
| `access.registration_mode` | `ACCESS_REGISTRATION_MODE` | `"open"` | `open` 开放注册；`admin` 关闭公开注册，由管理员在管理面板创建用户 |
| `access.media_url_ttl_minutes` | `ACCESS_MEDIA_URL_TTL_MINUTES` | `60` | 私有库签名封面/音频 URL 的有效分钟数；必须大于零，前端会在播放前续签 |
| `integrations.musicbee.submit_token` | `INTEGRATIONS_MUSICBEE_SUBMIT_TOKEN` | `""` | MusicBee 兼容提交凭证；至少 32 字节，留空时提交端点不存在 |
| `integrations.musicbee.submit_username` | `INTEGRATIONS_MUSICBEE_SUBMIT_USERNAME` | `""` | 提交记录归属的已启用本地用户；必须与 token 同时配置 |

`GET /api/v1/instance` 公开返回当前访问与注册能力，前端据此隐藏注册入口并保护私有库页面。`public` 仅表示音乐库读取公开；上传、修改、点赞和管理操作仍要求登录。`authenticated` 模式下 API 返回短期签名媒体 URL，避免把长期 JWT 放入 `<audio>` 或 `<img>` 查询参数；私有库读取和媒体响应带 `Cache-Control: private, no-store`，登出或会话失效时前端也会清除持久化播放队列。

关闭公开注册前，应先通过 `admin.bootstrap` 建立首个管理员。之后管理员可以在“管理面板 → 用户管理”创建普通用户或管理员；系统不会生成或发送邮件邀请。

MusicBee 兼容写入口默认关闭。配置两项后，客户端以 `Authorization: Bearer <submit_token>` 调用 `POST /api/v1/track/submit`，凭证只具有 `track:submit` 范围，创建的记录归属 `submit_username`。提交体使用与统一曲目元数据相同的 JSON 字段；撤销时删除或轮换 token 并重启实例。读取端点仍遵守 `access.library_mode`。生产环境优先使用 `INTEGRATIONS_MUSICBEE_SUBMIT_TOKEN_FILE`，并通过自定义 Compose override 将只读 secret 文件挂入容器。

提交契约为 `Content-Type: application/json`，`title` 与 `artist` 必填；调用只创建元数据记录，不上传或读取客户端文件路径。成功返回 HTTP 201 和未包裹的曲目对象。常用字段如下：

```json
{
  "title": "Track",
  "artist": "Artist feat. Guest",
  "artists": ["Artist", "Guest"],
  "album": "Release",
  "album_artist": "Album Artist",
  "album_artists": ["Album Artist"],
  "track_number": 2,
  "track_total": 12,
  "disc_number": 1,
  "disc_total": 2,
  "year": 2024,
  "release_date": "2024-03-02",
  "original_release_date": "2023",
  "genres": ["Ambient", "Chillout"],
  "comment": "tag comment",
  "isrcs": ["USABC2412345"],
  "duration": 201,
  "musicbrainz_recording_id": "00000000-0000-4000-8000-000000000001",
  "musicbrainz_track_id": "00000000-0000-4000-8000-000000000002",
  "musicbrainz_release_id": "00000000-0000-4000-8000-000000000003",
  "musicbrainz_release_group_id": "00000000-0000-4000-8000-000000000004",
  "musicbrainz_artist_ids": ["00000000-0000-4000-8000-000000000005"],
  "musicbrainz_album_artist_ids": ["00000000-0000-4000-8000-000000000006"]
}
```

日期接受 `YYYY`、`YYYY-MM` 或 `YYYY-MM-DD`；MusicBrainz 字段必须是对应实体的 UUID；ISRC 可带常见连字符，入库时规范为 12 位大写形式。旧客户端的 `musicbrainz_id` 和 `musicbrainz_artist_id` 仍分别作为 recording ID 和单个 artist ID 的输入别名，但响应只使用语义明确的新字段。`POST /api/v1/track/search` 不接受提交 token：公开库可匿名读取，私有库需正常用户 JWT，以免提交凭证扩张成通用会话。

`genres` 按来源保留多值及展示大小写，兼容字段 `genre` 由它拼接；响应中的只读 `genre_tokens` 才会拆分常见分隔符并折叠大小写，供检索和后续场景分类复用。`metadata_revision` 只在规范化后的曲目元数据实际变化时递增，音频替换则继续由内部 `file_hash` 单独标识。未来的音频分析与预设分类会放在独立派生表中，以这两个值判断规则层或音频层是否过期，不把分类结果写回原始标签字段。

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

管理员登记的只读媒体根不由应用复制或管理，必须按原存储系统单独备份；应用备份只包含它们的索引、
人工元数据和已提取到托管目录的封面。恢复到另一台主机或容器路径变化时，应先以原容器路径重新挂载，
否则已有曲目会保留但音频暂时无法播放。

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

统一元数据迁移会把旧 `music_tags` 中能以“标题 + 艺术家（以及存在时的专辑）”唯一对应到曲目的空缺字段合并进 `vinyl`。无法唯一对应的行不会猜测或丢弃；包含旧行的整张表随后保留为历史归档表 `legacy_music_tags_v1`，应用不再读写它。旧表为空时直接移除，不会给新实例留下第二套元数据表。旧 `use_count`、搜索向量和模糊匹配不再参与运行时行为；ID 空间无法安全映射的 `/music-tags` 按 ID 增删改查端点也会移除，避免旧 tag ID 误操作同号曲目。`/music-tags/search`、`/music-tags/match`、MBID 查询和 `/track` 客户端端点改为使用统一曲目模型。确认升级结果并保留过一次完整备份周期后，管理员可自行归档该历史表；应用不会自动删除它。

升级前请按上面的方式同时备份数据库、上传目录和配置。项目不提供自动向下迁移：需要回滚程序版本时，先停止服务，恢复升级前的完整备份，再部署旧版本程序。不要让旧版本程序直接使用已经执行过新迁移的数据库；旧程序检测到未知迁移版本时会拒绝启动。

## License

MIT
