import { useEffect, useState } from 'react'
import { Download, Share2, X } from 'lucide-react'

import { getMeetingAudioURL } from '../services/api/client'
import { useApiCall } from '../hooks/useApiCall'
import { audioExtension, canShareFile, safeFilename, shareOrDownload } from '../services/shareFile'
import { Sheet } from './Sheet'

type Phase = { kind: 'loading'; percent: number | null } | { kind: 'ready'; file: File } | { kind: 'error'; message: string }

/** 下載音檔，有 Content-Length 時回報進度（0–100）。 */
async function fetchAudio(url: string, signal: AbortSignal, onProgress: (p: number | null) => void): Promise<Blob> {
  const res = await fetch(url, { signal })
  if (!res.ok) throw new Error(`下載失敗（${res.status}）`)
  const total = Number(res.headers.get('Content-Length')) || 0
  const type = res.headers.get('Content-Type') ?? ''
  if (!res.body || total === 0) {
    onProgress(null)
    return await res.blob()
  }
  const reader = res.body.getReader()
  const chunks: Uint8Array[] = []
  let received = 0
  for (;;) {
    const { done, value } = await reader.read()
    if (done) break
    chunks.push(value)
    received += value.length
    onProgress(Math.min(100, Math.round((received / total) * 100)))
  }
  return new Blob(chunks as BlobPart[], { type })
}

/**
 * 分享錄音檔：先下載成檔案，再讓使用者點「分享」。
 * 分兩步是因為 iOS 的 navigator.share 必須在點按當下呼叫，下載大檔會讓點按失效。
 */
export function AudioShareSheet({ meetingId, title, onClose }: { meetingId: string; title: string; onClose: () => void }) {
  const call = useApiCall()
  const [phase, setPhase] = useState<Phase>({ kind: 'loading', percent: null })
  const [attempt, setAttempt] = useState(0)

  useEffect(() => {
    const ctrl = new AbortController()
    setPhase({ kind: 'loading', percent: null })
    void (async () => {
      try {
        const { url } = await call(getMeetingAudioURL, meetingId)
        const blob = await fetchAudio(url, ctrl.signal, (percent) => setPhase({ kind: 'loading', percent }))
        const name = `${safeFilename(title)}${audioExtension(blob.type, new URL(url).pathname)}`
        setPhase({ kind: 'ready', file: new File([blob], name, { type: blob.type || 'audio/mp4' }) })
      } catch (e) {
        if (ctrl.signal.aborted) return
        setPhase({ kind: 'error', message: e instanceof Error && e.message ? e.message : '無法取得錄音檔' })
      }
    })()
    return () => ctrl.abort()
  }, [call, meetingId, title, attempt])

  const share = async () => {
    if (phase.kind !== 'ready') return
    const result = await shareOrDownload(phase.file)
    if (result !== 'cancelled') onClose()
  }

  const shareable = phase.kind === 'ready' && canShareFile(phase.file)

  return (
    <Sheet onClose={onClose}>
      <div className="flex items-center justify-between">
        <h2 className="m-0 text-heading font-semibold">分享錄音檔</h2>
        <button type="button" className="btn btn-ghost size-9 px-0" aria-label="關閉" onClick={onClose}>
          <X className="size-4" />
        </button>
      </div>

      {phase.kind === 'loading' && (
        <div className="flex flex-col gap-2 py-2" role="status">
          <p className="m-0 text-sm text-muted">
            準備錄音檔中{phase.percent !== null ? `… ${phase.percent}%` : '…'}
          </p>
          <div className="h-1.5 overflow-hidden rounded-full bg-border">
            <div
              className={`h-full rounded-full bg-accent transition-[width] ${phase.percent === null ? 'w-1/3 animate-pulse' : ''}`}
              style={phase.percent !== null ? { width: `${phase.percent}%` } : undefined}
            />
          </div>
        </div>
      )}

      {phase.kind === 'error' && (
        <div className="flex flex-col gap-3 py-2">
          <p role="alert" className="m-0 text-sm text-danger">
            {phase.message}
          </p>
          <button type="button" className="btn btn-secondary" onClick={() => setAttempt((n) => n + 1)}>
            重試
          </button>
        </div>
      )}

      {phase.kind === 'ready' && (
        <div className="flex flex-col gap-3 py-2">
          <p className="m-0 text-sm text-muted">
            {phase.file.name}（{(phase.file.size / 1024 / 1024).toFixed(1)} MB）
          </p>
          <button type="button" className="btn btn-primary" onClick={() => void share()}>
            {shareable ? <Share2 className="size-4" /> : <Download className="size-4" />}
            {shareable ? '分享' : '下載'}
          </button>
        </div>
      )}
    </Sheet>
  )
}
