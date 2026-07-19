/// <reference lib="webworker" />
declare const self: ServiceWorkerGlobalScope

import { precacheAndRoute } from 'workbox-precaching'

precacheAndRoute(self.__WB_MANIFEST)

self.addEventListener('push', (event) => {
  let data: { title?: string; body?: string; url?: string } = {}
  try {
    data = event.data?.json() ?? {}
  } catch {
    // 非 JSON payload 忽略內容，仍顯示通知
  }
  event.waitUntil(
    self.registration.showNotification(data.title ?? 'Busy Bee', {
      body: data.body ?? '',
      icon: '/icon-192.png',
      badge: '/icon-192.png',
      data: { url: data.url ?? '/' },
    }),
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
