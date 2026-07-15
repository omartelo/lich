import path from "node:path";
import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";
import wails from "@wailsio/runtime/plugins/vite";

// https://vitejs.dev/config/
export default defineConfig({
  server: {
    host: "127.0.0.1",
    port: Number(process.env.WAILS_VITE_PORT) || 9245,
    strictPort: true,
  },
  resolve: {
    alias: {
      "@": path.resolve(__dirname, "./src"),
    },
  },
  build: {
    rollupOptions: {
      input: {
        // spike.html is the Chromium shell spike window (cmd/spike); see
        // docs/chromium-shell.md. Remove with it.
        index: path.resolve(__dirname, "index.html"),
        spike: path.resolve(__dirname, "spike.html"),
      },
    },
  },
  plugins: [react(), tailwindcss(), wails("./bindings")],
});
