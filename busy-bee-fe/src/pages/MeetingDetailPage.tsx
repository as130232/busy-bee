import { useCallback, useEffect, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import ReactMarkdown from 'react-markdown'
import { ChevronLeft } from 'lucide-react'

import { AppShell } from '../components/AppShell'
import { StatusBadge } from '../components/StatusBadge'
import { useMeetingStatusSocket } from '../hooks/useMeetingStatusSocket'
import {
  getMeeting,
  listArtifacts,
  retryMeeting,
  type Artifact,
  type MeetingDetail,
} from '../services/api/client'
import { getIdToken } from '../services/token'

type Tab = 'prd' | 'tech_spec' | 'transcript'

const tabLabels: Record<Tab, string> = {
  prd: 'PRD',
  tech_spec: 'Tech Spec',
  transcript: '逐字稿',
}

export function MeetingDetailPage() {
  const { id } = useParams<{ id: string }>()
  const [meeting, setMeeting] = useState<MeetingDetail | null>(null)
  const [artifacts, setArtifacts] = useState<Artifact[]>([])
  const [tab, setTab] = useState<Tab>('prd')
  const [error, setError] = useState<string | null>(null)

  const load = useCallback(async () => {
    if (!id) return
    try {
      const token = await getIdToken()
      const [m, a] = await Promise.all([getMeeting(token, id), listArtifacts(token, id)])
      setMeeting(m.meeting)
      setArtifacts(a.artifacts)
      setError(null)
    } catch (e) {
      setError(e instanceof Error ? e.message : '載入失敗')
    }
  }, [id])

  useEffect(() => {
    void load()
  }, [load])

  // 本會議狀態變更時重新載入（完成時文件才會出現）
  useMeetingStatusSocket((e) => {
    if (e.meetingId === id) void load()
  })

  const retry = async () => {
    if (!id) return
    try {
      await retryMeeting(await getIdToken(), id)
      void load()
    } catch (e) {
      setError(e instanceof Error ? e.message : '重試失敗')
    }
  }

  if (error) {
    return (
      <AppShell>
        <p className="py-10 text-center text-sm text-red-500">{error}</p>
      </AppShell>
    )
  }
  if (!meeting) {
    return (
      <AppShell>
        <p className="py-10 text-center text-sm text-muted">載入中…</p>
      </AppShell>
    )
  }

  const artifactByType = new Map(artifacts.map((a) => [a.type, a]))
  const content =
    tab === 'transcript' ? meeting.transcript : (artifactByType.get(tab)?.content ?? '')

  return (
    <AppShell>
      <header className="flex items-center gap-2">
        <Link to="/" className="btn btn-ghost size-11 shrink-0 px-0" aria-label="返回">
          <ChevronLeft className="size-5" />
        </Link>
        <h1 className="min-w-0 flex-1 truncate text-lg font-semibold">{meeting.title}</h1>
        <StatusBadge status={meeting.status} />
      </header>

      {meeting.status === 'failed' && (
        <div className="flex flex-wrap items-center justify-between gap-3 rounded-xl border border-red-500/30 bg-red-500/5 px-4 py-3">
          <p className="m-0 text-sm text-red-500">處理失敗：{meeting.errorMessage || '未知錯誤'}</p>
          <button type="button" className="btn btn-secondary h-9" onClick={() => void retry()}>
            重新處理
          </button>
        </div>
      )}

      <nav className="grid grid-cols-3 border-b border-border">
        {(Object.keys(tabLabels) as Tab[]).map((t) => (
          <button
            key={t}
            type="button"
            className={`-mb-px h-11 cursor-pointer border-b-2 text-sm font-medium transition ${
              tab === t
                ? 'border-accent text-fg'
                : 'border-transparent text-muted hover:text-fg'
            }`}
            onClick={() => setTab(t)}
          >
            {tabLabels[t]}
          </button>
        ))}
      </nav>

      <article className="rounded-xl border border-border bg-surface px-5 py-4">
        {content ? (
          tab === 'transcript' ? (
            <p className="text-sm leading-7 whitespace-pre-wrap">{content}</p>
          ) : (
            <div className="prose prose-sm prose-zinc dark:prose-invert max-w-none prose-headings:font-semibold prose-h1:text-xl prose-h2:mt-6 prose-h2:border-b prose-h2:border-border prose-h2:pb-1.5 prose-h2:text-base">
              <ReactMarkdown>{content}</ReactMarkdown>
            </div>
          )
        ) : (
          <p className="m-0 text-sm text-muted">
            {meeting.status === 'completed' ? '無內容' : '處理完成後將顯示於此。'}
          </p>
        )}
      </article>
    </AppShell>
  )
}
