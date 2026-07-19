import { useState } from 'react'

import { createScheduledMeeting, type Meeting } from '../services/api/client'
import { getIdToken } from '../services/token'

/** 建立排程會議（會前推播提醒）。 */
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
      <button type="button" className="secondary" onClick={() => setOpen(true)}>
        📅 排程會議提醒
      </button>
    )
  }

  return (
    <div className="schedule-form">
      <input
        type="text"
        placeholder="會議標題"
        value={title}
        onChange={(e) => setTitle(e.target.value)}
      />
      <input type="datetime-local" value={at} onChange={(e) => setAt(e.target.value)} />
      <label className="small">
        提前
        <input
          type="number"
          min={1}
          max={1440}
          value={remind}
          onChange={(e) => setRemind(Number(e.target.value))}
          style={{ width: '4em', margin: '0 0.3em' }}
        />
        分鐘提醒
      </label>
      <div className="schedule-actions">
        <button type="button" disabled={busy || !title.trim() || !at} onClick={() => void submit()}>
          建立
        </button>
        <button type="button" className="secondary" onClick={() => setOpen(false)}>
          取消
        </button>
      </div>
      {error && <p className="error small">{error}</p>}
    </div>
  )
}
