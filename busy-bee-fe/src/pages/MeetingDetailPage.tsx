import { type CSSProperties, type RefObject, useCallback, useEffect, useRef, useState } from 'react'
import { createPortal } from 'react-dom'
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
  Plus,
  Rewind,
  Trash2,
} from 'lucide-react'

import { ActionItemList } from '../components/ActionItemList'
import { AppShell } from '../components/AppShell'
import { ExportBar } from '../components/ExportBar'
import { Loader } from '../components/Loader'
import { ScheduleSheet } from '../components/ScheduleForm'
import { Sheet } from '../components/Sheet'
import { StatusBadge } from '../components/StatusBadge'
import { SummarySections } from '../components/SummarySections'
import { resolveSpeakerNames, speakerColor } from '../components/speakerColor'
import { useMeetingStatusSocket } from '../hooks/useMeetingStatusSocket'
import {
  addMeetingActionItem,
  deleteMeeting,
  editActionItem,
  editMeetingSegment,
  getMeeting,
  getMeetingAudioURL,
  listArtifacts,
  listMeetingActionItems,
  renameMeeting,
  retryMeeting,
  scenarioLabels,
  toggleActionItem,
  updateMeetingSpeakers,
  type ActionItem,
  type Artifact,
  type MeetingDetail,
  type TranscriptSegment,
} from '../services/api/client'
import { getIdToken } from '../services/token'

type Tab = 'summary' | 'prd' | 'tech_spec' | 'action_items' | 'transcript'

const tabLabels: Record<Tab, string> = {
  summary: '摘要',
  prd: 'PRD',
  tech_spec: 'Tech Spec',
  action_items: '待辦',
  transcript: '逐字稿',
}

// 核心頁籤一律顯示；PRD / Tech Spec 已改為選用，僅在對應 artifact 存在時才出現。
const coreTabs: Tab[] = ['summary', 'action_items', 'transcript']
const optionalDocTabs = ['prd', 'tech_spec'] as const satisfies readonly Tab[]

// buildSummaryMarkdown 把 AI 摘要（TL;DR + 各區塊）組成 Markdown 供匯出；講者代號一併換成顯示名。
function buildSummaryMarkdown(meeting: MeetingDetail): string {
  const names = meeting.speakerNames ?? {}
  const lines: string[] = []
  if (meeting.summary) lines.push(resolveSpeakerNames(meeting.summary, names), '')
  for (const s of meeting.summarySections) {
    if (s.items.length === 0) continue
    lines.push(`## ${s.title}`)
    for (const p of s.items) {
      const text = resolveSpeakerNames(p.text ?? '', names)
      const heading = resolveSpeakerNames(p.heading ?? '', names)
      const who = p.speaker ? `（${names[p.speaker]?.trim() || p.speaker}）` : ''
      lines.push(heading ? `- **${heading}**${who}${text ? `：${text}` : ''}` : `- ${text}`)
    }
    lines.push('')
  }
  return lines.join('\n').trim()
}

