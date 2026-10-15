import type { Meeting } from '../services/api/client'

/** 顯示用狀態：多一個「即時會議已結束、錄音待上傳」（後端為 scheduled + liveStartedAt）。 */
export type DisplayStatus = Meeting['status'] | 'awaitingUpload'

/** 依紀錄算出顯示用狀態。 */
export function displayStatus(m: Pick<Meeting, 'status' | 'liveStartedAt'>): DisplayStatus {
  return m.status === 'scheduled' && m.liveStartedAt ? 'awaitingUpload' : m.status
}
