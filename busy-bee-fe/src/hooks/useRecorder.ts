import { useCallback, useEffect, useRef, useState } from 'react'

export type RecorderPhase = 'idle' | 'recording' | 'paused' | 'unsupported'

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

  const isActive = state.phase === 'recording' || state.phase === 'paused'

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
      recorder.ondataavailable = (e) => {
        if (e.data.size > 0) chunksRef.current.push(e.data)
      }
      recorder.start(1000) // 每秒收一塊，避免長錄音單一大 chunk
      recorderRef.current = recorder
      setState({ phase: 'recording', elapsedSec: 0, error: null })
      startTimer()
    } catch (e) {
      const denied = e instanceof DOMException && e.name === 'NotAllowedError'
      setState((s) => ({
        ...s,
        error: denied
          ? '無法取得麥克風權限，請在瀏覽器網址列允許麥克風後重試。'
          : '啟動錄音失敗，請確認麥克風可用。',
      }))
    }
  }, [startTimer])

  const pause = useCallback(() => {
    recorderRef.current?.pause()
    stopTimer()
    setState((s) => ({ ...s, phase: 'paused' }))
  }, [stopTimer])

  const resume = useCallback(() => {
    recorderRef.current?.resume()
    startTimer()
    setState((s) => ({ ...s, phase: 'recording' }))
  }, [startTimer])

  /** 結束錄音並回傳音訊檔；無資料時回 null。 */
  const stop = useCallback((): Promise<File | null> => {
    return new Promise((resolve) => {
      const recorder = recorderRef.current
      if (!recorder) {
        resolve(null)
        return
      }
      recorder.onstop = () => {
        recorder.stream.getTracks().forEach((t) => t.stop()) // 釋放麥克風
        const mimeType = recorder.mimeType
        const blob = new Blob(chunksRef.current, { type: mimeType })
        chunksRef.current = []
        recorderRef.current = null
        stopTimer()
        setState({ phase: 'idle', elapsedSec: 0, error: null })
        if (blob.size === 0) {
          resolve(null)
          return
        }
        const stamp = new Date().toISOString().slice(0, 16).replace('T', ' ')
        resolve(new File([blob], `會議錄音 ${stamp}.${extFor(mimeType)}`, { type: mimeType }))
      }
      recorder.stop()
    })
  }, [stopTimer])

  const discard = useCallback(async () => {
    const recorder = recorderRef.current
    if (recorder) {
      recorder.onstop = () => recorder.stream.getTracks().forEach((t) => t.stop())
      recorder.stop()
    }
    chunksRef.current = []
    recorderRef.current = null
    stopTimer()
    setState({ phase: 'idle', elapsedSec: 0, error: null })
  }, [stopTimer])

  return { ...state, isActive, start, pause, resume, stop, discard }
}
