import { useCallback, useEffect, useState } from 'react'
import { Link, useSearchParams } from 'react-router-dom'
import { Search } from 'lucide-react'

import { AppShell } from '../components/AppShell'
import { NotificationToggle } from '../components/NotificationToggle'
import { PendingActionItems } from '../components/PendingActionItems'
import { RecorderPanel } from '../components/RecorderPanel'
import { ScheduleForm } from '../components/ScheduleForm'
import { StatusBadge } from '../components/StatusBadge'
import { UploadZone } from '../components/UploadZone'
import { useAuth } from '../hooks/useAuth'
import { useMeetingStatusSocket } from '../hooks/useMeetingStatusSocket'
import { listMeetings, type Meeting } from '../services/api/client'
import { getIdToken } from '../services/token'

export function DashboardPage() {
  const { user } = useAuth()
  const [meetings, setMeetings] = useState<Meeting[]>([])
  const [search, setSearch] = useState('')
  const [loadError, setLoadError] = useState<string | null>(null)

  // 由提醒推播深連結（/?record=1）進入時高亮錄音鈕，3 秒後清除 query
  const [searchParams, setSearchParams] = useSearchParams()
  const [highlightRecorder, setHighlightRecorder] = useState(false)
  useEffect(() => {
    if (searchParams.get('record') !== '1') return
    setHighlightRecorder(true)
    const timer = setTimeout(() => {
      setHighlightRecorder(false)
      setSearchParams({}, { replace: true })
    }, 3000)
    return () => clearTimeout(timer)
  }, [searchParams, setSearchParams])

  const load = useCallback(async (keyword: string) => {
    try {
      const { meetings } = await listMeetings(await getIdToken(), keyword)
      setMeetings(meetings)
      setLoadError(null)
    } catch (e) {
      setLoadError(e instanceof Error ? e.message : '會議列表載入失敗')
    }
  }, [])

  useEffect(() => {
    const timer = setTimeout(() => void load(search), search ? 300 : 0) // 搜尋防抖
    return () => clearTimeout(timer)
  }, [search, load])

  useMeetingStatusSocket((e) => {
    setMeetings((prev) =>
      prev.map((m) =>
        m.id === e.meetingId
          ? { ...m, status: e.status as Meeting['status'], errorMessage: e.errorMessage }
          : m,
      ),
    )
  })

  if (!user) return null // RequireAuth 已保證，防禦性判斷

  return (
    <AppShell>
      <RecorderPanel onUploaded={() => void load(search)} highlight={highlightRecorder} />
      <UploadZone onUploaded={() => void load(search)} />

      <div className="flex flex-wrap items-center justify-between gap-3">
        <ScheduleForm onCreated={() => void load(search)} />
        <NotificationToggle />
      </div>

      <PendingActionItems />

      <div className="relative">
        <Search className="pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2 text-muted" />
        <input
          className="input pl-9"
          type="search"
          placeholder="搜尋會議標題或逐字稿…"
          value={search}
          onChange={(e) => setSearch(e.target.value)}
        />
      </div>

      {loadError && (
        <div className="flex items-center justify-between gap-3 rounded-xl border border-red-500/30 bg-red-500/5 px-4 py-3 text-sm text-red-500">
          {loadError}
          <button type="button" className="btn btn-secondary h-9" onClick={() => void load(search)}>
            重新載入
          </button>
        </div>
      )}
      {meetings.length === 0 && !loadError ? (
        <p className="py-10 text-center text-sm text-muted">
          {search ? '沒有符合的會議。' : '尚無會議紀錄，錄下第一場會議吧。'}
        </p>
      ) : (
        <ul className="divide-y divide-border overflow-hidden rounded-xl border border-border bg-surface">
          {meetings.map((m) => (
            <li key={m.id}>
              <Link
                to={`/meetings/${m.id}`}
                className="flex items-center gap-3 px-4 py-3.5 transition hover:bg-surface-hover"
              >
                <span className="min-w-0 flex-1">
                  <span className="block truncate text-sm font-medium">{m.title}</span>
                  <span className="mt-0.5 block font-mono text-xs text-muted">
                    {m.status === 'scheduled' && m.scheduledAt
                      ? `排定 ${new Date(m.scheduledAt).toLocaleString()}`
                      : `${m.durationSeconds > 0 ? `${Math.round(m.durationSeconds / 60)} 分鐘 · ` : ''}${new Date(m.createdAt).toLocaleDateString()}`}
                  </span>
                </span>
                <StatusBadge status={m.status} />
              </Link>
            </li>
          ))}
        </ul>
      )}
    </AppShell>
  )
}
