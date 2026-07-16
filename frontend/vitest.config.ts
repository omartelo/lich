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
    coverage: {
      provider: "v8",
      reporter: ["text-summary", "json-summary"],
      reportsDirectory: "coverage",
      // The suite owns the pure logic layer (`.ts`: lib, hooks, stores); the
      // `.tsx` components are the DOM/xterm boundary invariant #1 exempts, so
      // they stay out of the denominator — including them would report a
      // misleadingly low number against a suite that never targeted them.
      include: ["src/**/*.ts"],
      exclude: ["src/**/*.test.ts", "src/**/*.d.ts"],
    },
  },
})
