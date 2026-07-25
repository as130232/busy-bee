import { Navigate } from 'react-router-dom'

import { BrandMark } from '../components/AppShell'
import { GoogleIcon } from '../components/GoogleIcon'
import { Loader } from '../components/Loader'
import { useAuth } from '../hooks/useAuth'
import { scenarioLabels } from '../services/api/client'

// 三色情境 chips：對齊錄音頁情境色（會議琥珀 / 閒聊天藍 / 面試翠綠）。
const chips = [
  { label: scenarioLabels.meeting, tone: 'bg-accent/10 text-accent' },
  { label: scenarioLabels.casual, tone: 'bg-sky-500/10 text-sky-500' },
  { label: scenarioLabels.interview, tone: 'bg-emerald-500/10 text-emerald-500' },
]

export function LoginPage() {
  const { user, initializing, error, signIn } = useAuth()

  if (initializing) {
    return (
      <main className="flex min-h-dvh items-center justify-center">
        <Loader />
      </main>
    )
  }
  if (user) return <Navigate to="/" replace />

  return (
    <main className="relative isolate flex min-h-dvh flex-col items-center justify-center gap-4 overflow-hidden px-6 pt-[env(safe-area-inset-top)] pb-[env(safe-area-inset-bottom)]">
      {/* 動態背景：琥珀光暈緩慢飄移 */}
      <div aria-hidden className="pointer-events-none absolute inset-0 -z-10 overflow-hidden">
        <span className="animate-aurora absolute -top-24 left-1/2 size-[32rem] -translate-x-1/2 rounded-full bg-accent/20 blur-3xl" />
        <span className="animate-aurora absolute -bottom-32 -left-24 size-[26rem] rounded-full bg-amber-500/10 blur-3xl [animation-delay:6s]" />
      </div>

      {/* 活蜂巢 hero：六角呼吸 + 聲納環擴散 + 環繞光點 */}
      <div className="animate-scale-in relative flex size-44 items-center justify-center">
        <span className="animate-sonar absolute size-28 rounded-full border border-accent/40" />
        <span className="animate-sonar absolute size-28 rounded-full border border-accent/40 [animation-delay:0.9s]" />
        <span className="animate-sonar absolute size-28 rounded-full border border-accent/40 [animation-delay:1.8s]" />
        <span className="animate-breathe absolute size-32 rounded-full bg-accent/25 blur-2xl" />
        <span className="absolute size-44 animate-[spin_9s_linear_infinite]">
          <span className="absolute top-0 left-1/2 size-1.5 -translate-x-1/2 rounded-full bg-amber-300 shadow-[0_0_8px] shadow-amber-300" />
          <span className="absolute bottom-1 left-1/2 size-1 -translate-x-1/2 rounded-full bg-amber-400/70" />
          <span className="absolute top-1/2 right-1 size-1 -translate-y-1/2 rounded-full bg-amber-300/60" />
        </span>
        <span className="animate-breathe relative inline-flex items-center justify-center rounded-2xl bg-accent/10 p-4 shadow-[0_0_60px_-12px] shadow-accent/50">
          <BrandMark className="size-12" />
        </span>
      </div>

      {/* 標題（琥珀漸層字） */}
      <h1
        className="animate-fade-in-up m-0 bg-gradient-to-r from-amber-300 to-amber-500 bg-clip-text text-3xl font-semibold tracking-tight text-transparent"
        style={{ animationDelay: '0.08s' }}
      >
        Busy Bee
      </h1>

      {/* 標語（更新：不再提 PRD/Tech Spec） */}
      <div
        className="animate-fade-in-up flex flex-col items-center gap-1 text-center"
        style={{ animationDelay: '0.16s' }}
      >
        <p className="m-0 text-[15px] font-medium text-fg">錄音一鍵成筆記</p>
        <p className="m-0 text-sm text-muted">AI 幫你整理重點與行動項</p>
      </div>

      {/* 三色情境 chips */}
      <div
        className="animate-fade-in-up flex items-center gap-2"
        style={{ animationDelay: '0.24s' }}
      >
        {chips.map((c) => (
          <span
            key={c.label}
            className={`rounded-full px-3 py-1 text-xs font-medium ${c.tone}`}
          >
            {c.label}
          </span>
        ))}
      </div>

      {/* Google 登入鈕：扁平、細邊框 + 毛玻璃底（隨主題自適應）；官方 G icon + 輕按壓回饋 */}
      <button
        type="button"
        onClick={() => void signIn()}
        className="animate-fade-in-up mt-4 flex h-12 w-full max-w-xs items-center justify-center gap-3 rounded-xl border border-border bg-surface/70 text-[15px] font-medium text-fg backdrop-blur-sm transition hover:bg-surface active:scale-[0.98]"
        style={{ animationDelay: '0.32s' }}
      >
        <GoogleIcon className="size-5" />
        使用 Google 登入
      </button>

      {error && <p className="animate-fade-in m-0 text-center text-sm text-red-500">{error}</p>}
    </main>
  )
}
