# Music Online Web

Frontend SPA for the Music Online project, built with Vue 3, TypeScript and Element Plus.

## Technology Stack

- Vue 3
- TypeScript
- Element Plus

## Project Structure

```
.
├── public
├── src
│   ├── assets
│   ├── components
│   ├── i18n    # Internationalization configuration
│   ├── layout  # Layout components
│   ├── pages   # Page components
│   ├── router  # Router configuration
│   ├── store   # State management
│   ├── styles  # Global styles
│   ├── utils   # Utility functions
│   ├── views   # Page view components
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

## Development

```bash
pnpm install
pnpm dev
```

The app runs at `http://localhost:5173`.

## Build

```bash
pnpm build
```

The production bundle is emitted to `../cmd/server/dist` and served by the Go backend binary.
