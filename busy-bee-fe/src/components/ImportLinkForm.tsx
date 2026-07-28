import { useState } from 'react'
import { CheckCircle2, ClipboardPaste, Link2 } from 'lucide-react'

import { importMeeting, type Meeting, type Scenario } from '../services/api/client'
import { getIdToken } from '../services/token'
import { extractURL } from '../services/url'

type State =
  | { phase: 'idle' }
  | { phase: 'submitting' }
  | { phase: 'done'; meeting: Meeting }
  | { phase: 'error'; message: string }

/** 貼連結匯入：YouTube / Podcast / 直接音檔連結 → 後端抓取音訊 → 轉錄摘要。 */
export function ImportLinkForm({
  scenario = 'meeting',
  onImported,
}: {
  scenario?: Scenario
  onImported?: (m: Meeting) => void
}) {
  const [url, setUrl] = useState('')
  const [state, setState] = useState<State>({ phase: 'idle' })

  // iOS 無標準分享目標：改用「貼上剪貼簿」——使用者手勢觸發讀取剪貼簿並填入連結。
  const pasteFromClipboard = async () => {
    try {
      const link = extractURL(await navigator.clipboard.readText())
      if (link) {
        setUrl(link)
        setState({ phase: 'idle' })
      } else {
        setState({ phase: 'error', message: '剪貼簿沒有連結，請先複製影片/音訊網址' })
      }
    } catch {
      setState({ phase: 'error', message: '無法讀取剪貼簿，請手動貼上' })
    }
  }

  const submit = async () => {
    const link = url.trim()
    if (!link || state.phase === 'submitting') return
    setState({ phase: 'submitting' })
    try {
      const { meeting } = await importMeeting(await getIdToken(), { url: link, scenario })
      setState({ phase: 'done', meeting })
      setUrl('')
      onImported?.(meeting)
    } catch (e) {
      setState({ phase: 'error', message: e instanceof Error ? e.message : '匯入失敗' })
    }
  }

  if (state.phase === 'done') {
    return (
      <div className="flex flex-col items-center gap-3 rounded-xl border border-border bg-surface px-4 py-5">
        <p className="m-0 flex items-center gap-2 text-sm">
          <CheckCircle2 className="size-4 text-emerald-500" />
          已開始匯入，稍後可在「紀錄」查看
        </p>
        <button type="button" className="btn btn-secondary h-9" onClick={() => setState({ phase: 'idle' })}>
          再匯入一個
        </button>
      </div>
    )
  }

  return (
    <form
      className="flex flex-col gap-2"
      onSubmit={(e) => {
        e.preventDefault()
        void submit()
      }}
    >
      <div className="flex gap-2">
        <div className="relative flex-1">
          <Link2 className="pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2 text-muted" />
          <input
            className="input pl-9"
            type="url"
            inputMode="url"
            placeholder="貼上 YouTube / Podcast / 音訊連結"
            value={url}
            onChange={(e) => setUrl(e.target.value)}
          />
        </div>
        <button
          type="button"
          className="btn btn-secondary shrink-0 px-3"
          onClick={() => void pasteFromClipboard()}
          aria-label="貼上剪貼簿連結"
          title="貼上剪貼簿連結"
        >
          <ClipboardPaste className="size-4" />
        </button>
      </div>
      <button
        type="submit"
        className="btn btn-secondary w-full sm:w-auto sm:self-center"
        disabled={state.phase === 'submitting' || !url.trim()}
      >
        <Link2 className="size-4" />
        {state.phase === 'submitting' ? '匯入中…' : '貼連結匯入'}
      </button>
      {state.phase === 'error' && <p className="m-0 text-xs text-red-500">{state.message}</p>}
      <p className="m-0 text-center text-xs text-muted">支援公開影片/單集，90 分鐘、200MB 以內</p>
    </form>
  )
}
