// 即時會議 session（F-LIVE）：串起音訊來源、Deepgram 即時辨識、即時重點、會議中提問、
// 錄音本機備份與結束上傳。不依賴 React（useLiveSession 以 useSyncExternalStore 訂閱），依賴皆可注入測試。
import type {
  addMeetingContextFile,
  addMeetingContextText,
  askLive,
  completeUpload,
  finishLive,
  getLiveToken,
  Language,
  LiveAudioDiagnostics,
  LiveNotes,
  LiveQA,
  LiveSummary,
  LiveUsage,
  MeetingContext,
  Scenario,
  sendLiveHeartbeat,
  startLiveMeeting,
  updateLiveNotes,
} from '../services/api/client'
import type { AudioSource, MixedRecorder, PcmCapture } from './audio'
import { NotesScheduler } from './notesScheduler'
import type { StoredSession, SessionMeta } from './recordingStore'
import type { TranscriberOptions, TranscriberState } from './transcriber'

/** Deepgram 串流取樣率（16kHz 單聲道 16-bit，約 32KB/s）。 */
export const SAMPLE_RATE = 16000
const HEARTBEAT_MS = 5 * 60_000
const NOTES_INTERVAL_MS = 90_000
const NOTES_MIN_CHARS = 200
const NOTES_MANUAL_GAP_MS = 15_000
/** 每秒量一次錄音峰值；連續 SILENCE_WARN_MS 都低於門檻視為「錄不到聲音」。 */
const SILENCE_CHECK_MS = 1000
const SILENCE_WARN_MS = 15_000
/** 約 -66 dBFS：麥克風底噪（含瀏覽器降噪後）通常仍高於此；只有真正的數位靜音（如音訊未啟動）會低於。 */
const SILENCE_PEAK = 0.0005

export type Who = 'remote' | 'local'

export interface TranscriptLine {
  id: number
  /** 收到 final 結果的時間（epoch ms）。 */
  at: number
  who: Who
  text: string
  /** 斷線缺漏提示列（不送給 AI）。 */
  gap?: boolean
}

export type SessionPhase = 'idle' | 'starting' | 'live' | 'finishing' | 'uploading' | 'done' | 'error'
export type ChannelState = TranscriberState | 'off'

export interface SessionState {
  phase: SessionPhase
  meetingId: string | null
  startedAt: number | null
  lines: TranscriptLine[]
  interim: Record<Who, string>
  notes: LiveSummary
  notesBusy: boolean
  notesError: string | null
  notesUpdatedAt: number | null
  qa: LiveQA[]
  asking: boolean
  askError: string | null
  remoteState: ChannelState
  localState: ChannelState
  usage: LiveUsage | null
  /** 額度用完或連線放棄：即時辨識已停，錄音照常。 */
  liveDisabled: boolean
  micDenied: boolean
  /** 錄音持續收不到任何聲音（分享的分頁沒聲音、麥克風無訊號或瀏覽器音訊未啟動）。 */
  noAudio: boolean
  backupFailed: boolean
  /** 使用者停止分享 / Meet 分頁關閉。 */
  sourceEnded: boolean
  contexts: MeetingContext[]
  contextBusy: boolean
  contextError: string | null
  uploadPercent: number
  error: string | null
}

export type ContextInput = File | { title?: string; text: string }

export interface StartInput {
  title: string
  scenario: Scenario
  language: Language
  micTranscription: boolean
  scheduledMeetingId?: string
  contexts: ContextInput[]
}

interface TranscriberLike {
  start(): Promise<void>
  push(frame: ArrayBuffer): void
  reconnect(): Promise<void>
  stop(): void
}

interface StoreLike {
  begin(meta: SessionMeta): Promise<void>
  saveNotes(meetingId: string, notes: LiveNotes): Promise<void>
  appendChunk(meetingId: string, blob: Blob): Promise<void>
  getRecording(meetingId: string): Promise<Blob | null>
  remove(meetingId: string): Promise<void>
  list(): Promise<StoredSession[]>
}

