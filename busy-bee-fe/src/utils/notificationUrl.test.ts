import { describe, expect, it } from 'vitest'

import { sameOriginPath } from './notificationUrl'

const origin = 'https://busy-bee.web.app'

describe('sameOriginPath', () => {
  // 稽核 F9：點通知只導向本站，外站網址一律回首頁
  it('站內相對路徑與同源絕對網址保留', () => {
    expect(sameOriginPath('/meetings/m1', origin)).toBe('/meetings/m1')
    expect(sameOriginPath('/?record=1', origin)).toBe('/?record=1')
    expect(sameOriginPath(`${origin}/meetings/m1#x`, origin)).toBe('/meetings/m1#x')
  })

  it('外站、協定相對、javascript: 與空值回首頁', () => {
    expect(sameOriginPath('https://evil.example/phish', origin)).toBe('/')
    expect(sameOriginPath('//evil.example/x', origin)).toBe('/')
    expect(sameOriginPath('javascript:alert(1)', origin)).toBe('/')
    expect(sameOriginPath(undefined, origin)).toBe('/')
  })
})
