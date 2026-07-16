import { defineConfig } from 'vitest/config';
import path from 'node:path';

export default defineConfig({
  resolve: {
    alias: {
      '@': path.resolve(__dirname, '../'), // __tests__ sibling of app/src root
      // vite-plugin-pwa's virtual module doesn't exist without the plugin;
      // point it at a stub so files importing it can be unit-tested.
      'virtual:pwa-register': path.resolve(__dirname, 'unit/mocks/virtual-pwa-register.ts')
    }
  },
  test: {
    include: ['src/__tests__/unit/**/*.{test,spec}.{ts,tsx}'],
    exclude: [
      'node_modules',
      'dist',
      'tests',
      '__tests__/e2e',
    ],
    environment: 'jsdom',
    setupFiles: ['src/__tests__/unit/setupTests.ts'],
    globals: true,
  },
});