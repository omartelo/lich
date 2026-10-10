import path from "node:path"
import { defineConfig } from "vitest/config"

// Standalone from vite.config.ts: most of the tested logic is pure, so a plain
// node environment is enough, with no need to drag the app's Vite plugins into
// the test runner. A suite that renders (the smoke, the render budgets, the
// component suites) asks for jsdom in its own docblock, which is why the
// default stays node: the suites that need a DOM pay for it, and the rest do not.
export default defineConfig({
  resolve: {
    alias: {
      "@": path.resolve(__dirname, "./src"),
    },
  },
  test: {
    environment: "node",
    // Parallel lich sessions each run this suite on the same CPU; vitest sizes
    // its pool as if it owned the machine, and four suites at once starve the
    // jsdom tests past their 5s timeout.
    maxWorkers: "50%",
    include: ["src/**/*.test.ts", "src/**/*.test.tsx"],
    coverage: {
      provider: "v8",
      reporter: ["text-summary", "json-summary"],
      reportsDirectory: "coverage",
      // Invariant #1, on the same number the CI summary reports.
      thresholds: { lines: 80 },
      // The denominator is the logic this suite can actually target: pure
      // modules under `lib` — stores, parsers, reducers, gates. Everything the
      // node environment cannot reach is excluded rather than counted as
      // uncovered, because the number is read in review (invariant #1) and a
      // denominator full of unreachable files reports a failing bar against a
      // suite that is doing its job.
      //
      // `include` alone does not do this: a file loaded by a test is measured
      // whether or not it matches, which is how the `.tsx` the comment here
      // once claimed to exclude ended up in the report anyway. The exclusions
      // below are what actually holds the line.
      include: ["src/**/*.ts"],
      exclude: [
        "src/**/*.test.ts",
        "src/**/*.d.ts",
        // Test utilities. Measuring the harness that measures the app says
        // nothing about the app.
        "src/test/**",
        // Components: the DOM/xterm boundary invariant #1 exempts. Pulled in
        // transitively by the tests that import their pure helpers.
        "src/**/*.tsx",
        // React hooks. The store behind each hook is what carries the tests;
        // the render budgets run a few of them for real but measure repaints,
        // not branches, so counting the hooks as covered would be a number
        // nobody earned. `useDiffEditor`/`useFileEditor` are hooks too, misfiled
        // under components because they own a CodeMirror view.
        "src/**/use-*.ts",
        "src/components/**/use*.ts",
        // OS/framework boundaries: fetch, EventSource, WebSocket, CodeMirror
        // wiring, the xterm privates and font loading a live terminal needs, and
        // the DEV-only paint reporter that production drops.
        "src/lib/rpc.ts",
        "src/lib/app-events.ts",
        "src/lib/codemirror.ts",
        "src/lib/terminal/term-transport.ts",
        "src/lib/terminal/term-view.ts",
        "src/lib/terminal/term-perf.ts",
      ],
    },
  },
})