export interface SessionDeps {
  getIdToken: () => Promise<string>
  api: {
    startLiveMeeting: typeof startLiveMeeting
    getLiveToken: typeof getLiveToken
    sendLiveHeartbeat: typeof sendLiveHeartbeat
    updateLiveNotes: typeof updateLiveNotes
    askLive: typeof askLive
    finishLive: typeof finishLive
    completeUpload: typeof completeUpload
    addMeetingContextText: typeof addMeetingContextText
    addMeetingContextFile: typeof addMeetingContextFile
  }
  createSource: () => AudioSource & { micDenied?: boolean }
  startPcm: (stream: MediaStream, sampleRate: number, onFrame: (pcm: ArrayBuffer) => void) => Promise<PcmCapture>
  startRecorder: (streams: MediaStream[], onChunk: (blob: Blob) => void) => MixedRecorder
  createTranscriber: (opts: TranscriberOptions) => TranscriberLike
  store: StoreLike
  upload: (url: string, headers: Record<string, string>, blob: Blob, onProgress: (p: number) => void) => Promise<void>
}

function hhmm(ts: number): string {
  const d = new Date(ts)
  return `${String(d.getHours()).padStart(2, '0')}:${String(d.getMinutes()).padStart(2, '0')}`
}

/** 送給 AI 的逐字稿格式：每行「[HH:MM] 對方/我: 內容」（略過斷線提示列）。 */
export function formatTranscript(lines: TranscriptLine[]): string {
  return lines
    .filter((l) => !l.gap)
    .map((l) => `[${hhmm(l.at)}] ${l.who === 'remote' ? '對方' : '我'}: ${l.text}`)
    .join('\n')
}

function message(e: unknown, fallback: string): string {
  return e instanceof Error && e.message ? e.message : fallback
}

const initialState: SessionState = {
  phase: 'idle',
  meetingId: null,
  startedAt: null,
  lines: [],
  interim: { remote: '', local: '' },
  notes: { current: '', points: [] },
  notesBusy: false,
  notesError: null,
  notesUpdatedAt: null,
  qa: [],
  asking: false,
  askError: null,
  remoteState: 'off',
  localState: 'off',
  usage: null,
  liveDisabled: false,
  micDenied: false,
  noAudio: false,
  backupFailed: false,
  sourceEnded: false,
  contexts: [],
  contextBusy: false,
  contextError: null,
  uploadPercent: 0,
  error: null,
}

export class LiveSession {
  private state: SessionState = initialState
  private listeners = new Set<() => void>()
  private source: (AudioSource & { micDenied?: boolean }) | null = null
  private transcribers: Partial<Record<Who, TranscriberLike>> = {}
  private captures: PcmCapture[] = []
  private recorder: MixedRecorder | null = null
  private memChunks: Blob[] = []
  private mimeType = 'audio/webm'
  private scheduler: NotesScheduler | null = null
  private heartbeatTimer: ReturnType<typeof setInterval> | undefined
  private notesCursor = 0
  private lineSeq = 0
  /** 即時辨識連線中的累計毫秒（心跳回報用）。 */
  private openSince: number | null = null
  private openAccumMs = 0
  private silenceTimer: ReturnType<typeof setInterval> | undefined
  private lastSoundAt = 0
  private pcmFrames = 0
  private diagnosticsSent = false
  private readonly deps: SessionDeps

  constructor(deps: SessionDeps) {
    this.deps = deps
  }

  subscribe = (listener: () => void): (() => void) => {
    this.listeners.add(listener)
    return () => this.listeners.delete(listener)
  }

  getState = (): SessionState => this.state

  private set(patch: Partial<SessionState>): void {
    this.state = { ...this.state, ...patch }
    this.listeners.forEach((l) => l())
  }

