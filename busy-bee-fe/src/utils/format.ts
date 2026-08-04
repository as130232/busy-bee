// 共用格式化工具：時間碼、時長、日期時間。集中一處避免各元件各自重複定義
//（原散落於 MeetingDetailPage / SummarySections / RecorderPanel / MeetingList / AskPage）。

/** 秒 → m:ss（時間碼／播放器共用）；容忍非有限值與負數，一律歸零。 */
export function formatClock(seconds: number): string {
  const safe = !Number.isFinite(seconds) || seconds < 0 ? 0 : seconds
  const m = Math.floor(safe / 60)
  const s = Math.floor(safe % 60)
  return `${m}:${String(s).padStart(2, '0')}`
}

/** 時長 → 「X 分 Y 秒」（不足 1 分只顯示秒；0 秒回空字串）。 */
export function formatDuration(totalSeconds: number): string {
  if (totalSeconds <= 0) return ''
  const min = Math.floor(totalSeconds / 60)
  const sec = Math.floor(totalSeconds % 60)
  if (min === 0) return `${sec} 秒`
  if (sec === 0) return `${min} 分`
  return `${min} 分 ${sec} 秒`
}

const dateTimeFmt: Intl.DateTimeFormatOptions = {
  year: 'numeric',
  month: '2-digit',
  day: '2-digit',
  hour: '2-digit',
  minute: '2-digit',
  hour12: false,
}

/** ISO 字串 / 時間戳 / Date → zh-TW 「YYYY/MM/DD HH:mm」（24 小時制）。 */
export function formatDateTime(value: string | number | Date): string {
  return new Date(value).toLocaleString('zh-TW', dateTimeFmt)
}
