import { act, renderHook } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

vi.mock('../services/firebase', () => ({
  auth: { currentUser: { getIdToken: vi.fn(async () => 'tok') } },
}))

import { HIDDEN_GRACE_MS, IDLE_TIMEOUT_MS, useMeetingStatusSocket } from './useMeetingStatusSocket'

class FakeWebSocket {
  static OPEN = 1
  static instances: FakeWebSocket[] = []
  readyState = 0
  onopen: (() => void) | null = null
  onmessage: ((e: { data: string }) => void) | null = null
  onclose: (() => void) | null = null
  closed = false
  constructor(public url: string) {
    FakeWebSocket.instances.push(this)
  }
  send = vi.fn()
  close() {
    if (this.closed) return
    this.closed = true
    this.onclose?.()
  }
}

let visibility: DocumentVisibilityState = 'visible'
const setVisibility = (v: DocumentVisibilityState) => {
  visibility = v
  document.dispatchEvent(new Event('visibilitychange'))
}
const openSockets = () => FakeWebSocket.instances.filter((s) => !s.closed)

beforeEach(() => {
  vi.useFakeTimers()
  FakeWebSocket.instances = []
  visibility = 'visible'
  vi.stubGlobal('WebSocket', FakeWebSocket)
  vi.spyOn(document, 'visibilityState', 'get').mockImplementation(() => visibility)
})

afterEach(() => {
  vi.useRealTimers()
  vi.unstubAllGlobals()
  vi.restoreAllMocks()
})

describe('useMeetingStatusSocket', () => {
  it('掛載時建立連線，非預期斷線會退避重連', () => {
    renderHook(() => useMeetingStatusSocket(() => {}))
    expect(FakeWebSocket.instances).toHaveLength(1)

    act(() => FakeWebSocket.instances[0].close())
    act(() => vi.advanceTimersByTime(1000))
    expect(openSockets()).toHaveLength(1)
    expect(FakeWebSocket.instances).toHaveLength(2)
  })

  it('背景超過寬限期才斷線，且不再重連', () => {
    renderHook(() => useMeetingStatusSocket(() => {}))
    act(() => setVisibility('hidden'))

    act(() => vi.advanceTimersByTime(HIDDEN_GRACE_MS - 1))
    expect(openSockets()).toHaveLength(1)

    act(() => vi.advanceTimersByTime(1))
    expect(openSockets()).toHaveLength(0)

    act(() => vi.advanceTimersByTime(60_000))
    expect(FakeWebSocket.instances).toHaveLength(1)
  })

  it('寬限期內回到前景不斷線、不重載', () => {
    const onResync = vi.fn()
    renderHook(() => useMeetingStatusSocket(() => {}, onResync))
    act(() => setVisibility('hidden'))
    act(() => vi.advanceTimersByTime(HIDDEN_GRACE_MS / 2))
    act(() => setVisibility('visible'))
    act(() => vi.advanceTimersByTime(HIDDEN_GRACE_MS))

    expect(openSockets()).toHaveLength(1)
    expect(FakeWebSocket.instances).toHaveLength(1)
    expect(onResync).not.toHaveBeenCalled()
  })

  it('斷線後回到前景：重連並呼叫 onResync', () => {
    const onResync = vi.fn()
    renderHook(() => useMeetingStatusSocket(() => {}, onResync))
    act(() => setVisibility('hidden'))
    act(() => vi.advanceTimersByTime(HIDDEN_GRACE_MS))
    expect(openSockets()).toHaveLength(0)

    act(() => setVisibility('visible'))
    expect(openSockets()).toHaveLength(1)
    expect(onResync).toHaveBeenCalledTimes(1)
  })

  it('可見但無操作超過閒置時間 → 斷線；操作後重連', () => {
    const onResync = vi.fn()
    renderHook(() => useMeetingStatusSocket(() => {}, onResync))

    act(() => vi.advanceTimersByTime(IDLE_TIMEOUT_MS - 1000))
    act(() => window.dispatchEvent(new Event('pointerdown'))) // 操作會重新計時
    act(() => vi.advanceTimersByTime(IDLE_TIMEOUT_MS - 1000))
    expect(openSockets()).toHaveLength(1)

    act(() => vi.advanceTimersByTime(1000))
    expect(openSockets()).toHaveLength(0)

    act(() => window.dispatchEvent(new Event('keydown')))
    expect(openSockets()).toHaveLength(1)
    expect(onResync).toHaveBeenCalledTimes(1)
  })

  it('背景狀態下掛載不連線，回到前景才連', () => {
    visibility = 'hidden'
    renderHook(() => useMeetingStatusSocket(() => {}))
    expect(FakeWebSocket.instances).toHaveLength(0)

    act(() => setVisibility('visible'))
    expect(openSockets()).toHaveLength(1)
  })

  it('unmount 關閉連線並移除監聽', () => {
    const { unmount } = renderHook(() => useMeetingStatusSocket(() => {}))
    unmount()
    expect(openSockets()).toHaveLength(0)

    act(() => setVisibility('visible'))
    act(() => window.dispatchEvent(new Event('pointerdown')))
    expect(FakeWebSocket.instances).toHaveLength(1)
  })

  it('收到 meetingStatus 事件轉給 onEvent', () => {
    const onEvent = vi.fn()
    renderHook(() => useMeetingStatusSocket(onEvent))
    act(() =>
      FakeWebSocket.instances[0].onmessage?.({
        data: JSON.stringify({ type: 'meetingStatus', meetingId: 'm1', status: 'done' }),
      }),
    )
    expect(onEvent).toHaveBeenCalledWith({ meetingId: 'm1', status: 'done', errorMessage: undefined })
  })
})
