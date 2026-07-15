import path from "node:path"
import { defineConfig } from "vitest/config"

// Standalone from vite.config.ts: the tested logic is pure, so a plain node
// environment is enough — no need to drag the app's Vite plugins into the
// test runner.
export default defineConfig({
  resolve: {
    alias: {
      "@": path.resolve(__dirname, "./src"),
    },
  },
  test: {
    environment: "node",
    include: ["src/**/*.test.ts"],
  },
})
