import vue from "@vitejs/plugin-vue";
import { fileURLToPath, URL } from "node:url";
import { defineConfig } from "vitest/config";

export default defineConfig({
  plugins: [vue()],
  resolve: {
    alias: {
      "@": fileURLToPath(new URL("./src", import.meta.url)),
    },
  },
  test: {
    clearMocks: true,
    coverage: {
      exclude: ["src/**/*.d.ts", "src/**/__tests__/**", "src/main.ts"],
      include: ["src/**/*.{ts,vue}"],
      reporter: ["text", "json-summary"],
    },
    environment: "happy-dom",
    include: ["src/**/*.test.ts"],
    globals: false,
  },
});
