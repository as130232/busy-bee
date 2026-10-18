import { LoaderCircle, RefreshCw } from 'lucide-react'

import type { LiveSummary } from '../../services/api/client'

function hhmm(ts: number): string {
  const d = new Date(ts)
  return `${String(d.getHours()).padStart(2, '0')}:${String(d.getMinutes()).padStart(2, '0')}`
}

/** 即時重點（跟上進度）：目前在討論什麼＋已談到的重點；約每 1.5 分鐘自動更新，也可手動更新。 */
export function LiveNotesPanel({
  notes,
  busy,
  error,
  updatedAt,
  disabled,
  onRefresh,
}: {
  notes: LiveSummary
  busy: boolean
  error: string | null
  updatedAt: number | null
  disabled: boolean
  onRefresh: () => void
}) {
  const empty = !notes.current && notes.points.length === 0
  return (
    <div className="flex min-h-0 flex-1 flex-col gap-3 overflow-y-auto">
      <div className="flex items-center gap-2 text-xs text-muted">
        {busy ? (
          <span className="flex items-center gap-1">
            <LoaderCircle className="size-3.5 animate-spin" />
            更新中…
          </span>
        ) : updatedAt ? (
          <span>更新於 {hhmm(updatedAt)}</span>
        ) : (
          <span>累積一些對話後會自動整理</span>
        )}
        <button
          type="button"
          className="btn btn-ghost ml-auto h-8 px-2 text-xs"
          disabled={busy || disabled}
          onClick={onRefresh}
        >
          <RefreshCw className="size-3.5" />
          立即更新
        </button>
      </div>
      {error && <p className="m-0 text-xs text-warning">{error}（保留上一版，稍後自動重試）</p>}
      {empty ? (
        <p className="m-0 py-6 text-center text-sm text-muted">還沒有重點</p>
      ) : (
        <>
          {notes.current && (
            <div className="rounded-lg bg-accent/10 px-3 py-2">
              <p className="m-0 text-xs text-muted">目前在討論</p>
              <p className="m-0 text-sm font-medium">{notes.current}</p>
            </div>
          )}
          <ol className="m-0 flex flex-col gap-1.5 pl-5 text-sm leading-6">
            {notes.points.map((p, i) => (
              <li key={i}>{p}</li>
            ))}
          </ol>
        </>
      )}
    </div>
  )
}
