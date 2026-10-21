import { useCallback, useEffect, useState } from 'react'

import {
  getCorrection,
  revertCorrection,
  startCorrection,
  type CorrectionState,
  type MeetingDetail,
} from '../services/api/client'
import { useApiCall } from '../hooks/useApiCall'
import { CorrectionGuidanceFields } from './CorrectionGuidanceFields'
import { emptyGuidanceDraft, toGuidance, type GuidanceDraft } from './correctionGuidance'
import { Sheet } from './Sheet'

/** 低信心比例達此門檻時建議使用者校正。 */
const SUGGEST_THRESHOLD = 0.05

/**
 * AI 錄音重新校正（F-CORRECT）：校正按鈕、已用次數、建議提示、進行中/失敗狀態、整份還原。
 * meeting 物件更新（WS 事件 resync / 還原後重抓）時重新取得校正狀態。
 * 只在「有事要說」時（建議校正、進行中、失敗、可還原、錯誤）顯示狀態列，平時不佔版面；
 * 一般入口在詳情頁「⋯」選單，透過遞增的 startSignal 開啟確認視窗。
 */
export function CorrectionPanel({
  meeting,
  onChanged,
  startSignal = 0,
}: {
  meeting: MeetingDetail
  onChanged: () => void
  startSignal?: number
}) {
  const call = useApiCall()
  const [state, setState] = useState<CorrectionState | null>(null)
  const [confirming, setConfirming] = useState<null | 'start' | 'revert'>(null)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [guidanceOpen, setGuidanceOpen] = useState(false)
  const [guidance, setGuidance] = useState<GuidanceDraft>(emptyGuidanceDraft)
  const [guidanceError, setGuidanceError] = useState<string | null>(null)

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

  useEffect(() => {
    if (startSignal > 0) setConfirming('start')
  }, [startSignal])

  const start = useCallback(async () => {
    const { guidance: g, error: gErr } = toGuidance(guidance)
    if (gErr) {
      setGuidanceError(gErr)
      return
    }
    setGuidanceError(null)
    setBusy(true)
    setError(null)
    try {
      const res = g ? await call(startCorrection, meeting.id, g) : await call(startCorrection, meeting.id)
      setState(res.correction)
      setGuidance(emptyGuidanceDraft())
      setGuidanceOpen(false)
    } catch (e) {
      // 例如另一台裝置已開始校正或次數已用完：提示並以伺服器狀態為準
      setError(e instanceof Error ? e.message : '無法開始校正')
      void refresh()
    } finally {
      setConfirming(null)
      setBusy(false)
    }
  }, [call, meeting.id, refresh, guidance])

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
      setConfirming(null)
      setBusy(false)
    }
  }, [call, meeting.id, onChanged, refresh])

  if (!completed || !state) return null
  const running = state.status === 'queued' || state.status === 'running'
  const exhausted = state.attemptsUsed >= state.attemptsMax
  const pct = Math.round(meeting.uncertainRatio * 100)
  const suggest = meeting.uncertainRatio >= SUGGEST_THRESHOLD && !running && !exhausted
  const showBar = suggest || running || state.status === 'failed' || state.canRevert || Boolean(error)

  return (
    <>
      {showBar && (
        <section className="flex flex-col gap-2 rounded-xl border border-accent/25 bg-accent/5 px-3 py-2.5 text-sm">
          {suggest && (
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
                <button type="button" className="btn btn-secondary h-9" disabled={busy} onClick={() => setConfirming('revert')}>
                  還原
                </button>
              )}
              <button
                type="button"
                className="btn btn-primary h-9"
                disabled={busy || running || exhausted}
                onClick={() => setConfirming('start')}
              >
                AI 校正
              </button>
            </div>
          </div>
          {error && (
            <p role="alert" className="m-0 text-xs text-danger">
              {error}
            </p>
          )}
        </section>
      )}
      {confirming === 'start' && (
        <Sheet onClose={() => setConfirming(null)}>
          <h2 className="m-0 text-base font-semibold">AI 重新聽錄音校正</h2>
          <p className="mt-2 text-sm leading-6">
            AI 會重新聽錄音修正逐字稿並更新摘要。每場最多 2 次，失敗也會計次；你手動修改過的段落與行動項不會被覆蓋，完成後可一鍵還原。
          </p>
          <p className="mt-1 text-xs leading-5 text-muted">新版之前手動修改過的段落無法辨識，可能會被校正。</p>
          <p className="m-0 text-xs text-muted">{`已使用 ${state.attemptsUsed}/${state.attemptsMax} 次`}</p>
          {guidanceOpen ? (
            <CorrectionGuidanceFields
              draft={guidance}
              disabled={busy}
              onChange={(d) => {
                setGuidanceError(null)
                setGuidance(d)
              }}
            />
          ) : (
            <button
              type="button"
              className="self-start text-sm text-accent"
              disabled={busy || running || exhausted}
              onClick={() => setGuidanceOpen(true)}
            >
              補充給 AI（選填）
            </button>
          )}
          {guidanceError && (
            <p role="alert" className="m-0 text-xs text-danger">
              {guidanceError}
            </p>
          )}
          <button
            type="button"
            className="btn btn-primary mt-1 h-10 w-full"
            disabled={busy || running || exhausted}
            onClick={() => void start()}
          >
            {exhausted ? '次數已用完' : running ? '校正中…' : '開始校正'}
          </button>
        </Sheet>
      )}
      {confirming === 'revert' && (
        <Sheet onClose={() => setConfirming(null)}>
          <h2 className="m-0 text-base font-semibold">還原 AI 校正</h2>
          <p className="mt-2 text-sm leading-6">
            逐字稿與摘要會恢復成校正前的版本（行動項不變），校正後你手動修改的段落會保留。已用掉的校正次數不會退回。
          </p>
          <button type="button" className="btn btn-primary mt-3 h-10 w-full" disabled={busy} onClick={() => void revert()}>
            確認還原
          </button>
        </Sheet>
      )}
    </>
  )
}
