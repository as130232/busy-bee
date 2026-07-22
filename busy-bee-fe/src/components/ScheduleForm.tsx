import { useState } from 'react'
import { CalendarPlus, X } from 'lucide-react'

import {
  createScheduledMeeting,
  updateMeetingSchedule,
  type Meeting,
  type Scenario,
} from '../services/api/client'
import { getIdToken } from '../services/token'
import { ScenarioToggle } from './ScenarioToggle'
import { Sheet } from './Sheet'

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
  const [scenario, setScenario] = useState<Scenario>('meeting')
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
      // 情境僅於建立時設定（排程編輯不改情境）。
      const { meeting } = editing
        ? await updateMeetingSchedule(token, editing.id, input)
        : await createScheduledMeeting(token, { ...input, scenario })
      onClose()
      onSaved?.(meeting)
    } catch (e) {
      setError(e instanceof Error ? e.message : editing ? '更新失敗' : '建立失敗')
    } finally {
      setBusy(false)
    }
  }

  return (
    <Sheet onClose={onClose} className="max-h-[85dvh] overflow-y-auto sm:max-h-[85dvh]">
      <div className="flex items-center justify-between">
          <h2 className="m-0 text-base font-semibold">{editing ? '編輯排程' : '排程會議提醒'}</h2>
          <button type="button" className="btn btn-ghost size-9 px-0" aria-label="關閉" onClick={onClose}>
            <X className="size-4" />
          </button>
        </div>
        <input
          className="input"
          type="text"
          placeholder="標題"
          value={title}
          onChange={(e) => setTitle(e.target.value)}
        />
        {!editing && (
          <div className="flex items-center gap-2">
            <span className="text-sm text-muted">情境</span>
            <ScenarioToggle value={scenario} onChange={setScenario} disabled={busy} />
          </div>
        )}
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
    </Sheet>
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
