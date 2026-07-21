import { type CSSProperties, type RefObject, useCallback, useEffect, useRef, useState } from 'react'
import { Link, useNavigate, useParams } from 'react-router-dom'
import ReactMarkdown from 'react-markdown'
import {
  BellRing,
  CalendarClock,
  ChevronLeft,
  FastForward,
  Pause,
  Pencil,
  Play,
  Rewind,
  Trash2,
} from 'lucide-react'

import { ActionItemList } from '../components/ActionItemList'
import { AppShell } from '../components/AppShell'
import { ExportBar } from '../components/ExportBar'
import { ScheduleSheet } from '../components/ScheduleForm'
import { StatusBadge } from '../components/StatusBadge'
import { useMeetingStatusSocket } from '../hooks/useMeetingStatusSocket'
import {
  deleteMeeting,
  editMeetingSegment,
  getMeeting,
  getMeetingAudioURL,
  listArtifacts,
  listMeetingActionItems,
  renameMeeting,
  retryMeeting,
  toggleActionItem,
  updateMeetingSpeakers,
  type ActionItem,
  type Artifact,
  type MeetingDetail,
  type TranscriptSegment,
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
  const navigate = useNavigate()
  const [meeting, setMeeting] = useState<MeetingDetail | null>(null)
  const [artifacts, setArtifacts] = useState<Artifact[]>([])
  const [actionItems, setActionItems] = useState<ActionItem[]>([])
  const [tab, setTab] = useState<Tab>('prd')
  const [error, setError] = useState<string | null>(null)
  const [confirmDelete, setConfirmDelete] = useState(false)
  const audioRef = useRef<HTMLAudioElement>(null)

  const isScheduled = meeting?.status === 'scheduled'

  // 刪除會議（任何狀態）→ 回會議列表。
  const remove = async () => {
    if (!id) return
    try {
      await deleteMeeting(await getIdToken(), id)
      navigate('/meetings')
    } catch (e) {
      setError(e instanceof Error ? e.message : '刪除失敗')
      setConfirmDelete(false)
    }
  }

  // 點逐字稿時間碼 → 音檔跳至該處並播放。
  const seekAudio = (seconds: number) => {
    const a = audioRef.current
    if (!a) return
    a.currentTime = seconds
    void a.play()
  }

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

  // 匯出的逐字稿以「顯示名: 內容」呈現（把代號 A/B 換成使用者設定的名字）。
  const transcriptExport =
    meeting.transcriptSegments.length > 0
      ? meeting.transcriptSegments
          .map((s) => `${meeting.speakerNames?.[s.speaker]?.trim() || s.speaker}: ${s.text}`)
          .join('\n')
      : meeting.transcript
  const exportContent = tab === 'transcript' ? transcriptExport : docContent

  return (
    <AppShell>
      <header className="flex items-center gap-2">
        <Link to="/meetings" className="btn btn-ghost size-11 shrink-0 px-0" aria-label="返回">
          <ChevronLeft className="size-5" />
        </Link>
        <EditableTitle
          meetingId={meeting.id}
          title={meeting.title}
          onRenamed={(title) => setMeeting({ ...meeting, title })}
        />
        <StatusBadge status={meeting.status} />
        <button
          type="button"
          className="btn btn-ghost size-9 shrink-0 px-0 text-muted hover:text-red-500"
          aria-label="刪除會議"
          onClick={() => setConfirmDelete(true)}
        >
          <Trash2 className="size-4" />
        </button>
      </header>

      {meeting.status === 'failed' && (
        <div className="flex flex-wrap items-center justify-between gap-3 rounded-xl border border-red-500/30 bg-red-500/5 px-4 py-3">
          <p className="m-0 text-sm text-red-500">處理失敗：{meeting.errorMessage || '未知錯誤'}</p>
          <button type="button" className="btn btn-secondary h-9" onClick={() => void retry()}>
            重新處理
          </button>
        </div>
      )}

      <AudioPlayer meetingId={meeting.id} durationSeconds={meeting.durationSeconds} audioRef={audioRef} />

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

      {tab !== 'action_items' && exportContent && (
        <div className="-mb-2 flex justify-end">
          <ExportBar content={exportContent} filename={`${meeting.title}-${tab}`} />
        </div>
      )}

      <article key={tab} className="animate-fade-in rounded-xl border border-border bg-surface px-5 py-4">
        {tab === 'action_items' ? (
          meeting.status === 'completed' ? (
            <ActionItemList items={actionItems} onToggle={toggleItem} />
          ) : (
            <p className="m-0 text-sm text-muted">處理完成後將顯示於此。</p>
          )
        ) : tab === 'transcript' && meeting.transcriptSegments.length > 0 ? (
          <TranscriptView meeting={meeting} onUpdated={setMeeting} onSeek={seekAudio} />
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

      {confirmDelete && (
        <>
          <div
            className="animate-fade-in fixed inset-0 z-40 bg-black/50"
            onClick={() => setConfirmDelete(false)}
          />
          <div className="animate-sheet-up sm:animate-fade-in fixed inset-x-0 bottom-0 z-50 flex flex-col gap-3 rounded-t-2xl border-t border-border bg-surface p-4 pb-[calc(1.25rem+env(safe-area-inset-bottom))] sm:inset-x-auto sm:top-1/2 sm:left-1/2 sm:bottom-auto sm:w-full sm:max-w-sm sm:-translate-x-1/2 sm:-translate-y-1/2 sm:rounded-2xl sm:border sm:pb-4">
            <p className="m-0 text-sm">
              確定刪除「{meeting.title}」？逐字稿、PRD、Tech Spec、行動項將一併刪除，此動作無法復原。
            </p>
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
      <MarqueeTitle title={title} />
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

/** 標題過長時以跑馬燈（音樂播放器風格）左右來回捲動；未溢出則靜止。 */
function MarqueeTitle({ title }: { title: string }) {
  const containerRef = useRef<HTMLDivElement>(null)
  const textRef = useRef<HTMLHeadingElement>(null)
  const [shift, setShift] = useState(0)

  useEffect(() => {
    const c = containerRef.current
    const t = textRef.current
    if (!c || !t) return
    const reduce = window.matchMedia('(prefers-reduced-motion: reduce)').matches
    const overflow = t.scrollWidth - c.clientWidth
    setShift(!reduce && overflow > 4 ? overflow : 0)
  }, [title])

  const style: CSSProperties | undefined =
    shift > 0
      ? ({
          animation: `marquee ${Math.max(6, shift / 24)}s ease-in-out infinite alternate`,
          '--marquee-shift': `-${shift}px`,
        } as CSSProperties)
      : undefined

  return (
    <div ref={containerRef} className="min-w-0 flex-1 overflow-hidden">
      <h1 ref={textRef} className="m-0 inline-block whitespace-nowrap text-lg font-semibold" style={style}>
        {title}
      </h1>
    </div>
  )
}

// 講者晶片配色：依首次出現順序輪替。
const SPEAKER_COLORS = [
  'bg-blue-500/15 text-blue-600 dark:text-blue-400',
  'bg-emerald-500/15 text-emerald-600 dark:text-emerald-400',
  'bg-amber-500/15 text-amber-600 dark:text-amber-400',
  'bg-fuchsia-500/15 text-fuchsia-600 dark:text-fuchsia-400',
  'bg-rose-500/15 text-rose-600 dark:text-rose-400',
  'bg-cyan-500/15 text-cyan-600 dark:text-cyan-400',
]

function speakerColor(code: string, order: string[]): string {
  const i = order.indexOf(code)
  return SPEAKER_COLORS[(i < 0 ? 0 : i) % SPEAKER_COLORS.length]
}

/** 分講者逐字稿：講者晶片可點擊改名（PATCH speakers），逐段顯示。 */
function TranscriptView({
  meeting,
  onUpdated,
  onSeek,
}: {
  meeting: MeetingDetail
  onUpdated: (m: MeetingDetail) => void
  onSeek?: (seconds: number) => void
}) {
  const [editing, setEditing] = useState<string | null>(null)

  const names = meeting.speakerNames ?? {}
  const displayName = (code: string) => names[code]?.trim() || code
  // 依首次出現順序取得講者代號
  const order: string[] = []
  for (const s of meeting.transcriptSegments) {
    if (!order.includes(s.speaker)) order.push(s.speaker)
  }

  return (
    <div className="flex flex-col gap-4">
      {/* 講者圖例：點晶片可改名 */}
      <div className="flex flex-wrap gap-2 border-b border-border pb-3">
        {order.map((code) => (
          <button
            key={code}
            type="button"
            className={`inline-flex items-center gap-1 rounded-full px-2.5 py-1 text-xs font-medium ${speakerColor(code, order)}`}
            onClick={() => setEditing(code)}
          >
            {displayName(code)}
            <Pencil className="size-3 opacity-60" />
          </button>
        ))}
      </div>

      {/* 逐段內容：標頭（時間 · 講者 · ▶）整列可點跳播、✏️ 可修正文字 + 全寬文字 */}
      <div className="flex flex-col gap-4">
        {meeting.transcriptSegments.map((s, i) => (
          <SegmentRow
            key={i}
            meetingId={meeting.id}
            index={i}
            seg={s}
            speakerName={displayName(s.speaker)}
            colorClass={speakerColor(s.speaker, order)}
            onSeek={onSeek}
            onUpdated={onUpdated}
          />
        ))}
      </div>

      {editing && (
        <SpeakerRenameSheet
          meetingId={meeting.id}
          code={editing}
          current={displayName(editing)}
          existingNames={names}
          onClose={() => setEditing(null)}
          onUpdated={(m) => {
            onUpdated(m)
            setEditing(null)
          }}
        />
      )}
    </div>
  )
}

/** 單段逐字稿：標頭可跳播、✏️ 可就地修正文字（PATCH /meetings/:id/transcript）。 */
function SegmentRow({
  meetingId,
  index,
  seg,
  speakerName,
  colorClass,
  onSeek,
  onUpdated,
}: {
  meetingId: string
  index: number
  seg: TranscriptSegment
  speakerName: string
  colorClass: string
  onSeek?: (seconds: number) => void
  onUpdated: (m: MeetingDetail) => void
}) {
  const [editing, setEditing] = useState(false)
  const [draft, setDraft] = useState(seg.text)
  const [busy, setBusy] = useState(false)

  const save = async () => {
    const next = draft.trim()
    if (!next || next === seg.text) {
      setEditing(false)
      setDraft(seg.text)
      return
    }
    setBusy(true)
    try {
      const { meeting } = await editMeetingSegment(await getIdToken(), meetingId, index, next)
      onUpdated(meeting)
      setEditing(false)
    } catch {
      setDraft(seg.text) // 失敗還原
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="flex flex-col gap-1">
      <div className="flex items-center gap-1">
        <button
          type="button"
          className="group -mx-1 flex flex-1 items-center gap-2 rounded-md px-1 py-0.5 transition-colors hover:bg-surface-hover"
          onClick={() => onSeek?.(seg.startMs / 1000)}
          aria-label={`從 ${fmtClock(seg.startMs / 1000)} 播放`}
        >
          <span className="font-mono text-[11px] tabular-nums text-muted">{fmtClock(seg.startMs / 1000)}</span>
          <span className={`rounded-full px-2 py-0.5 text-xs font-medium ${colorClass}`}>{speakerName}</span>
          <Play className="ml-auto size-3.5 shrink-0 text-muted transition-colors group-hover:text-accent" />
        </button>
        <button
          type="button"
          className="btn btn-ghost size-7 shrink-0 px-0 text-muted hover:text-accent"
          aria-label="修正文字"
          onClick={() => {
            setDraft(seg.text)
            setEditing(true)
          }}
        >
          <Pencil className="size-3.5" />
        </button>
      </div>

      {editing ? (
        <div className="flex flex-col gap-2">
          <textarea
            className="input min-h-[4.5rem] text-sm leading-7"
            value={draft}
            disabled={busy}
            autoFocus
            onChange={(e) => setDraft(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === 'Escape') {
                setDraft(seg.text)
                setEditing(false)
              }
            }}
          />
          <div className="flex justify-end gap-2">
            <button
              type="button"
              className="btn btn-secondary h-9"
              disabled={busy}
              onClick={() => {
                setDraft(seg.text)
                setEditing(false)
              }}
            >
              取消
            </button>
            <button type="button" className="btn btn-primary h-9" disabled={busy} onClick={() => void save()}>
              {busy ? '儲存中…' : '儲存'}
            </button>
          </div>
        </div>
      ) : (
        <p className="m-0 text-sm leading-7">{seg.text}</p>
      )}
    </div>
  )
}

/** 講者改名底部彈窗（PATCH /meetings/:id/speakers）。 */
function SpeakerRenameSheet({
  meetingId,
  code,
  current,
  existingNames,
  onClose,
  onUpdated,
}: {
  meetingId: string
  code: string
  current: string
  existingNames: Record<string, string>
  onClose: () => void
  onUpdated: (m: MeetingDetail) => void
}) {
  const [draft, setDraft] = useState(current === code ? '' : current)
  const [busy, setBusy] = useState(false)
  const [err, setErr] = useState<string | null>(null)

  const save = async () => {
    const next = draft.trim()
    setBusy(true)
    setErr(null)
    try {
      const merged = { ...existingNames }
      if (next) merged[code] = next
      else delete merged[code]
      const { meeting } = await updateMeetingSpeakers(await getIdToken(), meetingId, merged)
      onUpdated(meeting)
    } catch (e) {
      setErr(e instanceof Error ? e.message : '更新失敗')
      setBusy(false)
    }
  }

  return (
    <>
      <div className="animate-fade-in fixed inset-0 z-40 bg-black/50" onClick={onClose} />
      <div className="animate-sheet-up sm:animate-fade-in fixed inset-x-0 bottom-0 z-50 flex flex-col gap-3 rounded-t-2xl border-t border-border bg-surface p-4 pb-[calc(1.25rem+env(safe-area-inset-bottom))] sm:inset-x-auto sm:top-1/2 sm:left-1/2 sm:bottom-auto sm:w-full sm:max-w-sm sm:-translate-x-1/2 sm:-translate-y-1/2 sm:rounded-2xl sm:border sm:pb-4">
        <p className="m-0 text-sm font-medium">重新命名講者 {code}</p>
        <input
          className="input h-10"
          value={draft}
          placeholder={`例如 Ben（留空還原為 ${code}）`}
          disabled={busy}
          autoFocus
          onChange={(e) => setDraft(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === 'Enter') void save()
            if (e.key === 'Escape') onClose()
          }}
        />
        {err && <p className="m-0 text-xs text-red-500">{err}</p>}
        <div className="flex gap-2">
          <button type="button" className="btn btn-secondary flex-1" onClick={onClose} disabled={busy}>
            取消
          </button>
          <button type="button" className="btn btn-primary flex-1" onClick={() => void save()} disabled={busy}>
            {busy ? '儲存中…' : '儲存'}
          </button>
        </div>
      </div>
    </>
  )
}

// fmtClock 秒 → m:ss（時間碼／播放器共用）。
function fmtClock(seconds: number): string {
  if (!isFinite(seconds) || seconds < 0) seconds = 0
  const m = Math.floor(seconds / 60)
  const s = Math.floor(seconds % 60)
  return `${m}:${String(s).padStart(2, '0')}`
}

/** 音檔播放器：播放/暫停、±10 秒、可拖曳進度條。時長以後端 durationSeconds 為準
 *  （MediaRecorder 產生的 webm 常無 duration metadata）。 */
function AudioPlayer({
  meetingId,
  durationSeconds,
  audioRef,
}: {
  meetingId: string
  durationSeconds: number
  audioRef: RefObject<HTMLAudioElement | null>
}) {
  const [url, setUrl] = useState<string | null>(null)
  const [failed, setFailed] = useState(false)
  const [playing, setPlaying] = useState(false)
  const [cur, setCur] = useState(0)

  useEffect(() => {
    let active = true
    void (async () => {
      try {
        const { url } = await getMeetingAudioURL(await getIdToken(), meetingId)
        if (active) setUrl(url)
      } catch {
        if (active) setFailed(true)
      }
    })()
    return () => {
      active = false
    }
  }, [meetingId])

  if (failed) return null

  const total = durationSeconds > 0 ? durationSeconds : 0
  const toggle = () => {
    const a = audioRef.current
    if (!a) return
    if (a.paused) void a.play()
    else a.pause()
  }
  const skip = (delta: number) => {
    const a = audioRef.current
    if (a) a.currentTime = Math.max(0, a.currentTime + delta)
  }
  const seek = (v: number) => {
    const a = audioRef.current
    if (a) a.currentTime = v
    setCur(v)
  }

  return (
    <div className="flex items-center gap-2 rounded-xl border border-border bg-surface px-3 py-2.5">
      <audio
        ref={audioRef}
        src={url ?? undefined}
        preload="metadata"
        onTimeUpdate={(e) => setCur(e.currentTarget.currentTime)}
        onPlay={() => setPlaying(true)}
        onPause={() => setPlaying(false)}
        onEnded={() => setPlaying(false)}
      />
      <button
        type="button"
        className="btn btn-ghost size-9 shrink-0 px-0 text-muted"
        aria-label="倒退 10 秒"
        onClick={() => skip(-10)}
        disabled={!url}
      >
        <Rewind className="size-4" />
      </button>
      <button
        type="button"
        className="btn btn-primary size-10 shrink-0 rounded-full px-0"
        aria-label={playing ? '暫停' : '播放'}
        onClick={toggle}
        disabled={!url}
      >
        {playing ? <Pause className="size-4" /> : <Play className="size-4" />}
      </button>
      <button
        type="button"
        className="btn btn-ghost size-9 shrink-0 px-0 text-muted"
        aria-label="快轉 10 秒"
        onClick={() => skip(10)}
        disabled={!url}
      >
        <FastForward className="size-4" />
      </button>
      <span className="w-9 shrink-0 text-right font-mono text-[11px] tabular-nums text-muted">{fmtClock(cur)}</span>
      <input
        type="range"
        className="min-w-0 flex-1 accent-accent"
        min={0}
        max={total || 0}
        step={0.1}
        value={Math.min(cur, total || 0)}
        onChange={(e) => seek(Number(e.target.value))}
        disabled={!url}
        aria-label="播放進度"
      />
      <span className="w-9 shrink-0 font-mono text-[11px] tabular-nums text-muted">{fmtClock(total)}</span>
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
      await deleteMeeting(await getIdToken(), meeting.id)
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
