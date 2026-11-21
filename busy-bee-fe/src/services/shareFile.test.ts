import { afterEach, describe, expect, it, vi } from 'vitest'

import { audioExtension, safeFilename, shareOrDownload } from './shareFile'

afterEach(() => {
  vi.unstubAllGlobals()
  vi.restoreAllMocks()
})

function stubNavigator(nav: Partial<Navigator>) {
  vi.stubGlobal('navigator', { ...navigator, ...nav })
}

describe('safeFilename', () => {
  it('去除路徑與保留字元、壓縮空白', () => {
    expect(safeFilename('週會 / Q4:規劃?  ')).toBe('週會 Q4 規劃')
  })
  it('空字串用預設名', () => {
    expect(safeFilename('///')).toBe('busy-bee')
  })
})

describe('audioExtension', () => {
  it('優先用 Content-Type', () => {
    expect(audioExtension('audio/webm;codecs=opus', '/a/b.m4a')).toBe('.webm')
    expect(audioExtension('audio/mp4', '/a/b')).toBe('.m4a')
  })
  it('Content-Type 不明時看網址副檔名', () => {
    expect(audioExtension('application/octet-stream', '/audio/u/id.webm')).toBe('.webm')
    expect(audioExtension('', '/audio/u/id')).toBe('.m4a')
  })
})

describe('shareOrDownload', () => {
  const file = new File(['A: 你好'], '週會.md', { type: 'text/markdown' })

  it('支援檔案分享時以檔案分享（不是純文字）', async () => {
    const share = vi.fn(async () => {})
    stubNavigator({ share, canShare: () => true })
    await expect(shareOrDownload(file)).resolves.toBe('shared')
    expect(share).toHaveBeenCalledWith({ files: [file], title: '週會.md' })
  })

  it('使用者取消回傳 cancelled、不下載', async () => {
    const err = Object.assign(new Error('x'), { name: 'AbortError' })
    stubNavigator({ share: vi.fn(async () => Promise.reject(err)), canShare: () => true })
    const click = vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(() => {})
    await expect(shareOrDownload(file)).resolves.toBe('cancelled')
    expect(click).not.toHaveBeenCalled()
  })

  it('不支援檔案分享時下載', async () => {
    stubNavigator({ share: undefined, canShare: undefined })
    vi.stubGlobal('URL', Object.assign(URL, { createObjectURL: () => 'blob:x', revokeObjectURL: () => {} }))
    const click = vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(() => {})
    await expect(shareOrDownload(file)).resolves.toBe('downloaded')
    expect(click).toHaveBeenCalled()
  })
})
