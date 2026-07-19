import type { ReactNode } from 'react'
import { Link } from 'react-router-dom'
import { Hexagon, LogOut, Moon, Sun } from 'lucide-react'

import { useAuth } from '../hooks/useAuth'
import { useTheme } from '../hooks/useTheme'

export function BrandMark({ className = 'size-5' }: { className?: string }) {
  return (
    <span className="relative inline-flex items-center justify-center">
      <Hexagon className={`${className} rotate-90 text-accent`} strokeWidth={1.75} />
      <span className="absolute size-[27%] rounded-full bg-accent" />
    </span>
  )
}

/** 共用頁面骨架：sticky 頂欄（品牌 / 主題切換 / 使用者）+ 單欄內容容器。 */
export function AppShell({ children }: { children: ReactNode }) {
  const { user, signOut } = useAuth()
  const { dark, toggle } = useTheme()

  return (
    <div className="min-h-dvh">
      <header className="sticky top-0 z-10 border-b border-border bg-bg/85 pt-[env(safe-area-inset-top)] backdrop-blur">
        <div className="mx-auto flex h-14 max-w-xl items-center gap-2 px-4">
          <Link to="/" className="flex items-center gap-2 font-semibold">
            <BrandMark />
            Busy Bee
          </Link>
          <span className="flex-1" />
          <button
            type="button"
            className="btn btn-ghost size-11 px-0"
            aria-label="切換主題"
            onClick={toggle}
          >
            {dark ? <Sun className="size-5" /> : <Moon className="size-5" />}
          </button>
          {user?.avatarUrl && (
            <img
              className="size-8 rounded-full border border-border"
              src={user.avatarUrl}
              alt=""
              referrerPolicy="no-referrer"
            />
          )}
          <button
            type="button"
            className="btn btn-ghost size-11 px-0"
            aria-label="登出"
            onClick={() => void signOut()}
          >
            <LogOut className="size-5" />
          </button>
        </div>
      </header>
      <main className="mx-auto flex max-w-xl flex-col gap-6 px-4 pt-6 pb-[calc(3rem+env(safe-area-inset-bottom))]">
        {children}
      </main>
    </div>
  )
}
