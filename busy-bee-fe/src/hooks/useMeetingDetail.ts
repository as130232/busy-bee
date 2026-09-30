import { useCallback, useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'

import {
  deleteActionItem,
  deleteMeeting,
  editActionItem,
  getMeeting,
  listArtifacts,
  listMeetingActionItems,
  retryMeeting,
  toggleActionItem,
  type ActionItem,
  type Artifact,
  type MeetingDetail,
} from '../services/api/client'
import { useApiCall } from './useApiCall'
import { useMeetingStatusSocket } from './useMeetingStatusSocket'

/**
 * 會議詳情頁的資料層：載入詳情/文件/待辦、狀態事件重載、待辦與刪除的操作。
 * 由 MeetingDetailPage 拆分而來（原內嵌於元件），集中資料流並改用 useApiCall 注入 token。
 */
export function useMeetingDetail(id: string | undefined) {
  const navigate = useNavigate()
  const call = useApiCall()
  const [meeting, setMeeting] = useState<MeetingDetail | null>(null)
  const [artifacts, setArtifacts] = useState<Artifact[]>([])
  const [actionItems, setActionItems] = useState<ActionItem[]>([])
  const [error, setError] = useState<string | null>(null)

  const load = useCallback(async () => {
    if (!id) return
    try {
      const m = await call(getMeeting, id)
      setMeeting(m.meeting)
      // 排程會議尚無文件/行動項，不多打兩支 API
      if (m.meeting.status !== 'scheduled') {
        const [a, ai] = await Promise.all([call(listArtifacts, id), call(listMeetingActionItems, id)])
        setArtifacts(a.artifacts)
        setActionItems(ai.actionItems)
      }
      setError(null)
    } catch (e) {
      setError(e instanceof Error ? e.message : '載入失敗')
    }
  }, [id, call])

  useEffect(() => {
    void load()
  }, [load])

  // 本會議狀態變更時重新載入（完成時文件才會出現）
  // 閒置斷線後回來：重載補上漏掉的狀態
  useMeetingStatusSocket((e) => {
    if (e.meetingId === id) void load()
  }, () => void load())

  const retry = useCallback(async () => {
    if (!id) return
    try {
      await call(retryMeeting, id)
      void load()
    } catch (e) {
      setError(e instanceof Error ? e.message : '重試失敗')
    }
  }, [id, call, load])

  // 刪除會議（任何狀態）→ 回會議列表；失敗回傳 false 供頁面關閉確認框。
  const remove = useCallback(async (): Promise<boolean> => {
    if (!id) return false
    try {
      await call(deleteMeeting, id)
      navigate('/meetings')
      return true
    } catch (e) {
      setError(e instanceof Error ? e.message : '刪除失敗')
      return false
    }
  }, [id, call, navigate])

  const toggleItem = useCallback(async (itemId: string, done: boolean) => {
    setActionItems((prev) => prev.map((it) => (it.id === itemId ? { ...it, done } : it)))
    try {
      await call(toggleActionItem, itemId, done)
    } catch {
      void load() // 失敗回滾
    }
  }, [call, load])

  // 修改待辦內容：以伺服器回傳結果更新清單（失敗由 ActionItemRow 還原輸入）。
  const editItem = useCallback(async (itemId: string, description: string) => {
    const { actionItem } = await call(editActionItem, itemId, description)
    setActionItems((prev) => prev.map((it) => (it.id === itemId ? actionItem : it)))
  }, [call])

  // 移除待辦：樂觀移除，失敗則重載還原。
  const removeItem = useCallback(async (itemId: string) => {
    setActionItems((prev) => prev.filter((it) => it.id !== itemId))
    try {
      await call(deleteActionItem, itemId)
    } catch {
      void load()
    }
  }, [call, load])

  return {
    meeting,
    setMeeting,
    artifacts,
    actionItems,
    setActionItems,
    error,
    retry,
    remove,
    toggleItem,
    editItem,
    removeItem,
  }
}
