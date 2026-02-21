# Music Online

## 简介
一个全栈在线音乐平台的重构项目。
- **后端**: Go (Gin + GORM + PostgreSQL)
- **前端**: Vue 3 + Element Plus + Vite
- **部署**: 单文件分发 (前端静态资源嵌入 Go 二进制文件)

## 快速开始

### 1. 开发模式
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

### 2. 生产构建
```bash
# 1. 构建前端
cd web
npm run build

# 2. 构建后端 (自动嵌入前端产物)
cd ../
go build -o music-server ./cmd/server
```
运行生成的 `music-server`，访问 `http://localhost:8080`。
