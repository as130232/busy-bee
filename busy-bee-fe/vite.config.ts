import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'
import { VitePWA } from 'vite-plugin-pwa'

// https://vite.dev/config/
export default defineConfig({
  plugins: [
    react(),
    tailwindcss(),
    VitePWA({
      strategies: 'injectManifest',
      srcDir: 'src',
      filename: 'sw.ts',
      registerType: 'autoUpdate',
      devOptions: { enabled: true, type: 'module' }, // 本地可測推播
      manifest: {
        name: 'Busy Bee',
        short_name: 'Busy Bee',
        description: '開會錄音 → AI 生成 PRD / Tech Spec',
        theme_color: '#0e0e11',
        background_color: '#0e0e11',
        display: 'standalone',
        start_url: '/',
        icons: [
          { src: '/icon-192.png', sizes: '192x192', type: 'image/png', purpose: 'any maskable' },
          { src: '/icon-512.png', sizes: '512x512', type: 'image/png', purpose: 'any maskable' },
        ],
        // Android/桌面 Chrome：從其他 App「分享」到 Busy Bee 時，帶著連結開在 /，
        // 由 useShareImport 讀 url/text 參數自動匯入（iOS Safari 不支援，故另有剪貼簿貼上）。
        share_target: {
          action: '/',
          method: 'GET',
          params: { title: 'title', text: 'text', url: 'url' },
        },
      },
    }),
  ],
  server: {
    proxy: {
      '/api': { target: 'http://localhost:8080', ws: true },
      '/health': 'http://localhost:8080',
    },
  },
})
