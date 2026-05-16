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

**前端**（详见 [web/](./web/)）
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

参考 [config-example.yaml](./config-example.yaml)，支持环境变量覆盖。

| 配置项 | 说明 | 默认值 |
|---|---|---|
| `server.port` | 监听端口 | `8080` |
| `server.mode` | 运行模式 | `debug` |
| `database.type` | 数据库类型 | `sqlite` |
| `jwt.secret` | JWT 签名密钥 | - |

## License

MIT
