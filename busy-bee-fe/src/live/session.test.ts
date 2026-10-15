import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import type { TranscriberOptions } from './transcriber'
import { LiveSession, formatTranscript, type SessionDeps } from './session'

class FakeTranscriber {
  started = false
  stopped = false
  reconnects = 0
  pushed: ArrayBuffer[] = []
  opts: TranscriberOptions
  constructor(opts: TranscriberOptions) {
    this.opts = opts
  }
  async start() {
    this.started = true
    this.opts.onState('open')
  }
  push(f: ArrayBuffer) {
    this.pushed.push(f)
  }
  async reconnect() {
    this.reconnects++
  }
  stop() {
    this.stopped = true
    this.opts.onState('stopped')
  }
}

function setup(over: Partial<SessionDeps> = {}) {
  const transcribers: FakeTranscriber[] = []
  const pcmStreams: unknown[] = []
  const chunks: Blob[] = []
  let endedCb: (() => void) | undefined
  const source = {
    remote: { id: 'tab' } as unknown as MediaStream,
    local: { id: 'mic' } as unknown as MediaStream,
    micDenied: false,
    start: vi.fn(async () => {}),
    onEnded: (cb: () => void) => (endedCb = cb),
    stop: vi.fn(),
  }
  let onChunk: ((b: Blob) => void) | undefined
  const recorder = {
    mimeType: 'audio/webm;codecs=opus',
    stop: vi.fn(async () => onChunk?.(new Blob(['tail']))),
  }
  const api = {
    startLiveMeeting: vi.fn(async () => ({ meeting: { id: 'm1', title: '週會' } })),
    getLiveToken: vi.fn(async () => ({ accessToken: 'jwt', expiresIn: 30, model: 'nova-3', language: 'zh-TW', keyterms: [], usage: { usedMinutes: 0, limitMinutes: 3000, overLimit: false } })),
    sendLiveHeartbeat: vi.fn(async () => ({ usage: { usedMinutes: 5, limitMinutes: 3000, overLimit: false } })),
    updateLiveNotes: vi.fn(async () => ({ notes: { current: '新', points: ['重點'] } })),
    askLive: vi.fn(async () => ({ answer: '答', sources: [{ type: 'transcript' as const, label: '14:03' }] })),
    finishLive: vi.fn(async () => ({ meeting: { id: 'm1' }, upload: { url: 'https://u', headers: { 'Content-Type': 'audio/webm' } } })),
    completeUpload: vi.fn(async () => ({ meeting: { id: 'm1' } })),
    addMeetingContextText: vi.fn(async () => ({ context: { id: 'c1', title: '筆記', source: 'text', chars: 2, createdAt: '' } })),
    addMeetingContextFile: vi.fn(async () => ({ context: { id: 'c2', title: 'a.pdf', source: 'file', chars: 9, createdAt: '' } })),
  }
  const store = {
    begin: vi.fn(async () => {}),
    saveNotes: vi.fn(async () => {}),
    appendChunk: vi.fn(async (_id: string, b: Blob) => {
      chunks.push(b)
    }),
    getRecording: vi.fn(async () => null),
    remove: vi.fn(async () => {}),
    list: vi.fn(async () => []),
  }
  const upload = vi.fn(async (_u: string, _h: Record<string, string>, _b: Blob, p: (n: number) => void) => p(100))
  const deps: SessionDeps = {
    getIdToken: async () => 'id',
    api: api as unknown as SessionDeps['api'],
    createSource: () => source,
    startPcm: vi.fn(async (stream: MediaStream, _rate: number, onFrame: (f: ArrayBuffer) => void) => {
      pcmStreams.push(stream)
      onFrame(new ArrayBuffer(4))
      return { stop: vi.fn() }
    }),
    startRecorder: (_streams, cb) => {
      onChunk = cb
      cb(new Blob(['head']))
      return recorder
    },
    createTranscriber: (opts) => {
      const t = new FakeTranscriber(opts)
      transcribers.push(t)
      return t
    },
    store,
    upload,
    ...over,
  }
  const session = new LiveSession(deps)
  return { session, api, store, upload, transcribers, pcmStreams, source, recorder, chunks, ended: () => endedCb?.() }
}

