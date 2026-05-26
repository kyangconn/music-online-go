import css from "@eslint/css"
import js from "@eslint/js"
import eslintConfigPrettier from "eslint-config-prettier"
import perfectionist from "eslint-plugin-perfectionist"
import prettierPlugin from "eslint-plugin-prettier"
import pluginVue from "eslint-plugin-vue"
import { defineConfig } from "eslint/config"
import globals from "globals"
import tseslint from "typescript-eslint"

export default defineConfig([
  // ── Global ignores ──────────────────────────────────────────────
  {
    ignores: ["**/node_modules/**", "dist/", "build/", "public/", "env.d.ts", ".vite/", ".cache/"],
  },
  // ── JS / TS base (recommended) ─────────────────────────────────
  {
    files: ["**/*.{js,mjs,cjs,ts,mts,cts,vue}"],
    plugins: { js },
    languageOptions: { globals: globals.browser },
  },
  // ── TypeScript recommended ─────────────────────────────────────
  ...tseslint.configs.recommended,

  // ── Vue essential ──────────────────────────────────────────────
  pluginVue.configs["flat/essential"],
  {
    files: ["**/*.vue"],
    languageOptions: { parserOptions: { parser: tseslint.parser } },
  },

  // ── CSS recommended ────────────────────────────────────────────
  {
    files: ["**/*.css"],
    plugins: { css },
    extends: ["css/recommended"],
  },

  // ── Prettier integration ───────────────────────────────────────
  // Disable ESLint rules that conflict with Prettier
  eslintConfigPrettier,
  // Then re-enable Prettier as an ESLint error
  {
    plugins: {
      prettier: prettierPlugin,
    },
    rules: {
      "prettier/prettier": "error",
    },
  },

  // ── Code quality rules ─────────────────────────────────────────
  {
    rules: {
      "no-console": ["warn", { allow: ["warn", "error", "info"] }],
      "no-debugger": "warn",

      "@typescript-eslint/no-unused-vars": [
        "warn",
        {
          argsIgnorePattern: "^_",
          varsIgnorePattern: "^_",
          caughtErrorsIgnorePattern: "^_",
        },
      ],
      "@typescript-eslint/no-explicit-any": [
        "warn",
        {
          fixToUnknown: false,
          ignoreRestArgs: true,
        },
      ],
      "@typescript-eslint/explicit-module-boundary-types": "off",
    },
  },

  // ── Vue-specific rules ─────────────────────────────────────────
  {
    rules: {
      // Element Plus 场景下合理关闭
      "vue/multi-word-component-names": "off",
      "vue/require-default-prop": "off",
      "vue/v-on-event-hyphenation": "off",
      "vue/block-lang": "off",

      // XSS 防护 — v-html 需要显式确认安全
      "vue/no-v-html": "warn",

      // 提高代码质量的检查
      "vue/no-unused-components": "warn",
      "vue/no-useless-v-bind": "warn",
      "vue/no-useless-mustaches": "warn",
      "vue/prefer-true-attribute-shorthand": "warn",
      "vue/no-unused-vars": "warn",
    },
  },

  // ── Auto-sort imports ──────────────────────────────────────────
  {
    plugins: {
      perfectionist,
    },
    rules: {
      "perfectionist/sort-imports": [
        "warn",
        {
          type: "natural",
          order: "asc",
          newlinesBetween: "ignore",
        },
      ],
    },
  },
])