export function MeetingDetailPage() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const [meeting, setMeeting] = useState<MeetingDetail | null>(null)
  const [artifacts, setArtifacts] = useState<Artifact[]>([])
  const [actionItems, setActionItems] = useState<ActionItem[]>([])
  const [tab, setTab] = useState<Tab>('summary')
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

  // 修改待辦內容：以伺服器回傳結果更新清單（失敗由 ActionItemRow 還原輸入）。
  const editItem = async (itemId: string, description: string) => {
    const { actionItem } = await editActionItem(await getIdToken(), itemId, description)
    setActionItems((prev) => prev.map((it) => (it.id === itemId ? actionItem : it)))
  }

  if (error) {
    return (
      <AppShell hideTopBar>
        <p className="py-10 text-center text-sm text-red-500">{error}</p>
      </AppShell>
    )
  }
  if (!meeting) {
    return (
      <AppShell hideTopBar>
        <Loader className="py-16" />
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
  // 頁籤 = 核心頁籤 + 已存在的選用文件頁籤（PRD / Tech Spec 不再預設出現）。
  const visibleTabs: Tab[] = [...coreTabs, ...optionalDocTabs.filter((t) => artifactByType.has(t))]
  // meta 統計皆前端可得：講者數取自逐字稿實際出現的代號。
  const speakerCount = new Set(meeting.transcriptSegments.map((s) => s.speaker)).size
  // speakerOrder 依逐字稿首次出現順序，供摘要卡片講者徽章配色（與逐字稿一致）。
  const speakerOrder = [...new Set(meeting.transcriptSegments.map((s) => s.speaker))]
  const hasSummary = Boolean(meeting.summary) || meeting.summarySections.some((s) => s.items.length > 0)
  // 摘要 / 待辦 頁另行渲染，docContent 僅供逐字稿與選用文件（PRD/Tech Spec）。
  const docContent =
    tab === 'transcript'
      ? meeting.transcript
      : tab === 'summary' || tab === 'action_items'
        ? ''
        : (artifactByType.get(tab)?.content ?? '')

  // 匯出的逐字稿以「顯示名: 內容」呈現（把代號 A/B 換成使用者設定的名字）。
  const transcriptExport =
    meeting.transcriptSegments.length > 0
      ? meeting.transcriptSegments
          .map((s) => `${meeting.speakerNames?.[s.speaker]?.trim() || s.speaker}: ${s.text}`)
          .join('\n')
      : meeting.transcript
  const exportContent =
    tab === 'transcript' ? transcriptExport : tab === 'summary' ? buildSummaryMarkdown(meeting) : docContent

  return (
    <AppShell hideTopBar>
      <div className="flex flex-col gap-1.5">
        <header className="flex items-center gap-2">
          <Link to="/meetings" className="btn btn-ghost size-11 shrink-0 px-0" aria-label="返回">
            <ChevronLeft className="size-5" />
          </Link>
          <EditableTitle
            meetingId={meeting.id}
            title={meeting.title}
            onRenamed={(title) => setMeeting({ ...meeting, title })}
          />
          {/* 完成後不再顯示狀態（只在處理中/失敗等變化階段提示），版面更簡約 */}
          {meeting.status !== 'completed' && <StatusBadge status={meeting.status} />}
          <button
            type="button"
            className="btn btn-ghost size-9 shrink-0 px-0 text-muted hover:text-red-500"
            aria-label="刪除會議"
            onClick={() => setConfirmDelete(true)}
          >
            <Trash2 className="size-4" />
          </button>
        </header>
        {/* meta：情境 · 時長 · 講者數 · 待辦數（皆前端計算，一眼定位這場的樣貌） */}
        <div className="flex flex-wrap items-center gap-x-2 gap-y-0.5 pl-[3.25rem] text-xs text-muted tabular-nums">
          <span>{scenarioLabels[meeting.scenario]}</span>
          {meeting.durationSeconds > 0 && (
            <>
              <span aria-hidden>·</span>
              <span>{fmtClock(meeting.durationSeconds)}</span>
            </>
          )}
          {speakerCount > 0 && (
            <>
              <span aria-hidden>·</span>
              <span>{speakerCount} 位講者</span>
            </>
          )}
          {meeting.status === 'completed' && (
            <>
              <span aria-hidden>·</span>
              <span>{actionItems.length} 待辦</span>
            </>
          )}
        </div>
      </div>

      {meeting.status === 'failed' && (
        <div className="flex flex-wrap items-center justify-between gap-3 rounded-xl border border-red-500/30 bg-red-500/5 px-4 py-3">
          <p className="m-0 text-sm text-red-500">處理失敗：{meeting.errorMessage || '未知錯誤'}</p>
          <button type="button" className="btn btn-secondary h-9" onClick={() => void retry()}>
            重新處理
          </button>
        </div>
      )}

      <AudioPlayer meetingId={meeting.id} durationSeconds={meeting.durationSeconds} audioRef={audioRef} />

      <nav className="sticky top-0 z-20 flex border-b border-border bg-bg">
        {visibleTabs.map((t) => (
          <button
            key={t}
            type="button"
            className={`-mb-px h-11 flex-1 cursor-pointer border-b-2 text-sm font-medium transition ${
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
        {tab === 'summary' ? (
          hasSummary ? (
            <div>
              {meeting.summary && (
                <p className="m-0 text-[15px] leading-6 font-medium text-fg">
                  {resolveSpeakerNames(meeting.summary, meeting.speakerNames ?? {})}
                </p>
              )}
              <SummarySections
                sections={meeting.summarySections}
                bare
                className={meeting.summary ? 'mt-3' : ''}
                speakerNames={meeting.speakerNames ?? {}}
                speakerOrder={speakerOrder}
              />
            </div>
          ) : (
            <p className="m-0 text-sm text-muted">
              {meeting.status === 'completed' ? '無摘要' : '處理完成後將顯示於此。'}
            </p>
          )
        ) : tab === 'action_items' ? (
          meeting.status === 'completed' ? (
            <div className="flex flex-col gap-3">
              <AddTodoForm
                meetingId={meeting.id}
                speakerNames={meeting.speakerNames ?? {}}
                speakerOrder={speakerOrder}
                onAdded={(it) => setActionItems((prev) => [...prev, it])}
              />
              <ActionItemList
                items={actionItems}
                onToggle={toggleItem}
                onEdit={editItem}
                speakerNames={meeting.speakerNames ?? {}}
                speakerOrder={speakerOrder}
              />
            </div>
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

      {/* 讓開貼底 mini-player，避免最後內容被蓋住 */}
      <div aria-hidden className="h-16" />

      {confirmDelete && (
        <Sheet onClose={() => setConfirmDelete(false)}>
          <p className="m-0 text-sm">
            確定刪除「{meeting.title}」？逐字稿、PRD、Tech Spec、待辦將一併刪除，此動作無法復原。
          </p>
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

/** 手動新增待辦：輸入 + 指派人 + 送出（POST /meetings/:id/action-items），成功後回呼交父層插入清單。 */
function AddTodoForm({
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

  const submit = async () => {
    const desc = text.trim()
    if (!desc || busy) return
    setBusy(true)
    try {
      const { actionItem } = await addMeetingActionItem(await getIdToken(), meetingId, desc, assignee)
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
        <p className="m-0 text-sm leading-7">
          {seg.text}
          <button
            type="button"
            className="ml-1 inline-flex translate-y-0.5 text-muted transition-colors hover:text-accent"
            aria-label="修正文字"
            onClick={() => {
              setDraft(seg.text)
              setEditing(true)
            }}
          >
            <Pencil className="size-3.5" />
          </button>
        </p>
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
    <Sheet onClose={onClose}>
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
    </Sheet>
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

  // 貼底常駐 mini-player：透過 Portal 掛到 body，避免 AppShell 帶 transform 的 <main>
  // 成為 fixed containing block（與 Sheet 同源問題）。閱讀長逐字稿時隨時可播/seek。
  return createPortal(
    <div className="fixed inset-x-0 bottom-0 z-30 border-t border-border bg-surface/95 pb-[env(safe-area-inset-bottom)] backdrop-blur">
      <div className="mx-auto flex h-14 max-w-xl items-center gap-1.5 px-3">
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
    </div>,
    document.body,
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
