import type { MeetingDetail } from '../services/api/client'
import { resolveSpeakerNames, speakerColor } from './speakerColor'

type Section = MeetingDetail['summarySections'][number]
type Point = Section['items'][number]

/**
 * 依情境產生的結構化摘要區塊通用渲染器（一套邏輯服務所有情境）。
 * 每個重點看資料決定樣式：有 heading → 卡片（標題＋說明＋講者徽章）；否則 → 純條列。
 * bare=true 時去掉每區塊的外框，供 hero 摘要卡內共用。
 */
export function SummarySections({
  sections,
  bare = false,
  className = '',
  speakerNames = {},
  speakerOrder = [],
}: {
  sections: Section[]
  bare?: boolean
  className?: string
  // speakerNames 講者代號→顯示名；speakerOrder 決定徽章配色，與逐字稿一致。
  speakerNames?: Record<string, string>
  speakerOrder?: string[]
}) {
  // 只顯示有內容的區塊，避免空區塊佔版面。
  const visible = sections.filter((s) => s.items.length > 0)
  if (visible.length === 0) return null

  return (
    <div className={`space-y-3 ${className}`.trim()}>
      {visible.map((s, i) => (
        <section
          key={`${s.type}-${i}`}
          className={bare ? '' : 'rounded-xl border border-border bg-surface px-4 py-3'}
        >
          <h3 className="m-0 mb-1.5 text-sm font-semibold text-fg">{s.title}</h3>
          <div className="space-y-1.5">
            {s.items.map((it, j) => (
              <PointRow
                key={j}
                point={it}
                speakerNames={speakerNames}
                speakerOrder={speakerOrder}
              />
            ))}
          </div>
        </section>
      ))}
    </div>
  )
}

// PointRow 有 heading 渲染成卡片，否則渲染成單行條列。
function PointRow({
  point,
  speakerNames,
  speakerOrder,
}: {
  point: Point
  speakerNames: Record<string, string>
  speakerOrder: string[]
}) {
  // 內文/標題裡的講者代號（如 B）也換成顯示名，與徽章一致跟著改名連動。
  const heading = resolveSpeakerNames(point.heading ?? '', speakerNames)
  const text = resolveSpeakerNames(point.text, speakerNames)

  if (!point.heading) {
    return (
      <div className="flex gap-2 text-sm leading-6 text-fg">
        <span className="select-none text-muted">•</span>
        <span>{text}</span>
      </div>
    )
  }
  return (
    <div className="rounded-lg border border-border/60 bg-surface/60 px-3 py-2">
      <div className="flex items-start justify-between gap-2">
        <span className="text-sm font-semibold text-fg">{heading}</span>
        {point.speaker && (
          <span
            className={`shrink-0 rounded-full px-2 py-0.5 text-xs font-medium ${speakerColor(point.speaker, speakerOrder)}`}
          >
            {speakerNames[point.speaker]?.trim() || point.speaker}
          </span>
        )}
      </div>
      {text && <p className="m-0 mt-0.5 text-sm leading-6 text-muted">{text}</p>}
    </div>
  )
}
