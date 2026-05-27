import css from "@eslint/css"
import js from "@eslint/js"
import perfectionist from "eslint-plugin-perfectionist"
import pluginVue from "eslint-plugin-vue"
import { defineConfig } from "eslint/config"
import globals from "globals"
import tseslint from "typescript-eslint"

export default defineConfig([
  // ── Global ignores ──────────────────────────────────────────────
  {
    ignores: ["**/node_modules/**", "dist/", "build/", "public/", "env.d.ts", ".vite/", ".cache/"],
  },
  // ── JS / TS base ────────────────────────────────────────────────
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

  // ── Code quality ───────────────────────────────────────────────
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

  // ── Vue-specific ───────────────────────────────────────────────
  {
    rules: {
      "vue/multi-word-component-names": "off",
      "vue/require-default-prop": "off",
      "vue/v-on-event-hyphenation": "off",
      "vue/block-lang": "off",

      "vue/no-v-html": "warn",
      "vue/no-unused-components": "warn",
      "vue/no-useless-v-bind": "warn",
      "vue/no-useless-mustaches": "warn",
      "vue/prefer-true-attribute-shorthand": "warn",
      "vue/no-unused-vars": "warn",
    },
  },

  // ── Import order ───────────────────────────────────────────────
  {
    plugins: { perfectionist },
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
