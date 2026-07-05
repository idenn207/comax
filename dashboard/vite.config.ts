import path from 'node:path';
import { fileURLToPath } from 'node:url';
import react from '@vitejs/plugin-react';
import { defineConfig } from 'vite';

// __dirname shim — vite.config.ts runs as an ESM module.
const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);

// Vite is configured to write its production bundle directly into the
// Go embed dir (internal/server/dashboard/dist) so `go build -tags
// embed_dashboard` picks the latest SPA up without a copy step. The
// dev server proxies /api + /healthz to the Go process running on 8080.
// outDir is one level up now that the SPA lives at ./dashboard (was ./web/dashboard).
export default defineConfig({
  plugins: [react()],
  resolve: {
    alias: {
      '@': path.resolve(__dirname, 'src'),
    },
  },
  build: {
    outDir: path.resolve(__dirname, '../internal/server/dashboard/dist'),
    emptyOutDir: true,
    sourcemap: false,
    target: 'es2022',
    // Vite hashes asset filenames so the immutable cache header on the
    // Go side is safe.
    assetsDir: 'assets',
  },
  server: {
    port: 5173,
    strictPort: true,
    // Proxy API traffic to the Go process. changeOrigin stays false so
    // cookies preserve their original host (the browser thinks it is
    // talking to localhost:5173, the Go server sees the same).
    proxy: {
      '/api': {
        target: 'http://localhost:8080',
        changeOrigin: false,
      },
      '/healthz': {
        target: 'http://localhost:8080',
        changeOrigin: false,
      },
    },
  },
});
