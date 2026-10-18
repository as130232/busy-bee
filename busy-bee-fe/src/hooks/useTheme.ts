import { useCallback, useEffect, useState } from 'react'

export type ThemePref = 'light' | 'dark' | 'system'

function applyDark(dark: boolean) {
  document.documentElement.classList.toggle('dark', dark)
  document
    .querySelector('meta[name="theme-color"]')
    ?.setAttribute('content', dark ? '#0e0e11' : '#fafafa')
}

function systemDark(): boolean {
  return matchMedia('(prefers-color-scheme: dark)').matches
}

function readPref(): ThemePref {
  try {
    const v = localStorage.getItem('theme')
    return v === 'light' || v === 'dark' ? v : 'system'
  } catch {
    return 'system'
  }
}

/**
 * 主題偏好：淺色／深色／跟隨系統（預設）。手動選擇存 localStorage，跟隨系統則移除該鍵
 * （index.html 首屏 script 同邏輯：無鍵即依系統）。
 */
export function useTheme() {
  const [pref, setPrefState] = useState<ThemePref>(readPref)

  // 跟隨系統時監聽系統偏好變化
  useEffect(() => {
    if (pref !== 'system') return
    const mq = matchMedia('(prefers-color-scheme: dark)')
    const onChange = (e: MediaQueryListEvent) => applyDark(e.matches)
    mq.addEventListener('change', onChange)
    return () => mq.removeEventListener('change', onChange)
  }, [pref])

  const setPref = useCallback((next: ThemePref) => {
    try {
      if (next === 'system') localStorage.removeItem('theme')
      else localStorage.setItem('theme', next)
    } catch {
      // 儲存不可用時仍套用本次選擇
    }
    applyDark(next === 'system' ? systemDark() : next === 'dark')
    setPrefState(next)
  }, [])

  return { pref, setPref }
}
