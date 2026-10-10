import { useCallback, useEffect, useState } from 'react'

import {
  getCorrection,
  revertCorrection,
  startCorrection,
  type CorrectionState,
  type MeetingDetail,
} from '../services/api/client'
import { useApiCall } from '../hooks/useApiCall'
import { Sheet } from './Sheet'

/** 低信心比例達此門檻時建議使用者校正。 */
const SUGGEST_THRESHOLD = 0.05

/**
 * AI 錄音重新校正（F-CORRECT）：校正按鈕、已用次數、建議提示、進行中/失敗狀態、整份還原。
 * meeting 物件更新（WS 事件 resync / 還原後重抓）時重新取得校正狀態。
 */
export function CorrectionPanel({ meeting, onChanged }: { meeting: MeetingDetail; onChanged: () => void }) {
  const call = useApiCall()
  const [state, setState] = useState<CorrectionState | null>(null)
  const [confirming, setConfirming] = useState(false)
  const [busy, setBusy] = useState(false)

  const completed = meeting.status === 'completed'
  useEffect(() => {
    if (!completed) return
    let cancelled = false
    call(getCorrection, meeting.id)
      .then((r) => {
        if (!cancelled) setState(r.correction)
      })
      .catch(() => {}) // 取不到狀態時不顯示面板，不影響其他內容
    return () => {
      cancelled = true
    }
  }, [call, meeting, completed])

  const start = useCallback(async () => {
    setBusy(true)
    try {
      setState((await call(startCorrection, meeting.id)).correction)
      setConfirming(false)
    } finally {
      setBusy(false)
    }
  }, [call, meeting.id])

  const revert = useCallback(async () => {
    setBusy(true)
    try {
      setState((await call(revertCorrection, meeting.id)).correction)
      onChanged()
    } finally {
      setBusy(false)
    }
  }, [call, meeting.id, onChanged])

  if (!completed || !state) return null
  const running = state.status === 'queued' || state.status === 'running'
  const exhausted = state.attemptsUsed >= state.attemptsMax
  const pct = Math.round(meeting.uncertainRatio * 100)

  return (
    <section className="flex flex-col gap-2 rounded-lg border border-border bg-surface px-3 py-3 text-sm">
      {meeting.uncertainRatio >= SUGGEST_THRESHOLD && !running && (
        <p className="m-0 text-xs text-muted">{`本紀錄約 ${pct}% 內容語音辨識不確定，建議校正。`}</p>
      )}
      <div className="flex items-center justify-between gap-2">
        <span className="text-xs text-muted">
          {`AI 校正 ${state.attemptsUsed}/${state.attemptsMax}`}
          {running && '・校正中…'}
          {state.status === 'failed' && `・${state.errorMessage || '校正失敗'}`}
        </span>
        <div className="flex gap-2">
          {state.canRevert && (
            <button type="button" className="btn btn-secondary h-9" disabled={busy} onClick={() => void revert()}>
              還原
            </button>
          )}
          <button
            type="button"
            className="btn btn-primary h-9"
            disabled={busy || running || exhausted}
            onClick={() => setConfirming(true)}
          >
            AI 校正
          </button>
        </div>
      </div>
      {confirming && (
        <Sheet onClose={() => setConfirming(false)}>
          <h2 className="m-0 text-base font-semibold">AI 重新聽錄音校正</h2>
          <p className="mt-2 text-sm leading-6">
            AI 會重新聽錄音修正逐字稿並更新摘要。每場最多 2 次，失敗也會計次；你手動修改過的段落與行動項不會被覆蓋，完成後可一鍵還原。
          </p>
          <p className="mt-1 text-xs leading-5 text-muted">新版之前手動修改過的段落無法辨識，可能會被校正。</p>
          <button type="button" className="btn btn-primary mt-3 h-10 w-full" disabled={busy} onClick={() => void start()}>
            開始校正
          </button>
        </Sheet>
      )}
    </section>
  )
}
