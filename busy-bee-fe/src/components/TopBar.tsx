import { useState } from 'react'
import { Link } from 'react-router-dom'
import { User } from 'lucide-react'

import { BrandMark } from './BrandMark'
import { useAuth } from '../hooks/useAuth'
import type { User as AppUser } from '../services/api/client'

/** 使用者頭像：有 Google 大頭貼顯示圖，否則退成名字首字，再退成使用者圖示。 */
function Avatar({ user }: { user: AppUser | null }) {
  const [failed, setFailed] = useState(false)
  if (user?.avatarUrl && !failed) {
    return (
      <img
        src={user.avatarUrl}
        alt=""
        referrerPolicy="no-referrer"
        onError={() => setFailed(true)}
        className="size-8 rounded-full object-cover"
      />
    )
  }
  const initial = user?.displayName?.trim().charAt(0).toUpperCase()
  if (initial) {
    return (
      <span className="flex size-8 items-center justify-center rounded-full bg-accent/10 text-sm font-medium text-accent">
        {initial}
      </span>
    )
  }
  return <User className="size-5 text-muted" />
}

/** 共用頂欄：左側品牌，右側使用者頭像（點擊進設定）。 */
export function TopBar() {
  const { user } = useAuth()
  return (
    <header className="shrink-0 border-b border-border bg-bg pt-[env(safe-area-inset-top)]">
      <div className="mx-auto flex h-14 max-w-xl items-center px-4">
        <Link to="/" className="flex items-center gap-2 font-semibold">
          <BrandMark />
          Busy Bee
        </Link>
        <Link
          to="/settings"
          aria-label="設定"
          className="ml-auto flex size-9 items-center justify-center rounded-full ring-1 ring-border transition hover:ring-accent/50 active:scale-95"
        >
          <Avatar user={user} />
        </Link>
      </div>
    </header>
  )
}
