import { defineConfig } from 'vite';
import vue from '@vitejs/plugin-vue';

export default defineConfig({
  root: 'web',
  base: '/web/assets/',
  plugins: [vue()],
  build: { outDir: '../internal/api/http/playground/assets/web', emptyOutDir: true },
});
