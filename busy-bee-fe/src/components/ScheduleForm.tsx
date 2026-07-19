import { useState } from 'react'
import { CalendarPlus, X } from 'lucide-react'

import { createScheduledMeeting, type Meeting } from '../services/api/client'
import { getIdToken } from '../services/token'

/** 建立排程會議（會前推播提醒）。手機為底部 bottom sheet，桌面為置中對話框。 */
export function ScheduleForm({ onCreated }: { onCreated?: (m: Meeting) => void }) {
  const [open, setOpen] = useState(false)
  const [title, setTitle] = useState('')
  const [at, setAt] = useState('')
  const [remind, setRemind] = useState(15)
  const [error, setError] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)

  const submit = async () => {
    setBusy(true)
    setError(null)
    try {
      const { meeting } = await createScheduledMeeting(await getIdToken(), {
        title,
        scheduledAt: new Date(at).toISOString(),
        remindBeforeMin: remind,
      })
      setTitle('')
      setAt('')
      setOpen(false)
      onCreated?.(meeting)
    } catch (e) {
      setError(e instanceof Error ? e.message : '建立失敗')
    } finally {
      setBusy(false)
    }
  }

  if (!open) {
    return (
      <button type="button" className="btn btn-secondary" onClick={() => setOpen(true)}>
        <CalendarPlus className="size-4" />
        排程會議
      </button>
    )
  }

  return (
    <>
      <div className="fixed inset-0 z-40 bg-black/50" onClick={() => setOpen(false)} />
      <div className="fixed inset-x-0 bottom-0 z-50 flex flex-col gap-3 rounded-t-2xl border-t border-border bg-surface p-4 pb-[calc(1.25rem+env(safe-area-inset-bottom))] sm:inset-x-auto sm:top-1/2 sm:left-1/2 sm:bottom-auto sm:w-full sm:max-w-sm sm:-translate-x-1/2 sm:-translate-y-1/2 sm:rounded-2xl sm:border sm:pb-4">
        <div className="flex items-center justify-between">
          <h2 className="m-0 text-base font-semibold">排程會議提醒</h2>
          <button
            type="button"
            className="btn btn-ghost size-9 px-0"
            aria-label="關閉"
            onClick={() => setOpen(false)}
          >
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
          建立
        </button>
      </div>
    </>
  )
}
