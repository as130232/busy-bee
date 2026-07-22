import { NavLink } from 'react-router-dom'
import { CalendarClock, FileText, Mic, Settings } from 'lucide-react'

const tabs = [
  { to: '/', label: '錄音', Icon: Mic, end: true },
  { to: '/meetings', label: '紀錄', Icon: FileText, end: false },
  { to: '/schedule', label: '行程', Icon: CalendarClock, end: false },
  { to: '/settings', label: '設定', Icon: Settings, end: false },
]

/** 底部分頁導覽（行動優先，含 iOS safe-area）。 */
export function TabBar() {
  return (
    <nav className="shrink-0 border-t border-border bg-bg pb-[env(safe-area-inset-bottom)]">
      <div className="mx-auto flex max-w-xl">
        {tabs.map(({ to, label, Icon, end }) => (
          <NavLink
            key={to}
            to={to}
            end={end}
            className={({ isActive }) =>
              `flex flex-1 flex-col items-center gap-0.5 py-2.5 text-xs font-medium transition ${
                isActive ? 'text-accent' : 'text-muted hover:text-fg'
              }`
            }
          >
            {({ isActive }) => (
              <>
                <Icon
                  className={`size-5 transition-transform ${isActive ? 'scale-110' : ''}`}
                  strokeWidth={isActive ? 2.4 : 2}
                />
                {label}
              </>
            )}
          </NavLink>
        ))}
      </div>
    </nav>
  )
}
