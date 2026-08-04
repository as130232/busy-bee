import { useState } from 'react'
import { Plus } from 'lucide-react'

import { addMeetingActionItem, type ActionItem } from '../services/api/client'
import { useApiCall } from '../hooks/useApiCall'

/** 手動新增待辦：輸入 + 指派人 + 送出（POST /meetings/:id/action-items），成功後回呼交父層插入清單。 */
export function MeetingTodoForm({
  meetingId,
  speakerNames,
  speakerOrder,
  onAdded,
}: {
  meetingId: string
  speakerNames: Record<string, string>
  speakerOrder: string[]
  onAdded: (item: ActionItem) => void
}) {
  const [text, setText] = useState('')
  const [assignee, setAssignee] = useState('')
  const [busy, setBusy] = useState(false)
  const call = useApiCall()

  const submit = async () => {
    const desc = text.trim()
    if (!desc || busy) return
    setBusy(true)
    try {
      const { actionItem } = await call(addMeetingActionItem, meetingId, desc, assignee)
      onAdded(actionItem)
      setText('')
      setAssignee('')
    } catch {
      // 失敗保留輸入讓使用者重試
    } finally {
      setBusy(false)
    }
  }

  return (
    <form
      className="flex flex-col gap-2"
      onSubmit={(e) => {
        e.preventDefault()
        void submit()
      }}
    >
      <div className="flex items-center gap-2">
        <input
          type="text"
          value={text}
          onChange={(e) => setText(e.target.value)}
          placeholder="新增待辦…"
          className="min-w-0 flex-1 rounded-lg border border-border bg-bg px-3 py-2 text-sm text-fg outline-none focus:border-accent"
        />
        <button
          type="submit"
          disabled={!text.trim() || busy}
          aria-label="新增待辦"
          className="btn btn-primary size-9 shrink-0 px-0 disabled:opacity-40"
        >
          <Plus className="size-4" />
        </button>
      </div>
      {speakerOrder.length > 0 && (
        <label className="flex items-center gap-2 text-xs text-muted">
          指派給
          <select
            value={assignee}
            onChange={(e) => setAssignee(e.target.value)}
            className="rounded-lg border border-border bg-bg px-2 py-1 text-sm text-fg outline-none focus:border-accent"
          >
            <option value="">不指派</option>
            {speakerOrder.map((code) => (
              <option key={code} value={code}>
                {speakerNames[code]?.trim() || code}
              </option>
            ))}
          </select>
        </label>
      )}
    </form>
  )
}
