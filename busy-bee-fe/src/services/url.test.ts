import { describe, expect, it } from 'vitest'

import { extractURL } from './url'

describe('extractURL', () => {
  it('從整段文字抽出第一個 http(s) 連結', () => {
    expect(extractURL('看看這部影片 https://youtu.be/abc123 很讚')).toBe('https://youtu.be/abc123')
  })

  it('純網址沒帶協定時補 https', () => {
    expect(extractURL('youtu.be/abc123')).toBe('https://youtu.be/abc123')
  })

  it('空字串或無連結回空字串', () => {
    expect(extractURL('')).toBe('')
    expect(extractURL('   ')).toBe('')
    expect(extractURL('沒有任何連結的文字')).toBe('')
  })

  it('阻擋非 http/https 協定', () => {
    expect(extractURL('file:///etc/passwd')).toBe('')
    expect(extractURL('ftp://example.com/a')).toBe('')
    expect(extractURL('javascript:alert(1)')).toBe('')
  })

  it('阻擋私有／保留網段（SSRF 第一層）', () => {
    expect(extractURL('http://localhost:8080/admin')).toBe('')
    expect(extractURL('http://127.0.0.1/')).toBe('')
    expect(extractURL('http://169.254.169.254/latest/meta-data')).toBe('')
    expect(extractURL('http://10.0.0.5/')).toBe('')
    expect(extractURL('http://192.168.1.1/')).toBe('')
    expect(extractURL('http://172.16.0.1/')).toBe('')
  })

  it('允許正常公開網段', () => {
    expect(extractURL('http://172.15.0.1/')).toBe('http://172.15.0.1/')
    expect(extractURL('https://example.com/watch?v=1')).toBe('https://example.com/watch?v=1')
  })

  it('超過長度上限回空字串', () => {
    expect(extractURL('https://example.com/' + 'a'.repeat(2100))).toBe('')
  })
})
