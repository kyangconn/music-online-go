# Music Online Pages

Frontend SPA for Music Online — Vue 3 + TypeScript + Element Plus.

## Technology Stack

- Vue 3 + TypeScript
- Element Plus component library
- Pinia state management + Vue Router
- vue-i18n (Chinese / English)
- Vite build tool

## Project Structure

```
.
├── public/
│   ├── fonts/           # Font assets
│   └── icons/           # Favicon
├── src/
│   ├── assets/          # Static assets
│   ├── components/      # Shared & feature components
│   │   ├── settings/    # Settings sub-components
│   │   └── upload/      # Upload components
│   ├── i18n/            # Locale files
│   │   ├── en-US/
│   │   └── zh-CN/
│   ├── layout/          # Layout components
│   ├── router/          # Route config
│   ├── store/           # Pinia stores
│   ├── styles/          # Global styles
│   ├── utils/           # Utilities (request/upload)
│   ├── views/           # Page views
│   │   ├── admin/       # Admin dashboard
│   │   ├── auth/        # Login/Register
│   │   ├── music/       # Music pages
│   │   └── user/        # User pages
│   ├── App.vue
│   └── main.ts
├── .env.example         # Environment template
├── index.html
├── package.json
├── vite.config.ts
└── tsconfig.json
```

## Development

```bash
# From project root
make dev-fe

# Or within this directory
pnpm install
pnpm dev
```

Open `http://localhost:5173`. `/api` requests are proxied to `http://localhost:8080`.

## Build

```bash
# One-shot from root (recommended)
make build

# Frontend only
make build-fe

# Or within this directory
pnpm build
```

Output goes to `../cmd/server/dist/` and is embedded into the Go binary at compile time.

## Lint

```bash
make lint      # From root
pnpm eslint .  # Within this directory
```
