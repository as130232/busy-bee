import { useState } from 'react'

import { UploadZone } from '../components/UploadZone'
import { useAuth } from '../hooks/useAuth'
import type { Meeting } from '../services/api/client'

export function DashboardPage() {
  const { user, signOut } = useAuth()
  const [uploaded, setUploaded] = useState<Meeting[]>([])
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
        <UploadZone onUploaded={(m) => setUploaded((prev) => [m, ...prev])} />
        {uploaded.length > 0 && (
          <ul className="meeting-list">
            {uploaded.map((m) => (
              <li key={m.id}>
                <span>{m.title}</span>
                <span className={`status status-${m.status}`}>{m.status}</span>
              </li>
            ))}
          </ul>
        )}
        <p className="muted">錄音功能與即時處理狀態即將推出（M1-B）。</p>
      </section>
    </main>
  )
}
