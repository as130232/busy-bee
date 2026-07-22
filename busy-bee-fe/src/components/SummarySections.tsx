import type { MeetingDetail } from '../services/api/client'

type Section = MeetingDetail['summarySections'][number]

/**
 * 依情境產生的結構化摘要區塊通用渲染器（一套邏輯服務所有情境）。
 * bare=true 時去掉每區塊的卡片外框，供 hero 摘要卡內共用（單一容器內呈現導言＋重點）。
 */
export function SummarySections({
  sections,
  bare = false,
  className = '',
}: {
  sections: Section[]
  bare?: boolean
  className?: string
}) {
  // 只顯示有內容的區塊，避免空區塊佔版面。
  const visible = sections.filter((s) => s.items.length > 0)
  if (visible.length === 0) return null

  return (
    <div className={`${bare ? 'space-y-2.5' : 'space-y-3'} ${className}`.trim()}>
      {visible.map((s, i) => (
        <section
          key={`${s.type}-${i}`}
          className={bare ? '' : 'rounded-xl border border-border bg-surface px-4 py-3'}
        >
          <h3 className="m-0 mb-1.5 text-sm font-semibold text-fg">{s.title}</h3>
          <ul className="m-0 list-disc space-y-1 pl-5 text-sm leading-6 text-fg">
            {s.items.map((it, j) => (
              <li key={j}>{it}</li>
            ))}
          </ul>
        </section>
      ))}
    </div>
  )
}
