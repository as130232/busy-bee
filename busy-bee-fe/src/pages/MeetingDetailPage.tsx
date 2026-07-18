import { useCallback, useEffect, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import ReactMarkdown from 'react-markdown'

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

  if (error) return <main className="center"><p className="error">{error}</p></main>
  if (!meeting) return <main className="center">載入中…</main>

  const artifactByType = new Map(artifacts.map((a) => [a.type, a]))
  const content =
    tab === 'transcript' ? meeting.transcript : (artifactByType.get(tab)?.content ?? '')

  return (
    <main className="content detail">
      <header className="detail-header">
        <Link to="/">← 返回</Link>
        <h1>{meeting.title}</h1>
        <span className={`status status-${meeting.status}`}>{meeting.status}</span>
      </header>

      {meeting.status === 'failed' && (
        <div className="failed-box">
          <p className="error">處理失敗：{meeting.errorMessage || '未知錯誤'}</p>
          <button type="button" onClick={() => void retry()}>
            重新處理
          </button>
        </div>
      )}

      <nav className="tabs">
        {(Object.keys(tabLabels) as Tab[]).map((t) => (
          <button
            key={t}
            type="button"
            className={tab === t ? 'tab active' : 'tab'}
            onClick={() => setTab(t)}
          >
            {tabLabels[t]}
          </button>
        ))}
      </nav>

      <article className="doc">
        {content ? (
          tab === 'transcript' ? (
            <p className="transcript">{content}</p>
          ) : (
            <ReactMarkdown>{content}</ReactMarkdown>
          )
        ) : (
          <p className="muted">
            {meeting.status === 'completed' ? '無內容' : '處理完成後將顯示於此。'}
          </p>
        )}
      </article>
    </main>
  )
}