const startInput = { title: '週會', scenario: 'meeting' as const, language: 'zh-TW' as const, micTranscription: false, contexts: [] }

beforeEach(() => {
  vi.useFakeTimers()
  vi.setSystemTime(new Date(2026, 9, 15, 14, 3, 0))
})
afterEach(() => vi.useRealTimers())

describe('LiveSession', () => {
  it('開始：建立紀錄、本機備份、只即時辨識分頁（省錢版）', async () => {
    const { session, api, store, transcribers, pcmStreams } = setup()
    await session.start(startInput)
    expect(api.startLiveMeeting).toHaveBeenCalledWith('id', expect.objectContaining({ title: '週會', contentType: 'audio/webm' }))
    expect(store.begin).toHaveBeenCalledWith(expect.objectContaining({ meetingId: 'm1', mimeType: 'audio/webm;codecs=opus' }))
    expect(transcribers).toHaveLength(1)
    expect(pcmStreams).toEqual([{ id: 'tab' }])
    expect(transcribers[0].pushed).toHaveLength(1)
    expect(session.getState().phase).toBe('live')
    expect(session.getState().remoteState).toBe('open')
    expect(session.getState().localState).toBe('off')
  })

  it('開麥克風辨識時兩路都辨識，逐字稿標示對方 / 我', async () => {
    const { session, transcribers } = setup()
    await session.start({ ...startInput, micTranscription: true })
    expect(transcribers).toHaveLength(2)
    transcribers[0].opts.onFinal('上線時間是週三')
    transcribers[1].opts.onFinal('了解')
    const lines = session.getState().lines
    expect(lines.map((l) => [l.who, l.text])).toEqual([
      ['remote', '上線時間是週三'],
      ['local', '了解'],
    ])
    expect(formatTranscript(lines)).toBe('[14:03] 對方: 上線時間是週三\n[14:03] 我: 了解')
  })

  it('斷線缺漏插入提示列', async () => {
    const { session, transcribers } = setup()
    await session.start(startInput)
    transcribers[0].opts.onGap(12)
    expect(session.getState().lines.at(-1)).toMatchObject({ gap: true, text: '（斷線 12 秒，正式版會補上）' })
  })

  it('更新重點只送上次之後的新內容，成功後前進游標', async () => {
    const { session, api, transcribers } = setup()
    await session.start(startInput)
    transcribers[0].opts.onFinal('第一句')
    await session.updateNotesNow()
    expect(api.updateLiveNotes).toHaveBeenLastCalledWith('id', 'm1', { current: '', points: [] }, '[14:03] 對方: 第一句')
    expect(session.getState().notes).toEqual({ current: '新', points: ['重點'] })
    transcribers[0].opts.onFinal('第二句')
    vi.advanceTimersByTime(15_000)
    await session.updateNotesNow()
    expect(api.updateLiveNotes).toHaveBeenLastCalledWith('id', 'm1', { current: '新', points: ['重點'] }, '[14:03] 對方: 第二句')
  })

  it('更新重點失敗保留舊重點並顯示錯誤', async () => {
    const { session, api, transcribers } = setup()
    api.updateLiveNotes.mockRejectedValueOnce(new Error('boom'))
    await session.start(startInput)
    transcribers[0].opts.onFinal('x')
    await session.updateNotesNow()
    expect(session.getState().notes).toEqual({ current: '', points: [] })
    expect(session.getState().notesError).toBeTruthy()
  })

  it('提問帶完整逐字稿與先前問答，結果加入紀錄', async () => {
    const { session, api, transcribers } = setup()
    await session.start(startInput)
    transcribers[0].opts.onFinal('10/22 上線')
    await session.ask('  上線日期？ ')
    expect(api.askLive).toHaveBeenCalledWith('id', 'm1', { question: '上線日期？', transcript: '[14:03] 對方: 10/22 上線', history: [] })
    expect(session.getState().qa).toMatchObject([{ question: '上線日期？', answer: '答' }])
    await session.ask('   ')
    expect(api.askLive).toHaveBeenCalledTimes(1)
  })

  it('心跳回報超過額度 → 停止即時辨識，錄音照常', async () => {
    const { session, api, transcribers } = setup()
    api.sendLiveHeartbeat.mockResolvedValueOnce({ usage: { usedMinutes: 3000, limitMinutes: 3000, overLimit: true } })
    await session.start(startInput)
    await vi.advanceTimersByTimeAsync(5 * 60_000)
    expect(api.sendLiveHeartbeat).toHaveBeenCalledWith('id', 'm1', 300)
    expect(transcribers[0].stopped).toBe(true)
    expect(session.getState().liveDisabled).toBe(true)
    expect(session.getState().phase).toBe('live')
  })

  it('會前資料在背景上傳，完成後重開辨識套用術語', async () => {
    const { session, api, transcribers } = setup()
    await session.start({ ...startInput, contexts: [{ title: '筆記', text: '內容' }, new File(['%PDF'], 'a.pdf')] })
    await vi.advanceTimersByTimeAsync(0)
    expect(api.addMeetingContextText).toHaveBeenCalledWith('id', 'm1', { title: '筆記', text: '內容' })
    expect(api.addMeetingContextFile).toHaveBeenCalledTimes(1)
    expect(session.getState().contexts.map((c) => c.id)).toEqual(['c1', 'c2'])
    expect(transcribers[0].reconnects).toBe(1)
  })

  it('結束：停辨識與錄音、存筆記、上傳完整錄音、清本機備份', async () => {
    const { session, api, store, upload, transcribers, source } = setup()
    await session.start(startInput)
    transcribers[0].opts.onFinal('x')
    await session.ask('q')
    await session.finish()
    expect(transcribers[0].stopped).toBe(true)
    expect(source.stop).toHaveBeenCalled()
    expect(api.finishLive).toHaveBeenCalledWith('id', 'm1', { notes: { current: '', points: [] }, qa: [expect.objectContaining({ question: 'q' })] })
    const blob = upload.mock.calls[0][2] as Blob
    expect(await blob.text()).toBe('headtail')
    expect(api.completeUpload).toHaveBeenCalledWith('id', 'm1')
    expect(store.remove).toHaveBeenCalledWith('m1')
    expect(session.getState().phase).toBe('done')
  })

  it('上傳失敗保留本機備份，可重試', async () => {
    const { session, store, upload, api } = setup()
    upload.mockRejectedValueOnce(new Error('上傳失敗'))
    await session.start(startInput)
    await session.finish()
    expect(session.getState().phase).toBe('error')
    expect(store.remove).not.toHaveBeenCalled()
    await session.retryUpload()
    expect(api.finishLive).toHaveBeenCalledTimes(2)
    expect(session.getState().phase).toBe('done')
    expect(store.remove).toHaveBeenCalledWith('m1')
  })

  it('本機備份失敗不中斷錄音，只標記警告', async () => {
    const { session } = setup({
      store: {
        begin: vi.fn(async () => Promise.reject(new Error('quota'))),
        saveNotes: vi.fn(async () => {}),
        appendChunk: vi.fn(async () => Promise.reject(new Error('quota'))),
        getRecording: vi.fn(async () => null),
        remove: vi.fn(async () => {}),
        list: vi.fn(async () => []),
      },
    })
    await session.start(startInput)
    await vi.advanceTimersByTimeAsync(0)
    expect(session.getState().phase).toBe('live')
    expect(session.getState().backupFailed).toBe(true)
  })

  it('來源結束（停止分享）時通知頁面', async () => {
    const { session, ended } = setup()
    await session.start(startInput)
    ended()
    expect(session.getState().sourceEnded).toBe(true)
  })

  it('開始前取音訊失敗 → 不建立紀錄', async () => {
    const { session, api, source } = setup()
    source.start.mockRejectedValueOnce(new Error('沒有取得分頁音訊'))
    await session.start(startInput)
    expect(api.startLiveMeeting).not.toHaveBeenCalled()
    expect(session.getState()).toMatchObject({ phase: 'idle', error: '沒有取得分頁音訊' })
  })
})
