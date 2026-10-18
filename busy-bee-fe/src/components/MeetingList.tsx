import type { ReactNode } from 'react'
import { Link } from 'react-router-dom'
import { Link2 } from 'lucide-react'

import { EmptyState } from './EmptyState'
import { scenarioThemes } from './scenarioTheme'
import { StatusBadge } from './StatusBadge'
import { displayStatus } from '../utils/meetingStatus'
import { scenarioLabels, type Meeting } from '../services/api/client'
import { dateGroupLabel, formatDateTime, formatDuration, formatShortDateTime } from '../utils/format'

/** 每列最多顯示的標籤數，其餘以 +N 收合，避免列高不一。 */
const MAX_TAGS = 2

/** 第三行 meta：時間・時長・匯入・標籤（排程則顯示排定時間）。 */
function Meta({ m }: { m: Meeting }) {
  if (m.status === 'scheduled' && m.scheduledAt && !m.liveStartedAt) {
    return <span className="font-mono">排定 {formatDateTime(m.scheduledAt)}</span>
  }
  const dur = formatDuration(m.durationSeconds)
  const tags = m.tags ?? []
  return (
    <>
      <span className="font-mono">{formatShortDateTime(m.createdAt)}</span>
      {dur && <span>· {dur}</span>}
      {m.imported && (
        <span className="inline-flex items-center gap-0.5" title="匯入">
          · <Link2 className="size-3" aria-hidden />
          <span className="sr-only">匯入</span>
        </span>
      )}
      {tags.slice(0, MAX_TAGS).map((t) => (
        <span key={t} className="max-w-24 truncate rounded-full border border-border px-1.5 text-micro">
          #{t}
        </span>
      ))}
      {tags.length > MAX_TAGS && <span className="text-micro">+{tags.length - MAX_TAGS}</span>}
    </>
  )
}

/** 單列：①情境色點＋標題＋狀態 ②摘要或搜尋命中片段 ③meta。固定三行，掃讀不跳動。 */
function Row({ m, index }: { m: Meeting; index: number }) {
  const snippet = m.matchSnippet ? `…${m.matchSnippet}…` : m.summary
  return (
    <li className="animate-fade-in-up" style={{ animationDelay: `${Math.min(index, 8) * 40}ms` }}>
      <Link
        to={`/meetings/${m.id}`}
        className="flex flex-col gap-0.5 px-4 py-3 transition outline-none hover:bg-surface-hover focus-visible:bg-surface-hover active:bg-surface-hover"
      >
        <span className="flex items-center gap-2">
          <span aria-hidden className={`size-2 shrink-0 rounded-full ${scenarioThemes[m.scenario]?.dot ?? 'bg-muted'}`} />
          <span className="sr-only">{scenarioLabels[m.scenario]}</span>
          <span className="min-w-0 flex-1 truncate text-sm font-medium text-fg">{m.title}</span>
          {/* 完成後不顯示狀態徽章，只在處理中/排程/失敗等階段提示 */}
          {m.status !== 'completed' && <StatusBadge status={displayStatus(m)} />}
        </span>
        {snippet && (
          <span className={`truncate pl-4 text-xs text-muted ${m.matchSnippet ? 'italic' : ''}`}>{snippet}</span>
        )}
        <span className="flex min-w-0 items-center gap-1.5 overflow-hidden pl-4 text-xs whitespace-nowrap text-muted tabular-nums">
          <Meta m={m} />
        </span>
      </Link>
    </li>
  )
}

function Card({ children }: { children: ReactNode }) {
  return (
    <ul className="m-0 list-none divide-y divide-border overflow-hidden rounded-xl border border-border bg-surface p-0">
      {children}
    </ul>
  )
}

/**
 * 會議列表（單一 surface 容器 + 分隔線，整列可點）。
 * groupByDate：依建立時間分組（今天／昨天／近 7 天／年月），組名 sticky 於捲動容器頂端。
 */
export function MeetingList({
  meetings,
  emptyText,
  emptyDescription,
  emptyAction,
  groupByDate = false,
}: {
  meetings: Meeting[]
  emptyText: string
  emptyDescription?: string
  emptyAction?: ReactNode
  groupByDate?: boolean
}) {
  if (meetings.length === 0) {
    return <EmptyState title={emptyText} description={emptyDescription} action={emptyAction} />
  }

  if (!groupByDate) {
    return (
      <Card>
        {meetings.map((m, i) => (
          <Row key={m.id} m={m} index={i} />
        ))}
      </Card>
    )
  }

  // 依組名歸併（保留首次出現順序；列表本身為新→舊）。
  const now = new Date()
  const byLabel = new Map<string, Meeting[]>()
  for (const m of meetings) {
    const label = dateGroupLabel(m.createdAt, now)
    byLabel.set(label, [...(byLabel.get(label) ?? []), m])
  }
  const groups = [...byLabel].map(([label, items]) => ({ label, items }))

  let index = 0
  return (
    <div className="flex flex-col gap-4">
      {groups.map((g) => (
        <section key={g.label} className="flex flex-col gap-2">
          <h2 className="sticky top-0 z-10 m-0 -mx-4 bg-bg/95 px-5 py-1 text-xs font-medium text-muted backdrop-blur">
            {g.label}
          </h2>
          <Card>
            {g.items.map((m) => (
              <Row key={m.id} m={m} index={index++} />
            ))}
          </Card>
        </section>
      ))}
    </div>
  )
}
