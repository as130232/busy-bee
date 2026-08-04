import { useCallback, useRef, useState } from 'react'

import { auth } from '../services/firebase'
import { uploadAudio } from '../services/upload'
import type { Meeting, Scenario } from '../services/api/client'

/** 音檔上傳狀態機（RecorderPanel / UploadZone 共用）。 */
export type FileUploadState =
  | { phase: 'idle' }
  | { phase: 'uploading'; percent: number; file: File }
  | { phase: 'done'; meeting: Meeting }
  | { phase: 'error'; message: string; file: File }

export interface UseFileUploadOptions {
  scenario?: Scenario
  onUploaded?: (m: Meeting) => void
  /** 由檔案決定標題；未提供時取去副檔名的檔名（空則「未命名會議」）。 */
  titleFor?: (file: File) => string
}

/**
 * 封裝「取 idToken → uploadAudio（含進度）→ 狀態轉移」的共用上傳邏輯，
 * 消除 RecorderPanel 與 UploadZone 各自複製的上傳流程（見重構計畫 P6）。
 * options 以 ref 持有，故 upload / reset 身分穩定（空依賴），呼叫端不必再包 useCallback。
 */
export function useFileUpload(options: UseFileUploadOptions = {}) {
  const [state, setState] = useState<FileUploadState>({ phase: 'idle' })
  const optsRef = useRef(options)
  optsRef.current = options

  const upload = useCallback(async (file: File) => {
    const { scenario = 'meeting', onUploaded, titleFor } = optsRef.current
    const fbUser = auth.currentUser
    if (!fbUser) return
    setState({ phase: 'uploading', percent: 0, file })
    try {
      const token = await fbUser.getIdToken()
      const title = titleFor ? titleFor(file) : file.name.replace(/\.[^.]+$/, '') || '未命名會議'
      const meeting = await uploadAudio(
        token,
        title,
        file,
        (percent) => setState({ phase: 'uploading', percent, file }),
        scenario,
      )
      setState({ phase: 'done', meeting })
      onUploaded?.(meeting)
    } catch (e) {
      setState({ phase: 'error', message: e instanceof Error ? e.message : '上傳失敗', file })
    }
  }, [])

  const reset = useCallback(() => setState({ phase: 'idle' }), [])

  return { state, upload, reset }
}
