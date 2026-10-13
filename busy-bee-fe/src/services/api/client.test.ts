import { afterEach, describe, expect, it, vi } from 'vitest'

import { ApiError, REQUEST_TIMEOUT_MS, setUnauthorizedHandler, syncUser } from './client'

/** 以指定 status/body 造一個假 fetch 回應。 */
function mockFetch(status: number, body: string) {
  const res = { ok: status >= 200 && status < 300, status, text: async () => body }
  vi.stubGlobal('fetch', vi.fn(async () => res as unknown as Response))
}

afterEach(() => {
  vi.useRealTimers()
  vi.unstubAllGlobals()
  setUnauthorizedHandler(null)
})

describe('request（透過 syncUser）', () => {
  it('成功時解出 envelope.data 並帶上 Bearer', async () => {
    const fetchMock = vi.fn(async () => ({
      ok: true,
      status: 200,
      text: async () => JSON.stringify({ errCode: 0, data: { id: 'u1' } }),
    }) as unknown as Response)
    vi.stubGlobal('fetch', fetchMock)

    const user = await syncUser('tok-123')
    expect(user).toEqual({ id: 'u1' })
    const headers = (fetchMock.mock.calls[0][1] as RequestInit).headers as Headers
    expect(headers.get('Authorization')).toBe('Bearer tok-123')
  })

  it('業務錯誤碼轉為友善訊息並拋 ApiError', async () => {
    mockFetch(200, JSON.stringify({ errCode: 42901, msg: 'rate limited' }))
    await expect(syncUser('t')).rejects.toMatchObject({
      name: 'ApiError',
      errCode: 42901,
      message: '操作太頻繁，請稍後再試。',
    })
  })

  it('非 JSON body 回可讀錯誤', async () => {
    mockFetch(502, '<html>bad gateway</html>')
    await expect(syncUser('t')).rejects.toMatchObject({ status: 502 })
    await expect(syncUser('t')).rejects.toThrow(/伺服器回應異常/)
  })

  it('errCode 40101 觸發 unauthorized handler', async () => {
    const onUnauth = vi.fn()
    setUnauthorizedHandler(onUnauth)
    mockFetch(200, JSON.stringify({ errCode: 40101, msg: 'expired' }))
    await expect(syncUser('t')).rejects.toBeInstanceOf(ApiError)
    expect(onUnauth).toHaveBeenCalledTimes(1)
  })

  it('HTTP 401 亦觸發 unauthorized handler', async () => {
    const onUnauth = vi.fn()
    setUnauthorizedHandler(onUnauth)
    mockFetch(401, JSON.stringify({ errCode: 40100, msg: 'unauthorized' }))
    await expect(syncUser('t')).rejects.toBeInstanceOf(ApiError)
    expect(onUnauth).toHaveBeenCalledTimes(1)
  })
})

// 稽核 F4：請求無逾時時，斷網會永遠轉圈
describe('request 逾時與網路錯誤', () => {
  it('超過 REQUEST_TIMEOUT_MS 中止並拋可讀的逾時錯誤', async () => {
    vi.useFakeTimers()
    vi.stubGlobal(
      'fetch',
      vi.fn(
        (_: string, init: RequestInit) =>
          new Promise((_resolve, reject) => {
            init.signal?.addEventListener('abort', () => reject(new DOMException('aborted', 'AbortError')))
          }),
      ),
    )
    const p = syncUser('t')
    const assertion = expect(p).rejects.toThrow('連線逾時，請檢查網路後重試')
    await vi.advanceTimersByTimeAsync(REQUEST_TIMEOUT_MS)
    await assertion
  })

  it('網路失敗（fetch reject）轉為可讀錯誤', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => Promise.reject(new TypeError('Failed to fetch'))))
    await expect(syncUser('t')).rejects.toThrow('網路連線失敗，請檢查網路後重試')
  })
})
