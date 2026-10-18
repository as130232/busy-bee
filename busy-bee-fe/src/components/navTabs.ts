import { CalendarClock, FileText, Mic, Sparkles } from 'lucide-react'

// 底部分頁定義（TabBar 與 TopBar 頁名共用）。
// 設定已移至右上頭像入口，底部分頁不再列「設定」（路由仍保留）。
export const tabs = [
  { to: '/', label: '錄音', Icon: Mic, end: true },
  { to: '/meetings', label: '紀錄', Icon: FileText, end: false },
  { to: '/ask', label: '問答', Icon: Sparkles, end: false },
  { to: '/schedule', label: '行程', Icon: CalendarClock, end: false },
]
