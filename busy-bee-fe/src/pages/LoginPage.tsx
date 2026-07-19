import { Navigate } from 'react-router-dom'

import { BrandMark } from '../components/AppShell'
import { useAuth } from '../hooks/useAuth'

export function LoginPage() {
  const { user, initializing, error, signIn } = useAuth()

  if (initializing) {
    return <main className="flex min-h-dvh items-center justify-center text-muted">載入中…</main>
  }
  if (user) return <Navigate to="/" replace />

  return (
    <main className="flex min-h-dvh flex-col items-center justify-center gap-3 px-6 pb-[env(safe-area-inset-bottom)]">
      <div className="animate-scale-in mb-2 rounded-2xl bg-accent/10 p-4 shadow-[0_0_80px_-16px] shadow-accent/40">
        <BrandMark className="size-10" />
      </div>
      <h1
        className="animate-fade-in-up text-2xl font-semibold tracking-tight"
        style={{ animationDelay: '0.1s' }}
      >
        Busy Bee
      </h1>
      <p className="animate-fade-in-up text-sm text-muted" style={{ animationDelay: '0.18s' }}>
        開會錄音 → AI 生成 PRD / Tech Spec
      </p>
      <button
        type="button"
        className="btn btn-primary animate-fade-in-up mt-6 w-full max-w-xs"
        style={{ animationDelay: '0.26s' }}
        onClick={() => void signIn()}
      >
        使用 Google 登入
      </button>
      {error && <p className="text-sm text-red-500">{error}</p>}
    </main>
  )
}
