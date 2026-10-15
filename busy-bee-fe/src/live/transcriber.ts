// Deepgram 串流辨識 client（F-LIVE）：瀏覽器以後端簽發的短效金鑰直連 Deepgram。
// 音訊用 16-bit PCM（linear16）而非 webm：無容器標頭，斷線重連後可直接補送暫存的音訊。
import { ApiError } from '../services/api/client'

const DEEPGRAM_URL = 'wss://api.deepgram.com/v1/listen'
/** 斷線期間最多暫存的音訊秒數；超過丟最舊的，重連後回報缺漏秒數（錄音不受影響，正式版會補上）。 */
const MAX_BUFFER_SECONDS = 30
/** 超過這麼久沒送音訊就送 KeepAlive（Deepgram 約 10 秒沒資料會斷線）。 */
const KEEPALIVE_IDLE_MS = 5000
const KEEPALIVE_CHECK_MS = 2500
const BACKOFF_MS = [1000, 2000, 4000, 8000, 10000, 10000]
/** 連續這麼多次連不上（中間沒成功過）就放棄即時辨識。 */
const MAX_FAILURES = BACKOFF_MS.length

export interface TokenInfo {
  accessToken: string
  model: string
  language: string
  keyterms: string[]
}

export type TranscriberState = 'connecting' | 'open' | 'reconnecting' | 'failed' | 'quota' | 'stopped'

/** WebSocket 的最小介面（測試可替換）。 */
export interface SocketLike {
  binaryType: string
  readyState: number
  onopen: (() => void) | null
  onmessage: ((e: { data: string }) => void) | null
  onclose: (() => void) | null
  onerror: (() => void) | null
  send(data: ArrayBuffer | string): void
  close(): void
}

export interface TranscriberOptions {
  getToken: () => Promise<TokenInfo>
  sampleRate: number
  onFinal: (text: string) => void
  onInterim: (text: string) => void
  onState: (state: TranscriberState) => void
  /** 重連後回報斷線期間丟掉的音訊秒數。 */
  onGap: (seconds: number) => void
  createSocket?: (url: string, protocols: string[]) => SocketLike
}

export function buildListenURL(tok: TokenInfo, sampleRate: number): string {
  const q = new URLSearchParams({
    model: tok.model,
    language: tok.language,
    encoding: 'linear16',
    sample_rate: String(sampleRate),
    channels: '1',
    interim_results: 'true',
    smart_format: 'true',
    punctuate: 'true',
    endpointing: '300',
    utterance_end_ms: '1000',
  })
  for (const k of tok.keyterms) q.append('keyterm', k)
  return `${DEEPGRAM_URL}?${q}`
}

export class LiveTranscriber {
  private ws: SocketLike | null = null
  private queue: ArrayBuffer[] = []
  private queuedBytes = 0
  private droppedBytes = 0
  private failures = 0
  private lastSend = Date.now()
  private stopped = false
  private retryTimer: ReturnType<typeof setTimeout> | undefined
  private keepAliveTimer: ReturnType<typeof setInterval> | undefined
  private readonly maxBufferBytes: number
  private readonly createSocket: (url: string, protocols: string[]) => SocketLike
  private readonly opts: TranscriberOptions

  constructor(opts: TranscriberOptions) {
    this.opts = opts
    this.maxBufferBytes = MAX_BUFFER_SECONDS * opts.sampleRate * 2
    this.createSocket =
      opts.createSocket ?? ((url, protocols) => new WebSocket(url, protocols) as unknown as SocketLike)
  }

  async start(): Promise<void> {
    this.keepAliveTimer = setInterval(() => this.keepAlive(), KEEPALIVE_CHECK_MS)
    await this.connect('connecting')
  }