  async start(input: StartInput): Promise<void> {
    if (this.state.phase !== 'idle') return
    this.set({ phase: 'starting', error: null })
    const source = this.deps.createSource()
    try {
      await source.start() // 先取得音訊，成功才建立紀錄
    } catch (e) {
      source.stop()
      this.set({ phase: 'idle', error: message(e, '無法取得會議音訊') })
      return
    }
    this.source = source
    source.onEnded(() => this.set({ sourceEnded: true }))

    const streams = [source.remote, source.local].filter((s): s is MediaStream => s !== null)
    let meetingId: string
    try {
      this.recorder = this.deps.startRecorder(streams, (b) => this.onChunk(b))
      this.mimeType = this.recorder.mimeType
      const idToken = await this.deps.getIdToken()
      const { meeting } = await this.deps.api.startLiveMeeting(idToken, {
        title: input.title,
        scenario: input.scenario,
        language: input.language,
        contentType: this.mimeType.split(';')[0],
        scheduledMeetingId: input.scheduledMeetingId,
      })
      meetingId = meeting.id
    } catch (e) {
      void this.recorder?.stop()
      this.recorder = null
      source.stop()
      this.source = null
      this.set({ phase: 'idle', error: message(e, '開始即時會議失敗') })
      return
    }

    const startedAt = Date.now()
    this.set({ phase: 'live', meetingId, startedAt, micDenied: source.micDenied ?? false })
    this.deps.store
      .begin({ meetingId, title: input.title, startedAt, mimeType: this.mimeType })
      .then(() => Promise.all(this.memChunks.map((b) => this.deps.store.appendChunk(meetingId, b))))
      .catch(() => this.set({ backupFailed: true }))

    if (source.remote) await this.startChannel('remote', source.remote)
    if (input.micTranscription && source.local) await this.startChannel('local', source.local)

    this.scheduler = new NotesScheduler({
      intervalMs: NOTES_INTERVAL_MS,
      minChars: NOTES_MIN_CHARS,
      manualGapMs: NOTES_MANUAL_GAP_MS,
      pendingChars: () => this.pendingText().length,
      run: () => this.runNotes(),
    })
    this.scheduler.start()
    this.heartbeatTimer = setInterval(() => void this.heartbeat(), HEARTBEAT_MS)
    this.lastSoundAt = Date.now()
    this.silenceTimer = setInterval(() => this.checkSilence(), SILENCE_CHECK_MS)

    if (input.contexts.length > 0) void this.uploadContexts(input.contexts)
  }

  private async startChannel(who: Who, stream: MediaStream): Promise<void> {
    const key = who === 'remote' ? 'remoteState' : 'localState'
    const t = this.deps.createTranscriber({
      sampleRate: SAMPLE_RATE,
      getToken: async () => {
        const tok = await this.deps.api.getLiveToken(await this.deps.getIdToken(), this.state.meetingId!)
        this.set({ usage: tok.usage })
        return tok
      },
      onFinal: (text) => this.appendLine({ who, text }),
      onInterim: (text) => this.set({ interim: { ...this.state.interim, [who]: text } }),
      onGap: (sec) => this.appendLine({ who, text: `（斷線 ${sec} 秒，正式版會補上）`, gap: true }),
      onState: (s) => {
        this.set({ [key]: s } as Partial<SessionState>)
        if (who === 'remote') this.trackOpen(s)
        if (s === 'quota') this.disableLive()
      },
    })
    this.transcribers[who] = t
    try {
      this.captures.push(
        await this.deps.startPcm(stream, SAMPLE_RATE, (f) => {
          this.pcmFrames++
          t.push(f)
        }),
      )
    } catch {
      this.set({ [key]: 'failed' } as Partial<SessionState>)
      return
    }
    await t.start()
  }

  private trackOpen(s: TranscriberState): void {
    if (s === 'open') {
      this.openSince ??= Date.now()
    } else if (this.openSince !== null) {
      this.openAccumMs += Date.now() - this.openSince
      this.openSince = null
    }
  }

