import { useEffect, useState } from 'react'

import { useApiCall } from '../hooks/useApiCall'
import { replaceMeetingText, type MeetingDetail, type ReplaceCounts } from '../services/api/client'
import { Sheet } from './Sheet'

/** 輸入停頓多久後預覽符合筆數。 */
const PREVIEW_DELAY_MS = 400

function describeCounts(c: ReplaceCounts, includeSummary: boolean): string {
  const parts = [`逐字稿 ${c.transcript} 處`]
  if (includeSummary) parts.push(`摘要 ${c.summary} 處`, `行動項 ${c.actionItems} 處`)
  return parts.join('、')
}

/** 逐字稿「尋找與取代」：一次修正同一個辨識錯字（如 DBA → DVA），可一併改摘要與行動項，不呼叫 AI。 */
export function TranscriptReplaceSheet({
  meeting,
  onClose,
  onDone,
}: {
  meeting: MeetingDetail
  onClose: () => void
  onDone: (m: MeetingDetail) => void
}) {
  const [from, setFrom] = useState('')
  const [to, setTo] = useState('')
  const [includeSummary, setIncludeSummary] = useState(true)
  const [counts, setCounts] = useState<ReplaceCounts | null>(null)
  const [busy, setBusy] = useState(false)
  const [err, setErr] = useState<string | null>(null)
  const call = useApiCall()

  const f = from.trim()
  const t = to.trim()
  const ready = f !== '' && t !== '' && f !== t

  useEffect(() => {
    setCounts(null)
    if (!ready) return
    let cancelled = false
    const timer = setTimeout(() => {
      call(replaceMeetingText, meeting.id, { from: f, to: t, includeSummary, dryRun: true })
        .then((res) => !cancelled && setCounts(res.counts))
        .catch((e: unknown) => !cancelled && setErr(e instanceof Error ? e.message : '預覽失敗'))
    }, PREVIEW_DELAY_MS)
    return () => {
      cancelled = true
      clearTimeout(timer)
    }
  }, [call, meeting.id, f, t, includeSummary, ready])

  const total = counts ? counts.transcript + (includeSummary ? counts.summary + counts.actionItems : 0) : 0

  const submit = async () => {
    setBusy(true)
    setErr(null)
    try {
      const { meeting: updated } = await call(replaceMeetingText, meeting.id, { from: f, to: t, includeSummary })
      onDone(updated)
    } catch (e) {
      setErr(e instanceof Error ? e.message : '取代失敗')
      setBusy(false)
    }
  }

  return (
    <Sheet onClose={onClose}>
      <p className="m-0 text-sm font-medium">尋找與取代</p>
      <label className="flex flex-col gap-1 text-xs text-muted">
        尋找
        <input
          className="input h-10"
          value={from}
          placeholder="例如 DBA"
          disabled={busy}
          autoFocus
          onChange={(e) => {
            setErr(null)
            setFrom(e.target.value)
          }}
        />
      </label>
      <label className="flex flex-col gap-1 text-xs text-muted">
        取代為
        <input
          className="input h-10"
          value={to}
          placeholder="例如 DVA"
          disabled={busy}
          onChange={(e) => {
            setErr(null)
            setTo(e.target.value)
          }}
        />
      </label>
      <label className="flex items-center gap-2 text-sm">
        <input type="checkbox" checked={includeSummary} disabled={busy} onChange={(e) => setIncludeSummary(e.target.checked)} />
        同時取代摘要與行動項
      </label>
      <p className="m-0 text-xs leading-5 text-muted">
        大小寫需相符；英文與數字只比對完整單字（DBA 不會改到 DBAs）。改過的段落之後 AI 校正不會再動。
      </p>
      {counts && (
        <p className="m-0 text-sm">{total > 0 ? describeCounts(counts, includeSummary) : '找不到符合的文字'}</p>
      )}
      {err && <p className="m-0 text-xs text-danger">{err}</p>}
      <div className="flex gap-2">
        <button type="button" className="btn btn-secondary flex-1" onClick={onClose} disabled={busy}>
          取消
        </button>
        <button
          type="button"
          className="btn btn-primary flex-1"
          onClick={() => void submit()}
          disabled={busy || !ready || total === 0}
        >
          {busy ? '取代中…' : total > 0 ? `全部取代（${total} 處）` : '全部取代'}
        </button>
      </div>
    </Sheet>
  )
}
