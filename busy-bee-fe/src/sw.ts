/// <reference lib="webworker" />
declare const self: ServiceWorkerGlobalScope

import { precacheAndRoute } from 'workbox-precaching'

precacheAndRoute(self.__WB_MANIFEST)

self.addEventListener('push', (event) => {
  // [push-debug] 用 Mac 接線的 Safari 遠端除錯器看這些 log，確認 iPhone 是否收到 push 事件。
  console.log('[push-debug] push event received, hasData=', !!event.data)
  let data: { title?: string; body?: string; url?: string } = {}
  try {
    data = event.data?.json() ?? {}
  } catch {
    // 非 JSON payload 忽略內容，仍顯示通知
    console.warn('[push-debug] payload not JSON, showing fallback title')
  }
  console.log('[push-debug] title=', data.title)
  event.waitUntil(
    self.registration
      .showNotification(data.title ?? 'Busy Bee', {
        body: data.body ?? '',
        icon: '/icon-192.png',
        badge: '/icon-192.png',
        data: { url: data.url ?? '/' },
      })
      .then(() => console.log('[push-debug] showNotification resolved'))
      .catch((err) => console.error('[push-debug] showNotification failed', err)),
  )
})

self.addEventListener('notificationclick', (event) => {
  event.notification.close()
  const url = (event.notification.data as { url?: string } | undefined)?.url ?? '/'
  event.waitUntil(
    (async () => {
      // 已開啟本站分頁時，導向目標並聚焦，避免每次點通知都開新視窗
      const wins = await self.clients.matchAll({ type: 'window', includeUncontrolled: true })
      const existing = wins.find((w) => new URL(w.url).origin === self.location.origin)
      if (existing) {
        await existing.navigate(url)
        await existing.focus()
        return
      }
      await self.clients.openWindow(url)
    })(),
  )
})
