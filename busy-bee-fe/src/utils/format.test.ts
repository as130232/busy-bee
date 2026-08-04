import { describe, expect, it } from 'vitest'

import { formatClock, formatDateTime, formatDuration } from './format'

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
