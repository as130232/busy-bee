import { useState } from 'react'
import { CalendarPlus, X } from 'lucide-react'

import {
  createScheduledMeeting,
  updateMeetingSchedule,
  type Meeting,
} from '../services/api/client'
import { getIdToken } from '../services/token'

/** Date/ISO 字串 → datetime-local input 值（本地時區） */
function toLocalInputValue(iso: string): string {
  const d = new Date(iso)
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`
}

interface SheetProps {
  /** 帶入既有排程 → 編輯模式；未帶 → 建立模式 */
  editing?: Pick<Meeting, 'id' | 'title' | 'scheduledAt' | 'remindBeforeMin'>
  onClose: () => void
  onSaved?: (m: Meeting) => void
}

/** 排程表單 bottom sheet（手機底部滑入，桌面置中）；建立與編輯共用。 */
export function ScheduleSheet({ editing, onClose, onSaved }: SheetProps) {
  const [title, setTitle] = useState(editing?.title ?? '')
  const [at, setAt] = useState(editing?.scheduledAt ? toLocalInputValue(editing.scheduledAt) : '')
  const [remind, setRemind] = useState(editing?.remindBeforeMin ?? 15)
  const [error, setError] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)

  const submit = async () => {
    setBusy(true)
    setError(null)
    try {
      const input = { title, scheduledAt: new Date(at).toISOString(), remindBeforeMin: remind }
      const token = await getIdToken()
      const { meeting } = editing
        ? await updateMeetingSchedule(token, editing.id, input)
        : await createScheduledMeeting(token, input)
      onClose()
      onSaved?.(meeting)
    } catch (e) {
      setError(e instanceof Error ? e.message : editing ? '更新失敗' : '建立失敗')
    } finally {
      setBusy(false)
    }
  }

  return (
    <>
      <div className="animate-fade-in fixed inset-0 z-40 bg-black/50" onClick={onClose} />
      <div className="animate-sheet-up sm:animate-fade-in fixed inset-x-0 bottom-0 z-50 flex max-h-[85dvh] flex-col gap-3 overflow-y-auto rounded-t-2xl border-t border-border bg-surface p-4 pb-[calc(1.25rem+env(safe-area-inset-bottom))] sm:inset-x-auto sm:top-1/2 sm:left-1/2 sm:bottom-auto sm:max-h-[85dvh] sm:w-full sm:max-w-sm sm:-translate-x-1/2 sm:-translate-y-1/2 sm:rounded-2xl sm:border sm:pb-4">
        <div className="flex items-center justify-between">
          <h2 className="m-0 text-base font-semibold">{editing ? '編輯排程' : '排程會議提醒'}</h2>
          <button type="button" className="btn btn-ghost size-9 px-0" aria-label="關閉" onClick={onClose}>
            <X className="size-4" />
          </button>
        </div>
        <input
          className="input"
          type="text"
          placeholder="會議標題"
          value={title}
          onChange={(e) => setTitle(e.target.value)}
        />
        <input
          className="input"
          type="datetime-local"
          value={at}
          onChange={(e) => setAt(e.target.value)}
        />
        <label className="flex items-center gap-2 text-sm text-muted">
          提前
          <input
            className="input h-9 w-16 px-2 text-center"
            type="number"
            min={1}
            max={1440}
            value={remind}
            onChange={(e) => setRemind(Number(e.target.value))}
          />
          分鐘提醒
        </label>
        {error && <p className="m-0 text-sm text-red-500">{error}</p>}
        <button
          type="button"
          className="btn btn-primary mt-1"
          disabled={busy || !title.trim() || !at}
          onClick={() => void submit()}
        >
          {editing ? '儲存' : '建立'}
        </button>
      </div>
    </>
  )
}

/** 建立排程會議入口按鈕（點開 ScheduleSheet）。 */
export function ScheduleForm({ onCreated }: { onCreated?: (m: Meeting) => void }) {
  const [open, setOpen] = useState(false)

  if (!open) {
    return (
      <button type="button" className="btn btn-secondary" onClick={() => setOpen(true)}>
        <CalendarPlus className="size-4" />
        排程會議
      </button>
    )
  }
  return <ScheduleSheet onClose={() => setOpen(false)} onSaved={onCreated} />
}
