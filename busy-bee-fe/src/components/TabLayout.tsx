import { Outlet, useLocation } from 'react-router-dom'

import { TabBar } from './TabBar'
import { TopBar } from './TopBar'

/** 主分頁版面：頂欄 + 內容（Outlet）+ 底部分頁導覽。 */
export function TabLayout() {
  const location = useLocation()
  return (
    <div className="min-h-dvh">
      <TopBar />
      {/* key 綁路徑：切分頁時重播進場動畫並重新載入該頁資料 */}
      <main
        key={location.pathname}
        className="animate-fade-in-up mx-auto flex max-w-xl flex-col gap-6 px-4 pt-6 pb-[calc(5.5rem+env(safe-area-inset-bottom))]"
      >
        <Outlet />
      </main>
      <TabBar />
    </div>
  )
}
