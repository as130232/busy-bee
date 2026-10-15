import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { NotesScheduler } from './notesScheduler'

beforeEach(() => vi.useFakeTimers())
afterEach(() => vi.useRealTimers())

function make(pending: { chars: number }, run = vi.fn(async () => {})) {
  const s = new NotesScheduler({ intervalMs: 90_000, minChars: 200, manualGapMs: 15_000, pendingChars: () => pending.chars, run })
  return { s, run }
}

describe('NotesScheduler', () => {
  it('每輪新增字數達門檻才更新', async () => {
    const pending = { chars: 100 }
    const { s, run } = make(pending)
    s.start()
    await vi.advanceTimersByTimeAsync(90_000)
    expect(run).not.toHaveBeenCalled()
    pending.chars = 250
    await vi.advanceTimersByTimeAsync(90_000)
    expect(run).toHaveBeenCalledTimes(1)
    s.stop()
    await vi.advanceTimersByTimeAsync(200_000)
    expect(run).toHaveBeenCalledTimes(1)
  })

  it('同時只跑一個；前一個沒回來就跳過這輪', async () => {
    let resolve!: () => void
    const run = vi.fn(() => new Promise<void>((r) => (resolve = r)))
    const { s } = make({ chars: 500 }, run)
    s.start()
    await vi.advanceTimersByTimeAsync(90_000)
    await vi.advanceTimersByTimeAsync(90_000)
    expect(run).toHaveBeenCalledTimes(1)
    expect(s.busy).toBe(true)
    resolve()
    await vi.advanceTimersByTimeAsync(0)
    expect(s.busy).toBe(false)
  })

  it('手動更新不看門檻，但至少間隔 15 秒且要有新內容', async () => {
    const pending = { chars: 10 }
    const { s, run } = make(pending)
    expect(await s.manual()).toBe(true)
    expect(await s.manual()).toBe(false)
    await vi.advanceTimersByTimeAsync(15_000)
    pending.chars = 0
    expect(await s.manual()).toBe(false)
    pending.chars = 1
    expect(await s.manual()).toBe(true)
    expect(run).toHaveBeenCalledTimes(2)
  })

  it('更新失敗不影響下一輪', async () => {
    const run = vi.fn().mockRejectedValueOnce(new Error('x')).mockResolvedValue(undefined)
    const { s } = make({ chars: 500 }, run)
    s.start()
    await vi.advanceTimersByTimeAsync(90_000)
    await vi.advanceTimersByTimeAsync(90_000)
    expect(run).toHaveBeenCalledTimes(2)
    s.stop()
  })
})
