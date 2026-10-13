import { describe, expect, it } from 'vitest'

import { clearLocalUserData, QA_STORAGE_KEY } from './localData'

describe('clearLocalUserData', () => {
  // 稽核 F5：登出後不得留下上一個帳號的問答內容（共用裝置）
  it('清掉問答紀錄但保留主題偏好', () => {
    localStorage.setItem(QA_STORAGE_KEY, '{"question":"q"}')
    localStorage.setItem('theme', 'dark')
    clearLocalUserData()
    expect(localStorage.getItem(QA_STORAGE_KEY)).toBeNull()
    expect(localStorage.getItem('theme')).toBe('dark')
  })
})
