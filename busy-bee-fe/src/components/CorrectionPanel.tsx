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
  const [error, setError] = useState<string | null>(null)

  const completed = meeting.status === 'completed'
  const refresh = useCallback(async () => {
    try {
      setState((await call(getCorrection, meeting.id)).correction)
    } catch {
      // 取不到狀態時維持現狀（首次失敗則不顯示面板），不影響其他內容
    }
  }, [call, meeting.id])

  useEffect(() => {
    if (completed) void refresh()
  }, [refresh, meeting, completed])

  const start = useCallback(async () => {
    setBusy(true)
    setError(null)
    try {
      setState((await call(startCorrection, meeting.id)).correction)
    } catch (e) {
      // 例如另一台裝置已開始校正或次數已用完：提示並以伺服器狀態為準
      setError(e instanceof Error ? e.message : '無法開始校正')
      void refresh()
    } finally {
      setConfirming(false)
      setBusy(false)
    }
  }, [call, meeting.id, refresh])

  const revert = useCallback(async () => {
    setBusy(true)
    setError(null)
    try {
      setState((await call(revertCorrection, meeting.id)).correction)
      onChanged()
    } catch (e) {
      setError(e instanceof Error ? e.message : '無法還原')
      void refresh()
    } finally {
      setBusy(false)
    }
  }, [call, meeting.id, onChanged, refresh])

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
      {error && (
        <p role="alert" className="m-0 text-xs text-red-600">
          {error}
        </p>
      )}
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
