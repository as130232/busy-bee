import { NavLink } from 'react-router-dom'

import { tabs } from './navTabs'

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
              `flex flex-1 flex-col items-center gap-0.5 pt-2 pb-2.5 text-xs outline-none focus-visible:text-fg font-medium transition ${
                isActive ? 'text-accent-fg' : 'text-muted hover:text-fg'
              }`
            }
          >
            {({ isActive }) => (
              <>
                {/* 作用中：icon 後方一顆膠囊底色，比單純變色更好辨識 */}
                <span
                  className={`flex h-7 w-12 items-center justify-center rounded-full transition-colors ${
                    isActive ? 'bg-accent/15' : ''
                  }`}
                >
                  <Icon className="size-5" strokeWidth={isActive ? 2.4 : 2} />
                </span>
                {label}
              </>
            )}
          </NavLink>
        ))}
      </div>
    </nav>
  )
}
