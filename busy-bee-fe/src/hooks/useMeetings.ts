import { useCallback, useEffect, useState } from 'react'

import { listMeetings, type Meeting } from '../services/api/client'
import { getIdToken } from '../services/token'
import { useMeetingStatusSocket } from './useMeetingStatusSocket'

/** 共用會議列表資料：載入 + 搜尋防抖 + WebSocket 即時狀態更新。 */
export function useMeetings(search = '') {
  const [meetings, setMeetings] = useState<Meeting[]>([])
  const [error, setError] = useState<string | null>(null)

  const load = useCallback(async (keyword: string) => {
    try {
      const { meetings } = await listMeetings(await getIdToken(), keyword)
      setMeetings(meetings)
      setError(null)
    } catch (e) {
      setError(e instanceof Error ? e.message : '會議列表載入失敗')
    }
  }, [])

  useEffect(() => {
    const timer = setTimeout(() => void load(search), search ? 300 : 0) // 搜尋防抖
    return () => clearTimeout(timer)
  }, [search, load])

  useMeetingStatusSocket((e) => {
    setMeetings((prev) =>
      prev.map((m) =>
        m.id === e.meetingId
          ? { ...m, status: e.status as Meeting['status'], errorMessage: e.errorMessage }
          : m,
      ),
    )
  })

  return { meetings, error, reload: () => void load(search) }
}
