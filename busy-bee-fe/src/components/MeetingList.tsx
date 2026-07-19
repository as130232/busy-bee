import { Link } from 'react-router-dom'

import { StatusBadge } from './StatusBadge'
import type { Meeting } from '../services/api/client'

function subtitle(m: Meeting): string {
  if (m.status === 'scheduled' && m.scheduledAt) {
    return `排定 ${new Date(m.scheduledAt).toLocaleString()}`
  }
  const dur = m.durationSeconds > 0 ? `${Math.round(m.durationSeconds / 60)} 分鐘 · ` : ''
  return `${dur}${new Date(m.createdAt).toLocaleDateString()}`
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
              <span className="block truncate text-sm font-medium">{m.title}</span>
              <span className="mt-0.5 block font-mono text-xs text-muted">{subtitle(m)}</span>
            </span>
            <StatusBadge status={m.status} />
          </Link>
        </li>
      ))}
    </ul>
  )
}
