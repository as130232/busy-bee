import { useEffect, useState } from 'react'
import { HardDriveUpload, LoaderCircle } from 'lucide-react'

import { defaultRecoveryDeps, recordingStore } from '../live/defaults'
import { discardRecovered, uploadRecovered } from '../live/recovery'
import type { StoredSession } from '../live/recordingStore'
import { formatDateTime } from '../utils/format'
import { Sheet } from './Sheet'

/** App 開啟時檢查本機是否有未上傳的即時會議錄音（分頁意外關閉），提示上傳或捨棄。 */
export function LiveRecoveryPrompt() {
  const [pending, setPending] = useState<StoredSession[]>([])
  const [percent, setPercent] = useState<number | null>(null)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    if (typeof indexedDB === 'undefined') return
    recordingStore
      .list()
      .then(setPending)
      .catch(() => setPending([]))
  }, [])

  const current = pending[0]
  if (!current) return null

  const next = () => {
    setPending((p) => p.slice(1))
    setPercent(null)
    setError(null)
  }

  const upload = async () => {
    setError(null)
    setPercent(0)
    try {
      await uploadRecovered(defaultRecoveryDeps(), current, setPercent)
      next()
    } catch (e) {
      setPercent(null)
      setError(e instanceof Error ? e.message : '上傳失敗')
    }
  }

  const discard = async () => {
    await discardRecovered(defaultRecoveryDeps(), current.meetingId).catch(() => {})
    next()
  }

  return (
    <Sheet onClose={() => setPending([])}>
      <h2 className="m-0 flex items-center gap-2 text-base font-semibold">
        <HardDriveUpload className="size-5 text-accent" />
        有一場未完成的即時會議
      </h2>
      <p className="m-0 text-sm">
        「{current.title}」（{formatDateTime(current.startedAt)}）的錄音保存在這台電腦，還沒上傳。
      </p>
      {percent !== null && (
        <p className="m-0 flex items-center gap-1.5 text-xs text-muted">
          <LoaderCircle className="size-3.5 animate-spin" />
          上傳中… {percent}%
        </p>
      )}
      {error && <p className="m-0 text-xs text-red-500">{error}</p>}
      <div className="flex gap-2">
        <button type="button" className="btn btn-secondary flex-1" disabled={percent !== null} onClick={() => void discard()}>
          捨棄
        </button>
        <button type="button" className="btn btn-primary flex-1" disabled={percent !== null} onClick={() => void upload()}>
          上傳
        </button>
      </div>
      <p className="m-0 text-xs text-muted">按旁邊空白處可稍後再處理。</p>
    </Sheet>
  )
}
