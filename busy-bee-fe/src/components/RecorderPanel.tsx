import { useState } from 'react'
import { Mic, Pause, Play, Trash2 } from 'lucide-react'

import { useRecorder } from '../hooks/useRecorder'
import { auth } from '../services/firebase'
import { uploadAudio } from '../services/upload'
import { scenarioLabels, type Meeting, type Scenario } from '../services/api/client'
import { scenarioThemes } from './scenarioTheme'

function fmt(sec: number): string {
  const m = Math.floor(sec / 60)
  const s = sec % 60
  return `${m}:${String(s).padStart(2, '0')}`
}

type UploadState =
  | { phase: 'idle' }
  | { phase: 'uploading'; percent: number }
  | { phase: 'error'; message: string; file: File }

export function RecorderPanel({
  onUploaded,
  highlight = false,
  scenario = 'meeting',
}: {
  onUploaded?: (m: Meeting) => void
  highlight?: boolean
  scenario?: Scenario
}) {
  const rec = useRecorder()
  const [upload, setUpload] = useState<UploadState>({ phase: 'idle' })

  const uploadFile = async (file: File) => {
    const fbUser = auth.currentUser
    if (!fbUser) return
    setUpload({ phase: 'uploading', percent: 0 })
    try {
      const token = await fbUser.getIdToken()
      // 標題依情境組成：「會議錄音 …」/「閒聊錄音 …」（檔名前綴為中性「錄音 日期時間」）。
      const title = `${scenarioLabels[scenario]}${file.name.replace(/\.[^.]+$/, '')}`
      const meeting = await uploadAudio(
        token,
        title,
        file,
        (percent) => setUpload({ phase: 'uploading', percent }),
        scenario,
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
    return <p className="text-center text-sm text-muted">此瀏覽器不支援錄音，請改用下方檔案上傳。</p>
  }

  if (upload.phase === 'uploading') {
    return (
      <div className="flex flex-col items-center gap-3 py-8">
        <p className="m-0 text-sm text-muted">錄音上傳中…</p>
        <progress className="progress w-56" value={upload.percent} max={100} />
      </div>
    )
  }

  if (upload.phase === 'error') {
    return (
      <div className="flex flex-col items-center gap-3 py-8">
        <p className="m-0 text-sm text-red-500">{upload.message}</p>
        <button type="button" className="btn btn-primary" onClick={() => void uploadFile(upload.file)}>
          重試上傳
        </button>
      </div>
    )
  }

  const theme = scenarioThemes[scenario]

  if (!rec.isActive) {
    return (
      <div className="flex flex-col items-center gap-6">
        <button
          type="button"
          aria-label="開始錄音"
          onClick={() => void rec.start()}
          className="group relative flex size-40 items-center justify-center"
        >
          {/* 向外擴散的聲波環（三層錯開，像聲納）；顏色隨情境 */}
          <span className={`animate-sonar absolute size-28 rounded-full border ${theme.ring}`} />
          <span className={`animate-sonar absolute size-28 rounded-full border ${theme.ring} [animation-delay:0.9s]`} />
          <span className={`animate-sonar absolute size-28 rounded-full border ${theme.ring} [animation-delay:1.8s]`} />

          {/* 柔和光暈（呼吸脹縮） */}
          <span className={`animate-breathe absolute size-32 rounded-full blur-2xl ${theme.glow}`} />

          {/* 環繞旋轉的光點 */}
          <span className="absolute size-40 animate-[spin_9s_linear_infinite]">
            <span className={`absolute top-0 left-1/2 size-1.5 -translate-x-1/2 rounded-full shadow-[0_0_8px] ${theme.dotBright}`} />
            <span className={`absolute bottom-1 left-1/2 size-1 -translate-x-1/2 rounded-full ${theme.dotSoft}`} />
            <span className={`absolute top-1/2 right-1 size-1 -translate-y-1/2 rounded-full ${theme.dotFaint}`} />
          </span>

          {/* 主鈕（呼吸放大縮小）；漸層/陰影/高亮環皆隨情境 */}
          <span
            className={`animate-breathe relative flex size-24 items-center justify-center rounded-full bg-gradient-to-b text-zinc-900 shadow-[0_0_70px_-8px] transition-shadow duration-300 group-hover:shadow-[0_0_100px_-4px] ${theme.button} ${theme.buttonHover}${
              highlight ? ` ring-4 ${theme.highlightRing} ring-offset-2 ring-offset-bg` : ''
            }`}
          >
            <Mic className="size-9" strokeWidth={1.75} />
          </span>
        </button>
        <p className="m-0 text-sm text-muted">輕觸開始錄音</p>
        {rec.error && <p className="m-0 text-sm text-red-500">{rec.error}</p>}
      </div>
    )
  }

  return (
    <div className="animate-scale-in flex flex-col items-center gap-5 pt-8 pb-2">
      <div className="flex items-center gap-3">
        <span className="relative flex size-3">
          {rec.phase === 'recording' && (
            <span className="absolute inline-flex size-full animate-ping rounded-full bg-red-500 opacity-75" />
          )}
          <span
            className={`relative inline-flex size-3 rounded-full ${
              rec.phase === 'paused' ? 'bg-muted' : 'bg-red-500'
            }`}
          />
        </span>
        <span className="font-mono text-5xl font-medium tabular-nums">{fmt(rec.elapsedSec)}</span>
      </div>
      <div className="flex items-center gap-3">
        {rec.phase === 'recording' ? (
          <button type="button" className="btn btn-secondary" onClick={rec.pause}>
            <Pause className="size-4" />
            暫停
          </button>
        ) : (
          <button type="button" className="btn btn-secondary" onClick={rec.resume}>
            <Play className="size-4" />
            繼續
          </button>
        )}
        <button type="button" className="btn btn-primary" onClick={() => void finish()}>
          結束並上傳
        </button>
        <button
          type="button"
          aria-label="捨棄錄音"
          className="btn btn-ghost size-11 px-0 text-red-500 hover:text-red-500"
          onClick={() => void rec.discard()}
        >
          <Trash2 className="size-5" />
        </button>
      </div>
    </div>
  )
}