  private appendLine(line: Omit<TranscriptLine, 'id' | 'at'>): void {
    this.set({ lines: [...this.state.lines, { ...line, id: ++this.lineSeq, at: Date.now() }] })
  }

  private onChunk(blob: Blob): void {
    this.memChunks.push(blob)
    const id = this.state.meetingId
    if (!id) return // 紀錄建立前的資料，begin 完成後補寫
    this.deps.store.appendChunk(id, blob).catch(() => this.set({ backupFailed: true }))
  }

  private pendingText(): string {
    return formatTranscript(this.state.lines.slice(this.notesCursor))
  }

  private async runNotes(): Promise<void> {
    const upto = this.state.lines.length
    const text = this.pendingText()
    if (!text) return
    this.set({ notesBusy: true })
    try {
      const { notes } = await this.deps.api.updateLiveNotes(
        await this.deps.getIdToken(),
        this.state.meetingId!,
        this.state.notes,
        text,
      )
      this.notesCursor = upto
      this.set({ notes, notesError: null, notesUpdatedAt: Date.now() })
      this.deps.store.saveNotes(this.state.meetingId!, this.liveNotes()).catch(() => {})
    } catch (e) {
      this.set({ notesError: message(e, '更新失敗，稍後重試') })
      throw e
    } finally {
      this.set({ notesBusy: false })
    }
  }

  /** 手動「立即更新」；回傳是否有送出。 */
  async updateNotesNow(): Promise<boolean> {
    return (await this.scheduler?.manual()) ?? false
  }

  async ask(question: string): Promise<void> {
    const q = question.trim()
    if (!q || this.state.asking || !this.state.meetingId) return
    this.set({ asking: true, askError: null })
    try {
      const res = await this.deps.api.askLive(await this.deps.getIdToken(), this.state.meetingId, {
        question: q,
        transcript: formatTranscript(this.state.lines),
        history: this.state.qa,
      })
      this.set({
        qa: [...this.state.qa, { question: q, answer: res.answer, sources: res.sources, askedAt: new Date().toISOString() }],
      })
      this.deps.store.saveNotes(this.state.meetingId, this.liveNotes()).catch(() => {})
    } catch (e) {
      this.set({ askError: message(e, '提問失敗，請重試') })
    } finally {
      this.set({ asking: false })
    }
  }

  /** 會議中新增會前資料；完成後重開辨識套用新術語。 */
  async addContext(input: ContextInput): Promise<void> {
    await this.uploadContexts([input])
  }

  private async uploadContexts(inputs: ContextInput[]): Promise<void> {
    const id = this.state.meetingId
    if (!id) return
    this.set({ contextBusy: true, contextError: null })
    let added = false
    for (const input of inputs) {
      try {
        const idToken = await this.deps.getIdToken()
        const { context } =
          input instanceof File
            ? await this.deps.api.addMeetingContextFile(idToken, id, input)
            : await this.deps.api.addMeetingContextText(idToken, id, input)
        this.set({ contexts: [...this.state.contexts, context] })
        added = true
      } catch (e) {
        const name = input instanceof File ? input.name : (input.title ?? '貼上的文字')
        this.set({ contextError: `「${name}」${message(e, '新增失敗')}` })
      }
    }
    this.set({ contextBusy: false })
    if (added && this.state.phase === 'live') {
      await Promise.all(Object.values(this.transcribers).map((t) => t?.reconnect()))
    }
  }

  /** 錄音持續靜音時提醒使用者，並把瀏覽器端音訊狀態回報一次給後端日誌（排查用）。 */
  private checkSilence(): void {
    const peak = this.recorder?.peak() ?? 0
    const now = Date.now()
    if (peak >= SILENCE_PEAK) {
      this.lastSoundAt = now
      if (this.state.noAudio) this.set({ noAudio: false })
      return
    }
    if (this.state.noAudio || now - this.lastSoundAt < SILENCE_WARN_MS) return
    this.set({ noAudio: true })
    if (!this.diagnosticsSent) {
      this.diagnosticsSent = true
      void this.heartbeat(this.audioDiagnostics(peak))
    }
  }

