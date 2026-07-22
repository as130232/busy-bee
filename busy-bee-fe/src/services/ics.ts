import type { ActionItem, PendingActionItem } from './api/client'

type Item = ActionItem | PendingActionItem

const TIME_ZONE = 'Asia/Taipei'

// icsDate 取 dueAt 在台北時區的日期，輸出 iCalendar 全日事件用的 YYYYMMDD。
function icsDate(iso: string): string {
  // en-CA locale 產生 YYYY-MM-DD，去掉連字號即為 ICS DATE 值。
  return new Intl.DateTimeFormat('en-CA', { timeZone: TIME_ZONE }).format(new Date(iso)).replaceAll('-', '')
}

// nextDay 全日事件的 DTEND 為隔日（非包含結束），加 24h 後再取台北日期。
function nextDay(iso: string): string {
  return icsDate(new Date(new Date(iso).getTime() + 86_400_000).toISOString())
}

// stamp 目前時間的 UTC 緊湊格式（YYYYMMDDTHHMMSSZ），作 DTSTAMP。
function stamp(): string {
  return new Date().toISOString().replace(/[-:]/g, '').replace(/\.\d{3}/, '')
}

// escapeText 依 RFC 5545 轉義文字欄位中的反斜線、逗號、分號與換行。
function escapeText(s: string): string {
  return s.replace(/\\/g, '\\\\').replace(/([,;])/g, '\\$1').replace(/\n/g, '\\n')
}

function meetingTitleOf(item: Item): string | undefined {
  return 'meetingTitle' in item ? item.meetingTitle : undefined
}

// buildICS 由行動項組出單一全日事件的 .ics 內容（呼叫端需先確認 dueAt 存在）。
export function buildICS(item: Item, dueAt: string): string {
  const descParts = [
    item.assignee && `負責人：${item.assignee}`,
    item.dueText && `時限：${item.dueText}`,
    meetingTitleOf(item) && `會議：${meetingTitleOf(item)}`,
  ].filter(Boolean) as string[]

  const lines = [
    'BEGIN:VCALENDAR',
    'VERSION:2.0',
    'PRODID:-//Busy Bee//Action Item//ZH',
    'CALSCALE:GREGORIAN',
    'BEGIN:VEVENT',
    `UID:${item.id}@busy-bee`,
    `DTSTAMP:${stamp()}`,
    `DTSTART;VALUE=DATE:${icsDate(dueAt)}`,
    `DTEND;VALUE=DATE:${nextDay(dueAt)}`,
    `SUMMARY:${escapeText(item.description)}`,
    descParts.length > 0 && `DESCRIPTION:${escapeText(descParts.join('\n'))}`,
    'END:VEVENT',
    'END:VCALENDAR',
  ].filter(Boolean) as string[]

  return lines.join('\r\n')
}

// addToCalendar 行動優先：可分享檔案則走系統分享（iOS 顯示「加入行事曆」），否則下載 .ics。
export async function addToCalendar(item: Item): Promise<void> {
  if (!item.dueAt) return
  const ics = buildICS(item, item.dueAt)
  const filename = `${item.description.slice(0, 40) || 'todo'}.ics`
  const file = new File([ics], filename, { type: 'text/calendar' })

  if (typeof navigator.canShare === 'function' && navigator.canShare({ files: [file] })) {
    try {
      await navigator.share({ files: [file], title: item.description })
      return
    } catch (e) {
      if (e instanceof Error && e.name === 'AbortError') return // 用戶取消
      // 分享失敗則退回下載
    }
  }

  const url = URL.createObjectURL(file)
  const a = document.createElement('a')
  a.href = url
  a.download = filename
  a.click()
  URL.revokeObjectURL(url)
}
