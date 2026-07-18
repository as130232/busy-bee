import { useCallback, useEffect, useState } from 'react'
import { Link } from 'react-router-dom'

import { RecorderPanel } from '../components/RecorderPanel'
import { UploadZone } from '../components/UploadZone'
import { useAuth } from '../hooks/useAuth'
import { useMeetingStatusSocket } from '../hooks/useMeetingStatusSocket'
import { listMeetings, type Meeting } from '../services/api/client'
import { getIdToken } from '../services/token'

export function DashboardPage() {
  const { user, signOut } = useAuth()
  const [meetings, setMeetings] = useState<Meeting[]>([])
  const [search, setSearch] = useState('')

  const load = useCallback(async (keyword: string) => {
    try {
      const { meetings } = await listMeetings(await getIdToken(), keyword)
      setMeetings(meetings)
    } catch {
      // 列表載入失敗不阻斷頁面；上傳與 WS 更新仍可用
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
    <main>
      <header className="topbar">
        <span>🐝 Busy Bee</span>
        <span className="spacer" />
        {user.avatarUrl && <img className="avatar" src={user.avatarUrl} alt="" referrerPolicy="no-referrer" />}
        <span>{user.displayName || user.email}</span>
        <button type="button" onClick={() => void signOut()}>
          登出
        </button>
      </header>
      <section className="content">
        <RecorderPanel onUploaded={() => void load(search)} />
        <UploadZone onUploaded={() => void load(search)} />

        <input
          className="search"
          type="search"
          placeholder="搜尋會議標題或逐字稿…"
          value={search}
          onChange={(e) => setSearch(e.target.value)}
        />

        {meetings.length === 0 ? (
          <p className="muted center">{search ? '沒有符合的會議。' : '尚無會議紀錄，上傳第一個錄音吧。'}</p>
        ) : (
          <ul className="meeting-list">
            {meetings.map((m) => (
              <li key={m.id}>
                <Link to={`/meetings/${m.id}`} className="meeting-link">
                  <span>{m.title}</span>
                  <span className="muted small">
                    {m.durationSeconds > 0 && `${Math.round(m.durationSeconds / 60)} 分鐘 · `}
                    {new Date(m.createdAt).toLocaleDateString()}
                  </span>
                </Link>
                <span className={`status status-${m.status}`}>{m.status}</span>
              </li>
            ))}
          </ul>
        )}
      </section>
    </main>
  )
}
