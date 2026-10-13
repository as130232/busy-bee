import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { describe, expect, it } from 'vitest'

// 稽核 F6：禁止縮放違反 WCAG 1.4.4；iOS 聚焦自動放大改以輸入框 ≥16px 處理
describe('index.html viewport', () => {
  it('不禁止使用者縮放', () => {
    const html = readFileSync(resolve(__dirname, '../../index.html'), 'utf8')
    const viewport = /<meta\s+name="viewport"\s+content="([^"]*)"/.exec(html)?.[1] ?? ''
    expect(viewport).toContain('width=device-width')
    expect(viewport).not.toMatch(/user-scalable\s*=\s*no|maximum-scale\s*=\s*1(\.0)?\b/)
  })
})
