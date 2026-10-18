import type { ReactNode } from 'react'
import { LogOut, Monitor, Moon, Sun } from 'lucide-react'

import { LiveMicToggle } from '../components/LiveMicToggle'
import { NotificationToggle } from '../components/NotificationToggle'
import { useAuth } from '../hooks/useAuth'
import { useTheme, type ThemePref } from '../hooks/useTheme'

const themeOptions: { value: ThemePref; label: string; Icon: typeof Sun }[] = [
  { value: 'light', label: '淺色', Icon: Sun },
  { value: 'dark', label: '深色', Icon: Moon },
  { value: 'system', label: '跟隨系統', Icon: Monitor },
]

/** 設定分組：小標 + 卡片。 */
function Group({ title, children }: { title: string; children: ReactNode }) {
  return (
    <section className="flex flex-col gap-2">
      <h2 className="m-0 px-1 text-xs font-medium text-muted">{title}</h2>
      <div className="flex flex-col divide-y divide-border rounded-xl border border-border bg-surface [&>*]:p-4">
        {children}
      </div>
    </section>
  )
}

/** 設定頁：帳號、通知與錄音、外觀、登出。 */
export function SettingsPage() {
  const { user, signOut } = useAuth()
  const { pref, setPref } = useTheme()

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

      <Group title="通知與錄音">
        <NotificationToggle />
        <LiveMicToggle />
      </Group>

      <Group title="外觀">
        <div role="radiogroup" aria-label="外觀主題" className="grid grid-cols-3 gap-1 rounded-lg bg-bg p-1">
          {themeOptions.map(({ value, label, Icon }) => {
            const active = pref === value
            return (
              <button
                key={value}
                type="button"
                role="radio"
                aria-checked={active}
                onClick={() => setPref(value)}
                className={`flex h-10 items-center justify-center gap-1.5 rounded-md text-sm font-medium transition outline-none focus-visible:ring-2 focus-visible:ring-accent/50 ${
                  active ? 'bg-surface text-fg shadow-sm ring-1 ring-border' : 'text-muted hover:text-fg'
                }`}
              >
                <Icon className="size-4" />
                {label}
              </button>
            )
          })}
        </div>
      </Group>

      <button type="button" className="btn btn-secondary text-danger hover:text-danger" onClick={() => void signOut()}>
        <LogOut className="size-4" />
        登出
      </button>
    </>
  )
}
