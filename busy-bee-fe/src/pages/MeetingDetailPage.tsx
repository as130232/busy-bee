import { useCallback, useEffect, useState } from 'react'
import { Link, useNavigate, useParams } from 'react-router-dom'
import ReactMarkdown from 'react-markdown'
import { BellRing, CalendarClock, ChevronLeft, Pencil, Trash2 } from 'lucide-react'

import { ActionItemList } from '../components/ActionItemList'
import { AppShell } from '../components/AppShell'
import { ExportBar } from '../components/ExportBar'
import { ScheduleSheet } from '../components/ScheduleForm'
import { StatusBadge } from '../components/StatusBadge'
import { useMeetingStatusSocket } from '../hooks/useMeetingStatusSocket'
import {
  deleteScheduledMeeting,
  getMeeting,
  listArtifacts,
  listMeetingActionItems,
  renameMeeting,
  retryMeeting,
  toggleActionItem,
  type ActionItem,
  type Artifact,
  type MeetingDetail,
} from '../services/api/client'
import { getIdToken } from '../services/token'

type Tab = 'prd' | 'tech_spec' | 'action_items' | 'transcript'

const tabLabels: Record<Tab, string> = {
  prd: 'PRD',
  tech_spec: 'Tech Spec',
  action_items: '行動項',
  transcript: '逐字稿',
}

export function MeetingDetailPage() {
  const { id } = useParams<{ id: string }>()
  const [meeting, setMeeting] = useState<MeetingDetail | null>(null)
  const [artifacts, setArtifacts] = useState<Artifact[]>([])
  const [actionItems, setActionItems] = useState<ActionItem[]>([])
  const [tab, setTab] = useState<Tab>('prd')
  const [error, setError] = useState<string | null>(null)

  const isScheduled = meeting?.status === 'scheduled'

  const load = useCallback(async () => {
    if (!id) return
    try {
      const token = await getIdToken()
      const m = await getMeeting(token, id)
      setMeeting(m.meeting)
      // 排程會議尚無文件/行動項，不多打兩支 API
      if (m.meeting.status !== 'scheduled') {
        const [a, ai] = await Promise.all([listArtifacts(token, id), listMeetingActionItems(token, id)])
        setArtifacts(a.artifacts)
        setActionItems(ai.actionItems)
      }
      setError(null)
    } catch (e) {
      setError(e instanceof Error ? e.message : '載入失敗')
    }
  }, [id])

  useEffect(() => {
    void load()
  }, [load])

  // 本會議狀態變更時重新載入（完成時文件才會出現）
  useMeetingStatusSocket((e) => {
    if (e.meetingId === id) void load()
  })

  const retry = async () => {
    if (!id) return
    try {
      await retryMeeting(await getIdToken(), id)
      void load()
    } catch (e) {
      setError(e instanceof Error ? e.message : '重試失敗')
    }
  }

  const toggleItem = async (itemId: string, done: boolean) => {
    setActionItems((prev) => prev.map((it) => (it.id === itemId ? { ...it, done } : it)))
    try {
      await toggleActionItem(await getIdToken(), itemId, done)
    } catch {
      void load() // 失敗回滾
    }
  }

  if (error) {
    return (
      <AppShell>
        <p className="py-10 text-center text-sm text-red-500">{error}</p>
      </AppShell>
    )
  }
  if (!meeting) {
    return (
      <AppShell>
        <p className="py-10 text-center text-sm text-muted">載入中…</p>
      </AppShell>
    )
  }

  if (isScheduled) {
    return (
      <ScheduledDetail
        meeting={meeting}
        onChanged={(m) => setMeeting({ ...meeting, ...m })}
      />
    )
  }

  const artifactByType = new Map(artifacts.map((a) => [a.type, a]))
  const docContent =
    tab === 'transcript'
      ? meeting.transcript
      : tab === 'action_items'
        ? ''
        : (artifactByType.get(tab)?.content ?? '')

  return (
    <AppShell>
      <header className="flex items-center gap-2">
        <Link to="/" className="btn btn-ghost size-11 shrink-0 px-0" aria-label="返回">
          <ChevronLeft className="size-5" />
        </Link>
        <EditableTitle
          meetingId={meeting.id}
          title={meeting.title}
          onRenamed={(title) => setMeeting({ ...meeting, title })}
        />
        <StatusBadge status={meeting.status} />
      </header>

      {meeting.status === 'failed' && (
        <div className="flex flex-wrap items-center justify-between gap-3 rounded-xl border border-red-500/30 bg-red-500/5 px-4 py-3">
          <p className="m-0 text-sm text-red-500">處理失敗：{meeting.errorMessage || '未知錯誤'}</p>
          <button type="button" className="btn btn-secondary h-9" onClick={() => void retry()}>
            重新處理
          </button>
        </div>
      )}

      <nav className="grid grid-cols-4 border-b border-border">
        {(Object.keys(tabLabels) as Tab[]).map((t) => (
          <button
            key={t}
            type="button"
            className={`-mb-px h-11 cursor-pointer border-b-2 text-sm font-medium transition ${
              tab === t ? 'border-accent text-fg' : 'border-transparent text-muted hover:text-fg'
            }`}
            onClick={() => setTab(t)}
          >
            {tabLabels[t]}
          </button>
        ))}
      </nav>

      {tab !== 'action_items' && docContent && (
        <div className="-mb-2 flex justify-end">
          <ExportBar content={docContent} filename={`${meeting.title}-${tab}`} />
        </div>
      )}

      <article key={tab} className="animate-fade-in rounded-xl border border-border bg-surface px-5 py-4">
        {tab === 'action_items' ? (
          meeting.status === 'completed' ? (
            <ActionItemList items={actionItems} onToggle={toggleItem} />
          ) : (
            <p className="m-0 text-sm text-muted">處理完成後將顯示於此。</p>
          )
        ) : docContent ? (
          tab === 'transcript' ? (
            <p className="text-sm leading-7 whitespace-pre-wrap">{docContent}</p>
          ) : (
            <div className="prose prose-sm prose-zinc dark:prose-invert max-w-none prose-headings:font-semibold prose-h1:text-xl prose-h2:mt-6 prose-h2:border-b prose-h2:border-border prose-h2:pb-1.5 prose-h2:text-base">
              <ReactMarkdown>{docContent}</ReactMarkdown>
            </div>
          )
        ) : (
          <p className="m-0 text-sm text-muted">
            {meeting.status === 'completed' ? '無內容' : '處理完成後將顯示於此。'}
          </p>
        )}
      </article>
    </AppShell>
  )
}

