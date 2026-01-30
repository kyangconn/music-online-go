# Music Online

[English](#english) | [中文](#chinese)

<a name="english"></a>
## English

### Introduction
A full-stack online music platform rewrite.
- **Backend**: Go (Gin + GORM + PostgreSQL)
- **Frontend**: Vue 3 + Element Plus + Vite
- **Deployment**: Single binary distribution (Frontend embedded in Go binary)

### Tech Stack
- **Language**: Go 1.24+, TypeScript
- **Web Framework**: [Gin](https://github.com/gin-gonic/gin) (Go), Vue 3 (JS)
- **Database**: PostgreSQL
- **Build Tool**: Vite

### Getting Started

#### 1. Prerequisites
- Go 1.24+
- Node.js & npm
- PostgreSQL

#### 2. Development (Hot Reload)
Run backend and frontend in separate terminals.

**Backend:**
```bash
cp config-example.yaml config.yaml
# Update database config in config.yaml
go run cmd/server/main.go
```

**Frontend:**
```bash
cd web
npm install
npm run dev
```
Visit `http://localhost:5173`. API requests are proxied to port 8080.

#### 3. Production Build
Build a single executable file containing the frontend.

```bash
# 1. Build Frontend
cd web
npm run build

# 2. Build Backend (Embeds web/dist)
cd ../
go build -o music-server ./cmd/server
```

Run `./music-server`. Visit `http://localhost:8080`.

---

<a name="chinese"></a>
## 中文 (Chinese)

### 简介
一个全栈在线音乐平台的重构项目。
- **后端**: Go (Gin + GORM + PostgreSQL)
- **前端**: Vue 3 + Element Plus + Vite
- **部署**: 单文件分发 (前端静态资源嵌入 Go 二进制文件)

### 快速开始

#### 1. 开发模式
**后端:**
```bash
go run cmd/server/main.go
```

**前端:**
```bash
cd web
npm run dev
```
访问 `http://localhost:5173`。

#### 2. 生产构建
```bash
# 1. 构建前端
cd web
npm run build

# 2. 构建后端 (自动嵌入前端产物)
cd ../
go build -o music-server ./cmd/server
```
运行生成的 `music-server`，访问 `http://localhost:8080`。
