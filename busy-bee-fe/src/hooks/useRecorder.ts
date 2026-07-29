import { useCallback, useEffect, useRef, useState } from 'react'

export type RecorderPhase = 'idle' | 'recording' | 'paused' | 'interrupted' | 'unsupported'

interface RecorderState {
  phase: RecorderPhase
  /** 已錄製秒數（暫停時不累計） */
  elapsedSec: number
  error: string | null
}

/** 依瀏覽器支援度挑選錄音格式：Chrome/Firefox → webm/opus；Safari → mp4(aac) */
function pickMimeType(): string | null {
  const candidates = ['audio/webm;codecs=opus', 'audio/webm', 'audio/mp4']
  for (const t of candidates) {
    if (MediaRecorder.isTypeSupported(t)) return t
  }
  return null
}

function extFor(mimeType: string): string {
  return mimeType.startsWith('audio/mp4') ? 'm4a' : 'webm'
}

/**
 * 瀏覽器錄音。stop() 回傳可直接走上傳流程的 File；
 * 錄音中頁面關閉/重整會跳出瀏覽器原生警告（beforeunload）。
 *
 * iOS 休眠/進背景會凍結頁面並切斷麥克風串流，MediaRecorder 被系統停成 inactive：
 * - 錄音中持有 Screen Wake Lock，避免螢幕因閒置自動休眠（主要對症）。
 * - 被系統中斷時進入 'interrupted'（停掉假象計時器），仍能結束並上傳已錄片段。
 * - stop() 對已 inactive 的 recorder 也能收尾，不再卡死、不整段丟失。
 */
