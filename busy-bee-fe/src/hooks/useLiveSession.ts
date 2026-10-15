import { useEffect, useState, useSyncExternalStore } from 'react'

import { defaultSessionDeps } from '../live/defaults'
import { LiveSession } from '../live/session'

/** 即時會議 session（F-LIVE）：一個頁面一個 session，卸載時停止一切（本機錄音備份保留供救回）。 */
export function useLiveSession() {
  const [session] = useState(() => new LiveSession(defaultSessionDeps()))
  const state = useSyncExternalStore(session.subscribe, session.getState)
  useEffect(() => () => session.dispose(), [session])

  // 會議進行中離開頁面 → 原生「資料未儲存」警告
  const active = state.phase === 'live' || state.phase === 'finishing' || state.phase === 'uploading'
  useEffect(() => {
    if (!active) return
    const warn = (e: BeforeUnloadEvent) => e.preventDefault()
    window.addEventListener('beforeunload', warn)
    return () => window.removeEventListener('beforeunload', warn)
  }, [active])

  // 會議中持有 Wake Lock，避免筆電螢幕休眠
  useEffect(() => {
    if (state.phase !== 'live' || !('wakeLock' in navigator)) return
    let lock: WakeLockSentinel | null = null
    let cancelled = false
    navigator.wakeLock.request('screen').then(
      (l) => {
        if (cancelled) void l.release().catch(() => {})
        else lock = l
      },
      () => {},
    )
    return () => {
      cancelled = true
      void lock?.release().catch(() => {})
    }
  }, [state.phase])

  return { session, state }
}