  private audioDiagnostics(peak: number): LiveAudioDiagnostics {
    const track = (s: MediaStream | null | undefined) => {
      const t = s?.getAudioTracks?.()[0]
      return t ? `${t.readyState}${t.muted ? '/muted' : ''}${t.enabled ? '' : '/disabled'}` : 'none'
    }
    return {
      reason: 'no_audio',
      recorderCtx: this.recorder?.contextState() ?? 'none',
      pcmCtx: this.captures.map((c) => c.contextState()).join(',') || 'none',
      pcmFrames: this.pcmFrames,
      peakDb: peak > 0 ? Math.round(20 * Math.log10(peak)) : -120,
      tabTrack: track(this.source?.remote),
      micTrack: track(this.source?.local),
      micDenied: this.state.micDenied,
    }
  }

  private async heartbeat(diagnostics?: LiveAudioDiagnostics): Promise<void> {
    const id = this.state.meetingId
    if (!id || this.state.liveDisabled) return
    const now = Date.now()
    let ms = this.openAccumMs
    if (this.openSince !== null) {
      ms += now - this.openSince
      this.openSince = now
    }
    this.openAccumMs = 0
    try {
      const idToken = await this.deps.getIdToken()
      const seconds = Math.round(ms / 1000)
      const { usage } = diagnostics
        ? await this.deps.api.sendLiveHeartbeat(idToken, id, seconds, diagnostics)
        : await this.deps.api.sendLiveHeartbeat(idToken, id, seconds)
      this.set({ usage })
      if (usage.overLimit) this.disableLive()
    } catch {
      this.openAccumMs += ms // 下次一併回報
    }
  }

  /** 停止即時辨識與重點更新；錄音照常。 */
  private disableLive(): void {
    if (this.state.liveDisabled) return
    this.set({ liveDisabled: true })
    this.stopLive()
  }

  private stopLive(): void {
    this.scheduler?.stop()
    for (const t of Object.values(this.transcribers)) t?.stop()
    this.captures.forEach((c) => c.stop())
    this.captures = []
  }

  private liveNotes(): LiveNotes {
    return { notes: this.state.notes, qa: this.state.qa }
  }

  /** 結束會議：停止辨識與錄音 → 存筆記 → 上傳錄音 → 觸發正式處理 → 清除本機備份。 */
  async finish(): Promise<void> {
    if (this.state.phase !== 'live') return
    this.set({ phase: 'finishing' })
    await this.heartbeat()
    clearInterval(this.heartbeatTimer)
    clearInterval(this.silenceTimer)
    this.stopLive()
    await this.recorder?.stop()
    this.recorder = null
    this.source?.stop()
    await this.upload()
  }

  async retryUpload(): Promise<void> {
    if (this.state.phase !== 'error') return
    await this.upload()
  }

  private async upload(): Promise<void> {
    const id = this.state.meetingId!
    this.set({ phase: 'uploading', uploadPercent: 0, error: null })
    try {
      const idToken = await this.deps.getIdToken()
      const { upload } = await this.deps.api.finishLive(idToken, id, this.liveNotes())
      const blob = new Blob(this.memChunks, { type: this.mimeType })
      await this.deps.upload(upload.url, upload.headers, blob, (p) => this.set({ uploadPercent: p }))
      await this.deps.api.completeUpload(await this.deps.getIdToken(), id)
      await this.deps.store.remove(id).catch(() => {})
      this.set({ phase: 'done' })
    } catch (e) {
      this.set({ phase: 'error', error: message(e, '上傳失敗，請重試') })
    }
  }

  /** 頁面卸載：停止一切但保留本機備份（可之後救回上傳）。 */
  dispose(): void {
    clearInterval(this.heartbeatTimer)
    clearInterval(this.silenceTimer)
    this.stopLive()
    void this.recorder?.stop()
    this.source?.stop()
    this.listeners.clear()
  }
}
