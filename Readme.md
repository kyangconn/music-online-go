# Music Online

## 简介
一个全栈在线音乐平台的重构项目，目标是为个人音乐爱好者提供一个可自托管、可维护的在线听歌与管理平台。

- **后端**: Go (Gin + GORM + PostgreSQL)
- **前端**: Vue 3 + Element Plus + Vite
- **部署**: 前端静态资源打包后嵌入 Go 二进制，单文件分发

## 快速开始

### 1. 开发模式
**后端:**

```bash
# 复制示例配置并根据本地环境修改
cp config-example.yaml config.yaml

# 启动后端服务
go run cmd/server/main.go
```

**前端:**

```bash
cd web
npm install
npm run dev
```
访问 `http://localhost:5173`。

### 2. 生产构建

```bash
# 1. 构建前端（会输出到 cmd/server/dist 下）
cd web
npm run build

# 2. 构建后端 (自动嵌入前端产物)
cd ../
go build -o music-server ./cmd/server
```
运行生成的 `music-server`，访问 `http://localhost:8080`。

## 项目结构

- `cmd/server`：HTTP 入口、静态资源嵌入、路由注册
- `internal/config`：配置加载
- `internal/domain`：核心领域模型（用户、音乐等）
- `internal/repository`：数据库访问层（PostgreSQL + GORM）
- `internal/service`：业务逻辑（点赞、搜索等）
- `internal/handler`：HTTP Handler（Gin）
- `web`：前端单页应用（Vue 3 + Element Plus）

## 子模块 / 前端仓库

本仓库的 `web` 目录可以作为子模块挂到独立的前端仓库。

更多英文说明见 [Readme-EN.md](./Readme-EN.md)。
