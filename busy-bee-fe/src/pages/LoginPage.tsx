import { Navigate } from 'react-router-dom'

import { useAuth } from '../hooks/useAuth'

export function LoginPage() {
  const { user, initializing, error, signIn } = useAuth()

  if (initializing) return <main className="center">載入中…</main>
  if (user) return <Navigate to="/" replace />

  return (
    <main className="center">
      <h1>🐝 Busy Bee</h1>
      <p>開會錄音 → AI 生成 PRD / Tech Spec</p>
      <button type="button" onClick={() => void signIn()}>
        使用 Google 登入
      </button>
      {error && <p className="error">{error}</p>}
    </main>
  )
}
