import { act, renderHook } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { MIN_AUDIO_BYTES, NO_AUDIO_STALL_MS, useRecorder } from './useRecorder'

// 模擬 MediaRecorder：測試主動送出 dataavailable，驗證「有沒有真的收到聲音」的判斷。
class FakeRecorder {
  static isTypeSupported = () => true
  static last: FakeRecorder | null = null
  state: RecordingState = 'inactive'
  mimeType: string
  stream: MediaStream
  ondataavailable: ((e: { data: Blob }) => void) | null = null
  onstop: (() => void) | null = null
  onerror: (() => void) | null = null
  constructor(stream: MediaStream, opts: { mimeType: string }) {
    this.stream = stream
    this.mimeType = opts.mimeType
    FakeRecorder.last = this
  }
  start() {
    this.state = 'recording'
  }
  stop() {
    this.state = 'inactive'
    this.onstop?.()
  }
  pause() {
    this.state = 'paused'
  }
  resume() {
    this.state = 'recording'
  }
  requestData() {}
  emit(bytes: number) {
    this.ondataavailable?.({ data: new Blob([new Uint8Array(bytes)]) })
  }
}

let track: { stop: () => void; onmute: (() => void) | null }

beforeEach(() => {
  vi.useFakeTimers()
  vi.stubGlobal('MediaRecorder', FakeRecorder)
  track = { stop: vi.fn(), onmute: null }
  const stream = { getTracks: () => [track], getAudioTracks: () => [track] }
  Object.defineProperty(navigator, 'mediaDevices', {
    value: { getUserMedia: vi.fn(async () => stream) },
    configurable: true,
  })
})

afterEach(() => {
  vi.useRealTimers()
  vi.unstubAllGlobals()
})

async function startRecording() {
  const hook = renderHook(() => useRecorder())
  await act(async () => {
    await hook.result.current.start()
  })
  return hook
}

const recorder = () => FakeRecorder.last!

describe('useRecorder', () => {
  it('錄音中一段時間都沒收到聲音資料 → 顯示警告', async () => {
    const { result } = await startRecording()
    expect(result.current.warning).toBeNull()

    act(() => vi.advanceTimersByTime(NO_AUDIO_STALL_MS + 1000))

    expect(result.current.warning).toMatch(/沒有收到麥克風的聲音/)
  })

  it('持續收到聲音資料 → 不警告', async () => {
    const { result } = await startRecording()

    for (let i = 0; i < 15; i++) {
      act(() => {
        recorder().emit(4000)
        vi.advanceTimersByTime(1000)
      })
    }

    expect(result.current.warning).toBeNull()
  })

  it('麥克風被系統靜音（通話占用）→ 立即警告', async () => {
    const { result } = await startRecording()

    act(() => track.onmute?.())

    expect(result.current.warning).toMatch(/沒有收到麥克風的聲音/)
  })

  it('恢復收到聲音資料 → 警告消失', async () => {
    const { result } = await startRecording()
    act(() => vi.advanceTimersByTime(NO_AUDIO_STALL_MS + 1000))
    expect(result.current.warning).not.toBeNull()

    act(() => recorder().emit(4000))

    expect(result.current.warning).toBeNull()
  })

  it('暫停期間沒有資料不算異常', async () => {
    const { result } = await startRecording()
    act(() => result.current.pause())
    act(() => vi.advanceTimersByTime(NO_AUDIO_STALL_MS * 3))
    act(() => result.current.resume())
    act(() => vi.advanceTimersByTime(1000))

    expect(result.current.warning).toBeNull()
  })

  it('結束時音檔太小 → 不回傳檔案並提示沒錄到聲音', async () => {
    const { result } = await startRecording()
    act(() => recorder().emit(5))

    let file: File | null = new File([], 'x')
    await act(async () => {
      file = await result.current.stop()
    })

    expect(file).toBeNull()
    expect(result.current.error).toMatch(/沒有錄到聲音/)
  })

  it('正常錄音 → 回傳檔案', async () => {
    const { result } = await startRecording()
    act(() => recorder().emit(MIN_AUDIO_BYTES * 4))

    let file: File | null = null
    await act(async () => {
      file = await result.current.stop()
    })

    expect(file).not.toBeNull()
    expect(result.current.error).toBeNull()
  })
})
