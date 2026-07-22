import type { MeetingDetail } from '../services/api/client'

type Section = MeetingDetail['summarySections'][number]

/** 依情境產生的結構化摘要區塊通用渲染器（一套邏輯服務所有情境）。 */
export function SummarySections({ sections }: { sections: Section[] }) {
  // 只顯示有內容的區塊，避免空區塊佔版面。
  const visible = sections.filter((s) => s.items.length > 0)
  if (visible.length === 0) return null

  return (
    <div className="space-y-3">
      {visible.map((s, i) => (
        <section
          key={`${s.type}-${i}`}
          className="rounded-xl border border-border bg-surface px-4 py-3"
        >
          <h3 className="m-0 mb-2 text-sm font-semibold text-fg">{s.title}</h3>
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
