import { defineConfig } from 'vite';
import path from 'node:path';
import { svelte } from '@sveltejs/vite-plugin-svelte';
import tailwindcss from '@tailwindcss/vite';

export default defineConfig({
  plugins: [
    tailwindcss(),
    svelte(),
  ],
  resolve: {
    alias: {
      '@': path.resolve(process.cwd(), 'src'),
    },
  },
  build: {
    outDir: '../cmd/webui/web/dist',
    emptyOutDir: true,
    // Single-chunk output for simple embedding: everything ships as one
    // hashed JS/CSS pair that go:embed inlines into the binary, so keep the
    // warning limit above the actual app chunk size.
    chunkSizeWarningLimit: 600,
    rollupOptions: {
      output: {
        entryFileNames: 'assets/[name]-[hash].js',
        chunkFileNames: 'assets/[name]-[hash].js',
        assetFileNames: 'assets/[name]-[hash].[ext]',
      },
    },
  },
  server: {
    proxy: {
      '/api': 'http://localhost:8021',
      '/login': 'http://localhost:8021',
      '/logout': 'http://localhost:8021',
    },
  },
});
