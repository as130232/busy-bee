import { Outlet, useLocation } from 'react-router-dom'

import { TabBar } from './TabBar'
import { TopBar } from './TopBar'

/** 主分頁版面：頂欄 + 內容（Outlet）+ 底部分頁導覽。 */
export function TabLayout() {
  const location = useLocation()
  return (
    // 三段固定版面：頂欄 / 內容（flex-1 可捲）/ 底部分頁；內容精確填滿中間，錄音頁不需捲動
    <div className="flex h-dvh flex-col">
      <TopBar />
      {/* key 綁路徑：切分頁時重播進場動畫並重新載入該頁資料 */}
      <main key={location.pathname} className="flex-1 overflow-y-auto">
        <div className="animate-fade-in-up mx-auto flex min-h-full max-w-xl flex-col gap-6 px-4 py-6">
          <Outlet />
        </div>
      </main>
      <TabBar />
    </div>
  )
}
