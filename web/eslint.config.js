import { defineConfigWithVueTs, vueTsConfigs } from "@vue/eslint-config-typescript"
import eslintConfigPrettier from "eslint-config-prettier"
import perfectionist from "eslint-plugin-perfectionist"
import prettierPlugin from "eslint-plugin-prettier"
import pluginVue from "eslint-plugin-vue"

// Global ignores
const ignores = {
  ignores: ["**/node_modules/**", "dist/", "build/", "public/", "env.d.ts", ".vite/", ".cache/"],
}

// Prettier integration — reports formatting issues as ESLint errors
const prettier = {
  plugins: {
    prettier: prettierPlugin,
  },
  rules: {
    "prettier/prettier": "error",
  },
}

// Code quality rules
const base = {
  rules: {
    "no-console": ["warn", { allow: ["warn", "error", "info"] }],
    "no-debugger": "warn",

    // TypeScript — warn on `any`, encourage gradual typing
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
}

// Vue-specific rules
const vue = {
  rules: {
    // Element Plus 场景下合理关闭
    "vue/multi-word-component-names": "off",
    "vue/require-default-prop": "off",
    "vue/v-on-event-hyphenation": "off",
    "vue/block-lang": "off",

    // XSS 防护 — v-html 需要显式确认安全
    "vue/no-v-html": "warn",

    // 启用这些检查，提高代码质量
    "vue/no-unused-components": "warn",
    "vue/no-useless-v-bind": "warn",
    "vue/no-useless-mustaches": "warn",
    "vue/prefer-true-attribute-shorthand": "warn",
    "vue/no-unused-vars": "warn",
  },
}

// Auto-sort imports
const importOrder = {
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
}

// Export all configs
export default defineConfigWithVueTs(
  ignores,
  pluginVue.configs["flat/essential"],
  vueTsConfigs.recommended,
  // disable ESLint rules that conflict with Prettier
  eslintConfigPrettier,
  // custom rules (prettier/prettier: error re-enables Prettier reporting)
  prettier,
  base,
  vue,
  importOrder,
)
