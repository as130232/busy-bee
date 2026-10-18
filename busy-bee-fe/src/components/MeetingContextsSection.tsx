import { useCallback, useEffect, useState } from 'react'
import { FileText } from 'lucide-react'

import { useApiCall } from '../hooks/useApiCall'
import {
  addMeetingContextFile,
  addMeetingContextText,
  deleteMeetingContext,
  listMeetingContexts,
  type MeetingContext,
} from '../services/api/client'
import { CollapsibleSection } from './CollapsibleSection'
import { ContextPicker } from './live/ContextPicker'

/**
 * 紀錄的會前資料（F-LIVE）：排程時預先準備，或會後補上供 AI 校正與重新整理參考。
 * bare=true 時不包可收合卡片（放進詳情頁「⋯」選單的 Sheet 內使用）。
 */
export function MeetingContextsSection({
  meetingId,
  hint,
  bare = false,
}: {
  meetingId: string
  hint: string
  bare?: boolean
}) {
  const call = useApiCall()
  const [contexts, setContexts] = useState<MeetingContext[]>([])
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const load = useCallback(async () => {
    try {
      setContexts((await call(listMeetingContexts, meetingId)).contexts)
    } catch {
      /* 列表讀不到不影響其他功能 */
    }
  }, [call, meetingId])

  useEffect(() => {
    void load()
  }, [load])

  const run = async (fn: () => Promise<{ context: MeetingContext }>) => {
    setBusy(true)
    setError(null)
    try {
      const { context } = await fn()
      setContexts((cs) => [...cs, context])
    } catch (e) {
      setError(e instanceof Error ? e.message : '新增失敗')
    } finally {
      setBusy(false)
    }
  }

  const remove = async (id: string) => {
    try {
      await call(deleteMeetingContext, meetingId, id)
      setContexts((cs) => cs.filter((c) => c.id !== id))
    } catch (e) {
      setError(e instanceof Error ? e.message : '刪除失敗')
    }
  }

  const body = (
    <>
      <p className="m-0 mb-2 text-xs text-muted">{hint}</p>
      <ContextPicker
        items={contexts.map((c) => ({ key: c.id, title: c.title, detail: `${c.chars.toLocaleString()} 字` }))}
        busy={busy}
        error={error}
        onAddText={(t) => void run(() => call(addMeetingContextText, meetingId, t))}
        onAddFile={(f) => void run(() => call(addMeetingContextFile, meetingId, f))}
        onRemove={(id) => void remove(id)}
      />
    </>
  )
  if (bare) return body

  return (
    <CollapsibleSection
      title="會前資料"
      icon={<FileText className="size-4" />}
      count={contexts.length}
      defaultOpen={contexts.length > 0}
    >
      {body}
    </CollapsibleSection>
  )
}
