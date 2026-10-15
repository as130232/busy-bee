// 即時會議音訊（F-LIVE）：音訊來源抽象（Teams 等之後可新增其他來源）、PCM 擷取、混音錄音。

/** 音訊來源：remote 為遠端講者（如 Meet 分頁），local 為自己的麥克風（被拒時為 null）。 */
export interface AudioSource {
  readonly remote: MediaStream | null
  readonly local: MediaStream | null
  start(): Promise<void>
  /** 來源結束（使用者停止分享、分頁關閉）。 */
  onEnded(cb: () => void): void
  stop(): void
}

/** 分享時沒勾「分享分頁音訊」。 */
export class NoTabAudioError extends Error {
  constructor() {
    super('沒有取得分頁音訊，請重新分享並勾選「分享分頁音訊」。')
  }
}

/** 是否可用即時會議：需要桌面版 Chrome 類瀏覽器的分頁音訊分享（手機與 Safari 不支援）。 */
export function isLiveSupported(): boolean {
  if (typeof navigator === 'undefined') return false
  const md = navigator.mediaDevices as MediaDevices | undefined
  if (typeof md?.getDisplayMedia !== 'function') return false
  if (/Android|iPhone|iPad|iPod/i.test(navigator.userAgent)) return false
  return typeof AudioWorkletNode !== 'undefined' && typeof MediaRecorder !== 'undefined'
}

/** Google Meet 網頁版：Chrome 分頁音訊（遠端講者）＋麥克風（自己）。 */
export class MeetTabSource implements AudioSource {
  remote: MediaStream | null = null
  local: MediaStream | null = null
  micDenied = false
  private endedCbs: (() => void)[] = []

  async start(): Promise<void> {
    const display = await navigator.mediaDevices.getDisplayMedia({
      video: true,
      audio: true,
      // Chrome 專屬選項：預設選分頁、不列出 busy-bee 自己
      ...({ preferCurrentTab: false, selfBrowserSurface: 'exclude', surfaceSwitching: 'exclude' } as object),
    })
    // 只要音訊；影像軌立刻停掉（不錄畫面）
    display.getVideoTracks().forEach((t) => {
      t.stop()
      display.removeTrack(t)
    })
    const tabAudio = display.getAudioTracks()[0]
    if (!tabAudio) {
      display.getTracks().forEach((t) => t.stop())
      throw new NoTabAudioError()
    }
    tabAudio.addEventListener('ended', () => this.endedCbs.forEach((cb) => cb()))
    this.remote = display

    try {
      this.local = await navigator.mediaDevices.getUserMedia({
        audio: { echoCancellation: true, noiseSuppression: true },
      })
    } catch {
      this.local = null
      this.micDenied = true
    }
  }

  onEnded(cb: () => void): void {
    this.endedCbs.push(cb)
  }

  stop(): void {
    this.remote?.getTracks().forEach((t) => t.stop())
    this.local?.getTracks().forEach((t) => t.stop())
  }
}

/** Float32 [-1, 1] → 16-bit little-endian PCM（Deepgram linear16）。 */
export function floatTo16BitPCM(input: Float32Array): ArrayBuffer {
  const out = new Int16Array(input.length)
  for (let i = 0; i < input.length; i++) {
    const s = Math.max(-1, Math.min(1, input[i]))
    out[i] = s < 0 ? s * 0x8000 : Math.round(s * 0x7fff)
  }
  return out.buffer
}

// AudioWorklet：累積 2048 個樣本（16kHz 約 128ms）後送回主執行緒。
const PCM_WORKLET = `
class PcmCapture extends AudioWorkletProcessor {
  constructor() { super(); this.buf = new Float32Array(2048); this.n = 0 }
  process(inputs) {
    const ch = inputs[0] && inputs[0][0]
    if (ch) {
      for (let i = 0; i < ch.length; i++) {
        this.buf[this.n++] = ch[i]
        if (this.n === this.buf.length) { this.port.postMessage(this.buf.slice(0)); this.n = 0 }
      }
    }
    return true
  }
}
registerProcessor('pcm-capture', PcmCapture)
`

export interface PcmCapture {
  stop(): void
}

/** 以指定取樣率擷取單聲道 PCM（AudioContext 自動重新取樣），每約 128ms 回呼一次。 */
export async function startPcmCapture(
  stream: MediaStream,
  sampleRate: number,
  onFrame: (pcm: ArrayBuffer) => void,
): Promise<PcmCapture> {
  const ctx = new AudioContext({ sampleRate })
  const url = URL.createObjectURL(new Blob([PCM_WORKLET], { type: 'application/javascript' }))
  try {
    await ctx.audioWorklet.addModule(url)
  } finally {
    URL.revokeObjectURL(url)
  }
  const src = ctx.createMediaStreamSource(stream)
  const node = new AudioWorkletNode(ctx, 'pcm-capture', { channelCount: 1, channelCountMode: 'explicit' })
  node.port.onmessage = (e: MessageEvent<Float32Array>) => onFrame(floatTo16BitPCM(e.data))
  src.connect(node)
  return {
    stop() {
      node.port.onmessage = null
      src.disconnect()
      node.disconnect()
      void ctx.close()
    },
  }
}

/** 人聲錄音位元率（與一般錄音相同，見 useRecorder）。 */
const AUDIO_BITS_PER_SECOND = 32_000
/** 每 10 秒切一塊寫入本機備份。 */
const CHUNK_MS = 10_000

export interface MixedRecorder {
  mimeType: string
  /** 停止並等最後一塊資料送出。 */
  stop(): Promise<void>
}

export function pickRecorderMime(): string {
  for (const t of ['audio/webm;codecs=opus', 'audio/webm', 'audio/mp4']) {
    if (MediaRecorder.isTypeSupported(t)) return t
  }
  return 'audio/webm'
}

/** 把多路音訊（分頁＋麥克風）混成一路錄音；每 CHUNK_MS 回呼一塊。 */
export function startMixedRecorder(streams: MediaStream[], onChunk: (blob: Blob) => void): MixedRecorder {
  const ctx = new AudioContext()
  const dest = ctx.createMediaStreamDestination()
  const sources = streams.map((s) => {
    const src = ctx.createMediaStreamSource(s)
    src.connect(dest)
    return src
  })
  const mimeType = pickRecorderMime()
  const recorder = new MediaRecorder(dest.stream, { mimeType, audioBitsPerSecond: AUDIO_BITS_PER_SECOND })
  recorder.ondataavailable = (e) => {
    if (e.data.size > 0) onChunk(e.data)
  }
  recorder.start(CHUNK_MS)
  return {
    mimeType,
    stop: () =>
      new Promise<void>((resolve) => {
        const finish = () => {
          sources.forEach((s) => s.disconnect())
          void ctx.close()
          resolve()
        }
        if (recorder.state === 'inactive') {
          finish()
          return
        }
        recorder.onstop = finish
        recorder.stop()
      }),
  }
}
