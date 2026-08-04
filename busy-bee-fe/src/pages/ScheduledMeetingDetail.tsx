import { useState } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { BellRing, CalendarClock, ChevronLeft, Pencil, Trash2 } from 'lucide-react'

import { AppShell } from '../components/AppShell'
import { ScheduleSheet } from '../components/ScheduleForm'
import { Sheet } from '../components/Sheet'
import { StatusBadge } from '../components/StatusBadge'
import { useApiCall } from '../hooks/useApiCall'
import { deleteMeeting, type MeetingDetail } from '../services/api/client'

/** 排程會議詳情：顯示排程資訊，可編輯（含改名）與刪除。 */
export function ScheduledMeetingDetail({
  meeting,
  onChanged,
}: {
  meeting: MeetingDetail
  onChanged: (m: Partial<MeetingDetail>) => void
}) {
  const navigate = useNavigate()
  const call = useApiCall()
  const [editOpen, setEditOpen] = useState(false)
  const [confirmDelete, setConfirmDelete] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const scheduledAt = meeting.scheduledAt ? new Date(meeting.scheduledAt) : null
  const remindAt =
    scheduledAt && new Date(scheduledAt.getTime() - (meeting.remindBeforeMin ?? 15) * 60_000)
  const fmt = (d: Date) =>
    d.toLocaleString('zh-TW', { month: 'numeric', day: 'numeric', hour: '2-digit', minute: '2-digit', weekday: 'short' })

  const remove = async () => {
    try {
      await call(deleteMeeting, meeting.id)
      navigate('/schedule')
    } catch (e) {
      setError(e instanceof Error ? e.message : '刪除失敗')
      setConfirmDelete(false)
    }
  }

  return (
    <AppShell hideTopBar>
      <header className="flex items-center gap-2">
        <Link to="/schedule" className="btn btn-ghost size-11 shrink-0 px-0" aria-label="返回">
          <ChevronLeft className="size-5" />
        </Link>
        <h1 className="m-0 min-w-0 flex-1 truncate text-lg font-semibold">{meeting.title}</h1>
        <StatusBadge status={meeting.status} />
      </header>

      <section className="flex flex-col gap-4 rounded-xl border border-border bg-surface px-5 py-5">
        <div className="flex items-center gap-3">
          <CalendarClock className="size-5 shrink-0 text-accent" />
          <div className="min-w-0">
            <p className="m-0 text-xs text-muted">會議時間</p>
            <p className="m-0 text-sm font-medium">{scheduledAt ? fmt(scheduledAt) : '—'}</p>
          </div>
        </div>
        <div className="flex items-center gap-3">
          <BellRing className="size-5 shrink-0 text-accent" />
          <div className="min-w-0">
            <p className="m-0 text-xs text-muted">提醒</p>
            <p className="m-0 text-sm font-medium">
              提前 {meeting.remindBeforeMin ?? 15} 分鐘{remindAt ? `（${fmt(remindAt)}）` : ''}
            </p>
          </div>
        </div>
      </section>

      {error && <p className="m-0 text-sm text-red-500">{error}</p>}

      <div className="flex gap-2">
        <button type="button" className="btn btn-secondary flex-1" onClick={() => setEditOpen(true)}>
          <Pencil className="size-4" />
          編輯
        </button>
        <button
          type="button"
          className="btn btn-secondary flex-1 text-red-500"
          onClick={() => setConfirmDelete(true)}
        >
          <Trash2 className="size-4" />
          刪除
        </button>
      </div>

      <p className="m-0 text-xs text-muted">會議開始後上傳錄音，處理完成會出現 PRD、Tech Spec 與待辦。</p>

      {editOpen && (
        <ScheduleSheet
          editing={meeting}
          onClose={() => setEditOpen(false)}
          onSaved={(m) => onChanged(m)}
        />
      )}

      {confirmDelete && (
        <Sheet onClose={() => setConfirmDelete(false)}>
          <p className="m-0 text-sm">確定刪除「{meeting.title}」這筆排程？此動作無法復原。</p>
          <div className="flex gap-2">
            <button type="button" className="btn btn-secondary flex-1" onClick={() => setConfirmDelete(false)}>
              取消
            </button>
            <button type="button" className="btn btn-primary flex-1 bg-red-600" onClick={() => void remove()}>
              刪除
            </button>
          </div>
        </Sheet>
      )}
    </AppShell>
  )
}
