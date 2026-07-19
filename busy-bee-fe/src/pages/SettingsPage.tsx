import { LogOut, Moon, Sun } from 'lucide-react'

import { NotificationToggle } from '../components/NotificationToggle'
import { useAuth } from '../hooks/useAuth'
import { useTheme } from '../hooks/useTheme'

/** 設定分頁：帳號、通知、主題、登出。 */
export function SettingsPage() {
  const { user, signOut } = useAuth()
  const { dark, toggle } = useTheme()

  return (
    <>
      {user && (
        <section className="flex items-center gap-3 rounded-xl border border-border bg-surface p-4">
          {user.avatarUrl && (
            <img
              className="size-12 rounded-full border border-border"
              src={user.avatarUrl}
              alt=""
              referrerPolicy="no-referrer"
            />
          )}
          <div className="min-w-0">
            <p className="m-0 truncate text-sm font-medium">{user.displayName || '使用者'}</p>
            <p className="m-0 truncate text-xs text-muted">{user.email}</p>
          </div>
        </section>
      )}

      <section className="rounded-xl border border-border bg-surface p-4">
        <NotificationToggle />
      </section>

      <button
        type="button"
        onClick={toggle}
        className="flex items-center justify-between rounded-xl border border-border bg-surface p-4 text-left transition hover:bg-surface-hover"
      >
        <span className="flex items-center gap-2 text-sm">
          {dark ? <Moon className="size-4" /> : <Sun className="size-4" />}
          外觀主題
        </span>
        <span className="text-sm text-muted">{dark ? '深色' : '淺色'}</span>
      </button>

      <button
        type="button"
        className="btn btn-secondary text-red-500 hover:text-red-500"
        onClick={() => void signOut()}
      >
        <LogOut className="size-4" />
        登出
      </button>
    </>
  )
}
