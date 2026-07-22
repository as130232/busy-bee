import type { ReactNode } from 'react'

import { TopBar } from './TopBar'

export { BrandMark } from './BrandMark'

/**
 * 子頁版面（無底部分頁）：頂欄 + 單欄內容容器。詳情頁等使用。
 * hideTopBar：詳情頁隱藏全域品牌列（改由頁內返回鍵導覽），並自行補上安全區 top padding。
 */
export function AppShell({ children, hideTopBar = false }: { children: ReactNode; hideTopBar?: boolean }) {
  return (
    <div className="min-h-dvh">
      {!hideTopBar && <TopBar />}
      <main
        className={`animate-fade-in-up mx-auto flex max-w-xl flex-col gap-6 px-4 pb-[calc(3rem+env(safe-area-inset-bottom))] ${
          hideTopBar ? 'pt-[calc(0.75rem+env(safe-area-inset-top))]' : 'pt-6'
        }`}
      >
        {children}
      </main>
    </div>
  )
}
