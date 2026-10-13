import { defineConfig } from "vitest/config";
import react from "@vitejs/plugin-react";

// En desarrollo, Vite sirve el frontend en :5173 y reenvía /api al backend en :8080.
// Así el navegador ve UN solo origen: no hace falta CORS y la cookie funciona igual
// que en producción.
export default defineConfig({
  plugins: [react()],
  server: {
    port: 5173,
    proxy: { "/api": "http://localhost:8080" },
  },
  build: { outDir: "dist", sourcemap: false },
  test: { environment: "node" },
});
