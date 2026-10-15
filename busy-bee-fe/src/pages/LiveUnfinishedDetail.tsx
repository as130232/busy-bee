import { useEffect, useState } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { ChevronLeft, HardDriveUpload, LoaderCircle, Trash2, Upload } from 'lucide-react'

import { Sheet } from '../components/Sheet'
import { StatusBadge } from '../components/StatusBadge'
import { displayStatus } from '../utils/meetingStatus'
import { AppShell } from '../components/AppShell'
import { useApiCall } from '../hooks/useApiCall'
import { defaultRecoveryDeps, recordingStore } from '../live/defaults'
import { uploadRecovered } from '../live/recovery'
import type { StoredSession } from '../live/recordingStore'
import { completeUpload, deleteMeeting, finishLive, type MeetingDetail } from '../services/api/client'
import { putWithProgress } from '../services/upload'

/**
 * 未完成的即時會議（status=live，或已結束但錄音還沒上傳）：
 * 本機有備份就直接上傳；沒有則可手動選錄音檔上傳，或刪除。
 */
export function LiveUnfinishedDetail({ meeting, onUploaded }: { meeting: MeetingDetail; onUploaded: () => void }) {
  const navigate = useNavigate()
  const call = useApiCall()
  const [local, setLocal] = useState<StoredSession | null>(null)
  const [percent, setPercent] = useState<number | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [confirmDelete, setConfirmDelete] = useState(false)

  useEffect(() => {
    recordingStore
      .list()
      .then((list) => setLocal(list.find((s) => s.meetingId === meeting.id) ?? null))
      .catch(() => setLocal(null))
  }, [meeting.id])

  const run = async (fn: () => Promise<void>) => {
    setError(null)
    setPercent(0)
    try {
      await fn()
      onUploaded()
    } catch (e) {
      setError(e instanceof Error ? e.message : '上傳失敗')
    } finally {
      setPercent(null)
    }
  }

  const uploadLocal = () => run(() => uploadRecovered(defaultRecoveryDeps(), local!, setPercent))

  const uploadFile = (file: File) =>
    run(async () => {
      const { upload } = await call(finishLive, meeting.id, meeting.liveNotes ?? { notes: { current: '', points: [] }, qa: [] })
      await putWithProgress(upload.url, upload.headers, file, setPercent)
      await call(completeUpload, meeting.id)
    })

  const remove = async () => {
    try {
      await call(deleteMeeting, meeting.id)
      await recordingStore.remove(meeting.id).catch(() => {})
      navigate('/meetings')
    } catch (e) {
      setError(e instanceof Error ? e.message : '刪除失敗')
      setConfirmDelete(false)
    }
  }

  const busy = percent !== null

  return (
    <AppShell hideTopBar>
      <header className="flex items-center gap-2">
        <Link to="/meetings" className="btn btn-ghost size-11 shrink-0 px-0" aria-label="返回">
          <ChevronLeft className="size-5" />
        </Link>
        <h1 className="m-0 min-w-0 flex-1 truncate text-lg font-semibold">{meeting.title}</h1>
        <StatusBadge status={displayStatus(meeting)} />
      </header>

      <section className="flex flex-col gap-3 rounded-xl border border-border bg-surface p-4 text-sm">
        <p className="m-0">這場即時會議還沒有上傳錄音，上傳後會自動產生完整紀錄。</p>
        {local ? (
          <button type="button" className="btn btn-primary" disabled={busy} onClick={() => void uploadLocal()}>
            <HardDriveUpload className="size-4" />
            上傳這台電腦保存的錄音
          </button>
        ) : (
          <p className="m-0 text-xs text-muted">這台電腦沒有這場的錄音備份；如果你另外有錄音檔，可以手動上傳。</p>
        )}
        <label className={`btn btn-secondary cursor-pointer ${busy ? 'pointer-events-none opacity-50' : ''}`}>
          <Upload className="size-4" />
          選擇錄音檔上傳
          <input
            type="file"
            accept="audio/*"
            className="sr-only"
            onChange={(e) => {
              const f = e.target.files?.[0]
              e.target.value = ''
              if (f) void uploadFile(f)
            }}
          />
        </label>
        {busy && (
          <p className="m-0 flex items-center gap-1.5 text-xs text-muted">
            <LoaderCircle className="size-3.5 animate-spin" />
            上傳中… {percent}%
          </p>
        )}
        {error && <p className="m-0 text-xs text-red-500">{error}</p>}
      </section>

      <button type="button" className="btn btn-secondary text-red-500" onClick={() => setConfirmDelete(true)}>
        <Trash2 className="size-4" />
        刪除這場會議
      </button>

      {confirmDelete && (
        <Sheet onClose={() => setConfirmDelete(false)}>
          <p className="m-0 text-sm">確定刪除「{meeting.title}」？會議中筆記與本機錄音備份會一併刪除，無法復原。</p>
          <div className="flex gap-2">
            <button type="button" className="btn btn-secondary flex-1" onClick={() => setConfirmDelete(false)}>
              取消
            </button>
            <button type="button" className="btn btn-primary flex-1 bg-red-600" onClick={() => void remove()}>
              刪除
            </button>
          </div>
        </Sheet>
      )}
    </AppShell>
  )
}
