import { useCallback, useRef, useState, type DragEvent } from 'react'
import { CheckCircle2, Upload } from 'lucide-react'

import type { Language, Meeting, Scenario } from '../services/api/client'
import { useFileUpload } from '../hooks/useFileUpload'

export function UploadZone({
  onUploaded,
  scenario = 'meeting',
  language = 'zh-TW',
}: {
  onUploaded?: (m: Meeting) => void
  scenario?: Scenario
  language?: Language
}) {
  const { state, upload, reset } = useFileUpload({ scenario, language, onUploaded })
  const [dragOver, setDragOver] = useState(false)
  const inputRef = useRef<HTMLInputElement>(null)

  const onDrop = useCallback(
    (e: DragEvent) => {
      e.preventDefault()
      setDragOver(false)
      const file = e.dataTransfer.files[0]
      if (file) void upload(file)
    },
    [upload],
  )

  if (state.phase === 'uploading') {
    return (
      <div className="flex flex-col items-center gap-2 rounded-xl border border-border bg-surface px-4 py-5">
        <p className="m-0 max-w-full truncate text-sm">上傳中：{state.file.name}</p>
        <progress className="progress w-56" value={state.percent} max={100} />
        <p className="m-0 text-xs text-muted">{state.percent}%</p>
      </div>
    )
  }

  if (state.phase === 'done') {
    return (
      <div className="flex flex-col items-center gap-3 rounded-xl border border-border bg-surface px-4 py-5">
        <p className="m-0 flex items-center gap-2 text-sm">
          <CheckCircle2 className="size-4 text-emerald-500" />
          「{state.meeting.title}」已進入處理佇列
        </p>
        <button type="button" className="btn btn-secondary h-9" onClick={reset}>
          再上傳一個
        </button>
      </div>
    )
  }

  if (state.phase === 'error') {
    return (
      <div className="flex flex-col items-center gap-3 rounded-xl border border-red-500/30 bg-red-500/5 px-4 py-5">
        <p className="m-0 text-sm text-red-500">{state.message}</p>
        <div className="flex gap-2">
          <button type="button" className="btn btn-primary h-9" onClick={() => void upload(state.file)}>
            重試
          </button>
          <button type="button" className="btn btn-secondary h-9" onClick={reset}>
            取消
          </button>
        </div>
      </div>
    )
  }

  return (
    <div
      className={`flex flex-col items-center gap-2 sm:rounded-xl sm:border-2 sm:border-dashed sm:px-4 sm:py-6 ${
        dragOver ? 'sm:border-accent sm:bg-accent/5' : 'sm:border-border'
      }`}
      onDragOver={(e) => {
        e.preventDefault()
        setDragOver(true)
      }}
      onDragLeave={() => setDragOver(false)}
      onDrop={onDrop}
    >
      <p className="m-0 hidden text-sm text-muted sm:block">拖曳音訊檔到這裡，或</p>
      <button type="button" className="btn btn-secondary w-full sm:w-auto" onClick={() => inputRef.current?.click()}>
        <Upload className="size-4" />
        上傳音訊檔
      </button>
      <p className="m-0 text-xs text-muted">mp3 / m4a / webm / wav，200MB 以內</p>
      <input
        ref={inputRef}
        type="file"
        accept="audio/*,.m4a,.mp3,.wav,.webm"
        hidden
        onChange={(e) => {
          const file = e.target.files?.[0]
          if (file) void upload(file)
          e.target.value = ''
        }}
      />
    </div>
  )
}
