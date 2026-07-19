import { useCallback, useEffect, useState } from 'react'

function applyDark(dark: boolean) {
  document.documentElement.classList.toggle('dark', dark)
  document
    .querySelector('meta[name="theme-color"]')
    ?.setAttribute('content', dark ? '#0e0e11' : '#fafafa')
}

/** 暗/亮主題：預設跟隨系統，手動切換後存 localStorage（index.html 首屏 script 同邏輯）。 */
export function useTheme() {
  const [dark, setDark] = useState(() => document.documentElement.classList.contains('dark'))

  // 未手動設定過時跟隨系統偏好變化
  useEffect(() => {
    if (localStorage.getItem('theme')) return
    const mq = matchMedia('(prefers-color-scheme: dark)')
    const onChange = (e: MediaQueryListEvent) => {
      applyDark(e.matches)
      setDark(e.matches)
    }
    mq.addEventListener('change', onChange)
    return () => mq.removeEventListener('change', onChange)
  }, [])

  const toggle = useCallback(() => {
    setDark((prev) => {
      const next = !prev
      localStorage.setItem('theme', next ? 'dark' : 'light')
      applyDark(next)
      return next
    })
  }, [])

  return { dark, toggle }
}
