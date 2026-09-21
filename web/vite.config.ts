import { defineConfig } from 'vite';
import vue from '@vitejs/plugin-vue';

export default defineConfig({
  root: 'web',
  base: '/web/assets/',
  plugins: [vue()],
  build: {
    outDir: '../internal/api/http/playground/assets/web',
    emptyOutDir: true,
    rollupOptions: {
      input: {
        main: new URL('./index.html', import.meta.url).pathname,
        'cache-maintenance-overlay': new URL('./src/cache-maintenance-overlay.ts', import.meta.url).pathname,
      },
      output: {
        entryFileNames: (chunk) =>
          chunk.name === 'cache-maintenance-overlay'
            ? 'assets/cache-maintenance-overlay.js'
            : 'assets/[name]-[hash].js',
      },
    },
  },
});
