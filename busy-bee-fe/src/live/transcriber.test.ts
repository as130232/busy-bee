import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { ApiError } from '../services/api/client'
import { LiveTranscriber, type SocketLike, type TokenInfo } from './transcriber'

class FakeSocket implements SocketLike {
  static all: FakeSocket[] = []
  binaryType = ''
  readyState = 0
  sent: (ArrayBuffer | string)[] = []
  onopen: (() => void) | null = null
  onmessage: ((e: { data: string }) => void) | null = null
  onclose: (() => void) | null = null
  onerror: (() => void) | null = null
  url: string
  protocols: string[]
  constructor(url: string, protocols: string[]) {
    this.url = url
    this.protocols = protocols
    FakeSocket.all.push(this)
  }
  send(d: ArrayBuffer | string) {
    this.sent.push(d)
  }
  close() {
    this.readyState = 3
    this.onclose?.()
  }
  open() {
    this.readyState = 1
    this.onopen?.()
  }
  drop() {
    this.readyState = 3
    this.onclose?.()
  }
  result(transcript: string, isFinal: boolean) {
    this.onmessage?.({
      data: JSON.stringify({ type: 'Results', is_final: isFinal, channel: { alternatives: [{ transcript }] } }),
    })
  }
}

const token: TokenInfo = { accessToken: 'jwt', model: 'nova-3', language: 'zh-TW', keyterms: ['Stage', 'Q&A'] }
const frame = (bytes: number) => new ArrayBuffer(bytes)
const flush = () => vi.advanceTimersByTimeAsync(0)

function make(over: Partial<ConstructorParameters<typeof LiveTranscriber>[0]> = {}) {
  const events = {
    finals: [] as string[],
    interims: [] as string[],
    states: [] as string[],
    gaps: [] as number[],
  }
  const getToken = vi.fn(async () => token)
  const t = new LiveTranscriber({
    getToken,
    sampleRate: 16000,
    createSocket: (url, protocols) => new FakeSocket(url, protocols),
    onFinal: (s) => events.finals.push(s),
    onInterim: (s) => events.interims.push(s),
    onState: (s) => events.states.push(s),
    onGap: (s) => events.gaps.push(s),
    ...over,
  })
  return { t, events, getToken }
}

beforeEach(() => {
  vi.useFakeTimers()
  FakeSocket.all = []
})
afterEach(() => vi.useRealTimers())

describe('LiveTranscriber', () => {
  it('以 bearer subprotocol 與 PCM 參數連線 Deepgram', async () => {
    const { t } = make()
    await t.start()
    const s = FakeSocket.all[0]
    expect(s.protocols).toEqual(['bearer', 'jwt'])
    const url = new URL(s.url)
    expect(url.origin + url.pathname).toBe('wss://api.deepgram.com/v1/listen')
    expect(url.searchParams.get('encoding')).toBe('linear16')
    expect(url.searchParams.get('sample_rate')).toBe('16000')
    expect(url.searchParams.get('model')).toBe('nova-3')
    expect(url.searchParams.get('language')).toBe('zh-TW')
    expect(url.searchParams.get('interim_results')).toBe('true')
    expect(url.searchParams.getAll('keyterm')).toEqual(['Stage', 'Q&A'])
  })

  it('final 結果寫入、interim 只更新暫存列', async () => {
    const { t, events } = make()
    await t.start()
    const s = FakeSocket.all[0]
    s.open()
    s.result('大家', false)
    s.result('大家好', true)
    s.result('   ', true)
    expect(events.interims).toEqual(['大家', '', '']) // 每個 final（含空白）都清掉暫存列
    expect(events.finals).toEqual(['大家好'])
  })

  it('連線前的音訊先暫存，連上後依序補送', async () => {
    const { t } = make()
    await t.start()
    t.push(frame(10))
    t.push(frame(20))
    const s = FakeSocket.all[0]
    s.open()
    expect(s.sent.map((d) => (d as ArrayBuffer).byteLength)).toEqual([10, 20])
    t.push(frame(30))
    expect(s.sent).toHaveLength(3)
  })

  it('一段時間沒送音訊時送 KeepAlive', async () => {
    const { t } = make()
    await t.start()
    const s = FakeSocket.all[0]
    s.open()
    await vi.advanceTimersByTimeAsync(6000)
    expect(s.sent).toContain(JSON.stringify({ type: 'KeepAlive' }))
  })

  it('斷線後取新金鑰重連，期間音訊補送', async () => {
    const { t, events, getToken } = make()
    await t.start()
    FakeSocket.all[0].open()
    FakeSocket.all[0].drop()
    expect(events.states.at(-1)).toBe('reconnecting')
    t.push(frame(8))
    await vi.advanceTimersByTimeAsync(1000)
    expect(getToken).toHaveBeenCalledTimes(2)
    const s2 = FakeSocket.all[1]
    s2.open()
    expect(events.states.at(-1)).toBe('open')
    expect(s2.sent.map((d) => (d as ArrayBuffer).byteLength)).toEqual([8])
    expect(events.gaps).toEqual([])
  })

  it('暫存超過上限時丟最舊的，重連後回報斷線秒數', async () => {
    // 16kHz × 2 bytes = 32000 bytes/秒；上限 30 秒
    const { t, events } = make()
    await t.start()
    FakeSocket.all[0].open()
    FakeSocket.all[0].drop()
    for (let i = 0; i < 32; i++) t.push(frame(32000))
    await vi.advanceTimersByTimeAsync(1000)
    FakeSocket.all[1].open()
    expect(events.gaps).toEqual([2])
    expect(FakeSocket.all[1].sent).toHaveLength(30)
  })

  it('連續失敗達上限 → failed；額度用完 → 立即 failed(quota)', async () => {
    const { t, events } = make({ getToken: vi.fn(async () => Promise.reject(new Error('net'))) })
    await t.start()
    await vi.advanceTimersByTimeAsync(60_000)
    expect(events.states.at(-1)).toBe('failed')

    const quota = make({ getToken: vi.fn(async () => Promise.reject(new ApiError(42902, 'quota', 429))) })
    await quota.t.start()
    await flush()
    expect(quota.events.states.at(-1)).toBe('quota')
  })

  it('stop 送 CloseStream、之後不再重連', async () => {
    const { t, events, getToken } = make()
    await t.start()
    const s = FakeSocket.all[0]
    s.open()
    t.stop()
    expect(s.sent).toContain(JSON.stringify({ type: 'CloseStream' }))
    await vi.advanceTimersByTimeAsync(20_000)
    expect(getToken).toHaveBeenCalledTimes(1)
    expect(events.states.at(-1)).toBe('stopped')
  })

  it('reconnect() 以新參數重開連線且不掉音訊', async () => {
    const { t, getToken } = make()
    await t.start()
    FakeSocket.all[0].open()
    getToken.mockResolvedValueOnce({ ...token, keyterms: ['Jenkins'] })
    const p = t.reconnect()
    t.push(frame(5))
    await p
    const s2 = FakeSocket.all[1]
    expect(new URL(s2.url).searchParams.getAll('keyterm')).toEqual(['Jenkins'])
    s2.open()
    expect(s2.sent.map((d) => (d as ArrayBuffer).byteLength)).toEqual([5])
    expect(FakeSocket.all[0].readyState).toBe(3)
  })
})
