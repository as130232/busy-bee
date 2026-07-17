import { useCallback, useRef, useState, type DragEvent } from 'react'

import { auth } from '../services/firebase'
import { uploadAudio } from '../services/upload'
import type { Meeting } from '../services/api/client'

type UploadState =
  | { phase: 'idle' }
  | { phase: 'uploading'; percent: number; fileName: string }
  | { phase: 'done'; meeting: Meeting }
  | { phase: 'error'; message: string; file: File }

export function UploadZone({ onUploaded }: { onUploaded?: (m: Meeting) => void }) {
  const [state, setState] = useState<UploadState>({ phase: 'idle' })
  const [dragOver, setDragOver] = useState(false)
  const inputRef = useRef<HTMLInputElement>(null)

  const startUpload = useCallback(
    async (file: File) => {
      const fbUser = auth.currentUser
      if (!fbUser) return
      setState({ phase: 'uploading', percent: 0, fileName: file.name })
      try {
        const idToken = await fbUser.getIdToken()
        const title = file.name.replace(/\.[^.]+$/, '') || '未命名會議'
        const meeting = await uploadAudio(idToken, title, file, (percent) =>
          setState({ phase: 'uploading', percent, fileName: file.name }),
        )
        setState({ phase: 'done', meeting })
        onUploaded?.(meeting)
      } catch (e) {
        setState({
          phase: 'error',
          message: e instanceof Error ? e.message : '上傳失敗',
          file,
        })
      }
    },
    [onUploaded],
  )

  const onDrop = useCallback(
    (e: DragEvent) => {
      e.preventDefault()
      setDragOver(false)
      const file = e.dataTransfer.files[0]
      if (file) void startUpload(file)
    },
    [startUpload],
  )

  if (state.phase === 'uploading') {
    return (
      <div className="upload-zone">
        <p>上傳中：{state.fileName}</p>
        <progress value={state.percent} max={100} />
        <p className="muted">{state.percent}%</p>
      </div>
    )
  }

  if (state.phase === 'done') {
    return (
      <div className="upload-zone">
        <p>✅ 「{state.meeting.title}」已進入處理佇列</p>
        <button type="button" onClick={() => setState({ phase: 'idle' })}>
          再上傳一個
        </button>
      </div>
    )
  }

  if (state.phase === 'error') {
    return (
      <div className="upload-zone">
        <p className="error">{state.message}</p>
        <button type="button" onClick={() => void startUpload(state.file)}>
          重試
        </button>
        <button type="button" className="secondary" onClick={() => setState({ phase: 'idle' })}>
          取消
        </button>
      </div>
    )
  }

  return (
    <div
      className={`upload-zone${dragOver ? ' drag-over' : ''}`}
      onDragOver={(e) => {
        e.preventDefault()
        setDragOver(true)
      }}
      onDragLeave={() => setDragOver(false)}
      onDrop={onDrop}
    >
      <p>拖曳音訊檔到這裡（mp3 / m4a / webm / wav，200MB 以內）</p>
      <button type="button" onClick={() => inputRef.current?.click()}>
        選擇檔案
      </button>
      <input
        ref={inputRef}
        type="file"
        accept="audio/*,.m4a,.mp3,.wav,.webm"
        hidden
        onChange={(e) => {
          const file = e.target.files?.[0]
          if (file) void startUpload(file)
          e.target.value = ''
        }}
      />
    </div>
  )
}
