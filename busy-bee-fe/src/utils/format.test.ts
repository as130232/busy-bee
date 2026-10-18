import { describe, expect, it } from 'vitest'

import {
  dateGroupLabel,
  formatClock,
  formatDateTime,
  formatDuration,
  formatFileStamp,
  formatShortDateTime,
} from './format'

describe('formatClock', () => {
  it('格式化為 m:ss，秒補零', () => {
    expect(formatClock(0)).toBe('0:00')
    expect(formatClock(5)).toBe('0:05')
    expect(formatClock(65)).toBe('1:05')
    expect(formatClock(3599)).toBe('59:59')
  })

  it('向下取整、負數與非有限值一律歸零', () => {
    expect(formatClock(9.9)).toBe('0:09')
    expect(formatClock(-10)).toBe('0:00')
    expect(formatClock(Number.NaN)).toBe('0:00')
    expect(formatClock(Number.POSITIVE_INFINITY)).toBe('0:00')
  })
})

describe('formatDuration', () => {
  it('0 或負數回空字串', () => {
    expect(formatDuration(0)).toBe('')
    expect(formatDuration(-5)).toBe('')
  })

  it('不足一分只顯示秒', () => {
    expect(formatDuration(45)).toBe('45 秒')
  })

  it('整分不顯示秒；有分有秒兩者都顯示', () => {
    expect(formatDuration(60)).toBe('1 分')
    expect(formatDuration(150)).toBe('2 分 30 秒')
  })
})

describe('formatDateTime', () => {
  it('接受 ISO 字串並輸出含年份的 zh-TW 字串', () => {
    const out = formatDateTime('2024-03-05T08:09:00Z')
    expect(out).toMatch(/2024/)
    // 24 小時制、補零：時間部分應為 HH:mm 樣式
    expect(out).toMatch(/\d{2}:\d{2}/)
  })

  it('接受時間戳與 Date 物件', () => {
    const ts = Date.parse('2024-03-05T08:09:00Z')
    expect(formatDateTime(ts)).toBe(formatDateTime(new Date(ts)))
  })
})

describe('formatFileStamp', () => {
  // 稽核 F1：錄音檔名曾用 toISOString（UTC），台灣時間會少 8 小時
  it('以本地時間輸出 YYYY-MM-DD HH:mm', () => {
    expect(formatFileStamp(new Date(2026, 9, 13, 6, 5))).toBe('2026-10-13 06:05')
    expect(formatFileStamp(new Date(2026, 0, 1, 23, 59))).toBe('2026-01-01 23:59')
  })
})

describe('dateGroupLabel', () => {
  const now = new Date(2026, 9, 18, 10, 0)
  it('依日曆日分組：今天／昨天／近 7 天／年月', () => {
    expect(dateGroupLabel(new Date(2026, 9, 18, 0, 5), now)).toBe('今天')
    expect(dateGroupLabel(new Date(2026, 9, 17, 23, 59), now)).toBe('昨天')
    expect(dateGroupLabel(new Date(2026, 9, 12, 9, 0), now)).toBe('近 7 天')
    expect(dateGroupLabel(new Date(2026, 9, 11, 9, 0), now)).toBe('2026 年 10 月')
    expect(dateGroupLabel(new Date(2025, 11, 31, 9, 0), now)).toBe('2025 年 12 月')
  })
})

describe('formatShortDateTime', () => {
  const now = new Date(2026, 9, 18, 10, 0)
  it('今天/昨天只顯示時間，其餘含月日', () => {
    expect(formatShortDateTime(new Date(2026, 9, 18, 8, 5), now)).toBe('08:05')
    expect(formatShortDateTime(new Date(2026, 9, 17, 21, 30), now)).toBe('21:30')
    expect(formatShortDateTime(new Date(2026, 9, 3, 9, 0), now)).toBe('10/3 09:00')
  })
})
