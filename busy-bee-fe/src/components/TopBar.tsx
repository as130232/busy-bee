import { Link } from 'react-router-dom'
import { Moon, Sun } from 'lucide-react'

import { BrandMark } from './BrandMark'
import { useAuth } from '../hooks/useAuth'
import { useTheme } from '../hooks/useTheme'

/** 共用頂欄：品牌、主題切換、頭像（連往設定）。 */
export function TopBar() {
  const { user } = useAuth()
  const { dark, toggle } = useTheme()

  return (
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
          <Link to="/settings" aria-label="設定">
            <img
              className="size-8 rounded-full border border-border"
              src={user.avatarUrl}
              alt=""
              referrerPolicy="no-referrer"
            />
          </Link>
        )}
      </div>
    </header>
  )
}