  /** 送一段 16-bit PCM；未連線時暫存。 */
  push(frame: ArrayBuffer): void {
    if (this.stopped) return
    if (this.ws?.readyState === 1) {
      this.ws.send(frame)
      this.lastSend = Date.now()
      return
    }
    this.queue.push(frame)
    this.queuedBytes += frame.byteLength
    while (this.queuedBytes > this.maxBufferBytes && this.queue.length > 0) {
      const old = this.queue.shift()!
      this.queuedBytes -= old.byteLength
      this.droppedBytes += old.byteLength
    }
  }

  /** 以新參數（如新增會前資料後的術語）重開連線；期間音訊暫存補送，不掉字。 */
  async reconnect(): Promise<void> {
    if (this.stopped) return
    this.closeSocket()
    await this.connect('connecting')
  }

  stop(): void {
    if (this.stopped) return
    this.stopped = true
    clearTimeout(this.retryTimer)
    clearInterval(this.keepAliveTimer)
    if (this.ws?.readyState === 1) this.ws.send(JSON.stringify({ type: 'CloseStream' }))
    this.closeSocket()
    this.queue = []
    this.opts.onState('stopped')
  }

  private async connect(state: TranscriberState): Promise<void> {
    if (this.stopped) return
    this.opts.onState(state)
    let tok: TokenInfo
    try {
      tok = await this.opts.getToken()
    } catch (e) {
      if (e instanceof ApiError && e.errCode === 42902) {
        this.giveUp('quota')
        return
      }
      this.scheduleRetry()
      return
    }
    if (this.stopped) return
    const ws = this.createSocket(buildListenURL(tok, this.opts.sampleRate), ['bearer', tok.accessToken])
    ws.binaryType = 'arraybuffer'
    this.ws = ws
    ws.onopen = () => {
      if (ws !== this.ws) return
      this.failures = 0
      this.opts.onState('open')
      for (const f of this.queue) ws.send(f)
      this.queue = []
      this.queuedBytes = 0
      this.lastSend = Date.now()
      if (this.droppedBytes > 0) {
        this.opts.onGap(Math.round(this.droppedBytes / (this.opts.sampleRate * 2)))
        this.droppedBytes = 0
      }
    }
    ws.onmessage = (e) => this.handleMessage(e.data)
    ws.onerror = () => {
      /* 交給 onclose 處理重連 */
    }
    ws.onclose = () => {
      if (ws !== this.ws || this.stopped) return
      this.ws = null
      this.scheduleRetry()
    }
  }

  private scheduleRetry(): void {
    if (this.stopped) return
    if (this.failures >= MAX_FAILURES) {
      this.giveUp('failed')
      return
    }
    const delay = BACKOFF_MS[Math.min(this.failures, BACKOFF_MS.length - 1)]
    this.failures++
    this.opts.onState('reconnecting')
    this.retryTimer = setTimeout(() => void this.connect('reconnecting'), delay)
  }

  private giveUp(state: 'failed' | 'quota'): void {
    this.stopped = true
    clearInterval(this.keepAliveTimer)
    this.closeSocket()
    this.queue = []
    this.opts.onState(state)
  }

  private handleMessage(data: string): void {
    let msg: { type?: string; is_final?: boolean; channel?: { alternatives?: { transcript?: string }[] } }
    try {
      msg = JSON.parse(data)
    } catch {
      return
    }
    if (msg.type !== 'Results') return
    const text = (msg.channel?.alternatives?.[0]?.transcript ?? '').trim()
    if (msg.is_final) {
      if (text) this.opts.onFinal(text)
      this.opts.onInterim('')
    } else {
      this.opts.onInterim(text)
    }
  }

  private keepAlive(): void {
    if (this.ws?.readyState !== 1) return
    if (Date.now() - this.lastSend >= KEEPALIVE_IDLE_MS) {
      this.ws.send(JSON.stringify({ type: 'KeepAlive' }))
      this.lastSend = Date.now()
    }
  }

  /** 關閉目前連線而不觸發重連。 */
  private closeSocket(): void {
    const ws = this.ws
    this.ws = null
    if (ws) {
      ws.onclose = null
      try {
        ws.close()
      } catch {
        /* 已關閉 */
      }
    }
  }
}
