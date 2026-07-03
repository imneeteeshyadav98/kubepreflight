import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

// base: "/console/" matches the mount point internal/reportserver registers
// for the embedded build (web/embed.go -> web/dist), so built asset URLs
// resolve correctly when served from that subpath instead of "/".
export default defineConfig({
  base: "/console/",
  plugins: [react()],
  build: {
    outDir: "dist",
    emptyOutDir: true,
  },
  test: {
    environment: "jsdom",
    globals: true,
    setupFiles: "./src/test/setup.ts",
  },
});
