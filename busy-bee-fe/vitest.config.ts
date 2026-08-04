import { defineConfig } from 'vitest/config'
import react from '@vitejs/plugin-react'

// 測試專用設定（與 vite.config 的 PWA/build 設定隔離）：jsdom 環境跑元件/hook 測試。
export default defineConfig({
  plugins: [react()],
  test: {
    environment: 'jsdom',
    globals: true,
    setupFiles: ['./src/test/setup.ts'],
    css: false,
  },
})
