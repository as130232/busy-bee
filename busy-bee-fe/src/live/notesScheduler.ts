// 即時重點更新排程（F-LIVE）：每 intervalMs 檢查一次，新增逐字稿達 minChars 才呼叫；
// 同時只跑一個請求；手動「立即更新」不看門檻但至少間隔 manualGapMs（後端也有同樣冷卻）。
export interface NotesSchedulerOptions {
  intervalMs: number
  minChars: number
  manualGapMs: number
  /** 上次更新後新增的逐字稿字數。 */
  pendingChars: () => number
  /** 實際更新（呼叫後端）；失敗由呼叫端處理顯示，排程照常。 */
  run: () => Promise<void>
}

export class NotesScheduler {
  busy = false
  private timer: ReturnType<typeof setInterval> | undefined
  private lastRun = -Infinity
  private readonly opts: NotesSchedulerOptions

  constructor(opts: NotesSchedulerOptions) {
    this.opts = opts
  }

  start(): void {
    this.stop()
    this.timer = setInterval(() => {
      if (this.opts.pendingChars() >= this.opts.minChars) void this.exec()
    }, this.opts.intervalMs)
  }

  stop(): void {
    clearInterval(this.timer)
    this.timer = undefined
  }

  /** 手動更新；回傳是否有實際送出。 */
  async manual(): Promise<boolean> {
    if (this.busy || Date.now() - this.lastRun < this.opts.manualGapMs || this.opts.pendingChars() === 0) {
      return false
    }
    await this.exec()
    return true
  }

  private async exec(): Promise<void> {
    if (this.busy) return
    this.busy = true
    this.lastRun = Date.now()
    try {
      await this.opts.run()
    } catch {
      /* 失敗由 run 自行回報；下一輪重試 */
    } finally {
      this.busy = false
    }
  }
}
