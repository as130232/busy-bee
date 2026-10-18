import { useMemo, useRef, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import ReactMarkdown from 'react-markdown'
import { ChevronLeft, ChevronRight, FileText, MoreHorizontal, Sparkles, Trash2, X } from 'lucide-react'

import { formatClock } from '../utils/format'
import { ActionItemList } from '../components/ActionItemList'
import { AppShell } from '../components/AppShell'
import { AudioPlayer } from '../components/AudioPlayer'
import { EditableMeetingTitle } from '../components/EditableMeetingTitle'
import { ExportBar } from '../components/ExportBar'
import { Loader } from '../components/Loader'
import { MeetingTodoForm } from '../components/MeetingTodoForm'
import { Sheet } from '../components/Sheet'
import { StatusBadge } from '../components/StatusBadge'
import { CorrectionPanel } from '../components/CorrectionPanel'
import { SummarySections } from '../components/SummarySections'
import { TagEditor } from '../components/TagEditor'
import { TranscriptEditor } from '../components/TranscriptEditor'
import { resolveSpeakerNames } from '../components/speakerColor'
import { useMeetingDetail } from '../hooks/useMeetingDetail'
import { scenarioThemes } from '../components/scenarioTheme'
import { scenarioLabels, type MeetingDetail } from '../services/api/client'
import { LiveNotesSection } from '../components/LiveNotesSection'
import { MeetingContextsSection } from '../components/MeetingContextsSection'
import { LiveUnfinishedDetail } from './LiveUnfinishedDetail'
import { ScheduledMeetingDetail } from './ScheduledMeetingDetail'

type Tab = 'summary' | 'prd' | 'tech_spec' | 'action_items' | 'transcript'

const tabLabels: Record<Tab, string> = {
  summary: '摘要',
  prd: 'PRD',
  tech_spec: 'Tech Spec',
  action_items: '待辦',
  transcript: '逐字稿',
}

// 核心頁籤一律顯示；PRD / Tech Spec 已改為選用，僅在對應 artifact 存在時才出現。
const coreTabs: Tab[] = ['summary', 'transcript', 'action_items']
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
  const {
    meeting,
    setMeeting,
    reload,
    artifacts,
    actionItems,
    setActionItems,
    error,
    retry,
    remove,
    toggleItem,
    editItem,
    removeItem,
  } = useMeetingDetail(id)
  const [tab, setTab] = useState<Tab>('summary')
  const [confirmDelete, setConfirmDelete] = useState(false)
  // 「⋯」選單與其子視窗：低頻動作（AI 校正、會前資料、刪除）收進這裡，讓內文更早出現。
  const [menu, setMenu] = useState<null | 'actions' | 'contexts'>(null)
  const [correctionSignal, setCorrectionSignal] = useState(0)
  const audioRef = useRef<HTMLAudioElement>(null)

  // 點逐字稿時間碼 → 音檔跳至該處並播放。
  const seekAudio = (seconds: number) => {
    const a = audioRef.current
    if (!a) return
    a.currentTime = seconds
    void a.play()
  }

  // speakerOrder 依逐字稿首次出現順序，供摘要卡片講者徽章配色（與逐字稿一致）。
  // 在早期 return 之前計算並 memoize（遵守 hooks 規則）：僅逐字稿變動時才重建陣列，
  // 避免待辦/標籤更新時連帶重算所有講者配色。
  const speakerOrder = useMemo(
    () => [...new Set((meeting?.transcriptSegments ?? []).map((s) => s.speaker))],
    [meeting?.transcriptSegments],
  )

  if (error) {
    return (
      <AppShell hideTopBar>
        <p className="py-10 text-center text-sm text-danger">{error}</p>
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

  // 即時會議進行中／已結束但錄音未上傳
  if (meeting.status === 'live' || (meeting.status === 'scheduled' && meeting.liveStartedAt)) {
    return <LiveUnfinishedDetail meeting={meeting} onUploaded={() => void reload()} />
  }

  if (meeting.status === 'scheduled') {
    return <ScheduledMeetingDetail meeting={meeting} onChanged={(m) => setMeeting({ ...meeting, ...m })} />
  }

  const artifactByType = new Map(artifacts.map((a) => [a.type, a]))
  // 頁籤 = 核心頁籤 + 已存在的選用文件頁籤（PRD / Tech Spec 不再預設出現）。
  const visibleTabs: Tab[] = [...coreTabs, ...optionalDocTabs.filter((t) => artifactByType.has(t))]
  // meta 統計皆前端可得：講者數取自逐字稿實際出現的代號（與 speakerOrder 同源）。
  const speakerCount = speakerOrder.length
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
      <div className="flex flex-col gap-1">
        <header className="flex items-center gap-1">
          <Link to="/meetings" className="btn btn-ghost -ml-2 size-11 shrink-0 px-0" aria-label="返回">
            <ChevronLeft className="size-5" />
          </Link>
          <EditableMeetingTitle
            meetingId={meeting.id}
            title={meeting.title}
            onRenamed={(title) => setMeeting({ ...meeting, title })}
          />
          {/* 完成後不再顯示狀態（只在處理中/失敗等變化階段提示），版面更簡約 */}
          {meeting.status !== 'completed' && <StatusBadge status={meeting.status} />}
          <button
            type="button"
            className="btn btn-ghost size-11 shrink-0 px-0 text-muted"
            aria-label="更多動作"
            onClick={() => setMenu('actions')}
          >
            <MoreHorizontal className="size-5" />
          </button>
        </header>
        {/* meta：情境 · 時長 · 講者數 · 待辦數，標籤接在行尾（一行掌握這場的樣貌） */}
        <div className="flex flex-wrap items-center gap-x-2 gap-y-1.5 pl-9 text-xs text-muted tabular-nums">
          <span className={`rounded px-1.5 py-0.5 font-medium ${scenarioThemes[meeting.scenario].chip}`}>
            {scenarioLabels[meeting.scenario]}
          </span>
          {meeting.durationSeconds > 0 && <span>{formatClock(meeting.durationSeconds)}</span>}
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
          <TagEditor meeting={meeting} onUpdated={setMeeting} />
        </div>
      </div>

      {meeting.status === 'failed' && (
        <div className="flex flex-wrap items-center justify-between gap-3 rounded-xl border border-danger/30 bg-danger/5 px-4 py-3">
          <p className="m-0 text-sm text-danger">處理失敗：{meeting.errorMessage || '未知錯誤'}</p>
          <button type="button" className="btn btn-secondary h-9" onClick={() => void retry()}>
            重新處理
          </button>
        </div>
      )}

      <AudioPlayer meetingId={meeting.id} durationSeconds={meeting.durationSeconds} audioRef={audioRef} />

      {/* 分頁列可橫向捲動（PRD/Tech Spec 出現時不擠），匯出動作貼在右側 */}
      <div className="sticky top-0 z-20 -mx-4 flex items-center gap-2 border-b border-border bg-bg/95 px-4 backdrop-blur">
        <nav className="flex min-w-0 flex-1 gap-5 overflow-x-auto [scrollbar-width:none]">
          {visibleTabs.map((t) => (
            <button
              key={t}
              type="button"
              className={`-mb-px h-11 shrink-0 cursor-pointer border-b-2 text-sm font-medium whitespace-nowrap transition outline-none focus-visible:text-fg ${
                tab === t ? 'border-accent text-fg' : 'border-transparent text-muted hover:text-fg'
              }`}
              onClick={() => setTab(t)}
            >
              {tabLabels[t]}
            </button>
          ))}
        </nav>
        {tab !== 'action_items' && exportContent && (
          <ExportBar content={exportContent} filename={`${meeting.title}-${tab}`} />
        )}
      </div>

      <CorrectionPanel meeting={meeting} onChanged={() => void reload()} startSignal={correctionSignal} />

      {meeting.liveNotes && <LiveNotesSection notes={meeting.liveNotes} />}

      <article key={tab} className="animate-fade-in rounded-xl border border-border bg-surface px-5 py-4">
        {tab === 'summary' ? (
          hasSummary ? (
            <div>
              {meeting.summary && (
                <p className="m-0 text-body leading-6 font-medium text-fg">
                  {resolveSpeakerNames(meeting.summary, meeting.speakerNames ?? {})}
                </p>
              )}
              <SummarySections
                sections={meeting.summarySections}
                bare
                className={meeting.summary ? 'mt-3' : ''}
                speakerNames={meeting.speakerNames ?? {}}
                speakerOrder={speakerOrder}
                aiDividerBeforeType={meeting.scenario === 'idea' ? 'expansion' : undefined}
                onSeek={seekAudio}
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
              <MeetingTodoForm
                meetingId={meeting.id}
                speakerNames={meeting.speakerNames ?? {}}
                speakerOrder={speakerOrder}
                onAdded={(it) => setActionItems((prev) => [...prev, it])}
              />
              <ActionItemList
                items={actionItems}
                onToggle={toggleItem}
                onEdit={editItem}
                onRemove={removeItem}
                speakerNames={meeting.speakerNames ?? {}}
                speakerOrder={speakerOrder}
              />
            </div>
          ) : (
            <p className="m-0 text-sm text-muted">處理完成後將顯示於此。</p>
          )
        ) : tab === 'transcript' && meeting.transcriptSegments.length > 0 ? (
          <TranscriptEditor meeting={meeting} onUpdated={setMeeting} onSeek={seekAudio} />
        ) : docContent ? (
          tab === 'transcript' ? (
            <p className="text-sm leading-7 whitespace-pre-wrap">{docContent}</p>
          ) : (
            <div className="prose prose-sm prose-zinc dark:prose-invert max-w-none prose-headings:font-semibold prose-h1:text-xl prose-h2:mt-6 prose-h2:border-b prose-h2:border-border prose-h2:pb-1.5 prose-h2:text-base">
              <ReactMarkdown skipHtml>{docContent}</ReactMarkdown>
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

      {menu === 'actions' && (
        <Sheet onClose={() => setMenu(null)}>
          <ul className="m-0 flex list-none flex-col p-0">
            {meeting.status === 'completed' && (
              <MenuItem
                icon={<Sparkles className="size-5 text-accent" />}
                label="AI 重新聽錄音校正"
                hint="修正逐字稿並更新摘要"
                onClick={() => {
                  setMenu(null)
                  setCorrectionSignal((n) => n + 1)
                }}
              />
            )}
            <MenuItem
              icon={<FileText className="size-5 text-muted" />}
              label="會前資料"
              hint="提供專有名詞、背景，讓 AI 校正更準"
              onClick={() => setMenu('contexts')}
            />
            <MenuItem
              icon={<Trash2 className="size-5 text-danger" />}
              label="刪除紀錄"
              danger
              onClick={() => {
                setMenu(null)
                setConfirmDelete(true)
              }}
            />
          </ul>
        </Sheet>
      )}

      {menu === 'contexts' && (
        <Sheet onClose={() => setMenu(null)} className="max-h-[85dvh] overflow-y-auto">
          <div className="flex items-center justify-between">
            <h2 className="m-0 text-heading font-semibold">會前資料</h2>
            <button type="button" className="btn btn-ghost size-9 px-0" aria-label="關閉" onClick={() => setMenu(null)}>
              <X className="size-4" />
            </button>
          </div>
          <MeetingContextsSection
            meetingId={meeting.id}
            bare
            hint="背景資料會讓 AI 校正更準（專有名詞、術語），不會被當成會議內容。"
          />
        </Sheet>
      )}

      {confirmDelete && (
        <Sheet onClose={() => setConfirmDelete(false)}>
          <p className="m-0 text-sm">
            確定刪除「{meeting.title}」？摘要、逐字稿、待辦事項將一併刪除，此動作無法復原。
          </p>
          <div className="flex gap-2">
            <button type="button" className="btn btn-secondary flex-1" onClick={() => setConfirmDelete(false)}>
              取消
            </button>
            <button
              type="button"
              className="btn btn-danger flex-1"
              onClick={() => void remove().then((ok) => { if (!ok) setConfirmDelete(false) })}
            >
              刪除
            </button>
          </div>
        </Sheet>
      )}
    </AppShell>
  )
}

/** 「⋯」選單列：icon + 標題（+ 說明），整列可點。 */
function MenuItem({
  icon,
  label,
  hint,
  danger = false,
  onClick,
}: {
  icon: React.ReactNode
  label: string
  hint?: string
  danger?: boolean
  onClick: () => void
}) {
  return (
    <li>
      <button
        type="button"
        onClick={onClick}
        className="flex w-full items-center gap-3 rounded-xl px-2 py-3 text-left transition outline-none hover:bg-surface-hover focus-visible:bg-surface-hover"
      >
        {icon}
        <span className="min-w-0 flex-1">
          <span className={`block text-sm font-medium ${danger ? 'text-danger' : 'text-fg'}`}>{label}</span>
          {hint && <span className="block text-xs text-muted">{hint}</span>}
        </span>
        {!danger && <ChevronRight className="size-4 shrink-0 text-muted" />}
      </button>
    </li>
  )
}
