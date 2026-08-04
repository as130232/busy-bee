import { type RefObject, useEffect, useState } from 'react'
import { createPortal } from 'react-dom'
import { FastForward, Pause, Play, Rewind } from 'lucide-react'

import { getMeetingAudioURL } from '../services/api/client'
import { useApiCall } from '../hooks/useApiCall'
import { formatClock } from '../utils/format'

/** 音檔播放器：播放/暫停、±10 秒、可拖曳進度條。時長以後端 durationSeconds 為準
 *  （MediaRecorder 產生的 webm 常無 duration metadata）。 */
export function AudioPlayer({
  meetingId,
  durationSeconds,
  audioRef,
}: {
  meetingId: string
  durationSeconds: number
  audioRef: RefObject<HTMLAudioElement | null>
}) {
  const [url, setUrl] = useState<string | null>(null)
  const [failed, setFailed] = useState(false)
  const [playing, setPlaying] = useState(false)
  const [cur, setCur] = useState(0)
  const call = useApiCall()

  useEffect(() => {
    let active = true
    void (async () => {
      try {
        const { url } = await call(getMeetingAudioURL, meetingId)
        if (active) setUrl(url)
      } catch {
        if (active) setFailed(true)
      }
    })()
    return () => {
      active = false
    }
  }, [meetingId, call])

  if (failed) return null

  const total = durationSeconds > 0 ? durationSeconds : 0
  const toggle = () => {
    const a = audioRef.current
    if (!a) return
    if (a.paused) void a.play()
    else a.pause()
  }
  const skip = (delta: number) => {
    const a = audioRef.current
    if (a) a.currentTime = Math.max(0, a.currentTime + delta)
  }
  const seek = (v: number) => {
    const a = audioRef.current
    if (a) a.currentTime = v
    setCur(v)
  }

  // 貼底常駐 mini-player：透過 Portal 掛到 body，避免 AppShell 帶 transform 的 <main>
  // 成為 fixed containing block（與 Sheet 同源問題）。閱讀長逐字稿時隨時可播/seek。
  return createPortal(
    <div className="fixed inset-x-0 bottom-0 z-30 border-t border-border bg-surface/95 pb-[env(safe-area-inset-bottom)] backdrop-blur">
      <div className="mx-auto flex h-14 max-w-xl items-center gap-1.5 px-3">
        <audio
          ref={audioRef}
          src={url ?? undefined}
          preload="metadata"
          onTimeUpdate={(e) => setCur(e.currentTarget.currentTime)}
          onPlay={() => setPlaying(true)}
          onPause={() => setPlaying(false)}
          onEnded={() => setPlaying(false)}
        />
        <button
          type="button"
          className="btn btn-ghost size-9 shrink-0 px-0 text-muted"
          aria-label="倒退 10 秒"
          onClick={() => skip(-10)}
          disabled={!url}
        >
          <Rewind className="size-4" />
        </button>
        <button
          type="button"
          className="btn btn-primary size-10 shrink-0 rounded-full px-0"
          aria-label={playing ? '暫停' : '播放'}
          onClick={toggle}
          disabled={!url}
        >
          {playing ? <Pause className="size-4" /> : <Play className="size-4" />}
        </button>
        <button
          type="button"
          className="btn btn-ghost size-9 shrink-0 px-0 text-muted"
          aria-label="快轉 10 秒"
          onClick={() => skip(10)}
          disabled={!url}
        >
          <FastForward className="size-4" />
        </button>
        <span className="w-9 shrink-0 text-right font-mono text-[11px] tabular-nums text-muted">{formatClock(cur)}</span>
        <input
          type="range"
          className="min-w-0 flex-1 accent-accent"
          min={0}
          max={total || 0}
          step={0.1}
          value={Math.min(cur, total || 0)}
          onChange={(e) => seek(Number(e.target.value))}
          disabled={!url}
          aria-label="播放進度"
        />
        <span className="w-9 shrink-0 font-mono text-[11px] tabular-nums text-muted">{formatClock(total)}</span>
      </div>
    </div>,
    document.body,
  )
}
