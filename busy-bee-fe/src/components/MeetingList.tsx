import { Link } from 'react-router-dom'

import { StatusBadge } from './StatusBadge'
import { scenarioLabels, type Meeting } from '../services/api/client'

/** 情境標籤（會議/閒聊）；閒聊用不同色以利辨識。 */
function ScenarioTag({ scenario }: { scenario: Meeting['scenario'] }) {
  const label = scenarioLabels[scenario]
  if (!label) return null
  const tone =
    scenario === 'casual' ? 'bg-violet-500/10 text-violet-500' : 'bg-accent/10 text-accent'
  return <span className={`rounded px-1.5 py-0.5 text-xs font-medium ${tone}`}>{label}</span>
}

// formatDuration 以「X 分 Y 秒」呈現時長（不足 1 分只顯示秒；0 秒不顯示）。
function formatDuration(totalSeconds: number): string {
  if (totalSeconds <= 0) return ''
  const min = Math.floor(totalSeconds / 60)
  const sec = totalSeconds % 60
  if (min === 0) return `${sec} 秒`
  if (sec === 0) return `${min} 分`
  return `${min} 分 ${sec} 秒`
}

const dateTimeFmt: Intl.DateTimeFormatOptions = {
  year: 'numeric',
  month: '2-digit',
  day: '2-digit',
  hour: '2-digit',
  minute: '2-digit',
  hour12: false,
}

/** 副標題：日期時間在前，時長以強調 tag 呈現以區隔。 */
function Subtitle({ m }: { m: Meeting }) {
  if (m.status === 'scheduled' && m.scheduledAt) {
    return (
      <span className="mt-0.5 block font-mono text-xs text-muted">
        排定 {new Date(m.scheduledAt).toLocaleString('zh-TW', dateTimeFmt)}
      </span>
    )
  }
  const dur = formatDuration(m.durationSeconds)
  return (
    <span className="mt-0.5 flex items-center gap-1.5 text-xs text-muted">
      <span className="font-mono">{new Date(m.createdAt).toLocaleString('zh-TW', dateTimeFmt)}</span>
      {dur && <span className="tabular-nums text-muted/80">· {dur}</span>}
    </span>
  )
}

/** 會議列表（單一 surface 容器 + 分隔線，整列可點）。 */
export function MeetingList({ meetings, emptyText }: { meetings: Meeting[]; emptyText: string }) {
  if (meetings.length === 0) {
    return <p className="py-10 text-center text-sm text-muted">{emptyText}</p>
  }

  return (
    <ul className="divide-y divide-border overflow-hidden rounded-xl border border-border bg-surface">
      {meetings.map((m, i) => (
        <li
          key={m.id}
          className="animate-fade-in-up"
          style={{ animationDelay: `${Math.min(i, 8) * 45}ms` }}
        >
          <Link
            to={`/meetings/${m.id}`}
            className="flex items-center gap-3 px-4 py-3.5 transition hover:bg-surface-hover active:bg-surface-hover"
          >
            <span className="min-w-0 flex-1">
              <span className="flex items-center gap-2">
                <span className="shrink-0">
                  <ScenarioTag scenario={m.scenario} />
                </span>
                <span className="min-w-0 truncate text-sm font-medium">{m.title}</span>
              </span>
              {m.summary && <span className="mt-0.5 block truncate text-xs text-muted">{m.summary}</span>}
              <Subtitle m={m} />
              {m.matchSnippet && (
                <span className="mt-1 block truncate text-xs italic text-muted">…{m.matchSnippet}…</span>
              )}
            </span>
            <StatusBadge status={m.status} />
          </Link>
        </li>
      ))}
    </ul>
  )
}
