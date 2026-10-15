// 即時會議錄音救回（F-LIVE）：busy-bee 分頁意外關閉後，從 IndexedDB 取回錄音上傳或捨棄。
import type { completeUpload, deleteMeeting, finishLive } from '../services/api/client'
import type { StoredSession } from './recordingStore'

export interface RecoveryDeps {
  getIdToken: () => Promise<string>
  store: {
    getRecording(meetingId: string): Promise<Blob | null>
    remove(meetingId: string): Promise<void>
  }
  api: {
    finishLive: typeof finishLive
    completeUpload: typeof completeUpload
    deleteMeeting: typeof deleteMeeting
  }
  upload: (url: string, headers: Record<string, string>, blob: Blob, onProgress: (p: number) => void) => Promise<void>
}

/** 上傳救回的錄音：finish（冪等，已結束會重發上傳 URL）→ 直傳 → complete-upload → 清本機。 */
export async function uploadRecovered(
  deps: RecoveryDeps,
  session: StoredSession,
  onProgress: (p: number) => void,
): Promise<void> {
  const blob = await deps.store.getRecording(session.meetingId)
  if (!blob) throw new Error('本機找不到這場會議的錄音')
  const idToken = await deps.getIdToken()
  const notes = session.notes ?? { notes: { current: '', points: [] }, qa: [] }
  const { upload } = await deps.api.finishLive(idToken, session.meetingId, notes)
  await deps.upload(upload.url, upload.headers, blob, onProgress)
  await deps.api.completeUpload(await deps.getIdToken(), session.meetingId)
  await deps.store.remove(session.meetingId)
}

/** 捨棄：刪除紀錄（已不存在也無妨）與本機備份。 */
export async function discardRecovered(deps: RecoveryDeps, meetingId: string): Promise<void> {
  try {
    await deps.api.deleteMeeting(await deps.getIdToken(), meetingId)
  } catch {
    /* 紀錄可能已被刪除；本機備份照樣清掉 */
  }
  await deps.store.remove(meetingId)
}
