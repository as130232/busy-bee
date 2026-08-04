import { Fragment } from 'react'
import { Play, Sparkles } from 'lucide-react'

import type { MeetingDetail } from '../services/api/client'
import { formatClock } from '../utils/format'
import { resolveSpeakerNames, speakerColor } from './speakerColor'

/** 可點時間戳：跳到音檔對應片段（onSeek 由詳情頁注入，與 mini-player 共用 audio）。 */
function TimeChip({ startMs, onSeek }: { startMs: number; onSeek: (sec: number) => void }) {
  return (
    <button
      type="button"
      onClick={() => onSeek(startMs / 1000)}
      aria-label={`從 ${formatClock(startMs / 1000)} 播放`}
      className="inline-flex shrink-0 items-center gap-1 rounded-full bg-accent/10 px-2 py-0.5 font-mono text-[11px] tabular-nums text-accent transition hover:bg-accent/20"
    >
      <Play className="size-3" />
      {formatClock(startMs / 1000)}
    </button>
  )
}

type Section = MeetingDetail['summarySections'][number]
type Point = Section['items'][number]

/**
 * 依情境產生的結構化摘要區塊通用渲染器（一套邏輯服務所有情境）。
 * 每個重點看資料決定樣式：有 heading → 卡片（標題＋說明＋講者徽章）；否則 → 純條列。
 * bare=true 時去掉每區塊的外框，供 hero 摘要卡內共用。
 * aiDividerBeforeType：在指定 type 的區塊前插一條「AI 延伸」分隔（想法情境用，區隔忠實摘要與 AI 生成）。
 */
export function SummarySections({
  sections,
  bare = false,
  className = '',
  speakerNames = {},
  speakerOrder = [],
  aiDividerBeforeType,
  onSeek,
}: {
  sections: Section[]
  bare?: boolean
  className?: string
  // speakerNames 講者代號→顯示名；speakerOrder 決定徽章配色，與逐字稿一致。
  speakerNames?: Record<string, string>
  speakerOrder?: string[]
  aiDividerBeforeType?: string
  // onSeek 有值時，帶 startMs 的重點顯示可點時間戳跳轉音檔（詳情頁注入）。
  onSeek?: (seconds: number) => void
}) {
  // 只顯示有內容的區塊，避免空區塊佔版面。
  const visible = sections.filter((s) => s.items.length > 0)
  if (visible.length === 0) return null

  return (
    <div className={`space-y-3 ${className}`.trim()}>
      {visible.map((s, i) => (
        <Fragment key={`${s.type}-${i}`}>
          {aiDividerBeforeType && s.type === aiDividerBeforeType && (
            <div className="flex items-center gap-2 pt-1 text-xs font-medium text-violet-500">
              <span className="h-px flex-1 bg-border" />
              <Sparkles className="size-3.5" />
              以下為 AI 延伸
              <span className="h-px flex-1 bg-border" />
            </div>
          )}
          <section className={bare ? '' : 'rounded-xl border border-border bg-surface px-4 py-3'}>
            <h3 className="m-0 mb-1.5 text-sm font-semibold text-fg">{s.title}</h3>
            <div className="space-y-1.5">
              {s.items.map((it, j) => (
                <PointRow
                  key={j}
                  point={it}
                  speakerNames={speakerNames}
                  speakerOrder={speakerOrder}
                  onSeek={onSeek}
                />
              ))}
            </div>
          </section>
        </Fragment>
      ))}
    </div>
  )
}

// PointRow 有 heading 渲染成卡片，否則渲染成單行條列；有 startMs + onSeek 時顯示可點時間戳。
function PointRow({
  point,
  speakerNames,
  speakerOrder,
  onSeek,
}: {
  point: Point
  speakerNames: Record<string, string>
  speakerOrder: string[]
  onSeek?: (seconds: number) => void
}) {
  // 內文/標題裡的講者代號（如 B）也換成顯示名，與徽章一致跟著改名連動。
  const heading = resolveSpeakerNames(point.heading ?? '', speakerNames)
  const text = resolveSpeakerNames(point.text, speakerNames)
  const canSeek = onSeek && point.startMs != null

  if (!point.heading) {
    return (
      <div className="flex items-start gap-2 text-sm leading-6 text-fg">
        <span className="select-none text-muted">•</span>
        <span className="flex-1">{text}</span>
        {canSeek && <TimeChip startMs={point.startMs!} onSeek={onSeek} />}
      </div>
    )
  }
  return (
    <div className="rounded-lg border border-border/60 bg-surface/60 px-3 py-2">
      <div className="flex items-start justify-between gap-2">
        <span className="text-sm font-semibold text-fg">{heading}</span>
        <div className="flex shrink-0 items-center gap-1.5">
          {canSeek && <TimeChip startMs={point.startMs!} onSeek={onSeek} />}
          {point.speaker && (
            <span
              className={`rounded-full px-2 py-0.5 text-xs font-medium ${speakerColor(point.speaker, speakerOrder)}`}
            >
              {speakerNames[point.speaker]?.trim() || point.speaker}
            </span>
          )}
        </div>
      </div>
      {text && <p className="m-0 mt-0.5 text-sm leading-6 text-muted">{text}</p>}
    </div>
  )
}
