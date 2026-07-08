import path from "node:path"
import { defineConfig } from "vitest/config"

// Standalone from vite.config.ts: the Wails runtime plugin needs the desktop
// bridge, which is absent under the test runner. Session logic is pure, so a
// plain node environment is enough.
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
