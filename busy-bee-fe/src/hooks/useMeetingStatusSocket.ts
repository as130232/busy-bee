import { useEffect, useRef } from 'react'

import { auth } from '../services/firebase'

export interface MeetingStatusEvent {
  meetingId: string
  status: string
  errorMessage?: string
}

const RECONNECT_DELAYS_MS = [1000, 2000, 5000, 10000]
/** 分頁切到背景後，保留連線的寬限期（短暫切換分頁不斷線）。 */
export const HIDDEN_GRACE_MS = 60_000
/** 分頁可見但使用者無任何操作超過此時間，視為閒置並斷線。 */
export const IDLE_TIMEOUT_MS = 10 * 60_000

const ACTIVITY_EVENTS = ['pointerdown', 'keydown', 'scroll', 'touchstart'] as const

/**
 * 訂閱會議狀態事件。連線後第一則訊息帶 Firebase JWT（後端 ADR-002）；
 * 斷線自動以退避重連。onEvent / onResync 以 ref 持有，變動不會觸發重連。
 *
 * 省費用：Cloud Run 為以執行個體計費，只要 WS 連著 instance 就不會縮到 0。
 * 因此「沒在使用」時主動斷線且不重連——分頁背景超過 HIDDEN_GRACE_MS，
 * 或可見但無操作超過 IDLE_TIMEOUT_MS；使用者回來時重連並呼叫 onResync，
 * 讓呼叫端重新載入以補上斷線期間漏掉的狀態事件。
 */
export function useMeetingStatusSocket(
  onEvent: (e: MeetingStatusEvent) => void,
  onResync?: () => void,
) {
  const onEventRef = useRef(onEvent)
  onEventRef.current = onEvent
  const onResyncRef = useRef(onResync)
  onResyncRef.current = onResync

  useEffect(() => {
    let ws: WebSocket | null = null
    let attempt = 0
    let closed = false // unmount
    let paused = false // 閒置暫停
    let reconnectTimer: ReturnType<typeof setTimeout> | undefined
    let hiddenTimer: ReturnType<typeof setTimeout> | undefined
    let idleTimer: ReturnType<typeof setTimeout> | undefined

    const connect = () => {
      if (closed || paused) return
      // Firebase Hosting 不代理 WebSocket：production 直連 Cloud Run（VITE_WS_BASE）；
      // 本地開發走 Vite proxy（同源）。
      const proto = location.protocol === 'https:' ? 'wss' : 'ws'
      const base = import.meta.env.VITE_WS_BASE ?? `${proto}://${location.host}`
      const socket = new WebSocket(`${base}/api/v1/ws`)
      ws = socket

      socket.onopen = async () => {
        const fbUser = auth.currentUser
        if (!fbUser) {
          socket.close()
          return
        }
        const token = await fbUser.getIdToken()
        if (socket.readyState === WebSocket.OPEN) {
          socket.send(JSON.stringify({ type: 'auth', token }))
        }
      }

      socket.onmessage = (evt) => {
        try {
          const msg: unknown = JSON.parse(evt.data as string)
          if (typeof msg !== 'object' || msg === null) return
          const m = msg as Record<string, unknown>
          if (m.type === 'authOk') {
            attempt = 0 // 連線健康，重置退避
          } else if (
            m.type === 'meetingStatus' &&
            typeof m.meetingId === 'string' &&
            typeof m.status === 'string'
          ) {
            onEventRef.current({
              meetingId: m.meetingId,
              status: m.status,
              errorMessage: typeof m.errorMessage === 'string' ? m.errorMessage : undefined,
            })
          }
        } catch {
          // 非 JSON 或結構不符的訊息一律忽略
        }
      }

      socket.onclose = () => {
        if (ws === socket) ws = null
        if (closed || paused) return
        const delay = RECONNECT_DELAYS_MS[Math.min(attempt, RECONNECT_DELAYS_MS.length - 1)]
        attempt += 1
        reconnectTimer = setTimeout(connect, delay)
      }
    }

    const pause = () => {
      if (paused) return
      paused = true
      clearTimeout(reconnectTimer)
      clearTimeout(idleTimer)
      ws?.close()
      ws = null
    }

    const resume = () => {
      if (!paused) return
      paused = false
      attempt = 0
      connect()
      onResyncRef.current?.() // 補上暫停期間漏掉的事件
    }

    const armIdleTimer = () => {
      clearTimeout(idleTimer)
      idleTimer = setTimeout(pause, IDLE_TIMEOUT_MS)
    }

    const onActivity = () => {
      if (document.visibilityState !== 'visible') return
      resume()
      armIdleTimer()
    }

    const onVisibilityChange = () => {
      if (document.visibilityState === 'hidden') {
        clearTimeout(idleTimer)
        clearTimeout(hiddenTimer)
        hiddenTimer = setTimeout(pause, HIDDEN_GRACE_MS)
      } else {
        clearTimeout(hiddenTimer)
        onActivity()
      }
    }

    document.addEventListener('visibilitychange', onVisibilityChange)
    ACTIVITY_EVENTS.forEach((ev) => window.addEventListener(ev, onActivity, { passive: true }))

    if (document.visibilityState === 'hidden') {
      paused = true // 背景開啟（如 PWA 預載）不連線，等回到前景
    } else {
      connect()
      armIdleTimer()
    }

    return () => {
      closed = true
      clearTimeout(reconnectTimer)
      clearTimeout(hiddenTimer)
      clearTimeout(idleTimer)
      document.removeEventListener('visibilitychange', onVisibilityChange)
      ACTIVITY_EVENTS.forEach((ev) => window.removeEventListener(ev, onActivity))
      ws?.close()
    }
  }, [])
}
