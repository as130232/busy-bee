import { useState } from 'react'

import { useRecorder } from '../hooks/useRecorder'
import { auth } from '../services/firebase'
import { uploadAudio } from '../services/upload'
import type { Meeting } from '../services/api/client'

function fmt(sec: number): string {
  const m = Math.floor(sec / 60)
  const s = sec % 60
  return `${m}:${String(s).padStart(2, '0')}`
}

type UploadState =
  | { phase: 'idle' }
  | { phase: 'uploading'; percent: number }
  | { phase: 'error'; message: string; file: File }

export function RecorderPanel({ onUploaded }: { onUploaded?: (m: Meeting) => void }) {
  const rec = useRecorder()
  const [upload, setUpload] = useState<UploadState>({ phase: 'idle' })

  const uploadFile = async (file: File) => {
    const fbUser = auth.currentUser
    if (!fbUser) return
    setUpload({ phase: 'uploading', percent: 0 })
    try {
      const token = await fbUser.getIdToken()
      const title = file.name.replace(/\.[^.]+$/, '')
      const meeting = await uploadAudio(token, title, file, (percent) =>
        setUpload({ phase: 'uploading', percent }),
      )
      setUpload({ phase: 'idle' })
      onUploaded?.(meeting)
    } catch (e) {
      setUpload({ phase: 'error', message: e instanceof Error ? e.message : '上傳失敗', file })
    }
  }

  const finish = async () => {
    const file = await rec.stop()
    if (file) await uploadFile(file)
  }

  if (rec.phase === 'unsupported') {
    return <p className="muted">此瀏覽器不支援錄音，請使用上方檔案上傳。</p>
  }

  if (upload.phase === 'uploading') {
    return (
      <div className="recorder">
        <p>錄音上傳中…</p>
        <progress value={upload.percent} max={100} />
      </div>
    )
  }

  if (upload.phase === 'error') {
    return (
      <div className="recorder">
        <p className="error">{upload.message}</p>
        <button type="button" onClick={() => void uploadFile(upload.file)}>
          重試上傳
        </button>
      </div>
    )
  }

  if (!rec.isActive) {
    return (
      <div className="recorder">
        <button type="button" onClick={() => void rec.start()}>
          🎙️ 開始錄音
        </button>
        {rec.error && <p className="error">{rec.error}</p>}
      </div>
    )
  }

  return (
    <div className="recorder recording">
      <span className={`rec-dot${rec.phase === 'paused' ? ' paused' : ''}`} />
      <span className="rec-time">{fmt(rec.elapsedSec)}</span>
      {rec.phase === 'recording' ? (
        <button type="button" className="secondary" onClick={rec.pause}>
          暫停
        </button>
      ) : (
        <button type="button" className="secondary" onClick={rec.resume}>
          繼續
        </button>
      )}
      <button type="button" onClick={() => void finish()}>
        結束並上傳
      </button>
      <button type="button" className="secondary" onClick={() => void rec.discard()}>
        捨棄
      </button>
    </div>
  )
}
