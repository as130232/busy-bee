import { useState, type ReactNode } from 'react'
import { ChevronDown } from 'lucide-react'

/**
 * 可收納的區塊卡：標題列可點擊展開/收合，資料多時收起以免畫面過長。
 * 標題右側顯示筆數與展開狀態箭頭；內容在收合時不渲染於版面（僅隱藏顯示）。
 */
export function CollapsibleSection({
  title,
  icon,
  count,
  defaultOpen = true,
  children,
}: {
  title: string
  icon?: ReactNode
  count?: number
  defaultOpen?: boolean
  children: ReactNode
}) {
  const [open, setOpen] = useState(defaultOpen)

  return (
    <section className="animate-fade-in-up rounded-xl border border-border bg-surface p-4">
      <button
        type="button"
        onClick={() => setOpen((v) => !v)}
        aria-expanded={open}
        className="flex w-full items-center gap-2 text-sm font-semibold"
      >
        {icon}
        {title}
        {count !== undefined && <span className="text-muted">{count}</span>}
        <ChevronDown
          className={`ml-auto size-4 shrink-0 text-muted transition-transform ${open ? '' : '-rotate-90'}`}
        />
      </button>
      {open && <div className="mt-2">{children}</div>}
    </section>
  )
}