export function useRecorder() {
  const [state, setState] = useState<RecorderState>({
    phase: typeof MediaRecorder === 'undefined' ? 'unsupported' : 'idle',
    elapsedSec: 0,
    error: null,
  })
  const recorderRef = useRef<MediaRecorder | null>(null)
  const chunksRef = useRef<Blob[]>([])
  const timerRef = useRef<ReturnType<typeof setInterval> | undefined>(undefined)
  const wakeLockRef = useRef<WakeLockSentinel | null>(null)
  // 區分「使用者主動結束/捨棄」與「系統中斷」：後者才標記為 interrupted。
  const userStoppingRef = useRef(false)

  const isActive =
    state.phase === 'recording' || state.phase === 'paused' || state.phase === 'interrupted'

  // 錄音中離開頁面 → 原生「資料未儲存」警告
  useEffect(() => {
    if (!isActive) return
    const warn = (e: BeforeUnloadEvent) => e.preventDefault()
    window.addEventListener('beforeunload', warn)
    return () => window.removeEventListener('beforeunload', warn)
  }, [isActive])

  const startTimer = useCallback(() => {
    timerRef.current = setInterval(
      () => setState((s) => ({ ...s, elapsedSec: s.elapsedSec + 1 })),
      1000,
    )
  }, [])

  const stopTimer = useCallback(() => clearInterval(timerRef.current), [])

  // Screen Wake Lock：防止螢幕因閒置自動休眠（iOS 16.4+ / Chrome 支援；不支援時靜默略過）。
  const acquireWakeLock = useCallback(async () => {
    if (!('wakeLock' in navigator)) return
    try {
      wakeLockRef.current = await navigator.wakeLock.request('screen')
    } catch {
      wakeLockRef.current = null // 取不到（權限/背景）不影響錄音，忽略
    }
  }, [])

  const releaseWakeLock = useCallback(() => {
    wakeLockRef.current?.release().catch(() => {
      /* 已釋放或頁面關閉，忽略 */
    })
    wakeLockRef.current = null
  }, [])

  // 卸載時確保釋放 Wake Lock（避免殘留持有）
  useEffect(() => () => releaseWakeLock(), [releaseWakeLock])

  // 前景/背景切換：回前景且仍在錄音 → Wake Lock 進背景會被系統釋放，需重取；
  // 轉入背景前主動 flush 一塊，盡量保住進背景前的音檔。
  useEffect(() => {
    if (!isActive) return
    const onVisibility = () => {
      if (document.visibilityState === 'visible') {
        if (state.phase === 'recording') void acquireWakeLock()
      } else if (recorderRef.current?.state === 'recording') {
        try {
          recorderRef.current.requestData()
        } catch {
          /* 部分瀏覽器 inactive 時會丟錯，忽略 */
        }
      }
    }
    document.addEventListener('visibilitychange', onVisibility)
    return () => document.removeEventListener('visibilitychange', onVisibility)
  }, [isActive, state.phase, acquireWakeLock])

  // 從已收到的 chunks 組出檔案並清理狀態（無論 recorder 是使用者停或被系統停都適用）。
  const finalize = useCallback(
    (recorder: MediaRecorder): File | null => {
      recorder.stream.getTracks().forEach((t) => t.stop()) // 釋放麥克風（可重複呼叫）
      const mimeType = recorder.mimeType
      const blob = new Blob(chunksRef.current, { type: mimeType })
      chunksRef.current = []
      recorderRef.current = null
      stopTimer()
      releaseWakeLock()
      setState({ phase: 'idle', elapsedSec: 0, error: null })
      if (blob.size === 0) return null
      // 檔名前綴中性（「錄音」）；情境前綴（會議/閒聊）由上層依所選情境組合標題。
      const stamp = new Date().toISOString().slice(0, 16).replace('T', ' ')
      return new File([blob], `錄音 ${stamp}.${extFor(mimeType)}`, { type: mimeType })
    },
    [stopTimer, releaseWakeLock],
  )

  const start = useCallback(async () => {
    const mimeType = pickMimeType()
    if (!mimeType) {
      setState((s) => ({ ...s, phase: 'unsupported', error: '此瀏覽器不支援錄音，請改用檔案上傳。' }))
      return
    }
    try {
      const stream = await navigator.mediaDevices.getUserMedia({ audio: true })
      const recorder = new MediaRecorder(stream, { mimeType })
      chunksRef.current = []
      userStoppingRef.current = false
      recorder.ondataavailable = (e) => {
        if (e.data.size > 0) chunksRef.current.push(e.data)
      }
      // 系統中斷（休眠/來電/背景）會停掉 recorder；非使用者主動 → 標記 interrupted，
      // 停掉會誤導的計時器，已錄片段留在 chunksRef 等使用者結束上傳。
      const onUnexpectedStop = () => {
        if (userStoppingRef.current) return // 使用者主動：由 stop()/discard() 處理
        stopTimer()
        releaseWakeLock()
        recorder.stream.getTracks().forEach((t) => t.stop())
        setState((s) => ({ ...s, phase: 'interrupted' }))
      }
      recorder.onstop = onUnexpectedStop
      recorder.onerror = onUnexpectedStop
      recorder.start(1000) // 每秒收一塊，避免長錄音單一大 chunk
      recorderRef.current = recorder
      setState({ phase: 'recording', elapsedSec: 0, error: null })
      startTimer()
      void acquireWakeLock()
    } catch (e) {
      const denied = e instanceof DOMException && e.name === 'NotAllowedError'
      setState((s) => ({
        ...s,
        error: denied
          ? '無法取得麥克風權限，請在瀏覽器網址列允許麥克風後重試。'
          : '啟動錄音失敗，請確認麥克風可用。',
      }))
    }
  }, [startTimer, stopTimer, acquireWakeLock, releaseWakeLock])

  const pause = useCallback(() => {
    if (recorderRef.current?.state !== 'recording') return
    recorderRef.current.pause()
    stopTimer()
    releaseWakeLock()
    setState((s) => ({ ...s, phase: 'paused' }))
  }, [stopTimer, releaseWakeLock])

  const resume = useCallback(() => {
    if (recorderRef.current?.state !== 'paused') return
    recorderRef.current.resume()
    startTimer()
    void acquireWakeLock()
    setState((s) => ({ ...s, phase: 'recording' }))
  }, [startTimer, acquireWakeLock])

  /** 結束錄音並回傳音訊檔；無資料時回 null。對已被系統停止的 recorder 也能收尾。 */
  const stop = useCallback((): Promise<File | null> => {
    return new Promise((resolve) => {
      const recorder = recorderRef.current
      if (!recorder) {
        resolve(null)
        return
      }
      userStoppingRef.current = true
      // 已被系統停止（休眠/來電）：直接用手上已收到的 chunks 收尾，不等不會來的 onstop。
      if (recorder.state === 'inactive') {
        resolve(finalize(recorder))
        return
      }
      recorder.onstop = () => resolve(finalize(recorder))
      try {
        recorder.stop()
      } catch {
        resolve(finalize(recorder)) // stop() 對非 recording 狀態丟錯時，仍以現有 chunks 收尾
      }
    })
  }, [finalize])

  const discard = useCallback(() => {
    const recorder = recorderRef.current
    userStoppingRef.current = true
    if (recorder) {
      if (recorder.state !== 'inactive') {
        recorder.onstop = () => recorder.stream.getTracks().forEach((t) => t.stop())
        try {
          recorder.stop()
        } catch {
          recorder.stream.getTracks().forEach((t) => t.stop())
        }
      } else {
        recorder.stream.getTracks().forEach((t) => t.stop())
      }
    }
    chunksRef.current = []
    recorderRef.current = null
    stopTimer()
    releaseWakeLock()
    setState({ phase: 'idle', elapsedSec: 0, error: null })
  }, [stopTimer, releaseWakeLock])

  return { ...state, isActive, start, pause, resume, stop, discard }
}
