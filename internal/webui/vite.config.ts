import { defineConfig } from "vite";

export default defineConfig({
  root: ".",
  build: {
    outDir: "dist",
    emptyOutDir: true,
  },
  server: {
    proxy: {
      "/ui/api": {
        target: "https://localhost:8444",
        secure: false,
        changeOrigin: true,
      },
    },
  },
});
