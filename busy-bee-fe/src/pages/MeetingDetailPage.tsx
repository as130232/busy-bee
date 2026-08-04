import { useMemo, useRef, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import ReactMarkdown from 'react-markdown'
import { ChevronLeft, Trash2 } from 'lucide-react'

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
import { SummarySections } from '../components/SummarySections'
import { TagEditor } from '../components/TagEditor'
import { TranscriptEditor } from '../components/TranscriptEditor'
import { resolveSpeakerNames } from '../components/speakerColor'
import { useMeetingDetail } from '../hooks/useMeetingDetail'
import { scenarioLabels, type MeetingDetail } from '../services/api/client'
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
      <div className="flex flex-col gap-1.5">
        <header className="flex items-center gap-2">
          <Link to="/meetings" className="btn btn-ghost size-11 shrink-0 px-0" aria-label="返回">
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
              <span>{formatClock(meeting.durationSeconds)}</span>
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

      <TagEditor meeting={meeting} onUpdated={setMeeting} />

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
            className={`-mb-px h-11 flex-1 cursor-pointer border-b-2 text-sm font-medium transition ${tab === t ? 'border-accent text-fg' : 'border-transparent text-muted hover:text-fg'
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
              className="btn btn-primary flex-1 bg-red-600"
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
