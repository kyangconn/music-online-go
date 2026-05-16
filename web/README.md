# Music Online Pages

Music Online 前端页面，基于 Vue 3 + TypeScript + Element Plus 的单页应用。

## 技术栈

- Vue 3 + TypeScript
- Element Plus 组件库
- Pinia 状态管理 + Vue Router
- vue-i18n 国际化（中 / 英）
- Vite 构建工具

## 项目结构

```
.
├── public/
│   ├── fonts/           # 字体资源
│   └── icons/           # 网站图标
├── src/
│   ├── assets/          # 静态资源
│   ├── components/      # 通用 & 功能组件
│   │   ├── settings/    # 设置页子组件
│   │   └── upload/      # 上传组件
│   ├── i18n/            # 国际化语言包
│   │   ├── en-US/
│   │   └── zh-CN/
│   ├── layout/          # 布局组件
│   ├── router/          # 路由配置
│   ├── store/           # Pinia 状态管理
│   ├── styles/          # 全局样式
│   ├── utils/           # 工具函数（请求/上传）
│   ├── views/           # 页面视图
│   │   ├── admin/       # 管理后台
│   │   ├── auth/        # 登录/注册
│   │   ├── music/       # 音乐相关
│   │   └── user/        # 用户相关
│   ├── App.vue
│   └── main.ts
├── .env.example         # 环境变量模板
├── index.html
├── package.json
├── vite.config.ts
└── tsconfig.json
```

## 开发

```bash
# 在音乐城根目录下
make dev-fe

# 或在本目录下
pnpm install
pnpm dev
```

访问 `http://localhost:5173`，`/api` 请求自动代理到后端 `http://localhost:8080`。

## 构建

```bash
# 根目录一键构建（推荐）
make build

# 或单独构建前端
make build-fe

# 或在本目录下
pnpm build
```

产物输出到 `../cmd/server/dist/`，由 Go 后端编译时 embed 进单一二进制文件。

## Lint

```bash
make lint      # 根目录
pnpm eslint .  # 本目录
```
