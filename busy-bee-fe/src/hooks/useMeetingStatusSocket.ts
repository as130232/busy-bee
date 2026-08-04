import { useEffect, useRef } from 'react'

import { auth } from '../services/firebase'

export interface MeetingStatusEvent {
  meetingId: string
  status: string
  errorMessage?: string
}

const RECONNECT_DELAYS_MS = [1000, 2000, 5000, 10000]

/**
 * 訂閱會議狀態事件。連線後第一則訊息帶 Firebase JWT（後端 ADR-002）；
 * 斷線自動以退避重連。onEvent 以 ref 持有，變動不會觸發重連。
 */
export function useMeetingStatusSocket(onEvent: (e: MeetingStatusEvent) => void) {
  const onEventRef = useRef(onEvent)
  onEventRef.current = onEvent

  useEffect(() => {
    let ws: WebSocket | null = null
    let attempt = 0
    let closed = false
    let reconnectTimer: ReturnType<typeof setTimeout> | undefined

    const connect = () => {
      if (closed) return
      // Firebase Hosting 不代理 WebSocket：production 直連 Cloud Run（VITE_WS_BASE）；
      // 本地開發走 Vite proxy（同源）。
      const proto = location.protocol === 'https:' ? 'wss' : 'ws'
      const base = import.meta.env.VITE_WS_BASE ?? `${proto}://${location.host}`
      ws = new WebSocket(`${base}/api/v1/ws`)

      ws.onopen = async () => {
        const fbUser = auth.currentUser
        if (!fbUser) {
          ws?.close()
          return
        }
        const token = await fbUser.getIdToken()
        ws?.send(JSON.stringify({ type: 'auth', token }))
      }

      ws.onmessage = (evt) => {
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

      ws.onclose = () => {
        if (closed) return
        const delay = RECONNECT_DELAYS_MS[Math.min(attempt, RECONNECT_DELAYS_MS.length - 1)]
        attempt += 1
        reconnectTimer = setTimeout(connect, delay)
      }
    }

    connect()
    return () => {
      closed = true
      clearTimeout(reconnectTimer)
      ws?.close()
    }
  }, [])
}
