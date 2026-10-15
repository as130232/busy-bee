import { afterEach, describe, expect, it, vi } from 'vitest'

import { floatTo16BitPCM, MeetTabSource, NoTabAudioError } from './audio'

class FakeTrack {
  kind: string
  stopped = false
  private listeners: Record<string, (() => void)[]> = {}
  constructor(kind: string) {
    this.kind = kind
  }
  stop() {
    this.stopped = true
  }
  addEventListener(ev: string, cb: () => void) {
    ;(this.listeners[ev] ??= []).push(cb)
  }
  fire(ev: string) {
    this.listeners[ev]?.forEach((cb) => cb())
  }
}

class FakeStream {
  tracks: FakeTrack[]
  constructor(tracks: FakeTrack[]) {
    this.tracks = tracks
  }
  getTracks() {
    return this.tracks
  }
  getAudioTracks() {
    return this.tracks.filter((t) => t.kind === 'audio')
  }
  getVideoTracks() {
    return this.tracks.filter((t) => t.kind === 'video')
  }
  removeTrack(t: FakeTrack) {
    this.tracks = this.tracks.filter((x) => x !== t)
  }
}

function mockMedia(display: FakeStream, mic: FakeStream | DOMException) {
  const md = {
    getDisplayMedia: vi.fn(async () => display),
    getUserMedia: vi.fn(async () => {
      if (!(mic instanceof FakeStream)) throw mic
      return mic
    }),
  }
  vi.stubGlobal('navigator', { ...navigator, mediaDevices: md })
  return md
}

afterEach(() => vi.unstubAllGlobals())

describe('floatTo16BitPCM', () => {
  it('轉 16-bit 並截斷超出範圍的值', () => {
    const out = new Int16Array(floatTo16BitPCM(new Float32Array([0, 1, -1, 2, -2, 0.5])))
    expect(Array.from(out)).toEqual([0, 32767, -32768, 32767, -32768, 16384])
  })
})

describe('MeetTabSource', () => {
  it('取得分頁音訊與麥克風，停掉影像軌', async () => {
    const video = new FakeTrack('video')
    const display = new FakeStream([video, new FakeTrack('audio')])
    const mic = new FakeStream([new FakeTrack('audio')])
    mockMedia(display, mic)
    const src = new MeetTabSource()
    await src.start()
    expect(video.stopped).toBe(true)
    expect(src.remote).toBe(display)
    expect(src.local).toBe(mic)
    expect(src.micDenied).toBe(false)
  })

  it('沒勾分享分頁音訊 → NoTabAudioError，且釋放已取得的串流', async () => {
    const video = new FakeTrack('video')
    mockMedia(new FakeStream([video]), new FakeStream([new FakeTrack('audio')]))
    await expect(new MeetTabSource().start()).rejects.toBeInstanceOf(NoTabAudioError)
    expect(video.stopped).toBe(true)
  })

  it('麥克風被拒仍可進行，local 為 null', async () => {
    mockMedia(new FakeStream([new FakeTrack('audio')]), new DOMException('no', 'NotAllowedError'))
    const src = new MeetTabSource()
    await src.start()
    expect(src.local).toBeNull()
    expect(src.micDenied).toBe(true)
  })

  it('停止分享（分頁音訊軌結束）觸發 onEnded；stop 釋放全部軌', async () => {
    const tabAudio = new FakeTrack('audio')
    const micTrack = new FakeTrack('audio')
    mockMedia(new FakeStream([tabAudio]), new FakeStream([micTrack]))
    const src = new MeetTabSource()
    const ended = vi.fn()
    src.onEnded(ended)
    await src.start()
    tabAudio.fire('ended')
    expect(ended).toHaveBeenCalledTimes(1)
    src.stop()
    expect(tabAudio.stopped && micTrack.stopped).toBe(true)
  })
})
