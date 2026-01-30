# 全栈集成 TODO 清单

## 1. 前端配置 (Web)
- [x] **配置 Vite Proxy**: 修改 `vite.config.ts`，将 `/api` 请求代理到后端 (localhost:8080)，解决开发环境跨域问题。
- [ ] **构建输出配置**: 确保 `npm run build` 输出到 `dist` 目录 (默认即是)。
- [ ] **API 客户端封装**: 创建 `src/api/axios.ts`，封装 Axios 实例，统一处理 BaseURL 和拦截器。
- [ ] **生成静态资源**: 运行 `npm run build` 生成 `dist` 文件夹，供 Go 后端嵌入使用。

## 2. 后端集成 (Go)
- [ ] **引入 embed 包**: 在 `main.go` 中 import `embed`。
- [ ] **静态资源嵌入**: 在 `main.go` 中使用 `//go:embed web/dist` 将前端资源打包进二进制。(注意：需先有 `web/dist` 目录)
- [ ] **路由接管**: 修改 Gin 路由配置：
    - `/api` 开头的请求 -> 走 API 逻辑 (已有的)。
    - 其他请求 -> 返回 `web/dist/index.html` (前端路由 History 模式支持)。
    - 静态资源 (assets/) -> 直接通过 FileServer 返回。
- [ ] **CORS 配置调整**: 生产环境(嵌入模式)不需要 CORS，但开发环境需要允许 `localhost:5173`。

## 3. 验证与部署
- [ ] **本地联调**: 启动 Go 后端 (8080) 和 Vite 前端 (5173)，验证 API 调用是否通畅。
- [ ] **构建测试**: 运行 `npm run build`，然后 `go build`，运行生成的 `.exe`，验证是否能直接访问页面并播放音乐。