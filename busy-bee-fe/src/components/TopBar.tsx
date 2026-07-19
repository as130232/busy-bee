import { Link } from 'react-router-dom'

import { BrandMark } from './BrandMark'

/** 共用頂欄：僅品牌（帳號、主題切換已移至「設定」分頁）。 */
export function TopBar() {
  return (
    <header className="shrink-0 border-b border-border bg-bg pt-[env(safe-area-inset-top)]">
      <div className="mx-auto flex h-14 max-w-xl items-center px-4">
        <Link to="/" className="flex items-center gap-2 font-semibold">
          <BrandMark />
          Busy Bee
        </Link>
      </div>
    </header>
  )
}
