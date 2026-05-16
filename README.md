# Music Online Pages

这是 Music Online 项目的 Web 前端页面，一个我出思路，AI实现的，基于 Vue 3 + TypeScript + Element Plus 的前端单页应用，用于 Music Online 项目的 Web 界面。

## 技术栈

- Vue 3
- TypeScript
- Element Plus

## 项目结构

```
.
├── public
├── src
│   ├── assets
│   ├── components
│   ├── i18n    # 国际化配置
│   ├── layout  # 布局组件
│   ├── pages   # 页面组件
│   ├── router  # 路由配置
│   ├── store   # 状态管理
│   ├── styles  # 全局样式
│   ├── utils   # 工具函数
│   ├── views   # 页面视图组件
│   ├── App.vue
│   ├── main.ts
├── .env
├── .gitignore
├── index.html
├── package.json
├── README.md
├── tsconfig.json
└── vite.config.ts
```

## 开发

```bash
pnpm install
pnpm dev
```

访问 `http://localhost:5173`。

## 构建

```bash
pnpm build
```

构建产物默认输出到 `../cmd/server/dist`，由后端 Go 服务嵌入并对外提供。