/** 標題 + 鉛筆編輯（PATCH rename）。 */
function EditableTitle({
  meetingId,
  title,
  onRenamed,
}: {
  meetingId: string
  title: string
  onRenamed: (title: string) => void
}) {
  const [editing, setEditing] = useState(false)
  const [draft, setDraft] = useState(title)
  const [busy, setBusy] = useState(false)

  const save = async () => {
    const next = draft.trim()
    if (!next || next === title) {
      setEditing(false)
      setDraft(title)
      return
    }
    setBusy(true)
    try {
      const { meeting } = await renameMeeting(await getIdToken(), meetingId, next)
      onRenamed(meeting.title)
      setEditing(false)
    } catch {
      setDraft(title) // 失敗還原
      setEditing(false)
    } finally {
      setBusy(false)
    }
  }

  if (editing) {
    return (
      <input
        className="input h-9 min-w-0 flex-1"
        value={draft}
        disabled={busy}
        autoFocus
        onChange={(e) => setDraft(e.target.value)}
        onBlur={() => void save()}
        onKeyDown={(e) => {
          if (e.key === 'Enter') e.currentTarget.blur()
          if (e.key === 'Escape') {
            setDraft(title)
            setEditing(false)
          }
        }}
      />
    )
  }
  return (
    <div className="flex min-w-0 flex-1 items-center gap-1.5">
      <h1 className="m-0 min-w-0 truncate text-lg font-semibold">{title}</h1>
      <button
        type="button"
        className="btn btn-ghost size-8 shrink-0 px-0 text-muted"
        aria-label="重新命名"
        onClick={() => {
          setDraft(title)
          setEditing(true)
        }}
      >
        <Pencil className="size-3.5" />
      </button>
    </div>
  )
}

/** 排程會議詳情：顯示排程資訊，可編輯（含改名）與刪除。 */
function ScheduledDetail({
  meeting,
  onChanged,
}: {
  meeting: MeetingDetail
  onChanged: (m: Partial<MeetingDetail>) => void
}) {
  const navigate = useNavigate()
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
      await deleteScheduledMeeting(await getIdToken(), meeting.id)
      navigate('/schedule')
    } catch (e) {
      setError(e instanceof Error ? e.message : '刪除失敗')
      setConfirmDelete(false)
    }
  }

  return (
    <AppShell>
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

      <p className="m-0 text-xs text-muted">會議開始後上傳錄音，處理完成會出現 PRD、Tech Spec 與行動項。</p>

      {editOpen && (
        <ScheduleSheet
          editing={meeting}
          onClose={() => setEditOpen(false)}
          onSaved={(m) => onChanged(m)}
        />
      )}

      {confirmDelete && (
        <>
          <div className="animate-fade-in fixed inset-0 z-40 bg-black/50" onClick={() => setConfirmDelete(false)} />
          <div className="animate-sheet-up sm:animate-fade-in fixed inset-x-0 bottom-0 z-50 flex flex-col gap-3 rounded-t-2xl border-t border-border bg-surface p-4 pb-[calc(1.25rem+env(safe-area-inset-bottom))] sm:inset-x-auto sm:top-1/2 sm:left-1/2 sm:bottom-auto sm:w-full sm:max-w-sm sm:-translate-x-1/2 sm:-translate-y-1/2 sm:rounded-2xl sm:border sm:pb-4">
            <p className="m-0 text-sm">確定刪除「{meeting.title}」這筆排程？此動作無法復原。</p>
            <div className="flex gap-2">
              <button type="button" className="btn btn-secondary flex-1" onClick={() => setConfirmDelete(false)}>
                取消
              </button>
              <button type="button" className="btn btn-primary flex-1 bg-red-600" onClick={() => void remove()}>
                刪除
              </button>
            </div>
          </div>
        </>
      )}
    </AppShell>
  )
}
