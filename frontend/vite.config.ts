import { fileURLToPath, URL } from 'node:url';
import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';

export default defineConfig({
  plugins: [react()],
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url)),
    },
  },
  server: {
    proxy: {
      '/api/v1/panel': {
        target: 'http://localhost:8081',
        changeOrigin: true,
      },
      '/api/v1/runtime': {
        target: 'http://localhost:8082',
        changeOrigin: true,
      },
      '/api/v1/analytics': {
        target: 'http://localhost:8083',
        changeOrigin: true,
      },
    },
  },
});
